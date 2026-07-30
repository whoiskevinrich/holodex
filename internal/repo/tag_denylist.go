package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
)

// DeniedTag is one globally-blocked term (F50, ADR-075 D2) -- a term dimension
// only, no entity or provider (contrast entity_keep_separate/enrichment_dismissals).
type DeniedTag struct {
	Term      string `json:"term"`
	CreatedAt string `json:"created_at"`
}

// ErrTagDenied is returned by resolveOrCreateByName when name matches a denied
// term (ADR-075 D2). Every tag-creation path (scanner, manual attach,
// materialization) routes through resolveOrCreateByName, so gating here is the
// single choke point -- a denied term can never become a tags row from any
// origin, by construction rather than by three independently-maintained checks.
var ErrTagDenied = errors.New("tag: term is denied")

// isTagDeniedQuery is built once at init, like identityQueryByType (identity.go)
// -- resolveOrCreateByName calls isTagDenied on every scanned tag, so this avoids
// re-running nameKeyExpr's fmt.Sprintf on that hot path for a string that never
// varies (entityType is always model.EntityTag here).
var isTagDeniedQuery = `SELECT 1 FROM denied_tags WHERE term_key = ` + nameKeyExpr(model.EntityTag, "?")

// isTagDenied reports whether name's fold matches an existing denied_tags row,
// using the identical nameKeyExpr fold tags themselves resolve by -- so
// "GNOME" matches a denial of "gnome" but "Garden Gnome" does not. qr is
// queryRower (identity.go) so the same check runs inside
// resolveOrCreateByName's transaction (*sql.Tx) and, via the exported
// IsTagDenied below, standalone (*sql.Db) for genre writeback.
func isTagDenied(ctx context.Context, qr queryRower, name string) (bool, error) {
	var x int
	err := qr.QueryRowContext(ctx, isTagDeniedQuery, name).Scan(&x)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("check tag deny-list: %w", err)
	}
}

// IsTagDenied reports whether name matches an existing deny-list entry —
// exported for genre writeback (F50 P0-10, ADR-075 RD9), which must filter
// the raw resolved `genres` union through the deny-list the same way
// resolveOrCreateByName gates new tags, but outside any write transaction.
func (r *Repo) IsTagDenied(ctx context.Context, name string) (bool, error) {
	return isTagDenied(ctx, r.db, name)
}

// ListDeniedTags returns every denied term, newest first (the owner's
// /owner/tags Deny-list tab).
func (r *Repo) ListDeniedTags(ctx context.Context) ([]DeniedTag, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT term, created_at FROM denied_tags ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list denied tags: %w", err)
	}
	defer rows.Close()

	var out []DeniedTag
	for rows.Next() {
		var d DeniedTag
		if err := rows.Scan(&d.Term, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan denied tag: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DenyTag adds term to the global tag deny-list. Idempotent: re-denying an
// already-denied term (case/whitespace variants included) is a no-op.
func (r *Repo) DenyTag(ctx context.Context, term string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO denied_tags (term_key, term, created_at) VALUES (`+nameKeyExpr(model.EntityTag, "?")+`, ?, ?)`,
		term, term, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("deny tag: %w", err)
	}
	return nil
}

// RemoveDeniedTag removes term from the deny-list, matched by the same fold
// DenyTag stores by. Returns ErrNotFound if term isn't currently denied.
func (r *Repo) RemoveDeniedTag(ctx context.Context, term string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx,
		`DELETE FROM denied_tags WHERE term_key = `+nameKeyExpr(model.EntityTag, "?"), term)
	if err != nil {
		return fmt.Errorf("remove denied tag: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
