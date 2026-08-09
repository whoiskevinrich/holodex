package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// enrichMultiSep joins/splits multi-valued enrichment fields within one row.
// Newline can't appear in a sanitized provider value (the enrich service strips
// control chars), so it's an unambiguous separator.
const enrichMultiSep = "\n"

// EnrichmentRow is one stored plugin-sourced field for an entity (F22, ADR-033).
// Values is already split for multi-valued fields.
type EnrichmentRow struct {
	Provider   string
	FieldKey   string
	Values     []string
	ExternalID string
	FetchedAt  time.Time
}

// UpsertEnrichment writes a provider's fetched fields for one entity into the
// shadow layer, in a single transaction under the write lock. Each canonical
// field is one row (multi values newline-joined); a re-fetch overwrites by the
// unique key. The external match id is stamped on every row so a later re-enrich
// can skip identity (F22.4b). Passing an empty fields map is a no-op success.
func (r *Repo) UpsertEnrichment(ctx context.Context, entityType string, entityID int64, provider, externalID string, fields map[string][]string) error {
	if len(fields) == 0 {
		return nil
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	now := time.Now().UTC().Format(timeLayout)
	for key, vals := range fields {
		joined := strings.Join(vals, enrichMultiSep)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO entity_enrichment (entity_type, entity_id, provider, field_key, value, external_id, fetched_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(entity_type, entity_id, provider, field_key) DO UPDATE SET
				value       = excluded.value,
				external_id = excluded.external_id,
				fetched_at  = excluded.fetched_at`,
			entityType, entityID, provider, key, joined, externalID, now); err != nil {
			return fmt.Errorf("upsert enrichment: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit enrichment: %w", err)
	}
	return nil
}

// EnrichmentForEntity returns the stored enrichment rows for one entity, ordered
// by provider then field for stable display (F22.7).
func (r *Repo) EnrichmentForEntity(ctx context.Context, entityType string, entityID int64) ([]EnrichmentRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT provider, field_key, value, external_id, fetched_at
		FROM entity_enrichment
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY provider, field_key`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("enrichment for entity: %w", err)
	}
	defer rows.Close()

	var out []EnrichmentRow
	for rows.Next() {
		var (
			er         EnrichmentRow
			value      string
			fetchedStr string
		)
		if err := rows.Scan(&er.Provider, &er.FieldKey, &value, &er.ExternalID, &fetchedStr); err != nil {
			return nil, err
		}
		er.Values = strings.Split(value, enrichMultiSep)
		er.FetchedAt, _ = time.Parse(timeLayout, fetchedStr)
		out = append(out, er)
	}
	return out, rows.Err()
}

// MatchExternalID returns the upstream record id a provider was last confirmed
// against for an entity, so a re-enrich skips the disambiguation step (F22.4b).
// ok=false when the entity has never been enriched by that provider.
func (r *Repo) MatchExternalID(ctx context.Context, entityType string, entityID int64, provider string) (string, bool, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		SELECT external_id FROM entity_enrichment
		WHERE entity_type = ? AND entity_id = ? AND provider = ? AND external_id <> ''
		LIMIT 1`, entityType, entityID, provider).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("match external id: %w", err)
	}
	return id, true, nil
}

// EnrichmentForVideos returns stored enrichment rows for a batch of video IDs,
// grouped by id — a single query so list pages avoid N+1 (F27). The returned map
// only has keys for ids that actually have rows; a missing key means no enrichment.
func (r *Repo) EnrichmentForVideos(ctx context.Context, ids []int64) (map[int64][]EnrichmentRow, error) {
	return r.EnrichmentForEntities(ctx, "video", ids)
}

// EnrichmentForEntities is EnrichmentForVideos generalized to any entity type — the
// F55 list-wide completeness resolve (ADR-081 D4) needs the same batch shape for
// person/studio, which EnrichmentForVideos' hardcoded "video" can't serve.
func (r *Repo) EnrichmentForEntities(ctx context.Context, entityType string, ids []int64) (map[int64][]EnrichmentRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := append([]any{entityType}, toAnySlice(ids)...)
	rows, err := r.db.QueryContext(ctx, `
		SELECT entity_id, provider, field_key, value, external_id, fetched_at
		FROM entity_enrichment
		WHERE entity_type = ? AND entity_id IN (`+placeholders(len(ids))+`)
		ORDER BY entity_id, provider, field_key`, args...)
	if err != nil {
		return nil, fmt.Errorf("enrichment for entities: %w", err)
	}
	defer rows.Close()

	out := make(map[int64][]EnrichmentRow, len(ids))
	for rows.Next() {
		var (
			eid        int64
			er         EnrichmentRow
			value      string
			fetchedStr string
		)
		if err := rows.Scan(&eid, &er.Provider, &er.FieldKey, &value, &er.ExternalID, &fetchedStr); err != nil {
			return nil, err
		}
		er.Values = strings.Split(value, enrichMultiSep)
		er.FetchedAt, _ = time.Parse(timeLayout, fetchedStr)
		out[eid] = append(out[eid], er)
	}
	return out, rows.Err()
}

// InsertWriteback appends one successful file-writeback to the audit log (F28,
// ADR-041). Called only after Write() returns nil — a failed write must never
// produce an audit row.
func (r *Repo) InsertWriteback(ctx context.Context, videoID int64, fieldKey, tagName, value, source string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO file_writebacks (video_id, field_key, tag_name, value, source, written_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		videoID, fieldKey, tagName, value, source, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("insert writeback: %w", err)
	}
	return nil
}

// DeleteEnrichmentByProvider removes one provider's contribution for an entity so
// the affected fields fall back to their next source (F22.7b). Returns the number
// of rows removed.
func (r *Repo) DeleteEnrichmentByProvider(ctx context.Context, entityType string, entityID int64, provider string) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM entity_enrichment
		WHERE entity_type = ? AND entity_id = ? AND provider = ?`, entityType, entityID, provider)
	if err != nil {
		return 0, fmt.Errorf("delete enrichment: %w", err)
	}
	return res.RowsAffected()
}
