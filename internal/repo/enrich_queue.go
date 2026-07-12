package repo

import (
	"context"
	"fmt"

	"holodex/internal/model"
)

// Owner review-queue listing (F47, ADR-065 RD2/RD9/P0-1): every Person/Studio/Media
// entity missing at least one supporting provider's data. The direct structural
// precedent is review_queue.go's Duplicates listing — grouped-by-type, one query per
// group, no N+1. Unlike Duplicates, membership here is a pure "is a row present in
// entity_enrichment" check, so building the list never calls a provider (RD2's
// "zero-cost DB signal").

// EnrichQueueRow is one review-queue entity: the entity plus its outstanding
// (not-yet-linked) providers, each carrying a state derived purely from stored data.
type EnrichQueueRow struct {
	EntityType string                     `json:"entity_type"`
	EntityID   int64                      `json:"entity_id"`
	Name       string                     `json:"name"`
	Providers  []EnrichQueueProviderState `json:"providers"`
}

// EnrichQueueProviderState is one row's per-provider status (RD9 — never a single
// collapsed flag): 'unreviewed' (no entity_enrichment row, no dismissal) or
// 'not_matched' (dismissed). A linked provider (has a row) is omitted entirely — it
// needs no review. 'auto_applied'/'needs_review' only ever exist client-side, after
// the owner opens the row and a /resolve call actually runs (S3) — the backend never
// computes them here.
type EnrichQueueProviderState struct {
	Provider string `json:"provider"`
	State    string `json:"state"`
}

// enrichQueueEntityOrder is the design handoff's resolved grouping (spec Q3): People →
// Studios → Media, nav order — not Duplicates' frequency-driven tags-first ordering,
// since no entity type dominates the enrichment backlog the way tags dominate
// near-miss duplicates.
var enrichQueueEntityOrder = []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EnrichEntityVideo}

// EnrichQueue builds the owner's review-queue listing. providersByType maps each
// entity type to the provider names that support it (the caller derives this from the
// live provider registry — the repo layer knows nothing about providers). An entity
// qualifies only when it has at least one outstanding provider that is NOT dismissed;
// a fully-dismissed entity drops out of a fresh load (the frontend may still show it
// locally after an in-place dismiss until the next reload, RD3). No provider is ever
// called to build this list.
func (r *Repo) EnrichQueue(ctx context.Context, providersByType map[string][]string) ([]EnrichQueueRow, error) {
	var out []EnrichQueueRow
	for _, et := range enrichQueueEntityOrder {
		providers := providersByType[et]
		if len(providers) == 0 {
			continue
		}
		rows, err := r.enrichQueueForType(ctx, et, providers)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

// enrichQueueForType builds the queue rows for one entity type, batched: one query for
// every entity of the type, one for which (entity,provider) pairs already have stored
// enrichment, one for which are dismissed — never N+1 per entity.
func (r *Repo) enrichQueueForType(ctx context.Context, entityType string, providers []string) ([]EnrichQueueRow, error) {
	entities, err := r.enrichQueueEntities(ctx, entityType)
	if err != nil {
		return nil, err
	}
	if len(entities) == 0 {
		return nil, nil
	}
	linked, err := r.entityProviderPairs(ctx, "entity_enrichment", entityType, providers)
	if err != nil {
		return nil, fmt.Errorf("enrich queue linked (%s): %w", entityType, err)
	}
	dismissed, err := r.entityProviderPairs(ctx, "enrichment_dismissals", entityType, providers)
	if err != nil {
		return nil, fmt.Errorf("enrich queue dismissed (%s): %w", entityType, err)
	}

	var out []EnrichQueueRow
	for _, e := range entities {
		row := EnrichQueueRow{EntityType: entityType, EntityID: e.ID, Name: e.Name}
		hasUnreviewed := false
		for _, p := range providers {
			if linked[e.ID][p] {
				continue // has data — nothing to review
			}
			state := "unreviewed"
			if dismissed[e.ID][p] {
				state = "not_matched"
			} else {
				hasUnreviewed = true
			}
			row.Providers = append(row.Providers, EnrichQueueProviderState{Provider: p, State: state})
		}
		if hasUnreviewed {
			out = append(out, row)
		}
	}
	return out, nil
}

// enrichQueueEntities returns (id, name) for every entity of a queue-eligible type,
// ordered by name. Person/studio read their canonical table directly (no active flag —
// they have no soft-delete concept); video is restricted to active/not-deleted, since a
// trashed item has no business in the owner's backlog.
func (r *Repo) enrichQueueEntities(ctx context.Context, entityType string) ([]model.EntityRef, error) {
	var q string
	switch entityType {
	case model.EnrichEntityPerson:
		q = `SELECT id, name FROM people ORDER BY name COLLATE NOCASE`
	case model.EnrichEntityStudio:
		q = `SELECT id, name FROM studios ORDER BY name COLLATE NOCASE`
	case model.EnrichEntityVideo:
		q = `SELECT id, title FROM videos WHERE active = 1 AND deleted_at IS NULL ORDER BY title COLLATE NOCASE`
	default:
		return nil, fmt.Errorf("enrich queue: unknown entity type %q", entityType)
	}
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("enrich queue entities (%s): %w", entityType, err)
	}
	defer rows.Close()
	var out []model.EntityRef
	for rows.Next() {
		var ref model.EntityRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// entityProviderPairs reads which (entity_id, provider) pairs exist in `table`
// (entity_enrichment or enrichment_dismissals — both share the entity_type/entity_id/
// provider shape) for the given entity type and provider set, as a nested existence
// map. table is a trusted internal literal, never user input.
func (r *Repo) entityProviderPairs(ctx context.Context, table, entityType string, providers []string) (map[int64]map[string]bool, error) {
	args := append([]any{entityType}, toAnySlice(providers)...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT entity_id, provider FROM `+table+`
		WHERE entity_type = ? AND provider IN (`+placeholders(len(providers))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[string]bool{}
	for rows.Next() {
		var id int64
		var provider string
		if err := rows.Scan(&id, &provider); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]bool{}
		}
		out[id][provider] = true
	}
	return out, rows.Err()
}
