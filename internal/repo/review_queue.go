package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
// name + active-video count), the variation kind that made them a near-miss, and the
// match kind — the strongest live evidence still connecting them (see MatchKind below).
type ReviewPair struct {
	EntityType string          `json:"entity_type"`
	A          model.EntityRef `json:"a"`
	B          model.EntityRef `json:"b"`
	Variation  string          `json:"variation"`
	MatchKind  string          `json:"match_kind"`
}

// entityNamesUnion returns the (eid, kind, nm) subquery over one entity type's full
// live name pool — canonical name ∪ every current alias. Two uses: (1) re-validating
// that a stored identity_review_queue row still collides under the CURRENT name set,
// not just whatever it was at flag-time — a rename or alias edit can otherwise leave a
// stale row behind forever, since today only merge/dismiss ever remove one; (2)
// classifying which side of the match is canonical vs. alias, so a pair backed by two
// coincidentally-similar aliases (weak evidence — aliases on unrelated entities collide
// far more than canonical names do) can be told apart from a canonical-name collision
// (strong evidence). entityType is a trusted internal literal, never user input.
func entityNamesUnion(table, entityType string) string {
	return fmt.Sprintf(`(SELECT id AS eid, 'canonical' AS kind, name AS nm FROM %s
		UNION ALL SELECT entity_id, 'alias', alias FROM entity_aliases WHERE entity_type = %s)`,
		table, sqlStringLit(entityType))
}

// variationCase classifies a near-miss pair's difference as internal-whitespace (the
// two names are equal once all spaces are removed) vs. punctuation — matching S4's seed
// classification. Shared by the scan-time flag and the near-miss lookup.
func variationCase(colA, colB string) string {
	return fmt.Sprintf(`CASE WHEN replace(lower(trim(%s)), ' ', '') = replace(lower(trim(%s)), ' ', '')
		THEN 'internal-whitespace' ELSE 'punctuation' END`, colA, colB)
}

// execer is the exec-only capability *sql.DB and *sql.Tx share (queryRower's
// write-side counterpart, identity.go) — lets flagNearMissForName run inside a
// caller's transaction (scan, merge, rename) or standalone (AddEntityAlias, which
// isn't itself transactional).
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// flagNearMissForName records identity_review_queue pair(s) for `name` — a candidate
// name-form belonging to `selfID` — against every OTHER entity of entityType's full
// live name pool (canonical name ∪ every alias, matching entityNamesUnion / the S4
// seed's own universe, so real-time and batch detection never disagree). `name` may be
// a brand-new entity's own canonical name (FlagNearMiss below) or a freshly added
// alias: a manual add (AddEntityAlias), a merge's "name → alias" step, or a rename's
// "old name → alias" step. Flagging all three live, instead of leaving them for the
// next boot-time SeedIdentityReviewQueue pass, is the point — a near-miss should
// surface the moment it exists, not accumulate silently until the next restart (the
// private-media investigation this follows up on found a batch of such pairs
// appearing at once for exactly that reason). Excludes selfID's own name-forms (an
// entity never near-misses itself) and any kept-separate pair. Idempotent (INSERT OR
// IGNORE); only ever inserts a fuzzyVariations row (see ListReviewPairs) — this has
// nothing to do with the separate provider-alias/exact-conflict path.
func flagNearMissForName(ctx context.Context, ex execer, entityType string, selfID int64, name string) error {
	table := canonicalTable(entityType)
	if table == "" {
		return fmt.Errorf("flag near-miss for name: unknown entity type %q", entityType)
	}
	names := entityNamesUnion(table, entityType)
	_, err := ex.ExecContext(ctx, fmt.Sprintf(`
		INSERT OR IGNORE INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
		SELECT %[1]s, min(o.eid, n.eid), max(o.eid, n.eid), %[2]s
		FROM %[3]s o, (SELECT ? AS eid, ? AS nm) n
		WHERE o.eid <> n.eid AND %[4]s = %[5]s AND %[6]s <> %[7]s
		  AND NOT EXISTS (SELECT 1 FROM entity_keep_separate ks
		                  WHERE ks.entity_type = %[1]s AND ks.id_lo = min(o.eid, n.eid) AND ks.id_hi = max(o.eid, n.eid))`,
		sqlStringLit(entityType), variationCase("o.nm", "n.nm"), names,
		looseKeyExpr("o.nm"), looseKeyExpr("n.nm"),
		nameKeyExpr(entityType, "o.nm"), nameKeyExpr(entityType, "n.nm")),
		selfID, name)
	if err != nil {
		return fmt.Errorf("flag near-miss for name (%s): %w", entityType, err)
	}
	return nil
}

// FlagNearMiss records the review-queue pair(s) for a single just-created entity (F43
// S5 scan-time flagging): looks up the entity's own (freshly inserted) canonical name
// and delegates to flagNearMissForName, so scan-time flagging shares one detection
// path with the real-time alias flagging below — including now catching a brand-new
// entity's name against an EXISTING ALIAS on another entity, which this check's
// previous canonical-only comparison used to miss entirely. Runs inside the caller's
// scan transaction; idempotent. A no-op when the new entity matches nothing.
func FlagNearMiss(ctx context.Context, tx *sql.Tx, entityType string, id int64) error {
	table := canonicalTable(entityType)
	if table == "" {
		return fmt.Errorf("flag near-miss: unknown entity type %q", entityType)
	}
	var name string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM `+table+` WHERE id = ?`, id).Scan(&name); err != nil {
		return fmt.Errorf("flag near-miss: load %s name: %w", entityType, err)
	}
	return flagNearMissForName(ctx, tx, entityType, id, name)
}

// fuzzyVariations lists the variation values the near-miss detector itself produces
// (identity_queue.go's seed + review_queue.go's scan-time flag) — the only rows
// ListReviewPairs' live-revalidation applies to. Any OTHER variation (today just
// F58/ADR-088's "provider-alias": an EXACT alias conflict between two entities whose
// actual names are NOT required to resemble each other at all — see ListReviewPairs)
// is passed through untouched, on purpose: re-validating "do these two entities'
// names/aliases loosely collide" would silently drop a live, resolvable conflict that
// was never about name similarity in the first place. New variations default to this
// pass-through side unless explicitly added here.
var fuzzyVariations = []string{"internal-whitespace", "punctuation"}

// ListReviewPairs returns every flagged pair still live, grouped tags-first (then
// strongest-evidence-first within a group), each carrying both entities' names +
// active-video counts + variation + match kind (F43 P1-3).
//
// Two INNER JOINs keep the queue self-healing for near-miss rows instead of trusting a
// stored row forever: the join to the canonical table drops a pair whose entity was
// merged/deleted elsewhere, and (fuzzyVariations rows only) the join to the live
// match-kind subquery drops a pair whose current names/aliases no longer collide AT
// ALL — which a rename or alias edit can cause silently, since (unlike merge) neither
// one ever touched identity_review_queue. Confirmed on the private-media instance: of
// 207 stored person pairs, only 4 still collided under the live name set — the other
// 203 were exactly this kind of orphan, most from renames done long after the pair was
// flagged. A non-fuzzy row (provider-alias) always passes through with match_kind ”.
//
// For a fuzzy row, match_kind is the strongest live evidence connecting the pair:
// "canonical" (both sides' canonical names collide — the strong, "same entity typo'd
// twice" case), "mixed" (one side needs an alias), or "alias" (only an alias on EACH
// side collides — the weakest signal, since aliases on genuinely distinct entities
// coincide far more often than canonical names do, especially after a few rounds of
// merging). Rows sort strongest-first (provider-alias, being about a concrete,
// resolvable conflict rather than a fuzziness tier, sorts first of all) so the weak
// alias-alias matches sink to the bottom of each group instead of mixing in
// indistinguishably with real duplicates.
func (r *Repo) ListReviewPairs(ctx context.Context) ([]ReviewPair, error) {
	fuzzyList := "'" + strings.Join(fuzzyVariations, "','") + "'"
	var out []ReviewPair
	for _, et := range reviewEntityOrder {
		table := canonicalTable(et)
		jn := reviewJunction[et]
		names := entityNamesUnion(table, et)
		q := fmt.Sprintf(`
			SELECT q.id_lo, la.name, %[3]s, q.id_hi, lb.name, %[4]s, q.variation,
			       CASE WHEN q.variation NOT IN (%[8]s) THEN '' ELSE coalesce(m.match_kind, '') END
			FROM identity_review_queue q
			JOIN %[1]s la ON la.id = q.id_lo
			JOIN %[1]s lb ON lb.id = q.id_hi
			LEFT JOIN (
				SELECT iq.id_lo, iq.id_hi,
					CASE min(CASE WHEN x.kind = 'canonical' AND y.kind = 'canonical' THEN 0
					              WHEN x.kind = 'alias'     AND y.kind = 'alias'     THEN 2
					              ELSE 1 END)
						WHEN 0 THEN 'canonical' WHEN 2 THEN 'alias' ELSE 'mixed' END AS match_kind
				FROM identity_review_queue iq
				JOIN %[5]s x ON x.eid = iq.id_lo
				JOIN %[5]s y ON y.eid = iq.id_hi AND %[6]s = %[7]s
				WHERE iq.entity_type = ? AND iq.variation IN (%[8]s)
				GROUP BY iq.id_lo, iq.id_hi
			) m ON m.id_lo = q.id_lo AND m.id_hi = q.id_hi
			WHERE q.entity_type = ?
			  AND (q.variation NOT IN (%[8]s) OR m.match_kind IS NOT NULL)
			ORDER BY CASE WHEN q.variation NOT IN (%[8]s) THEN -1
			              WHEN m.match_kind = 'canonical' THEN 0
			              WHEN m.match_kind = 'mixed' THEN 1
			              ELSE 2 END,
			         la.name COLLATE NOCASE, lb.name COLLATE NOCASE`,
			table, jn[0], reviewCountExpr(jn, "q.id_lo"), reviewCountExpr(jn, "q.id_hi"),
			names, looseKeyExpr("x.nm"), looseKeyExpr("y.nm"), fuzzyList)
		rows, err := r.db.QueryContext(ctx, q, et, et)
		if err != nil {
			return nil, fmt.Errorf("list review pairs (%s): %w", et, err)
		}
		for rows.Next() {
			p := ReviewPair{EntityType: et}
			if err := rows.Scan(&p.A.ID, &p.A.Name, &p.A.VideoCount, &p.B.ID, &p.B.Name, &p.B.VideoCount, &p.Variation, &p.MatchKind); err != nil {
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
