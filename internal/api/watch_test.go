package api_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"holodex/internal/api"
)

// F89 slice 4 (testing-strategy §40.4, §40.5). Plays, the mark, the three conditions and
// where the fact may and may not appear.

func (e *rvEnv) play(t *testing.T, id int64) {
	t.Helper()
	if err := e.repo.RecordPlay(context.Background(), id, 0.9); err != nil {
		t.Fatal(err)
	}
}

func (e *rvEnv) mark(t *testing.T, id int64, watched bool) {
	t.Helper()
	if err := e.repo.SetWatchedMark(context.Background(), id, watched); err != nil {
		t.Fatal(err)
	}
}

// raw reads a response body as text, to look for a fact anywhere in it.
func (e *rvEnv) raw(t *testing.T, u, token string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// R9: the mark decides until the next counted play, which clears it.
func TestWatchedMarkPrecedence(t *testing.T) {
	e := newRVEnv(t)
	never := e.video(t, "/m/never.mkv")
	marked := e.video(t, "/m/marked.mkv")
	unmarked := e.video(t, "/m/unmarked.mkv")
	played := e.video(t, "/m/played.mkv")
	e.mark(t, marked, true)
	e.play(t, unmarked)
	e.play(t, unmarked)
	e.mark(t, unmarked, false)
	e.play(t, played)

	e.want(t, `watched:yes`, marked, played)
	e.want(t, `watched:no`, never, unmarked)
	e.want(t, `plays:0`, never, marked)
	e.want(t, `plays:2`, unmarked)
	e.want(t, `plays:>=1`, unmarked, played)

	// A counted play after either mark clears it.
	e.mark(t, played, false)
	e.want(t, `watched:no`, never, unmarked, played)
	e.play(t, played)
	e.play(t, unmarked)
	e.want(t, `watched:yes`, marked, unmarked, played)

	// A mark isn't a play: played: never sees it.
	e.want(t, `played:<7d`, unmarked, played)
	e.want(t, `-played:<7d`, never, marked)
	e.want(t, `played:>7d`)
}

// P0-8's own example, and the conditions composing with the rest of the language.
func TestWatchConditionsCompose(t *testing.T) {
	e := newRVEnv(t)
	seen := e.video(t, "/m/seen.mkv", tag("Director", "Ann Lee"))
	unseen := e.video(t, "/m/unseen.mkv", tag("Director", "Ann Lee"))
	other := e.video(t, "/m/other.mkv", tag("Director", "Del Moss"))
	e.play(t, seen)

	e.want(t, `watched:no director:"Ann Lee"`, unseen)
	e.want(t, `watched:yes OR director:"Del Moss"`, seen, other)
	e.want(t, `plays:0..1 -director:"Ann Lee"`, other)
}

// watched and unwatched are each other's negation, in one spelling.
func TestWatchedNegationIsOneSpelling(t *testing.T) {
	e := newRVEnv(t)
	base := e.srv.URL + "/api/v1/query/parse?expr="
	for in, want := range map[string]string{`-watched:yes`: "watched:no", `NOT watched:no`: "watched:yes", `--watched:yes`: "watched:yes", `watched:YES`: "watched:yes"} {
		_, body := getJSONTok(t, base+url.QueryEscape(in), "s3cret")
		if got := body["canonical"]; got != want {
			t.Errorf("%s reads as %v, want %s", in, got, want)
		}
	}
	code, body := getJSONTok(t, base+url.QueryEscape(`watched:maybe`), "s3cret")
	if code != http.StatusBadRequest {
		t.Errorf("watched:maybe = %d %v", code, body)
	}
}

// A6: a visitor is refused every one of the three, under a NOT and in an OR group too.
func TestWatchConditionsAreOwnerOnly(t *testing.T) {
	e := newRVEnv(t)
	e.video(t, "/m/a.mkv")
	base := e.srv.URL + "/api/v1"
	for _, expr := range []string{`watched:no`, `-watched:no`, `plays:>1`, `played:<7d`, `-played:<7d`, `year:2020 OR plays:0`} {
		for _, path := range []string{"/media?expr=", "/media/ids?expr=", "/query/parse?expr="} {
			if code, _ := getJSONTok(t, base+path+url.QueryEscape(expr), ""); code != http.StatusUnauthorized {
				t.Errorf("visitor %s%s = %d, want 401", path, expr, code)
			}
		}
		if code, _ := getJSONTok(t, base+"/media/ids?expr="+url.QueryEscape(expr), "s3cret"); code != http.StatusOK {
			t.Errorf("owner %s = %d", expr, code)
		}
	}
}

// A7: both writes sit behind the owner gate, and write nothing for a visitor.
func TestWatchWritesAreOwnerOnly(t *testing.T) {
	e := newRVEnv(t)
	id := e.video(t, "/m/a.mkv")
	path := "/api/v1/media/" + itoa(id)
	send := func(method, route, body, token string) int {
		t.Helper()
		req, err := http.NewRequest(method, e.srv.URL+route, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set(api.AdminTokenHeader, token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := send(http.MethodPost, path+"/plays", `{"progress":0.9}`, ""); code != http.StatusUnauthorized {
		t.Errorf("visitor play = %d", code)
	}
	if code := send(http.MethodPut, path+"/watched", `{"watched":true}`, ""); code != http.StatusUnauthorized {
		t.Errorf("visitor mark = %d", code)
	}
	e.want(t, `plays:>=1 OR watched:yes`)

	for _, body := range []string{``, `{}`, `{"progress":-0.1}`, `{"progress":1.1}`, `{"progress":"0.9"}`} {
		if code := send(http.MethodPost, path+"/plays", body, "s3cret"); code != http.StatusBadRequest {
			t.Errorf("play with %q = %d", body, code)
		}
	}
	for _, body := range []string{``, `{}`, `{"watched":"yes"}`} {
		if code := send(http.MethodPut, path+"/watched", body, "s3cret"); code != http.StatusBadRequest {
			t.Errorf("mark with %q = %d", body, code)
		}
	}
	if code := send(http.MethodPost, "/api/v1/media/9999/plays", `{"progress":0.9}`, "s3cret"); code != http.StatusNotFound {
		t.Errorf("play on no video = %d", code)
	}

	if code := send(http.MethodPost, path+"/plays", `{"progress":0.9}`, "s3cret"); code != http.StatusNoContent {
		t.Errorf("owner play = %d", code)
	}
	e.want(t, `plays:1`, id)
	if code := send(http.MethodPut, path+"/watched", `{"watched":false}`, "s3cret"); code != http.StatusNoContent {
		t.Errorf("owner mark = %d", code)
	}
	e.want(t, `watched:no plays:1`, id)
}

// A7: the fact reaches the owner's tiles and media page, and no visitor response carries it.
func TestWatchedFactIsTheOwners(t *testing.T) {
	e := newRVEnv(t)
	id := e.video(t, "/m/a.mkv")
	e.play(t, id)
	base := e.srv.URL + "/api/v1"
	for _, path := range []string{"/media", "/media/" + itoa(id)} {
		if !strings.Contains(e.raw(t, base+path, "s3cret"), `"watched":true`) {
			t.Errorf("owner %s carries no watched fact", path)
		}
		if strings.Contains(e.raw(t, base+path, ""), `"watched"`) {
			t.Errorf("visitor %s carries the watched fact", path)
		}
	}
}

// A7: the film page and the related shelves carry the fact the same way.
func TestWatchedFactOnFilmsAndShelves(t *testing.T) {
	e := newRVEnv(t)
	id := e.video(t, "/m/a.mkv")
	other := e.video(t, "/m/b.mkv")
	e.play(t, other)
	base := e.srv.URL + "/api/v1"
	for _, v := range []int64{id, other} {
		if code, body := reqTokBody(t, http.MethodPost, base+"/media/"+itoa(v)+"/tags", "s3cret", map[string]any{"name": "noir"}); code >= 300 {
			t.Fatalf("tag: %d %v", code, body)
		}
	}
	res, err := e.db.Exec(`INSERT INTO films (name) VALUES ('A film')`)
	if err != nil {
		t.Fatal(err)
	}
	film, _ := res.LastInsertId()
	if _, err := e.db.Exec(`INSERT INTO film_videos (film_id, video_id, is_full_film, created_at) VALUES (?, ?, 0, '2026-01-01T00:00:00Z')`, film, other); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/films/" + itoa(film), "/media/" + itoa(id) + "/related"} {
		owner, visitor := e.raw(t, base+path, "s3cret"), e.raw(t, base+path, "")
		if !strings.Contains(owner, `"watched":true`) {
			t.Errorf("owner %s carries no watched fact: %s", path, owner)
		}
		if strings.Contains(visitor, `"watched"`) {
			t.Errorf("visitor %s carries the watched fact", path)
		}
	}
}

// QA 6, SEC-4: a playlist using a watch condition can't go public, and can't be read by a
// visitor. The owner-only copy is the shared one.
func TestWatchConditionsKeepAPlaylistPrivate(t *testing.T) {
	e := newRVEnv(t)
	e.video(t, "/m/a.mkv")
	base := e.srv.URL + "/api/v1"
	for _, q := range []string{"watched:no", "plays:0", "played:<7d", "-played:<7d"} {
		pl := namedSmart(t, base, q, q, nil)
		if code, resp := reqTokBody(t, http.MethodPatch, base+"/playlists/"+itoa(pl), "s3cret", map[string]any{"visibility": "public"}); code != http.StatusBadRequest {
			t.Errorf("%s went public: %d %v", q, code, resp)
		}
		if code, _ := getJSONTok(t, base+"/playlists/"+itoa(pl), ""); code == http.StatusOK {
			t.Errorf("visitor read private %s", q)
		}
	}
}

// RD14: a frozen snapshot can go public whatever built it, and shows a visitor its videos
// with no condition and no watched fact.
func TestFrozenWatchPlaylistCanGoPublic(t *testing.T) {
	e := newRVEnv(t)
	id := e.video(t, "/m/a.mkv")
	e.video(t, "/m/b.mkv")
	e.play(t, id)
	base := e.srv.URL + "/api/v1"
	pl := namedSmart(t, base, "Seen", "watched:yes", nil)
	path := base + "/playlists/" + itoa(pl)
	if code, resp := postTok(t, path+"/freeze", "s3cret", nil); code != http.StatusOK {
		t.Fatalf("freeze: %d %v", code, resp)
	}
	if code, resp := reqTokBody(t, http.MethodPatch, path, "s3cret", map[string]any{"visibility": "public"}); code != http.StatusOK {
		t.Fatalf("public: %d %v", code, resp)
	}
	code, body := getJSONTok(t, path, "")
	if code != http.StatusOK {
		t.Fatalf("visitor read: %d %v", code, body)
	}
	if got := numIDs(body["ids"]); len(got) != 1 || got[0] != id {
		t.Errorf("visitor ids = %v, want [%d]", got, id)
	}
	if text := e.raw(t, path, ""); strings.Contains(text, "watched") {
		t.Errorf("visitor response names watched: %s", text)
	}
}
