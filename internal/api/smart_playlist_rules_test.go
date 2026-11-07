package api_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// F89 smart playlist rules, slice 1 (testing-strategy §40): caps, `playlist:`
// references and the visibility rules around them.

// namedSmart creates a smart playlist from an F87 expression and returns its id.
func namedSmart(t *testing.T, base, name, expr string, extra map[string]any) int64 {
	t.Helper()
	body := map[string]any{"name": name, "query": "expr=" + url.QueryEscape(expr)}
	for k, v := range extra {
		body[k] = v
	}
	code, resp := postTok(t, base+"/playlists", "s3cret", body)
	if code != http.StatusCreated {
		t.Fatalf("create %q (%s): %d %v", name, expr, code, resp)
	}
	return int64(resp["playlist"].(map[string]any)["id"].(float64))
}

func plIDs(t *testing.T, base string, id int64, token string) ([]int64, map[string]any) {
	t.Helper()
	code, body := getJSONTok(t, base+"/playlists/"+itoa(id), token)
	if code != http.StatusOK {
		t.Fatalf("get playlist %d: %d %v", id, code, body)
	}
	return numIDs(body["ids"]), body
}

// R11 / P0-2: the cap cuts the count, the ids and Freeze, with the match count for the
// owner's readout; a reference to a capped playlist sees the capped set.
func TestSmartPlaylistCap(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	vids := seedPlaylistVideos(t, r, 5) // all amv

	capped := namedSmart(t, base, "Newest 3", "tag:amv", map[string]any{"sort": "added_desc", "cap": 3})
	ids, body := plIDs(t, base, capped, "s3cret")
	pl := body["playlist"].(map[string]any)
	if want := []int64{vids[4], vids[3], vids[2]}; !slices.Equal(ids, want) {
		t.Errorf("capped ids %v, want %v", ids, want)
	}
	if pl["item_count"].(float64) != 3 || pl["match_count"].(float64) != 5 || pl["cap"].(float64) != 3 {
		t.Errorf("counts: item %v match %v cap %v", pl["item_count"], pl["match_count"], pl["cap"])
	}

	// Another playlist that excludes it sees only the capped three removed.
	rest := namedSmart(t, base, "The rest", `tag:amv -playlist:"newest 3"`, map[string]any{"sort": "added_asc"})
	if ids, _ := plIDs(t, base, rest, "s3cret"); !slices.Equal(ids, []int64{vids[0], vids[1]}) {
		t.Errorf("tag:amv -playlist:capped = %v, want the two oldest", ids)
	}

	// Clearing the cap (null) restores the whole set; Freeze keeps what it shows.
	if code, resp := patchTok(t, base+"/playlists/"+itoa(capped), "s3cret", map[string]any{"cap": nil}); code != http.StatusOK || resp["playlist"].(map[string]any)["cap"] != nil {
		t.Fatalf("clear cap: %d %v", code, resp)
	}
	if ids, _ := plIDs(t, base, capped, "s3cret"); len(ids) != 5 {
		t.Errorf("uncapped: %v", ids)
	}
	patchTok(t, base+"/playlists/"+itoa(capped), "s3cret", map[string]any{"cap": 2})
	if code, resp := postTok(t, base+"/playlists/"+itoa(capped)+"/freeze", "s3cret", nil); code != http.StatusOK || resp["playlist"].(map[string]any)["cap"] != nil {
		t.Fatalf("freeze: %d %v", code, resp)
	}
	if ids, _ := plIDs(t, base, capped, "s3cret"); !slices.Equal(ids, []int64{vids[4], vids[3]}) {
		t.Errorf("frozen membership %v, want the capped two", ids)
	}
}

// A4 / SEC-7: cap bounds, and no cap on a snapshot.
func TestSmartPlaylistCapBounds(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 1)
	for _, bad := range []any{0, -1, 10001, 1.5, "25", 12345678901234567890.0} {
		if code, _ := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "x", "query": "tag=1", "cap": bad}); code != http.StatusBadRequest {
			t.Errorf("cap %v: %d, want 400", bad, code)
		}
	}
	for _, good := range []any{nil, 1, 10000} {
		if code, resp := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "x", "query": "q=a", "cap": good}); code != http.StatusCreated {
			t.Errorf("cap %v: %d %v, want 201", good, code, resp)
		}
	}
	if code, _ := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "snap", "cap": 5}); code != http.StatusBadRequest {
		t.Errorf("cap on a snapshot: %d, want 400", code)
	}
	if code, _ := postTok(t, base+"/playlists", "", map[string]any{"name": "x", "query": "q=a", "cap": 5}); code != http.StatusUnauthorized {
		t.Errorf("visitor create with a cap: %d, want 401", code)
	}
}

// R12 / Q7 / Q9 / A9: references read the referenced playlist as it is now; a rename
// changes nothing; a loop is refused at save and reads stale when another edit closes
// it; a Random + capped target is refused; deleting a target makes referrers stale.
func TestSmartPlaylistReferences(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	vids := seedPlaylistVideos(t, r, 4) // 0 and 2 are even

	b := namedSmart(t, base, "Evens", "tag:even", nil)
	a := namedSmart(t, base, "Odd amv", `tag:amv -playlist:Evens`, map[string]any{"sort": "added_asc"})
	if ids, _ := plIDs(t, base, a, "s3cret"); !slices.Equal(ids, []int64{vids[1], vids[3]}) {
		t.Fatalf("amv minus Evens = %v", ids)
	}
	// Browse reads the same reference, never widening the NOT.
	_, media := getJSONTok(t, base+"/media/ids?sort=added_asc&expr="+url.QueryEscape("tag:amv -playlist:Evens"), "s3cret")
	if got := numIDs(media["ids"]); !slices.Equal(got, []int64{vids[1], vids[3]}) {
		t.Errorf("/media amv minus Evens = %v", got)
	}
	patchTok(t, base+"/playlists/"+itoa(b), "s3cret", map[string]any{"name": "Even ones"})
	if ids, _ := plIDs(t, base, a, "s3cret"); !slices.Equal(ids, []int64{vids[1], vids[3]}) {
		t.Errorf("after renaming the target: %v", ids)
	}

	// B → A would close a loop: refused, naming both.
	code, resp := patchTok(t, base+"/playlists/"+itoa(b), "s3cret", map[string]any{"query": "expr=" + url.QueryEscape("playlist:"+itoa(a))})
	if code != http.StatusBadRequest || !strings.Contains(resp["error"].(string), "loop") || !strings.Contains(resp["error"].(string), "Odd amv") {
		t.Errorf("loop save: %d %v", code, resp)
	}
	if code, _ := patchTok(t, base+"/playlists/"+itoa(a), "s3cret", map[string]any{"query": "expr=" + url.QueryEscape("playlist:"+itoa(a))}); code != http.StatusBadRequest {
		t.Errorf("self-reference save: %d, want 400", code)
	}

	// A Random + capped target has no fixed contents.
	shuffled := namedSmart(t, base, "Shuffled 2", "tag:amv", map[string]any{"sort": "random", "cap": 2})
	if code, resp := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "x", "query": "expr=" + url.QueryEscape("playlist:"+itoa(shuffled))}); code != http.StatusBadRequest || !strings.Contains(resp["error"].(string), "shuffled and capped") {
		t.Errorf("ref to random+cap: %d %v", code, resp)
	}
	// Given Random + cap later, its referrer reads stale (RD13).
	patchTok(t, base+"/playlists/"+itoa(b), "s3cret", map[string]any{"sort": "random", "cap": 1})
	if _, body := plIDs(t, base, a, "s3cret"); !slices.Equal(staleKinds(body), []string{"unstable"}) {
		t.Errorf("referrer of a now random+capped target: stale %v", body["stale_refs"])
	}
	patchTok(t, base+"/playlists/"+itoa(b), "s3cret", map[string]any{"sort": "added_desc", "cap": nil})

	// The delete confirmation names the referrer; after the delete it is stale.
	_, body := plIDs(t, base, b, "s3cret")
	if refs := body["referenced_by"].([]any); len(refs) != 1 || refs[0].(map[string]any)["name"] != "Odd amv" {
		t.Errorf("referenced_by = %v", body["referenced_by"])
	}
	if code := sendTok(t, http.MethodDelete, base+"/playlists/"+itoa(b), "s3cret"); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	if ids, body := plIDs(t, base, a, "s3cret"); len(ids) != 0 || !slices.Equal(staleKinds(body), []string{"missing"}) {
		t.Errorf("after deleting the target: ids %v stale %v", ids, body["stale_refs"])
	}
}

// Q6 / SEC-5: four levels deep expand, a fifth is refused with the limit named.
func TestSmartPlaylistReferenceDepth(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 2)
	prev := namedSmart(t, base, "p0", "tag:amv", nil)
	for i := 1; i <= 4; i++ {
		prev = namedSmart(t, base, "p"+itoa(int64(i)), "playlist:"+itoa(prev), nil)
	}
	if ids, _ := plIDs(t, base, prev, "s3cret"); len(ids) != 2 {
		t.Errorf("4 deep: %v, want both videos", ids)
	}
	code, resp := postTok(t, base+"/playlists", "s3cret", map[string]any{"name": "p5", "query": "expr=" + url.QueryEscape("playlist:"+itoa(prev))})
	if code != http.StatusBadRequest || !strings.Contains(resp["error"].(string), "at most 4 deep") {
		t.Errorf("5 deep: %d %v", code, resp)
	}
}

// A1 / SEC-1: a visitor's reference to a private playlist reads exactly like one to a
// playlist that never existed.
func TestSmartPlaylistPrivateReferenceIsUnknown(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 1)
	priv := namedSmart(t, base, "Secret", "tag:amv", nil)

	for _, ep := range []string{"/media?expr=", "/query/parse?expr="} {
		cp, bp := getJSONTok(t, base+ep+url.QueryEscape("playlist:"+itoa(priv)), "")
		cu, bu := getJSONTok(t, base+ep+url.QueryEscape("playlist:987654"), "")
		qp, _ := bp["query_error"].(map[string]any)
		qu, _ := bu["query_error"].(map[string]any)
		if cp != cu || cp == http.StatusUnauthorized || cp == http.StatusForbidden || qp["code"] != qu["code"] {
			t.Errorf("%s: private %d %v vs unknown %d %v", ep, cp, bp, cu, bu)
		}
		cn, bn := getJSONTok(t, base+ep+url.QueryEscape(`playlist:"Secret"`), "")
		cx, bx := getJSONTok(t, base+ep+url.QueryEscape(`playlist:"Nothing here"`), "")
		if cn != cx || bn["query_error"].(map[string]any)["code"] != bx["query_error"].(map[string]any)["code"] {
			t.Errorf("%s by name: %d %v vs %d %v", ep, cn, bn, cx, bx)
		}
	}
	if code, _ := getJSONTok(t, base+"/media?expr="+url.QueryEscape("playlist:"+itoa(priv)), "s3cret"); code != http.StatusOK {
		t.Errorf("owner reference: %d", code)
	}
}

// P0-2: a request can't page past the cap, with a negative offset or any other.
func TestSmartPlaylistCapHoldsForAnyOffset(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 5)
	capped := namedSmart(t, base, "Newest 2", "tag:amv", map[string]any{"sort": "added_desc", "cap": 2})
	code, body := getJSONTok(t, base+"/playlists/"+itoa(capped)+"?offset=-100000&limit=10", "s3cret")
	if code != http.StatusOK {
		t.Fatalf("%d %v", code, body)
	}
	if items, _ := body["items"].([]any); len(items) > 2 {
		t.Errorf("%d tiles past a cap of 2", len(items))
	}
}

// P0-4 / SEC-11: ?check=1 runs every rule of a save and writes nothing, and names the chip
// a deep problem comes through.
func TestSmartPlaylistDefinitionCheck(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 2)
	inner := namedSmart(t, base, "Inner", "tag:even", nil)
	middle := namedSmart(t, base, "Middle", "playlist:"+itoa(inner), nil)
	outer := namedSmart(t, base, "Outer", "tag:amv", nil)
	url1 := base + "/playlists/" + itoa(outer)
	draft := "expr=" + url.QueryEscape("-playlist:"+itoa(middle))

	code, resp := patchTok(t, url1+"?check=1", "s3cret", map[string]any{"query": draft, "cap": 5})
	if code != http.StatusOK || resp["ok"] != true {
		t.Fatalf("clean draft: %d %v", code, resp)
	}
	_, body := getJSONTok(t, url1, "s3cret")
	if pl := body["playlist"].(map[string]any); pl["cap"] != nil || strings.Contains(pl["query"].(string), "playlist") {
		t.Errorf("check wrote: %v", pl)
	}
	if code, _ := patchTok(t, url1+"?check=1", "", map[string]any{"query": draft}); code != http.StatusUnauthorized {
		t.Errorf("visitor check: %d, want 401", code)
	}
	// Inner → Outer would close Outer → Middle → Inner → Outer once saved.
	patchTok(t, url1, "s3cret", map[string]any{"query": draft})
	code, resp = patchTok(t, base+"/playlists/"+itoa(inner)+"?check=1", "s3cret", map[string]any{"query": "expr=" + url.QueryEscape("playlist:"+itoa(outer))})
	refs, _ := resp["stale_refs"].([]any)
	if code != http.StatusBadRequest || len(refs) != 1 || refs[0].(map[string]any)["kind"] != "loop" ||
		refs[0].(map[string]any)["via"].(float64) != float64(outer) || resp["fix"] != "Remove “in: Outer”." {
		t.Errorf("deep loop: %d %v", code, resp)
	}
	// Going public through a chain names the direct reference, the chip to remove.
	code, resp = patchTok(t, url1, "s3cret", map[string]any{"visibility": "public"})
	if code != http.StatusBadRequest || !strings.Contains(resp["error"].(string), "“Middle”, which is private") {
		t.Errorf("public via private: %d %v", code, resp)
	}
}

// P0-4: a dry run reports every problem at once; a real save still stops at the first.
func TestSmartPlaylistCheckReportsEveryProblem(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 2)
	a := namedSmart(t, base, "A", "tag:amv", nil)
	b := namedSmart(t, base, "B", "playlist:"+itoa(a), nil)
	patchTok(t, base+"/playlists/"+itoa(a), "s3cret", map[string]any{"visibility": "public"})
	patchTok(t, base+"/playlists/"+itoa(b), "s3cret", map[string]any{"visibility": "public"})
	url1 := base + "/playlists/" + itoa(a)
	draft := map[string]any{"query": "expr=" + url.QueryEscape("watched:no playlist:"+itoa(b))}

	code, resp := patchTok(t, url1+"?check=1", "s3cret", draft)
	also, _ := resp["also"].([]any)
	if code != http.StatusBadRequest || resp["owner_only"] != true || len(also) == 0 {
		t.Fatalf("check: %d %v", code, resp)
	}
	var loop bool
	for _, p := range also {
		m := p.(map[string]any)
		if strings.Contains(m["error"].(string), "would make a loop") && m["via"].(float64) == float64(b) {
			loop = true
		}
	}
	if !loop {
		t.Errorf("no loop among %v", also)
	}

	code, resp = patchTok(t, url1, "s3cret", draft)
	if code != http.StatusBadRequest || resp["owner_only"] != true || resp["also"] != nil {
		t.Errorf("save: %d %v", code, resp)
	}
}

// A stored definition parses as itself: a deleted reference is marked, not re-read as a
// name, and only for the owner.
func TestQueryParseStored(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 1)
	expr := url.QueryEscape("playlist:987654")
	code, body := getJSONTok(t, base+"/query/parse?stored=1&expr="+expr, "s3cret")
	tree, _ := body["tree"].(map[string]any)
	if code != http.StatusOK || tree["missing"] != true || tree["label"] != "#987654" {
		t.Errorf("stored as owner: %d %v", code, body)
	}
	if code, _ := getJSONTok(t, base+"/query/parse?stored=1&expr="+expr, ""); code != http.StatusBadRequest {
		t.Errorf("stored as visitor: %d, want the normal unknown-name 400", code)
	}
}

// A2 / A11 / SEC-2 / SEC-3: going public is refused while the playlist reaches a private
// one; going private always works and turns the public referrer stale, which a visitor
// sees with no detail; a visitor never gets a smart playlist's definition.
func TestSmartPlaylistReferenceVisibility(t *testing.T) {
	srv, r := identityServer(t, "s3cret")
	base := srv.URL + "/api/v1"
	seedPlaylistVideos(t, r, 2)
	target := namedSmart(t, base, "Target", "tag:even", nil)
	referrer := namedSmart(t, base, "Referrer", "tag:amv -playlist:"+itoa(target), nil)

	code, resp := patchTok(t, base+"/playlists/"+itoa(referrer), "s3cret", map[string]any{"visibility": "public"})
	if code != http.StatusBadRequest || !strings.Contains(resp["error"].(string), "Target") {
		t.Errorf("public referrer of a private target: %d %v", code, resp)
	}
	if code, _ := patchTok(t, base+"/playlists/"+itoa(target), "s3cret", map[string]any{"visibility": "public"}); code != http.StatusOK {
		t.Fatalf("target public: %d", code)
	}
	if code, _ := patchTok(t, base+"/playlists/"+itoa(referrer), "s3cret", map[string]any{"visibility": "public"}); code != http.StatusOK {
		t.Fatalf("referrer public: %d", code)
	}
	_, body := plIDs(t, base, referrer, "")
	if pl := body["playlist"].(map[string]any); pl["query"] != nil || pl["smart"] != true {
		t.Errorf("visitor sees query %v smart %v", pl["query"], pl["smart"])
	}
	_, list := getJSONTok(t, base+"/playlists", "")
	for _, it := range list["items"].([]any) {
		if it.(map[string]any)["query"] != nil {
			t.Errorf("visitor list carries a query: %v", it)
		}
	}

	// Going private always succeeds; the public referrer goes stale.
	if code, _ := patchTok(t, base+"/playlists/"+itoa(target), "s3cret", map[string]any{"visibility": "private"}); code != http.StatusOK {
		t.Fatalf("target private: %d", code)
	}
	ids, body := plIDs(t, base, referrer, "")
	if len(ids) != 0 || !slices.Equal(staleKinds(body), []string{"owner_only"}) {
		t.Errorf("visitor on the stale referrer: ids %v stale %v", ids, body["stale_refs"])
	}
	if refs := body["stale_refs"].([]any); refs[0].(map[string]any)["value"] != nil {
		t.Errorf("visitor's stale ref names something: %v", refs)
	}
	// Browsing through the public referrer names nothing private either.
	if code, resp := getJSONTok(t, base+"/media?expr="+url.QueryEscape("playlist:"+itoa(referrer)), ""); code != http.StatusBadRequest || strings.Contains(resp["error"].(string), "Target") {
		t.Errorf("visitor browse via the stale referrer: %d %v", code, resp)
	}
	if _, body := plIDs(t, base, referrer, "s3cret"); !slices.Equal(staleKinds(body), []string{"owner_only"}) {
		t.Errorf("owner on the stale referrer: %v", body["stale_refs"])
	}
}
