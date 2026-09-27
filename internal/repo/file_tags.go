package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"holodex/internal/model"
)

// recordFileTags stores the tag names the extractor read from the file as
// videos.file_tags (ADR-111 D1) — every name, including the ones
// replaceAssociations skips (denied, oversized, category-colliding): those are
// still on the file, and are exactly what the next genres write drops. Runs in
// UpsertVideo's transaction, so the scan, the refresh path and the post-write
// re-extract all keep it current. A file with no tags records "[]", never NULL
// (NULL means "never read", ADR-111).
func recordFileTags(ctx context.Context, tx *sql.Tx, videoID int64, tags []model.Tag) error {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	raw, err := json.Marshal(names)
	if err != nil {
		return fmt.Errorf("encode file tags: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE videos SET file_tags = ? WHERE id = ?`, string(raw), videoID); err != nil {
		return fmt.Errorf("record file tags: %w", err)
	}
	return nil
}

// FileTags returns the file's tag names as last read (ADR-111 D1). known is
// false when the video has not been read since file_tags shipped — the caller
// must then report "unknown", never "not on the file".
func (r *Repo) FileTags(ctx context.Context, videoID int64) (names []string, known bool, err error) {
	var raw sql.NullString
	switch err := r.db.QueryRowContext(ctx, `SELECT file_tags FROM videos WHERE id = ?`, videoID).Scan(&raw); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, false, ErrNotFound
	case err != nil:
		return nil, false, fmt.Errorf("file tags: %w", err)
	}
	if !raw.Valid {
		return nil, false, nil
	}
	if err := json.Unmarshal([]byte(raw.String), &names); err != nil {
		return nil, false, fmt.Errorf("decode file tags: %w", err)
	}
	return names, true, nil
}

// TagNameKey mirrors the tag name key SQL (nameKeyExpr for tags: lowercased,
// trimmed, spaces removed), so equal-keyed names match without a lookup.
func TagNameKey(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "")
}

// TagIdentityKeys maps each name's TagNameKey to its tag identity: "id:<n>" when
// the name resolves to a tag through the name-identity spine (canonical name,
// then alias — so a merged-away name lands on its survivor), else "key:<name
// key>" (a denied term, say, resolves to nothing). Two names are the same tag
// iff their identities are equal. The one membership rule shared by the
// genres writer's extra-key filter and the on-file read-back (ADR-111 D2), so
// "kept on write" and "on the file" can never disagree.
func (r *Repo) TagIdentityKeys(ctx context.Context, names []string) (map[string]string, error) {
	out := make(map[string]string, len(names))
	for _, n := range names {
		k := TagNameKey(n)
		if _, done := out[k]; done || k == "" {
			continue
		}
		id, ok, err := r.LookupEntityIDByName(ctx, model.EntityTag, n)
		if err != nil {
			return nil, err
		}
		if ok {
			out[k] = "id:" + strconv.FormatInt(id, 10)
		} else {
			out[k] = "key:" + k
		}
	}
	return out, nil
}
