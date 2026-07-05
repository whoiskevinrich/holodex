package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/repo"
)

// iconEnv wires the provider surface with a self-hosted-icon store and a fake provider
// whose base_url is a live httptest server serving BOTH /describe (advertising a
// brand_icon) and the icon JPEG on its own host — so the ADR-059 download rides the
// ADR-039 base-host allowlist (http is permitted for the base host; a cross-host URL
// would require https). setBrand swaps the URL /describe advertises.
type iconEnv struct {
	srv         *httptest.Server
	repo        *repo.Repo
	h           *api.Handlers
	svc         *enrich.Service
	iconHits    *int32
	base        string
	sourcesPath string
	setBrand    func(url string)
}

func newProviderIconEnv(t *testing.T) *iconEnv {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)

	jpg := tinyJPEG(t)
	var iconHits int32
	var brand atomic.Value // string; "" → omit brand_icon
	brand.Store("")

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/describe":
			m := map[string]any{
				"provider":         "fake",
				"version":          "1.0.0",
				"protocol_version": 1,
				"entity_types":     []string{"person"},
				"id_namespaces":    []string{"fake"},
				"fields":           []string{"bio"},
			}
			if b := brand.Load().(string); b != "" {
				m["brand_icon"] = map[string]string{"url": b}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(m)
		case "/icon.jpg":
			atomic.AddInt32(&iconHits, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(jpg)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(cdn.Close)

	sp := filepath.Join(dir, "sources.yaml")
	yaml := "sources:\n  - name: fake\n    base_url: " + cdn.URL + "\n    entity_types: [person]\n    enabled: true\n"
	if err := os.WriteFile(sp, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write sources: %v", err)
	}
	store, err := enrich.NewStore(sp)
	if err != nil {
		t.Fatalf("sources store: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := enrich.NewService(store, r, log)

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetEnrichment(svc)
	h.SetProviderIcons(filepath.Join(dir, "provider-icons"), 256)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	return &iconEnv{
		srv: srv, repo: r, h: h, svc: svc, iconHits: &iconHits, base: cdn.URL, sourcesPath: sp,
		setBrand: func(u string) { brand.Store(u) },
	}
}

// providersDirectory maps provider name → icon_url from the public /providers directory.
func providersDirectory(t *testing.T, srv *httptest.Server) map[string]string {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/providers")
	if err != nil {
		t.Fatalf("get providers: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Providers []struct {
			Name    string `json:"name"`
			IconURL string `json:"icon_url"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode providers: %v", err)
	}
	out := make(map[string]string, len(body.Providers))
	for _, p := range body.Providers {
		out[p.Name] = p.IconURL
	}
	return out
}

// TestProviderIcon_DownloadNormalizeServe is the end-to-end happy path: the advertised
// brand_icon is fetched through the SSRF perimeter, normalized, stored, served from our
// origin with an immutable cache, and surfaced on the public /providers directory —
// plus a re-sync with an unchanged URL is a no-op (no re-download).
func TestProviderIcon_DownloadNormalizeServe(t *testing.T) {
	env := newProviderIconEnv(t)
	env.setBrand(env.base + "/icon.jpg")

	if err := env.h.RelinkProviderIcon(context.Background(), "fake"); err != nil {
		t.Fatalf("relink: %v", err)
	}
	if *env.iconHits != 1 {
		t.Fatalf("icon fetches = %d, want 1", *env.iconHits)
	}

	icon, err := env.repo.GetProviderIcon(context.Background(), "fake")
	if err != nil {
		t.Fatalf("get provider icon: %v", err)
	}

	// Served: 200, image/jpeg, immutable long cache, decodable image.
	resp, err := http.Get(env.srv.URL + "/api/v1/providers/fake/icon")
	if err != nil {
		t.Fatalf("get icon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("icon code = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("content-type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !bytes.Contains([]byte(cc), []byte("immutable")) {
		t.Fatalf("cache-control = %q, want immutable", cc)
	}
	body, _ := io.ReadAll(resp.Body)
	if _, _, err := image.Decode(bytes.NewReader(body)); err != nil {
		t.Fatalf("served bytes are not a decodable image: %v", err)
	}

	// The public directory carries the served URL (our origin), cache-busted by row id.
	if got := providersDirectory(t, env.srv)["fake"]; got != "/api/v1/providers/fake/icon?v="+itoa(icon.ID) {
		t.Fatalf("directory icon_url = %q", got)
	}

	// Idempotent: same advertised URL → no re-download, same row id.
	if err := env.h.RelinkProviderIcon(context.Background(), "fake"); err != nil {
		t.Fatalf("relink again: %v", err)
	}
	if *env.iconHits != 1 {
		t.Fatalf("icon fetches after no-op relink = %d, want 1", *env.iconHits)
	}
	if after, _ := env.repo.GetProviderIcon(context.Background(), "fake"); after.ID != icon.ID {
		t.Fatalf("row id changed on no-op relink: %d → %d", icon.ID, after.ID)
	}
}

// TestProviderIcon_NoneAnd404 — a provider that advertises no brand_icon caches nothing
// and its icon route 404s (the SPA renders a monogram); the directory omits icon_url.
func TestProviderIcon_NoneAnd404(t *testing.T) {
	env := newProviderIconEnv(t)
	// No setBrand → /describe omits brand_icon.
	if err := env.h.RelinkProviderIcon(context.Background(), "fake"); err != nil {
		t.Fatalf("relink: %v", err)
	}
	if *env.iconHits != 0 {
		t.Fatalf("icon fetches = %d, want 0 (no brand_icon advertised)", *env.iconHits)
	}
	resp, err := http.Get(env.srv.URL + "/api/v1/providers/fake/icon")
	if err != nil {
		t.Fatalf("get icon: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("icon code = %d, want 404", resp.StatusCode)
	}
	if got, ok := providersDirectory(t, env.srv)["fake"]; !ok || got != "" {
		t.Fatalf("directory icon_url = %q (present=%v), want empty", got, ok)
	}
}

// TestProviderIcon_SSRFRefused — a brand_icon on a host outside the provider's allowlist
// is refused by FetchAsset; nothing is cached (the ADR-039 perimeter holds for the
// provider-icon download too).
func TestProviderIcon_SSRFRefused(t *testing.T) {
	env := newProviderIconEnv(t)
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(tinyJPEG(t))
	}))
	defer evil.Close()
	env.setBrand(evil.URL + "/icon.jpg") // not base host, not asset_host → refused

	if err := env.h.RelinkProviderIcon(context.Background(), "fake"); err == nil {
		t.Fatalf("relink should refuse an off-allowlist brand_icon host")
	}
	if _, err := env.repo.GetProviderIcon(context.Background(), "fake"); err == nil {
		t.Fatalf("nothing should be cached after an SSRF refusal")
	}
}

// TestProviderIcon_ReconcilePrunesOrphan — RefreshProviderIcons drops the cached icon
// of a provider that has left the registry (there is no FK cascade; the boot/reload
// reconcile is what prunes it).
func TestProviderIcon_ReconcilePrunesOrphan(t *testing.T) {
	env := newProviderIconEnv(t)
	env.setBrand(env.base + "/icon.jpg")
	if err := env.h.RelinkProviderIcon(context.Background(), "fake"); err != nil {
		t.Fatalf("relink: %v", err)
	}
	if _, err := env.repo.GetProviderIcon(context.Background(), "fake"); err != nil {
		t.Fatalf("precondition: icon should be cached: %v", err)
	}

	// Drop the provider from the registry and reload, then reconcile.
	if err := os.WriteFile(env.sourcesPath, []byte("sources: []\n"), 0o644); err != nil {
		t.Fatalf("rewrite sources: %v", err)
	}
	if err := env.svc.Store().Reload(); err != nil {
		t.Fatalf("reload sources: %v", err)
	}
	env.h.RefreshProviderIcons(context.Background())

	if _, err := env.repo.GetProviderIcon(context.Background(), "fake"); err == nil {
		t.Fatalf("orphan icon should be pruned after the provider left the registry")
	}
	resp, err := http.Get(env.srv.URL + "/api/v1/providers/fake/icon")
	if err != nil {
		t.Fatalf("get icon: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("icon code after prune = %d, want 404", resp.StatusCode)
	}
}
