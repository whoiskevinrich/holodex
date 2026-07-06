package repo

import (
	"context"
	"fmt"
	"strings"
)

// Near-miss review-queue seed (F43/RD10/P0-7, ADR-061). The pure-case hard pairs
// were auto-folded in migration 0022 (in-SQL, before the nameKey unique indexes);
// this is the other half of the one-time backfill: it detects the residual FUZZY
// near-misses — spelling/punctuation/internal-whitespace variants that are NOT
// identity-equal — and records each as a pair in identity_review_queue for the owner
// to confirm or keep-separate. It NEVER merges a near-miss (the homonym rule): a pair
// is a review candidate, not a decision. The cmd/holodex boot job (ADR-028) wraps this
// in one observable, idempotent job run.

// looseKeyStrip are the characters the collision probe folds — in addition to case
// and whitespace — to form the loose near-miss key. Kept in exact sync with
// scripts/detect_entity_collisions.sql: the probe is the evidence basis for the queue
// (14 hard pairs, ~56 near-misses), so the detector must fold identically or the
// two disagree. Whitespace (" ") is folded first so a name that is all punctuation
// still collapses to the empty key and is skipped (see the lkey <> ” guard).
var looseKeyStrip = []string{
	" ", ".", ",", "'", "-", "–", "—", "&", "(", ")", ":", "!", "?", "/", "’",
}

// looseKeyExpr returns the SQLite expression that folds `col` to the probe's loose
// near-miss key: lower + trim, then strip whitespace and looseKeyStrip punctuation.
// `col` is a trusted literal (a column expression), never user input.
func looseKeyExpr(col string) string {
	expr := fmt.Sprintf("lower(trim(%s))", col)
	for _, ch := range looseKeyStrip {
		expr = fmt.Sprintf("replace(%s, %s, '')", expr, sqlStringLit(ch))
	}
	return expr
}

// sqlStringLit renders s as a single-quoted SQLite string literal (doubling any
// embedded quote). Used only on the fixed looseKeyStrip set — never on user input.
func sqlStringLit(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// seedReviewQueueSQL is built once at init from looseKeyExpr. It walks every entity
// name (canonical ∪ alias, matching the probe's universe), groups them by the loose
// key, and inserts a queue row for each pair of DISTINCT entities whose names collide
// loosely but differ in identity key (hkey) — i.e. a fuzzy near-miss the fold left
// behind. Pairs already asserted keep-separate (RD5) are excluded, and the all-empty
// key is skipped so blank/punctuation-only names don't cluster. INSERT OR IGNORE on
// the (entity_type, id_lo, id_hi) PK makes the whole pass idempotent and safe to
// re-run alongside scan-time flagging (S5). variation classifies the pair: names that
// differ only by whitespace are 'internal-whitespace', otherwise 'punctuation'.
var seedReviewQueueSQL = fmt.Sprintf(`
WITH names(et, eid, nm) AS (
    SELECT 'person', id, name FROM people
    UNION ALL SELECT 'person', entity_id, alias FROM entity_aliases WHERE entity_type = 'person'
    UNION ALL SELECT 'studio', id, name FROM studios
    UNION ALL SELECT 'studio', entity_id, alias FROM entity_aliases WHERE entity_type = 'studio'
    UNION ALL SELECT 'tag', id, name FROM tags
    UNION ALL SELECT 'tag', entity_id, alias FROM entity_aliases WHERE entity_type = 'tag'
),
keyed(et, eid, hkey, lkey, wskey) AS (
    SELECT et, eid,
           CASE WHEN et = 'tag' THEN replace(lower(trim(nm)), ' ', '') ELSE lower(trim(nm)) END,
           %s,
           replace(lower(trim(nm)), ' ', '')
    FROM names
),
pairs(et, id_lo, id_hi, variation) AS (
    SELECT a.et, a.eid, b.eid,
           CASE WHEN a.wskey = b.wskey THEN 'internal-whitespace' ELSE 'punctuation' END
    FROM keyed a
    JOIN keyed b ON a.et = b.et AND a.lkey = b.lkey AND a.eid < b.eid AND a.hkey <> b.hkey
    WHERE a.lkey <> ''
)
INSERT OR IGNORE INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
SELECT p.et, p.id_lo, p.id_hi, min(p.variation)
FROM pairs p
WHERE NOT EXISTS (
    SELECT 1 FROM entity_keep_separate ks
    WHERE ks.entity_type = p.et AND ks.id_lo = p.id_lo AND ks.id_hi = p.id_hi
)
GROUP BY p.et, p.id_lo, p.id_hi`, looseKeyExpr("nm"))

// SeedIdentityReviewQueue detects fuzzy name near-misses across person / studio / tag
// and records each as an identity_review_queue pair, returning the number of NEW pairs
// inserted (INSERT OR IGNORE, so re-running the pass returns 0). It never merges an
// entity — it only writes the queue — and it honors keep-separate (a dismissed pair is
// never re-proposed). Serialized under the single writer (writeMu) like every write.
func (r *Repo) SeedIdentityReviewQueue(ctx context.Context) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx, seedReviewQueueSQL)
	if err != nil {
		return 0, fmt.Errorf("seed identity review queue: %w", err)
	}
	return res.RowsAffected()
}
