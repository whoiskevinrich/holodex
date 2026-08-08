package api

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// newCompletenessHandlers builds a Handlers over a fresh temp-file repo, wired
// with a minimal video field mapping (title/poster_url/actors/studio — the four
// Critical video facets, per registry.go) so completenessForVideos has
// something to resolve. Person/studio completeness needs no mapping store:
// personFields/studioFields synthesize their field list independent of YAML.
func newCompletenessHandlers(t *testing.T) (*Handlers, *repo.Repo) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [file:title]\n" +
		"  - canonical: poster_url\n    label: Poster\n    sources: [tmdb:poster_url]\n" +
		"  - canonical: actors\n    label: Actors\n    sources: [Artist]\n" +
		"  - canonical: studio\n    label: Studio\n    sources: [Publisher]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	return h, r
}

// seedVideo inserts one video ("A", /m/a.mkv) with the given file-tag extras.
func seedVideo(t *testing.T, r *repo.Repo, extra ...model.ExtraMetadata) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, extra)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	return id
}

func facetByCanonical(facets []resolver.FacetScore, canonical string) (resolver.FacetScore, bool) {
	for _, f := range facets {
		if f.Canonical == canonical {
			return f, true
		}
	}
	return resolver.FacetScore{}, false
}

// TestCompletenessForVideos_ScoresCriticalFacets covers the four Critical video
// facets (title, poster_url, actors, studio, per registry.go): title and studio
// resolve from file baseline/tags (curated tier), poster_url and actors are left
// unset (missing tier) — score should land at exactly 50 (2 of 4 equally-weighted
// critical facets curated).
func TestCompletenessForVideos_ScoresCriticalFacets(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	seedVideo(t, r, model.ExtraMetadata{SourceKey: "Publisher", Value: "Acme"})

	out, err := h.completenessForVideos(ctx)
	if err != nil {
		t.Fatalf("completenessForVideos: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("videos = %d, want 1", len(out))
	}
	got := out[0].Completeness
	if got.Score != 50 {
		t.Errorf("score = %d, want 50", got.Score)
	}
	if f, ok := facetByCanonical(got.Facets, "title"); !ok || f.Tier != resolver.TierCurated {
		t.Errorf("title facet = %+v, want curated", f)
	}
	if f, ok := facetByCanonical(got.Facets, "studio"); !ok || f.Tier != resolver.TierCurated {
		t.Errorf("studio facet = %+v, want curated", f)
	}
	if f, ok := facetByCanonical(got.Facets, "poster_url"); !ok || f.Tier != resolver.TierMissing {
		t.Errorf("poster_url facet = %+v, want missing", f)
	}
	if f, ok := facetByCanonical(got.Facets, "actors"); !ok || f.Tier != resolver.TierMissing {
		t.Errorf("actors facet = %+v, want missing", f)
	}
}

// TestCompletenessForVideos_NotApplicableExcluded covers the tri-state
// not-applicable exclusion: marking poster_url not-applicable removes it from
// the score's denominator entirely rather than counting it as missing.
func TestCompletenessForVideos_NotApplicableExcluded(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	id := seedVideo(t, r, model.ExtraMetadata{SourceKey: "Publisher", Value: "Acme"})
	if err := r.SetFacetNotApplicable(ctx, model.EnrichEntityVideo, id, "actors"); err != nil {
		t.Fatalf("set not applicable: %v", err)
	}

	out, err := h.completenessForVideos(ctx)
	if err != nil {
		t.Fatalf("completenessForVideos: %v", err)
	}
	got := out[0].Completeness
	// title + studio curated, poster_url missing, actors excluded: 2 of 3 scored
	// critical facets curated → round(100*2/3) = 67.
	if got.Score != 67 {
		t.Errorf("score = %d, want 67", got.Score)
	}
	f, ok := facetByCanonical(got.Facets, "actors")
	if !ok || !f.NotApplicable {
		t.Errorf("actors facet = %+v, want not_applicable", f)
	}
}

// TestCompletenessForPeople_PhotoInjection covers the injectAssetFacet gap this
// session found: photo is delivered as an asset (person_images), never a field
// value, so personFields() never produces a row for it — completenessForPeople
// must inject a synthetic facet keyed on HeadshotVersion or every person's photo
// facet silently vanishes from scoring instead of counting as missing.
func TestCompletenessForPeople_PhotoInjection(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	vid := seedVideo(t, r)
	linkPeopleT(t, r, vid, "Hayao Miyazaki")
	pid, ok, err := r.PersonIDByName(ctx, "Hayao Miyazaki")
	if err != nil || !ok {
		t.Fatalf("person id: ok=%v err=%v", ok, err)
	}

	out, err := h.completenessForPeople(ctx)
	if err != nil {
		t.Fatalf("completenessForPeople: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("people = %d, want 1", len(out))
	}
	// No headshot yet: photo scores missing, so score is 0 (bio/birthdate/
	// nationality/aliases/photo all unresolved).
	if out[0].Completeness.Score != 0 {
		t.Errorf("score before headshot = %d, want 0", out[0].Completeness.Score)
	}
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "photo"); !ok || f.Tier != resolver.TierMissing {
		t.Errorf("photo facet before headshot = %+v, want missing", f)
	}

	if _, err := r.InsertPersonImage(ctx, repo.PersonImageInsert{
		PersonID: pid, Role: model.PersonImageHeadshot, Source: model.PersonImageSourceUpload,
		Width: 100, Height: 100, ByteSize: 1000,
	}); err != nil {
		t.Fatalf("insert headshot: %v", err)
	}

	out, err = h.completenessForPeople(ctx)
	if err != nil {
		t.Fatalf("completenessForPeople after headshot: %v", err)
	}
	// Scored facets are bio/birthdate/nationality/aliases (nice-to-have, weight 1
	// each) plus photo (critical, weight 3); photo now curated:
	// (0+0+0+0+3)/(1+1+1+1+3) = 3/7 → round(42.86) = 43.
	if out[0].Completeness.Score != 43 {
		t.Errorf("score after headshot = %d, want 43", out[0].Completeness.Score)
	}
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "photo"); !ok || f.Tier != resolver.TierCurated {
		t.Errorf("photo facet after headshot = %+v, want curated", f)
	}
}

// TestCompletenessForStudios_BrandingImageInjection covers the studio-side twin
// of the photo gap: branding_image is a composite of icon/logo/poster asset
// roles (F55.13 — resolved if any is set), never a resolver field row, so
// completenessForStudios must inject it off ListStudios' already-batched
// ImageVersions rather than leaving it unscored.
func TestCompletenessForStudios_BrandingImageInjection(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	vid := seedVideo(t, r, model.ExtraMetadata{SourceKey: "Publisher", Value: "Acme"})
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Acme"}, nil); err != nil {
		t.Fatalf("link studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("list studios: %v (studios=%v)", err, studios)
	}
	sid := studios[0].ID

	out, err := h.completenessForStudios(ctx)
	if err != nil {
		t.Fatalf("completenessForStudios: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("studios = %d, want 1", len(out))
	}
	// No image yet: description/country/branding_image all missing → score 0.
	if out[0].Completeness.Score != 0 {
		t.Errorf("score before image = %d, want 0", out[0].Completeness.Score)
	}
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "branding_image"); !ok || f.Tier != resolver.TierMissing {
		t.Errorf("branding_image facet before image = %+v, want missing", f)
	}

	if _, err := r.ReplaceStudioImage(ctx, repo.StudioImageInsert{
		StudioID: sid, Role: model.StudioImageIcon, Source: model.StudioImageSourceUpload,
		Width: 100, Height: 100, ByteSize: 1000,
	}); err != nil {
		t.Fatalf("insert studio image: %v", err)
	}

	out, err = h.completenessForStudios(ctx)
	if err != nil {
		t.Fatalf("completenessForStudios after image: %v", err)
	}
	// branding_image now curated: (0+0+1)/(1+1+1) = 1/3 → round(33.33) = 33.
	if out[0].Completeness.Score != 33 {
		t.Errorf("score after image = %d, want 33", out[0].Completeness.Score)
	}
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "branding_image"); !ok || f.Tier != resolver.TierCurated {
		t.Errorf("branding_image facet after image = %+v, want curated", f)
	}
}

// linkPeopleT re-links a video's people the same way repo_test's linkPeople
// helper does, without importing the repo_test package (unexported, external).
func linkPeopleT(t *testing.T, r *repo.Repo, videoID int64, names ...string) {
	t.Helper()
	links := make([]repo.PersonRoleName, len(names))
	for i, n := range names {
		links[i] = repo.PersonRoleName{Name: n, Role: "actor"}
	}
	if err := r.ReconcileVideoPeople(context.Background(), videoID, links, nil); err != nil {
		t.Fatalf("link people: %v", err)
	}
}
