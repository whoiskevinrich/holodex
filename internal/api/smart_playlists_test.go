package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// smartPlaylist creates a smart playlist from a browse query as the owner and returns
// its URL and the stored (canonical) query.
func smartPlaylist(t *testing.T, base, query string, extra map[string]any) (string, string) {
	t.Helper()
	body := map[string]any{"name": "smart", "query": query}
	for k, v := range extra {
		body[k] = v
	}
	code, resp := postTok(t, base+"/playlists", "s3cret", body)
	if code != http.StatusCreated {
		t.Fatalf("create smart %q: %d %v", query, code, resp)
	}
	p := resp["playlist"].(map[string]any)
	return base + "/playlists/" + itoa(int64(p["id"].(float64))), p["query"].(string)
}

func numIDs(v any) []int64 {
	var out []int64
	for _, x := range v.([]any) {
		out = append(out, int64(x.(float64)))
	}
	return out
}

func staleKinds(body map[string]any) []string {
	var out []string
	for _, s := range body["stale_refs"].([]any) {
		out = append(out, s.(map[string]any)["kind"].(string))
	}
	return out
}

// TestSmartPlaylistCanonicalQuery covers ADR-121 D2: sort and fetch keys are stripped
// (sort moves to playlists.sort), unknown keys and malformed values are refused, and
// two spellings of the same set store the same string.
func TestSmartPlaylistCanonicalQuery(t *testing.T) {
	srv, _ := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"

	_, a := smartPlaylist(t, base, "?tag=7&tag=3&tag=7&q=noir&limit=20&offset=40&seed=9&sort=title_asc&year_min=", nil)
	_, b := smartPlaylist(t, base, "q=noir&tag=3,7", nil)
	if a != b || a != "q=noir&tag=3&tag=7" {
		t.Errorf("canonical queries = %q / %q, want both %q", a, b, "q=noir&tag=3&tag=7")
	}
	plURL, _ := smartPlaylist(t, base, "tag=3&sort=title_asc", nil)
	if _, body := getJSONTok(t, plURL, "s3cret"); body["playlist"].(map[string]any)["sort"] != "title_asc" {
		t.Errorf("sort = %v, want the query's title_asc", body["playlist"].(map[string]any)["sort"])
	}

	for _, bad := range []string{"tagz=3", "tag=abc", "person=0", "year_min=nope", "resolution=8K"} {
		if code, _ := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "x", "query": bad}); code != http.StatusBadRequest {
			t.Errorf("query %q: code %d, want 400", bad, code)
		}
	}
	if code, _ := postTok(t, base+"/playlists", "s3cret",
		map[string]any{"name": "x", "query": "tag=3", "from_query": "tag=3"}); code != http.StatusBadRequest {
		t.Errorf("query + from_query: code %d, want 400", code)
	}
	if code, _ := postTok(t, base+"/playlists", "s3cret",
		map[string]any{"name": "x", "query": "tag=3", "sort": "manual"}); code != http.StatusBadRequest {
		t.Errorf("smart + manual: code %d, want 400", code)
	}
}

// TestSmartPlaylistLiveRead covers spec P0-5/P0-6 and ADR-121 D3: membership equals
// /media for the same query and order, a newly matching video appears with no refresh,
// trash is inherited, tiles page while ids don't, and membership writes are refused.
func TestSmartPlaylistLiveRead(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	vids := seedPlaylistVideos(t, r, 4)
	even := tagID(t, r, "even")

	plURL, _ := smartPlaylist(t, base, "tag="+itoa(even)+"&sort=title_asc", nil)
	_, media := getJSONTok(t, base+"/media/ids?tag="+itoa(even)+"&sort=title_asc", "s3cret")
	_, body := getJSONTok(t, plURL, "s3cret")
	if got := numIDs(body["ids"]); !sameOrder(got, numIDs(media["ids"])) || len(got) != 2 {
		t.Fatalf("smart ids %v, /media ids %v", got, media["ids"])
	}
	if body["playlist"].(map[string]any)["item_count"].(float64) != 2 || len(staleKinds(body)) != 0 {
		t.Errorf("item_count %v stale %v", body["playlist"].(map[string]any)["item_count"], body["stale_refs"])
	}

	// A video tagged after saving appears on the next read.
	id, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/late.mkv", FileSize: 1, Title: "Aardvark", Duration: 10,
		FileMtime: time.Now().UTC().Truncate(time.Second), Tags: []model.Tag{{Name: "even"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, body = getJSONTok(t, plURL, "s3cret")
	if got := numIDs(body["ids"]); len(got) != 3 || got[0] != id {
		t.Errorf("after tagging: ids %v, want the new video first of 3", got)
	}
	// Tiles page; ids stay whole.
	_, body = getJSONTok(t, plURL+"?limit=1&offset=1", "s3cret")
	if items := itemIDs(body["items"]); len(items) != 1 || items[0] != numIDs(body["ids"])[1] || body["total"].(float64) != 3 {
		t.Errorf("paged: items %v ids %v total %v", items, body["ids"], body["total"])
	}
	// Trash is inherited.
	if err := r.SoftDelete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, body = getJSONTok(t, plURL, "s3cret"); len(numIDs(body["ids"])) != 2 {
		t.Errorf("after trash: ids %v", body["ids"])
	}

	// The list counts it live.
	_, list := getJSONTok(t, base+"/playlists", "s3cret")
	if c := list["items"].([]any)[0].(map[string]any)["item_count"].(float64); c != 2 {
		t.Errorf("list item_count = %v, want 2", c)
	}

	// No manual membership, no manual sort.
	if code := sendTok(t, http.MethodPut, plURL+"/videos/"+itoa(vids[1]), "s3cret"); code != http.StatusBadRequest {
		t.Errorf("add to smart: %d, want 400", code)
	}
	if code := sendTok(t, http.MethodDelete, plURL+"/videos/"+itoa(vids[0]), "s3cret"); code != http.StatusBadRequest {
		t.Errorf("remove from smart: %d, want 400", code)
	}
	if code, _ := patchTok(t, plURL, "s3cret", map[string]any{"sort": "manual"}); code != http.StatusBadRequest {
		t.Errorf("smart → manual: %d, want 400", code)
	}

	// Edit filter → Update replaces the query, re-canonicalised.
	code, resp := patchTok(t, plURL, "s3cret", map[string]any{"query": "?tag=" + itoa(tagID(t, r, "amv")) + "&limit=5"})
	if code != http.StatusOK || resp["playlist"].(map[string]any)["query"] != "tag="+itoa(tagID(t, r, "amv")) {
		t.Fatalf("update query: %d %v", code, resp)
	}
	if _, body = getJSONTok(t, plURL, "s3cret"); len(numIDs(body["ids"])) != 4 {
		t.Errorf("after update: ids %v, want all 4", body["ids"])
	}
}

// TestSmartPlaylistVisibility covers ADR-121 D3/D4 and spec P0-10: visibility is checked
// before anything else, owner-only inputs can't be made public, and a public playlist
// that somehow holds one reports it to a visitor instead of evaluating.
func TestSmartPlaylistVisibility(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 2)

	// A private smart playlist with a stale ref answers a visitor exactly like an unknown id.
	plURL, _ := smartPlaylist(t, base, "tag=999999", nil)
	codeP, bodyP := getJSONTok(t, plURL, "")
	codeU, bodyU := getJSONTok(t, base+"/playlists/987654", "")
	if codeP != http.StatusNotFound || codeP != codeU || bodyP["error"] != bodyU["error"] {
		t.Errorf("visitor private: %d %v vs unknown %d %v", codeP, bodyP, codeU, bodyU)
	}

	// D4 refusals, whichever arrives second.
	if code, _ := postTok(t, base+"/playlists", "s3cret",
		map[string]any{"name": "x", "query": "missing_facet=poster", "visibility": "public"}); code != http.StatusBadRequest {
		t.Errorf("public + missing_facet: %d, want 400", code)
	}
	mf, _ := smartPlaylist(t, base, "missing_facet=poster", nil)
	if code, _ := patchTok(t, mf, "s3cret", map[string]any{"visibility": "public"}); code != http.StatusBadRequest {
		t.Errorf("visibility after missing_facet: %d, want 400", code)
	}
	pub, _ := smartPlaylist(t, base, "q=a", map[string]any{"visibility": "public"})
	if code, _ := patchTok(t, pub, "s3cret", map[string]any{"query": "missing_facet=poster"}); code != http.StatusBadRequest {
		t.Errorf("missing_facet after public: %d, want 400", code)
	}
	if code, _ := patchTok(t, pub, "s3cret", map[string]any{"sort": repo.SortCompletenessAsc}); code != http.StatusBadRequest {
		t.Errorf("completeness sort on public: %d, want 400", code)
	}

	// A playlist that predates the rule (public, completeness sort) can still be renamed
	// or set to always shuffle: neither changes the state D4 judges.
	public := model.PlaylistPublic
	completeness := repo.SortCompletenessAsc
	_, legacy := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "legacy", "visibility": "public"})
	legacyID := int64(legacy["playlist"].(map[string]any)["id"].(float64))
	if err := r.UpdatePlaylist(context.Background(), legacyID, repo.PlaylistPatch{Sort: &completeness}); err != nil {
		t.Fatal(err)
	}
	if code, _ := patchTok(t, base+"/playlists/"+itoa(legacyID), "s3cret",
		map[string]any{"name": "renamed", "play_shuffled": true}); code != http.StatusOK {
		t.Errorf("rename legacy public completeness playlist: %d, want 200", code)
	}

	// Read-time re-check: force the forbidden state past the API, then read as a visitor.
	id := mustPlaylistID(t, mf)
	if err := r.UpdatePlaylist(context.Background(), id, repo.PlaylistPatch{Visibility: &public}); err != nil {
		t.Fatal(err)
	}
	code, body := getJSONTok(t, mf, "")
	if kinds := staleKinds(body); code != http.StatusOK || len(kinds) != 1 || kinds[0] != "owner_only" || len(body["items"].([]any)) != 0 {
		t.Errorf("visitor on owner-only public: %d stale %v items %v", code, body["stale_refs"], body["items"])
	}

	// A public smart playlist gives a visitor exactly what /media gives them.
	_, body = getJSONTok(t, pub, "")
	_, media := getJSONTok(t, base+"/media/ids?q=a", "")
	if !sameOrder(numIDs(body["ids"]), numIDs(media["ids"])) {
		t.Errorf("visitor public smart ids %v, /media %v", body["ids"], media["ids"])
	}
}

func mustPlaylistID(t *testing.T, plURL string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(path.Base(plURL), 10, 64)
	if err != nil {
		t.Fatalf("playlist id from %q: %v", plURL, err)
	}
	return id
}

// TestSmartPlaylistMergeAndStale covers ADR-121 D5: a merge rewrites the stored query
// to the survivor; a deleted entity or a vanished mapped key is reported, nothing is
// evaluated, and nothing is rewritten.
func TestSmartPlaylistMergeAndStale(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	ctx := context.Background()
	seedPlaylistVideos(t, r, 4)
	amv, even := tagID(t, r, "amv"), tagID(t, r, "even")

	// Tag merge: even → amv.
	plURL, _ := smartPlaylist(t, base, "tag="+itoa(even), nil)
	if _, err := r.MergeEntitiesWithAffectedVideos(ctx, model.EntityTag, amv, even); err != nil {
		t.Fatal(err)
	}
	_, body := getJSONTok(t, plURL, "s3cret")
	if q := body["playlist"].(map[string]any)["query"]; q != "tag="+itoa(amv) || len(staleKinds(body)) != 0 {
		t.Errorf("after tag merge: query %v stale %v", q, body["stale_refs"])
	}
	_, media := getJSONTok(t, base+"/media/ids?tag="+itoa(amv), "s3cret")
	if !sameOrder(numIDs(body["ids"]), numIDs(media["ids"])) {
		t.Errorf("after merge: ids %v, /media %v", body["ids"], media["ids"])
	}

	// Person merge: Bea → Ann.
	for i, name := range []string{"Ann", "Bea"} {
		vid, err := r.UpsertVideo(ctx, &model.Video{
			FilePath: "/m/p" + itoa(int64(i)) + ".mkv", FileSize: 1, Title: name,
			FileMtime: time.Now().UTC().Truncate(time.Second), People: []model.Person{{Name: name}},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		linkPeople(t, r, vid, name)
	}
	ann, _, _ := r.PersonIDByName(ctx, "Ann")
	bea, _, _ := r.PersonIDByName(ctx, "Bea")
	personURL, _ := smartPlaylist(t, base, "person="+itoa(bea)+"&person="+itoa(ann), nil)
	if _, err := r.MergePersonsWithAffectedVideos(ctx, ann, bea); err != nil {
		t.Fatal(err)
	}
	if _, body = getJSONTok(t, personURL, "s3cret"); body["playlist"].(map[string]any)["query"] != "person="+itoa(ann) {
		t.Errorf("after person merge: query %v, want person=%d once", body["playlist"].(map[string]any)["query"], ann)
	}

	// A deleted category is flagged, not dropped.
	cat, err := r.CreateCategory(ctx, "genres")
	if err != nil {
		t.Fatal(err)
	}
	catURL, stored := smartPlaylist(t, base, "category_id="+itoa(cat.ID), nil)
	if err := r.DeleteCategory(ctx, cat.ID); err != nil {
		t.Fatal(err)
	}
	_, body = getJSONTok(t, catURL, "s3cret")
	refs := body["stale_refs"].([]any)
	if len(refs) != 1 || refs[0].(map[string]any)["key"] != "category_id" || len(body["ids"].([]any)) != 0 ||
		body["playlist"].(map[string]any)["query"] != stored {
		t.Errorf("deleted category: stale %v ids %v query %v", refs, body["ids"], body["playlist"].(map[string]any)["query"])
	}
	if code, resp := postTok(t, catURL+"/freeze", "s3cret", nil); code != http.StatusConflict {
		t.Errorf("freeze stale: %d %v, want 409", code, resp)
	}

	// A mapped key that is no longer filterable is flagged.
	id := mustPlaylistID(t, plURL)
	gone := "studio=Acme"
	if err := r.UpdatePlaylist(ctx, id, repo.PlaylistPatch{Query: &gone}); err != nil {
		t.Fatal(err)
	}
	if _, body = getJSONTok(t, plURL, "s3cret"); len(staleKinds(body)) != 1 || staleKinds(body)[0] != "unknown_key" {
		t.Errorf("vanished key: stale %v", body["stale_refs"])
	}

	// An unparsable stored string is flagged too, and doesn't take the list down.
	broken := "%zz"
	if err := r.UpdatePlaylist(ctx, id, repo.PlaylistPatch{Query: &broken}); err != nil {
		t.Fatal(err)
	}
	if _, body = getJSONTok(t, plURL, "s3cret"); len(staleKinds(body)) != 1 || staleKinds(body)[0] != "invalid" {
		t.Errorf("unparsable query: stale %v", body["stale_refs"])
	}
	if code, _ := getJSONTok(t, base+"/playlists", "s3cret"); code != http.StatusOK {
		t.Errorf("list with an unparsable smart playlist: %d, want 200", code)
	}
}

// TestSmartPlaylistFreezeAndShuffle covers spec P0-7 and P0-12: Freeze fixes the set
// and frees the manual sort; play_shuffled is a setting on any playlist.
func TestSmartPlaylistFreezeAndShuffle(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 4)
	even := tagID(t, r, "even")

	plURL, _ := smartPlaylist(t, base, "tag="+itoa(even)+"&sort=title_asc", nil)
	_, before := getJSONTok(t, plURL, "s3cret")
	code, resp := postTok(t, plURL+"/freeze", "s3cret", nil)
	if code != http.StatusOK || resp["playlist"].(map[string]any)["query"] != nil {
		t.Fatalf("freeze: %d %v", code, resp)
	}
	if _, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/late.mkv", FileSize: 1, Title: "Aardvark",
		FileMtime: time.Now().UTC().Truncate(time.Second), Tags: []model.Tag{{Name: "even"}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if code, _ := patchTok(t, plURL, "s3cret", map[string]any{"sort": "manual"}); code != http.StatusOK {
		t.Errorf("manual after freeze: %d", code)
	}
	_, after := getJSONTok(t, plURL, "s3cret")
	if !sameOrder(numIDs(after["ids"]), numIDs(before["ids"])) {
		t.Errorf("frozen ids %v, want the pre-freeze order %v", after["ids"], before["ids"])
	}
	if code, _ := postTok(t, plURL+"/freeze", "s3cret", nil); code != http.StatusBadRequest {
		t.Errorf("second freeze: %d, want 400", code)
	}
	if code, _ := patchTok(t, plURL, "s3cret", map[string]any{"query": "tag=1"}); code != http.StatusBadRequest {
		t.Errorf("query on a snapshot: %d, want 400", code)
	}

	code, resp = patchTok(t, plURL, "s3cret", map[string]any{"play_shuffled": true})
	if code != http.StatusOK || resp["playlist"].(map[string]any)["play_shuffled"] != true {
		t.Errorf("play_shuffled: %d %v", code, resp)
	}
	if code := sendTok(t, http.MethodPost, plURL+"/freeze", ""); code != http.StatusUnauthorized {
		t.Errorf("visitor freeze: %d, want 401", code)
	}
}

// --- testing-strategy §23.3, filled in -------------------------------------------------

// filmTok is filmEntityServer's owner token.
const filmTok = "tok"

// smartAs creates a smart playlist with the given token and returns its URL and id.
func smartAs(t *testing.T, base, token, query string, extra map[string]any) (string, int64) {
	t.Helper()
	body := map[string]any{"name": "smart", "query": query}
	for k, v := range extra {
		body[k] = v
	}
	code, resp := postTok(t, base+"/playlists", token, body)
	if code != http.StatusCreated {
		t.Fatalf("create smart %q: %d %v", query, code, resp)
	}
	id := int64(resp["playlist"].(map[string]any)["id"].(float64))
	return base + "/playlists/" + itoa(id), id
}

// rawGet returns the status and the exact response body bytes.
func rawGet(t *testing.T, url, token string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return resp.StatusCode, b
}

// rawItems returns each element of a response's `items` array as its exact JSON bytes.
func rawItems(t *testing.T, body []byte) []json.RawMessage {
	t.Helper()
	var out struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode items: %v\n%s", err, body)
	}
	return out.Items
}

func storedQuery(t *testing.T, sqlDB *sql.DB, id int64) sql.NullString {
	t.Helper()
	var q sql.NullString
	if err := sqlDB.QueryRow(`SELECT query FROM playlists WHERE id = ?`, id).Scan(&q); err != nil {
		t.Fatalf("stored query %d: %v", id, err)
	}
	return q
}

// attachFullFilm makes vid the full-film video of a new film, which /media hides (RD6).
func attachFullFilm(t *testing.T, r *repo.Repo, vid int64, name string) {
	t.Helper()
	ctx := context.Background()
	filmID, err := r.CreateFilm(ctx, name, 2025)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, vid, nil, true); err != nil {
		t.Fatalf("attach full film: %v", err)
	}
}

// TestSmartPlaylistCreateDefaultsAndList covers §23.3 Create and Lists: a smart
// playlist is private unless asked, a visitor can't create one, and GET /playlists
// marks smart rows with a non-null query and snapshot rows with null.
func TestSmartPlaylistCreateDefaultsAndList(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 2)

	code, resp := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "s", "query": "?tag=" + itoa(tagID(t, r, "amv")) + "&limit=9"})
	if code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, resp)
	}
	p := resp["playlist"].(map[string]any)
	if p["visibility"] != model.PlaylistPrivate || p["query"] != "tag="+itoa(tagID(t, r, "amv")) {
		t.Errorf("create defaults: visibility %v query %v", p["visibility"], p["query"])
	}
	if code, _ := postTok(t, base+"/playlists", "", map[string]any{"name": "v", "query": "q=a"}); code != http.StatusUnauthorized {
		t.Errorf("visitor create smart: %d, want 401", code)
	}

	if code, _ := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "snap", "from_query": "q=a"}); code != http.StatusCreated {
		t.Fatalf("create snapshot: %d", code)
	}
	_, list := getJSONTok(t, base+"/playlists", "s3cret")
	seen := map[string]bool{}
	for _, it := range list["items"].([]any) {
		row := it.(map[string]any)
		q, present := row["query"]
		if !present {
			t.Errorf("row %v: no query key (want null or a string)", row["name"])
		}
		switch row["name"] {
		case "s":
			if q != "tag="+itoa(tagID(t, r, "amv")) {
				t.Errorf("smart row query = %v", q)
			}
		case "snap":
			if q != nil {
				t.Errorf("snapshot row query = %v, want null", q)
			}
		}
		seen[row["name"].(string)] = true
	}
	if !seen["s"] || !seen["snap"] {
		t.Errorf("list rows %v, want both", seen)
	}
}

// TestSmartPlaylistRestoreAndFullFilm covers §23.3 Live reads: restore brings a trashed
// match back, and a full-film video is hidden on the smart path exactly as on /media.
func TestSmartPlaylistRestoreAndFullFilm(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	base := srv.URL + "/api/v1"
	ctx := context.Background()

	a := seedPlainVideo(t, r, "RestoreA")
	tag := seedTag(t, sqlDB, a, "restore-tag")
	b := seedPlainVideo(t, r, "RestoreB")
	full := seedPlainVideo(t, r, "RestoreFull")
	for _, id := range []int64{b, full} {
		if _, err := sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, id, tag); err != nil {
			t.Fatal(err)
		}
	}
	attachFullFilm(t, r, full, "Restore Film")

	plURL, _ := smartAs(t, base, filmTok, "tag="+itoa(tag)+"&sort=title_asc", nil)
	_, body := getJSONTok(t, plURL, filmTok)
	media := mediaIDs(t, base+"/media/ids?tag="+itoa(tag)+"&sort=title_asc")
	if got := numIDs(body["ids"]); !sameOrder(got, media.IDs) || !sameOrder(got, []int64{a, b}) {
		t.Fatalf("smart ids %v, /media/ids %v, want [%d %d] (full film %d hidden)", got, media.IDs, a, b, full)
	}
	for _, id := range itemIDs(body["items"]) {
		if id == full {
			t.Errorf("full-film video %d in smart items", id)
		}
	}

	if err := r.SoftDelete(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, body = getJSONTok(t, plURL, filmTok); !sameOrder(numIDs(body["ids"]), []int64{b}) {
		t.Errorf("after trash: ids %v, want [%d]", body["ids"], b)
	}
	if err := r.Restore(ctx, a); err != nil {
		t.Fatal(err)
	}
	_, body = getJSONTok(t, plURL, filmTok)
	if !sameOrder(numIDs(body["ids"]), []int64{a, b}) || body["playlist"].(map[string]any)["item_count"].(float64) != 2 {
		t.Errorf("after restore: ids %v item_count %v, want [%d %d] / 2", body["ids"], body["playlist"].(map[string]any)["item_count"], a, b)
	}
}

// TestSmartPlaylistRefusedPatchKeepsQuery covers §23.3 Edit filter: a refused PATCH
// leaves the stored query byte-identical, even when the refused body also carries a
// valid new query.
func TestSmartPlaylistRefusedPatchKeepsQuery(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	base := srv.URL + "/api/v1"
	v := seedPlainVideo(t, r, "Refused")
	tag := seedTag(t, sqlDB, v, "refused-tag")

	_, privID := smartAs(t, base, filmTok, "tag="+itoa(tag)+"&missing_facet=poster_url", nil)
	pubURL, pubID := smartAs(t, base, filmTok, "tag="+itoa(tag), map[string]any{"visibility": "public"})
	privURL := base + "/playlists/" + itoa(privID)

	cases := []struct {
		name string
		url  string
		id   int64
		body map[string]any
	}{
		{"manual sort", pubURL, pubID, map[string]any{"sort": "manual"}},
		{"valid query + manual sort", pubURL, pubID, map[string]any{"query": "q=other", "sort": "manual"}},
		{"unknown key", pubURL, pubID, map[string]any{"query": "tagz=1"}},
		{"missing_facet on public", pubURL, pubID, map[string]any{"query": "missing_facet=poster_url"}},
		{"public + missing_facet", privURL, privID, map[string]any{"visibility": "public"}},
		{"public + missing_facet in one body", privURL, privID, map[string]any{"visibility": "public", "query": "q=x&missing_facet=studio"}},
	}
	for _, c := range cases {
		before := storedQuery(t, sqlDB, c.id)
		if code, resp := patchTok(t, c.url, filmTok, c.body); code != http.StatusBadRequest {
			t.Errorf("%s: %d %v, want 400", c.name, code, resp)
		}
		if after := storedQuery(t, sqlDB, c.id); after != before {
			t.Errorf("%s: stored query %q → %q, want unchanged", c.name, before.String, after.String)
		}
	}
}

// TestSmartPlaylistAlwaysShuffle covers §23.3 Always shuffle: owner-only to set, carried
// to a visitor on a public playlist, and the display order ignores it.
func TestSmartPlaylistAlwaysShuffle(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 4)

	plURL, _ := smartPlaylist(t, base, "tag="+itoa(tagID(t, r, "amv"))+"&sort=title_asc", map[string]any{"visibility": "public"})
	_, before := getJSONTok(t, plURL, "")
	if before["playlist"].(map[string]any)["play_shuffled"] != false {
		t.Fatalf("play_shuffled default = %v, want false", before["playlist"].(map[string]any)["play_shuffled"])
	}
	if code, _ := patchTok(t, plURL, "", map[string]any{"play_shuffled": true}); code != http.StatusUnauthorized {
		t.Errorf("visitor PATCH play_shuffled: %d, want 401", code)
	}
	if code, _ := patchTok(t, plURL, "s3cret", map[string]any{"play_shuffled": true}); code != http.StatusOK {
		t.Fatalf("owner PATCH play_shuffled: %d", code)
	}
	_, after := getJSONTok(t, plURL, "")
	if after["playlist"].(map[string]any)["play_shuffled"] != true {
		t.Errorf("visitor GET play_shuffled = %v, want true", after["playlist"].(map[string]any)["play_shuffled"])
	}
	if !sameOrder(itemIDs(after["items"]), itemIDs(before["items"])) || !sameOrder(numIDs(after["ids"]), numIDs(before["ids"])) {
		t.Errorf("display order changed by play_shuffled: %v → %v", itemIDs(before["items"]), itemIDs(after["items"]))
	}
}

// TestSmartPlaylistDeletedEntityStale covers §23.3 Merge and delete: a deleted tag,
// person or studio is reported under its own key, nothing is evaluated, and the stored
// query is left as it was.
func TestSmartPlaylistDeletedEntityStale(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	base := srv.URL + "/api/v1"
	ctx := context.Background()

	v := seedPlainVideo(t, r, "Stale")
	tag := seedTag(t, sqlDB, v, "stale-tag")
	studio := seedStudio(t, sqlDB, v, "Stale Studio")
	linkPeople(t, r, v, "Stale Person")
	person, _, err := r.PersonIDByName(ctx, "Stale Person")
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		key, table string
		id         int64
	}{
		{"tag", "tags", tag}, {"person", "people", person}, {"studio_id", "studios", studio},
	} {
		plURL, _ := smartAs(t, base, filmTok, c.key+"="+itoa(c.id), nil)
		_, body := getJSONTok(t, plURL, filmTok)
		if len(staleKinds(body)) != 0 || len(numIDs(body["ids"])) != 1 {
			t.Fatalf("%s before delete: stale %v ids %v", c.key, body["stale_refs"], body["ids"])
		}
		if _, err := sqlDB.Exec(`DELETE FROM `+c.table+` WHERE id = ?`, c.id); err != nil {
			t.Fatalf("delete %s: %v", c.table, err)
		}
		_, body = getJSONTok(t, plURL, filmTok)
		refs := body["stale_refs"].([]any)
		if len(refs) != 1 {
			t.Fatalf("%s: stale_refs %v, want one", c.key, refs)
		}
		ref := refs[0].(map[string]any)
		if ref["kind"] != "missing" || ref["key"] != c.key || ref["value"] != itoa(c.id) {
			t.Errorf("%s: stale ref %v, want {missing %s %d}", c.key, ref, c.key, c.id)
		}
		if len(body["items"].([]any)) != 0 || len(body["ids"].([]any)) != 0 {
			t.Errorf("%s: items %v ids %v, want none", c.key, body["items"], body["ids"])
		}
		if q := body["playlist"].(map[string]any)["query"]; q != c.key+"="+itoa(c.id) {
			t.Errorf("%s: query %v, want unchanged", c.key, q)
		}
	}
}

// TestSmartPlaylistPrivate404IsUnknown404 covers §23.3 Private (risk 1): a private smart
// playlist answers a visitor with the unknown-id 404 byte for byte, whatever its query
// holds, and the owner on a private playlist with owner-only inputs gets it evaluated.
func TestSmartPlaylistPrivate404IsUnknown404(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	base := srv.URL + "/api/v1"
	v := seedPlainVideo(t, r, "Private")
	tag := seedTag(t, sqlDB, v, "private-tag")

	codeU, bodyU := rawGet(t, base+"/playlists/987654", "")
	if codeU != http.StatusNotFound {
		t.Fatalf("unknown id: %d", codeU)
	}
	_, ownerOnlyID := smartAs(t, base, filmTok, "tag="+itoa(tag)+"&missing_facet=poster_url", nil)
	_, completenessID := smartAs(t, base, filmTok, "tag="+itoa(tag)+"&sort="+repo.SortCompletenessAsc, nil)
	_, staleID := smartAs(t, base, filmTok, "tag=999999", nil)
	_, plainID := smartAs(t, base, filmTok, "tag="+itoa(tag), nil)
	for name, id := range map[string]int64{"plain": plainID, "stale": staleID, "missing_facet": ownerOnlyID, "completeness sort": completenessID} {
		code, body := rawGet(t, base+"/playlists/"+itoa(id), "")
		if code != codeU || !bytes.Equal(body, bodyU) {
			t.Errorf("visitor on private %s: %d %q, want the unknown-id %d %q", name, code, body, codeU, bodyU)
		}
	}

	// The owner reads a private owner-only playlist normally (D4 only bars public ones).
	_, body := getJSONTok(t, base+"/playlists/"+itoa(ownerOnlyID), filmTok)
	_, media := getJSONTok(t, base+"/media/ids?tag="+itoa(tag)+"&missing_facet=poster_url", filmTok)
	if len(staleKinds(body)) != 0 || !sameOrder(numIDs(body["ids"]), numIDs(media["ids"])) {
		t.Errorf("owner on private missing_facet: stale %v ids %v, /media/ids %v", body["stale_refs"], body["ids"], media["ids"])
	}

	// §23.8 Q5: made public behind the API's back (D4 refuses it there), the playlist reports
	// owner_only to every reader, owner included, and evaluates nothing.
	pub := model.PlaylistPublic
	if err := r.UpdatePlaylist(context.Background(), ownerOnlyID, repo.PlaylistPatch{Visibility: &pub}); err != nil {
		t.Fatal(err)
	}
	for who, tok := range map[string]string{"owner": filmTok, "visitor": ""} {
		_, body := getJSONTok(t, base+"/playlists/"+itoa(ownerOnlyID), tok)
		if kinds := staleKinds(body); len(kinds) != 1 || kinds[0] != "owner_only" || len(numIDs(body["ids"])) != 0 {
			t.Errorf("%s on public missing_facet: stale %v ids %v, want [owner_only] and none", who, body["stale_refs"], body["ids"])
		}
	}
}

// TestSmartPlaylistVisitorTilesMatchMedia covers §23.3 Public parity (risk 1): a visitor's
// tiles from a public smart playlist are byte-for-byte a visitor's /media tiles for the
// same query, on fixtures that carry owner-only file metadata, so a smart path that
// skipped redactFileMetadataForVisitors would show up as a diff.
func TestSmartPlaylistVisitorTilesMatchMedia(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	base := srv.URL + "/api/v1"
	ctx := context.Background()

	var tag int64
	for i, title := range []string{"Tile Bravo", "Tile Alpha", "Tile Charlie"} {
		id, err := r.UpsertVideo(ctx, &model.Video{
			FilePath: "/secret/library/tile" + itoa(int64(i)) + ".mkv", FileSize: 1, Title: title,
			Duration: 60, Width: 1920, Height: 1080, Container: "Matroska", VideoCodec: "hevc",
			AudioCodec: "eac3", BitrateKbps: 5000, FileMtime: time.Now().UTC().Truncate(time.Second),
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			tag = seedTag(t, sqlDB, id, "tile-tag")
		} else if _, err := sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, id, tag); err != nil {
			t.Fatal(err)
		}
	}
	q := "tag=" + itoa(tag) + "&sort=title_asc"
	plURL, _ := smartAs(t, base, filmTok, q, map[string]any{"visibility": "public"})

	// The fixture really carries owner-only metadata: the owner sees it on both paths.
	if _, owner := rawGet(t, plURL, filmTok); !bytes.Contains(owner, []byte("hevc")) || !bytes.Contains(owner, []byte("/secret/library/")) {
		t.Fatalf("owner smart tiles carry no file metadata; the fixture proves nothing:\n%s", owner)
	}

	_, smartBody := rawGet(t, plURL, "")
	_, mediaBody := rawGet(t, base+"/media?"+q, "")
	smart, media := rawItems(t, smartBody), rawItems(t, mediaBody)
	if len(smart) != 3 || len(smart) != len(media) {
		t.Fatalf("visitor tiles: smart %d, /media %d, want 3 each", len(smart), len(media))
	}
	for i := range smart {
		if !bytes.Equal(smart[i], media[i]) {
			t.Errorf("tile %d differs\nsmart %s\nmedia %s", i, smart[i], media[i])
		}
	}
	if bytes.Contains(smartBody, []byte("hevc")) || bytes.Contains(smartBody, []byte("/secret/library/")) {
		t.Errorf("visitor smart response leaks file metadata:\n%s", smartBody)
	}
}

// TestSmartPlaylistOwnerOnlyAcceptedWhenPrivate covers §23.3's D4 counterpart: the four
// refused entry points are all accepted, as owner, on a private playlist.
func TestSmartPlaylistOwnerOnlyAcceptedWhenPrivate(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 2)

	// create private + missing_facet
	mf, _ := smartPlaylist(t, base, "missing_facet=poster_url", nil)
	// PATCH visibility (to private) on a missing_facet query
	if code, resp := patchTok(t, mf, "s3cret", map[string]any{"visibility": "private"}); code != http.StatusOK {
		t.Errorf("visibility private on missing_facet: %d %v", code, resp)
	}
	plain, _ := smartPlaylist(t, base, "q=a", nil)
	// PATCH query with missing_facet on a private playlist
	code, resp := patchTok(t, plain, "s3cret", map[string]any{"query": "q=a&missing_facet=studio"})
	if code != http.StatusOK || resp["playlist"].(map[string]any)["query"] != "missing_facet=studio&q=a" {
		t.Errorf("missing_facet query on private: %d %v", code, resp)
	}
	// PATCH a completeness sort on a private playlist
	for _, s := range []string{repo.SortCompletenessAsc, repo.SortCompletenessDesc} {
		if code, resp := patchTok(t, plain, "s3cret", map[string]any{"sort": s}); code != http.StatusOK {
			t.Errorf("sort %s on private: %d %v", s, code, resp)
		}
	}
	if _, body := getJSONTok(t, plain, "s3cret"); len(staleKinds(body)) != 0 {
		t.Errorf("owner read of private owner-only playlist: stale %v", body["stale_refs"])
	}
}

// TestSmartPlaylistNotInVideoDetail pins §23.8 Q1: a video's detail `playlists` lists
// the playlists it is a member of, never a smart playlist it merely matches.
func TestSmartPlaylistNotInVideoDetail(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	vids := seedPlaylistVideos(t, r, 2)

	smartURL, _ := smartPlaylist(t, base, "tag="+itoa(tagID(t, r, "amv")), map[string]any{"visibility": "public"})
	_, snap := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "snap", "visibility": "public"})
	snapID := int64(snap["playlist"].(map[string]any)["id"].(float64))
	if code := sendTok(t, http.MethodPut, base+"/playlists/"+itoa(snapID)+"/videos/"+itoa(vids[0]), "s3cret"); code != http.StatusOK {
		t.Fatalf("add to snapshot: %d", code)
	}
	if _, body := getJSONTok(t, smartURL, "s3cret"); len(numIDs(body["ids"])) != 2 {
		t.Fatalf("smart playlist doesn't match the video: %v", body["ids"])
	}
	for _, tok := range []string{"s3cret", ""} {
		_, detail := getJSONTok(t, base+"/media/"+itoa(vids[0]), tok)
		var got []int64
		for _, p := range detail["playlists"].([]any) {
			got = append(got, int64(p.(map[string]any)["id"].(float64)))
		}
		if !sameOrder(got, []int64{snapID}) {
			t.Errorf("token %q: detail playlists %v, want only the snapshot %d", tok, got, snapID)
		}
	}
}

// TestSmartPlaylistParity is §23.3's core assertion (risk 3): for a small corpus of
// queries crossed with every non-random sort, a smart playlist's tiles across pages,
// its ids and its item_count equal /media's pages, /media/ids and /media's total.
func TestSmartPlaylistParity(t *testing.T) {
	srv, r, sqlDB := filmEntityServer(t)
	base := srv.URL + "/api/v1"

	var ids []int64
	titles := []string{"Parity Kilo", "Parity Alpha", "Zebra Parity", "Parity Echo", "Parity Bravo", "Zebra Delta", "Parity Golf"}
	for i, title := range titles {
		id := seedPlainVideo(t, r, title)
		ids = append(ids, id)
		if _, err := sqlDB.Exec(`UPDATE videos SET indexed_at = ?, duration_sec = ?, width = ?, height = ? WHERE id = ?`,
			fmt.Sprintf("2026-03-%02dT00:00:00Z", 1+(i*5)%7), 60+(i*17)%9, 640+(i%3)*640, 360+(i%3)*360, id); err != nil {
			t.Fatal(err)
		}
	}
	tag := seedTag(t, sqlDB, ids[0], "parity-tag")
	studio := seedStudio(t, sqlDB, ids[1], "Parity Studio")
	for _, id := range ids[1:5] {
		sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, id, tag)
	}
	for _, id := range ids[3:] {
		sqlDB.Exec(`INSERT INTO video_studios (video_id, studio_id) VALUES (?, ?)`, id, studio)
	}
	for _, id := range ids[2:6] {
		linkPeople(t, r, id, "Parity Person")
	}
	person, _, err := r.PersonIDByName(context.Background(), "Parity Person")
	if err != nil {
		t.Fatal(err)
	}
	full := seedPlainVideo(t, r, "Parity Full")
	sqlDB.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, full, tag)
	attachFullFilm(t, r, full, "Parity Film")

	queries := []string{
		"tag=" + itoa(tag),
		"person=" + itoa(person),
		"studio_id=" + itoa(studio),
		"tag=" + itoa(tag) + "&person=" + itoa(person),
		"q=zebra",
		"",
	}
	sorts := []string{"added_desc", "added_asc", "title_asc", "title_desc", "duration_desc", "duration_asc",
		"resolution_desc", "resolution_asc", repo.SortCompletenessAsc, repo.SortCompletenessDesc}
	for _, sort := range sorts {
		if !repo.ValidSort(sort) {
			t.Fatalf("sort %q is not a browse sort", sort)
		}
	}
	for _, q := range queries {
		for _, sort := range sorts {
			qs := strings.TrimPrefix(q+"&sort="+sort, "&")
			t.Run(qs, func(t *testing.T) {
				plURL, _ := smartAs(t, base, filmTok, qs, nil)
				var smartTiles, mediaTiles []int64
				var mediaTotal float64
				var smartBody map[string]any
				for offset := 0; ; offset += 3 {
					_, sb := getJSONTok(t, fmt.Sprintf("%s?limit=3&offset=%d", plURL, offset), filmTok)
					_, mb := getJSONTok(t, fmt.Sprintf("%s/media?%s&limit=3&offset=%d", base, qs, offset), filmTok)
					smartTiles = append(smartTiles, itemIDs(sb["items"])...)
					mediaTiles = append(mediaTiles, itemIDs(mb["items"])...)
					smartBody, mediaTotal = sb, mb["total"].(float64)
					if len(itemIDs(mb["items"])) == 0 || len(mediaTiles) >= int(mediaTotal) {
						break
					}
				}
				if len(staleKinds(smartBody)) != 0 {
					t.Fatalf("stale %v", smartBody["stale_refs"])
				}
				if !sameOrder(smartTiles, mediaTiles) {
					t.Errorf("tiles differ\nsmart %v\nmedia %v", smartTiles, mediaTiles)
				}
				if c := smartBody["playlist"].(map[string]any)["item_count"].(float64); c != mediaTotal {
					t.Errorf("item_count %v, /media total %v", c, mediaTotal)
				}
				_, mi := getJSONTok(t, base+"/media/ids?"+qs, filmTok)
				if !sameOrder(numIDs(smartBody["ids"]), numIDs(mi["ids"])) || !sameOrder(numIDs(mi["ids"]), mediaTiles) {
					t.Errorf("ids differ\nsmart %v\n/media/ids %v\n/media pages %v", smartBody["ids"], mi["ids"], mediaTiles)
				}
				if len(mediaTiles) == 0 {
					t.Errorf("empty result: the corpus doesn't exercise this query")
				}
			})
		}
	}
}
