package repo

import (
	"context"
	"fmt"
	"time"
)

// SetFacetNotApplicable marks a canonical facet not-applicable for one entity
// (F55, ADR-081 D2) — an owner-asserted exclusion from that entity's
// completeness score and the remediation queue, independent of whatever
// field_source_decisions says (or doesn't say) for the same field. Idempotent:
// marking an already-excluded facet is a no-op.
func (r *Repo) SetFacetNotApplicable(ctx context.Context, entityType string, entityID int64, canonicalField string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO facet_not_applicable (entity_type, entity_id, canonical_field, created_at)
		VALUES (?, ?, ?, ?)`,
		entityType, entityID, canonicalField, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("set facet not applicable: %w", err)
	}
	return nil
}

// ClearFacetNotApplicable removes the not-applicable exclusion for one
// entity's facet, restoring it to normal scoring. Clearing an unmarked facet
// is an idempotent no-op success. Returns rows removed.
func (r *Repo) ClearFacetNotApplicable(ctx context.Context, entityType string, entityID int64, canonicalField string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM facet_not_applicable
		WHERE entity_type = ? AND entity_id = ? AND canonical_field = ?`,
		entityType, entityID, canonicalField)
	if err != nil {
		return 0, fmt.Errorf("clear facet not applicable: %w", err)
	}
	return res.RowsAffected()
}

// FacetsNotApplicableForEntity returns the set of canonical facets marked
// not-applicable for one entity, keyed for O(1) membership checks by the
// completeness scorer.
func (r *Repo) FacetsNotApplicableForEntity(ctx context.Context, entityType string, entityID int64) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT canonical_field FROM facet_not_applicable
		WHERE entity_type = ? AND entity_id = ?`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("facets not applicable for entity: %w", err)
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var field string
		if err := rows.Scan(&field); err != nil {
			return nil, err
		}
		out[field] = true
	}
	return out, rows.Err()
}
