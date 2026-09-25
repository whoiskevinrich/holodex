package repo

import (
	"context"
	"fmt"
	"strings"

	"holodex/internal/model"
)

// Shared-provider-external-id duplicate detection (F71, ADR-107). Two entities of one
// kind carrying the same provider external id is the strongest positive merge evidence
// the system has — the provider has already asserted they are one record — and until F71
// nothing could see it: every other detector works by name, and ADR-061's unique nameKey
// index means a split identity always wears two DIFFERENT names.
//
// Two producers write these rows, and both land here:
//   - the write-time guard on Repo.AttachExternalID (identity.go), which closes the source
//     by turning a dropped identity claim into a review row;
//   - SweepSharedExternalIDs below, which reconciles the whole library every boot.
//
// Neither ever merges. ADR-107 D3: the merge is irreversible (ADR-061), providers carry
// their own duplicate entries, and decisively the colliding data was produced by an owner
// picking the wrong entity in the enrich UI — so auto-merging would let one mis-click fold
// two real people together.

// sharedExternalIDKind reports whether a contested provider id on this kind is a
// reviewable duplicate (ADR-107 D5). Person, studio and film share the one enrich path
// and the one spine table. Video has no spine row and two files of one movie
// legitimately carry the same provider id; tag is not enrichable.
func sharedExternalIDKind(entityType string) bool {
	switch entityType {
	case model.EnrichEntityPerson, model.EnrichEntityStudio, model.EnrichEntityFilm:
		return true
	}
	return false
}

// providerOf returns the namespace half of a "<provider>:<id>" external id — the name the
// queue row's chip cites as having asserted the pair ("tmdb says one person", design
// handoff). Empty when the id carries no namespace, which the row renders as an unnamed
// provider rather than guessing; the enrich call site only attaches identityShaped ids, so
// that is a defensive case rather than a reachable one.
func providerOf(externalID string) string {
	ns, _, ok := strings.Cut(externalID, ":")
	if !ok {
		return ""
	}
	return ns
}

// queueSharedExternalIDPair records two entities of one kind that both claim a provider
// external id, for the owner to merge or dismiss (F71 P0-2), and reports how many rows it
// wrote — 1 for a new pair or an upgraded one, 0 when the row already said this. Shared by
// both producers.
//
// Ordered id_lo/id_hi so the same pair reached from either direction is one row, and gated
// on entity_keep_separate so a pair the owner has already dismissed is never re-proposed
// (F43 RD5 / ADR-061's durable no — and a refresh sweep would otherwise nag on every run).
//
// `detail` (0045) carries the ASSERTING PROVIDER's namespace, which is the one fact the row
// cannot derive from the two entities — the design chose a chip that cites who made the
// claim over printing the raw variation slug, so the row needs the name. Not the external id
// itself: that is a provider-internal string the owner cannot act on, and the compare panel
// already shows it as a provider-link badge. Upgrading a `provider-alias` row therefore
// replaces its detail (the dropped alias name) with this provider; that information has no
// reader today and the row is becoming a different kind of finding anyway — the same
// one-fact-per-pair trade the single `variation` column already makes.
//
// The upsert UPGRADES a weaker variation rather than leaving it (spec RD8). A pair can be
// both a shared-id finding and a name near-miss — 1 of the 15 found on the live library was
// — and under a plain INSERT OR IGNORE that pair would keep a label the queue renders as
// *weak* and sort in the fuzzy band, which is the opposite of what this evidence means. It
// is a one-way ratchet: SeedIdentityReviewQueue and queueProviderAliasPair both write with
// INSERT OR IGNORE, so nothing can demote a row back. The DO UPDATE's own WHERE keeps a row
// that already carries the value from counting as written, which is what lets the sweep
// report 0 on an unchanged pass. `detail` stays unset (0045): unlike provider-alias, both
// sides of this pair are readable from the entities themselves.
func queueSharedExternalIDPair(ctx context.Context, ex execer, entityType string, a, b int64, provider string) (int64, error) {
	lo, hi := orderPair(a, b)
	// The WHERE clause is also what lets the upsert parse after a SELECT — without one,
	// SQLite can read the ON CONFLICT as a join constraint. The DO UPDATE's own WHERE is
	// what keeps an unchanged row from counting as written, so the sweep can report 0; the
	// detail half of it also makes a row whose provider is missing or stale self-heal on
	// the next pass.
	res, err := ex.ExecContext(ctx, `
		INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation, detail)
		SELECT ?, ?, ?, 'shared-external-id', ?
		WHERE NOT EXISTS (
			SELECT 1 FROM entity_keep_separate ks
			 WHERE ks.entity_type = ? AND ks.id_lo = ? AND ks.id_hi = ?)
		ON CONFLICT (entity_type, id_lo, id_hi) DO UPDATE
		   SET variation = 'shared-external-id', detail = excluded.detail
		 WHERE identity_review_queue.variation <> 'shared-external-id'
		    OR identity_review_queue.detail <> excluded.detail`,
		entityType, lo, hi, provider, entityType, lo, hi)
	if err != nil {
		return 0, fmt.Errorf("queue shared-external-id pair (%s): %w", entityType, err)
	}
	return res.RowsAffected()
}

// sharedExternalIDPairsSQL finds every pair of entities of one kind that both claim the
// same provider external id. It is the Go port of migration 0052's steps 1–3 (F71 P0-9),
// which is the version to port from: unlike the probe script it carries the
// tie-break-toward-agreement rule and the entity-exists guard.
//
// The claimant set is the spine's one owner UNION every memo holder, and pairs are taken
// across that whole set (ADR-107 D1). A memo-to-spine join is NOT sufficient: the live
// library has one studio id claimed by THREE studios, and a join anchored on the spine
// owner emits the two pairs that touch it while silently dropping the memo-to-memo pair
// between the other two — a duplicate the owner could never reach. Both halves of the union
// are load-bearing: some ids have a spine owner holding no memo (invisible to a memo
// self-join) and some have two memo holders and no spine row at all (invisible to a
// spine-anchored join).
//
// ADR-107 D6's reading rules, in order:
//  1. a non-empty external_id — the whole filename-extract population memoizes the empty
//     string by design (internal/extract/store.go).
//  2. newest fetched_at per (entity_type, entity_id, provider) — a re-enrich that wrote a
//     NARROWER field set leaves older rows behind, so only the newest memo counts. Ties
//     break toward the memo that AGREES with the spine, so a tie yields no finding.
//  3. person / studio / film only (rule 3 / D5) — video and tag never enter. Enforced
//     TWICE, independently: the explicit IN list on the memo scan, and the per-kind
//     entity-exists guard below, which enumerates exactly those three kinds and so drops a
//     video or tag row for having no branch to match. Mutation-checked — either clause
//     alone still excludes both kinds, and removing BOTH leaks exactly the video pair and
//     the tag pair the test names. Keep both anyway: the IN list is the statement of the
//     rule and it keeps the correlated subquery off the video memos, which are the bulk of
//     entity_enrichment, while the exists guard is what makes the exclusion structural.
//  4. entity_keep_separate — applied at the write, in queueSharedExternalIDPair, so a
//     dismissed pair is excluded identically by both producers.
//
// claimant is MATERIALIZED because it is self-joined: without the hint SQLite may re-run
// the whole memo chain underneath it twice.
//
// GROUP BY, not SELECT DISTINCT: one pair can collide on TWO providers (the probe's fixture
// has such a case), and the queue stores a pair once. min() over the namespace picks one
// deterministically for the row's chip rather than letting insertion order decide which
// provider gets cited.
const sharedExternalIDPairsSQL = `
WITH memo_winner AS (
    SELECT e.entity_type, e.entity_id,
           (SELECT f.external_id
              FROM entity_enrichment f
             WHERE f.entity_type = e.entity_type
               AND f.entity_id   = e.entity_id
               AND f.provider    = e.provider
               AND f.external_id <> ''
             ORDER BY f.fetched_at DESC,
                      EXISTS (SELECT 1 FROM entity_external_ids x
                               WHERE x.entity_type = f.entity_type
                                 AND x.external_id = f.external_id
                                 AND x.entity_id   = f.entity_id) DESC,
                      f.external_id
             LIMIT 1) AS external_id
      FROM entity_enrichment e
     WHERE e.entity_type IN ('person', 'studio', 'film')
       AND e.external_id <> ''
     GROUP BY e.entity_type, e.entity_id, e.provider
),
memo AS (
    SELECT DISTINCT w.entity_type, w.entity_id, w.external_id
      FROM memo_winner w
     WHERE instr(w.external_id, ':') > 1
       AND instr(w.external_id, ':') < length(w.external_id)
       AND ((w.entity_type = 'person' AND EXISTS (SELECT 1 FROM people  p WHERE p.id = w.entity_id))
         OR (w.entity_type = 'studio' AND EXISTS (SELECT 1 FROM studios s WHERE s.id = w.entity_id))
         OR (w.entity_type = 'film'   AND EXISTS (SELECT 1 FROM films   f WHERE f.id = w.entity_id)))
),
claimant AS MATERIALIZED (
    SELECT DISTINCT entity_type, external_id, entity_id FROM (
        SELECT entity_type, external_id, entity_id FROM entity_external_ids
         WHERE entity_type IN ('person', 'studio', 'film')
        UNION ALL
        SELECT entity_type, external_id, entity_id FROM memo)
)
SELECT a.entity_type, a.entity_id, b.entity_id,
       min(substr(a.external_id, 1, instr(a.external_id, ':') - 1)) AS provider
  FROM claimant a
  JOIN claimant b ON a.entity_type = b.entity_type
                 AND a.external_id = b.external_id
                 AND a.entity_id   < b.entity_id
 GROUP BY a.entity_type, a.entity_id, b.entity_id
 ORDER BY a.entity_type, a.entity_id, b.entity_id`

// SweepSharedExternalIDs reconciles the whole library: every pair of entities of one kind
// that both claim a provider external id becomes an identity_review_queue row, and it
// returns how many rows it actually wrote — new pairs plus pairs upgraded from a weaker
// variation. Re-running an unchanged library returns 0.
//
// Not gated on HasSuccessfulJobRun, unlike SeedIdentityReviewQueue (spec RD5). That gate
// exists because the F43 near-miss seed was a one-time historical normalization; this is a
// cheap idempotent reconciliation whose INPUT keeps changing — every enrich can add a memo.
// Migration 0052 drained the historical backlog once; this keeps it drained.
//
// It never merges and it honors keep-separate, exactly as the write-time guard does: both
// go through queueSharedExternalIDPair.
//
// Pairs are read into memory before any write. The set is tiny (17 on the live library) and
// it keeps a read cursor from being held open across the writes on another pooled
// connection.
func (r *Repo) SweepSharedExternalIDs(ctx context.Context) (int64, error) {
	type pair struct {
		entityType string
		a, b       int64
		provider   string
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	rows, err := r.db.QueryContext(ctx, sharedExternalIDPairsSQL)
	if err != nil {
		return 0, fmt.Errorf("detect shared external ids: %w", err)
	}
	var pairs []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.entityType, &p.a, &p.b, &p.provider); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan shared external id pair: %w", err)
		}
		pairs = append(pairs, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("detect shared external ids: %w", err)
	}

	var written int64
	for _, p := range pairs {
		n, err := queueSharedExternalIDPair(ctx, r.db, p.entityType, p.a, p.b, p.provider)
		if err != nil {
			return written, err
		}
		written += n
	}
	return written, nil
}
