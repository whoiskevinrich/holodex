package personimage

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"holodex/internal/repo"
)

// BackfillRepo is the repo subset the one-time content-hash backfill needs
// (satisfied by *repo.Repo). Kept an interface so the pass is testable without a DB.
type BackfillRepo interface {
	PersonImagesMissingHash(ctx context.Context) ([]repo.PersonImageRef, error)
	SetPersonImageHash(ctx context.Context, id int64, hash string) error
	CollapseDuplicateGalleryExtras(ctx context.Context) ([]repo.PersonImageRef, error)
}

// Backfill is the one-time F34/ADR-050 startup pass that hashes pre-existing person
// images and collapses already-duplicated gallery extras. It is idempotent: once
// every row has a content_hash and every duplicate extra is gone it does nothing, so
// it is safe to run on every boot. SQL can't sha256 on-disk bytes, so this lives in
// Go (the established repair-pass pattern, like PruneJobRuns at startup).
//
// Steps:
//  1. For each row missing a hash, read its stored JPEG and record the hash. A
//     missing/unreadable file logs and is skipped (it stays unhashed and so never
//     matches — retried next boot), rather than aborting the whole pass.
//  2. Collapse duplicate gallery extras (keep earliest; an extra matching a core
//     image yields to the core), deleting the redundant rows' on-disk files.
//
// Returns (hashed, removed) counts for the startup log. Best-effort: a per-row error
// is logged and the pass continues.
func Backfill(ctx context.Context, r BackfillRepo, dir string, log *slog.Logger) (hashed, removed int, err error) {
	missing, err := r.PersonImagesMissingHash(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list unhashed images: %w", err)
	}
	for _, ref := range missing {
		data, err := os.ReadFile(ImagePath(dir, ref.PersonID, ref.ID))
		if err != nil {
			// File gone (row outlived its bytes, or an interrupted write): leave the row
			// unhashed so a later boot can retry once the file exists; it can't match
			// anything with an empty hash, so dedup stays correct meanwhile.
			log.Warn("person image backfill: read failed", "person", ref.PersonID, "image", ref.ID, "err", err)
			continue
		}
		if err := r.SetPersonImageHash(ctx, ref.ID, Hash(data)); err != nil {
			log.Warn("person image backfill: hash write failed", "image", ref.ID, "err", err)
			continue
		}
		hashed++
	}

	victims, err := r.CollapseDuplicateGalleryExtras(ctx)
	if err != nil {
		// The hashing above still committed; report the collapse failure but don't undo it.
		return hashed, 0, fmt.Errorf("collapse duplicate gallery extras: %w", err)
	}
	// The victims are already removed from the DB (the authoritative dedup) — clean up
	// their now-orphaned files best-effort; a failed unlink doesn't un-collapse the
	// duplicate, so the count reported is rows collapsed, not files unlinked.
	for _, v := range victims {
		if err := Remove(dir, v.PersonID, v.ID); err != nil {
			log.Warn("person image backfill: remove file failed", "person", v.PersonID, "image", v.ID, "err", err)
		}
	}
	return hashed, len(victims), nil
}
