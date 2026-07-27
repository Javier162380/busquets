package claudeviewer

import (
	"context"
	"os"
	"path/filepath"
)

// MigrateStorageLayout moves every plan's mirror file and version files to
// the id-keyed layout (viewerDir/plans/<id>/... instead of the old
// viewerDir/<slugified-label>/<fileName> and viewerDir/versions/<fileName>/
// schemes). Idempotent — skips anything already on the new layout — and safe
// to call on every startup. Copies to the new location and removes the old
// file once the copy succeeds, falling back to regenerating from the DB's
// content column when the old on-disk file is already missing — recovering
// gracefully from the cross-source collisions this migration exists to
// retire. Returns the number of plans whose mirror file was migrated.
func (s *Service) MigrateStorageLayout(ctx context.Context) (int, error) {
	plans, err := s.db.ListAllPlansFull(ctx)
	if err != nil {
		return 0, err
	}

	migrated := 0
	for _, p := range plans {
		newPath := s.mirrorPathFor(p.ID, p.FileName)
		if p.FilePath != newPath {
			writeFile := func() error {
				if err := os.MkdirAll(filepath.Dir(newPath), 0o750); err != nil {
					return err
				}
				return migrateFile(p.FilePath, newPath, p.Content)
			}
			if err := s.db.UpdatePlanFilePath(ctx, p.ID, newPath, writeFile); err != nil {
				return migrated, err
			}
			migrated++
		}

		// Versions are checked independently of the mirror above (not skipped
		// just because the mirror was already migrated) so an interrupted prior
		// run — mirror migrated and committed, process died before its versions
		// were — still finishes the job on the next call.
		versions, err := s.db.ListPlanVersionsAll(ctx, p.ID)
		if err != nil {
			return migrated, err
		}
		for _, v := range versions {
			newVPath := filepath.Join(s.versionsDirFor(p.ID), filepath.Base(v.FilePath))
			if v.FilePath == newVPath {
				continue
			}
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
	}
	return migrated, nil
}

// migrateFile copies src to dst and then removes src, so the old on-disk
// layout doesn't linger as duplicate data once a plan is fully on the new
// one. If src is already gone (lost to a historical cross-source collision,
// or already consumed by another row's migration step when both historically
// shared the same old path), it regenerates dst from content instead — the
// DB is authoritative, the on-disk file was always just a derived mirror.
// Guards src == dst so an already-migrated path can never be deleted out
// from under itself.
func migrateFile(src, dst, content string) error {
	if src == dst {
		return nil
	}
	//nolint:gosec // G304: src is a stored DB path, not user input
	if data, err := os.ReadFile(src); err == nil {
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			return err
		}
		if err := os.Remove(src); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(dst, []byte(content), 0o600)
}
