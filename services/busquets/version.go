package busquets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	domain "github.com/Javier162380/busquets"
	"github.com/Javier162380/busquets/internal/gitdiff"
	"github.com/Javier162380/busquets/internal/retrier"
	"github.com/Javier162380/busquets/services/busquets/dto"
)

// shouldCreateVersion reports whether newContent differs from the latest
// snapshot. A missing snapshot is a create: GetLatestVersionNumber returns 0
// when plan_versions has no row, and that number must not be treated as a match.
func shouldCreateVersion(latestFound bool, latestContent, newContent string) bool {
	if !latestFound {
		return true
	}
	return latestContent != newContent
}

func (s *Service) versionContentChanged(ctx context.Context, planID int64, content string) (bool, error) {
	lastVersionNum, err := s.db.GetLatestVersionNumber(ctx, planID)
	if err != nil {
		return false, fmt.Errorf("failed to get latest version number: %w", err)
	}
	lastVersion, err := s.db.GetPlanVersionByNumber(ctx, planID, lastVersionNum)
	switch {
	case dto.IsNotFound(err):
		return true, nil
	case err != nil:
		return false, fmt.Errorf("failed to get latest version: %w", err)
	default:
		return shouldCreateVersion(true, lastVersion.Content, content), nil
	}
}

// optionalVersionWriter returns the version-file callback when content differs
// from the latest snapshot. A nil callback tells UpdatePlanContent to skip the
// snapshot while still updating the plan files and row.
func (s *Service) optionalVersionWriter(ctx context.Context, planID int64, content string, createdAt time.Time, wordCount int) (func(planID, versionNumber int64) (string, int64, error), bool, error) {
	create, err := s.versionContentChanged(ctx, planID, content)
	if err != nil || !create {
		return nil, false, err
	}
	return s.writeVersionFile(createdAt, content, wordCount), true, nil
}

// SavePlanVersion creates a new version snapshot of a plan.
func (s *Service) SavePlanVersion(ctx context.Context, planName, syncSource, content string) error {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	createVersion, err := s.versionContentChanged(ctx, plan.ID, content)
	if err != nil {
		return err
	}
	if !createVersion {
		return domain.ErrNoContentChanges
	}

	versionsDir := s.versionsDirFor(plan.ID)
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(versionsDir, 0o750); err != nil {
			return fmt.Errorf("failed to create versions directory: %w", err)
		}
	}

	now := s.nowProvider.Now()
	timestamp := strconv.FormatInt(now.Unix(), 10)

	lastVersionNum, err := s.db.GetLatestVersionNumber(ctx, plan.ID)
	if err != nil {
		return fmt.Errorf("failed to get latest version number: %w", err)
	}
	nextVersionNum := lastVersionNum + 1

	versionFileName := fmt.Sprintf("%d-%s.md", nextVersionNum, timestamp)
	versionFilePath := filepath.Join(versionsDir, versionFileName)

	wordCount := CountWords(content)
	err = s.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
		PlanID:        plan.ID,
		VersionNumber: nextVersionNum,
		FilePath:      versionFilePath,
		Content:       content,
		WordCount:     int64(wordCount),
		CreatedAt:     now,
	}, func() error {
		if err = os.WriteFile(versionFilePath, []byte(content), 0o600); err != nil {
			r := retrier.NewRetrier(3, 1*time.Second)
			deleteErr := r.Do(ctx, func(ctx context.Context) error {
				removeErr := os.Remove(versionFilePath)
				switch {
				case removeErr == nil:
					return nil
				case os.IsNotExist(removeErr):
					return retrier.ErrNonRetriableError
				default:
					return removeErr
				}
			})
			if deleteErr != nil {
				//nolint:errorlint // Need to include cleanup error as context
				return fmt.Errorf("failed to write version file: %w (also failed to clean up file: %v)", err, deleteErr)
			}
			return fmt.Errorf("failed to write version file: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to save version to database: %w", err)
	}

	return nil
}

// GetPlanVersionHistory retrieves version history for a plan with pagination.
func (s *Service) GetPlanVersionHistory(ctx context.Context, planName, syncSource string, offset, limit int64) ([]PlanVersionDetail, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	versions, err := s.db.GetPlanVersionHistory(ctx, dto.VersionHistoryParams{
		PlanID: plan.ID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get version history: %w", err)
	}

	tags, err := s.db.GetPlanTags(ctx, plan.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan tags: %w", err)
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)

	planVersionsDetail := make([]PlanVersionDetail, len(versions))
	for i, version := range versions {
		planVersionsDetail[i] = PlanVersionDetail{
			PlanVersion: toPlanVersion(version),
			ReadingTime: s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM),
			Tags:        toTags(tags),
		}
	}

	return planVersionsDetail, nil
}

// GetPlanVersion retrieves a specific version of a plan.
func (s *Service) GetPlanVersion(ctx context.Context, planName, syncSource string, versionNumber int64) (*PlanVersionDetail, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	return s.planVersionDetail(ctx, plan.ID, versionNumber)
}

// planVersionDetail loads one version with its reading time and the plan's tags.
func (s *Service) planVersionDetail(ctx context.Context, planID, versionNumber int64) (*PlanVersionDetail, error) {
	version, err := s.db.GetPlanVersionByNumber(ctx, planID, versionNumber)
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}

	tags, err := s.db.GetPlanTags(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan tags: %w", err)
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)
	readingTime := s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM)

	return &PlanVersionDetail{
		PlanVersion: toPlanVersion(version),
		ReadingTime: readingTime,
		Tags:        toTags(tags),
	}, nil
}

// SetVersionMilestones replaces a version's milestone set; an empty set clears it.
func (s *Service) SetVersionMilestones(ctx context.Context, planID, versionNumber int64, milestones []VersionMilestone) (*PlanVersionDetail, error) {
	validated, err := ValidateVersionMilestones(milestoneStrings(milestones))
	if err != nil {
		return nil, err
	}

	if err := s.db.SetPlanVersionMilestones(ctx, dto.SetPlanVersionMilestonesParams{
		PlanID:        planID,
		VersionNumber: versionNumber,
		Milestones:    milestoneStrings(validated),
	}); err != nil {
		return nil, fmt.Errorf("failed to set milestones on version %d: %w", versionNumber, err)
	}

	return s.planVersionDetail(ctx, planID, versionNumber)
}

// SetVersionMilestonesByFileName resolves the plan by fileName+syncSource, then calls SetVersionMilestones.
func (s *Service) SetVersionMilestonesByFileName(ctx context.Context, fileName, syncSource string, versionNumber int64, milestones []VersionMilestone) (*PlanVersionDetail, error) {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}
	return s.SetVersionMilestones(ctx, plan.ID, versionNumber, milestones)
}

// DiffPlanVersions returns a unified diff of fromVersion against toVersion,
// plus both versions' metadata.
func (s *Service) DiffPlanVersions(ctx context.Context, fileName, syncSource string, fromVersion, toVersion int64) (VersionDiff, error) {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return VersionDiff{}, fmt.Errorf("plan not found: %w", err)
	}

	from, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, fromVersion)
	if err != nil {
		return VersionDiff{}, fmt.Errorf("version %d not found: %w", fromVersion, err)
	}
	to, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, toVersion)
	if err != nil {
		return VersionDiff{}, fmt.Errorf("version %d not found: %w", toVersion, err)
	}

	diff, err := gitdiff.Diff(from.Content, to.Content,
		fmt.Sprintf("Version %d", from.VersionNumber), fmt.Sprintf("Version %d", to.VersionNumber))
	if err != nil {
		return VersionDiff{}, fmt.Errorf("failed to build diff: %w", err)
	}

	return VersionDiff{Diff: diff, From: toPlanVersion(from), To: toPlanVersion(to)}, nil
}

// RestorePlanVersion restores a plan to a previous version: writes that
// version's content back to the source and viewer files, updates the plan
// row, and records the restore itself as a new version — atomically via
// UpdatePlanContent, same as UpdatePlan/SavePlanLocal. A body that already
// matches the latest snapshot updates the files and row without a new version.
func (s *Service) RestorePlanVersion(ctx context.Context, planID, versionNumber int64) error {
	plan, err := s.db.GetPlanByID(ctx, planID)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	version, err := s.db.GetPlanVersionByNumber(ctx, plan.ID, versionNumber)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	sourcePath := filepath.Join(plan.SyncSource, plan.FileName)
	viewerPath := plan.FilePath

	title := extractTitle(version.Content)
	wordCount := CountWords(version.Content)

	contentToStore := version.Content
	if !s.indexFullContent {
		contentToStore = title
	}

	now := s.nowProvider.Now()

	writeVersion, _, err := s.optionalVersionWriter(ctx, plan.ID, version.Content, now, wordCount)
	if err != nil {
		return err
	}

	_, err = s.db.UpdatePlanContent(ctx, dto.UpdatePlanContentParams{
		Plan: dto.UpdatePlanParams{
			FileName:   plan.FileName,
			SyncSource: plan.SyncSource,
			Title:      title,
			Content:    contentToStore,
			IndexedAt:  now,
			WordCount:  int64(wordCount),
		},
		VersionContent:   version.Content,
		VersionCreatedAt: now,
	}, func() (time.Time, int64, error) {
		if err := os.WriteFile(sourcePath, []byte(version.Content), 0o600); err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to write source file: %w", err)
		}
		if err := os.WriteFile(viewerPath, []byte(version.Content), 0o600); err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to write viewer file: %w", err)
		}
		info, err := os.Stat(viewerPath)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("failed to stat restored file: %w", err)
		}
		return info.ModTime(), info.Size(), nil
	}, writeVersion)
	if err != nil {
		return fmt.Errorf("failed to restore plan: %w", err)
	}

	return nil
}

// RestorePlanVersionByFileName resolves a plan by fileName+syncSource and restores it to the
// given version — same as RestorePlanVersion, but callers (like the MCP layer) don't need to
// already know the plan's internal ID.
func (s *Service) RestorePlanVersionByFileName(ctx context.Context, fileName, syncSource string, versionNumber int64) error {
	plan, err := s.db.GetPlanByFileName(ctx, fileName, syncSource)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}
	return s.RestorePlanVersion(ctx, plan.ID, versionNumber)
}

// GetVersionCount returns the total number of versions for a plan.
func (s *Service) GetVersionCount(ctx context.Context, planName, syncSource string) (int64, error) {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return 0, fmt.Errorf("plan not found: %w", err)
	}

	count, err := s.db.GetVersionCount(ctx, plan.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get version count: %w", err)
	}

	return count, nil
}

// CleanupOldVersions deletes versions beyond the limit (keeping only the most recent ones).
func (s *Service) CleanupOldVersions(ctx context.Context, planName, syncSource string, maxVersions int64) error {
	plan, err := s.db.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	versions, err := s.db.GetPlanVersionHistory(ctx, dto.VersionHistoryParams{
		PlanID: plan.ID,
		Limit:  1000,
		Offset: 0,
	})
	if err != nil {
		return fmt.Errorf("failed to get versions for cleanup: %w", err)
	}

	if int64(len(versions)) > maxVersions {
		versionsToDelete := versions[maxVersions:]

		for _, version := range versionsToDelete {
			if err := os.Remove(version.FilePath); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("failed to delete old version file", "path", version.FilePath, "error", err)
			}
		}

		if len(versionsToDelete) > 0 {
			cutoffVersionNum := versionsToDelete[0].VersionNumber
			err = s.db.DeleteVersionsOlderThan(ctx, dto.DeleteVersionsParams{
				PlanID:        plan.ID,
				VersionNumber: cutoffVersionNum,
			})
			if err != nil {
				return fmt.Errorf("failed to delete old versions from database: %w", err)
			}
		}
	}

	return nil
}

// SearchVersions searches across all versions of a plan for matching content.
func (s *Service) SearchVersions(ctx context.Context, planName, syncSource, query string) ([]PlanVersionDetail, error) {
	plan, err := s.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	versions, err := s.db.SearchVersionsByContent(ctx, dto.SearchVersionsParams{
		PlanID: plan.ID,
		Query:  query,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	readingSpeedWPM := s.GetReadingSpeedForDisplay(ctx)

	planVersionsDetail := make([]PlanVersionDetail, len(versions))
	for i, version := range versions {
		planVersionsDetail[i] = PlanVersionDetail{
			PlanVersion: toPlanVersion(version),
			ReadingTime: s.CalculateReadingTimeWithWPM(int(version.WordCount), readingSpeedWPM),
			Tags:        toTags(plan.Tags),
		}
	}

	return planVersionsDetail, nil
}

// VersionMilestone is a lifecycle stamp on a plan version. Asserted by a user or agent; never derived.
type VersionMilestone string

const (
	MilestoneLLMGenerated             VersionMilestone = "LLMGenerated"
	MilestoneHumanReview              VersionMilestone = "HumanReview"
	MilestonePlanImplementationStart  VersionMilestone = "PlanImplementationStart"
	MilestonePlanImplementationFinish VersionMilestone = "PlanImplementationFinish"
	MilestonePlanAbandoned            VersionMilestone = "PlanAbandoned"
)

// VersionMilestones lists every milestone in lifecycle order; storage and display follow it.
var VersionMilestones = []VersionMilestone{
	MilestoneLLMGenerated,
	MilestoneHumanReview,
	MilestonePlanImplementationStart,
	MilestonePlanImplementationFinish,
	MilestonePlanAbandoned,
}

// conflictingMilestones lists pairs that can never share a version.
var conflictingMilestones = [][2]VersionMilestone{
	{MilestoneLLMGenerated, MilestoneHumanReview},
	{MilestonePlanImplementationFinish, MilestonePlanAbandoned},
}

// ValidateVersionMilestones is the write gate: known values, no conflicting pair.
func ValidateVersionMilestones(raw []string) ([]VersionMilestone, error) {
	parsed, err := ParseVersionMilestones(raw)
	if err != nil {
		return nil, err
	}
	for _, pair := range conflictingMilestones {
		if slices.Contains(parsed, pair[0]) && slices.Contains(parsed, pair[1]) {
			return nil, &dto.Error{
				Category: dto.CategoryValidation,
				Op:       "ValidateVersionMilestones",
				Message:  fmt.Sprintf("%s and %s cannot be on the same version", pair[0], pair[1]),
			}
		}
	}
	return parsed, nil
}

// ParseVersionMilestones rejects unknown values; returns the rest deduplicated in lifecycle order.
func ParseVersionMilestones(raw []string) ([]VersionMilestone, error) {
	for _, value := range raw {
		if !slices.Contains(VersionMilestones, VersionMilestone(value)) {
			return nil, &dto.Error{
				Category: dto.CategoryValidation,
				Op:       "ParseVersionMilestones",
				Message:  fmt.Sprintf("unknown milestone %q", value),
			}
		}
	}
	return storedMilestones(raw), nil
}

// storedMilestones filters the canonical list by raw: dedupe and lifecycle order for free, and unknown
// values are skipped so a stored row always loads. nil when none.
func storedMilestones(raw []string) []VersionMilestone {
	if len(raw) == 0 {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(VersionMilestones), func(milestone VersionMilestone) bool {
		return !slices.Contains(raw, string(milestone))
	})
}

func milestoneStrings(milestones []VersionMilestone) []string {
	raw := make([]string, len(milestones))
	for i, milestone := range milestones {
		raw[i] = string(milestone)
	}
	return raw
}
