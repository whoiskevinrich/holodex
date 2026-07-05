package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// tinyJPEG encodes a real 8×8 JPEG so the ingest normalizer (decode → re-encode)
// accepts it — a placeholder byte string would be rejected as a non-image.
func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 32), G: uint8(y * 32), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

// studioLogoServer wires the studio surface with a self-hosted-logo store and a
// provider whose base_url is a live "CDN" httptest server that serves the logo JPEG
// on its own host (so the ADR-039 allowlist permits the http download — a non-base
// host would require https). Returns the studio id, the handlers (to drive
// RelinkStudioLogo directly), the studio-logo dir, the CDN URL, and a hit counter.
func studioLogoServer(t *testing.T, token string) (srv *httptest.Server, r *repo.Repo, h *api.Handlers, sid int64, cdnURL string, cdnHits *int32) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r = repo.New(database)
	ctx := context.Background()

	jpg := tinyJPEG(t)
	var hits int32
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/logo.jpg" {
			atomic.AddInt32(&hits, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(jpg)
			return
		}
		http.NotFound(w, req)
	}))
	t.Cleanup(cdn.Close)

	// The provider's base_url IS the CDN host, so its own logo URL is base-host http
	// (allowed) rather than a cross-host CDN (https-required).
	sp := filepath.Join(dir, "sources.yaml")
	yaml := "sources:\n  - name: fake\n    base_url: " + cdn.URL + "\n    entity_types: [studio]\n    enabled: true\n"
	if err := os.WriteFile(sp, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write sources: %v", err)
	}
	store, err := enrich.NewStore(sp)
	if err != nil {
		t.Fatalf("sources store: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := enrich.NewService(store, r, log)

	h = api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetEnrichment(svc)
	h.SetStudioImages(filepath.Join(dir, "studio-logos"), 1000)
	h.SetAuth(api.NewAuth(token), false)
	srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/s.mkv", Title: "Clip",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Studio Ghibli"}, nil); err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("list studios: %v (%d)", err, len(studios))
	}
	return srv, r, h, studios[0].ID, cdn.URL, &hits
}

// seedLogoField stores a `logo` enrichment URL for the studio under provider "fake".
func seedLogoField(t *testing.T, r *repo.Repo, sid int64, url string) {
	t.Helper()
	if err := r.UpsertEnrichment(context.Background(), model.EnrichEntityStudio, sid, "fake", "tmdb:1",
		map[string][]string{"logo": {url}}); err != nil {
		t.Fatalf("seed logo field: %v", err)
	}
}

// TestStudioLogo_DownloadNormalizeServe is the end-to-end happy path: a resolved logo
// URL is fetched through the SSRF perimeter, normalized, stored, and served from our
// own origin with an immutable cache — plus the /studios list carries the served URL.
func TestStudioLogo_DownloadNormalizeServe(t *testing.T) {
	srv, r, h, sid, cdn, hits := studioLogoServer(t, "")
	seedLogoField(t, r, sid, cdn+"/logo.jpg")

	if err := h.RelinkStudioLogo(context.Background(), sid); err != nil {
		t.Fatalf("relink: %v", err)
	}
	if *hits != 1 {
		t.Fatalf("cdn hits = %d, want 1", *hits)
	}

	// Served: 200, image/jpeg, immutable long cache.
	resp, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(sid) + "/logo")
	if err != nil {
		t.Fatalf("get logo: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logo code = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("content-type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc == "" || !bytes.Contains([]byte(cc), []byte("immutable")) {
		t.Fatalf("cache-control = %q, want immutable", cc)
	}
	body, _ := io.ReadAll(resp.Body)
	if _, _, err := image.Decode(bytes.NewReader(body)); err != nil {
		t.Fatalf("served bytes are not a decodable image: %v", err)
	}

	// The list carries the self-hosted served URL (our origin), not the CDN.
	names := studioLogoURLs(t, srv)
	if got := names["Studio Ghibli"]; got != "/api/v1/studios/"+itoa(sid)+"/logo?v="+itoa(logoVersion(t, r, sid)) {
		t.Fatalf("list logo_url = %q", got)
	}

	// Idempotent: same resolved URL → no re-download, same row id.
	before := logoVersion(t, r, sid)
	if err := h.RelinkStudioLogo(context.Background(), sid); err != nil {
		t.Fatalf("relink again: %v", err)
	}
	if *hits != 1 {
		t.Fatalf("cdn hits after idempotent relink = %d, want 1 (no re-download)", *hits)
	}
	if after := logoVersion(t, r, sid); after != before {
		t.Fatalf("logo version changed on no-op relink: %d → %d", before, after)
	}
}

// TestStudioLogo_None404 — a studio with no cached logo serves 404 (the SPA renders
// the monogram), never a placeholder.
func TestStudioLogo_None404(t *testing.T) {
	srv, _, _, sid, _, _ := studioLogoServer(t, "")
	resp, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(sid) + "/logo")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", resp.StatusCode)
	}
}

// TestStudioLogo_BlankPinClears — pinning the logo to the record baseline (blank)
// through the owner endpoint fires the relink trigger and drops the cached logo.
func TestStudioLogo_BlankPinClears(t *testing.T) {
	srv, r, h, sid, cdn, _ := studioLogoServer(t, "")
	seedLogoField(t, r, sid, cdn+"/logo.jpg")
	if err := h.RelinkStudioLogo(context.Background(), sid); err != nil {
		t.Fatalf("relink: %v", err)
	}
	if _, err := r.GetStudioLogo(context.Background(), sid); err != nil {
		t.Fatalf("precondition: logo should be cached: %v", err)
	}

	// Blank-pin the logo (source=record). The decision endpoint relinks the logo.
	code := sendDecision(t, http.MethodPut, srv.URL+"/api/v1/studios/"+itoa(sid)+"/fields/logo/decision", "",
		map[string]string{"source": "record"})
	if code != http.StatusNoContent {
		t.Fatalf("decision code = %d, want 204", code)
	}

	if _, err := r.GetStudioLogo(context.Background(), sid); err == nil {
		t.Fatalf("logo cache should be cleared after blank-pin")
	}
	resp, _ := http.Get(srv.URL + "/api/v1/studios/" + itoa(sid) + "/logo")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("logo code after blank-pin = %d, want 404", resp.StatusCode)
	}
}

// TestStudioLogo_SSRFRefused — a resolved logo URL on a host outside the provider's
// allowlist is refused by FetchAsset; nothing is cached (the ADR-039 perimeter holds
// for the studio-logo path, not just person portraits).
func TestStudioLogo_SSRFRefused(t *testing.T) {
	_, r, h, sid, _, _ := studioLogoServer(t, "")

	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(tinyJPEG(t))
	}))
	defer evil.Close()
	seedLogoField(t, r, sid, evil.URL+"/logo.jpg") // not base host, not asset_host → refused

	if err := h.RelinkStudioLogo(context.Background(), sid); err == nil {
		t.Fatalf("relink should refuse an off-allowlist logo host")
	}
	if _, err := r.GetStudioLogo(context.Background(), sid); err == nil {
		t.Fatalf("nothing should be cached after an SSRF refusal")
	}
}

// studioLogoURLs maps each listed studio's name to its logo_url (empty when absent).
func studioLogoURLs(t *testing.T, srv *httptest.Server) map[string]string {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/studios")
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Items []struct {
			Name    string `json:"name"`
			LogoURL string `json:"logo_url"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode studios: %v", err)
	}
	out := make(map[string]string, len(body.Items))
	for _, s := range body.Items {
		out[s.Name] = s.LogoURL
	}
	return out
}

// logoVersion returns the studio's cached logo row id (0 if none).
func logoVersion(t *testing.T, r *repo.Repo, sid int64) int64 {
	t.Helper()
	s, err := r.GetStudio(context.Background(), sid)
	if err != nil {
		t.Fatalf("get studio: %v", err)
	}
	return s.LogoVersion
}
