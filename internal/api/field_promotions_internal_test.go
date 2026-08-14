package api

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"holodex/internal/enrich"
	"holodex/internal/registry"
	"holodex/internal/resolver"
)

// gateTestHandlers builds a *Handlers wired to a real enrich.Service over two
// declared providers with distinct allowlists, so gateImageURL can be exercised
// against a genuine ADR-039 host check rather than a stub.
func gateTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	dir := t.TempDir()
	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte(`sources:
  - name: tmdb
    base_url: https://tmdb.example
    entity_types: [video]
    enabled: true
    asset_hosts: [image.tmdb.org]
  - name: other
    base_url: https://other.example
    entity_types: [video]
    enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := enrich.NewStore(sp, log)
	if err != nil {
		t.Fatal(err)
	}
	return &Handlers{enrich: enrich.NewServiceWithClient(store, nil, log, func(enrich.Source) enrich.ProviderClient {
		return enrich.NewFake("fake")
	})}
}

// TestGateImageURL_MergedField covers the ADR-039/056 allowlist gate on a field
// merged from more than one provider (HOLODEX-212): gateImageURL must check every
// item, not just the first — a merge/auto-registered field (F30/F39) can carry
// values from more than one provider, and checking only the first would leave a
// second, disallowed provider free to smuggle an unvetted URL into the same field.
func TestGateImageURL_MergedField(t *testing.T) {
	h := gateTestHandlers(t)

	allowedItem := resolver.ResolvedValue{Value: "https://image.tmdb.org/a.jpg", Sources: []string{"tmdb"}}
	disallowedItem := resolver.ResolvedValue{Value: "https://evil.example/b.jpg", Sources: []string{"other"}}

	if got := h.gateImageURL(registry.DisplayImageURL, []resolver.ResolvedValue{allowedItem, disallowedItem}); got != registry.DisplayText {
		t.Errorf("one disallowed merged value must degrade the whole field, got %q", got)
	}
	if got := h.gateImageURL(registry.DisplayImageURL, []resolver.ResolvedValue{allowedItem}); got != registry.DisplayImageURL {
		t.Errorf("every value allowed must keep image display, got %q", got)
	}
}
