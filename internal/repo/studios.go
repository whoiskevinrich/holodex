package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"holodex/internal/model"
)

// Studios (F38, ADR-053). video_studios is a derived index over the resolved
// `studio` field: ReconcileVideoStudios is the SOLE writer of the table, called by
// the Relinker after every write that can change a video's resolved studio value
// (scan/enrich/decision/curation). Reads (list/detail/facet/search) hit the tables
// directly. Reads are lock-free (WAL); writes take writeMu like the rest of the
// write path.

// resolveOrCreateStudio returns the id of the studio for (`name`, `externalID`),
// inserting it if absent. Routes through the shared name-identity spine (F43,
// ADR-061): the provider `externalID` matches FIRST (ADR-054 §4 — so two spellings
// sharing a provider company id converge), then the case/whitespace-folded nameKey
// over canonical names and aliases (so "fox"/"Fox" converge and a merged-away studio
// name survives RelinkVideoStudios re-derivation), then create. `externalID` is
// namespace-qualified ("tmdb:174") or empty. Any id in hand is attached to the
// resolved studio. Runs inside the caller's transaction; the nameKey unique index +
// studio_external_ids PK + writeMu serialization make the select-then-insert race-free.
func resolveOrCreateStudio(ctx context.Context, tx *sql.Tx, name, externalID string) (int64, error) {
	return resolveOrCreateByName(ctx, tx, model.EnrichEntityStudio, name, externalID)
}

// ReconcileVideoStudios makes video_studios for one video hold exactly the studios
// named in `names` (the video's resolved studio value(s); empty/duplicate names are
// dropped). It resolves-or-creates each name, inserts the missing links, deletes the
// stale ones, and prunes any studio left with zero links (prune-on-empty, ADR-053
// §2 step 4 — what keeps a derived-identity studio honest without alias routing).
// One write transaction under writeMu; idempotent. Passing nil/empty `names`
// removes all of the video's studio links (and prunes) — the soft-delete/blank-pin
// path. `extIDByName` maps a resolved name → its provider external id (ADR-054), so
// resolve-or-create can de-dup by company id; a name absent from the map (custom or
// id-less) resolves by name only. Pass nil when no ids are known.
func (r *Repo) ReconcileVideoStudios(ctx context.Context, videoID int64, names []string, extIDByName map[string]string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	// Desired set — resolve-or-create each distinct, non-empty name (id-first).
	desired := make(map[int64]struct{}, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		sid, err := resolveOrCreateStudio(ctx, tx, trimmed, extIDByName[trimmed])
		if err != nil {
			return err
		}
		desired[sid] = struct{}{}
	}

	// Current set.
	current := map[int64]struct{}{}
	rows, err := tx.QueryContext(ctx, `SELECT studio_id FROM video_studios WHERE video_id = ?`, videoID)
	if err != nil {
		return fmt.Errorf("current studio links: %w", err)
	}
	for rows.Next() {
		var sid int64
		if err := rows.Scan(&sid); err != nil {
			rows.Close()
			return err
		}
		current[sid] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Insert missing.
	for sid := range desired {
		if _, have := current[sid]; have {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO video_studios (video_id, studio_id) VALUES (?, ?)`, videoID, sid); err != nil {
			return fmt.Errorf("link studio: %w", err)
		}
	}

	// Delete stale, then prune any of those studios left with no links.
	for sid := range current {
		if _, keep := desired[sid]; keep {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM video_studios WHERE video_id = ? AND studio_id = ?`, videoID, sid); err != nil {
			return fmt.Errorf("unlink studio: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM studios WHERE id = ? AND NOT EXISTS
			 (SELECT 1 FROM video_studios WHERE studio_id = ?)`, sid, sid); err != nil {
			return fmt.Errorf("prune studio: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit studio links: %w", err)
	}
	return nil
}

// ListStudios returns every studio with at least one active, non-deleted video,
// with counts. sortByCount orders by video count desc (else name asc). Empty
// studios never appear (prune-on-empty removes them; the INNER JOIN also excludes a
// studio whose only videos are soft-deleted).
func (r *Repo) ListStudios(ctx context.Context, sortByCount bool) ([]model.Studio, error) {
	rows, err := r.db.QueryContext(ctx, namedCountQuery("studios", "video_studios", "studio_id", sortByCount, false))
	if err != nil {
		return nil, fmt.Errorf("list studios: %w", err)
	}
	defer rows.Close()
	var out []model.Studio
	for rows.Next() {
		var s model.Studio
		if err := rows.Scan(&s.ID, &s.Name, &s.VideoCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachStudioImages(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetStudio returns a studio by id with active-video count and its image versions
// (F51, ADR-079), or ErrNotFound.
func (r *Repo) GetStudio(ctx context.Context, id int64) (*model.Studio, error) {
	var s model.Studio
	err := r.db.QueryRowContext(ctx, `
		SELECT s.id, s.name,
		       (SELECT COUNT(*) FROM video_studios vs JOIN videos v ON v.id = vs.video_id
		        WHERE vs.studio_id = s.id AND v.active = 1 AND v.deleted_at IS NULL)
		FROM studios s WHERE s.id = ?`, id).Scan(&s.ID, &s.Name, &s.VideoCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	versions, err := r.studioImageVersions(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	s.ImageVersions = versions[id]
	// Owner-curated aliases (F43, ADR-061) for the detail view.
	if s.Aliases, err = r.AliasesForEntity(ctx, model.EnrichEntityStudio, id); err != nil {
		return nil, err
	}
	return &s, nil
}

// StudioLinkCount returns the total number of video_studios rows — the fast-path
// gate for the one-time startup backfill (ADR-053 §5): once any link exists the
// backfill is skipped. (A library where nothing resolves to a studio keeps a count
// of 0; the backfill uses a job-run marker to avoid re-passing every boot in that
// case — see cmd/holodex backfillStudioLinks.)
func (r *Repo) StudioLinkCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM video_studios`).Scan(&n)
	return n, err
}

// AllActiveVideoIDs returns the ids of every visible video (on disk, not
// soft-deleted) — the input to the one-time studio-link backfill (ADR-053 §5).
func (r *Repo) AllActiveVideoIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM videos WHERE active = 1 AND deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("all active video ids: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// StudiosForVideos returns the studios linked to each of the given videos, keyed by
// video id (F38) — used to attach studios[] to media reads so the resolved studio
// value can link to its entity (ADR-053 RD1: the link always matches the displayed
// value). Videos with no studio link are absent from the map.
func (r *Repo) StudiosForVideos(ctx context.Context, ids []int64) (map[int64][]model.Studio, error) {
	if len(ids) == 0 {
		return map[int64][]model.Studio{}, nil
	}
	q := `SELECT vs.video_id, s.id, s.name
	      FROM video_studios vs JOIN studios s ON s.id = vs.studio_id
	      WHERE vs.video_id IN (` + placeholders(len(ids)) + `)
	      ORDER BY s.name COLLATE NOCASE`
	rows, err := r.db.QueryContext(ctx, q, toAnySlice(ids)...)
	if err != nil {
		return nil, fmt.Errorf("studios for videos: %w", err)
	}
	defer rows.Close()
	out := make(map[int64][]model.Studio, len(ids))
	for rows.Next() {
		var vid int64
		var s model.Studio
		if err := rows.Scan(&vid, &s.ID, &s.Name); err != nil {
			return nil, err
		}
		out[vid] = append(out[vid], s)
	}
	return out, rows.Err()
}
