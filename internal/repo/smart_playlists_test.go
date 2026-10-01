package repo_test

import (
	"context"
	"database/sql"
	"net/url"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// testing-strategy §23.2: the repo half of smart playlists (ADR-121 D1/D2/D5).

const oldStamp = "2000-01-01T00:00:00.000Z"

// smartRow creates a playlist (smart when query is non-nil) and backdates its updated_at,
// so a later write to it is visible.
func smartRow(t *testing.T, r *repo.Repo, sqlDB *sql.DB, query *string) int64 {
	t.Helper()
	p, err := r.CreatePlaylist(context.Background(), "pl", "added_desc", model.PlaylistPrivate, query, nil)
	if err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	if _, err := sqlDB.Exec(`UPDATE playlists SET updated_at = ? WHERE id = ?`, oldStamp, p.ID); err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func strp(s string) *string { return &s }

type storedRow struct {
	query     sql.NullString
	updatedAt string
}

func readRow(t *testing.T, sqlDB *sql.DB, id int64) storedRow {
	t.Helper()
	var row storedRow
	if err := sqlDB.QueryRow(`SELECT query, updated_at FROM playlists WHERE id = ?`, id).Scan(&row.query, &row.updatedAt); err != nil {
		t.Fatalf("read playlist %d: %v", id, err)
	}
	return row
}

// assertNullAgreement: query is NULL iff query_version is NULL, on every row (D1/D2).
func assertNullAgreement(t *testing.T, sqlDB *sql.DB, after string) {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM playlists WHERE (query IS NULL) <> (query_version IS NULL)`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("after %s: %d playlists where query and query_version disagree on NULL", after, n)
	}
}

// assertCanonical: a stored query re-normalises to itself (still canonical after a rewrite).
func assertCanonical(t *testing.T, stored string) {
	t.Helper()
	q, err := url.ParseQuery(stored)
	if err != nil {
		t.Fatalf("stored %q doesn't parse: %v", stored, err)
	}
	if again := repo.EncodePlaylistQuery(q); again != stored {
		t.Errorf("stored %q is not canonical (re-encodes to %q)", stored, again)
	}
}

// TestRewriteSmartPlaylistRefs_PersonMerge covers §23.2's rewriteSmartPlaylistRefs table
// through the person merge entry point: the merged id follows to the survivor and
// de-duplicates, the rewrite parses rather than string-replaces (person=12 survives a
// merge of person 1), it is scoped to the merged entity's key, and only changed rows
// are written.
func TestRewriteSmartPlaylistRefs_PersonMerge(t *testing.T) {
	r, sqlDB := newRepoDB(t)
	ctx := context.Background()
	if _, err := sqlDB.Exec(`INSERT INTO people (id, name) VALUES (1, 'Person One'), (5, 'Person Five'),
		(12, 'Person Twelve'), (40, 'Person Forty')`); err != nil {
		t.Fatalf("seed people: %v", err)
	}

	one := smartRow(t, r, sqlDB, strp("person=1"))
	twelve := smartRow(t, r, sqlDB, strp("person=12"))               // a naive replace of "person=1" corrupts it
	dedup := smartRow(t, r, sqlDB, strp("person=1&person=40"))       // collapses to the survivor once
	resort := smartRow(t, r, sqlDB, strp("person=1&person=5"))       // 40 must sort after 5
	tags := smartRow(t, r, sqlDB, strp("q=person%3D1&tag=1&tag=12")) // other keys, same digits
	snapshot := smartRow(t, r, sqlDB, nil)
	assertNullAgreement(t, sqlDB, "create")

	if _, err := r.MergePersonsWithAffectedVideos(ctx, 40, 1); err != nil {
		t.Fatalf("merge 1 → 40: %v", err)
	}
	assertNullAgreement(t, sqlDB, "merge")
	for id, want := range map[int64]string{
		one: "person=40", twelve: "person=12", dedup: "person=40", resort: "person=5&person=40",
		tags: "q=person%3D1&tag=1&tag=12",
	} {
		got := readRow(t, sqlDB, id).query
		if !got.Valid || got.String != want {
			t.Errorf("playlist %d after merging person 1: query %q, want %q", id, got.String, want)
		}
		assertCanonical(t, got.String)
	}
	for _, id := range []int64{twelve, tags, snapshot} {
		if row := readRow(t, sqlDB, id); row.updatedAt != oldStamp {
			t.Errorf("untouched playlist %d: updated_at %q, want it left at %q", id, row.updatedAt, oldStamp)
		}
	}
	if q := readRow(t, sqlDB, snapshot).query; q.Valid {
		t.Errorf("snapshot playlist gained a query %q", q.String)
	}

	// A person 12 merge rewrites person=12 and leaves tag=12 alone.
	if _, err := r.MergePersonsWithAffectedVideos(ctx, 40, 12); err != nil {
		t.Fatalf("merge 12 → 40: %v", err)
	}
	if got := readRow(t, sqlDB, twelve).query.String; got != "person=40" {
		t.Errorf("after merging person 12: %q, want person=40", got)
	}
	if got := readRow(t, sqlDB, tags).query.String; got != "q=person%3D1&tag=1&tag=12" {
		t.Errorf("tag query after a person 12 merge: %q, want untouched", got)
	}
	assertNullAgreement(t, sqlDB, "second merge")
}

// TestRewriteSmartPlaylistRefs_StudioMerge covers §23.2's studio merge path
// (MergeEntitiesWithAffectedVideos): studio_id follows the survivor, and a studio=<name>
// mapped value is not an id and is left alone.
func TestRewriteSmartPlaylistRefs_StudioMerge(t *testing.T) {
	r, sqlDB := newRepoDB(t)
	ctx := context.Background()
	if _, err := sqlDB.Exec(`INSERT INTO studios (id, name) VALUES (7, 'Acme'), (70, 'Acme Pictures')`); err != nil {
		t.Fatalf("seed studios: %v", err)
	}
	byID := smartRow(t, r, sqlDB, strp("studio_id=7&tag=7"))
	byName := smartRow(t, r, sqlDB, strp("studio=Acme"))

	if _, err := r.MergeEntitiesWithAffectedVideos(ctx, model.EnrichEntityStudio, 70, 7); err != nil {
		t.Fatalf("studio merge: %v", err)
	}
	if got := readRow(t, sqlDB, byID).query.String; got != "studio_id=70&tag=7" {
		t.Errorf("after studio merge: %q, want studio_id=70&tag=7", got)
	}
	if row := readRow(t, sqlDB, byName); row.query.String != "studio=Acme" || row.updatedAt != oldStamp {
		t.Errorf("studio=<name> playlist: query %q updated_at %q, want untouched", row.query.String, row.updatedAt)
	}
	assertNullAgreement(t, sqlDB, "studio merge")
}

// TestFreezePlaylist covers §23.2 Freeze (D1): the given order becomes the membership with
// position = rank, query and query_version are nulled together, play_shuffled is kept,
// and a second freeze is refused rather than appending the set again.
func TestFreezePlaylist(t *testing.T) {
	r, sqlDB := newRepoDB(t)
	ctx := context.Background()
	var vids []int64
	for _, title := range []string{"One", "Two", "Three"} {
		id, err := r.UpsertVideo(ctx, sampleVideo("/m/freeze-"+title+".mkv", title, nil, nil), nil)
		if err != nil {
			t.Fatal(err)
		}
		vids = append(vids, id)
	}
	id := smartRow(t, r, sqlDB, strp("q=x"))
	shuffled := true
	if err := r.UpdatePlaylist(ctx, id, repo.PlaylistPatch{PlayShuffled: &shuffled, Query: strp("q=y")}); err != nil {
		t.Fatal(err)
	}
	assertNullAgreement(t, sqlDB, "update")

	order := []int64{vids[2], vids[0], vids[1]}
	if err := r.FreezePlaylist(ctx, id, order); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	assertNullAgreement(t, sqlDB, "freeze")
	var query sql.NullString
	var version sql.NullInt64
	var playShuffled int
	if err := sqlDB.QueryRow(`SELECT query, query_version, play_shuffled FROM playlists WHERE id = ?`, id).
		Scan(&query, &version, &playShuffled); err != nil {
		t.Fatal(err)
	}
	if query.Valid || version.Valid || playShuffled != 1 {
		t.Errorf("after freeze: query %v version %v play_shuffled %d, want NULL NULL 1", query, version, playShuffled)
	}
	rows, err := sqlDB.Query(`SELECT video_id, position FROM playlist_videos WHERE playlist_id = ? ORDER BY position`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var vid int64
		var pos int
		if err := rows.Scan(&vid, &pos); err != nil {
			t.Fatal(err)
		}
		if i >= len(order) || vid != order[i] || pos != i+1 {
			t.Errorf("member %d: video %d at position %d, want video %v at %d", i, vid, pos, order, i+1)
		}
		i++
	}
	if i != len(order) {
		t.Errorf("%d members, want %d", i, len(order))
	}
	p, err := r.GetPlaylist(ctx, id, false)
	if err != nil || p.Smart() || p.QueryVersion != nil || !p.PlayShuffled {
		t.Errorf("GetPlaylist after freeze: %+v %v", p, err)
	}
	if err := r.FreezePlaylist(ctx, id, order); err != repo.ErrNotSmart {
		t.Errorf("second freeze: %v, want ErrNotSmart", err)
	}
}
