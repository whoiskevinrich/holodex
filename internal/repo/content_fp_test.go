package repo_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"holodex/internal/model"
)

func contentFP(t *testing.T, database *sql.DB, id int64) sql.NullString {
	t.Helper()
	var fp sql.NullString
	if err := database.QueryRow(`SELECT content_fp FROM videos WHERE id = ?`, id).Scan(&fp); err != nil {
		t.Fatal(err)
	}
	return fp
}

// UpsertVideo writes the fingerprint on insert and replaces it on update, and an
// empty fingerprint is stored as NULL (unknown), never as '' (F79).
func TestUpsertVideoStoresContentFingerprint(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	v := &model.Video{FilePath: "/m/a.mp4", Title: "a", FileSize: 10,
		FileMtime: time.Now().UTC().Truncate(time.Second), ContentFP: "fp-1"}
	id, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := contentFP(t, database, id); got.String != "fp-1" {
		t.Fatalf("after insert content_fp = %v, want fp-1", got)
	}
	st, ok, err := r.StatByPath(ctx, v.FilePath)
	if err != nil || !ok || !st.FingerprintKnown {
		t.Fatalf("StatByPath = %+v ok=%v err=%v, want FingerprintKnown", st, ok, err)
	}

	v.ContentFP = "fp-2"
	if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
		t.Fatal(err)
	}
	if got := contentFP(t, database, id); got.String != "fp-2" {
		t.Fatalf("after update content_fp = %v, want fp-2", got)
	}

	v.ContentFP = ""
	if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
		t.Fatal(err)
	}
	if got := contentFP(t, database, id); got.Valid {
		t.Fatalf("empty fingerprint stored as %q, want NULL", got.String)
	}
	if st, _, _ := r.StatByPath(ctx, v.FilePath); st.FingerprintKnown {
		t.Fatal("FingerprintKnown true for a NULL content_fp")
	}
}

// The lazy backfill writes only the fingerprint: indexed_at and the extracted
// facts stay as they were (a backfill is not an update, HOLODEX-567).
func TestSetContentFingerprintTouchesNothingElse(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/a.mp4", Title: "a", FileSize: 10,
		FileMtime: time.Now().UTC().Truncate(time.Second)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	row := func() (indexed string, size int64, title string) {
		if err := database.QueryRow(`SELECT indexed_at, file_size, title FROM videos WHERE id = ?`, id).
			Scan(&indexed, &size, &title); err != nil {
			t.Fatal(err)
		}
		return
	}
	i0, s0, t0 := row()
	if err := r.SetContentFingerprint(ctx, id, "fp-x"); err != nil {
		t.Fatal(err)
	}
	if got := contentFP(t, database, id); got.String != "fp-x" {
		t.Fatalf("content_fp = %v, want fp-x", got)
	}
	if i1, s1, t1 := row(); i1 != i0 || s1 != s0 || t1 != t0 {
		t.Fatalf("row changed: (%s,%d,%s) → (%s,%d,%s)", i0, s0, t0, i1, s1, t1)
	}
}
