package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"holodex/internal/model"
)

// Orphan lifecycle (HOLODEX-535, docs/architecture/entity-relationships.md "one
// orphan stamp and one sweep"). People, studios and tags carry orphaned_at, which
// the migration-0059 triggers stamp when an entity's last video link goes (any
// path, including a purge cascade) and clear when a link returns. SweepOrphans is
// the only place an orphan is deleted, and it never deletes one carrying authored
// data.

// orphanAuthoredSQL is, per entity type, a predicate over the candidate row `e`
// that is true when the entity carries authored data the sweep must never destroy.
// An alias also covers merge history: a merge registers the loser's name as an
// alias of the survivor. A keep-separate pair is an owner decision too. An image
// counts only when the owner made it (uploaded, or a promoted copy): a
// provider-downloaded one comes back on the next enrich (HOLODEX-548), and every
// cast member a video enrich creates carries one. Owner settings on a downloaded
// image that an enrich cannot restore still count: a rejected headshot
// (suppression) and a studio logo's halo choice.
var orphanAuthoredSQL = map[string]string{
	model.EnrichEntityPerson: `
		EXISTS(SELECT 1 FROM entity_aliases WHERE entity_type = 'person' AND entity_id = e.id)
		OR EXISTS(SELECT 1 FROM entity_keep_separate WHERE entity_type = 'person' AND e.id IN (id_lo, id_hi))
		OR EXISTS(SELECT 1 FROM person_images WHERE person_id = e.id AND source != 'enrichment')
		OR EXISTS(SELECT 1 FROM person_image_suppressions WHERE person_id = e.id)
		OR EXISTS(SELECT 1 FROM film_people_roles WHERE person_id = e.id)
		OR EXISTS(SELECT 1 FROM field_source_decisions WHERE entity_type = 'person' AND entity_id = e.id)
		OR EXISTS(SELECT 1 FROM metadata_curation WHERE entity_type = 'person' AND entity_id = e.id)`,
	model.EnrichEntityStudio: `
		EXISTS(SELECT 1 FROM entity_aliases WHERE entity_type = 'studio' AND entity_id = e.id)
		OR EXISTS(SELECT 1 FROM entity_keep_separate WHERE entity_type = 'studio' AND e.id IN (id_lo, id_hi))
		OR EXISTS(SELECT 1 FROM studio_images WHERE studio_id = e.id AND source != 'enrichment')
		OR EXISTS(SELECT 1 FROM studio_image_halo WHERE studio_id = e.id)
		OR EXISTS(SELECT 1 FROM field_source_decisions WHERE entity_type = 'studio' AND entity_id = e.id)
		OR EXISTS(SELECT 1 FROM metadata_curation WHERE entity_type = 'studio' AND entity_id = e.id)`,
	model.EntityTag: `
		EXISTS(SELECT 1 FROM entity_aliases WHERE entity_type = 'tag' AND entity_id = e.id
		       AND alias <> replace(e.name, ' ', '-')) -- its own dashed alias is automatic (F43 P0-12), not authored
		OR EXISTS(SELECT 1 FROM entity_keep_separate WHERE entity_type = 'tag' AND e.id IN (id_lo, id_hi))
		OR EXISTS(SELECT 1 FROM field_source_decisions WHERE entity_type = 'tag' AND entity_id = e.id)
		OR EXISTS(SELECT 1 FROM metadata_curation WHERE entity_type = 'tag' AND entity_id = e.id)
		OR EXISTS(SELECT 1 FROM category_tags WHERE tag_id = e.id)
		OR EXISTS(SELECT 1 FROM tags WHERE parent_tag_id = e.id)
		OR e.parent_tag_id IS NOT NULL OR e.writeback_enabled = 0`,
}

// orphanSweepOrder is the order SweepOrphans visits entity types (deterministic).
var orphanSweepOrder = []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EntityTag}

// SweepOrphans deletes people, studios and tags orphaned more than graceDays ago
// that still have no link and carry no authored data (orphanAuthoredSQL), along
// with their entity_enrichment and identity_review_queue rows (polymorphic, no
// trigger; an id can be reused, so a stale pair would attach to a new entity). FK cascades and the
// *_ad_* triggers clean up the rest. Authored orphans are skipped and counted,
// never deleted. One transaction under writeMu; idempotent. Returns totals across
// all three types for the caller's System Activity record.
func (r *Repo) SweepOrphans(ctx context.Context, graceDays int) (deleted, skipped int, err error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	cutoff := time.Now().UTC().AddDate(0, 0, -graceDays).Format(timeLayout)
	for _, entityType := range orphanSweepOrder {
		d, s, err := sweepOrphansOfType(ctx, tx, entityType, cutoff)
		if err != nil {
			return 0, 0, err
		}
		deleted += d
		skipped += s
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit orphan sweep: %w", err)
	}
	return deleted, skipped, nil
}

func sweepOrphansOfType(ctx context.Context, tx *sql.Tx, entityType, cutoff string) (deleted, skipped int, err error) {
	cfg := entityIdentityByType[entityType]
	// The NOT EXISTS re-check makes a stale stamp harmless: the sweep never deletes
	// an entity that has a link, whatever its orphaned_at says.
	rows, err := tx.QueryContext(ctx, `SELECT e.id, `+orphanAuthoredSQL[entityType]+`
		FROM `+cfg.table+` e
		WHERE e.orphaned_at IS NOT NULL AND e.orphaned_at < ?
		  AND NOT EXISTS (SELECT 1 FROM `+cfg.assoc+` WHERE `+cfg.assocFK+` = e.id)`,
		cutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("find orphaned %s: %w", entityType, err)
	}
	var doomed []int64
	for rows.Next() {
		var id int64
		var authored bool
		if err := rows.Scan(&id, &authored); err != nil {
			rows.Close()
			return 0, 0, err
		}
		if authored {
			skipped++
			continue
		}
		doomed = append(doomed, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	for _, id := range doomed {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM entity_enrichment WHERE entity_type = ? AND entity_id = ?`, entityType, id); err != nil {
			return 0, 0, fmt.Errorf("drop orphaned %s enrichment: %w", entityType, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM identity_review_queue WHERE entity_type = ? AND ? IN (id_lo, id_hi)`, entityType, id); err != nil {
			return 0, 0, fmt.Errorf("drop orphaned %s review pairs: %w", entityType, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+cfg.table+` WHERE id = ?`, id); err != nil {
			return 0, 0, fmt.Errorf("delete orphaned %s: %w", entityType, err)
		}
		deleted++
	}
	return deleted, skipped, nil
}
