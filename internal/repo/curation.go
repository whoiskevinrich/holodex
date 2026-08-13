package repo

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Curation action kinds (F30, ADR-048).
const (
	CurationAdd      = "add"      // owner-added manual value
	CurationSuppress = "suppress" // tombstone: hidden + never written
	CurationNoWrite  = "nowrite"  // shown but excluded from file writeback
)

// CurationRow is one stored value-level curation decision for an entity (F30).
type CurationRow struct {
	FieldKey  string
	NormValue string
	Value     string
	Action    string
}

// curationNorm is the dedup/match key. Must match resolver.normKey (trim + lower).
func curationNorm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// CurationForEntity returns all curation rows for one entity, ordered by field then
// value for stable display (F30.2).
func (r *Repo) CurationForEntity(ctx context.Context, entityType string, entityID int64) ([]CurationRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT field_key, norm_value, value, action
		FROM metadata_curation
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY field_key, norm_value`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("curation for entity: %w", err)
	}
	defer rows.Close()

	var out []CurationRow
	for rows.Next() {
		var cr CurationRow
		if err := rows.Scan(&cr.FieldKey, &cr.NormValue, &cr.Value, &cr.Action); err != nil {
			return nil, err
		}
		out = append(out, cr)
	}
	return out, rows.Err()
}

// CurationForVideos returns curation rows for a batch of video IDs, grouped by id —
// a single query so list pages avoid N+1 (mirrors EnrichmentForVideos). Missing keys
// mean no curation.
func (r *Repo) CurationForVideos(ctx context.Context, ids []int64) (map[int64][]CurationRow, error) {
	return r.CurationForEntities(ctx, "video", ids)
}

// CurationForEntities is CurationForVideos generalized to any entity type — the F55
// list-wide completeness resolve (ADR-081 D4) needs the same batch shape for
// person/studio.
func (r *Repo) CurationForEntities(ctx context.Context, entityType string, ids []int64) (map[int64][]CurationRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := append([]any{entityType}, toAnySlice(ids)...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT entity_id, field_key, norm_value, value, action
		FROM metadata_curation
		WHERE entity_type = ? AND entity_id IN (`+placeholders(len(ids))+`)
		ORDER BY entity_id, field_key, norm_value`, args...)
	if err != nil {
		return nil, fmt.Errorf("curation for entities: %w", err)
	}
	defer rows.Close()

	out := make(map[int64][]CurationRow, len(ids))
	for rows.Next() {
		var (
			eid int64
			cr  CurationRow
		)
		if err := rows.Scan(&eid, &cr.FieldKey, &cr.NormValue, &cr.Value, &cr.Action); err != nil {
			return nil, err
		}
		out[eid] = append(out[eid], cr)
	}
	return out, rows.Err()
}

// SetCuration records one value-level decision (idempotent by the unique key). For
// action=add the value's display form is stored; for suppress/nowrite the value is
// the value being acted on (its norm key is what matches at resolution). Returns
// nil for an empty value.
func (r *Repo) SetCuration(ctx context.Context, entityType string, entityID int64, fieldKey, value, action string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return r.setCurationLocked(ctx, entityType, entityID, fieldKey, value, action)
}

// SetCurationChecked runs check() and, absent a collision, the curation write, as one
// writeMu-locked operation — mirrors SetDecisionChecked's atomicity guarantee
// (decisions.go), needed for People (HOLODEX-272): a person-typed field add/suppress
// changes video_people's composite key exactly as a Title/Studio decision changes
// their own dimension, so two concurrent edits must not both pass their collision
// check before either commits. A nil check skips straight to the write (the override
// path, which must commit regardless of what a collision check would report).
func (r *Repo) SetCurationChecked(ctx context.Context, entityType string, entityID int64, fieldKey, value, action string, check func() (*VideoCollision, error)) (*VideoCollision, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if check != nil {
		if collision, err := check(); err != nil || collision != nil {
			return collision, err
		}
	}
	return nil, r.setCurationLocked(ctx, entityType, entityID, fieldKey, value, action)
}

// setCurationLocked is SetCuration's implementation, assuming the caller already
// holds writeMu — shared by SetCuration and SetCurationChecked.
func (r *Repo) setCurationLocked(ctx context.Context, entityType string, entityID int64, fieldKey, value, action string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("curation: empty value")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO metadata_curation (entity_type, entity_id, field_key, norm_value, value, action, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'manual', ?)
		ON CONFLICT(entity_type, entity_id, field_key, norm_value, action) DO UPDATE SET
			value = excluded.value`,
		entityType, entityID, fieldKey, curationNorm(value), value, action,
		time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("set curation: %w", err)
	}
	return nil
}

// ClearCuration removes one decision so the underlying source value is restored
// (F30.2e). Matching is by normalized value + action. Returns rows removed.
func (r *Repo) ClearCuration(ctx context.Context, entityType string, entityID int64, fieldKey, value, action string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM metadata_curation
		WHERE entity_type = ? AND entity_id = ? AND field_key = ? AND norm_value = ? AND action = ?`,
		entityType, entityID, fieldKey, curationNorm(value), action)
	if err != nil {
		return 0, fmt.Errorf("clear curation: %w", err)
	}
	return res.RowsAffected()
}
