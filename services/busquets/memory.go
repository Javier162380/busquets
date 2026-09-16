package busquets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Javier162380/busquets/internal/connectors"
	"github.com/Javier162380/busquets/internal/gitdiff"
	"github.com/Javier162380/busquets/services/busquets/dto"
)

// MemoryEventKind identifies what kind of change a timeline entry describes.
type MemoryEventKind string

const (
	MemoryEventVersion MemoryEventKind = "version"
	MemoryEventComment MemoryEventKind = "comment"
	MemoryEventRestore MemoryEventKind = "restore"
)

// NoMemoryCoverage is the covers_up_to_version of a memory that has narrated
// nothing. It is -1, not 0, because v0 is a real version.
const NoMemoryCoverage int64 = -1

// MemoryMode selects how a generation run treats the existing memory.
type MemoryMode string

const (
	MemoryModeIncremental MemoryMode = "incremental"
	MemoryModeRebuild     MemoryMode = "rebuild"
)

// MemoryEvent is one entry on a plan's timeline. Every field except Narrative is
// computed from plan_versions/plan_comments.
type MemoryEvent struct {
	Kind MemoryEventKind
	// RefID is plan_versions.id, or plan_comments.id for comment events. It is
	// how a caller-supplied narrative is matched back to a computed event.
	RefID int64
	// Pointers because 0 is a legitimate value (v0), so nil differs from zero.
	VersionNumber *int64
	RestoredFrom  *int64
	OccurredAt    time.Time
	LinesAdded    int
	LinesRemoved  int
	WordCount     int
	Body          string // comment text; empty on version events
	Narrative     string // transport only; "" means not narrated
}

// PlanMemory is a plan's stored memory plus the timeline it describes. Events is
// always recomputed on read, never loaded from storage.
type PlanMemory struct {
	ID                  int64
	PlanID              int64
	FileName            string
	SyncSource          string
	SyncLabel           string
	PlanTitle           string
	FilePath            string
	Content             string
	Summary             string
	CoversUpToVersion   int64
	CoversUpToCommentID int64
	GeneratedBy         string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Events              []MemoryEvent
}

// Exists reports whether a memory has been generated. A PlanMemory with
// Exists() == false still carries a valid computed timeline.
func (m PlanMemory) Exists() bool {
	return m.ID != 0
}

// MemoryStaleness reports how far a memory has fallen behind its plan. With no
// memory the counts are the totals.
type MemoryStaleness struct {
	HasMemory   bool
	NewVersions int
	NewComments int
}

// IsStale reports whether anything is left to narrate.
func (m MemoryStaleness) IsStale() bool {
	return m.NewVersions > 0 || m.NewComments > 0
}

// BuildMemoryTimeline computes a plan's timeline from its versions and comments.
// No LLM, nothing persisted: derived fresh every time, so it cannot go stale.
func (s *Service) BuildMemoryTimeline(ctx context.Context, fileName, syncSource string) ([]MemoryEvent, error) {
	history, err := s.loadPlanHistory(ctx, fileName, syncSource)
	if err != nil {
		return nil, err
	}
	return history.events, nil
}

// planHistory is everything the memory paths need about a plan, loaded once.
type planHistory struct {
	plan     dto.Plan
	versions []dto.PlanVersion // ascending by version number
	comments []dto.Comment
	events   []MemoryEvent
}

func (s *Service) loadPlanHistory(ctx context.Context, fileName, syncSource string) (planHistory, error) {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return planHistory{}, fmt.Errorf("plan not found: %w", err)
	}

	versions, err := s.db.ListPlanVersionsAll(ctx, plan.ID)
	if err != nil {
		return planHistory{}, fmt.Errorf("failed to list plan versions: %w", err)
	}

	comments, err := s.db.GetPlanComments(ctx, plan.ID)
	if err != nil {
		return planHistory{}, fmt.Errorf("failed to get plan comments: %w", err)
	}

	return planHistory{
		plan:     plan,
		versions: sortedVersions(versions),
		comments: comments,
		events:   buildTimeline(versions, comments),
	}, nil
}

// sortedVersions returns versions ordered by version number, leaving the input
// untouched.
func sortedVersions(versions []dto.PlanVersion) []dto.PlanVersion {
	ordered := make([]dto.PlanVersion, len(versions))
	copy(ordered, versions)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].VersionNumber < ordered[j].VersionNumber
	})
	return ordered
}

// buildTimeline merges versions and comments into one chronological timeline.
// Free of the service receiver so it can be tested without a database.
func buildTimeline(versions []dto.PlanVersion, comments []dto.Comment) []MemoryEvent {
	events := make([]MemoryEvent, 0, len(versions)+len(comments))

	// ListPlanVersionsAll has no ORDER BY, and every diff below compares against
	// the preceding element.
	ordered := sortedVersions(versions)

	// Earliest version carrying each content, so a later repeat reads as a
	// restore rather than a baffling large diff.
	contentOrigin := make(map[string]int64, len(ordered))

	for i, version := range ordered {
		versionNumber := version.VersionNumber
		event := MemoryEvent{
			Kind:          MemoryEventVersion,
			RefID:         version.ID,
			VersionNumber: &versionNumber,
			OccurredAt:    version.CreatedAt,
			WordCount:     int(version.WordCount),
		}

		// The first version is a baseline, not a change, so it carries no diff.
		if i > 0 {
			event.LinesAdded, event.LinesRemoved = gitdiff.Stats(ordered[i-1].Content, version.Content)
		}

		if origin, seen := contentOrigin[version.Content]; seen {
			restoredFrom := origin
			event.Kind = MemoryEventRestore
			event.RestoredFrom = &restoredFrom
		} else {
			contentOrigin[version.Content] = versionNumber
		}

		events = append(events, event)
	}

	for _, comment := range comments {
		events = append(events, MemoryEvent{
			Kind:       MemoryEventComment,
			RefID:      comment.ID,
			OccurredAt: comment.CreatedAt,
			Body:       comment.Content,
		})
	}

	sortTimeline(events)
	return events
}

// sortTimeline orders events oldest first — a timeline reads forward.
func sortTimeline(events []MemoryEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		left, right := events[i], events[j]
		if !left.OccurredAt.Equal(right.OccurredAt) {
			return left.OccurredAt.Before(right.OccurredAt)
		}
		// Same instant: the version comes first, since a comment reacts to
		// content that already exists.
		if (left.Kind == MemoryEventComment) != (right.Kind == MemoryEventComment) {
			return right.Kind == MemoryEventComment
		}
		return left.RefID < right.RefID
	})
}

// GetPlanMemory returns a plan's stored memory together with its freshly
// computed timeline. A plan with no memory is not an error: the result has
// Exists() == false and still carries the timeline.
func (s *Service) GetPlanMemory(ctx context.Context, fileName, syncSource string) (*PlanMemory, error) {
	history, err := s.loadPlanHistory(ctx, fileName, syncSource)
	if err != nil {
		return nil, err
	}
	plan, events := history.plan, history.events

	memory := PlanMemory{
		PlanID:              plan.ID,
		FileName:            plan.FileName,
		SyncSource:          plan.SyncSource,
		SyncLabel:           s.labelForSource(plan.SyncSource),
		PlanTitle:           plan.Title,
		FilePath:            s.memoryPathFor(plan.ID),
		CoversUpToVersion:   NoMemoryCoverage,
		CoversUpToCommentID: 0,
		Events:              events,
	}

	stored, err := s.db.GetPlanMemoryByPlanID(ctx, plan.ID)
	switch {
	case err == nil:
		memory.ID = stored.ID
		memory.FilePath = stored.FilePath
		memory.Content = stored.Content
		memory.Summary = stored.Summary
		memory.CoversUpToVersion = stored.CoversUpToVersion
		memory.CoversUpToCommentID = stored.CoversUpToCommentID
		memory.GeneratedBy = stored.GeneratedBy
		memory.CreatedAt = stored.CreatedAt
		memory.UpdatedAt = stored.UpdatedAt
	case dto.IsNotFound(err):
		// Nothing generated yet; the computed timeline stands on its own.
	default:
		return nil, fmt.Errorf("failed to get plan memory: %w", err)
	}

	return &memory, nil
}

// MemoryStaleness reports how many events a plan's memory has yet to narrate.
func (s *Service) MemoryStaleness(ctx context.Context, fileName, syncSource string) (MemoryStaleness, error) {
	history, err := s.loadPlanHistory(ctx, fileName, syncSource)
	if err != nil {
		return MemoryStaleness{}, err
	}
	plan := history.plan

	// Watermarks of a memory that has narrated nothing, so a plan without one
	// reports every event as new.
	coversVersion, coversComment := NoMemoryCoverage, int64(0)
	hasMemory := false

	stored, err := s.db.GetPlanMemoryByPlanID(ctx, plan.ID)
	switch {
	case err == nil:
		hasMemory = true
		coversVersion = stored.CoversUpToVersion
		coversComment = stored.CoversUpToCommentID
	case dto.IsNotFound(err):
	default:
		return MemoryStaleness{}, fmt.Errorf("failed to get plan memory: %w", err)
	}

	staleness := MemoryStaleness{HasMemory: hasMemory}
	for _, version := range history.versions {
		if version.VersionNumber > coversVersion {
			staleness.NewVersions++
		}
	}
	for _, comment := range history.comments {
		if comment.ID > coversComment {
			staleness.NewComments++
		}
	}

	return staleness, nil
}

// SavePlanMemoryEvent is one caller-written narrative. Only the identity of the
// event and its prose are accepted: timestamps and change stats are computed.
type SavePlanMemoryEvent struct {
	Kind      MemoryEventKind
	RefID     int64
	Narrative string
}

// SavePlanMemoryRequest persists a memory written elsewhere — by the MCP caller
// or the summary connector.
type SavePlanMemoryRequest struct {
	FileName    string
	SyncSource  string
	Mode        MemoryMode
	Summary     string
	GeneratedBy string
	Events      []SavePlanMemoryEvent
}

// MemoryEventPrompt is one event plus the request that will produce its
// narrative.
type MemoryEventPrompt struct {
	Event      MemoryEvent
	UserPrompt string
}

// MemoryPromptSet is everything needed to write a memory without calling an LLM
// here: the prompts, and the events still to narrate.
type MemoryPromptSet struct {
	FileName            string
	SyncSource          string
	PlanTitle           string
	Mode                MemoryMode
	SummarySystemPrompt string
	SummaryUserPrompt   string
	EventSystemPrompt   string
	Events              []MemoryEventPrompt
}

// memoryEventKey identifies an event by the table its RefID belongs to. Version
// and restore events share plan_versions, so a caller labelling a restore as a
// plain version still matches: identity is what matters, not the label.
func memoryEventKey(kind MemoryEventKind, refID int64) string {
	table := "version"
	if kind == MemoryEventComment {
		table = "comment"
	}
	return table + ":" + strconv.FormatInt(refID, 10)
}

// isNewMemoryEvent reports whether an event falls past the given watermarks.
func isNewMemoryEvent(event MemoryEvent, coversVersion, coversComment int64) bool {
	if event.Kind == MemoryEventComment {
		return event.RefID > coversComment
	}
	if event.VersionNumber == nil {
		return false
	}
	return *event.VersionNumber > coversVersion
}

// advanceWatermark returns the highest value whose predecessors in ordered are
// all covered. Advancing to the plain maximum would mark a skipped event as
// narrated, so a gap stops the walk.
func advanceWatermark(ordered []int64, covered map[int64]bool, start int64) int64 {
	watermark := start
	for _, value := range ordered {
		if value <= start {
			continue
		}
		if !covered[value] {
			break
		}
		watermark = value
	}
	return watermark
}

// BuildMemoryPrompts returns the prompts for the events a plan's memory has yet
// to narrate — all of them under MemoryModeRebuild. It calls no LLM.
func (s *Service) BuildMemoryPrompts(ctx context.Context, fileName, syncSource string, mode MemoryMode) (*MemoryPromptSet, error) {
	history, err := s.loadPlanHistory(ctx, fileName, syncSource)
	if err != nil {
		return nil, err
	}

	coversVersion, coversComment := NoMemoryCoverage, int64(0)
	if mode != MemoryModeRebuild {
		stored, storedErr := s.db.GetPlanMemoryByPlanID(ctx, history.plan.ID)
		switch {
		case storedErr == nil:
			coversVersion, coversComment = stored.CoversUpToVersion, stored.CoversUpToCommentID
		case dto.IsNotFound(storedErr):
		default:
			return nil, fmt.Errorf("failed to get plan memory: %w", storedErr)
		}
	}

	positionByVersionID := make(map[int64]int, len(history.versions))
	for i, version := range history.versions {
		positionByVersionID[version.ID] = i
	}

	prompts := make([]MemoryEventPrompt, 0, len(history.events))
	for _, event := range history.events {
		if !isNewMemoryEvent(event, coversVersion, coversComment) {
			continue
		}

		var diff, versionContent string
		if event.Kind != MemoryEventComment {
			position, ok := positionByVersionID[event.RefID]
			if !ok {
				return nil, fmt.Errorf("version %d missing from plan history", event.RefID)
			}
			current := history.versions[position]
			versionContent = current.Content
			if position > 0 {
				previous := history.versions[position-1]
				diff, err = gitdiff.Diff(
					previous.Content, current.Content,
					memoryVersionLabel(previous.VersionNumber, previous.CreatedAt),
					memoryVersionLabel(current.VersionNumber, current.CreatedAt),
				)
				if err != nil {
					return nil, fmt.Errorf("failed to diff version %d: %w", current.VersionNumber, err)
				}
			}
		}

		prompts = append(prompts, MemoryEventPrompt{
			Event:      event,
			UserPrompt: memoryEventUserPrompt(history.plan.Title, event, diff, versionContent),
		})
	}

	return &MemoryPromptSet{
		FileName:            history.plan.FileName,
		SyncSource:          history.plan.SyncSource,
		PlanTitle:           history.plan.Title,
		Mode:                mode,
		SummarySystemPrompt: connectors.SummarySystemPrompt,
		SummaryUserPrompt: fmt.Sprintf("Summarize this plan:\n\nTitle: %s\n\n%s",
			history.plan.Title, history.plan.Content),
		EventSystemPrompt: connectors.MemoryEventSystemPrompt,
		Events:            prompts,
	}, nil
}

// SavePlanMemory persists caller-written narratives.
//
// Narratives are matched against a freshly computed timeline and anything that
// does not correspond to a real event is rejected, so the facts in the rendered
// document are always the application's own.
func (s *Service) SavePlanMemory(ctx context.Context, req SavePlanMemoryRequest) (*PlanMemory, error) {
	history, err := s.loadPlanHistory(ctx, req.FileName, req.SyncSource)
	if err != nil {
		return nil, err
	}

	computed := make(map[string]MemoryEvent, len(history.events))
	for _, event := range history.events {
		computed[memoryEventKey(event.Kind, event.RefID)] = event
	}

	narrated := make([]MemoryEvent, 0, len(req.Events))
	for _, supplied := range req.Events {
		key := memoryEventKey(supplied.Kind, supplied.RefID)
		event, ok := computed[key]
		if !ok {
			return nil, fmt.Errorf("event %s is not in the computed timeline for %s", key, req.FileName)
		}
		if strings.TrimSpace(supplied.Narrative) == "" {
			return nil, fmt.Errorf("event %s has an empty narrative", key)
		}
		event.Narrative = supplied.Narrative
		narrated = append(narrated, event)
	}
	sortTimeline(narrated)

	stored, err := s.db.GetPlanMemoryByPlanID(ctx, history.plan.ID)
	exists := err == nil
	if err != nil && !dto.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get plan memory: %w", err)
	}

	mode := req.Mode
	if mode == "" {
		mode = MemoryModeIncremental
	}
	appending := mode == MemoryModeIncremental && exists

	summary := req.Summary
	if summary == "" && exists {
		summary = stored.Summary
	}

	sections := renderMemorySections(narrated)
	coversVersion, coversComment := NoMemoryCoverage, int64(0)
	if appending {
		if previous := existingMemorySections(stored.Content); previous != "" {
			sections = previous + "\n\n" + sections
		}
		coversVersion, coversComment = stored.CoversUpToVersion, stored.CoversUpToCommentID
	}

	coveredVersions := make(map[int64]bool, len(narrated))
	coveredComments := make(map[int64]bool, len(narrated))
	for _, event := range narrated {
		if event.Kind == MemoryEventComment {
			coveredComments[event.RefID] = true
			continue
		}
		if event.VersionNumber != nil {
			coveredVersions[*event.VersionNumber] = true
		}
	}

	allVersions := make([]int64, 0, len(history.versions))
	for _, version := range history.versions {
		allVersions = append(allVersions, version.VersionNumber)
	}
	allComments := make([]int64, 0, len(history.comments))
	for _, comment := range history.comments {
		allComments = append(allComments, comment.ID)
	}
	sort.Slice(allComments, func(i, j int) bool { return allComments[i] < allComments[j] })

	coversVersion = advanceWatermark(allVersions, coveredVersions, coversVersion)
	coversComment = advanceWatermark(allComments, coveredComments, coversComment)

	generatedBy := req.GeneratedBy
	if generatedBy == "" {
		generatedBy = "mcp"
	}

	now := s.nowProvider.Now()
	createdAt := now
	if exists {
		createdAt = stored.CreatedAt
	}

	content := renderMemoryDocument(history.plan.Title, summary, sections)
	path := s.memoryPathFor(history.plan.ID)

	saved, err := s.db.UpsertPlanMemory(ctx, dto.UpsertPlanMemoryParams{
		PlanID:              history.plan.ID,
		FilePath:            path,
		Content:             content,
		Summary:             summary,
		CoversUpToVersion:   coversVersion,
		CoversUpToCommentID: coversComment,
		GeneratedBy:         generatedBy,
		CreatedAt:           createdAt,
		UpdatedAt:           now,
	}, func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("failed to create memory directory: %w", err)
		}
		return os.WriteFile(path, []byte(content), 0o600)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save plan memory: %w", err)
	}

	return &PlanMemory{
		ID:                  saved.ID,
		PlanID:              history.plan.ID,
		FileName:            history.plan.FileName,
		SyncSource:          history.plan.SyncSource,
		SyncLabel:           s.labelForSource(history.plan.SyncSource),
		PlanTitle:           history.plan.Title,
		FilePath:            saved.FilePath,
		Content:             saved.Content,
		Summary:             saved.Summary,
		CoversUpToVersion:   saved.CoversUpToVersion,
		CoversUpToCommentID: saved.CoversUpToCommentID,
		GeneratedBy:         saved.GeneratedBy,
		CreatedAt:           saved.CreatedAt,
		UpdatedAt:           saved.UpdatedAt,
		Events:              history.events,
	}, nil
}

// DeletePlanMemory removes a plan's memory row and its document.
//
// The file is moved aside inside the transaction rather than unlinked, so a
// failed commit can put it back; it is purged only once the delete has
// committed, when a failure leaves a stray file rather than an inconsistency.
func (s *Service) DeletePlanMemory(ctx context.Context, fileName, syncSource string) error {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	stored, err := s.db.GetPlanMemoryByPlanID(ctx, plan.ID)
	if err != nil {
		return err
	}

	parked := fmt.Sprintf("%s.deleting-%d", stored.FilePath, s.nowProvider.Now().UnixNano())
	moved := false

	err = s.db.DeletePlanMemories(ctx, plan.ID, func() error {
		if renameErr := os.Rename(stored.FilePath, parked); renameErr != nil {
			if os.IsNotExist(renameErr) {
				return nil
			}
			return fmt.Errorf("failed to move memory file aside: %w", renameErr)
		}
		moved = true
		return nil
	})
	if err != nil {
		if moved {
			_ = os.Rename(parked, stored.FilePath)
		}
		return fmt.Errorf("failed to delete plan memory: %w", err)
	}

	if moved {
		if rmErr := os.Remove(parked); rmErr != nil {
			s.logger.Warn("failed to purge deleted memory file", "path", parked, "error", rmErr)
		}
	}
	return nil
}

// SettingMemoryMaxEventsPerRun bounds how many events one generation run
// narrates. A plan with a long history would otherwise fire one LLM call per
// event on the first run; the remainder is picked up by the next refresh.
const SettingMemoryMaxEventsPerRun = "memory_max_events_per_run"

// DefaultMemoryMaxEventsPerRun is the cap applied when the setting is unset.
const DefaultMemoryMaxEventsPerRun = 20

// MemoryMaxEventsPerRun returns the configured cap, falling back to the default.
func (s *Service) MemoryMaxEventsPerRun(ctx context.Context) int {
	setting, exists, err := s.GetSetting(ctx, SettingMemoryMaxEventsPerRun)
	if err != nil || !exists || !setting.IsNumber() {
		return DefaultMemoryMaxEventsPerRun
	}
	if value := int(setting.GetNumberValue()); value > 0 {
		return value
	}
	return DefaultMemoryMaxEventsPerRun
}

// MemoryProgress reports how far a generation run has got. Emitted per event so
// a long first run does not look frozen.
type MemoryProgress struct {
	FileName   string
	SyncSource string
	Current    int
	Total      int
	Done       bool
	Err        error
}

// GetMemoryProgressChannel returns the channel carrying generation progress.
func (s *Service) GetMemoryProgressChannel() <-chan MemoryProgress {
	return s.memoryProgress
}

// emitMemoryProgress publishes progress without blocking: generation must not
// stall because nobody is listening, and a dropped update is only a repaint.
func (s *Service) emitMemoryProgress(progress MemoryProgress) {
	if s.memoryProgress == nil {
		return
	}
	select {
	case s.memoryProgress <- progress:
	default:
	}
}

// GeneratePlanMemory writes a plan's memory using the configured summary
// connector: one call per event, then one for the header summary.
//
// Per-event calls are not an optimisation. A local model's context cannot hold
// the concatenated diffs of a long history, and truncation there is silent, so
// each event is asked about on its own.
func (s *Service) GeneratePlanMemory(ctx context.Context, fileName, syncSource string, mode MemoryMode) (*PlanMemory, error) {
	if s.connectorManager == nil {
		return nil, dto.ErrConnectorDisabled
	}

	prompts, err := s.BuildMemoryPrompts(ctx, fileName, syncSource, mode)
	if err != nil {
		return nil, err
	}
	if len(prompts.Events) == 0 {
		return s.GetPlanMemory(ctx, fileName, syncSource)
	}

	pending := prompts.Events
	if limit := s.MemoryMaxEventsPerRun(ctx); len(pending) > limit {
		pending = pending[:limit]
	}

	narrated := make([]SavePlanMemoryEvent, 0, len(pending))
	truncatedEvents := 0

	for index, prompt := range pending {
		s.emitMemoryProgress(MemoryProgress{
			FileName: fileName, SyncSource: syncSource,
			Current: index + 1, Total: len(pending),
		})

		result, genErr := s.connectorManager.GenerateWithPrompt(ctx, connectors.GeneratorOpts{
			SystemPrompt: prompts.EventSystemPrompt,
			UserPrompt:   prompt.UserPrompt,
		})
		if genErr != nil {
			// Keep whatever has already been written rather than losing a long
			// run to one failure, and surface the error either way.
			if len(narrated) == 0 {
				err := fmt.Errorf("failed to narrate event %d: %w", prompt.Event.RefID, genErr)
				s.emitMemoryProgress(MemoryProgress{
					FileName: fileName, SyncSource: syncSource, Done: true, Err: err,
				})
				return nil, err
			}
			s.logger.Warn("memory generation stopped early",
				"file_name", fileName, "ref_id", prompt.Event.RefID, "error", genErr)
			break
		}

		narrative := strings.TrimSpace(*result.Response)
		if result.Truncated {
			truncatedEvents++
			narrative += "\n\n_(written from truncated content)_"
		}

		narrated = append(narrated, SavePlanMemoryEvent{
			Kind:      prompt.Event.Kind,
			RefID:     prompt.Event.RefID,
			Narrative: narrative,
		})
	}

	if len(narrated) == 0 {
		return nil, fmt.Errorf("no events could be narrated for %s", fileName)
	}

	summary, err := s.generateMemorySummary(ctx, prompts)
	if err != nil {
		return nil, err
	}

	if truncatedEvents > 0 {
		s.logger.Warn("memory narratives written from truncated content",
			"file_name", fileName, "events", truncatedEvents)
	}

	memory, err := s.SavePlanMemory(ctx, SavePlanMemoryRequest{
		FileName:    fileName,
		SyncSource:  syncSource,
		Mode:        mode,
		Summary:     summary,
		GeneratedBy: "ollama",
		Events:      narrated,
	})

	s.emitMemoryProgress(MemoryProgress{
		FileName: fileName, SyncSource: syncSource,
		Current: len(narrated), Total: len(pending), Done: true, Err: err,
	})

	return memory, err
}

// generateMemorySummary writes the document header using the existing TLDR
// prompt, so a memory's summary reads exactly like the plan's TLDR.
func (s *Service) generateMemorySummary(ctx context.Context, prompts *MemoryPromptSet) (string, error) {
	result, err := s.connectorManager.GenerateWithPrompt(ctx, connectors.GeneratorOpts{
		SystemPrompt: prompts.SummarySystemPrompt,
		UserPrompt:   prompts.SummaryUserPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate memory summary: %w", err)
	}
	return strings.TrimSpace(*result.Response), nil
}
