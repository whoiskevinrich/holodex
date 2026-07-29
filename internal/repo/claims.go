package repo

import (
	"context"
	"fmt"
	"time"
)

// ClaimRow is one stored claimed provider key (F49, ADR-074): the statement that a
// canonical field owns a differently-named provider key, so the key contributes its
// value as a candidate source of that field instead of auto-registering as its own
// display-only row. It carries identity only — no precedence position (D3), because
// adding a claim must never move the resolved winner.
type ClaimRow struct {
	Provider  string
	FieldKey  string
	Canonical string
}

// ClaimsForEntityType returns all claims for one entity type, ordered by
// (provider, field_key) — the order they append in, so resolution is reproducible from
// the table's contents rather than from edit history (ADR-074 §D3). Each entity resolve
// pre-loads this small set and materializes it into the []mapping.Field before
// ResolveFields runs, so resolution stays pure. Mirrors PromotionsForEntityType,
// including its atomic-pointer cache: lazily loaded, invalidated by SetClaim/ClearClaim
// (the only writers), so the visitor detail-page read path never queries field_claims.
func (r *Repo) ClaimsForEntityType(ctx context.Context, entityType string) ([]ClaimRow, error) {
	if p := r.claims.Load(); p != nil {
		return (*p)[entityType], nil
	}
	all, err := r.reloadClaims(ctx)
	if err != nil {
		return nil, err
	}
	return all[entityType], nil
}

// reloadClaims reads every field_claims row and swaps the grouped map into the cache
// atomically.
func (r *Repo) reloadClaims(ctx context.Context) (map[string][]ClaimRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT entity_type, provider, field_key, canonical
		FROM field_claims
		ORDER BY entity_type, provider, field_key`)
	if err != nil {
		return nil, fmt.Errorf("claims for entity type: %w", err)
	}
	defer rows.Close()

	out := map[string][]ClaimRow{}
	for rows.Next() {
		var entityType string
		var c ClaimRow
		if err := rows.Scan(&entityType, &c.Provider, &c.FieldKey, &c.Canonical); err != nil {
			return nil, err
		}
		out[entityType] = append(out[entityType], c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.claims.Store(&out)
	return out, nil
}

// SetClaim records one claim (upsert by the entity_type+provider+field_key primary key)
// and, in the same transaction, clears any F44 promotion of that key: a key may be
// promoted or claimed, never both (spec RD3, ADR-074 §D5). One transaction is the point
// — a claim that landed while its promotion survived would leave the key rendering both
// as its own promoted field and as a source of the claim target.
//
// The clear is a real delete, not a suspension: ClearClaim does not bring the promotion
// back. The affordance names the promotion it will destroy before applying (FR5/DD6),
// which is why write-time is the right moment — only there can the owner still decline.
//
// DB-only. No enrichment value and no file is ever touched; the key's shadow rows are
// untouched, and the claim manifests only where the provider actually supplied a value.
func (r *Repo) SetClaim(ctx context.Context, entityType, provider, fieldKey, canonical string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("set claim: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	now := time.Now().UTC().Format(timeLayout)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO field_claims (entity_type, provider, field_key, canonical, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, provider, field_key) DO UPDATE SET
			canonical  = excluded.canonical,
			updated_at = excluded.updated_at`,
		entityType, provider, fieldKey, canonical, now, now); err != nil {
		return fmt.Errorf("set claim: %w", err)
	}
	// RD3: mutually exclusive with promotion. field_promotions is keyed without a
	// provider, so claiming any provider's spelling of the key retires the promotion of
	// that key — which is correct: the promoted row and the claim target would otherwise
	// both render it.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM field_promotions
		WHERE entity_type = ? AND field_key = ?`, entityType, fieldKey); err != nil {
		return fmt.Errorf("set claim (clear promotion): %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("set claim: %w", err)
	}
	r.claims.Store(nil)     // invalidate: next read reloads from the table
	r.promotions.Store(nil) // the promotion clear above invalidates that cache too
	return nil
}

// ClearClaim removes a claim, so the key stops feeding its target field and returns to
// F39 auto-registration as its own display-only row. A clear of a non-existent claim is
// an idempotent no-op success. Returns rows removed.
//
// It does **not** restore a promotion that SetClaim cleared — that clear is a delete,
// and the owner re-promotes if they want it back. Nothing else is undone either: the
// target field's own F36 decisions and F30 curation are keyed by canonical, independent
// of any claim, and are untouched.
func (r *Repo) ClearClaim(ctx context.Context, entityType, provider, fieldKey string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM field_claims
		WHERE entity_type = ? AND provider = ? AND field_key = ?`,
		entityType, provider, fieldKey)
	if err != nil {
		return 0, fmt.Errorf("clear claim: %w", err)
	}
	r.claims.Store(nil) // invalidate: next read reloads from the table
	return res.RowsAffected()
}
