package api_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/query"
	"holodex/internal/repo"
)

// queryFixture is testing-strategy §38.2's library, built so every wrong reading of a
// motivating query returns a different set.
type queryFixture struct {
	base  string
	r     *repo.Repo
	db    *sql.DB
	v     map[string]int64 // video by title
	tag   map[string]int64
	ford  int64
	para  int64
	other int64
}

func newQueryFixture(t *testing.T) *queryFixture {
	t.Helper()
	srv, r, sqlDB := filmEntityServer(t)
	ctx := context.Background()
	fx := &queryFixture{base: srv.URL + "/api/v1", r: r, db: sqlDB, v: map[string]int64{}, tag: map[string]int64{}}
	exec := func(q string, args ...any) sql.Result {
		t.Helper()
		res, err := sqlDB.Exec(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return res
	}
	for _, name := range []string{"science-fiction", "fantasy", "dark-fantasy", "action", "comedy", "drama"} {
		id, _ := exec(`INSERT INTO tags (name) VALUES (?)`, name).LastInsertId()
		fx.tag[name] = id
	}
	exec(`UPDATE tags SET parent_tag_id = ? WHERE id = ?`, fx.tag["fantasy"], fx.tag["dark-fantasy"])
	if _, err := r.AddEntityAlias(ctx, model.EntityTag, fx.tag["science-fiction"], "sci-fi"); err != nil {
		t.Fatal(err)
	}
	fx.para, _ = exec(`INSERT INTO studios (name) VALUES ('Paramount')`).LastInsertId()
	fx.other, _ = exec(`INSERT INTO studios (name) VALUES ('Other Pictures')`).LastInsertId()

	type row struct {
		title    string
		dur, w   int
		year     int
		tags     []string
		ford     bool
		studios  []int64
		provider string
	}
	rows := []row{
		{"Blade Ward", 7200, 1920, 1982, []string{"science-fiction", "fantasy"}, true, nil, ""},
		{"Rush Hour", 5400, 1280, 1998, []string{"action"}, true, nil, ""},
		{"Funny Action", 90, 1920, 2001, []string{"action", "comedy"}, true, nil, ""},
		{"Cold Space", 6000, 3840, 2010, []string{"science-fiction"}, true, nil, ""},
		{"Other Hero", 5000, 1920, 2012, []string{"action"}, false, nil, ""},
		{"Neither", 4000, 640, 1975, []string{"drama"}, true, nil, ""},
		{"Wizard Moon", 6500, 1920, 2015, []string{"science-fiction", "dark-fantasy"}, false, nil, ""},
		{"Para Unmatched", 3000, 1920, 2005, nil, false, []int64{fx.para, fx.other}, ""},
		{"Para Matched", 3000, 1920, 2006, nil, false, []int64{fx.para}, "provider-1"},
		{"Other Unmatched", 3000, 1920, 0, nil, false, []int64{fx.other}, ""},
		{"Long Comedy", 200, 1920, 2020, []string{"comedy"}, false, nil, ""},
		{"Comedy Night", 3600, 1920, 2021, nil, false, nil, ""},
	}
	for i, rw := range rows {
		v := &model.Video{
			FilePath: "/q/" + itoa(int64(i)) + ".mkv", FileSize: 1, Title: rw.title, Duration: rw.dur,
			Width: rw.w, Height: rw.w * 9 / 16, FileMtime: time.Now().UTC().Truncate(time.Second),
		}
		if rw.year > 0 {
			at := time.Date(rw.year, 6, 1, 0, 0, 0, 0, time.UTC)
			v.RecordedAt = &at
		}
		id, err := r.UpsertVideo(ctx, v, nil)
		if err != nil {
			t.Fatal(err)
		}
		fx.v[rw.title] = id
		for _, tg := range rw.tags {
			exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (?, ?)`, id, fx.tag[tg])
		}
		for _, s := range rw.studios {
			exec(`INSERT INTO video_studios (video_id, studio_id) VALUES (?, ?)`, id, s)
		}
		if rw.ford {
			linkPeople(t, r, id, "Harrison Ford")
		}
		if rw.provider != "" {
			if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, id, rw.provider, "x1", map[string][]string{"title": {rw.title}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	fx.ford, _, _ = r.PersonIDByName(ctx, "Harrison Ford")
	return fx
}

// ids GETs /media/ids with a raw query string as the owner (token) or a visitor ("").
func (fx *queryFixture) ids(t *testing.T, rawQuery, token string) []int64 {
	t.Helper()
	code, body := getJSONTok(t, fx.base+"/media/ids?"+rawQuery, token)
	if code != http.StatusOK {
		t.Fatalf("/media/ids?%s: %d %v", rawQuery, code, body)
	}
	out := numIDs(body["ids"])
	slices.Sort(out)
	return out
}

// smart creates a smart playlist as the owner and returns its URL and stored query.
func (fx *queryFixture) smart(t *testing.T, q string, extra map[string]any) (string, string) {
	t.Helper()
	plURL, _ := smartAs(t, fx.base, filmTok, q, extra)
	_, body := getJSONTok(t, plURL, filmTok)
	return plURL, body["playlist"].(map[string]any)["query"].(string)
}

func (fx *queryFixture) expr(t *testing.T, e, token string) []int64 {
	t.Helper()
	return fx.ids(t, "expr="+url.QueryEscape(e), token)
}

func (fx *queryFixture) want(titles ...string) []int64 {
	out := []int64{}
	for _, ti := range titles {
		out = append(out, fx.v[ti])
	}
	slices.Sort(out)
	return out
}

func (fx *queryFixture) all(except ...string) []int64 {
	out := []int64{}
	for ti, id := range fx.v {
		if !slices.Contains(except, ti) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// A1: each flat key and the same condition in expr return the same ids.
func TestQueryExprEquivalence(t *testing.T) {
	fx := newQueryFixture(t)
	pairs := []struct{ flat, expr string }{
		{"tag=" + itoa(fx.tag["fantasy"]), "tag:" + itoa(fx.tag["fantasy"])},
		{"tag=" + itoa(fx.tag["fantasy"]), "tag:fantasy"},
		{"person=" + itoa(fx.ford), `person:"Harrison Ford"`},
		{"studio_id=" + itoa(fx.other), "studio:" + itoa(fx.other)},
		{"studio_id=" + itoa(fx.para), "studio:paramount"},
		{"duration_min=60", "duration:>=60m"},
		{"duration_max=60", "duration:<=1h"},
		{"duration_min=50&duration_max=100", "duration:50m..100m"},
		{"year_min=2000", "year:>=2000"},
		{"year_max=2000", "year:<=2000"},
		{"year_min=1990&year_max=2010", "year:1990..2010"},
		{"resolution=HD", "resolution:HD"},
		{"resolution=4K", "resolution:4k"},
		{"q=comedy", "comedy -tag:comedy"},
	}
	cat, err := fx.r.CreateCategory(context.Background(), "genres")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.db.Exec(`INSERT INTO category_tags (category_id, tag_id) VALUES (?, ?)`, cat.ID, fx.tag["drama"]); err != nil {
		t.Fatal(err)
	}
	pairs = append(pairs, struct{ flat, expr string }{"category_id=" + itoa(cat.ID), "category:genres"})
	for _, p := range pairs {
		flat := fx.ids(t, p.flat, filmTok)
		got := fx.expr(t, p.expr, filmTok)
		if p.flat == "q=comedy" {
			flat = slices.DeleteFunc(flat, func(id int64) bool { return id == fx.v["Long Comedy"] || id == fx.v["Funny Action"] })
		}
		if !slices.Equal(flat, got) {
			t.Errorf("?%s = %v, expr %q = %v", p.flat, flat, p.expr, got)
		}
	}
	if got := fx.expr(t, "tag:fantasy", filmTok); !slices.Equal(got, fx.want("Blade Ward", "Wizard Moon")) {
		t.Errorf("tag:fantasy should include the dark-fantasy descendant: %v", got)
	}
	if got := fx.expr(t, "studio:paramount studio:"+itoa(fx.other), filmTok); !slices.Equal(got, fx.want("Para Unmatched")) {
		t.Errorf("two studios ANDed: %v", got)
	}
}

// A2: the three motivating queries return exactly their sets.
func TestQueryMotivating(t *testing.T) {
	fx := newQueryFixture(t)
	q1 := `((tag:science-fiction tag:fantasy) OR tag:action) -tag:comedy person:"Harrison Ford"`
	if got := fx.expr(t, q1, filmTok); !slices.Equal(got, fx.want("Blade Ward", "Rush Hour")) {
		t.Errorf("query 1 = %v", got)
	}
	wrong := `(tag:science-fiction tag:fantasy) OR tag:action -tag:comedy person:"Harrison Ford"`
	if got := fx.expr(t, wrong, filmTok); !slices.Equal(got, fx.want("Blade Ward", "Rush Hour", "Wizard Moon")) {
		t.Errorf("query 1 without outer parentheses = %v", got)
	}
	if got := fx.expr(t, "studio:Paramount -matched:any", filmTok); !slices.Equal(got, fx.want("Para Unmatched")) {
		t.Errorf("query 2 = %v", got)
	}
	if got := fx.expr(t, "tag:comedy duration:<2m", filmTok); !slices.Equal(got, fx.want("Funny Action")) {
		t.Errorf("query 3 = %v", got)
	}
}

// A3: negation alone, the unmatched set, and an empty result.
func TestQueryNegationAndEmpty(t *testing.T) {
	fx := newQueryFixture(t)
	if got := fx.expr(t, "-tag:comedy", filmTok); !slices.Equal(got, fx.all("Funny Action", "Long Comedy")) {
		t.Errorf("-tag:comedy = %v", got)
	}
	if got := fx.expr(t, "-matched:any", filmTok); !slices.Equal(got, fx.all("Para Matched")) {
		t.Errorf("-matched:any = %v", got)
	}
	if got := fx.expr(t, "matched:provider-1", filmTok); !slices.Equal(got, fx.want("Para Matched")) {
		t.Errorf("matched:provider-1 = %v", got)
	}
	if got := fx.expr(t, "tag:comedy tag:drama", filmTok); len(got) != 0 {
		t.Errorf("an expr matching nothing = %v", got)
	}
	if got := fx.expr(t, "-year:2001", filmTok); !slices.Contains(got, fx.v["Other Unmatched"]) {
		t.Errorf("-year:2001 drops the video with no recorded date: %v", got)
	}
}

// A4: a bare word matches the title or the exact Tag; part of a Tag name is title only;
// a Tag created later widens a stored bare word.
func TestQueryBareWords(t *testing.T) {
	fx := newQueryFixture(t)
	if got := fx.expr(t, "comedy", filmTok); !slices.Equal(got, fx.want("Funny Action", "Long Comedy", "Comedy Night")) {
		t.Errorf("comedy = %v", got)
	}
	if got := fx.expr(t, "sci-fi", filmTok); !slices.Equal(got, fx.want("Blade Ward", "Cold Space", "Wizard Moon")) {
		t.Errorf("a bare alias = %v", got)
	}
	if got := fx.expr(t, "science", filmTok); len(got) != 0 {
		t.Errorf("science (part of science-fiction) = %v, want title matches only (none)", got)
	}
	plURL, _ := smartAs(t, fx.base, filmTok, "expr=western", nil)
	if _, body := getJSONTok(t, plURL, filmTok); len(numIDs(body["ids"])) != 0 {
		t.Fatalf("western before the tag exists: %v", body["ids"])
	}
	seedTag(t, fx.db, fx.v["Neither"], "Western")
	_, body := getJSONTok(t, plURL, filmTok)
	if got := numIDs(body["ids"]); len(got) != 1 || got[0] != fx.v["Neither"] {
		t.Errorf("western after the tag exists: %v", got)
	}
	if q := body["playlist"].(map[string]any)["query"]; q != "expr=western" {
		t.Errorf("stored query %v was rewritten", q)
	}
}

// A5: FTS and SQL syntax in a query are searched as data.
func TestQuerySyntaxAsData(t *testing.T) {
	fx := newQueryFixture(t)
	for _, e := range []string{`"*"`, "*", "NEAR", `"OR("`, `"'; DROP TABLE videos; --"`, `NEAR/2`, `tag:"it's"`, "a'b", "^title"} {
		code, body := getJSONTok(t, fx.base+"/media/ids?expr="+url.QueryEscape(e), filmTok)
		if code != http.StatusOK && !(code == http.StatusBadRequest && body["query_error"] != nil) {
			t.Errorf("expr %q: %d %v", e, code, body)
		}
	}
	if got := fx.expr(t, `"'; DROP TABLE videos; --"`, filmTok); len(got) != 0 {
		t.Errorf("injection-shaped word matched %v", got)
	}
	if got := fx.ids(t, "", filmTok); len(got) != len(fx.v) {
		t.Errorf("library changed after injection-shaped queries: %d videos", len(got))
	}
}

// A6: the owner gate and the caps on /media.
func TestQueryMediaGateAndCaps(t *testing.T) {
	fx := newQueryFixture(t)
	code, body := getJSONTok(t, fx.base+"/media?expr="+url.QueryEscape("tag:action OR -matched:any"), "")
	_, facetBody := getJSONTok(t, fx.base+"/media?missing_facet=title", "")
	if code != http.StatusUnauthorized || body["error"] != facetBody["error"] {
		t.Errorf("visitor matched: %d %v, want 401 like missing_facet (%v)", code, body, facetBody)
	}
	if code, _ := getJSONTok(t, fx.base+"/media?expr=matched:any", filmTok); code != http.StatusOK {
		t.Errorf("owner matched: %d", code)
	}
	if got := fx.expr(t, "tag:comedy", ""); !slices.Equal(got, fx.want("Funny Action", "Long Comedy")) {
		t.Errorf("visitor tag:comedy = %v", got)
	}
	for e, code := range map[string]string{
		strings.Repeat("a", query.MaxBytes+1):                                               query.CodeTooLong,
		strings.Repeat("(", query.MaxDepth+1) + "a" + strings.Repeat(")", query.MaxDepth+1): query.CodeTooDeep,
		strings.TrimSpace(strings.Repeat("a ", query.MaxConditions+1)):                      query.CodeTooMany,
		"tag:nosuchtag": query.CodeUnknownName,
	} {
		status, body := getJSONTok(t, fx.base+"/media?expr="+url.QueryEscape(e), "")
		qe, _ := body["query_error"].(map[string]any)
		if status != http.StatusBadRequest || qe == nil || qe["code"] != code {
			t.Errorf("expr of %d bytes: %d %v, want 400 %s", len(e), status, body, code)
		}
	}
}

// A7: the parse endpoint's canonical string, display tree, errors and gate.
func TestQueryParseEndpoint(t *testing.T) {
	fx := newQueryFixture(t)
	parse := func(e, token string) (int, map[string]any) {
		return getJSONTok(t, fx.base+"/query/parse?expr="+url.QueryEscape(e), token)
	}
	code, body := parse(`(tag:sci-fi tag:fantasy) OR tag:action -person:"harrison ford"`, "")
	want := "tag:" + itoa(fx.tag["science-fiction"]) + " tag:" + itoa(fx.tag["fantasy"]) + " OR tag:" + itoa(fx.tag["action"]) + " -person:" + itoa(fx.ford)
	if code != http.StatusOK || body["canonical"] != want {
		t.Fatalf("parse: %d %v, want canonical %q", code, body, want)
	}
	tree := body["tree"].(map[string]any)
	first := tree["items"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)
	if tree["kind"] != "or" || first["label"] != "science-fiction" || first["expr"] != "tag:"+itoa(fx.tag["science-fiction"]) {
		t.Errorf("display tree = %v", tree)
	}

	code, body = parse("tag:sciencefiction OR tag:action", "")
	qe, _ := body["query_error"].(map[string]any)
	if code != http.StatusBadRequest || qe["code"] != query.CodeUnknownName || qe["start"] != 4.0 || qe["end"] != 18.0 {
		t.Errorf("unknown tag: %d %v", code, body)
	}
	if sugg, _ := qe["suggestions"].([]any); len(sugg) == 0 || sugg[0] != "science-fiction" {
		t.Errorf("unknown tag suggestions = %v", qe["suggestions"])
	}

	code, body = parse("studio:paramount -matched:any", "")
	if qe, _ := body["query_error"].(map[string]any); code != http.StatusUnauthorized || qe["code"] != query.CodeOwnerOnly {
		t.Errorf("visitor matched: %d %v", code, body)
	}
	if code, _ = parse("studio:paramount -matched:any", filmTok); code != http.StatusOK {
		t.Errorf("owner matched: %d", code)
	}

	if _, err := fx.r.RenameEntity(context.Background(), model.EntityTag, fx.tag["science-fiction"], "sf"); err != nil {
		t.Fatal(err)
	}
	_, body = parse("tag:"+itoa(fx.tag["science-fiction"]), "")
	if body["canonical"] != "tag:"+itoa(fx.tag["science-fiction"]) || body["tree"].(map[string]any)["label"] != "sf" {
		t.Errorf("after rename: %v", body)
	}
}

// A8: a smart playlist stores the canonical expr; a public one can't hold matched:.
func TestQuerySmartPlaylistSave(t *testing.T) {
	fx := newQueryFixture(t)
	plURL, stored := fx.smart(t, "expr="+url.QueryEscape("tag:sci-fi -tag:comedy"), map[string]any{"name": "x"})
	_ = plURL
	want := "expr=" + url.QueryEscape("tag:"+itoa(fx.tag["science-fiction"])+" -tag:"+itoa(fx.tag["comedy"]))
	if stored != want {
		t.Errorf("stored %q, want %q", stored, want)
	}
	pub := map[string]any{"name": "pub", "query": "expr=" + url.QueryEscape("-matched:any"), "visibility": model.PlaylistPublic}
	if code, body := postTok(t, fx.base+"/playlists", filmTok, pub); code != http.StatusBadRequest {
		t.Errorf("public matched playlist: %d %v, want 400", code, body)
	}
	pub["visibility"] = model.PlaylistPrivate
	if code, body := postTok(t, fx.base+"/playlists", filmTok, pub); code != http.StatusCreated {
		t.Errorf("private matched playlist: %d %v, want 201", code, body)
	}
	bad := map[string]any{"name": "bad", "query": "expr=" + url.QueryEscape("tag:nosuchtag")}
	if code, _ := postTok(t, fx.base+"/playlists", filmTok, bad); code != http.StatusBadRequest {
		t.Errorf("unknown name in a saved expr: %d, want 400", code)
	}
}

// A9: stored queries with no expr read exactly as before — the stored string and the ids.
func TestQueryExistingPlaylistsUnchanged(t *testing.T) {
	fx := newQueryFixture(t)
	for _, q := range []string{
		"tag=" + itoa(fx.tag["action"]),
		"person=" + itoa(fx.ford),
		"studio_id=" + itoa(fx.para),
		"duration_max=2&q=funny",
		"resolution=FHD&year_min=2000",
	} {
		plURL, stored := fx.smart(t, q, nil)
		if stored != q {
			t.Errorf("stored %q, want %q", stored, q)
		}
		_, body := getJSONTok(t, plURL, filmTok)
		got := numIDs(body["ids"])
		slices.Sort(got)
		if want := fx.ids(t, q, filmTok); !slices.Equal(got, want) {
			t.Errorf("%s: playlist %v, /media %v", q, got, want)
		}
	}
}

// A10: a merge rewrites an id inside a stored expr; a deleted Tag in one is a stale ref.
func TestQuerySmartPlaylistMergeAndStale(t *testing.T) {
	fx := newQueryFixture(t)
	ctx := context.Background()
	linkPeople(t, fx.r, fx.v["Neither"], "H. Ford")
	dup, _, _ := fx.r.PersonIDByName(ctx, "H. Ford")
	plURL, _ := fx.smart(t, "expr="+url.QueryEscape("person:"+itoa(dup)+" OR tag:drama"), nil)
	if _, err := fx.r.MergePersonsWithAffectedVideos(ctx, fx.ford, dup); err != nil {
		t.Fatal(err)
	}
	_, body := getJSONTok(t, plURL, filmTok)
	want := "expr=" + url.QueryEscape("person:"+itoa(fx.ford)+" OR tag:"+itoa(fx.tag["drama"]))
	if q := body["playlist"].(map[string]any)["query"]; q != want || len(staleKinds(body)) != 0 {
		t.Errorf("after merge: query %v stale %v, want %q", q, body["stale_refs"], want)
	}

	tagURL, _ := fx.smart(t, "expr="+url.QueryEscape("tag:drama OR tag:action"), nil)
	if _, err := fx.db.Exec(`DELETE FROM tags WHERE id = ?`, fx.tag["drama"]); err != nil {
		t.Fatal(err)
	}
	_, body = getJSONTok(t, tagURL, filmTok)
	refs, _ := body["stale_refs"].([]any)
	if len(refs) != 1 || refs[0].(map[string]any)["key"] != "expr" || len(numIDs(body["ids"])) != 0 {
		t.Errorf("deleted tag in expr: stale %v ids %v", body["stale_refs"], body["ids"])
	}
}
