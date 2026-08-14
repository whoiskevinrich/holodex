package enrich

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

func writeSources(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sources.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write sources: %v", err)
	}
	return p
}

// Registry: enabled filter + the ByName allowlist that prevents an unknown or
// disabled provider from ever being dialed (the SSRF guard, F22.2b).
func TestRegistryLoadAndAllowlist(t *testing.T) {
	reg, err := Load(writeSources(t, `
sources:
  - name: tmdb
    base_url: http://tmdb:9100
    entity_types: [person]
    enabled: true
  - name: imdb
    base_url: http://imdb:9100
    entity_types: [person]
    enabled: false
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Enabled()) != 1 {
		t.Errorf("enabled = %d, want 1", len(reg.Enabled()))
	}
	if _, ok := reg.ByName("tmdb"); !ok {
		t.Error("enabled tmdb not resolved")
	}
	if _, ok := reg.ByName("imdb"); ok {
		t.Error("disabled imdb resolved — allowlist breach")
	}
	if _, ok := reg.ByName("evil"); ok {
		t.Error("unknown provider resolved — allowlist breach")
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	reg, err := Load(filepath.Join(t.TempDir(), "none.yaml"), nil)
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(reg.Enabled()) != 0 {
		t.Error("missing file should yield no providers")
	}
}

// ADR-080 D3/FR3: a valid search_pattern/default_search_pattern is kept verbatim; an
// unknown-token pattern is dropped at config-load time (logged, not the file's
// problem) without disabling the provider itself.
func TestRegistryLoadSearchPatternValidation(t *testing.T) {
	path := writeSources(t, `
default_search_pattern: "{studio?} {year?}"
sources:
  - name: good
    base_url: http://good:9100
    entity_types: [video]
    enabled: true
    search_pattern: "{title?} {performers?}"
  - name: bad
    base_url: http://bad:9100
    entity_types: [video]
    enabled: true
    search_pattern: "{director?}"
`)
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	reg, err := Load(path, log)
	if err != nil {
		t.Fatal(err)
	}
	if got := reg.DefaultSearchPattern(); got != "{studio?} {year?}" {
		t.Errorf("valid default_search_pattern should survive verbatim, got %q", got)
	}
	good, ok := reg.ByName("good")
	if !ok || good.SearchPattern != "{title?} {performers?}" {
		t.Errorf("valid search_pattern should survive verbatim, got %+v ok=%v", good, ok)
	}
	bad, ok := reg.ByName("bad")
	if !ok {
		t.Fatal("a provider with an invalid search_pattern must stay enabled")
	}
	if bad.SearchPattern != "" {
		t.Errorf("unknown-token search_pattern should be dropped, got %q", bad.SearchPattern)
	}
	if !strings.Contains(buf.String(), "bad.search_pattern") {
		t.Errorf("expected a warning naming the offending field, got log: %s", buf.String())
	}
}

// A nil logger (the common case in tests that don't care about warnings) must not
// panic when a pattern is invalid.
func TestRegistryLoadSearchPatternValidation_NilLoggerSafe(t *testing.T) {
	path := writeSources(t, "sources:\n  - name: bad\n    base_url: http://bad:9100\n    entity_types: [video]\n    enabled: true\n    search_pattern: \"{director?}\"\n")
	if _, err := Load(path, nil); err != nil {
		t.Fatalf("nil logger should not error: %v", err)
	}
}

// SSRF: a provider response that redirects to a different host must NOT be
// followed (F22.9c). The "internal" target stays untouched.
func TestHTTPClientNoCrossHostRedirect(t *testing.T) {
	var evilHits int
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		evilHits++
		_, _ = w.Write([]byte(`{"provider":"evil","protocol_version":1}`))
	}))
	defer evil.Close()

	prov := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/describe", http.StatusFound)
	}))
	defer prov.Close()

	c := newHTTPClient(Source{BaseURL: prov.URL})
	if _, err := c.Describe(context.Background()); err == nil {
		t.Error("expected error when redirect is not followed")
	}
	if evilHits != 0 {
		t.Errorf("followed cross-host redirect (%d hits on internal host)", evilHits)
	}
}

// The HTTP transport speaks the contract against a stub server.
func TestHTTPClientContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/describe":
			_, _ = w.Write([]byte(`{"provider":"t","protocol_version":1,"entity_types":["person"]}`))
		case "/resolve":
			_, _ = w.Write([]byte(`{"candidates":[{"external_id":"tmdb:1","label":"X","confidence":0.9}]}`))
		case "/enrich":
			_, _ = w.Write([]byte(`{"fields":{"bio":["hi"]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newHTTPClient(Source{BaseURL: srv.URL})
	ctx := context.Background()
	if m, err := c.Describe(ctx); err != nil || m.ProtocolVersion != 1 {
		t.Fatalf("describe: %+v err=%v", m, err)
	}
	if cands, err := c.Resolve(ctx, "person", Hint{Query: "x"}); err != nil || len(cands) != 1 {
		t.Fatalf("resolve: %v err=%v", cands, err)
	}
	if res, err := c.Enrich(ctx, "person", "tmdb:1"); err != nil || len(res.Fields["bio"]) != 1 {
		t.Fatalf("enrich: %+v err=%v", res, err)
	}
}

func newSvc(t *testing.T, fake *Fake) (*Service, *repo.Repo) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	store, err := NewStore(writeSources(t, `
sources:
  - name: fake
    base_url: http://fake:9100
    entity_types: [person, studio]
    enabled: true
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewServiceWithClient(store, r, log, func(Source) ProviderClient { return fake })
	return svc, r
}

// Service end-to-end against the in-process fake: resolve → enrich → provenance →
// match persistence → clear. No network, no keys (F22.10).
func TestServiceResolveEnrichClear(t *testing.T) {
	svc, _ := newSvc(t, NewFake("fake"))
	ctx := context.Background()

	cands, err := svc.Resolve(ctx, "fake", model.EnrichEntityPerson, Hint{Query: "miyazaki"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cands) != 1 || cands[0].ExternalID != "tmdb:608" {
		t.Fatalf("candidates = %+v", cands)
	}
	if !cands[0].AutoApply {
		t.Errorf("AutoApply = false for confidence %v, want true (>= StrongMatchThreshold)", cands[0].Confidence)
	}

	fields, err := svc.Enrich(ctx, model.EnrichEntityPerson, 1, "fake", "tmdb:608", false)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	got := map[string]model.EnrichedField{}
	for _, f := range fields {
		got[f.Canonical] = f
	}
	if got["bio"].Provider != "fake" {
		t.Errorf("provenance = %q, want fake", got["bio"].Provider)
	}
	if got["birthdate"].Label != "Born" {
		t.Errorf("label = %q, want Born", got["birthdate"].Label)
	}
	if len(got["aliases"].Values) != 2 {
		t.Errorf("aliases = %v", got["aliases"].Values)
	}

	if id, ok, _ := svc.ExistingMatch(ctx, model.EnrichEntityPerson, 1, "fake"); !ok || id != "tmdb:608" {
		t.Errorf("existing match = %q ok=%v", id, ok)
	}

	if err := svc.Clear(ctx, model.EnrichEntityPerson, 1, "fake"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if after, _ := svc.Fields(ctx, model.EnrichEntityPerson, 1); len(after) != 0 {
		t.Errorf("fields after clear = %d, want 0", len(after))
	}
}

// A candidate's profile_url is scheme-validated server-side before it can ever
// reach an API response and become a picker `href` (F47, RD6/P1-1): a hostile
// scheme is dropped silently (the candidate itself stays usable), a normal
// http(s) URL round-trips unchanged.
func TestServiceResolveSanitizesProfileURL(t *testing.T) {
	fake := NewFake("fake")
	fake.People["tmdb:1"] = FakePerson{Label: "Hostile Match", ProfileURL: "javascript:alert(1)"}
	fake.People["tmdb:2"] = FakePerson{Label: "Clean Match", ProfileURL: "https://www.themoviedb.org/person/2"}
	svc, _ := newSvc(t, fake)
	ctx := context.Background()

	cands, err := svc.Resolve(ctx, "fake", model.EnrichEntityPerson, Hint{Query: "match"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got := map[string]string{}
	for _, c := range cands {
		got[c.ExternalID] = c.ProfileURL
	}
	if got["tmdb:1"] != "" {
		t.Errorf("hostile-scheme profile_url survived sanitization: %q", got["tmdb:1"])
	}
	if got["tmdb:2"] != "https://www.themoviedb.org/person/2" {
		t.Errorf("clean profile_url = %q, want round-tripped unchanged", got["tmdb:2"])
	}
}

// Studio entity end-to-end (F38 S3): resolve → enrich → provenance → clear against
// the in-process fake studio. Proves the entity-generic Enrich service works for a
// third entity type with no core diffs, and that the logo arrives as a plain field.
func TestServiceStudioEnrich(t *testing.T) {
	svc, _ := newSvc(t, NewFake("fake"))
	ctx := context.Background()

	cands, err := svc.Resolve(ctx, "fake", model.EnrichEntityStudio, Hint{Query: "ghibli"})
	if err != nil {
		t.Fatalf("resolve studio: %v", err)
	}
	if len(cands) != 1 || cands[0].ExternalID != "tmdb:10342" {
		t.Fatalf("candidates = %+v", cands)
	}

	fields, err := svc.Enrich(ctx, model.EnrichEntityStudio, 7, "fake", "tmdb:10342", false)
	if err != nil {
		t.Fatalf("enrich studio: %v", err)
	}
	got := map[string]model.EnrichedField{}
	for _, f := range fields {
		got[f.Canonical] = f
	}
	if got["description"].Provider != "fake" {
		t.Errorf("description provenance = %q, want fake", got["description"].Provider)
	}
	if got["country"].Values[0] != "JP" {
		t.Errorf("country = %v, want [JP]", got["country"].Values)
	}
	// logo is NOT among the resolved fields (F51, ADR-079): it arrives as an image
	// asset and is stored via the image sink (see TestEnrichDownloadsStudioAssets),
	// not as a plain image_url field value. No sink is wired here, so the asset is
	// silently ignored — the field-only path this test otherwise exercises.
	if _, ok := got["logo"]; ok {
		t.Errorf("logo should not appear among resolved fields: %+v", got["logo"])
	}

	if err := svc.Clear(ctx, model.EnrichEntityStudio, 7, "fake"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if after, _ := svc.Fields(ctx, model.EnrichEntityStudio, 7); len(after) != 0 {
		t.Errorf("fields after clear = %d, want 0", len(after))
	}
}

// An unknown/disabled provider is rejected before any client is dialed.
func TestServiceUnknownProviderNotDialed(t *testing.T) {
	fake := NewFake("fake")
	svc, _ := newSvc(t, fake)
	if _, err := svc.Resolve(context.Background(), "evil", model.EnrichEntityPerson, Hint{Query: "x"}); err == nil {
		t.Error("expected error for unknown provider")
	}
	if fake.Calls != 0 {
		t.Errorf("fake dialed for unknown provider: %d calls", fake.Calls)
	}
}

// A provider speaking a different protocol major is refused (F22.1e).
func TestServiceProtocolMismatch(t *testing.T) {
	fake := NewFake("fake")
	fake.Protocol = ProtocolVersion + 1
	svc, _ := newSvc(t, fake)
	if _, err := svc.Resolve(context.Background(), "fake", model.EnrichEntityPerson, Hint{Query: "miyazaki"}); err == nil {
		t.Error("expected protocol-mismatch error")
	}
}

// A successful enrich records a kind=enrich pass in the activity history with a
// provider/entity detail and no leaked secrets (F22.6b, ADR-028).
func TestServiceEnrichRecordsJobRun(t *testing.T) {
	svc, r := newSvc(t, NewFake("fake"))
	ctx := context.Background()
	if _, err := svc.Enrich(ctx, model.EnrichEntityPerson, 7, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	runs, err := r.ListJobRuns(ctx, 1)
	if err != nil {
		t.Fatalf("list job runs: %v", err)
	}
	var job *model.JobRun
	for i := range runs {
		if runs[i].Kind == model.JobKindEnrich {
			job = &runs[i]
			break
		}
	}
	if job == nil {
		t.Fatal("no kind=enrich job recorded")
	}
	if job.Status != model.JobStatusOK {
		t.Errorf("status = %q, want success", job.Status)
	}
	if !strings.Contains(job.Detail, "fake") || !strings.Contains(job.Detail, "#7") {
		t.Errorf("detail = %q, want provider + entity ref", job.Detail)
	}
	// Attribution (ADR-071) — the entity the detail names, as columns, so
	// "what touched person #7?" is an indexed query rather than a text search.
	if job.EntityType != model.EnrichEntityPerson || job.EntityID != 7 {
		t.Errorf("attribution = %q/#%d, want person/#7", job.EntityType, job.EntityID)
	}
}

// --- F25 asset download ---

// recordingSink captures StoreAsset calls instead of touching disk, so the asset
// orchestration is testable with no filesystem. suppress is the set of URLs it
// reports as owner-deleted (F25/ADR-043 suppression).
type recordingSink struct {
	stored   []storedAsset
	suppress map[string]struct{}
	locked   map[string]struct{} // core roles the owner set by hand (F33, ADR-049)
	existing map[string]struct{} // asset URLs already stored (F34/ADR-050 URL fast-path)
}

type storedAsset struct {
	entityType string
	personID   int64
	role       string
	provider   string
	externalID string
	url        string
	bytes      int
	overCap    bool
}

func (s *recordingSink) StoreAsset(_ context.Context, entityType string, personID int64, role, provider, externalID, url string, raw []byte, overCap bool) error {
	s.stored = append(s.stored, storedAsset{entityType, personID, role, provider, externalID, url, len(raw), overCap})
	return nil
}

// StoreAssetIfAbsent records a core-role store only when that (person, role) slot has
// not already been filled in this run — mirroring the real sink's empty-slot guard.
func (s *recordingSink) StoreAssetIfAbsent(ctx context.Context, entityType string, personID int64, role, provider, externalID, url string, raw []byte) error {
	for _, a := range s.stored {
		if a.personID == personID && a.role == role {
			return nil // slot already filled
		}
	}
	return s.StoreAsset(ctx, entityType, personID, role, provider, externalID, url, raw, false)
}

func (s *recordingSink) SuppressedAssetURLs(_ context.Context, _ string, _ int64) (map[string]struct{}, error) {
	return s.suppress, nil
}

func (s *recordingSink) LockedCoreRoles(_ context.Context, _ string, _ int64) (map[string]struct{}, error) {
	return s.locked, nil
}

func (s *recordingSink) ExistingAssetURLs(_ context.Context, _ string, _ int64) (map[string]struct{}, error) {
	return s.existing, nil
}

// A person enrich run with image assets fetches each through the (test-injected)
// SSRF-guarded asset client and stores it via the sink, mapping kinds → core roles.
func TestEnrichDownloadsAssets(t *testing.T) {
	// An origin that serves any path some bytes — stands in for the provider's asset
	// host. The injected fetcher hits it directly.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("rawimagebytes"))
	}))
	defer origin.Close()

	fake := NewFake("fake")
	rec := fake.People["tmdb:608"]
	rec.Assets = []Asset{
		{Kind: "photo", URL: origin.URL + "/p.jpg"},
		{Kind: "banner", URL: origin.URL + "/b.jpg"},
		{Kind: "mystery", URL: origin.URL + "/x.jpg"}, // unknown kind → skipped
	}
	fake.People["tmdb:608"] = rec

	svc, _ := newSvc(t, fake)
	sink := &recordingSink{}
	svc.SetImageSink(sink)
	// Inject an asset fetcher that just GETs the URL (the SSRF host-pinning is unit-
	// tested separately in TestAssetClientSSRF); here we exercise the orchestration.
	svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	// 3 stored: the photo→headshot, the banner, and a poster auto-seeded from the
	// headshot portrait (F25.29). The unknown kind is skipped.
	if len(sink.stored) != 3 {
		t.Fatalf("stored %d assets, want 3 (headshot, banner, seeded poster; unknown skipped): %+v", len(sink.stored), sink.stored)
	}
	roles := map[string]bool{}
	for _, a := range sink.stored {
		roles[a.role] = true
		if a.provider != "fake" || a.externalID != "tmdb:608" {
			t.Errorf("asset provenance = %+v, want provider=fake external=tmdb:608", a)
		}
	}
	if !roles[model.PersonImageHeadshot] || !roles[model.PersonImageBanner] || !roles[model.PersonImagePoster] {
		t.Errorf("asset roles = %v, want headshot+banner+poster", roles)
	}
}

// A studio enrich run with an image asset fetches it and stores it via the sink,
// under entityType="studio" (F51, ADR-079 — the second real use of downloadAssets).
// A locked (owner-uploaded) role is skipped entirely, mirroring the person
// provenance-lock (ADR-049) generalized to a second entity.
func TestEnrichDownloadsStudioAssets(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("rawimagebytes"))
	}))
	defer origin.Close()

	fake := NewFake("fake")
	rec := fake.Studios["tmdb:10342"]
	rec.Assets = []Asset{{Kind: "logo", URL: origin.URL + "/logo.jpg"}}
	fake.Studios["tmdb:10342"] = rec

	svc, _ := newSvc(t, fake)
	sink := &recordingSink{}
	svc.SetImageSink(sink)
	svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityStudio, 3, "fake", "tmdb:10342", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(sink.stored) != 1 {
		t.Fatalf("stored %d assets, want 1 (logo; no poster-seed for studios): %+v", len(sink.stored), sink.stored)
	}
	a := sink.stored[0]
	if a.entityType != model.EnrichEntityStudio || a.role != model.StudioImageLogo || a.provider != "fake" || a.externalID != "tmdb:10342" {
		t.Errorf("stored asset = %+v, want studio/logo/fake/tmdb:10342", a)
	}

	// A locked (owner-uploaded) logo is never overwritten by enrichment.
	sink2 := &recordingSink{locked: map[string]struct{}{model.StudioImageLogo: {}}}
	svc.SetImageSink(sink2)
	if _, err := svc.Enrich(context.Background(), model.EnrichEntityStudio, 3, "fake", "tmdb:10342", false); err != nil {
		t.Fatalf("enrich (locked): %v", err)
	}
	if len(sink2.stored) != 0 {
		t.Errorf("stored %d assets for a locked logo, want 0", len(sink2.stored))
	}
}

// A re-enrich skips an asset whose URL the owner previously deleted (F25/ADR-043):
// the suppressed URL is never fetched or stored, while other assets still flow.
func TestEnrichSkipsSuppressedAssets(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("rawimagebytes"))
	}))
	defer origin.Close()

	fake := NewFake("fake")
	rec := fake.People["tmdb:608"]
	rec.Assets = []Asset{
		{Kind: "gallery", URL: origin.URL + "/keep.jpg"},
		{Kind: "gallery", URL: origin.URL + "/deleted.jpg"}, // owner removed this before
	}
	fake.People["tmdb:608"] = rec

	svc, _ := newSvc(t, fake)
	sink := &recordingSink{suppress: map[string]struct{}{origin.URL + "/deleted.jpg": {}}}
	svc.SetImageSink(sink)
	svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(sink.stored) != 1 {
		t.Fatalf("stored %d assets, want 1 (suppressed url skipped): %+v", len(sink.stored), sink.stored)
	}
	if sink.stored[0].url != origin.URL+"/keep.jpg" {
		t.Errorf("stored url = %q, want the non-suppressed keep.jpg", sink.stored[0].url)
	}
}

// A re-enrich skips a gallery asset whose URL the person already holds (F34/ADR-050
// URL fast-path): the already-stored URL is not re-fetched/re-stored, while a new
// gallery URL still flows. Scoped to the gallery — core roles replace in place.
func TestEnrichSkipsAlreadyHeldGalleryURL(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("rawimagebytes"))
	}))
	defer origin.Close()

	fake := NewFake("fake")
	rec := fake.People["tmdb:608"]
	rec.Assets = []Asset{
		{Kind: "gallery", URL: origin.URL + "/have.jpg"}, // already stored → skipped
		{Kind: "gallery", URL: origin.URL + "/new.jpg"},  // fresh → stored
	}
	fake.People["tmdb:608"] = rec

	svc, _ := newSvc(t, fake)
	sink := &recordingSink{existing: map[string]struct{}{origin.URL + "/have.jpg": {}}}
	svc.SetImageSink(sink)
	svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(sink.stored) != 1 {
		t.Fatalf("stored %d assets, want 1 (already-held url skipped): %+v", len(sink.stored), sink.stored)
	}
	if sink.stored[0].url != origin.URL+"/new.jpg" {
		t.Errorf("stored url = %q, want the fresh new.jpg", sink.stored[0].url)
	}
}

// Enrichment never overwrites a core image the owner set by hand (F33, ADR-049):
// a locked role is skipped before its bytes are even fetched, while empty/provider-set
// roles still flow. A locked role left empty by enrichment also blocks the poster seed.
func TestEnrichKeepsOwnerSetCoreImages(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("rawimagebytes"))
	}))
	defer origin.Close()

	t.Run("locked headshot+banner are kept; gallery still flows", func(t *testing.T) {
		fake := NewFake("fake")
		rec := fake.People["tmdb:608"]
		rec.Assets = []Asset{
			{Kind: "photo", URL: origin.URL + "/p.jpg"},   // → headshot, owner-locked
			{Kind: "banner", URL: origin.URL + "/b.jpg"},  // → banner, owner-locked
			{Kind: "gallery", URL: origin.URL + "/g.jpg"}, // → extra, not a core slot
		}
		fake.People["tmdb:608"] = rec

		svc, _ := newSvc(t, fake)
		sink := &recordingSink{locked: map[string]struct{}{
			model.PersonImageHeadshot: {}, model.PersonImageBanner: {},
		}}
		svc.SetImageSink(sink)
		svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

		if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608", false); err != nil {
			t.Fatalf("enrich: %v", err)
		}
		// Only the gallery item stores: both locked core slots are skipped, and with no
		// headshot stored there is nothing to seed a poster from.
		if len(sink.stored) != 1 {
			t.Fatalf("stored %d assets, want 1 (gallery only; locked headshot/banner kept): %+v", len(sink.stored), sink.stored)
		}
		if sink.stored[0].role != model.PersonImageExtra {
			t.Errorf("stored role = %q, want the gallery extra", sink.stored[0].role)
		}
	})

	t.Run("locked poster blocks the headshot seed", func(t *testing.T) {
		fake := NewFake("fake")
		rec := fake.People["tmdb:608"]
		rec.Assets = []Asset{{Kind: "photo", URL: origin.URL + "/p.jpg"}} // → headshot only
		fake.People["tmdb:608"] = rec

		svc, _ := newSvc(t, fake)
		sink := &recordingSink{locked: map[string]struct{}{model.PersonImagePoster: {}}}
		svc.SetImageSink(sink)
		svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

		if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608", false); err != nil {
			t.Fatalf("enrich: %v", err)
		}
		// Headshot flows (not locked); the poster seed is suppressed by the lock.
		if len(sink.stored) != 1 || sink.stored[0].role != model.PersonImageHeadshot {
			t.Fatalf("stored = %+v, want only the headshot (poster seed blocked by lock)", sink.stored)
		}
	})
}

// A person enrich that returns a portrait but no poster seeds the poster from the
// headshot (F25.29) — the same 2:3 image, so people read richly on video credits. A
// poster the person already has is left untouched.
func TestEnrichSeedsPosterFromHeadshot(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("rawimagebytes"))
	}))
	defer origin.Close()

	newFakeWithPhoto := func() *Fake {
		fake := NewFake("fake")
		rec := fake.People["tmdb:608"]
		rec.Assets = []Asset{{Kind: "photo", URL: origin.URL + "/p.jpg"}}
		fake.People["tmdb:608"] = rec
		return fake
	}

	t.Run("empty poster is seeded from the headshot", func(t *testing.T) {
		svc, _ := newSvc(t, newFakeWithPhoto())
		sink := &recordingSink{}
		svc.SetImageSink(sink)
		svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

		if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608", false); err != nil {
			t.Fatalf("enrich: %v", err)
		}
		roles := map[string]int{}
		var posterURL string
		for _, a := range sink.stored {
			roles[a.role]++
			if a.role == model.PersonImagePoster {
				posterURL = a.url
			}
		}
		if roles[model.PersonImageHeadshot] != 1 || roles[model.PersonImagePoster] != 1 {
			t.Fatalf("stored roles = %v, want one headshot + one seeded poster", roles)
		}
		if posterURL != origin.URL+"/p.jpg" {
			t.Errorf("seeded poster url = %q, want the headshot url (same image)", posterURL)
		}
	})

	t.Run("existing poster is not clobbered", func(t *testing.T) {
		svc, _ := newSvc(t, newFakeWithPhoto())
		// The person already has a poster (e.g. owner-set): the seed must leave it alone.
		sink := &recordingSink{stored: []storedAsset{{personID: 5, role: model.PersonImagePoster, url: "owner.jpg"}}}
		svc.SetImageSink(sink)
		svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

		if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608", false); err != nil {
			t.Fatalf("enrich: %v", err)
		}
		posters := 0
		for _, a := range sink.stored {
			if a.role == model.PersonImagePoster {
				posters++
			}
		}
		if posters != 1 {
			t.Fatalf("poster count = %d, want 1 (pre-existing kept, not re-seeded): %+v", posters, sink.stored)
		}
	})
}

// passthroughFetcher fetches a URL with no host allowlist — used only to drive the
// orchestration test above (the real guard is TestAssetClientSSRF).
type passthroughFetcher struct{}

func (passthroughFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// The asset client refuses any host other than the provider's configured base_url
// host (the SSRF allowlist), and rejects non-http(s) schemes.
func TestAssetClientSSRF(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("should-never-be-read"))
	}))
	defer internal.Close()

	// The provider is configured for some other host; an asset URL pointing at the
	// internal server must be refused before any request is made.
	c := newAssetClient(Source{BaseURL: "http://provider.example:9100"})
	if _, err := c.Fetch(context.Background(), internal.URL+"/secret"); err == nil {
		t.Error("expected refusal of off-allowlist asset host")
	}
	if _, err := c.Fetch(context.Background(), "file:///etc/passwd"); err == nil {
		t.Error("expected refusal of non-http scheme")
	}

	// An asset on the provider's own host is allowed.
	prov := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok-bytes"))
	}))
	defer prov.Close()
	allow := newAssetClient(Source{BaseURL: prov.URL})
	got, err := allow.Fetch(context.Background(), prov.URL+"/photo.jpg")
	if err != nil {
		t.Fatalf("same-host fetch should succeed: %v", err)
	}
	if string(got) != "ok-bytes" {
		t.Errorf("fetched %q, want ok-bytes", got)
	}
}

// ADR-039: operator-listed asset_hosts expand the allowlist beyond the base host.
// We verify the allowlist layer independently of the https-enforcement layer (which
// is tested in TestAssetClientNonBaseHostRequiresHTTPS). A listed host gets past the
// allowlist check — if it then fails on https enforcement, that's correct behaviour
// (in production the URL would be https://). An unlisted host is rejected before
// https is even checked.
func TestAssetClientAssetHosts(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("should-not-reach"))
	}))
	defer evil.Close()
	unrelated := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("should-not-reach"))
	}))
	defer unrelated.Close()

	listedHost := func(rawURL string) string {
		u, _ := url.Parse(rawURL)
		return u.Host
	}

	c := newAssetClient(Source{
		BaseURL:    "http://provider.example:9100",
		AssetHosts: []string{listedHost(evil.URL)},
	})

	// Listed host: passes the allowlist check. Fails on https enforcement because
	// httptest uses http — the error must mention "https", not "not on the provider allowlist".
	_, err := c.Fetch(context.Background(), evil.URL+"/photo.jpg")
	if err == nil {
		// This would only succeed if somehow https enforcement was skipped.
		t.Error("expected https enforcement error for listed non-base http host")
	} else if strings.Contains(err.Error(), "not on the provider allowlist") {
		t.Errorf("listed host should pass allowlist check; got: %v", err)
	}

	// Unlisted host: rejected at the allowlist layer, not at https.
	_, err = c.Fetch(context.Background(), unrelated.URL+"/x")
	if err == nil {
		t.Error("unlisted host should be refused")
	} else if !strings.Contains(err.Error(), "not on the provider allowlist") {
		t.Errorf("expected allowlist error for unlisted host, got: %v", err)
	}
}

// ADR-039 §3: a non-base host in asset_hosts requires https (not http).
// We cannot start a real https server in a unit test without certs, so we verify
// the guard fires before the request is made: a URL with scheme "http" pointing at
// a listed-but-non-base host must be refused with a scheme error, not a network error.
func TestAssetClientNonBaseHostRequiresHTTPS(t *testing.T) {
	c := newAssetClient(Source{
		BaseURL:    "http://provider.example:9100",
		AssetHosts: []string{"image.tmdb.org"},
	})
	_, err := c.Fetch(context.Background(), "http://image.tmdb.org/t/p/original/photo.jpg")
	if err == nil {
		t.Fatal("expected https enforcement error for non-base asset host")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Errorf("error should mention https requirement, got: %v", err)
	}
}

// ADR-039 §5: first-success-per-role — only the first fetchable asset of a given
// role is stored; subsequent entries of the same kind are skipped.
func TestDownloadAssetsFirstSuccessPerRole(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("bytes"))
	}))
	defer origin.Close()

	fake := NewFake("fake")
	rec := fake.People["tmdb:608"]
	// Two photos: only the first should be stored (first-success-per-role).
	rec.Assets = []Asset{
		{Kind: "photo", URL: origin.URL + "/first.jpg"},
		{Kind: "photo", URL: origin.URL + "/second.jpg"},
	}
	fake.People["tmdb:608"] = rec

	svc, _ := newSvc(t, fake)
	sink := &recordingSink{}
	svc.SetImageSink(sink)
	svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	// Exactly one headshot stored, not two.
	var headshots int
	for _, a := range sink.stored {
		if a.role == model.PersonImageHeadshot {
			headshots++
		}
	}
	if headshots != 1 {
		t.Errorf("stored %d headshots, want 1 (first-success-per-role)", headshots)
	}
}

// A failed asset fetch never fails the field enrichment (best-effort, guarded).
func TestEnrichAssetFailureIsNonFatal(t *testing.T) {
	fake := NewFake("fake")
	rec := fake.People["tmdb:608"]
	rec.Assets = []Asset{{Kind: "photo", URL: "http://unreachable.invalid/x.jpg"}}
	fake.People["tmdb:608"] = rec

	svc, _ := newSvc(t, fake)
	svc.SetImageSink(&recordingSink{})
	// Default real asset client: the fake's source host is "fake:9100", so the
	// invalid asset host is refused — the enrich must still return fields.
	fields, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 9, "fake", "tmdb:608", false)
	if err != nil {
		t.Fatalf("enrich must not fail on a bad asset: %v", err)
	}
	if len(fields) == 0 {
		t.Error("fields empty despite successful field enrichment")
	}
}

// An Assets[] for an entity type with no image sink (F51, ADR-079: only person and
// studio are image-backed; e.g. video is not) is discarded, but the drop is logged
// loudly instead of silently, so a provider author who mirrors the person/studio
// asset pattern gets a diagnosable signal instead of an inert no-op
// (docs/specs/metadata-provider-contract.md).
func TestEnrichWarnsOnDiscardedAssetsForUnsupportedEntityType(t *testing.T) {
	fake := NewFake("fake")
	rec := fake.People["tmdb:608"]
	rec.Assets = []Asset{{Kind: "poster", URL: "http://example.invalid/logo.png"}}
	fake.People["tmdb:608"] = rec

	database, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	store, err := NewStore(writeSources(t, `
sources:
  - name: fake
    base_url: http://fake:9100
    entity_types: [person, studio, video]
    enabled: true
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	svc := NewServiceWithClient(store, r, log, func(Source) ProviderClient { return fake })
	sink := &recordingSink{}
	svc.SetImageSink(sink)

	// Fake.records() falls back to People for any entityType != studio, so "video"
	// resolves the same fixture under a type the sink doesn't back.
	if _, err := svc.Enrich(context.Background(), model.EnrichEntityVideo, 7, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(sink.stored) != 0 {
		t.Errorf("stored %d assets for video, want 0 (no image sink for this entity type)", len(sink.stored))
	}
	logged := buf.String()
	if !strings.Contains(logged, "discarding assets for an entity type with no image sink") {
		t.Errorf("log = %q, want a warning about discarded assets", logged)
	}
	if !strings.Contains(logged, "video") || !strings.Contains(logged, "poster") {
		t.Errorf("log = %q, want entity_type=video and asset_kinds containing poster", logged)
	}
}

// A video enrich response's structured people[] credits (F32, contract §4.5) get
// resolved to real Person rows (id-first, ADR-055) and their headshots downloaded —
// keyed by each PERSON's own id, not the video's — while a credit with no external_id
// is skipped entirely (ADR-055: refused, not name-matched) rather than creating a
// bare Person. The _person_external_ids sidecar also persists, ready for
// RelinkVideoPeople's later name-based reconcile to consume.
func TestEnrichVideoConsumesPeopleCredits(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("rawimagebytes"))
	}))
	defer origin.Close()

	fake := NewFake("fake")
	fake.People["tmdb:550"] = FakePerson{
		Label: "Fight Club",
		Fields: map[string][]string{
			"title":    {"Fight Club"},
			"actors":   {"Brad Pitt"},
			"director": {"David Fincher"},
		},
		People: []ProviderPerson{
			{Name: "Brad Pitt", Role: "actor", ExternalID: "tmdb:287", Order: 0,
				Headshot: &Asset{Kind: "photo", URL: origin.URL + "/pitt.jpg"}},
			{Name: "David Fincher", Role: "director", ExternalID: "tmdb:7467"}, // no headshot
			{Name: "No External ID", Role: "actor", ExternalID: ""},            // dropped, ADR-055
		},
	}

	database, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	store, err := NewStore(writeSources(t, `
sources:
  - name: fake
    base_url: http://fake:9100
    entity_types: [person, studio, video]
    enabled: true
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewServiceWithClient(store, r, log, func(Source) ProviderClient { return fake })
	sink := &recordingSink{}
	svc.SetImageSink(sink)
	svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityVideo, 42, "fake", "tmdb:550", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	// Only Brad Pitt has a headshot; it's stored under HIS person id, not video 42's —
	// 2 entries, because downloadAssets also seeds an empty poster from a fresh
	// headshot (F25.29), same as any other person enrich (TestEnrichDownloadsAssets).
	if len(sink.stored) != 2 {
		t.Fatalf("stored = %+v, want 2 (headshot + seeded poster, Brad Pitt only)", sink.stored)
	}
	pitt := sink.stored[0]
	if pitt.entityType != model.EnrichEntityPerson || pitt.role != model.PersonImageHeadshot {
		t.Errorf("stored asset[0] = %+v, want entityType=person role=headshot", pitt)
	}
	if pitt.externalID != "tmdb:287" {
		t.Errorf("stored asset externalID = %q, want tmdb:287", pitt.externalID)
	}
	for _, a := range sink.stored {
		if a.personID != pitt.personID {
			t.Errorf("stored asset %+v under a different person than the headshot (%d)", a, pitt.personID)
		}
	}

	// Exactly 2 people exist — the id-less credit created no third row.
	var personCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM people`).Scan(&personCount); err != nil {
		t.Fatalf("count people: %v", err)
	}
	if personCount != 2 {
		t.Errorf("people count = %d, want 2 (id-less credit skipped)", personCount)
	}

	// Both credited people resolved by external_id, and the headshot landed on the
	// same person id resolveOrCreate returned (not some other/new row).
	var pittID, fincherID int64
	if err := database.QueryRow(`SELECT person_id FROM person_external_ids WHERE external_id = ?`, "tmdb:287").Scan(&pittID); err != nil {
		t.Fatalf("brad pitt not resolved by external id: %v", err)
	}
	if err := database.QueryRow(`SELECT person_id FROM person_external_ids WHERE external_id = ?`, "tmdb:7467").Scan(&fincherID); err != nil {
		t.Fatalf("david fincher not resolved by external id: %v", err)
	}
	if pitt.personID != pittID {
		t.Errorf("headshot stored under person %d, want the resolved id %d", pitt.personID, pittID)
	}

	// The _person_external_ids sidecar persisted for RelinkVideoPeople's caller to
	// recover later — same shape as _studio_external_ids ("<external_id> <name>").
	rows, err := r.EnrichmentForEntity(context.Background(), model.EnrichEntityVideo, 42)
	if err != nil {
		t.Fatalf("enrichment rows: %v", err)
	}
	var sidecar []string
	for _, row := range rows {
		if row.FieldKey == model.PersonExternalIDsField {
			sidecar = row.Values
		}
	}
	want := []string{"tmdb:287 Brad Pitt", "tmdb:7467 David Fincher"}
	if len(sidecar) != len(want) || sidecar[0] != want[0] || sidecar[1] != want[1] {
		t.Errorf("_person_external_ids = %v, want %v", sidecar, want)
	}
}

// TestEnrichVideoRejectsMalformedStudioExternalID is HOLODEX-258's end-to-end guard: a video
// enrich response carrying a malformed _studio_external_ids value (mimicking a
// malicious/compromised provider) must not have that value persisted — proving the call-site
// wiring (sanitizeStudioExternalIDs applied right after sanitizeFields), not just the pure
// function tested by TestSanitizeStudioExternalIDsRejectsMalformedID above.
func TestEnrichVideoRejectsMalformedStudioExternalID(t *testing.T) {
	fake := NewFake("fake")
	fake.People["tmdb:608"] = FakePerson{
		Label: "Men in Black",
		Fields: map[string][]string{
			"title":                      {"Men in Black"},
			"studio":                     {"Amblin Entertainment", "Attacker Studio"},
			model.StudioExternalIDsField: {"tmdb:56 Amblin Entertainment", "noSpaceAtAll"},
		},
	}

	database, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	store, err := NewStore(writeSources(t, `
sources:
  - name: fake
    base_url: http://fake:9100
    entity_types: [person, studio, video]
    enabled: true
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewServiceWithClient(store, r, log, func(Source) ProviderClient { return fake })

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityVideo, 42, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	rows, err := r.EnrichmentForEntity(context.Background(), model.EnrichEntityVideo, 42)
	if err != nil {
		t.Fatalf("enrichment rows: %v", err)
	}
	var sidecar []string
	for _, row := range rows {
		if row.FieldKey == model.StudioExternalIDsField {
			sidecar = row.Values
		}
	}
	want := []string{"tmdb:56 Amblin Entertainment"}
	if len(sidecar) != len(want) || sidecar[0] != want[0] {
		t.Errorf("_studio_external_ids = %v, want %v (malformed entry dropped)", sidecar, want)
	}
}

// TestEnrichVideoClearsStaleStudioExternalIDOnReenrich is HOLODEX-258's self-heal guard: a
// _studio_external_ids row already persisted (e.g. from before this fix shipped, when any shape
// could get stored) must not survive untouched forever just because sanitizeStudioExternalIDs now
// rejects bad values on the write side. UpsertEnrichment only upserts keys present in the fields
// map it's given and never deletes a key merely absent from it, so a naive `delete(fields, ...)`
// when every value is malformed would leave a pre-existing stale row completely untouched. The
// call site must instead keep the key (mapped to an empty slice) whenever the provider actually
// sent it, so the next re-enrich's upsert overwrites the stale row rather than skipping it.
func TestEnrichVideoClearsStaleStudioExternalIDOnReenrich(t *testing.T) {
	fake := NewFake("fake")
	fake.People["tmdb:608"] = FakePerson{
		Label: "Men in Black",
		Fields: map[string][]string{
			"title":                      {"Men in Black"},
			"studio":                     {"Amblin Entertainment"},
			model.StudioExternalIDsField: {"tmdb:56 Amblin Entertainment"},
		},
	}

	database, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	store, err := NewStore(writeSources(t, `
sources:
  - name: fake
    base_url: http://fake:9100
    entity_types: [person, studio, video]
    enabled: true
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewServiceWithClient(store, r, log, func(Source) ProviderClient { return fake })

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityVideo, 43, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich (initial, well-formed): %v", err)
	}

	// Provider now returns only a malformed value on re-enrich — mimics a stale row left
	// over from before this fix shipped, when a compromised provider could persist anything.
	fake.People["tmdb:608"] = FakePerson{
		Label: "Men in Black",
		Fields: map[string][]string{
			"title":                      {"Men in Black"},
			"studio":                     {"Attacker Studio"},
			model.StudioExternalIDsField: {"noSpaceAtAll"},
		},
	}
	if _, err := svc.Enrich(context.Background(), model.EnrichEntityVideo, 43, "fake", "tmdb:608", false); err != nil {
		t.Fatalf("enrich (re-enrich, malformed): %v", err)
	}

	rows, err := r.EnrichmentForEntity(context.Background(), model.EnrichEntityVideo, 43)
	if err != nil {
		t.Fatalf("enrichment rows: %v", err)
	}
	found := false
	for _, row := range rows {
		if row.FieldKey == model.StudioExternalIDsField {
			found = true
			if len(row.Values) != 1 || row.Values[0] != "" {
				t.Errorf("_studio_external_ids = %v, want the stale value cleared after a fully-malformed re-enrich", row.Values)
			}
		}
	}
	if !found {
		t.Fatalf("_studio_external_ids row missing entirely after re-enrich, want a cleared (empty) row")
	}
}

// A people[].headshot always fills the single-occupancy headshot core role, even
// when a (contract-conformant, non-TMDB) provider sends a Kind other than "photo" —
// e.g. "gallery", which for the general assets[] list maps to the capped
// PersonImageExtra role. The headshot field's presence, not its Kind string, is what
// designates it as the headshot; a provider-supplied Kind must never silently divert
// it into the capped gallery instead.
func TestEnrichVideoCreditHeadshotIgnoresProviderKind(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("rawimagebytes"))
	}))
	defer origin.Close()

	fake := NewFake("fake")
	fake.People["tmdb:551"] = FakePerson{
		Label:  "Some Movie",
		Fields: map[string][]string{"title": {"Some Movie"}},
		People: []ProviderPerson{
			{Name: "Someone", Role: "actor", ExternalID: "tmdb:999",
				Headshot: &Asset{Kind: "gallery", URL: origin.URL + "/x.jpg"}},
		},
	}

	database, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	store, err := NewStore(writeSources(t, `
sources:
  - name: fake
    base_url: http://fake:9100
    entity_types: [person, studio, video]
    enabled: true
`), nil)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewServiceWithClient(store, r, log, func(Source) ProviderClient { return fake })
	sink := &recordingSink{}
	svc.SetImageSink(sink)
	svc.newAssetGet = func(Source) assetFetcher { return passthroughFetcher{} }

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityVideo, 99, "fake", "tmdb:551", false); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	for _, a := range sink.stored {
		if a.role == model.PersonImageExtra {
			t.Fatalf("headshot landed in the capped gallery role: %+v", sink.stored)
		}
	}
	found := false
	for _, a := range sink.stored {
		if a.role == model.PersonImageHeadshot {
			found = true
		}
	}
	if !found {
		t.Errorf("stored = %+v, want a headshot-role entry despite provider Kind=gallery", sink.stored)
	}
}

// Untrusted-response bounding (F22.9b).
func TestSanitizeValue(t *testing.T) {
	if got := SanitizeValue("a\x00b\nc"); got != "ab c" {
		t.Errorf("sanitizeValue = %q, want %q", got, "ab c")
	}
	long := strings.Repeat("x", maxFieldLen+10)
	if got := SanitizeValue(long); len(got) != maxFieldLen {
		t.Errorf("length cap = %d, want %d", len(got), maxFieldLen)
	}
}

// A candidate's profile_url becomes a picker `href` client-side (F47, RD6/P1-1),
// so only http(s) survives — everything else, including no-scheme/relative values,
// is dropped rather than erroring.
func TestSanitizeProfileURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://www.themoviedb.org/person/608", "https://www.themoviedb.org/person/608"},
		{"http://example.com/p", "http://example.com/p"},
		{"javascript:alert(1)", ""},
		{"data:text/html,<script>alert(1)</script>", ""},
		{"//evil.example.com", ""},
		{"/relative/path", ""},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := sanitizeProfileURL(c.in); got != c.want {
			t.Errorf("sanitizeProfileURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeFieldsCaps(t *testing.T) {
	vals := make([]string, maxValuesPerField+5)
	for i := range vals {
		vals[i] = "v"
	}
	out := sanitizeFields(map[string][]string{"k": vals, "  ": {"x"}})
	if len(out["k"]) != maxValuesPerField {
		t.Errorf("values per field = %d, want %d", len(out["k"]), maxValuesPerField)
	}
	if _, ok := out[""]; ok {
		t.Error("blank field key should be dropped")
	}
}

// TestSanitizePeopleRejectsWhitespaceInExternalID is the ADR-055/F32 spoofing guard:
// personExternalIDsField round-trips external_id through the "_person_external_ids"
// sidecar as "<external_id> <name>", split back on the FIRST space by
// externalIDsFromRows — an external_id containing a space (or a control character
// SanitizeValue collapses to one, e.g. newline) would let a malicious provider
// smuggle an attacker-chosen id/name pair past that split, resolving a video credit
// onto an unrelated existing Person. sanitizePeople must refuse any such entry, not
// merely trim it.
func TestSanitizePeopleRejectsWhitespaceInExternalID(t *testing.T) {
	in := []ProviderPerson{
		{Name: "Attacker", ExternalID: "tmdb:999 Victim"},  // space inside the id
		{Name: "Attacker", ExternalID: "tmdb:999\nVictim"}, // newline → collapses to space
		{Name: "Legit", ExternalID: "tmdb:287"},            // well-formed, must survive
		{Name: "No Colon", ExternalID: "tmdb999"},          // not namespace-qualified
		{Name: "Empty Namespace", ExternalID: ":999"},
		{Name: "Empty ID", ExternalID: "tmdb:"},
	}
	out := sanitizePeople(in)
	if len(out) != 1 {
		t.Fatalf("sanitizePeople = %+v, want exactly 1 survivor (Legit)", out)
	}
	if out[0].Name != "Legit" || out[0].ExternalID != "tmdb:287" {
		t.Errorf("survivor = %+v, want Legit/tmdb:287", out[0])
	}
}

// TestSanitizeStudioExternalIDsRejectsMalformedID is HOLODEX-258's guard, the studio-sidecar
// sibling of TestSanitizePeopleRejectsWhitespaceInExternalID above: _studio_external_ids is
// provider-authored as one self-describing "<id> <name>" string (no separate structured
// external_id field to validate before construction), so sanitizeStudioExternalIDs must itself
// reject any value whose id token isn't a well-formed "<namespace>:<id>" pair.
func TestSanitizeStudioExternalIDsRejectsMalformedID(t *testing.T) {
	in := []string{
		"tmdb:174 Warner Bros. Pictures", // well-formed, must survive
		"tmdb999 Attacker Studio",        // no colon — not namespace-qualified
		":174 Attacker Studio",           // empty namespace
		"tmdb: Attacker Studio",          // empty id
		"noSpaceAtAll",                   // no separator at all
		"tmdb:174 ",                      // id only, name empty after trim — defensive: not
		// reachable via the real sanitizeFields call site today (its SanitizeValue pass
		// already trims the whole value first, collapsing this into the no-separator case
		// above), but exercised directly here to lock down the function's own contract.
	}
	out := sanitizeStudioExternalIDs(in)
	if len(out) != 1 || out[0] != "tmdb:174 Warner Bros. Pictures" {
		t.Fatalf("sanitizeStudioExternalIDs = %+v, want exactly 1 survivor", out)
	}
}

// TestSanitizeCandidatesAutoApply asserts sanitizeCandidates sets AutoApply from
// Confidence >= StrongMatchThreshold (ADR-066 D1) — the single server-side
// computation every /resolve caller and the frontend's "Strong match" chip now rely
// on instead of a duplicated confidence literal. Table-driven over the boundary,
// mirroring TestSingleStrongMatch's 0/1/2 cases.
func TestSanitizeCandidatesAutoApply(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		want       bool
	}{
		{"below threshold", 0.84, false},
		{"exactly at threshold", StrongMatchThreshold, true},
		{"above threshold", 0.95, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := sanitizeCandidates([]Candidate{{ExternalID: "a", Confidence: tt.confidence}})
			if got := out[0].AutoApply; got != tt.want {
				t.Errorf("AutoApply = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFieldsFromRowsHidesInternal asserts that "_"-prefixed sidecar keys (ADR-054,
// e.g. _studio_external_ids) are never surfaced as display fields — they are
// provider→core plumbing the core reads directly.
func TestFieldsFromRowsHidesInternal(t *testing.T) {
	rows := []repo.EnrichmentRow{
		{Provider: "tmdb", FieldKey: "overview", Values: []string{"A film."}},
		{Provider: "tmdb", FieldKey: model.StudioExternalIDsField, Values: []string{"tmdb:174 Warner Bros."}},
	}
	got := (&Service{}).FieldsFromRows(rows)
	if len(got) != 1 {
		t.Fatalf("FieldsFromRows returned %d fields, want 1 (internal hidden): %+v", len(got), got)
	}
	if got[0].Canonical != "overview" {
		t.Errorf("visible field = %q, want overview", got[0].Canonical)
	}
	for _, f := range got {
		if strings.HasPrefix(f.Canonical, model.InternalFieldPrefix) {
			t.Errorf("internal sidecar field leaked to display: %q", f.Canonical)
		}
	}
}
