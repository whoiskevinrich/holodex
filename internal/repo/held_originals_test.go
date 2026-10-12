package repo_test

import (
	"context"
	"testing"
	"time"

	"holodex/internal/repo"
)

func TestHeldOriginals_JournalLifecycle(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	v := seedLaneVideo(t, r, "a.mkv")
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

	if h, err := r.GetHeldOriginal(ctx, v); err != nil || h != nil {
		t.Fatalf("fresh video has a row: %+v %v", h, err)
	}
	if err := r.BeginSwap(ctx, v, "/m/a.mkv.holodex-orig", "hflip", now, now.Add(7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := r.BeginSwap(ctx, v, "/m/a.mkv.holodex-orig", "hflip", now, now); err == nil {
		t.Error("a second BeginSwap for the same video succeeded")
	}
	swapping, _ := r.ListHeldOriginals(ctx, repo.HeldSwapping)
	if len(swapping) != 1 || swapping[0].VideoID != v {
		t.Fatalf("swapping rows = %+v", swapping)
	}

	if err := r.SetHeldState(ctx, v, repo.HeldHeld); err != nil {
		t.Fatal(err)
	}
	h, err := r.GetHeldOriginal(ctx, v)
	if err != nil || h == nil {
		t.Fatalf("get: %+v %v", h, err)
	}
	if h.State != repo.HeldHeld || h.Op != "hflip" || h.HeldPath != "/m/a.mkv.holodex-orig" ||
		!h.CreatedAt.Equal(now) || !h.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Errorf("row = %+v", h)
	}

	if err := r.DeleteHeldOriginal(ctx, v); err != nil {
		t.Fatal(err)
	}
	if err := r.SetHeldState(ctx, v, repo.HeldHeld); err == nil {
		t.Error("SetHeldState on a missing row reported success")
	}
	if err := r.DeleteHeldOriginal(ctx, v); err != nil {
		t.Errorf("deleting a missing row: %v", err)
	}
}
