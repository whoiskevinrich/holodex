package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	hasType := func(want string) bool {
		for _, et := range body.EntityTypes {
			if et == want {
				return true
			}
		}
		return false
	}
	if !hasType("person") {
		t.Errorf("entity_types = %v, want to contain person", body.EntityTypes)
	}
	if !hasType("video") {
		t.Errorf("entity_types = %v, want to contain video", body.EntityTypes)
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
	body := `{"entity_type":"series","hint":{"query":"test"}}`
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
	body := `{"entity_type":"series","external_id":"tmdb:608"}`
	h.enrich(w, httptest.NewRequest("POST", "/enrich", strings.NewReader(body)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

// ---- TMDB client tests (mocked TMDB API) ----

func TestTMDBResolveNameSearch(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	cands, err := c.resolve(context.Background(), hintBody{Query: "Hayao Miyazaki"}, "person")
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

	cands, err := c.resolve(context.Background(), hintBody{Query: "xyzzy_no_match_9999"}, "person")
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

	cands, err := c.resolve(context.Background(), hintBody{ExternalIDs: []string{"tmdb:608"}}, "person")
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

	cands, err := c.resolve(context.Background(), hintBody{ExternalIDs: []string{"imdb:nm0594503"}}, "person")
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

	res, err := c.enrich(context.Background(), "tmdb:608", "person")
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
	// website is the person's TMDB page (not their personal site).
	if got := res.Fields["website"]; len(got) == 0 || got[0] != "https://www.themoviedb.org/person/608-hayao-miyazaki" {
		t.Errorf("website = %v, want [https://www.themoviedb.org/person/608-hayao-miyazaki]", got)
	}
	// Expect: headshot (first profile) + gallery (second profile) + banner (first backdrop).
	if len(res.Assets) < 3 {
		t.Errorf("want ≥3 assets (headshot+gallery+banner), got %d: %v", len(res.Assets), res.Assets)
	}
	if res.Assets[0].Kind != "headshot" {
		t.Errorf("assets[0].kind = %q, want headshot", res.Assets[0].Kind)
	}
	if !strings.HasPrefix(res.Assets[0].URL, "https://image.tmdb.org/") {
		t.Errorf("asset URL = %q, want https://image.tmdb.org/...", res.Assets[0].URL)
	}
	if res.Assets[1].Kind != "gallery" {
		t.Errorf("assets[1].kind = %q, want gallery", res.Assets[1].Kind)
	}
	// Banner should be present (backdrop1.jpg, aspect_ratio 1.778 from tagged_images).
	hasBanner := false
	for _, a := range res.Assets {
		if a.Kind == "banner" {
			hasBanner = true
		}
	}
	if !hasBanner {
		t.Errorf("no banner asset found in %v", res.Assets)
	}
}

func TestTMDBEnrichUnknownID(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	_, err := c.enrich(context.Background(), "tmdb:999999999", "person")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestTMDBEnrichBadIDFormat(t *testing.T) {
	c := newTMDBClient("tok", "", "en-US")
	if _, err := c.enrich(context.Background(), "notanid", "person"); err == nil {
		t.Error("expected error for missing namespace prefix")
	}
	if _, err := c.enrich(context.Background(), "imdb:nm0594503", "person"); err == nil {
		t.Error("expected error for non-tmdb namespace")
	}
}

func TestTMDBResolveMovieByName(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	cands, err := c.resolve(context.Background(), hintBody{Query: "Fight Club"}, "video")
	if err != nil {
		t.Fatalf("resolve movie: %v", err)
	}
	if len(cands) == 0 {
		t.Fatal("no movie candidates")
	}
	if cands[0].ExternalID != "tmdb:550" {
		t.Errorf("external_id = %q, want tmdb:550", cands[0].ExternalID)
	}
	if cands[0].Disambiguation == "" {
		t.Error("movie candidate should have a year in disambiguation")
	}
}

func TestTMDBResolveMovieByTMDBID(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	cands, err := c.resolve(context.Background(), hintBody{ExternalIDs: []string{"tmdb:550"}}, "video")
	if err != nil {
		t.Fatalf("resolve movie by id: %v", err)
	}
	if len(cands) != 1 || cands[0].ExternalID != "tmdb:550" {
		t.Errorf("candidates = %v", cands)
	}
	if cands[0].Confidence != 1.0 {
		t.Errorf("confidence = %v, want 1.0 for direct-id path", cands[0].Confidence)
	}
}

func TestTMDBResolveMovieByIMDBID(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	cands, err := c.resolve(context.Background(), hintBody{ExternalIDs: []string{"imdb:tt0137523"}}, "video")
	if err != nil {
		t.Fatalf("resolve movie by imdb id: %v", err)
	}
	if len(cands) == 0 {
		t.Fatal("no movie candidates for imdb id")
	}
	if cands[0].Namespace != "tmdb" {
		t.Errorf("namespace = %q, want tmdb", cands[0].Namespace)
	}
}

func TestTMDBEnrichMovie(t *testing.T) {
	srv := fakeTMDB(t)
	c := clientWith(srv)

	res, err := c.enrich(context.Background(), "tmdb:550", "video")
	if err != nil {
		t.Fatalf("enrich movie: %v", err)
	}
	if len(res.Fields["overview"]) == 0 {
		t.Error("overview field missing")
	}
	if len(res.Fields["release_date"]) == 0 {
		t.Error("release_date field missing")
	}
	if len(res.Fields["genres"]) == 0 {
		t.Error("genres field missing")
	}
	if len(res.Fields["poster_url"]) == 0 {
		t.Error("poster_url field missing")
	}
	if !strings.HasPrefix(res.Fields["poster_url"][0], "https://image.tmdb.org/") {
		t.Errorf("poster_url = %q, want https://image.tmdb.org/...", res.Fields["poster_url"][0])
	}
	// Credits-sourced fields.
	if len(res.Fields["actors"]) == 0 {
		t.Error("actors field missing")
	}
	if res.Fields["actors"][0] != "Brad Pitt" {
		t.Errorf("actors[0] = %q, want Brad Pitt", res.Fields["actors"][0])
	}
	if len(res.Fields["director"]) == 0 || res.Fields["director"][0] != "David Fincher" {
		t.Errorf("director = %v, want [David Fincher]", res.Fields["director"])
	}
	if len(res.Fields["studio"]) == 0 {
		t.Error("studio field missing")
	}
	// The _studio_external_ids sidecar (ADR-054) carries each company's TMDB id,
	// self-describing as "tmdb:<id> <name>" and aligned with the studio names.
	sidecar := res.Fields[studioExternalIDsField]
	if len(sidecar) != 2 {
		t.Fatalf("_studio_external_ids = %v, want 2 entries", sidecar)
	}
	if sidecar[0] != "tmdb:508 Regency Enterprises" {
		t.Errorf("sidecar[0] = %q, want %q", sidecar[0], "tmdb:508 Regency Enterprises")
	}
	if sidecar[1] != "tmdb:711 Fox 2000 Pictures" {
		t.Errorf("sidecar[1] = %q, want %q", sidecar[1], "tmdb:711 Fox 2000 Pictures")
	}
	// homepage is the movie's TMDB page (not the studio's official site).
	if got := res.Fields["homepage"]; len(got) == 0 || got[0] != "https://www.themoviedb.org/movie/550-fight-club" {
		t.Errorf("homepage = %v, want [https://www.themoviedb.org/movie/550-fight-club]", got)
	}
	// Movie fields go into Fields, not Assets.
	if len(res.Assets) != 0 {
		t.Errorf("movie enrich should have no assets, got %v", res.Assets)
	}
}

func TestTMDBEnrichMovieNoPoster(t *testing.T) {
	det := movieDetails{
		ID:          1,
		Title:       "Silent Film",
		Overview:    "A film with no poster.",
		ReleaseDate: "1920-01-01",
		PosterPath:  "",
	}
	res := buildMovieEnrichResponse(det, movieCredits{})
	if _, ok := res.Fields["poster_url"]; ok {
		t.Error("poster_url should not be set when PosterPath is empty")
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
	res := buildEnrichResponse(det, personImagesResult{}, taggedImagesResult{})
	if len(res.Assets) != 0 {
		t.Errorf("expected no assets when ProfilePath is empty, got %v", res.Assets)
	}
}

func TestBuildEnrichResponseMultiplePhotos(t *testing.T) {
	det := personDetails{ID: 1, Name: "Test Person", ProfilePath: "/fallback.jpg"}
	imgs := personImagesResult{
		Profiles: []personProfile{
			{FilePath: "/photo1.jpg", AspectRatio: 0.667},
			{FilePath: "/photo2.jpg", AspectRatio: 0.667},
			{FilePath: "/photo3.jpg", AspectRatio: 0.667},
		},
	}
	tags := taggedImagesResult{
		Results: []taggedImageEntry{
			{FilePath: "/portrait_tagged.jpg", AspectRatio: 0.667}, // skipped: portrait
			{FilePath: "/backdrop.jpg", AspectRatio: 1.778},         // banner
		},
	}
	res := buildEnrichResponse(det, imgs, tags)

	if len(res.Assets) != 4 { // 1 headshot + 2 gallery + 1 banner
		t.Fatalf("want 4 assets, got %d: %v", len(res.Assets), res.Assets)
	}
	if res.Assets[0].Kind != "headshot" || res.Assets[0].URL != "https://image.tmdb.org/t/p/original/photo1.jpg" {
		t.Errorf("assets[0] = %+v, want headshot /photo1.jpg", res.Assets[0])
	}
	if res.Assets[1].Kind != "gallery" {
		t.Errorf("assets[1].kind = %q, want gallery", res.Assets[1].Kind)
	}
	if res.Assets[2].Kind != "gallery" {
		t.Errorf("assets[2].kind = %q, want gallery", res.Assets[2].Kind)
	}
	if res.Assets[3].Kind != "banner" || res.Assets[3].URL != "https://image.tmdb.org/t/p/original/backdrop.jpg" {
		t.Errorf("assets[3] = %+v, want banner /backdrop.jpg", res.Assets[3])
	}
}

func TestBuildEnrichResponseFallsBackToProfilePath(t *testing.T) {
	det := personDetails{ID: 1, Name: "Fallback Person", ProfilePath: "/main.jpg"}
	// Empty images result (e.g. the /images call failed)
	res := buildEnrichResponse(det, personImagesResult{}, taggedImagesResult{})
	if len(res.Assets) != 1 {
		t.Fatalf("want 1 asset (fallback), got %d: %v", len(res.Assets), res.Assets)
	}
	if res.Assets[0].Kind != "headshot" {
		t.Errorf("assets[0].kind = %q, want headshot", res.Assets[0].Kind)
	}
}

func TestBuildEnrichResponseSkipsEmptyFilePath(t *testing.T) {
	// A profile with an empty file_path must be skipped (continue), not terminate
	// the loop (break) — later valid profiles still produce assets, and the headshot
	// goes to the first kept entry, not the first array index.
	det := personDetails{ID: 1, Name: "Sparse Photos"}
	imgs := personImagesResult{
		Profiles: []personProfile{
			{FilePath: ""},           // malformed — skipped
			{FilePath: "/good1.jpg"}, // becomes the headshot
			{FilePath: ""},           // malformed — skipped
			{FilePath: "/good2.jpg"}, // gallery
		},
	}
	res := buildEnrichResponse(det, imgs, taggedImagesResult{})
	if len(res.Assets) != 2 {
		t.Fatalf("want 2 assets (empties skipped), got %d: %v", len(res.Assets), res.Assets)
	}
	if res.Assets[0].Kind != "headshot" || res.Assets[0].URL != "https://image.tmdb.org/t/p/original/good1.jpg" {
		t.Errorf("assets[0] = %+v, want headshot /good1.jpg", res.Assets[0])
	}
	if res.Assets[1].Kind != "gallery" || res.Assets[1].URL != "https://image.tmdb.org/t/p/original/good2.jpg" {
		t.Errorf("assets[1] = %+v, want gallery /good2.jpg", res.Assets[1])
	}
}

func TestBuildEnrichResponseCapsAt20(t *testing.T) {
	det := personDetails{ID: 1, Name: "Many Photos"}
	var profiles []personProfile
	for i := range 25 {
		profiles = append(profiles, personProfile{FilePath: fmt.Sprintf("/photo%d.jpg", i), AspectRatio: 0.667})
	}
	res := buildEnrichResponse(det, personImagesResult{Profiles: profiles}, taggedImagesResult{})
	if len(res.Assets) != maxPersonPhotos {
		t.Errorf("want %d assets (cap), got %d", maxPersonPhotos, len(res.Assets))
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
		case r.URL.Path == "/3/person/608/images":
			io.WriteString(w, `{"profiles":[{"file_path":"/akhpeJSfFKMValElDDjsKi2jryl.jpg","aspect_ratio":0.667,"vote_average":5.4},{"file_path":"/secondprofile.jpg","aspect_ratio":0.667,"vote_average":4.8}]}`) //nolint:errcheck
		case r.URL.Path == "/3/person/608/tagged_images":
			io.WriteString(w, `{"results":[{"file_path":"/backdrop1.jpg","aspect_ratio":1.778},{"file_path":"/portrait_tagged.jpg","aspect_ratio":0.667}]}`) //nolint:errcheck
		case strings.HasPrefix(r.URL.Path, "/3/person/"):
			http.NotFound(w, r)
		case r.URL.Path == "/3/search/movie":
			q := r.URL.Query().Get("query")
			if strings.Contains(strings.ToLower(q), "fight club") {
				io.WriteString(w, `{"results":[{"id":550,"title":"Fight Club","release_date":"1999-10-15","popularity":42.1}]}`) //nolint:errcheck
			} else {
				io.WriteString(w, `{"results":[]}`) //nolint:errcheck
			}
		case r.URL.Path == "/3/movie/550/credits":
			io.WriteString(w, `{"cast":[{"name":"Brad Pitt","order":0},{"name":"Edward Norton","order":1},{"name":"Helena Bonham Carter","order":2}],"crew":[{"name":"David Fincher","job":"Director"},{"name":"Art Linson","job":"Producer"}]}`) //nolint:errcheck
		case r.URL.Path == "/3/movie/550":
			io.WriteString(w, `{"id":550,"title":"Fight Club","original_title":"Fight Club","overview":"An insomniac office worker forms an underground fight club.","release_date":"1999-10-15","runtime":139,"genres":[{"name":"Drama"},{"name":"Thriller"}],"tagline":"Mischief. Mayhem. Soap.","original_language":"en","status":"Released","imdb_id":"tt0137523","poster_path":"/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg","production_companies":[{"id":508,"name":"Regency Enterprises"},{"id":711,"name":"Fox 2000 Pictures"}]}`) //nolint:errcheck
		case strings.HasPrefix(r.URL.Path, "/3/movie/"):
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/3/find/"):
			if strings.Contains(r.URL.Path, "nm0594503") {
				io.WriteString(w, `{"person_results":[{"id":608,"name":"Hayao Miyazaki","popularity":25.3,"known_for_department":"Directing","known_for":[]}],"movie_results":[]}`) //nolint:errcheck
			} else if strings.Contains(r.URL.Path, "tt0137523") {
				io.WriteString(w, `{"person_results":[],"movie_results":[{"id":550,"title":"Fight Club","release_date":"1999-10-15","popularity":42.1}]}`) //nolint:errcheck
			} else {
				io.WriteString(w, `{"person_results":[],"movie_results":[]}`) //nolint:errcheck
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

func TestParseReleaseFilename(t *testing.T) {
	cases := []struct {
		in    string
		title string
		year  string
	}{
		// Standard release-group patterns
		{"Dune.2021.2160p.HMAX.WEB-DL.DDP5.1.Atmos.HDR.HEVC-CMRG", "Dune", "2021"},
		{"Dune.Part.Two.2024.2160p.HMAX.WEB-DL", "Dune Part Two", "2024"},
		{"Fight.Club.1999.1080p.BluRay.x264", "Fight Club", "1999"},
		{"2001.A.Space.Odyssey.1968.1080p.BluRay", "2001 A Space Odyssey", "1968"},
		// Fewer than 3 dots — not a recognisable filename, pass through unchanged
		{"Rio.2011", "Rio.2011", ""},
		// Plain search queries — must pass through unchanged
		{"Dune", "Dune", ""},
		{"Fight Club", "Fight Club", ""},
		{"Dune 2021", "Dune 2021", ""},   // spaces not dots, < 3 dots
		{"Dune.2021", "Dune.2021", ""},   // only 1 dot — too few
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			title, year := parseReleaseFilename(tc.in)
			if title != tc.title || year != tc.year {
				t.Errorf("parseReleaseFilename(%q) = (%q, %q), want (%q, %q)",
					tc.in, title, year, tc.title, tc.year)
			}
		})
	}
}

func TestTMDBMovieURL(t *testing.T) {
	cases := []struct {
		id    int
		title string
		want  string
	}{
		{438631, "Dune", "https://www.themoviedb.org/movie/438631-dune"},
		{693134, "Dune: Part Two", "https://www.themoviedb.org/movie/693134-dune-part-two"},
		{62, "2001: A Space Odyssey", "https://www.themoviedb.org/movie/62-2001-a-space-odyssey"},
		// Latin diacritics fold to ASCII (é→e), matching TMDB's slug form.
		{194, "Amélie", "https://www.themoviedb.org/movie/194-amelie"},
		// Non-Latin title slugifies to empty → bare-id URL (TMDB redirects to the slug).
		{129, "千と千尋の神隠し", "https://www.themoviedb.org/movie/129"},
	}
	for _, tc := range cases {
		if got := tmdbMovieURL(tc.id, tc.title); got != tc.want {
			t.Errorf("tmdbMovieURL(%d, %q) = %q, want %q", tc.id, tc.title, got, tc.want)
		}
	}
}

// TestSlugifyConcurrent exercises slugify from many goroutines. slugify builds its
// transform.Chain per call precisely because the transformer is stateful; this guards
// against a regression to a shared one — `go test -race` (CI) would flag the race.
func TestSlugifyConcurrent(t *testing.T) {
	const want = "https://www.themoviedb.org/movie/194-amelie-le-fabuleux-destin"
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := tmdbMovieURL(194, "Amélie: Le Fabuleux Destin"); got != want {
				t.Errorf("tmdbMovieURL = %q, want %q", got, want)
			}
		}()
	}
	wg.Wait()
}

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
