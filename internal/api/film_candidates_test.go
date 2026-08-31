package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// filmEntityServer wires a real repo + Handlers with films_enabled on, exposing
// the raw *sql.DB so tests can seed video_studios/video_tags/video_people
// directly -- those junction tables are DERIVED (RelinkVideoEntity, ADR-053/072)
// so seeding them straight avoids standing up the full mapping/enrichment
// machinery just to test the films-by-entity filter and video-candidates picker.
func filmEntityServer(t *testing.T) (*httptest.Server, *repo.Repo, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth("tok"), false)
	h.SetFilmsEnabled(true)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, database
}

func seedPlainVideo(t *testing.T, r *repo.Repo, title string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/" + title + ".mkv", FileSize: 1, Title: title,
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video %s: %v", title, err)
	}
	return id
}

func seedStudio(t *testing.T, sqlDB *sql.DB, videoID int64, name string) int64 {
	t.Helper()
	res, err := sqlDB.Exec(`INSERT INTO studios (name) VALUES (?)`, name)
	if err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	sid, _ := res.LastInsertId()
	if _, err := sqlDB.Exec(`INSERT INTO video_studios (video_id, studio_id) VALUES (?, ?)`, videoID, sid); err != nil {
		t.Fatalf("link video studio: %v", err)
	}
	return sid
}

func seedTag(t *testing.T, sqlDB *sql.DB, videoID int64, name string) int64 {
	t.Helper()
	res, err := sqlDB.Exec(`INSERT INTO tags (name) VALUES (?)`, name)
	if err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	tid, _ := res.LastInsertId()
	if _, err := sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, videoID, tid); err != nil {
		t.Fatalf("link video tag: %v", err)
	}
	return tid
}

func decodeFilmItems(t *testing.T, resp *http.Response) []model.Film {
	t.Helper()
	defer resp.Body.Close()
	var body struct {
		Items []model.Film `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode films list: %v", err)
	}
	return body.Items
}

// TestListFilmsForEntity covers GET /films?studio_id= (F56): a film matches when
// ANY of its attached videos carries the studio, and is absent from an unrelated
// studio's filter.
func TestListFilmsForEntity(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	ctx := context.Background()

	v1 := seedPlainVideo(t, r, "scene-with-studio")
	studioID := seedStudio(t, sqlDB, v1, "Acme Films")

	filmID, err := r.CreateFilm(ctx, "Studio Filter Test", 2023)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, v1, nil, false); err != nil {
		t.Fatalf("attach video: %v", err)
	}

	matched, err := http.Get(srv.URL + "/api/v1/films?studio_id=" + itoa(studioID))
	if err != nil {
		t.Fatalf("get films by studio: %v", err)
	}
	items := decodeFilmItems(t, matched)
	if len(items) != 1 || items[0].ID != filmID {
		t.Fatalf("films for matching studio: got %+v, want [film %d]", items, filmID)
	}

	missed, err := http.Get(srv.URL + "/api/v1/films?studio_id=999999")
	if err != nil {
		t.Fatalf("get films by unrelated studio: %v", err)
	}
	if items := decodeFilmItems(t, missed); len(items) != 0 {
		t.Fatalf("films for unrelated studio: got %+v, want none", items)
	}
}

// TestFilmVideoCandidates covers GET /films/{id}/video-candidates (F56, design
// handoff §4): default scope excludes videos already attached to ANY film;
// ?unattached=false includes them and flags already_attached; a video already
// attached to the film being edited is excluded in both scopes. elsewhere is
// attached as a full-film video specifically to prove the RD6/HOLODEX-282
// hiding filter (films_enabled=true, set by filmEntityServer) does not leak
// into this owner picker -- it must still surface an is_full_film video so a
// conflicting attachment stays visible/resolvable, unlike the public list
// surfaces (browse/search/RelatedShelf/EntityVideos) that hide it.
func TestFilmVideoCandidates(t *testing.T) {
	srv, r, _ := filmEntityServer(t)
	ctx := context.Background()

	unattached := seedPlainVideo(t, r, "never-attached")
	elsewhere := seedPlainVideo(t, r, "attached-elsewhere")
	ownScene := seedPlainVideo(t, r, "already-in-this-film")

	filmID, err := r.CreateFilm(ctx, "Candidates Test", 2024)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	otherFilmID, err := r.CreateFilm(ctx, "Other Film", 2024)
	if err != nil {
		t.Fatalf("create other film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, otherFilmID, elsewhere, nil, true); err != nil {
		t.Fatalf("attach elsewhere: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, ownScene, nil, false); err != nil {
		t.Fatalf("attach own scene: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/films/"+itoa(filmID)+"/video-candidates", nil)
	req.Header.Set(api.AdminTokenHeader, "tok")
	defaultResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get video candidates: %v", err)
	}
	defer defaultResp.Body.Close()
	defaultBytes, err := io.ReadAll(defaultResp.Body)
	if err != nil {
		t.Fatalf("read default candidates: %v", err)
	}
	// The frontend picker calls .filter() on every row's already_attached
	// unconditionally (FilmBulkAttachDialog.svelte) -- a bare `null` (Go's zero
	// value for a missing map entry) throws there and silently blanks the whole
	// candidate list, so the wire contract must always be `[]`, never `null`.
	if bytes.Contains(defaultBytes, []byte(`"already_attached":null`)) {
		t.Fatalf("already_attached must serialize as [] not null: %s", defaultBytes)
	}
	var defaultBody struct {
		Items []struct {
			Video           model.Video           `json:"video"`
			AlreadyAttached []repo.FilmAttachment `json:"already_attached"`
		} `json:"items"`
	}
	if err := json.Unmarshal(defaultBytes, &defaultBody); err != nil {
		t.Fatalf("decode default candidates: %v", err)
	}
	if len(defaultBody.Items) != 1 || defaultBody.Items[0].Video.ID != unattached {
		t.Fatalf("default-scope candidates: got %+v, want only video %d", defaultBody.Items, unattached)
	}

	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/films/"+itoa(filmID)+"/video-candidates?unattached=false", nil)
	req2.Header.Set(api.AdminTokenHeader, "tok")
	allResp, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("get all candidates: %v", err)
	}
	defer allResp.Body.Close()
	var allBody struct {
		Items []struct {
			Video           model.Video           `json:"video"`
			AlreadyAttached []repo.FilmAttachment `json:"already_attached"`
		} `json:"items"`
	}
	if err := json.NewDecoder(allResp.Body).Decode(&allBody); err != nil {
		t.Fatalf("decode all candidates: %v", err)
	}
	// ownScene must still be excluded (it's already attached to *this* film).
	byID := map[int64][]repo.FilmAttachment{}
	for _, it := range allBody.Items {
		byID[it.Video.ID] = it.AlreadyAttached
	}
	if _, present := byID[ownScene]; present {
		t.Fatalf("video already attached to this film must be excluded: got items %+v", allBody.Items)
	}
	if len(allBody.Items) != 2 {
		t.Fatalf("unattached=false candidates: got %d items, want 2 (unattached + elsewhere)", len(allBody.Items))
	}
	attached, ok := byID[elsewhere]
	if !ok || len(attached) != 1 || attached[0].FilmID != otherFilmID {
		t.Fatalf("already_attached for elsewhere video: got %+v", attached)
	}
}

// TestFullFilmVideoHiddenFromListSurfaces covers RD6/HOLODEX-282 end-to-end
// through the HTTP layer (films_enabled=true, set by filmEntityServer): a
// video attached as is_full_film must be excluded from browse (GET /media),
// global search (GET /search), the "More with ..." shelves (GET /media/{id}
// /related), and the person/tag/studio detail video lists, while the video's
// own detail page (GET /media/{id}) stays reachable.
func TestFullFilmVideoHiddenFromListSurfaces(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	ctx := context.Background()

	full := seedPlainVideo(t, r, "FullFilmVideo")
	scene := seedPlainVideo(t, r, "SceneVideo")

	filmID, err := r.CreateFilm(ctx, "Hidden Surfaces Test", 2025)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, full, nil, true); err != nil {
		t.Fatalf("attach full-film video: %v", err)
	}
	one := int64(1)
	if _, err := r.AttachFilmVideo(ctx, filmID, scene, &one, false); err != nil {
		t.Fatalf("attach scene video: %v", err)
	}

	// Both videos share a tag, a studio, and a person -- so full-film would
	// otherwise appear in scene's tag shelf and in every entity's video list.
	tagID := seedTag(t, sqlDB, full, "shared-tag")
	if _, err := sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, scene, tagID); err != nil {
		t.Fatalf("tag scene video: %v", err)
	}
	studioID := seedStudio(t, sqlDB, full, "Shared Studio")
	if _, err := sqlDB.Exec(`INSERT INTO video_studios (video_id, studio_id) VALUES (?, ?)`, scene, studioID); err != nil {
		t.Fatalf("studio-link scene video: %v", err)
	}
	linkPeople(t, r, full, "Shared Person")
	linkPeople(t, r, scene, "Shared Person")
	peopleByVideo, err := r.PeopleForVideos(ctx, []int64{full})
	if err != nil || len(peopleByVideo[full]) == 0 {
		t.Fatalf("lookup shared person: %v", err)
	}
	personID := peopleByVideo[full][0].ID

	decodeVideoIDs := func(t *testing.T, resp *http.Response) []int64 {
		t.Helper()
		defer resp.Body.Close()
		var body struct {
			Items []model.Video `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode items: %v", err)
		}
		ids := make([]int64, len(body.Items))
		for i, v := range body.Items {
			ids[i] = v.ID
		}
		return ids
	}
	hasID := func(ids []int64, want int64) bool {
		for _, id := range ids {
			if id == want {
				return true
			}
		}
		return false
	}

	// Browse.
	mediaResp, err := http.Get(srv.URL + "/api/v1/media")
	if err != nil {
		t.Fatalf("get media: %v", err)
	}
	ids := decodeVideoIDs(t, mediaResp)
	if hasID(ids, full) {
		t.Errorf("GET /media: full-film video present, want hidden: %v", ids)
	}
	if !hasID(ids, scene) {
		t.Errorf("GET /media: scene video missing, want present: %v", ids)
	}

	// Global search: title-prefix match on the full-film video's own title.
	searchResp, err := http.Get(srv.URL + "/api/v1/search?q=FullFilmVideo")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	defer searchResp.Body.Close()
	var searchBody struct {
		Videos []model.Video `json:"videos"`
	}
	if err := json.NewDecoder(searchResp.Body).Decode(&searchBody); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	for _, v := range searchBody.Videos {
		if v.ID == full {
			t.Errorf("GET /search: full-film video present, want hidden: %+v", searchBody.Videos)
		}
	}

	// Related shelf: scene's tag shelf must not surface the full-film sibling.
	relatedResp, err := http.Get(srv.URL + "/api/v1/media/" + itoa(scene) + "/related")
	if err != nil {
		t.Fatalf("get related: %v", err)
	}
	defer relatedResp.Body.Close()
	var relatedBody struct {
		Tag *struct {
			Items []model.Video `json:"items"`
		} `json:"tag"`
	}
	if err := json.NewDecoder(relatedResp.Body).Decode(&relatedBody); err != nil {
		t.Fatalf("decode related: %v", err)
	}
	if relatedBody.Tag != nil {
		for _, v := range relatedBody.Tag.Items {
			if v.ID == full {
				t.Errorf("GET /media/%d/related: full-film video present in tag shelf, want hidden: %+v", scene, relatedBody.Tag.Items)
			}
		}
	}

	// Entity detail pages: person, tag, studio.
	personResp, err := http.Get(srv.URL + "/api/v1/people/" + itoa(personID))
	if err != nil {
		t.Fatalf("get person: %v", err)
	}
	if ids := decodeVideoIDs(t, personResp); hasID(ids, full) {
		t.Errorf("GET /people/%d: full-film video present, want hidden: %v", personID, ids)
	}
	tagResp, err := http.Get(srv.URL + "/api/v1/tags/" + itoa(tagID))
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if ids := decodeVideoIDs(t, tagResp); hasID(ids, full) {
		t.Errorf("GET /tags/%d: full-film video present, want hidden: %v", tagID, ids)
	}
	studioResp, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(studioID))
	if err != nil {
		t.Fatalf("get studio: %v", err)
	}
	if ids := decodeVideoIDs(t, studioResp); hasID(ids, full) {
		t.Errorf("GET /studios/%d: full-film video present, want hidden: %v", studioID, ids)
	}

	// The full-film video's own detail page always stays reachable.
	detailResp, err := http.Get(srv.URL + "/api/v1/media/" + itoa(full))
	if err != nil {
		t.Fatalf("get media detail: %v", err)
	}
	defer detailResp.Body.Close()
	if detailResp.StatusCode != http.StatusOK {
		t.Errorf("GET /media/%d: got %d, want 200 (detail page always reachable)", full, detailResp.StatusCode)
	}
}

// TestSearchIncludesFilms covers HOLODEX-283: GET /search returns a films group
// natively (FTS over films_fts), gated on films_enabled the same as every other
// films surface, replacing the old frontend-only merge.
func TestSearchIncludesFilms(t *testing.T) {
	srv, r, _ := filmEntityServer(t)
	ctx := context.Background()

	if _, err := r.CreateFilm(ctx, "Neon Skyline", 2019); err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.CreateFilm(ctx, "Unrelated Title", 2020); err != nil {
		t.Fatalf("create other film: %v", err)
	}

	code, body := getJSON(t, srv.URL+"/api/v1/search?q=Neon")
	if code != http.StatusOK {
		t.Fatalf("search code=%d", code)
	}
	films, _ := body["films"].([]any)
	if len(films) != 1 {
		t.Fatalf("search films = %v, want 1 match", body["films"])
	}
	if name, _ := films[0].(map[string]any)["name"].(string); name != "Neon Skyline" {
		t.Errorf("search films[0].name = %q, want %q", name, "Neon Skyline")
	}
}

// TestSearchFilmsDisabled covers HOLODEX-283's films_enabled gate: with the flag
// off, GET /search never surfaces films (route itself is not films-gated, so it
// must return an empty films group rather than 404 or omitting the key).
func TestSearchFilmsDisabled(t *testing.T) {
	srv, r, _ := newServer(t)
	ctx := context.Background()

	if _, err := r.CreateFilm(ctx, "Neon Skyline", 2019); err != nil {
		t.Fatalf("create film: %v", err)
	}

	code, body := getJSON(t, srv.URL+"/api/v1/search?q=Neon")
	if code != http.StatusOK {
		t.Fatalf("search code=%d", code)
	}
	films, ok := body["films"].([]any)
	if !ok {
		t.Fatalf("search films field missing or wrong type: %v", body["films"])
	}
	if len(films) != 0 {
		t.Errorf("search films = %v, want empty (films_enabled=false)", body["films"])
	}
}
