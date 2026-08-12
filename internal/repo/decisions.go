package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/fieldsource"
)

// DecisionRow is one stored standing decision for a single field of an entity
// (F36, ADR-051). Source is "file" | "provider:<name>" | "manual"; ManualValue is
// the frozen literal, populated only when Source == "manual".
type DecisionRow struct {
	FieldKey    string
	Source      string
	ManualValue string
}

// DecisionsForEntity returns all standing field-source decisions for one entity,
// ordered by field for stable output (F36). The resolver pre-loads this map and
// consults it before mapping order — no per-field query, resolution stays pure.
func (r *Repo) DecisionsForEntity(ctx context.Context, entityType string, entityID int64) ([]DecisionRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT field_key, source, manual_value
		FROM field_source_decisions
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY field_key`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("decisions for entity: %w", err)
	}
	defer rows.Close()

	var out []DecisionRow
	for rows.Next() {
		var dr DecisionRow
		if err := rows.Scan(&dr.FieldKey, &dr.Source, &dr.ManualValue); err != nil {
			return nil, err
		}
		out = append(out, dr)
	}
	return out, rows.Err()
}

// DecisionsForVideos returns decisions for a batch of video IDs grouped by id — a
// single query so list pages avoid N+1 (mirrors CurationForVideos). Missing keys
// mean no decision (the file-first default). Used by the browse-title path so a
// decided title drives the card just as it drives the detail view.
func (r *Repo) DecisionsForVideos(ctx context.Context, ids []int64) (map[int64][]DecisionRow, error) {
	return r.DecisionsForEntities(ctx, "video", ids)
}

// DecisionsForEntities is DecisionsForVideos generalized to any entity type — the
// F55 list-wide completeness resolve (ADR-081 D4) needs the same batch shape for
// person/studio.
func (r *Repo) DecisionsForEntities(ctx context.Context, entityType string, ids []int64) (map[int64][]DecisionRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := append([]any{entityType}, toAnySlice(ids)...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT entity_id, field_key, source, manual_value
		FROM field_source_decisions
		WHERE entity_type = ? AND entity_id IN (`+placeholders(len(ids))+`)
		ORDER BY entity_id, field_key`, args...)
	if err != nil {
		return nil, fmt.Errorf("decisions for entities: %w", err)
	}
	defer rows.Close()

	out := make(map[int64][]DecisionRow, len(ids))
	for rows.Next() {
		var (
			eid int64
			dr  DecisionRow
		)
		if err := rows.Scan(&eid, &dr.FieldKey, &dr.Source, &dr.ManualValue); err != nil {
			return nil, err
		}
		out[eid] = append(out[eid], dr)
	}
	return out, rows.Err()
}

// SetDecision records one standing decision (upsert by the unique entity+field key).
// Setting or clearing a decision is DB-only — no file is ever touched (RD5); the file
// changes solely via the explicit "Write decisions to file" action. manualValue is
// stored only when source == "manual"; for file/provider it is forced empty so a
// later source change can't leave a stale literal behind.
func (r *Repo) SetDecision(ctx context.Context, entityType string, entityID int64, fieldKey, source, manualValue string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return r.setDecisionLocked(ctx, entityType, entityID, fieldKey, source, manualValue)
}

// SetDecisionChecked runs check() and, absent a collision, the decision write, as one
// writeMu-locked operation — the atomicity every composite-key collision gate needs:
// a manual title rename or a Studio chip/search/create pick both change the video's
// composite key, so two concurrent edits to the same field must not both pass their
// collision check before either commits (HOLODEX-270/271). check is caller-supplied
// so this stays entity/field-agnostic: Title's check (FindTitleCollision) is fully
// self-contained in this package, while Studio's needs a resolver pass that lives in
// internal/api (ADR-051 layering) — passing it in as a closure either way keeps
// writeMu and the locked write path private to repo instead of exposing them to
// outside callers. A nil check skips straight to the write (the override path, which
// must commit regardless of what a collision check would report).
func (r *Repo) SetDecisionChecked(ctx context.Context, entityType string, entityID int64, fieldKey, source, manualValue string, check func() (*VideoCollision, error)) (*VideoCollision, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if check != nil {
		if collision, err := check(); err != nil || collision != nil {
			return collision, err
		}
	}
	return nil, r.setDecisionLocked(ctx, entityType, entityID, fieldKey, source, manualValue)
}

// setDecisionLocked is SetDecision's implementation, assuming the caller already
// holds writeMu — shared by SetDecision and SetDecisionChecked.
func (r *Repo) setDecisionLocked(ctx context.Context, entityType string, entityID int64, fieldKey, source, manualValue string) error {
	if source != fieldsource.Manual {
		manualValue = ""
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO field_source_decisions (entity_type, entity_id, field_key, source, manual_value, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_id, field_key) DO UPDATE SET
			source       = excluded.source,
			manual_value = excluded.manual_value`,
		entityType, entityID, fieldKey, source, manualValue,
		time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("set decision: %w", err)
	}
	return nil
}

// HasManualSource reports whether entityID's fieldKey currently carries a
// manual: decision (F36) — F48.3e's one-time-import rule: extraction never
// auto-applies over an owner's manual override, regardless of score.
func (r *Repo) HasManualSource(ctx context.Context, entityType string, entityID int64, fieldKey string) (bool, error) {
	var exists int
	switch err := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM field_source_decisions
		WHERE entity_type = ? AND entity_id = ? AND field_key = ? AND source = ?
		LIMIT 1`, entityType, entityID, fieldKey, fieldsource.Manual).Scan(&exists); {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("has manual source: %w", err)
	}
}

// ClearDecision removes the standing decision for a field, reverting it to the
// file-first default (F36). A clear of a non-existent decision is an idempotent
// no-op success. Returns rows removed.
func (r *Repo) ClearDecision(ctx context.Context, entityType string, entityID int64, fieldKey string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM field_source_decisions
		WHERE entity_type = ? AND entity_id = ? AND field_key = ?`,
		entityType, entityID, fieldKey)
	if err != nil {
		return 0, fmt.Errorf("clear decision: %w", err)
	}
	return res.RowsAffected()
}
