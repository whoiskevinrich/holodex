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
    entity_types: [person]
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

	fields, err := svc.Enrich(ctx, model.EnrichEntityPerson, 1, "fake", "tmdb:608")
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
	if _, err := svc.Enrich(ctx, model.EnrichEntityPerson, 7, "fake", "tmdb:608"); err != nil {
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
// orchestration is testable with no filesystem.
type recordingSink struct {
	stored []storedAsset
}

type storedAsset struct {
	personID   int64
	role       string
	provider   string
	externalID string
	bytes      int
}

func (s *recordingSink) StoreAsset(_ context.Context, personID int64, role, provider, externalID string, raw []byte) error {
	s.stored = append(s.stored, storedAsset{personID, role, provider, externalID, len(raw)})
	return nil
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

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608"); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(sink.stored) != 2 {
		t.Fatalf("stored %d assets, want 2 (unknown kind skipped): %+v", len(sink.stored), sink.stored)
	}
	roles := map[string]bool{}
	for _, a := range sink.stored {
		roles[a.role] = true
		if a.provider != "fake" || a.externalID != "tmdb:608" {
			t.Errorf("asset provenance = %+v, want provider=fake external=tmdb:608", a)
		}
	}
	if !roles[model.PersonImageHeadshot] || !roles[model.PersonImageBanner] {
		t.Errorf("asset roles = %v, want headshot+banner", roles)
	}
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

	if _, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 5, "fake", "tmdb:608"); err != nil {
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
	fields, err := svc.Enrich(context.Background(), model.EnrichEntityPerson, 9, "fake", "tmdb:608")
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
