package repo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"holodex/internal/repo"
)

// TestAttachTagToVideo covers P0-7 (F50, ADR-075): resolve-or-create by name,
// source='manual', idempotent re-attach, deny-list 422 semantics (ErrTagDenied),
// the item-11 length cap (ErrTagNameTooLong), and video not-found.
func TestAttachTagToVideo(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	tag, err := r.AttachTagToVideo(ctx, vid, "Documentary")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if tag.Name != "Documentary" {
		t.Errorf("tag.Name = %q, want Documentary", tag.Name)
	}
	v, _, err := r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if len(v.Tags) != 1 || v.Tags[0].Source != "manual" {
		t.Fatalf("video.Tags = %+v, want one tag with source=manual", v.Tags)
	}

	// Idempotent: re-attaching the same (case/whitespace-variant) name is a no-op,
	// not a second link or an error.
	if _, err := r.AttachTagToVideo(ctx, vid, "  documentary "); err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	v, _, err = r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video after re-attach: %v", err)
	}
	if len(v.Tags) != 1 {
		t.Errorf("video.Tags after re-attach = %+v, want still one", v.Tags)
	}

	// Deny-list: attaching a denied term is refused, not silently created.
	if err := r.DenyTag(ctx, "Gnome"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if _, err := r.AttachTagToVideo(ctx, vid, "Gnome"); !errors.Is(err, repo.ErrTagDenied) {
		t.Errorf("attach denied term = %v, want ErrTagDenied", err)
	}

	// Length cap (ADR-075 item 11): the same choke point resolveOrCreateByName
	// enforces for every tag-creation path.
	if _, err := r.AttachTagToVideo(ctx, vid, strings.Repeat("a", 201)); !errors.Is(err, repo.ErrTagNameTooLong) {
		t.Errorf("attach over-long name = %v, want ErrTagNameTooLong", err)
	}

	// Video not found.
	if _, err := r.AttachTagToVideo(ctx, 999999, "Whatever"); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("attach to unknown video = %v, want ErrNotFound", err)
	}
}

// TestDetachTagFromVideo covers P0-7's DELETE side: removes the link, and reports
// ErrNotFound (not a silent no-op) when the tag isn't currently attached.
func TestDetachTagFromVideo(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", nil, []string{"Action"}), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	tagID := tagIDByName(t, r, "Action")

	if err := r.DetachTagFromVideo(ctx, vid, tagID); err != nil {
		t.Fatalf("detach: %v", err)
	}
	v, _, err := r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if len(v.Tags) != 0 {
		t.Errorf("video.Tags after detach = %+v, want none", v.Tags)
	}

	// Not attached (already removed, or never was): ErrNotFound, not a no-op.
	if err := r.DetachTagFromVideo(ctx, vid, tagID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("detach again = %v, want ErrNotFound", err)
	}
	if err := r.DetachTagFromVideo(ctx, 999999, tagID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("detach unknown video = %v, want ErrNotFound", err)
	}
}
