package repo

import (
	"context"
	"fmt"
	"time"
)

// PromotionRow is one stored in-app field promotion (F44, ADR-062): an owner-authored
// presentation override for a single non-canonical field key of an entity *type*. It
// carries presentation only (label / render / group / order) — the field's F36/F30
// candidate sources are derived at resolve time from shadow provenance, never stored.
// Empty presentation columns inherit from the lower tiers (provider hint → title-case).
type PromotionRow struct {
	FieldKey string
	Label    string
	Render   string
	Group    string
	Order    int
}

// PromotionsForEntityType returns all field promotions for one entity type, ordered by
// field for stable output (F44). Each entity resolve pre-loads this small set and
// materializes it into the []mapping.Field before ResolveFields runs — no per-field
// query, resolution stays pure (ADR-062). A missing key means the field is not promoted
// (it stays F39 auto-registered).
//
// Served from the atomic-pointer cache (HOLODEX-172, mirrors enrich.Service.FieldHints):
// lazily loaded on first call and invalidated by SetPromotion/ClearPromotion, the only
// writers, so the visitor detail-page read path never queries field_promotions.
func (r *Repo) PromotionsForEntityType(ctx context.Context, entityType string) ([]PromotionRow, error) {
	all, err := r.promotionsCache(ctx)
	if err != nil {
		return nil, err
	}
	return all[entityType], nil
}

// promotionsCache returns the cached entity-type → promotions map, loading it on first
// use.
func (r *Repo) promotionsCache(ctx context.Context) (map[string][]PromotionRow, error) {
	if p := r.promotions.Load(); p != nil {
		return *p, nil
	}
	return r.reloadPromotions(ctx)
}

// reloadPromotions reads every field_promotions row and swaps the grouped map into the
// cache atomically.
func (r *Repo) reloadPromotions(ctx context.Context) (map[string][]PromotionRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT entity_type, field_key, label, render, hint_group, ord
		FROM field_promotions
		ORDER BY entity_type, field_key`)
	if err != nil {
		return nil, fmt.Errorf("promotions for entity type: %w", err)
	}
	defer rows.Close()

	out := map[string][]PromotionRow{}
	for rows.Next() {
		var entityType string
		var pr PromotionRow
		if err := rows.Scan(&entityType, &pr.FieldKey, &pr.Label, &pr.Render, &pr.Group, &pr.Order); err != nil {
			return nil, err
		}
		out[entityType] = append(out[entityType], pr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.promotions.Store(&out)
	return out, nil
}

// SetPromotion records one promotion (upsert by the entity_type+field_key primary key).
// Promoting or editing a promotion is DB-only — no file and no enrichment value is ever
// touched (D2/D-reversible); the field's shadow value is untouched. The caller has
// already sanitized label and coerced render/group to the F39 vocabulary and validated
// the key is non-canonical (ADR-062 Mechanism 5). created_at is preserved across an
// update; updated_at is stamped every write.
func (r *Repo) SetPromotion(ctx context.Context, entityType, fieldKey, label, render, group string, order int) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	now := time.Now().UTC().Format(timeLayout)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO field_promotions (entity_type, field_key, label, render, hint_group, ord, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, field_key) DO UPDATE SET
			label      = excluded.label,
			render     = excluded.render,
			hint_group = excluded.hint_group,
			ord        = excluded.ord,
			updated_at = excluded.updated_at`,
		entityType, fieldKey, label, render, group, order, now, now)
	if err != nil {
		return fmt.Errorf("set promotion: %w", err)
	}
	r.promotions.Store(nil) // invalidate: next read reloads from the table
	return nil
}

// ClearPromotion removes a promotion, reverting the field to its F39 auto-registered,
// display-only state (D-reversible). The underlying shadow value and any prior
// field_source_decisions / metadata_curation rows are untouched (they are keyed by
// field_key, independent of the promotion row) and re-apply on re-promotion. A clear of
// a non-existent promotion is an idempotent no-op success. Returns rows removed.
func (r *Repo) ClearPromotion(ctx context.Context, entityType, fieldKey string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM field_promotions
		WHERE entity_type = ? AND field_key = ?`,
		entityType, fieldKey)
	if err != nil {
		return 0, fmt.Errorf("clear promotion: %w", err)
	}
	r.promotions.Store(nil) // invalidate: next read reloads from the table
	return res.RowsAffected()
}
