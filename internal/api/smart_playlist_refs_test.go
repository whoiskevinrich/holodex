package api_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// F89 reference limits and transitive visibility (testing-strategy §40.3 Q6, Q8; §40.5 A2,
// A3, A8). Expansion lives in the api layer, so these run through its routes.

const refClean = "duration:>=1s" // matches every fixture video

// refChain builds top → … → bottom of depth references, each named L<n>, and returns them
// top first. public makes every link public, bottom up, so each step is allowed.
func refChain(t *testing.T, base string, depth int, public bool) []int64 {
	t.Helper()
	ids := make([]int64, depth+1)
	for i := depth; i >= 0; i-- {
		expr := refClean
		if i < depth {
			expr = "playlist:" + itoa(ids[i+1])
		}
		ids[i] = namedSmart(t, base, "L"+strconv.Itoa(i), expr, nil)
		if public {
			setVisibility(t, base, ids[i], "public", http.StatusOK)
		}
	}
	return ids
}

func setVisibility(t *testing.T, base string, id int64, v string, want int) map[string]any {
	t.Helper()
	code, resp := patchTok(t, base+"/playlists/"+itoa(id), "s3cret", map[string]any{"visibility": v})
	if code != want {
		t.Fatalf("visibility %s on %d: %d %v, want %d", v, id, code, resp, want)
	}
	return resp
}

func firstStale(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	refs, _ := resp["stale_refs"].([]any)
	if len(refs) == 0 {
		t.Fatalf("no stale refs in %v", resp)
	}
	return refs[0].(map[string]any)
}

// Q6, depth: four references deep expands, five is refused naming the limit, and a second
// edit that pushes a saved chain past it makes the top stale rather than running it.
func TestRefDepthLimit(t *testing.T) {
	e := newRVEnv(t)
	v := e.video(t, "/m/a.mkv")
	base := e.srv.URL + "/api/v1"
	chain := refChain(t, base, 4, false)
	if ids, body := plIDs(t, base, chain[0], "s3cret"); len(ids) != 1 || ids[0] != v {
		t.Fatalf("4 deep: ids %v stale %v", ids, body["stale_refs"])
	}

	code, resp := postTok(t, base+"/playlists", "s3cret", map[string]any{
		"name": "Too deep", "query": "expr=" + url.QueryEscape("playlist:"+itoa(chain[0])),
	})
	if code != http.StatusBadRequest {
		t.Fatalf("5 deep saved: %d %v", code, resp)
	}
	if s := firstStale(t, resp); s["kind"] != "limit" || s["key"] != "depth" || s["value"] != "4" {
		t.Errorf("5 deep refused with %v", s)
	}
	if !strings.Contains(resp["error"].(string), "4 deep") {
		t.Errorf("refusal doesn't carry the limit: %v", resp["error"])
	}

	// The bottom gains a reference: fine on its own, too deep from the top.
	extra := namedSmart(t, base, "Extra", refClean, nil)
	code, resp = patchTok(t, base+"/playlists/"+itoa(chain[4]), "s3cret", map[string]any{"query": "expr=" + url.QueryEscape("playlist:"+itoa(extra))})
	if code != http.StatusOK {
		t.Fatalf("bottom edit: %d %v", code, resp)
	}
	ids, body := plIDs(t, base, chain[0], "s3cret")
	if len(ids) != 0 || firstStale(t, body)["key"] != "depth" {
		t.Errorf("top past the limit: ids %v stale %v", ids, body["stale_refs"])
	}
}

// Q6, fan-out: sixteen distinct playlists expand, seventeen are refused.
func TestRefPlaylistLimit(t *testing.T) {
	e := newRVEnv(t)
	v := e.video(t, "/m/a.mkv")
	base := e.srv.URL + "/api/v1"
	var refs []string
	for i := range 17 {
		refs = append(refs, "playlist:"+itoa(namedSmart(t, base, fmt.Sprintf("Star %d", i), refClean, nil)))
	}
	star := namedSmart(t, base, "Sixteen", strings.Join(refs[:16], " OR "), nil)
	if ids, body := plIDs(t, base, star, "s3cret"); len(ids) != 1 || ids[0] != v {
		t.Errorf("16 playlists: ids %v stale %v", ids, body["stale_refs"])
	}
	code, resp := postTok(t, base+"/playlists", "s3cret", map[string]any{
		"name": "Seventeen", "query": "expr=" + url.QueryEscape(strings.Join(refs, " OR ")),
	})
	if code != http.StatusBadRequest {
		t.Fatalf("17 playlists saved: %d %v", code, resp)
	}
	if s := firstStale(t, resp); s["kind"] != "limit" || s["key"] != "playlists" || s["value"] != "16" {
		t.Errorf("17 refused with %v", s)
	}
}

// Q6, conditions: 256 across a definition and what it includes pass, 257 are refused. Each
// stored string stays inside F87's own cap of 64.
func TestRefConditionLimit(t *testing.T) {
	e := newRVEnv(t)
	v := e.video(t, "/m/a.mkv")
	base := e.srv.URL + "/api/v1"
	leaf := func(name string, n int) int64 {
		conds := []string{refClean}
		for i := 1; i < n; i++ {
			conds = append(conds, "year:"+strconv.Itoa(1900+i))
		}
		return namedSmart(t, base, name, strings.Join(conds, " OR "), nil)
	}
	full := []int64{leaf("A", 64), leaf("B", 64), leaf("C", 64)}
	expr := func(last int64) string {
		return "playlist:" + itoa(full[0]) + " OR playlist:" + itoa(full[1]) + " OR playlist:" + itoa(full[2]) + " OR playlist:" + itoa(last)
	}
	// Four references of its own, 192 in three leaves, so 60 in the last makes 256.
	exact := namedSmart(t, base, "Exactly", expr(leaf("D", 60)), nil)
	if ids, body := plIDs(t, base, exact, "s3cret"); len(ids) != 1 || ids[0] != v {
		t.Errorf("256 conditions: ids %v stale %v", ids, body["stale_refs"])
	}
	code, resp := postTok(t, base+"/playlists", "s3cret", map[string]any{
		"name": "One over", "query": "expr=" + url.QueryEscape(expr(leaf("E", 61))),
	})
	if code != http.StatusBadRequest {
		t.Fatalf("257 conditions saved: %d %v", code, resp)
	}
	if s := firstStale(t, resp); s["kind"] != "limit" || s["key"] != "conditions" || s["value"] != "256" {
		t.Errorf("257 refused with %v", s)
	}
}

// Q8, A2, A3: at every depth, a private playlist or an owner-only condition at the bottom of
// a public chain stops the top: a visitor gets nothing and no names, the owner sees why, the
// top can't go public again or save anything else, and the same chain clean reads through.
func TestRefVisibilityIsTransitive(t *testing.T) {
	causes := []struct {
		name  string
		apply func(t *testing.T, e *rvEnv, base string, bottom int64)
	}{
		{"private bottom", func(t *testing.T, e *rvEnv, base string, bottom int64) {
			setVisibility(t, base, bottom, "private", http.StatusOK) // SEC-3: never refused
		}},
		// No save can leave a public playlist holding these, so an older row stands in.
		{"watched at the bottom", func(t *testing.T, e *rvEnv, base string, bottom int64) {
			if _, err := e.db.Exec(`UPDATE playlists SET query = ? WHERE id = ?`, "expr=watched%3Ano", bottom); err != nil {
				t.Fatal(err)
			}
		}},
		{"matched at the bottom", func(t *testing.T, e *rvEnv, base string, bottom int64) {
			if _, err := e.db.Exec(`UPDATE playlists SET query = ? WHERE id = ?`, "expr=matched%3Aany", bottom); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for depth := 1; depth <= 4; depth++ {
		for _, cause := range causes {
			t.Run(fmt.Sprintf("depth %d, %s", depth, cause.name), func(t *testing.T) {
				e := newRVEnv(t)
				v := e.video(t, "/m/a.mkv")
				base := e.srv.URL + "/api/v1"
				chain := refChain(t, base, depth, true)
				top, direct, bottom := chain[0], "L1", chain[depth]

				if ids, _ := plIDs(t, base, top, ""); len(ids) != 1 || ids[0] != v {
					t.Fatalf("clean public chain reads %v", ids)
				}
				cause.apply(t, e, base, bottom)

				ids, body := plIDs(t, base, top, "")
				if len(ids) != 0 || firstStale(t, body)["kind"] != "owner_only" {
					t.Errorf("visitor on the top: ids %v stale %v", ids, body["stale_refs"])
				}
				if raw := e.raw(t, base+"/playlists/"+itoa(top), ""); strings.Contains(raw, "L"+strconv.Itoa(depth)) || strings.Contains(raw, "watched") || strings.Contains(raw, "matched") {
					t.Errorf("visitor's notice names something: %s", raw)
				}
				if code, _ := getJSONTok(t, base+"/media?expr="+url.QueryEscape("playlist:"+itoa(top)), ""); code != http.StatusBadRequest {
					t.Errorf("visitor browsing through the top: %d", code)
				}
				if _, body := plIDs(t, base, top, "s3cret"); firstStale(t, body)["kind"] != "owner_only" {
					t.Errorf("owner on the top: %v", body["stale_refs"])
				}

				// A3: a save that doesn't touch the reference is still refused.
				if code, resp := patchTok(t, base+"/playlists/"+itoa(top), "s3cret", map[string]any{"cap": 5}); code != http.StatusBadRequest {
					t.Errorf("cap on the top: %d %v", code, resp)
				}
				// A2: back to private is always allowed; public again is refused, naming the
				// reference the owner can remove, however deep the cause.
				setVisibility(t, base, top, "private", http.StatusOK)
				resp := setVisibility(t, base, top, "public", http.StatusBadRequest)
				if !strings.Contains(resp["error"].(string), direct) {
					t.Errorf("refusal doesn't name %s: %v", direct, resp["error"])
				}
			})
		}
	}
}

// A2: the cause in the playlist's own definition gets the same refusal, and needs no
// reference to reach.
func TestOwnConditionKeepsAPlaylistPrivate(t *testing.T) {
	e := newRVEnv(t)
	e.video(t, "/m/a.mkv")
	base := e.srv.URL + "/api/v1"
	for _, expr := range []string{"watched:no", "matched:any", "container:matroska"} {
		pl := namedSmart(t, base, expr, expr, nil)
		setVisibility(t, base, pl, "public", http.StatusBadRequest)
		code, resp := patchTok(t, base+"/playlists/"+itoa(pl), "s3cret", map[string]any{"visibility": "public", "query": "expr=" + url.QueryEscape(refClean)})
		if code != http.StatusOK {
			t.Errorf("%s replaced and public: %d %v", expr, code, resp)
		}
	}
}
