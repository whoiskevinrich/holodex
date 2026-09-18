package metadata_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/metadata"
	"holodex/internal/repo"
	"holodex/internal/scanner"
)

// exifExtractor is a scanner.Extractor that returns what a real exiftool run
// would have produced for each file, parsed by the real mapExiftool — the seam
// every earlier relink test skipped by seeding the Artist row into extra by hand.
type exifExtractor map[string]map[string]any

func (e exifExtractor) Extract(_ context.Context, path string) (metadata.Extracted, error) {
	return metadata.MapExiftool(e[filepath.Base(path)]), nil
}

// TestFirstImportLinksPeopleFromEmbeddedTags pins HOLODEX-408 + HOLODEX-409 end
// to end: a first scan of a file whose people exist only in embedded tags must
// derive video_people through the resolved actors/director fields (ADR-072), and
// a comma-joined tag must split even when the mapping omits `multi: true`.
func TestFirstImportLinksPeopleFromEmbeddedTags(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Deliberately no `multi: true` on either field (HOLODEX-409).
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	if err := os.WriteFile(mpath, []byte(`fields:
  - canonical: actors
    sources: [Artist, Cast]
  - canonical: director
    sources: [Director]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	mstore, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(mstore, cache.Noop{})

	media := filepath.Join(dir, "media")
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "a.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ext := exifExtractor{"a.mp4": {
		"Title":    "Comma Cast",
		"Artist":   "Tag One, Tag Two",
		"Cast":     "Tag Three,Tag Four",
		"Director": "Dee Rector",
	}}
	s := scanner.New(scanner.Config{MediaPath: media, MaxDepth: 4, Workers: 1}, log, r, ext)
	s.SetRelinker(h.RelinkVideoEntity)

	ctx := context.Background()
	if err := s.ScanOnce(ctx); err != nil {
		t.Fatalf("scan: %v", err)
	}
	stat, ok, err := r.StatByPath(ctx, filepath.Join(media, "a.mp4"))
	if err != nil || !ok {
		t.Fatalf("video not indexed: ok=%v err=%v", ok, err)
	}
	people, err := r.PeopleForVideos(ctx, []int64{stat.ID})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range people[stat.ID] {
		got = append(got, p.Name)
	}
	sort.Strings(got)
	want := []string{"Dee Rector", "Tag Four", "Tag One", "Tag Three", "Tag Two"}
	if len(got) != len(want) {
		t.Fatalf("linked people = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("linked people = %v, want %v", got, want)
		}
	}
}
