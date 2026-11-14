package repo

import (
	"context"
	"database/sql"
	"fmt"

	"holodex/internal/model"
)

// FieldState is one cached field of an entity (F90, field-resolution.md "Field presence and
// provider contribution are cached in entity_field_state"): Shown when the entity's page
// displays a value for it, Enriched when a declared provider stored data that maps to it.
// The drain writes a row only when one of the two holds.
type FieldState struct {
	Key      string
	Shown    bool
	Enriched bool
}

// cachesFieldState reports whether the drain keeps entity_field_state for an entity type.
func cachesFieldState(entityType string) bool {
	switch entityType {
	case model.EnrichEntityVideo, model.EnrichEntityPerson, model.EnrichEntityStudio:
		return true
	}
	return false
}

// writeFieldState replaces the cached field rows of ids with rows' Fields, in the drain's
// transaction. An id absent from rows (inactive, a person with no active video) is left with
// no rows, so it reads as missing every field.
func writeFieldState(ctx context.Context, tx *sql.Tx, entityType string, ids []int64, rows []CompletenessRow) error {
	args := append([]any{entityType}, toAnySlice(ids)...)
	if _, err := tx.ExecContext(ctx, `DELETE FROM entity_field_state WHERE entity_type = ? AND entity_id IN (`+placeholders(len(ids))+`)`, args...); err != nil {
		return fmt.Errorf("write field state: clear %s: %w", entityType, err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO entity_field_state (entity_type, entity_id, field_key, shown, enriched) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("write field state: prepare: %w", err)
	}
	defer stmt.Close()
	for _, row := range rows {
		for _, f := range row.Fields {
			if !f.Shown && !f.Enriched {
				continue
			}
			if _, err := stmt.ExecContext(ctx, entityType, row.EntityID, f.Key, f.Shown, f.Enriched); err != nil {
				return fmt.Errorf("write field state: insert %s/%d/%s: %w", entityType, row.EntityID, f.Key, err)
			}
		}
	}
	return nil
}

// The People and Studios Provider link filter (F90 P0-3): linked or not linked. Empty is any.
const (
	LinkLinked   = "linked"
	LinkUnlinked = "unlinked"
)

// The enriched sorts (F90 P0-4) on Media, People and Studios.
const (
	SortEnrichedDesc = "enriched_desc"
	SortEnrichedAsc  = "enriched_asc"
)

// linkedClause is "a declared provider has data for this entity itself" (F90 P0-1), the one
// meaning of linked: `matched:any` on Media, the Provider link filter on People and Studios.
// With no declared provider nothing is linked. entityType and idCol are trusted constants.
func linkedClause(entityType, idCol string, declared []string) (string, []any) {
	if len(declared) == 0 {
		return "0", nil
	}
	return "EXISTS (SELECT 1 FROM entity_enrichment ee WHERE ee.entity_type = '" + entityType + "' AND ee.entity_id = " + idCol +
		" AND ee.provider IN (" + placeholders(len(declared)) + "))", toAnySlice(declared)
}

// shownClause is "the entity's page shows a value for field ?" over the drain's cache
// (field-resolution.md, "Reads"): an entity not yet drained reads as missing the field.
func shownClause(entityType, idCol string) string {
	return "EXISTS (SELECT 1 FROM entity_field_state efs WHERE efs.entity_type = '" + entityType + "' AND efs.entity_id = " + idCol +
		" AND efs.field_key = ? AND efs.shown = 1)"
}

// enrichedOrder orders by the enriched count (F90 P0-4): how many fields a declared
// provider contributed to. The caller appends the list's default order as the tiebreak.
func enrichedOrder(entityType, idCol, dir string) string {
	return "(SELECT COUNT(*) FROM entity_field_state efs WHERE efs.entity_type = '" + entityType + "' AND efs.entity_id = " + idCol +
		" AND efs.enriched = 1) " + dir
}
