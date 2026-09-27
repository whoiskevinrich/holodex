package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// TestGetMedia_OffersEmptyOverviewToOwnerOnly covers ADR-113 D1–D3 on the detail
// endpoint: a video with no overview from any source carries a writable, empty
// overview row for the owner (the "+ Add overview" row and the writeback dialog's
// row), and no overview row at all for a visitor.
func TestGetMedia_OffersEmptyOverviewToOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)

	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/keeper.mkv", FileSize: 1, Title: "The Keeper", Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n  - canonical: overview\n    sources: [Comment, tmdb:overview]\n  - canonical: tagline\n    sources: [Description]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	rowFor := func(body map[string]any, canonical string) map[string]any {
		rows, _ := body["resolved"].([]any)
		for _, row := range rows {
			if m, _ := row.(map[string]any); m["canonical"] == canonical {
				return m
			}
		}
		return nil
	}

	// Owner (open gate): the empty overview is offered, writable, and undecided.
	_, body := getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	ov := rowFor(body, "overview")
	if ov == nil {
		t.Fatalf("owner: want an offered overview row in resolved[], got %v", body["resolved"])
	}
	if vals, _ := ov["values"].([]any); len(vals) != 0 {
		t.Errorf("owner: offered overview should have no values, got %v", vals)
	}
	if wt, _ := ov["write_target"].(string); wt == "" {
		t.Errorf("owner: offered overview must carry a write_target so the dialog can write it, got %v", ov["write_target"])
	}
	if dec, _ := ov["decision"].(map[string]any); dec == nil || dec["standing"] == true {
		t.Errorf("owner: offered overview should carry a non-standing decision, got %v", ov["decision"])
	}
	// Offering a row is not supplying a value: the completeness facet stays missing.
	if c, ok := body["completeness"].(map[string]any); ok {
		if f := facetMap(t, c)["overview"]; f == nil || f["tier"] != "missing" {
			t.Errorf("owner: offered overview facet = %v, want tier missing", f)
		}
	} else {
		t.Errorf("owner: completeness missing from the detail: %v", body["completeness"])
	}
	// An unflagged empty field is not offered (per-field adoption, D2).
	if rowFor(body, "tagline") != nil {
		t.Errorf("owner: tagline has not adopted OfferWhenEmpty and must still drop")
	}

	// A standing decision that resolves empty (the decided provider lost its value) is a
	// blank pin, not an offer: it keeps today's unwritable stamp, so the dialog never
	// submits an empty write for it.
	if err := r.SetDecision(context.Background(), model.EnrichEntityVideo, id, "overview", "provider:tmdb", ""); err != nil {
		t.Fatalf("set decision: %v", err)
	}
	_, body = getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	if pinned := rowFor(body, "overview"); pinned == nil {
		t.Fatalf("owner: a standing decision keeps the overview row")
	} else if wt, _ := pinned["write_target"].(string); wt != "" {
		t.Errorf("owner: a blank-pinned overview must stay unwritable, got write_target %q", wt)
	}

	// Visitor: no offered row (D3). The decision above keeps a row regardless (F37 RD3),
	// so clear it first.
	if _, err := r.ClearDecision(context.Background(), model.EnrichEntityVideo, id, "overview"); err != nil {
		t.Fatalf("clear decision: %v", err)
	}
	h.SetAuth(api.NewAuth("secret"), false)
	_, body = getJSON(t, srv.URL+"/api/v1/media/"+itoa(id))
	if rowFor(body, "overview") != nil {
		t.Errorf("visitor: an empty overview must not be offered")
	}
}
