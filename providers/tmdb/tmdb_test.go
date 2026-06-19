package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- handler tests (no real TMDB) ----

func TestHealthz(t *testing.T) {
	h := newHandler(nil, newDiscardLogger())
	w := httptest.NewRecorder()
	h.healthz(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body) //nolint:errcheck
	if body["provider"] != "tmdb" {
		t.Errorf("provider = %q, want tmdb", body["provider"])
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestDescribe(t *testing.T) {
	h := newHandler(nil, newDiscardLogger())
	w := httptest.NewRecorder()
	h.describe(w, httptest.NewRequest("GET", "/describe", nil))
	if w.Code != 200 {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	var body describeResponse
	json.NewDecoder(w.Body).Decode(&body) //nolint:errcheck
	if body.ProtocolVersion != 1 {
		t.Errorf("protocol_version = %d, want 1", body.ProtocolVersion)
	}
	if len(body.EntityTypes) == 0 || body.EntityTypes[0] != "person" {
		t.Errorf("entity_types = %v, want [person]", body.EntityTypes)
	}
	if len(body.AssetKinds) == 0 || body.AssetKinds[0] != "photo" {
		t.Errorf("asset_kinds = %v, want [photo]", body.AssetKinds)
	}
	// photo must NOT appear in fields (ADR-039 §2).
	for _, f := range body.Fields {
		if f == "photo" {
			t.Error("photo must not be in fields[] — it belongs in asset_kinds[]")
		}
	}
}

func TestResolveUnknownEntityType(t *testing.T) {
	h := newHandler(nil, newDiscardLogger())
	w := httptest.NewRecorder()
	body := `{"entity_type":"video","hint":{"query":"test"}}`
	h.resolve(w, httptest.NewRequest("POST", "/resolve", strings.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	cands, _ := resp["candidates"].([]any)
	if len(cands) != 0 {
		t.Errorf("candidates = %v, want []", cands)
	}
}

func TestEnrichUnknownEntityType(t *testing.T) {
	h := newHandler(nil, newDiscardLogger())
	w := httptest.NewRecorder()
	body := `{"entity_type":"video","external_id":"tmdb:608"}`
	h.enrich(w, httptest.NewRequest("POST", "/enrich", strings.NewReader(body)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

// ---- TMDB client tests (mocked TMDB API) ----

func TestTMDBResolveNameSearch(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	cands, err := c.resolve(context.Background(), hintBody{Query: "Hayao Miyazaki"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cands) == 0 {
		t.Fatal("no candidates")
	}
	if cands[0].ExternalID != "tmdb:608" {
		t.Errorf("external_id = %q, want tmdb:608", cands[0].ExternalID)
	}
	if cands[0].Namespace != "tmdb" {
		t.Errorf("namespace = %q, want tmdb", cands[0].Namespace)
	}
	if cands[0].Confidence <= 0 || cands[0].Confidence > 1 {
		t.Errorf("confidence %v not in (0,1]", cands[0].Confidence)
	}
}

func TestTMDBResolveNoMatch(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	cands, err := c.resolve(context.Background(), hintBody{Query: "xyzzy_no_match_9999"})
	if err != nil {
		t.Fatalf("resolve no-match: %v", err)
	}
	if len(cands) != 0 {
		t.Errorf("candidates = %v, want []", cands)
	}
}

func TestTMDBResolveByTMDBID(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	cands, err := c.resolve(context.Background(), hintBody{ExternalIDs: []string{"tmdb:608"}})
	if err != nil {
		t.Fatalf("resolve by tmdb id: %v", err)
	}
	if len(cands) != 1 || cands[0].ExternalID != "tmdb:608" {
		t.Errorf("candidates = %v", cands)
	}
	if cands[0].Confidence != 1.0 {
		t.Errorf("confidence = %v, want 1.0 for direct-id path", cands[0].Confidence)
	}
}

func TestTMDBResolveByIMDBID(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	cands, err := c.resolve(context.Background(), hintBody{ExternalIDs: []string{"imdb:nm0594503"}})
	if err != nil {
		t.Fatalf("resolve by imdb id: %v", err)
	}
	if len(cands) == 0 {
		t.Fatal("no candidates for imdb id")
	}
	if cands[0].Namespace != "tmdb" {
		t.Errorf("namespace = %q, want tmdb", cands[0].Namespace)
	}
}

func TestTMDBEnrich(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	res, err := c.enrich(context.Background(), "tmdb:608")
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(res.Fields["bio"]) == 0 {
		t.Error("bio field missing")
	}
	if len(res.Fields["birthdate"]) == 0 {
		t.Error("birthdate field missing")
	}
	if len(res.Fields["aliases"]) == 0 {
		t.Error("aliases field missing")
	}
	if len(res.Assets) == 0 || res.Assets[0].Kind != "photo" {
		t.Errorf("assets = %v, want [{kind:photo,...}]", res.Assets)
	}
	if !strings.HasPrefix(res.Assets[0].URL, "https://image.tmdb.org/") {
		t.Errorf("asset URL = %q, want https://image.tmdb.org/...", res.Assets[0].URL)
	}
}

func TestTMDBEnrichUnknownID(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	_, err := c.enrich(context.Background(), "tmdb:999999999")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestTMDBEnrichBadIDFormat(t *testing.T) {
	c := newTMDBClient("tok", "", "en-US")
	if _, err := c.enrich(context.Background(), "notanid"); err == nil {
		t.Error("expected error for missing namespace prefix")
	}
	if _, err := c.enrich(context.Background(), "imdb:nm0594503"); err == nil {
		t.Error("expected error for non-tmdb namespace")
	}
}

func TestNoSecretsInResponses(t *testing.T) {
	const secret = "super-secret-token-never-log-this"
	h := newHandler(newTMDBClient(secret, "", "en-US"), newDiscardLogger())

	for _, tt := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		req  *http.Request
	}{
		{"healthz", h.healthz, httptest.NewRequest("GET", "/healthz", nil)},
		{"describe", h.describe, httptest.NewRequest("GET", "/describe", nil)},
	} {
		w := httptest.NewRecorder()
		tt.fn(w, tt.req)
		if strings.Contains(w.Body.String(), secret) {
			t.Errorf("%s response leaks the token", tt.name)
		}
	}
}

func TestBioTrimAtSentence(t *testing.T) {
	long := strings.Repeat("sentence. ", 500) // 5000 chars
	result := trimAtSentence(long, 4000)
	if len(result) > 4000 {
		t.Errorf("trimAtSentence result too long: %d", len(result))
	}
	if !strings.HasSuffix(result, ".") {
		t.Errorf("expected sentence boundary trim, got: ...%q", result[len(result)-10:])
	}
}

func TestRankConfidence(t *testing.T) {
	first := rankConfidence(0, 0)
	second := rankConfidence(1, 0)
	if first <= second {
		t.Errorf("rank 0 confidence %v should be > rank 1 confidence %v", first, second)
	}
	if first > 1.0 {
		t.Errorf("confidence %v exceeds 1.0", first)
	}
	withPop := rankConfidence(0, 50)
	if withPop < first {
		t.Errorf("popularity bonus not applied: %v < %v", withPop, first)
	}
	floor := rankConfidence(100, 0)
	if floor < 0.1 {
		t.Errorf("confidence floor < 0.1: %v", floor)
	}
}

func TestEnrichNilAssetWhenNoProfilePath(t *testing.T) {
	det := personDetails{
		ID:          1,
		Name:        "No Photo",
		Biography:   "A person without a profile photo.",
		Birthday:    "1990-01-01",
		ProfilePath: "", // no photo
	}
	res := buildEnrichResponse(det)
	if len(res.Assets) != 0 {
		t.Errorf("expected no assets when ProfilePath is empty, got %v", res.Assets)
	}
}

func TestDisambiguate(t *testing.T) {
	p := tmdbPerson{
		KnownForDepartment: "Directing",
		KnownFor: []knownFor{
			{Title: "Spirited Away", ReleaseDate: "2001-07-20"},
		},
	}
	d := disambiguate(p)
	if !strings.Contains(d, "Directing") {
		t.Errorf("disambiguate = %q, want Directing", d)
	}
	if !strings.Contains(d, "Spirited Away") {
		t.Errorf("disambiguate = %q, want Spirited Away", d)
	}
	if !strings.Contains(d, "2001") {
		t.Errorf("disambiguate = %q, want year 2001", d)
	}
}

// ---- helpers ----

// fakeTMDB returns an httptest.Server that serves the TMDB paths used by the client.
func fakeTMDB(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/3/search/person":
			q := r.URL.Query().Get("query")
			if strings.Contains(strings.ToLower(q), "miyazaki") {
				io.WriteString(w, `{"results":[{"id":608,"name":"Hayao Miyazaki","popularity":25.3,"known_for_department":"Directing","known_for":[{"title":"Spirited Away","release_date":"2001-07-20"}]}]}`) //nolint:errcheck
			} else {
				io.WriteString(w, `{"results":[]}`) //nolint:errcheck
			}
		case r.URL.Path == "/3/person/608":
			io.WriteString(w, `{"id":608,"name":"Hayao Miyazaki","biography":"Japanese filmmaker and co-founder of Studio Ghibli.","birthday":"1941-01-05","place_of_birth":"Bunkyō, Tokyo, Japan","profile_path":"/akhpeJSfFKMValElDDjsKi2jryl.jpg","also_known_as":["宮崎駿","Miyazaki Hayao"]}`) //nolint:errcheck
		case strings.HasPrefix(r.URL.Path, "/3/person/"):
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/3/find/"):
			if strings.Contains(r.URL.Path, "nm0594503") {
				io.WriteString(w, `{"person_results":[{"id":608,"name":"Hayao Miyazaki","popularity":25.3,"known_for_department":"Directing","known_for":[]}]}`) //nolint:errcheck
			} else {
				io.WriteString(w, `{"person_results":[]}`) //nolint:errcheck
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// clientWith returns a tmdbClient wired to hit srv instead of api.themoviedb.org.
func clientWith(srv *httptest.Server) *tmdbClient {
	c := newTMDBClient("test-token", "", "en-US")
	c.hc.Transport = rebaseTransport{base: srv.URL}
	return c
}

// rebaseTransport rewrites request URLs to hit the fake server instead of TMDB.
type rebaseTransport struct{ base string }

func (t rebaseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(t.base, "http://")
	return http.DefaultTransport.RoundTrip(clone)
}

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
