package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// enrichment_dismissals (F47, ADR-066 D2): the durable "not matched" verdict store —
// the direct structural sibling of entity_keep_separate (review_queue.go), scoped to
// (entity_type, entity_id, provider) rather than a pair of entities.

// DismissEnrichment records that the owner reviewed a provider's candidates for an
// entity and none matched (RD4). Idempotent — re-dismissing refreshes dismissed_at
// rather than erroring or duplicating.
func (r *Repo) DismissEnrichment(ctx context.Context, entityType string, entityID int64, provider string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO enrichment_dismissals (entity_type, entity_id, provider, dismissed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_id, provider) DO UPDATE SET dismissed_at = excluded.dismissed_at`,
		entityType, entityID, provider, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("dismiss enrichment: %w", err)
	}
	return nil
}

// UndismissEnrichment clears a dismissal ("Try again", RD4), so a future /resolve for
// the pair is no longer blocked. Idempotent — clearing a non-existent dismissal is a
// no-op success.
func (r *Repo) UndismissEnrichment(ctx context.Context, entityType string, entityID int64, provider string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM enrichment_dismissals WHERE entity_type = ? AND entity_id = ? AND provider = ?`,
		entityType, entityID, provider)
	if err != nil {
		return fmt.Errorf("undismiss enrichment: %w", err)
	}
	return nil
}

// EnrichmentDismissed reports whether (entityType, entityID, provider) carries a
// durable "not matched" verdict — the guard /resolve consults before ever dialing a
// provider again for a pair the owner already rejected (RD4).
func (r *Repo) EnrichmentDismissed(ctx context.Context, entityType string, entityID int64, provider string) (bool, error) {
	var x int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM enrichment_dismissals WHERE entity_type = ? AND entity_id = ? AND provider = ?`,
		entityType, entityID, provider).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("enrichment dismissed: %w", err)
	}
	return true, nil
}
