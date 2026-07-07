package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"holodex/internal/model"
)

// Near-miss review queue — the S5 surface built on the S4 seed (identity_queue.go).
// S4's SeedIdentityReviewQueue does the one-time detection over canonical ∪ aliases;
// this file adds the parts S5 owns: scan-time flagging of a newly-created entity,
// reading the queue for the owner's Duplicates tab, dismissing a pair (keep-separate),
// the editor's near-miss lookup, and dropping a pair on merge. All reuse S4's
// looseKeyExpr + the shared canonicalTable/nameKeyExpr, so detection stays identical.

// reviewJunction maps each named entity type to its (junction table, fk column), for
// the active-video counts shown on a review pair / near-miss look-alike.
var reviewJunction = map[string][2]string{
	model.EnrichEntityPerson: {"video_people", "person_id"},
	model.EnrichEntityStudio: {"video_studios", "studio_id"},
	model.EntityTag:          {"video_tags", "tag_id"},
}

// reviewEntityOrder lists the entity types tags-first (they dominate the queue) — the
// grouped read follows it.
var reviewEntityOrder = []string{model.EntityTag, model.EnrichEntityStudio, model.EnrichEntityPerson}

// ReviewPair is one flagged possible-duplicate pair (F43 S5): the two entities (id +
// name + active-video count) and the variation kind that made them a near-miss.
type ReviewPair struct {
	EntityType string          `json:"entity_type"`
	A          model.EntityRef `json:"a"`
	B          model.EntityRef `json:"b"`
	Variation  string          `json:"variation"`
}

// variationCase classifies a near-miss pair's difference as internal-whitespace (the
// two names are equal once all spaces are removed) vs. punctuation — matching S4's seed
// classification. Shared by the scan-time flag and the near-miss lookup.
func variationCase(colA, colB string) string {
	return fmt.Sprintf(`CASE WHEN replace(lower(trim(%s)), ' ', '') = replace(lower(trim(%s)), ' ', '')
		THEN 'internal-whitespace' ELSE 'punctuation' END`, colA, colB)
}

// FlagNearMiss records the review-queue pair(s) for a single just-created entity (F43
// S5 scan-time flagging): any existing same-type entity that is a loose-key near-miss
// of `id` (different nameKey, not kept-separate) is queued. Runs inside the caller's
// scan transaction; idempotent. A no-op when the new entity matches nothing. Reuses
// S4's looseKeyExpr so scan-time flagging and the boot seed detect identically.
func FlagNearMiss(ctx context.Context, tx *sql.Tx, entityType string, id int64) error {
	table := canonicalTable(entityType)
	if table == "" {
		return fmt.Errorf("flag near-miss: unknown entity type %q", entityType)
	}
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT OR IGNORE INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
		SELECT '%s', min(o.id, n.id), max(o.id, n.id), %s
		FROM %s n JOIN %s o ON o.id <> n.id
		WHERE n.id = ? AND %s = %s AND %s <> %s
		  AND NOT EXISTS (SELECT 1 FROM entity_keep_separate ks
		                  WHERE ks.entity_type = '%s'
		                    AND ks.id_lo = min(o.id, n.id) AND ks.id_hi = max(o.id, n.id))`,
		entityType, variationCase("o.name", "n.name"), table, table,
		looseKeyExpr("o.name"), looseKeyExpr("n.name"),
		nameKeyExpr(entityType, "o.name"), nameKeyExpr(entityType, "n.name"),
		entityType), id)
	if err != nil {
		return fmt.Errorf("flag near-miss (%s): %w", entityType, err)
	}
	return nil
}

// ListReviewPairs returns every flagged pair, grouped tags-first, each carrying both
// entities' names + active-video counts + variation (F43 P1-3). An INNER JOIN to the
// canonical table drops stale rows whose entity was merged/deleted elsewhere, so the
// queue self-heals (a resolved pair simply stops appearing).
func (r *Repo) ListReviewPairs(ctx context.Context) ([]ReviewPair, error) {
	var out []ReviewPair
	for _, et := range reviewEntityOrder {
		table := canonicalTable(et)
		jn := reviewJunction[et]
		q := fmt.Sprintf(`
			SELECT q.id_lo, la.name, %[3]s, q.id_hi, lb.name, %[4]s, q.variation
			FROM identity_review_queue q
			JOIN %[1]s la ON la.id = q.id_lo
			JOIN %[1]s lb ON lb.id = q.id_hi
			WHERE q.entity_type = ?
			ORDER BY la.name COLLATE NOCASE, lb.name COLLATE NOCASE`,
			table, jn[0], reviewCountExpr(jn, "q.id_lo"), reviewCountExpr(jn, "q.id_hi"))
		rows, err := r.db.QueryContext(ctx, q, et)
		if err != nil {
			return nil, fmt.Errorf("list review pairs (%s): %w", et, err)
		}
		for rows.Next() {
			p := ReviewPair{EntityType: et}
			if err := rows.Scan(&p.A.ID, &p.A.Name, &p.A.VideoCount, &p.B.ID, &p.B.Name, &p.B.VideoCount, &p.Variation); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// reviewCountExpr is the active-video count subquery for one side of a pair (matching
// EntityRef). jn is the entity's (junction, fk); idCol is the queue column to count.
func reviewCountExpr(jn [2]string, idCol string) string {
	return fmt.Sprintf(`(SELECT COUNT(*) FROM %s j JOIN videos v ON v.id = j.video_id
		WHERE j.%s = %s AND v.active = 1 AND v.deleted_at IS NULL)`, jn[0], jn[1], idCol)
}

// DismissReviewPair marks a pair keep-separate (so the detector never re-proposes it)
// and removes it from the queue, in one transaction (F43 P1-3, RD5). Idempotent.
func (r *Repo) DismissReviewPair(ctx context.Context, entityType string, idA, idB int64) error {
	if canonicalTable(entityType) == "" {
		return fmt.Errorf("dismiss: unknown entity type %q", entityType)
	}
	lo, hi := orderPair(idA, idB)
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES (?, ?, ?)`,
		entityType, lo, hi); err != nil {
		return fmt.Errorf("dismiss: keep separate: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM identity_review_queue WHERE entity_type = ? AND id_lo = ? AND id_hi = ?`,
		entityType, lo, hi); err != nil {
		return fmt.Errorf("dismiss: remove pair: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dismiss: %w", err)
	}
	return nil
}

// dropReviewPairsFor removes any queue rows referencing an entity id (either side) —
// called from a merge so a resolved pair leaves no stale row. Runs inside the caller's
// transaction (already under writeMu).
func dropReviewPairsFor(ctx context.Context, tx *sql.Tx, entityType string, id int64) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM identity_review_queue WHERE entity_type = ? AND (id_lo = ? OR id_hi = ?)`,
		entityType, id, id)
	if err != nil {
		return fmt.Errorf("drop review pairs: %w", err)
	}
	return nil
}

// NearMiss returns the fuzzy near-miss entity for a candidate name — an existing entity
// whose loose key matches but whose identity nameKey differs (so it is NOT an exact
// collision, which EntityConflict handles), excluding selfID and any kept-separate pair
// — or nil if none. Backs the editor's non-blocking "looks like X — merge instead?"
// hint (F43 P1-5). Returns one match (rarely more than one at personal scale).
func (r *Repo) NearMiss(ctx context.Context, entityType string, selfID int64, name string) (*model.EntityRef, error) {
	table := canonicalTable(entityType)
	if table == "" {
		return nil, fmt.Errorf("near-miss: unknown entity type %q", entityType)
	}
	jn := reviewJunction[entityType]
	var ref model.EntityRef
	err := r.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT e.id, e.name, %s
		FROM %s e
		WHERE e.id <> ? AND %s = %s AND %s <> %s
		  AND NOT EXISTS (SELECT 1 FROM entity_keep_separate ks WHERE ks.entity_type = ?
		                  AND ks.id_lo = min(e.id, ?) AND ks.id_hi = max(e.id, ?))
		ORDER BY e.name COLLATE NOCASE LIMIT 1`,
		reviewCountExpr(jn, "e.id"), table,
		looseKeyExpr("e.name"), looseKeyExpr("?"),
		nameKeyExpr(entityType, "e.name"), nameKeyExpr(entityType, "?")),
		selfID, name, name, entityType, selfID, selfID).Scan(&ref.ID, &ref.Name, &ref.VideoCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("near-miss (%s): %w", entityType, err)
	}
	return &ref, nil
}
