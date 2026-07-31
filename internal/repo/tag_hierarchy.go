package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"holodex/internal/model"
)

// Tag hierarchy (F50, ADR-075 D1): a strict one-parent tree over
// tags.parent_tag_id (migration 0032). No closure table, no materialized
// path -- a nullable self-reference is the least structure that expresses a
// strict tree, and descendant expansion runs as a query-time WITH RECURSIVE
// (below), matching F43's own precedent of resolving aliases at query time
// rather than flattening them into a cache.

// ErrTagCycle is returned by SetTagParent when the proposed parent is the tag
// itself, or a descendant of it -- either would make the tag its own ancestor.
var ErrTagCycle = errors.New("tag: parent would create a cycle")

// tagSubtreeQuery selects a tag id plus every descendant reachable by walking
// down parent_tag_id (children, grandchildren, ...), given the root id as the
// query's one parameter. This is the single query both the cycle-guard
// (isTagDescendant, below) and descendant-inclusive tag filtering
// (VideoFilter.build, repo.go) are built from, so "descendant" means the same
// thing in both places.
const tagSubtreeQuery = `
	WITH RECURSIVE tag_subtree(id) AS (
		SELECT ?
		UNION ALL
		SELECT t.id FROM tags t JOIN tag_subtree s ON t.parent_tag_id = s.id
	)
	SELECT id FROM tag_subtree`

// isTagDescendant reports whether candidateID is rootID itself or appears
// anywhere in rootID's subtree.
func isTagDescendant(ctx context.Context, db *sql.DB, rootID, candidateID int64) (bool, error) {
	var x int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM (`+tagSubtreeQuery+`) WHERE id = ?`, rootID, candidateID).Scan(&x)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("check tag descendant: %w", err)
	}
}

// videoTagAncestorsQuery expands a video's attached tags to include every
// ancestor (walking UP parent_tag_id from each attached tag to the root) — the
// upward counterpart to tagSubtreeQuery's downward descendant expansion, used
// by genre writeback (F50 P0-10, ADR-075 RD9): a video tagged "German Shepherd"
// (child of "Dog", child of "Animal") writes back all three names, not just
// the leaf.
//
// The recursive walk itself climbs through every tag regardless of its own
// writeback_enabled flag (HOLODEX-239, ADR-077 D1) — a disabled tag still
// contributes its ancestors to the expansion. Only the final projection's
// WHERE drops a disabled tag's own name from the result: exclusion is a
// property of each name in the output, not of the path that reached it, so
// disabling "Dog" removes "Dog" but not "Animal" (a further ancestor) or
// "German Shepherd" (a descendant, reached independently as its own
// directly-attached row with its own flag).
const videoTagAncestorsQuery = `
	WITH RECURSIVE tag_ancestors(id) AS (
		SELECT tag_id FROM video_tags WHERE video_id = ?
		UNION
		SELECT t.parent_tag_id FROM tags t
		JOIN tag_ancestors a ON t.id = a.id
		WHERE t.parent_tag_id IS NOT NULL
	)
	SELECT DISTINCT t.name FROM tags t JOIN tag_ancestors a ON t.id = a.id
	WHERE t.writeback_enabled = 1`

// tagAncestorsQuery walks UP parent_tag_id from a single tag id (excluding
// itself) to the root, tracking depth so the final SELECT can order root-first
// — the breadcrumb order (P1-3, ADR-075 D1) — rather than leaf-first, which is
// the order the recursion itself discovers rows in.
const tagAncestorsQuery = `
	WITH RECURSIVE tag_ancestors(id, depth) AS (
		SELECT parent_tag_id, 1 FROM tags WHERE id = ?
		UNION ALL
		SELECT t.parent_tag_id, a.depth + 1
		FROM tags t JOIN tag_ancestors a ON t.id = a.id
		WHERE t.parent_tag_id IS NOT NULL
	)
	SELECT t.name FROM tag_ancestors a JOIN tags t ON t.id = a.id
	ORDER BY a.depth DESC`

// AncestorNamesForTag returns id's ancestor chain, root-first (e.g. "Animal",
// "Mammal", "Dog" for a tag "GermanShepherd") — the tag-detail breadcrumb
// (P1-3). A root tag (no parent) returns an empty, non-nil slice.
func (r *Repo) AncestorNamesForTag(ctx context.Context, id int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, tagAncestorsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("ancestor names for tag: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// TagNamesForVideo returns videoID's attached tags' names, ancestor-expanded
// (F50 P0-10) — the tag-side input to genre writeback's value union
// (genreWritebackValues, internal/api).
func (r *Repo) TagNamesForVideo(ctx context.Context, videoID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, videoTagAncestorsQuery, videoID)
	if err != nil {
		return nil, fmt.Errorf("tag names for video: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// SetTagParent sets id's parent to parentID (nil clears it to root), rejecting
// a cycle with ErrTagCycle: parentID equal to id, or parentID already a
// descendant of id (the tag being reparented would become its own ancestor).
// Returns the updated tag. ErrNotFound if id or a non-nil parentID doesn't
// exist.
func (r *Repo) SetTagParent(ctx context.Context, id int64, parentID *int64) (*model.Tag, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if err := r.EntityExists(ctx, model.EntityTag, id); err != nil {
		return nil, err
	}
	if parentID != nil {
		if err := r.EntityExists(ctx, model.EntityTag, *parentID); err != nil {
			return nil, err
		}
		// tagSubtreeQuery is self-inclusive, so this also catches parentID == id
		// (a tag is trivially "descendant" of itself) without a separate check.
		descendant, err := isTagDescendant(ctx, r.db, id, *parentID)
		if err != nil {
			return nil, err
		}
		if descendant {
			return nil, ErrTagCycle
		}
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE tags SET parent_tag_id = ? WHERE id = ?`, parentID, id); err != nil {
		return nil, fmt.Errorf("set tag parent: %w", err)
	}
	return r.GetTag(ctx, id)
}

// SetTagWritebackEnabled sets one tag's Genre-writeback participation flag
// (HOLODEX-239, ADR-077 D1/D2). Never enqueues a write itself — the spec's
// P0 requirement that changing the flag alone never triggers a file write.
// Returns the updated tag, or ErrNotFound.
func (r *Repo) SetTagWritebackEnabled(ctx context.Context, id int64, enabled bool) (*model.Tag, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if err := r.EntityExists(ctx, model.EntityTag, id); err != nil {
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE tags SET writeback_enabled = ? WHERE id = ?`, enabled, id); err != nil {
		return nil, fmt.Errorf("set tag writeback enabled: %w", err)
	}
	return r.GetTag(ctx, id)
}

// SetTagsWritebackEnabled bulk-sets the writeback flag for every tag in ids
// to enabled, ignoring each tag's individual prior state on the way in
// (spec P0: "applies... regardless of individual prior state"). Never
// enqueues a write. A nil/empty ids is a no-op success.
func (r *Repo) SetTagsWritebackEnabled(ctx context.Context, ids []int64, enabled bool) error {
	if len(ids) == 0 {
		return nil
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	args := make([]any, 0, len(ids)+1)
	args = append(args, enabled)
	args = append(args, toAnySlice(ids)...)
	if _, err := r.db.ExecContext(ctx,
		`UPDATE tags SET writeback_enabled = ? WHERE id IN (`+placeholders(len(ids))+`)`, args...); err != nil {
		return fmt.Errorf("set tags writeback enabled: %w", err)
	}
	return nil
}

// videoIDsForTagsQuery selects the distinct, active/non-deleted video ids
// currently carrying any tag id in the placeholder set — the union of
// videos across one or more tags (HOLODEX-239, ADR-077 D2), mirroring
// namedCountQuery's own active/non-deleted join predicate (repo.go).
func videoIDsForTagsQuery(n int) string {
	return `SELECT DISTINCT vt.video_id FROM video_tags vt
		JOIN videos v ON v.id = vt.video_id AND v.active = 1 AND v.deleted_at IS NULL
		WHERE vt.tag_id IN (` + placeholders(n) + `)`
}

// VideoIDsForTag returns the active/non-deleted video ids currently carrying
// tag id — the manual sync trigger's single-tag scope (ADR-077 D2).
func (r *Repo) VideoIDsForTag(ctx context.Context, id int64) ([]int64, error) {
	return r.VideoIDsForTags(ctx, []int64{id})
}

// VideoIDsForTags returns the deduplicated union of active/non-deleted video
// ids across every tag in ids — the bulk manual sync trigger's scope
// (ADR-077 D2), so a video attached to two selected tags is returned once.
func (r *Repo) VideoIDsForTags(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, videoIDsForTagsQuery(len(ids)), toAnySlice(ids)...)
	if err != nil {
		return nil, fmt.Errorf("video ids for tags: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var videoID int64
		if err := rows.Scan(&videoID); err != nil {
			return nil, err
		}
		out = append(out, videoID)
	}
	return out, rows.Err()
}
