package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"holodex/internal/model"
)

// Materialized completeness store (F65, ADR-099 D3/D4). entity_completeness and
// entity_completeness_missing cache resolver.Complete's output per entity so the
// list surfaces can sort, filter and badge in SQL; completeness_dirty is the
// trigger-fed set of entities whose inputs changed since they were last scored
// (migration 0048 owns the trigger list). The API layer drains the dirty set on
// owner reads through DrainCompleteness, supplying the resolve + Complete pass.

// CompletenessRow is one entity's scored bands plus the scored facets it is
// missing — what the drain writes and the detail self-heal compares against.
type CompletenessRow struct {
	EntityType string
	EntityID   int64
	Required   *int
	Extras     *int
	Missing    []MissingFacet
}

// MissingFacet is one absent scored facet: its canonical name and the band
// (registry criticality) it belongs to.
type MissingFacet struct {
	Canonical string
	Band      string
}

// MissingFacetCount is the "Missing facet" chip's per-facet count for one
// entity type (F55.6, F65.7): a GROUP BY over entity_completeness_missing.
type MissingFacetCount struct {
	Canonical string
	Band      string
	Count     int
}

// DirtyCompleteness returns every entity awaiting a recompute, grouped by
// entity type. A stray row for an entity type nobody scores is returned too,
// so the drain clears it rather than re-reading it forever.
func (r *Repo) DirtyCompleteness(ctx context.Context) (map[string][]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT entity_type, entity_id FROM completeness_dirty ORDER BY entity_type, entity_id`)
	if err != nil {
		return nil, fmt.Errorf("dirty completeness: %w", err)
	}
	defer rows.Close()
	out := make(map[string][]int64)
	for rows.Next() {
		var t string
		var id int64
		if err := rows.Scan(&t, &id); err != nil {
			return nil, err
		}
		out[t] = append(out[t], id)
	}
	return out, rows.Err()
}

// DrainCompleteness runs one drain of the dirty set (ADR-099 D4): under
// writeMu — so no input write can slip between reading the set and clearing
// it and be lost — it reads every dirty (entity_type, ids), asks compute for
// the fresh rows, stores them, and deletes the drained ids. Chunked so the
// "everything is dirty" case (boot, reload) never binds more ids into one
// IN (...) than SQLite allows, one transaction per chunk. compute must only
// read (WAL reads never take writeMu); an id it does not return — soft-
// deleted, inactive, a person with no active video, an entity type nothing
// scores — ends up with no store rows, the correct "nothing to badge" state.
func (r *Repo) DrainCompleteness(ctx context.Context, compute func(entityType string, ids []int64) ([]CompletenessRow, error)) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	dirty, err := r.DirtyCompleteness(ctx)
	if err != nil {
		return err
	}
	for entityType, ids := range dirty {
		for len(ids) > 0 {
			n := min(len(ids), drainChunk)
			chunk := ids[:n]
			ids = ids[n:]
			rows, err := compute(entityType, chunk)
			if err != nil {
				return err
			}
			if err := r.writeCompleteness(ctx, entityType, chunk, rows, true); err != nil {
				return err
			}
		}
	}
	return nil
}

const drainChunk = 500

// StoreCompleteness writes one entity's row — the detail page's self-heal
// (ADR-099 D3). It deliberately leaves completeness_dirty alone: the page's
// live result was computed outside writeMu, so a concurrent input write may
// already have re-dirtied the entity, and a pending recompute is always
// correct to keep.
func (r *Repo) StoreCompleteness(ctx context.Context, row CompletenessRow) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return r.writeCompleteness(ctx, row.EntityType, []int64{row.EntityID}, []CompletenessRow{row}, false)
}

// writeCompleteness replaces the store rows for ids with rows (an id absent
// from rows is simply cleared) and, when clearDirty, drops the ids from the
// dirty set — one transaction. Caller holds writeMu.
func (r *Repo) writeCompleteness(ctx context.Context, entityType string, ids []int64, rows []CompletenessRow, clearDirty bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("write completeness: begin: %w", err)
	}
	defer tx.Rollback()

	// One statement per table for the whole chunk, not per id — the boot /
	// reload drain clears thousands of ids under writeMu.
	in := " WHERE entity_type = ? AND entity_id IN (" + placeholders(len(ids)) + ")"
	args := append([]any{entityType}, toAnySlice(ids)...)
	clears := []string{
		`DELETE FROM entity_completeness` + in,
		`DELETE FROM entity_completeness_missing` + in,
	}
	if clearDirty {
		clears = append(clears, `DELETE FROM completeness_dirty`+in)
	}
	for _, q := range clears {
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("write completeness: clear %s: %w", entityType, err)
		}
	}
	now := time.Now().UTC().Format(timeLayout)
	for _, row := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO entity_completeness (entity_type, entity_id, required, extras, computed_at)
			VALUES (?, ?, ?, ?, ?)`,
			entityType, row.EntityID, nullInt(row.Required), nullInt(row.Extras), now); err != nil {
			return fmt.Errorf("write completeness: insert %s/%d: %w", entityType, row.EntityID, err)
		}
		for _, m := range row.Missing {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO entity_completeness_missing (entity_type, entity_id, canonical, band)
				VALUES (?, ?, ?, ?)`,
				entityType, row.EntityID, m.Canonical, m.Band); err != nil {
				return fmt.Errorf("write completeness: insert missing %s/%d/%s: %w", entityType, row.EntityID, m.Canonical, err)
			}
		}
	}
	return tx.Commit()
}

// MarkAllCompletenessDirty flags every video, person and studio for recompute
// — the denominator-change hook (ADR-099 D4): boot (registry criticality is
// compiled in) and metadata-mapping reload (the field list changed).
func (r *Repo) MarkAllCompletenessDirty(ctx context.Context) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	for _, q := range []string{
		`INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'video', id FROM videos`,
		`INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'person', id FROM people`,
		`INSERT OR IGNORE INTO completeness_dirty (entity_type, entity_id) SELECT 'studio', id FROM studios`,
	} {
		if _, err := r.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("mark all completeness dirty: %w", err)
		}
	}
	return nil
}

// StoredCompleteness reads one entity's cached row plus its missing facets —
// what the detail page compares its live result against before self-healing
// (ADR-099 D3). ErrNotFound when the entity has never been scored.
func (r *Repo) StoredCompleteness(ctx context.Context, entityType string, entityID int64) (*CompletenessRow, error) {
	row := &CompletenessRow{EntityType: entityType, EntityID: entityID}
	var required, extras sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT required, extras FROM entity_completeness
		WHERE entity_type = ? AND entity_id = ?`, entityType, entityID).Scan(&required, &extras)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("stored completeness: %w", err)
	}
	row.Required, row.Extras = intPtr(required), intPtr(extras)
	rows, err := r.db.QueryContext(ctx, `
		SELECT canonical, band FROM entity_completeness_missing
		WHERE entity_type = ? AND entity_id = ? ORDER BY canonical`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("stored completeness missing: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m MissingFacet
		if err := rows.Scan(&m.Canonical, &m.Band); err != nil {
			return nil, err
		}
		row.Missing = append(row.Missing, m)
	}
	return row, rows.Err()
}

// CompletenessForEntities batch-reads the stored bands for a page of list
// items (the ring-badge payload, F65.5). Entities with no row are absent from
// the map — the item then carries no completeness, and the card draws nothing.
func (r *Repo) CompletenessForEntities(ctx context.Context, entityType string, ids []int64) (map[int64]model.CompletenessSummary, error) {
	out := make(map[int64]model.CompletenessSummary, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := append([]any{entityType}, toAnySlice(ids)...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT entity_id, required, extras FROM entity_completeness
		WHERE entity_type = ? AND entity_id IN (`+placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("completeness for entities: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var required, extras sql.NullInt64
		if err := rows.Scan(&id, &required, &extras); err != nil {
			return nil, err
		}
		out[id] = model.CompletenessSummary{Required: intPtr(required), Extras: intPtr(extras)}
	}
	return out, rows.Err()
}

// MissingFacetCounts returns, per scored facet, how many entities of the type
// are currently missing it (F55.6 via the store, F65.7). A facet nobody is
// missing has no row — the chip has nothing to filter to for it anyway.
func (r *Repo) MissingFacetCounts(ctx context.Context, entityType string) ([]MissingFacetCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT canonical, band, COUNT(*) FROM entity_completeness_missing
		WHERE entity_type = ?
		GROUP BY canonical, band
		ORDER BY canonical`, entityType)
	if err != nil {
		return nil, fmt.Errorf("missing facet counts: %w", err)
	}
	defer rows.Close()
	var out []MissingFacetCount
	for rows.Next() {
		var c MissingFacetCount
		if err := rows.Scan(&c.Canonical, &c.Band, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MissingFacetCountsForVideos is MissingFacetCounts for videos composed with
// the caller's browse filters (tags / person / studio / query / ...), so the
// chip's counts reflect the subset the owner is looking at (F55.6). Limit,
// Offset and Sort are ignored.
func (r *Repo) MissingFacetCountsForVideos(ctx context.Context, f VideoFilter) ([]MissingFacetCount, error) {
	where, args := f.build()
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.canonical, m.band, COUNT(*)
		FROM entity_completeness_missing m
		JOIN videos v ON v.id = m.entity_id
		`+where+` AND m.entity_type = 'video'
		GROUP BY m.canonical, m.band
		ORDER BY m.canonical`, args...)
	if err != nil {
		return nil, fmt.Errorf("missing facet counts for videos: %w", err)
	}
	defer rows.Close()
	var out []MissingFacetCount
	for rows.Next() {
		var c MissingFacetCount
		if err := rows.Scan(&c.Canonical, &c.Band, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func intPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}
