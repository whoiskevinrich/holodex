package api_test

import (
	"context"
	"net/http"
	"path"
	"strconv"
	"testing"
	"time"

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
