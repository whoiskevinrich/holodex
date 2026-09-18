package api

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
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

// bandsOf renders (Required, Extras) as "50/null" — the spec's worked-example
// column pair — so an assertion reads as the two v2 numbers, never a blend.
func bandsOf(c resolver.Completeness) string {
	f := func(p *int) string {
		if p == nil {
			return "null"
		}
		return strconv.Itoa(*p)
	}
	return f(c.Required) + "/" + f(c.Extras)
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
// unset (missing tier) — required lands at exactly 50 (2 of 4 critical facets
// present) and extras is null: the fixture maps no nice_to_have facet (F65).
func TestCompletenessForVideos_ScoresCriticalFacets(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	seedVideo(t, r, model.ExtraMetadata{SourceKey: "Publisher", Value: "Acme"})

	out, err := h.completenessForVideos(ctx, repo.VideoFilter{})
	if err != nil {
		t.Fatalf("completenessForVideos: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("videos = %d, want 1", len(out))
	}
	got := out[0].Completeness
	if b := bandsOf(got); b != "50/null" {
		t.Errorf("required/extras = %s, want 50/null", b)
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

	out, err := h.completenessForVideos(ctx, repo.VideoFilter{})
	if err != nil {
		t.Fatalf("completenessForVideos: %v", err)
	}
	got := out[0].Completeness
	// title + studio present, poster_url missing, actors excluded: 2 of 3
	// applicable critical facets present → round(100*2/3) = 67.
	if b := bandsOf(got); b != "67/null" {
		t.Errorf("required/extras = %s, want 67/null", b)
	}
	f, ok := facetByCanonical(got.Facets, "actors")
	if !ok || !f.NotApplicable {
		t.Errorf("actors facet = %+v, want not_applicable", f)
	}
}

// TestCompletenessForPeople_PhotoInjection covers the injectSyntheticFacet gap this
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

	out, err := h.completenessForPeople(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("completenessForPeople: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("people = %d, want 1", len(out))
	}
	// No headshot yet: photo (the only critical person facet) is missing, so
	// required is 0; bio/birthdate (the extras) are unresolved too.
	if b := bandsOf(out[0].Completeness); b != "0/0" {
		t.Errorf("required/extras before headshot = %s, want 0/0", b)
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

	out, err = h.completenessForPeople(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("completenessForPeople after headshot: %v", err)
	}
	// photo now present: required 100 (1 of 1 critical). extras stays 0 —
	// bio/birthdate are still missing, and nationality/alternate_names are
	// optional (F65 RD4), so they sit in neither band.
	if b := bandsOf(out[0].Completeness); b != "100/0" {
		t.Errorf("required/extras after headshot = %s, want 100/0", b)
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

	out, err := h.completenessForStudios(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("completenessForStudios: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("studios = %d, want 1", len(out))
	}
	// No image yet: a studio has no critical facet, so required is null (never
	// a vacuous 100 — ADR-099 D1) and extras, its ring, is 0.
	if b := bandsOf(out[0].Completeness); b != "null/0" {
		t.Errorf("required/extras before image = %s, want null/0", b)
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

	out, err = h.completenessForStudios(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("completenessForStudios after image: %v", err)
	}
	// branding_image now present: with description/country demoted to optional
	// (F65 RD4) it is the only studio extra, so extras is 1/1 → 100 and the
	// studio's ring is full ("has branding art").
	if b := bandsOf(out[0].Completeness); b != "null/100" {
		t.Errorf("required/extras after image = %s, want null/100", b)
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

// TestCompleteness_AlternateNamesFacet covers the F58/ADR-088 D7 replacement for the
// scored facet retiring `aliases` removed. It is synthetic in the same sense
// branding_image is — the resolver never produces it, entity_aliases is read directly —
// and it is deliberately blind to `source`: a name the owner typed and one a provider
// supplied are the same evidence that this entity's alternate names are recorded.
func TestCompleteness_AlternateNamesFacet(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	vid := seedVideo(t, r)
	linkPeopleT(t, r, vid, "Hayao Miyazaki")
	pid, ok, err := r.PersonIDByName(ctx, "Hayao Miyazaki")
	if err != nil || !ok {
		t.Fatalf("person id: ok=%v err=%v", ok, err)
	}

	out, err := h.completenessForPeople(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("completenessForPeople: %v", err)
	}
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "alternate_names"); !ok || f.Tier != resolver.TierMissing {
		t.Errorf("facet with no aliases = %+v, want missing", f)
	}
	// The retired field must not also be scored — two facets for one concept would
	// double-count exactly the duplication this feature removed.
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "aliases"); ok {
		t.Errorf("retired `aliases` is still a scored facet: %+v", f)
	}

	// A provider-sourced alias resolves the facet, with no owner action at all.
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, pid, "tmdb",
		[]string{"Miyazaki Hayao"}); err != nil {
		t.Fatalf("apply provider aliases: %v", err)
	}
	out, err = h.completenessForPeople(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("completenessForPeople after alias: %v", err)
	}
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "alternate_names"); !ok || f.Tier != resolver.TierCurated {
		t.Errorf("facet after a provider alias = %+v, want curated — source is provenance, not privilege", f)
	}
}

// TestCompletenessForStudios_AlternateNamesFacet is the studio half: the facet is
// entity-generic, so a studio-only regression would otherwise hide behind the person test.
func TestCompletenessForStudios_AlternateNamesFacet(t *testing.T) {
	h, r := newCompletenessHandlers(t)
	ctx := context.Background()

	vid := seedVideo(t, r, model.ExtraMetadata{SourceKey: "Publisher", Value: "Ghibli Co"})
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Ghibli Co"}, nil); err != nil {
		t.Fatalf("link studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("studios = %v err=%v", studios, err)
	}

	out, err := h.completenessForStudios(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("completenessForStudios: %v", err)
	}
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "alternate_names"); !ok || f.Tier != resolver.TierMissing {
		t.Errorf("studio facet with no aliases = %+v, want missing", f)
	}

	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityStudio, studios[0].ID, "Ghibli"); err != nil {
		t.Fatalf("add alias: %v", err)
	}
	out, err = h.completenessForStudios(ctx, repo.NamedListFilter{})
	if err != nil {
		t.Fatalf("completenessForStudios after alias: %v", err)
	}
	if f, ok := facetByCanonical(out[0].Completeness.Facets, "alternate_names"); !ok || f.Tier != resolver.TierCurated {
		t.Errorf("studio facet after an owner alias = %+v, want curated", f)
	}
}
