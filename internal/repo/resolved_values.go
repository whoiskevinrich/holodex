package repo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"holodex/internal/model"
	"holodex/internal/query"
)

// The resolved-value index and the change stamp (F89 P0-6, RD2; field-resolution.md,
// "Winning values are indexed in resolved_values"). The completeness drain rewrites both
// from its one pass, in the transaction that clears the dirty flags.

// ResolvedValue is one winning value of one queryable field.
type ResolvedValue struct {
	FieldKey string
	Norm     string // query.NormValue's form
	Value    string
}

// writeResolvedValues replaces the index rows for ids with the rows' values, and moves
// changed_at for any video whose inputs differ from its stored fingerprint.
func writeResolvedValues(ctx context.Context, tx *sql.Tx, ids []int64, rows []CompletenessRow, now string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM resolved_values WHERE video_id IN (`+placeholders(len(ids))+`)`, toAnySlice(ids)...); err != nil {
		return fmt.Errorf("write resolved values: clear: %w", err)
	}
	for _, row := range rows {
		for _, v := range row.Values {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO resolved_values (video_id, field_key, norm_value, value)
				VALUES (?, ?, ?, ?)`, row.EntityID, v.FieldKey, v.Norm, v.Value); err != nil {
				return fmt.Errorf("write resolved values: insert %d/%s: %w", row.EntityID, v.FieldKey, err)
			}
		}
	}
	sigs, err := videoChangeSigs(ctx, tx, ids)
	if err != nil {
		return err
	}
	for id, sig := range sigs {
		// A first fingerprint moves nothing, and an unchanged one writes nothing.
		if _, err := tx.ExecContext(ctx, `
			UPDATE videos SET
				changed_at = CASE WHEN change_sig IS NULL THEN COALESCE(changed_at, indexed_at) ELSE ? END,
				change_sig = ?
			WHERE id = ? AND change_sig IS NOT ?`, now, sig, id, sig); err != nil {
			return fmt.Errorf("write resolved values: stamp %d: %w", id, err)
		}
	}
	return nil
}

// videoChangeSigs fingerprints what the owner counts as a change made in Holodex (RD2): a
// video's links, enrichment, curation and decisions. Inputs rather than resolved rows, so
// a Tag or a losing provider value counts, while a mapping edit moves nothing. Timestamps
// and internal `_` keys stay out, so a refresh that rewrites identical rows is no change.
// What's read from the file stays out too: a changed file moves changed_at at the upsert,
// and reading more out of an unchanged one is no change (F12.7). Every video gets an entry,
// from the videos row, so a video with no links still takes its first fingerprint.
func videoChangeSigs(ctx context.Context, tx *sql.Tx, ids []int64) (map[int64]string, error) {
	in := " IN (" + placeholders(len(ids)) + ")"
	enrichment := "entity_type = '" + model.EnrichEntityVideo + "' AND entity_id" + in
	parts := []string{
		`SELECT id, '', '', '' FROM videos WHERE id` + in,
		`SELECT video_id, 'tag', tag_id, '' FROM video_tags WHERE video_id` + in,
		`SELECT video_id, 'person', person_id, role FROM video_people WHERE video_id` + in,
		`SELECT video_id, 'studio', studio_id, '' FROM video_studios WHERE video_id` + in,
		`SELECT fv.video_id, 'film', fv.film_id, f.name FROM film_videos fv JOIN films f ON f.id = fv.film_id WHERE fv.video_id` + in,
		`SELECT entity_id, 'enrichment', provider || '/' || field_key, value FROM entity_enrichment WHERE field_key NOT LIKE '\_%' ESCAPE '\' AND ` + enrichment,
		`SELECT entity_id, 'curation', field_key || '/' || action, norm_value FROM metadata_curation WHERE ` + enrichment,
		`SELECT entity_id, 'decision', field_key, source FROM field_source_decisions WHERE ` + enrichment,
	}
	var args []any
	for range parts {
		args = append(args, toAnySlice(ids)...)
	}
	rows, err := tx.QueryContext(ctx, strings.Join(parts, " UNION ALL ")+" ORDER BY 1, 2, 3, 4", args...)
	if err != nil {
		return nil, fmt.Errorf("video change sigs: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]string, len(ids))
	h := sha256.New()
	flush := func(id int64) {
		if id != 0 {
			out[id] = hex.EncodeToString(h.Sum(nil))
			h.Reset()
		}
	}
	var cur int64
	for rows.Next() {
		var id int64
		var kind string
		var a, b sql.NullString
		if err := rows.Scan(&id, &kind, &a, &b); err != nil {
			return nil, err
		}
		if id != cur {
			flush(cur)
			cur = id
		}
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", kind, a.String, b.String)
	}
	flush(cur)
	return out, rows.Err()
}

// resolvedClause is a mapped field's condition (F89 P0-6): the video has a winning value
// that satisfies it. field comes from the catalogue, and is bound regardless (SEC-8).
func resolvedClause(field, kind string, c *query.Cond, now time.Time) (string, []any) {
	test, args := resolvedTest(kind, c, now)
	if test == "" {
		return "0", nil
	}
	return "EXISTS (SELECT 1 FROM resolved_values rv WHERE rv.video_id = v.id AND rv.field_key = ? AND " + test + ")",
		append([]any{field}, args...)
}

func resolvedTest(kind string, c *query.Cond, now time.Time) (string, []any) {
	switch kind {
	case query.KindList:
		norm, ok := query.NormValue(kind, c.Value)
		if !ok {
			return "", nil
		}
		return "rv.norm_value = ?", []any{norm}
	case query.KindText:
		norm, _ := query.NormValue(kind, c.Value)
		words := strings.Fields(norm)
		if len(words) == 0 {
			return "", nil
		}
		tests := make([]string, len(words))
		args := make([]any, len(words))
		for i, w := range words {
			tests[i] = `rv.norm_value LIKE ? ESCAPE '\'`
			args[i] = "%" + likeEscaper.Replace(w) + "%"
		}
		return strings.Join(tests, " AND "), args
	case query.KindDate:
		// A stored date is a calendar day, so the bounds are days in the same zone.
		lo, hi, ok := query.Bounds(c, now, time.Local)
		if !ok {
			return "", nil
		}
		s, args := "1", []any(nil)
		if !lo.IsZero() {
			s, args = s+" AND rv.norm_value >= ?", append(args, lo.Format(time.DateOnly))
		}
		if !hi.IsZero() {
			s, args = s+" AND rv.norm_value < ?", append(args, hi.Format(time.DateOnly))
		}
		return s, args
	case query.KindNumber:
		const num = "rv.norm_value GLOB '[0-9]*' AND CAST(rv.norm_value AS INTEGER) "
		if c.Op == query.OpRange {
			return num + ">= ? AND CAST(rv.norm_value AS INTEGER) <= ?", []any{c.Lo, c.Hi}
		}
		return num + sqlOp(c.Op) + " ?", []any{c.Lo}
	}
	return "", nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// ResolvedFieldValues lists a field's existing values containing text, the most used
// first, for suggestions after `director:` (P0-6). It reads only videos browse would show:
// a row can outlive its video's soft delete until the next drain, and full-film videos are
// indexed but hidden (hideFullFilm, as VideoFilter.HideFullFilmVideos).
func (r *Repo) ResolvedFieldValues(ctx context.Context, field, text string, hideFullFilm bool, limit int) ([]string, error) {
	norm, _ := query.NormValue(query.KindText, text)
	visible := "v.active = 1 AND v.deleted_at IS NULL"
	if hideFullFilm {
		visible += " AND " + hideFullFilmVideosClause
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT MIN(rv.value) FROM resolved_values rv
		JOIN videos v ON v.id = rv.video_id
		WHERE rv.field_key = ? AND rv.norm_value LIKE ? ESCAPE '\' AND `+visible+`
		GROUP BY rv.norm_value
		ORDER BY COUNT(*) DESC, rv.norm_value
		LIMIT ?`, field, "%"+likeEscaper.Replace(norm)+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("resolved field values: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
