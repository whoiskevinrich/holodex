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
