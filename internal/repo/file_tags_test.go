package repo_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// TestFileTags_RecordedAtUpsert covers F72 P0-1 (ADR-111 D1): every UpsertVideo records
// the extracted names — a denied one included, since it is still on the file — and a
// row never read since 0054 reports unknown rather than "no tags".
func TestFileTags_RecordedAtUpsert(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	if _, err := r.DenyTag(ctx, "junk"); err != nil {
		t.Fatal(err)
	}
	v := &model.Video{FilePath: "/m/a.mp4", FileSize: 1, Title: "A", FileMtime: time.Now().UTC().Truncate(time.Second),
		Tags: []model.Tag{{Name: "Drama"}, {Name: "junk"}}}
	id, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatal(err)
	}
	names, known, err := r.FileTags(ctx, id)
	if err != nil || !known || !slices.Equal(names, []string{"Drama", "junk"}) {
		t.Fatalf("FileTags = %v known=%v err=%v, want [Drama junk] known", names, known, err)
	}

	v.Tags = nil
	if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
		t.Fatal(err)
	}
	if names, known, _ := r.FileTags(ctx, id); !known || len(names) != 0 {
		t.Errorf("untagged file: FileTags = %v known=%v, want [] known", names, known)
	}

	if _, err := db.Exec(`UPDATE videos SET file_tags = NULL WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, known, err := r.FileTags(ctx, id); known || err != nil {
		t.Errorf("pre-0054 row: known=%v err=%v, want unknown", known, err)
	}
}

// TestTagIdentityKeys: a name, its case/space variant and an alias share one identity;
// an unknown name falls back to its name key (ADR-111 D2).
func TestTagIdentityKeys(t *testing.T) {
	r, _ := newRepoDB(t)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/b.mp4", FileSize: 1, Title: "B", FileMtime: time.Now().UTC()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := r.AttachTagToVideo(ctx, id, "science fiction")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddEntityAlias(ctx, model.EntityTag, tag.ID, "Sci-Fi"); err != nil {
		t.Fatal(err)
	}
	keys, err := r.TagIdentityKeys(ctx, []string{"Science Fiction", "sci-fi", "Unheard Of"})
	if err != nil {
		t.Fatal(err)
	}
	if keys[repo.TagNameKey("Science Fiction")] != keys[repo.TagNameKey("sci-fi")] {
		t.Errorf("alias and canonical differ: %v", keys)
	}
	if got := keys[repo.TagNameKey("Unheard Of")]; got != "key:unheardof" {
		t.Errorf("unknown name identity = %q, want key:unheardof", got)
	}
}
