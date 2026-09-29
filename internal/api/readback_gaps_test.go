package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
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

// readbackGapServer maps `overview` to a provider only — a read-back gap, since
// writeback writes it to Matroska's Comment and nothing reads Comment back — and
// `title` to file:title, which is not one. The video is Matroska so the dialog hint
// has a write target to name.
func readbackGapServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "File Title", Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, id, "tmdb", "ext-1", map[string][]string{
		"overview": {"TMDB synopsis."},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [file:title]\n" +
		"  - canonical: overview\n    label: Overview\n    sources: [tmdb:overview]\n"
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
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, id
}

// TestReadbackGaps_OwnerEndpointAndReloadCount pins ADR-119 D4's owner surfaces: the
// gap list is owner-gated and names the tag written and the key to add, and the
// reload-config response carries the count the toast reports.
func TestReadbackGaps_OwnerEndpointAndReloadCount(t *testing.T) {
	srv, _, _ := readbackGapServer(t, "s3cret")
	url := srv.URL + "/api/v1/owner/readback-gaps"

	if code := sendTok(t, http.MethodGet, url, ""); code != http.StatusUnauthorized {
		t.Errorf("no-token list = %d, want 401", code)
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set(api.AdminTokenHeader, "s3cret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var gaps []struct {
		Canonical string   `json:"canonical"`
		WriteTag  string   `json:"write_tag"`
		AddOneOf  []string `json:"add_one_of"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gaps); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("list = %d, %v", resp.StatusCode, err)
	}
	if len(gaps) != 1 || gaps[0].Canonical != "overview" || gaps[0].WriteTag == "" || len(gaps[0].AddOneOf) == 0 {
		t.Fatalf("gaps = %+v, want one overview gap with a write tag and a key to add", gaps)
	}

	code, body := postTok(t, srv.URL+"/api/v1/admin/reload-config", "s3cret", nil)
	if code != http.StatusOK || body["readback_gaps"] != float64(1) {
		t.Errorf("reload = %d %v, want readback_gaps 1", code, body)
	}
}

// TestReadbackGaps_DetailLedgerWitnessAndHint pins ADR-119 D1 and D4 end to end on the
// media detail read: a decided gap field is unknown until Holodex writes it, carries the
// hint for the owner only, and reads in sync once the ledger holds the decided value.
func TestReadbackGaps_DetailLedgerWitnessAndHint(t *testing.T) {
	srv, r, id := readbackGapServer(t, "s3cret")
	detail := srv.URL + "/api/v1/media/" + itoa(id)
	overview := func(token string) map[string]any {
		t.Helper()
		_, body := getJSONTok(t, detail, token)
		fields, _ := body["resolved"].([]any)
		for _, f := range fields {
			if m := f.(map[string]any); m["canonical"] == "overview" {
				return m
			}
		}
		t.Fatalf("no overview row in %v", body["resolved"])
		return nil
	}

	decision := srv.URL + "/api/v1/media/" + itoa(id) + "/fields/overview/decision"
	if code, err := doJSONRequest(http.MethodPut, decision, "s3cret", map[string]any{"source": "provider:tmdb"}); err != nil || code/100 != 2 {
		t.Fatalf("decide = %d, %v", code, err)
	}

	row := overview("s3cret")
	if _, ok := row["in_sync"]; ok {
		t.Errorf("never written: in_sync = %v, want unknown (omitted)", row["in_sync"])
	}
	hint, _ := row["readback_gap"].(map[string]any)
	if hint == nil || hint["write_tag"] != "Comment" {
		t.Errorf("owner row readback_gap = %v, want write_tag Comment", row["readback_gap"])
	}
	if visitor := overview(""); visitor["readback_gap"] != nil {
		t.Errorf("visitor row carries readback_gap %v, want none", visitor["readback_gap"])
	}

	if err := r.InsertWriteback(context.Background(), id, "overview", "Comment", "TMDB synopsis.", "tmdb:overview"); err != nil {
		t.Fatal(err)
	}
	if row := overview("s3cret"); row["in_sync"] != true {
		t.Errorf("written: in_sync = %v, want true (ledger witness)", row["in_sync"])
	}
}
