package enrich

import (
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
`))
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
	reg, err := Load(filepath.Join(t.TempDir(), "none.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(reg.Enabled()) != 0 {
		t.Error("missing file should yield no providers")
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
`))
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
	// logo is a plain image_url field, rendered directly — never on the person-image
	// asset-download path (which is gated to person; studios have no image store).
	if got["logo"].Display != "image_url" {
		t.Errorf("logo display = %q, want image_url", got["logo"].Display)
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
	personID   int64
	role       string
	provider   string
	externalID string
	url        string
	bytes      int
	overCap    bool
}

func (s *recordingSink) StoreAsset(_ context.Context, personID int64, role, provider, externalID, url string, raw []byte, overCap bool) error {
	s.stored = append(s.stored, storedAsset{personID, role, provider, externalID, url, len(raw), overCap})
	return nil
}

// StoreAssetIfAbsent records a core-role store only when that (person, role) slot has
// not already been filled in this run — mirroring the real sink's empty-slot guard.
func (s *recordingSink) StoreAssetIfAbsent(ctx context.Context, personID int64, role, provider, externalID, url string, raw []byte) error {
	for _, a := range s.stored {
		if a.personID == personID && a.role == role {
			return nil // slot already filled
		}
	}
	return s.StoreAsset(ctx, personID, role, provider, externalID, url, raw, false)
}

func (s *recordingSink) SuppressedAssetURLs(_ context.Context, _ int64) (map[string]struct{}, error) {
	return s.suppress, nil
}

func (s *recordingSink) LockedCoreRoles(_ context.Context, _ int64) (map[string]struct{}, error) {
	return s.locked, nil
}

func (s *recordingSink) ExistingAssetURLs(_ context.Context, _ int64) (map[string]struct{}, error) {
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
