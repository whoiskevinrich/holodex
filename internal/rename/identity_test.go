package rename

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/montage"
	"holodex/internal/scanner"
	"holodex/internal/thumbnail"
)

type fixedExtractor struct{}

func (fixedExtractor) Extract(context.Context, string) (metadata.Extracted, error) {
	return metadata.Extracted{Title: "file title", Width: 1920, Height: 1080, DurationSec: 60}, nil
}

// R12: a curated video keeps its identity through a rename and a rescan. The same id, every row
// that hangs off it, its thumbnail and montage (files keyed by id, state unchanged), and its place
// in Date added (id) and Updated (indexed_at) order.
func TestRenameKeepsIdentityThroughRescan(t *testing.T) {
	l := newLib(t)
	s := scanner.New(scanner.Config{MediaPath: l.root, FollowSymlinks: true, MaxDepth: 64, MinAge: time.Minute, Workers: 2},
		slog.New(slog.NewTextHandler(io.Discard, nil)), l.r, fixedExtractor{})
	aged := func(name string, size int) {
		p := filepath.Join(l.dir, name)
		b := make([]byte, size)
		for i := range b {
			b[i] = byte(i*7 + size)
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Hour)
		os.Chtimes(p, old, old)
	}
	scan := func() {
		t.Helper()
		if err := s.ScanOnce(l.ctx); err != nil {
			t.Fatal(err)
		}
	}
	aged("junk.mkv", 1<<20)
	scan()
	aged("other.mkv", 1<<19)
	scan()
	st, ok, _ := l.r.StatByPath(l.ctx, filepath.Join(l.dir, "junk.mkv"))
	if !ok {
		t.Fatal("junk.mkv was not indexed")
	}
	id := st.ID
	l.titles[id] = "Ada"

	// Curate it, and give it a thumbnail and a montage.
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(l.r.SetDecision(l.ctx, model.EnrichEntityVideo, id, "title", "manual", "Ada"))
	_, err := l.r.AttachTagToVideo(l.ctx, id, "noir")
	must(err)
	_, err = l.r.CreatePlaylist(l.ctx, "watch", "manual", "private", nil, []int64{id})
	must(err)
	film, err := l.r.CreateFilm(l.ctx, "Ada", 2023)
	must(err)
	_, err = l.r.AttachFilmVideo(l.ctx, film, id, nil, true)
	must(err)
	must(l.r.UpsertEnrichment(l.ctx, model.EnrichEntityVideo, id, "provider-1", "ext-1", map[string][]string{"title": {"Ada"}}))
	must(l.r.SetThumbnailState(l.ctx, id, "generated"))
	must(l.r.SetMontageState(l.ctx, id, "generated", ""))
	thumbs, montages := t.TempDir(), t.TempDir()
	for _, p := range []string{thumbnail.ThumbPath(thumbs, id), montage.Path(montages, id)} {
		must(os.MkdirAll(filepath.Dir(p), 0o755))
		must(os.WriteFile(p, []byte("image"), 0o644))
	}

	snapshot := func() string {
		t.Helper()
		return l.scalar(`SELECT
			(SELECT count(*) FROM videos) || '|' ||
			(SELECT indexed_at || ':' || coalesce(thumbnail_state, '') || ':' || active FROM videos WHERE id = ?1) || '|' ||
			(SELECT group_concat(id) FROM (SELECT id FROM videos ORDER BY indexed_at, id)) || '|' ||
			(SELECT count(*) FROM field_source_decisions WHERE entity_id = ?1) || '|' ||
			(SELECT count(*) FROM video_tags WHERE video_id = ?1) || '|' ||
			(SELECT count(*) FROM playlist_videos WHERE video_id = ?1) || '|' ||
			(SELECT count(*) FROM film_videos WHERE video_id = ?1) || '|' ||
			(SELECT count(*) FROM entity_enrichment WHERE entity_type = 'video' AND entity_id = ?1) || '|' ||
			(SELECT state FROM video_montages WHERE video_id = ?1)`, id)
	}
	before := snapshot()
	if !strings.HasSuffix(before, "|1|1|1|1|1|generated") {
		t.Fatalf("the curation did not land: %s", before)
	}

	l.enqueue("b", id)
	if errs := l.drain(); errs[id] != nil {
		t.Fatal(errs[id])
	}
	if l.path(id) != "Ada.mkv" {
		t.Fatalf("not renamed: %s", l.path(id))
	}
	scan()

	st, ok, _ = l.r.StatByPath(l.ctx, filepath.Join(l.dir, "Ada.mkv"))
	if !ok || st.ID != id {
		t.Fatalf("the renamed file did not keep its id: %+v %v", st, ok)
	}
	if after := snapshot(); after != before {
		t.Errorf("identity changed through rename + rescan:\n before %s\n after  %s", before, after)
	}
	for _, p := range []string{thumbnail.ThumbPath(thumbs, id), montage.Path(montages, id)} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("the id-keyed file is gone: %v", err)
		}
	}
	if locked, _ := l.r.NameLocked(l.ctx, id); locked {
		t.Error("a Holodex rename locked the name")
	}
}
