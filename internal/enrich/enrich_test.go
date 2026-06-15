package enrich

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

// Untrusted-response bounding (F22.9b).
func TestSanitizeValue(t *testing.T) {
	if got := sanitizeValue("a\x00b\nc"); got != "ab c" {
		t.Errorf("sanitizeValue = %q, want %q", got, "ab c")
	}
	long := strings.Repeat("x", maxFieldLen+10)
	if got := sanitizeValue(long); len(got) != maxFieldLen {
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
