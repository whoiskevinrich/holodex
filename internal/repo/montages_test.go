package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

func TestMontageState_UpsertAndAttempted(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", nil, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.MontageState(ctx, a); err != nil || ok {
		t.Fatalf("never attempted: ok=%v err=%v", ok, err)
	}
	if err := r.SetMontageState(ctx, a, "failed", "can't read file"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetMontageState(ctx, a, "generated", ""); err != nil {
		t.Fatal(err)
	}
	row, ok, err := r.MontageState(ctx, a)
	if err != nil || !ok || row.State != "generated" || row.Error != "" {
		t.Fatalf("after upsert = %+v ok=%v err=%v", row, ok, err)
	}
	got, err := r.MontageAttempted(ctx, []int64{a, b})
	if err != nil || !got[a] || got[b] {
		t.Errorf("attempted = %v err=%v; want only %d", got, err, a)
	}

	// Stills are untouched: the montage never writes thumbnail_state (F83 RD5).
	v, _, err := r.GetVideo(ctx, a)
	if err != nil || v.ThumbnailState != model.ThumbnailNone {
		t.Errorf("thumbnail_state = %q err=%v, want untouched", v.ThumbnailState, err)
	}
}

func TestMontageBackfillCandidates(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	mk := func(path string, dur int) int64 {
		t.Helper()
		v := sampleVideo(path, path, nil, nil)
		v.Duration = dur
		id, err := r.UpsertVideo(ctx, v, nil)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	long := mk("/m/long.mkv", 600)
	short := mk("/m/short.mkv", 4)
	done := mk("/m/done.mkv", 600)
	gone := mk("/m/gone.mkv", 600)
	_ = r.SetMontageState(ctx, done, "generated", "")
	if err := r.SoftDelete(ctx, gone); err != nil {
		t.Fatal(err)
	}
	cands, err := r.MontageBackfillCandidates(ctx, 6, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].ID != long {
		t.Errorf("candidates = %+v, want only %d (short=%d done=%d gone=%d excluded)", cands, long, short, done, gone)
	}
	if _, ok, _ := r.MontageCandidateByID(ctx, gone); ok {
		t.Error("a soft-deleted video is still a montage candidate")
	}
}
