package busquets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SettingViewerDirMigrated marks that MigrateLegacyFilePathPrefix has
// already reconciled every file_path row against the current viewer dir, so
// New() can skip the full-table scan on every subsequent startup. Internal
// bookkeeping only — not one of the user-facing Setting* constants.
const SettingViewerDirMigrated = "viewer_dir_migrated_v1"

// MigrateLegacyFilePathPrefix reconciles plans.file_path and
// plan_versions.file_path rows that still carry the oldDir prefix after
// config.MigrateLegacyViewerDir has already renamed the directory on disk.
// The physical files were already moved by that single os.Rename; this only
// rewrites the DB's record of where they now live.
//
// Each row's DB update runs through UpdatePlanFilePath/
// UpdatePlanVersionFilePath, so it's transactional per AGENTS.md rule 12 —
// the writeFile callback here confirms the file exists at the new path
// before the row is committed, falling back to regenerating it from the
// row's own Content column (the authoritative source, same fallback
// migrate_storage.go's migrateFile already uses) when it's missing — a row
// already orphaned before this migration ran must not brick startup for
// every command from here on. A row can still never end up pointing at a
// path with nothing behind it: it's guaranteed present, one way or another,
// before the DB update commits.
//
// Gated by the SettingViewerDirMigrated flag so the full-table scan runs
// once, not on every startup — but the scan is naturally idempotent (matches
// zero rows once done) if the flag is ever missing, so it self-heals.
func (s *Service) MigrateLegacyFilePathPrefix(ctx context.Context, oldDir, newDir string) (int, error) {
	if setting, ok, err := s.GetSetting(ctx, SettingViewerDirMigrated); err == nil && ok && setting.GetBooleanValue() {
		return 0, nil
	}

	fixed := 0

	plans, err := s.db.ListAllPlansFull(ctx)
	if err != nil {
		return fixed, err
	}
	for _, p := range plans {
		if strings.HasPrefix(p.FilePath, oldDir) {
			newPath := newDir + strings.TrimPrefix(p.FilePath, oldDir)
			content := p.Content
			ensure := func() error { return ensureFileExists(newPath, content) }
			if err := s.db.UpdatePlanFilePath(ctx, p.ID, newPath, ensure); err != nil {
				return fixed, fmt.Errorf("failed to update file_path for plan %d: %w", p.ID, err)
			}
			fixed++
		}

		versions, err := s.db.ListPlanVersionsAll(ctx, p.ID)
		if err != nil {
			return fixed, err
		}
		for _, v := range versions {
			if !strings.HasPrefix(v.FilePath, oldDir) {
				continue
			}
			newVPath := newDir + strings.TrimPrefix(v.FilePath, oldDir)
			content := v.Content
			ensure := func() error { return ensureFileExists(newVPath, content) }
			if err := s.db.UpdatePlanVersionFilePath(ctx, v.ID, newVPath, ensure); err != nil {
				return fixed, fmt.Errorf("failed to update file_path for plan version %d: %w", v.ID, err)
			}
			fixed++
		}
	}

	migratedFlag := true
	if err := s.SetSetting(ctx, SettingViewerDirMigrated, SettingValues{BooleanValue: &migratedFlag}); err != nil {
		return fixed, fmt.Errorf("failed to record viewer-dir migration completion: %w", err)
	}
	return fixed, nil
}

// ensureFileExists confirms path exists on disk (the common case — the
// os.Rename in config.MigrateLegacyViewerDir already brought it along), and
// regenerates it from content when it doesn't: the row was already orphaned
// before this migration ran, and the DB's content column is the
// authoritative source regardless — same fallback migrate_storage.go's
// migrateFile uses for the same reason.
func ensureFileExists(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}
	return os.WriteFile(path, []byte(content), 0o600)
}
