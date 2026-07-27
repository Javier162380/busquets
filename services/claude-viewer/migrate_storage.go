package claudeviewer

import (
	"context"
	"os"
	"path/filepath"
)

// MigrateStorageLayout moves every plan's mirror file and version files to
// the id-keyed layout (viewerDir/plans/<id>/... instead of the old
// viewerDir/<slugified-label>/<fileName> and viewerDir/versions/<fileName>/
// schemes). Idempotent — skips any plan already on the new layout — and
// safe to call on every startup. Copies (never moves/deletes) from the old
// location, and falls back to the DB's content column when the old on-disk
// file is missing — recovering gracefully from the cross-source collisions
// this migration exists to retire. Returns the number of plans migrated.
func (s *Service) MigrateStorageLayout(ctx context.Context) (int, error) {
	plans, err := s.db.ListAllPlansFull(ctx)
	if err != nil {
		return 0, err
	}

	migrated := 0
	for _, p := range plans {
		newPath := s.mirrorPathFor(p.ID, p.FileName)
		if p.FilePath == newPath {
			continue
		}

		writeFile := func() error {
			if err := os.MkdirAll(filepath.Dir(newPath), 0o750); err != nil {
				return err
			}
			return migrateFile(p.FilePath, newPath, p.Content)
		}
		if err := s.db.UpdatePlanFilePath(ctx, p.ID, newPath, writeFile); err != nil {
			return migrated, err
		}

		versions, err := s.db.ListPlanVersionsAll(ctx, p.ID)
		if err != nil {
			return migrated, err
		}
		for _, v := range versions {
			newVPath := filepath.Join(s.versionsDirFor(p.ID), filepath.Base(v.FilePath))
			writeVersionFile := func() error {
				if err := os.MkdirAll(filepath.Dir(newVPath), 0o750); err != nil {
					return err
				}
				return migrateFile(v.FilePath, newVPath, v.Content)
			}
			if err := s.db.UpdatePlanVersionFilePath(ctx, v.ID, newVPath, writeVersionFile); err != nil {
				return migrated, err
			}
		}
		migrated++
	}
	return migrated, nil
}

// migrateFile copies src to dst if src still exists; otherwise regenerates
// dst from content (the DB is authoritative — the on-disk file was always
// a derived mirror, so a missing/clobbered old file is recoverable).
func migrateFile(src, dst, content string) error {
	//nolint:gosec // G304: src is a stored DB path, not user input
	if data, err := os.ReadFile(src); err == nil {
		return os.WriteFile(dst, data, 0o600)
	}
	return os.WriteFile(dst, []byte(content), 0o600)
}
