package repo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"holodex/internal/model"
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
	if _, err := r.DenyTag(ctx, "Gnome"); err != nil {
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

// TestAttachMaterializedTags covers P0-9 (F50, ADR-075 D4): batch attach in one
// transaction, idempotency (re-running the same batch adds no rows), alias
// canonicalization (an aliased value attaches the canonical tag, not a second tag
// under the alias spelling), and the deny-list silent-skip enrichment gets instead of
// manual-attach's surfaced error.
func TestAttachMaterializedTags(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "T", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	if err := r.AttachMaterializedTags(ctx, vid, []repo.MaterializedTag{
		{Name: "Action", Source: "provider:tmdb"},
		{Name: "Comedy", Source: "provider:tmdb"},
	}); err != nil {
		t.Fatalf("attach materialized: %v", err)
	}
	v, _, err := r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if len(v.Tags) != 2 {
		t.Fatalf("video.Tags = %+v, want 2", v.Tags)
	}
	for _, tg := range v.Tags {
		if tg.Source != "provider:tmdb" {
			t.Errorf("tag %q source = %q, want provider:tmdb", tg.Name, tg.Source)
		}
	}

	// Idempotent: re-running the same batch adds no new rows.
	if err := r.AttachMaterializedTags(ctx, vid, []repo.MaterializedTag{
		{Name: "Action", Source: "provider:tmdb"},
		{Name: "Comedy", Source: "provider:tmdb"},
	}); err != nil {
		t.Fatalf("re-attach materialized: %v", err)
	}
	v, _, err = r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video after re-attach: %v", err)
	}
	if len(v.Tags) != 2 {
		t.Errorf("video.Tags after re-attach = %+v, want still 2", v.Tags)
	}

	// Alias canonicalization (the ADR's own "azure" -> "blue" example): a value that
	// aliases to an existing tag attaches that tag, never a second tag under the
	// alias spelling.
	otherVid, err := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "T2", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed second video: %v", err)
	}
	if err := r.AttachMaterializedTags(ctx, otherVid, []repo.MaterializedTag{{Name: "Blue", Source: "provider:tmdb"}}); err != nil {
		t.Fatalf("seed blue tag: %v", err)
	}
	blueID := tagIDByName(t, r, "Blue")
	if _, err := r.AddEntityAlias(ctx, model.EntityTag, blueID, "azure"); err != nil {
		t.Fatalf("add alias: %v", err)
	}
	if err := r.AttachMaterializedTags(ctx, vid, []repo.MaterializedTag{{Name: "azure", Source: "provider:tmdb"}}); err != nil {
		t.Fatalf("attach aliased: %v", err)
	}
	v, _, err = r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video after aliased attach: %v", err)
	}
	if len(v.Tags) != 3 {
		t.Fatalf("video.Tags after aliased attach = %+v, want 3 (Action, Comedy, Blue)", v.Tags)
	}
	var gotBlue bool
	for _, tg := range v.Tags {
		if tg.Name == "azure" {
			t.Errorf("video gained a literal 'azure' tag, want it canonicalized to 'Blue': %+v", v.Tags)
		}
		if tg.Name == "Blue" {
			gotBlue = true
		}
	}
	if !gotBlue {
		t.Errorf("video.Tags = %+v, want to include canonical 'Blue'", v.Tags)
	}

	// Deny-list: unlike AttachTagToVideo, a denied term is silently skipped, not
	// surfaced as an error -- enrichment is unattended, so there is no owner to show
	// a rejection to (ADR-075 D2).
	if _, err := r.DenyTag(ctx, "Gnome"); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if err := r.AttachMaterializedTags(ctx, vid, []repo.MaterializedTag{{Name: "Gnome", Source: "provider:tmdb"}}); err != nil {
		t.Fatalf("attach denied term (should be silently skipped): %v", err)
	}
	v, _, err = r.GetVideo(ctx, vid)
	if err != nil {
		t.Fatalf("get video after denied attach: %v", err)
	}
	if len(v.Tags) != 3 {
		t.Errorf("video.Tags after denied attach = %+v, want still 3 (denied term skipped)", v.Tags)
	}

	// An unknown video is a hard error, not a silent skip -- there is no video to
	// attach anything to (the FK on video_tags.video_id rejects it).
	if err := r.AttachMaterializedTags(ctx, 999999, []repo.MaterializedTag{{Name: "Whatever", Source: "provider:tmdb"}}); err == nil {
		t.Error("attach to unknown video = nil, want an error")
	}

	// An empty batch is a no-op, not an error.
	if err := r.AttachMaterializedTags(ctx, vid, nil); err != nil {
		t.Errorf("attach empty batch = %v, want nil", err)
	}
}
