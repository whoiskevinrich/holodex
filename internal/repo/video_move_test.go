package repo_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

var moveMtime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// legacyVideo is a file with every fact the legacy rule compares set.
func legacyVideo(path string) *model.Video {
	return &model.Video{FilePath: path, Title: "t", FileSize: 4096, Duration: 61, Width: 1920, Height: 1080,
		VideoCodec: "h264", AudioCodec: "aac", BitrateKbps: 5000, Container: "mp4",
		FileMtime: moveMtime, Tags: []model.Tag{{Name: "beach"}}}
}

func candidateIDs(t *testing.T, r *repo.Repo, v *model.Video) []int64 {
	t.Helper()
	cs, err := r.MoveCandidates(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, c := range cs {
		ids = append(ids, c.ID)
	}
	return ids
}

func TestMoveCandidatesByFingerprint(t *testing.T) {
	r, _ := newRepoDB(t)
	ctx := context.Background()
	old := legacyVideo("/m/a.mp4")
	old.ContentFP = "fp-a"
	id, err := r.UpsertVideo(ctx, old, nil)
	if err != nil {
		t.Fatal(err)
	}

	moved := legacyVideo("/m/b.mp4")
	moved.ContentFP = "fp-a"
	if got := candidateIDs(t, r, moved); len(got) != 1 || got[0] != id {
		t.Fatalf("candidates = %v, want [%d]", got, id)
	}

	// Identical facts but a different fingerprint never match a fingerprinted
	// row: size and stream facts alone are not enough (P0-1).
	other := legacyVideo("/m/c.mp4")
	other.ContentFP = "fp-other"
	if got := candidateIDs(t, r, other); len(got) != 0 {
		t.Fatalf("facts-only twin matched a fingerprinted row: %v", got)
	}
}

func TestMoveCandidatesLegacyExactFacts(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, legacyVideo("/m/a.mp4"), nil) // no fingerprint: a legacy row
	if err != nil {
		t.Fatal(err)
	}

	probe := func(edit func(v *model.Video)) *model.Video {
		v := legacyVideo("/m/b.mp4")
		v.ContentFP = "fp-new"
		edit(v)
		return v
	}
	if got := candidateIDs(t, r, probe(func(*model.Video) {})); len(got) != 1 || got[0] != id {
		t.Fatalf("exact facts: candidates = %v, want [%d]", got, id)
	}

	for name, edit := range map[string]func(v *model.Video){
		"size":      func(v *model.Video) { v.FileSize++ },
		"mtime":     func(v *model.Video) { v.FileMtime = v.FileMtime.Add(time.Second) },
		"duration":  func(v *model.Video) { v.Duration++ },
		"width":     func(v *model.Video) { v.Width = 1280 },
		"height":    func(v *model.Video) { v.Height = 720 },
		"vcodec":    func(v *model.Video) { v.VideoCodec = "hevc" },
		"acodec":    func(v *model.Video) { v.AudioCodec = "opus" },
		"bitrate":   func(v *model.Video) { v.BitrateKbps++ },
		"container": func(v *model.Video) { v.Container = "mkv" },
		"tags":      func(v *model.Video) { v.Tags = []model.Tag{{Name: "forest"}} },
	} {
		t.Run(name, func(t *testing.T) {
			if got := candidateIDs(t, r, probe(edit)); len(got) != 0 {
				t.Fatalf("a %s difference still matched: %v", name, got)
			}
		})
	}

	// A legacy row that never recorded its file tags is compared on the rest.
	if _, err := database.Exec(`UPDATE videos SET file_tags = NULL WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if got := candidateIDs(t, r, probe(func(v *model.Video) { v.Tags = nil })); len(got) != 1 {
		t.Fatalf("unrecorded tags: candidates = %v, want the legacy row", got)
	}
}

func TestMoveCandidatesSkipTrashedRows(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	v := legacyVideo("/m/a.mp4")
	v.ContentFP = "fp-a"
	id, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE videos SET deleted_at = '2026-01-01T00:00:00Z' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	moved := legacyVideo("/m/b.mp4")
	moved.ContentFP = "fp-a"
	if got := candidateIDs(t, r, moved); len(got) != 0 {
		t.Fatalf("trashed row offered as a candidate: %v", got)
	}
	if twin, _ := r.HasSizeTwin(ctx, v.FileSize); twin {
		t.Error("a trashed row counted as a size twin")
	}
}

func TestHasSizeTwin(t *testing.T) {
	r, _ := newRepoDB(t)
	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, legacyVideo("/m/a.mp4"), nil); err != nil {
		t.Fatal(err)
	}
	if twin, err := r.HasSizeTwin(ctx, 4096); err != nil || !twin {
		t.Fatalf("HasSizeTwin(4096) = %v, %v; want true", twin, err)
	}
	if twin, _ := r.HasSizeTwin(ctx, 4097); twin {
		t.Error("HasSizeTwin(4097) = true; want false")
	}
}

// A repoint keeps the id and indexed_at (a move is neither an arrival nor an
// update), takes the new path and facts, and shows the row again (P0-3).
func TestRepointVideoKeepsIdentity(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, legacyVideo("/m/a.mp4"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE videos SET active = 0, indexed_at = '2025-05-05T05:05:05Z' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}

	moved := legacyVideo("/m/sub/b.mp4")
	moved.ContentFP = "fp-b"
	if err := r.RepointVideo(ctx, id, moved, nil); err != nil {
		t.Fatal(err)
	}
	var (
		path, indexed string
		active        bool
		fp            sql.NullString
	)
	if err := database.QueryRow(`SELECT file_path, indexed_at, active, content_fp FROM videos WHERE id = ?`, id).
		Scan(&path, &indexed, &active, &fp); err != nil {
		t.Fatal(err)
	}
	if path != "/m/sub/b.mp4" || !active || fp.String != "fp-b" {
		t.Fatalf("after repoint: path=%s active=%v fp=%v", path, active, fp)
	}
	if indexed != "2025-05-05T05:05:05Z" {
		t.Fatalf("indexed_at changed to %s", indexed)
	}
	if _, ok, _ := r.StatByPath(ctx, "/m/a.mp4"); ok {
		t.Error("old path still resolves")
	}
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM videos`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("row count = %d (%v), want 1", n, err)
	}
}

func TestRepointVideoRefusesTrashedRow(t *testing.T) {
	r, database := newRepoDB(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, legacyVideo("/m/a.mp4"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE videos SET deleted_at = '2026-01-01T00:00:00Z' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if err := r.RepointVideo(ctx, id, legacyVideo("/m/b.mp4"), nil); err == nil {
		t.Fatal("repointed a trashed row")
	}
}
