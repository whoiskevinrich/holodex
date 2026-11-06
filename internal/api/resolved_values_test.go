package api_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/query"
	"holodex/internal/repo"
)

// F89 slice 3 (testing-strategy §40): field conditions over the resolver's winning values,
// and the change stamp. With no decision the file's value wins a replace field like title.
// director is person-linked, which the mapping always makes a merged list: its sources
// union and take no decision, so title carries the decision cases.

const rvMappings = "fields:\n" +
	"  - canonical: title\n    label: Title\n    sources: [fake:title, file:title]\n" +
	"  - canonical: director\n    label: Director\n    sources: [fake:director, file:Director]\n" +
	"  - canonical: actors\n    label: Actors\n    multi: true\n    sources: [file:Cast]\n" +
	"  - canonical: release_date\n    label: Release date\n    sources: [file:Year]\n" +
	"  - canonical: runtime\n    label: Runtime\n    sources: [file:Runtime]\n" +
	"  - canonical: container\n    label: Container\n    sources: [file:Format]\n"

type rvEnv struct {
	srv   *httptest.Server
	repo  *repo.Repo
	db    *sql.DB
	store *mapping.Store
	mpath string
}

func newRVEnv(t *testing.T) *rvEnv {
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
	h.SetAuth(api.NewAuth("s3cret"), false)
	h.SetFilmsEnabled(true)
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	if err := os.WriteFile(mpath, []byte(rvMappings), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatalf("load mappings: %v", err)
	}
	h.SetMetadataFields(store, cache.Noop{})
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return &rvEnv{srv: srv, repo: r, db: database, store: store, mpath: mpath}
}

func (e *rvEnv) video(t *testing.T, path string, extra ...model.ExtraMetadata) int64 {
	t.Helper()
	return e.titled(t, path, filepath.Base(path), extra...)
}

func (e *rvEnv) titled(t *testing.T, path, title string, extra ...model.ExtraMetadata) int64 {
	t.Helper()
	id, err := e.repo.UpsertVideo(context.Background(), &model.Video{
		FilePath: path, FileSize: 1, Title: title, Duration: 60,
		FileMtime: time.Unix(1_700_000_000, 0).UTC(),
	}, extra)
	if err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
	return id
}

// ids runs an expression through /media/ids. token "" reads as a visitor.
func (e *rvEnv) ids(t *testing.T, expr, token string) []int64 {
	t.Helper()
	code, body := getJSONTok(t, e.srv.URL+"/api/v1/media/ids?expr="+url.QueryEscape(expr), token)
	if code != http.StatusOK {
		t.Fatalf("%s: %d %v", expr, code, body)
	}
	got := numIDs(body["ids"])
	slices.Sort(got)
	return got
}

func (e *rvEnv) want(t *testing.T, expr string, want ...int64) {
	t.Helper()
	if got := e.ids(t, expr, "s3cret"); !slices.Equal(got, want) && (len(got) != 0 || len(want) != 0) {
		t.Errorf("%s = %v, want %v", expr, got, want)
	}
}

func tag(k, v string) model.ExtraMetadata { return model.ExtraMetadata{SourceKey: k, Value: v} }

// P0-6, R4, SEC-8: the winning value matches, wherever it came from; the losing one never.
func TestFieldConditionMatchesWinningValueOnly(t *testing.T) {
	e := newRVEnv(t)
	ctx := context.Background()
	v1 := e.titled(t, "/m/1.mkv", "The Alien Story")
	v2 := e.titled(t, "/m/2.mkv", "second")
	v3 := e.titled(t, "/m/3.mkv", "Bravo")
	v4 := e.titled(t, "/m/4.mkv", "Zulu")
	for _, id := range []int64{v2, v3} {
		if err := e.repo.UpsertEnrichment(ctx, "video", id, "fake", "x", map[string][]string{"title": {"Alien"}}); err != nil {
			t.Fatal(err)
		}
	}

	e.want(t, `title:alien`, v1) // the file wins with no decision; the provider's value loses
	e.want(t, `title:bravo`, v3)

	// The owner chooses a provider: the next read follows, and the file's value now loses.
	for _, id := range []int64{v2, v3} {
		if err := e.repo.SetDecision(ctx, "video", id, "title", "provider:fake", ""); err != nil {
			t.Fatal(err)
		}
	}
	e.want(t, `title:alien`, v1, v2, v3)
	e.want(t, `title:bravo`)

	// And back to the file: the provider's value never matches.
	if err := e.repo.SetDecision(ctx, "video", v3, "title", "file", ""); err != nil {
		t.Fatal(err)
	}
	e.want(t, `title:alien`, v1, v2)
	e.want(t, `title:bravo`, v3)

	// Text contains the words, in any case; a NOT includes a video with no match.
	e.want(t, `title:"STORY the"`, v1)
	e.want(t, `-title:alien`, v3, v4)
	e.want(t, `title:%`) // a LIKE wildcard is a literal

	// A merged list holds every source's value: from the file alone, or a provider alone.
	d1 := e.video(t, "/m/d1.mkv", tag("Director", "Ann Lee"))
	d2 := e.video(t, "/m/d2.mkv")
	if err := e.repo.UpsertEnrichment(ctx, "video", d2, "fake", "x", map[string][]string{"director": {"Ann Lee"}}); err != nil {
		t.Fatal(err)
	}
	e.want(t, `director:"ann lee"`, d1, d2)
	e.want(t, `director:ann`)

	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM resolved_values WHERE field_key LIKE '\_%' ESCAPE '\'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("internal keys indexed: %d, %v", n, err)
	}
}

// A list matches one whole item, a number compares, and conditions compose.
func TestFieldConditionKinds(t *testing.T) {
	e := newRVEnv(t)
	a := e.video(t, "/m/a.mkv", tag("Cast", "Sigourney Weaver"), tag("Cast", "Tom Skerritt"), tag("Runtime", "117"))
	b := e.video(t, "/m/b.mkv", tag("Cast", "Weaver"), tag("Runtime", "90"))
	c := e.video(t, "/m/c.mkv", tag("Runtime", "unknown"))

	e.want(t, `actors:"sigourney weaver"`, a)
	e.want(t, `actors:weaver`, b)
	e.want(t, `actors:sigourney`)
	e.want(t, `runtime:>100`, a)
	e.want(t, `runtime:<100`, b) // "unknown" isn't zero
	e.want(t, `runtime:90..117`, a, b)
	e.want(t, `runtime:>100 OR actors:weaver`, a, b)
	e.want(t, `-runtime:>100 -actors:weaver`, c)

	// Count, order and offset agree with the id list.
	code, body := getJSONTok(t, e.srv.URL+"/api/v1/media?limit=1&offset=1&sort=title_asc&expr="+url.QueryEscape(`runtime:90..117`), "s3cret")
	if code != http.StatusOK {
		t.Fatalf("media: %d %v", code, body)
	}
	if body["total"].(float64) != 2 {
		t.Errorf("total = %v", body["total"])
	}
}

// P0-1: released reads the winning date; a video with none only matches the negation.
func TestReleasedCondition(t *testing.T) {
	e := newRVEnv(t)
	y := time.Now().Year()
	recent := e.video(t, "/m/r.mkv", tag("Year", time.Now().AddDate(0, -1, 0).Format(time.DateOnly)))
	old := e.video(t, "/m/o.mkv", tag("Year", "1999"))
	none := e.video(t, "/m/n.mkv")
	junk := e.video(t, "/m/j.mkv", tag("Year", "soon"))

	e.want(t, `released:<1y`, recent)
	e.want(t, `-released:<1y`, old, none, junk)
	e.want(t, `released:1999`, old)
	e.want(t, `released:<2000`, old)
	if time.Now().Month() > 1 {
		e.want(t, `released:`+itoa(int64(y)), recent)
	}
}

// R2: each write path shows in the very next read, with no explicit drain.
func TestStaleReadAfterEachWrite(t *testing.T) {
	e := newRVEnv(t)
	ctx := context.Background()
	v := e.titled(t, "/m/v.mkv", "Anna")
	e.want(t, `title:anna`, v)

	steps := []struct {
		name  string
		write func() error
		expr  string
	}{
		{"enrichment joins a merged list", func() error {
			return e.repo.UpsertEnrichment(ctx, "video", v, "fake", "x", map[string][]string{"title": {"Cyan"}, "director": {"Ann Lee"}})
		}, `director:"ann lee"`},
		{"decision for the provider", func() error { return e.repo.SetDecision(ctx, "video", v, "title", "provider:fake", "") }, `title:cyan`},
		{"decision cleared", func() error { _, err := e.repo.ClearDecision(ctx, "video", v, "title"); return err }, `title:anna`},
		{"custom value", func() error { return e.repo.SetDecision(ctx, "video", v, "title", "manual", "Dusk") }, `title:dusk`},
		{"curation adds to a list", func() error { return e.repo.SetCuration(ctx, "video", v, "actors", "Gil Tan", "add") }, `actors:"gil tan"`},
		{"rescan changes the file's value", func() error {
			if _, err := e.repo.ClearDecision(ctx, "video", v, "title"); err != nil {
				return err
			}
			if _, err := e.db.Exec(`DELETE FROM entity_enrichment WHERE entity_id = ?`, v); err != nil {
				return err
			}
			e.titled(t, "/m/v.mkv", "Echo", tag("Cast", "Hal Roy"))
			return nil
		}, `title:echo actors:"hal roy"`},
	}
	for _, s := range steps {
		if err := s.write(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if got := e.ids(t, s.expr, "s3cret"); !slices.Equal(got, []int64{v}) {
			t.Errorf("after %s: %s = %v", s.name, s.expr, got)
		}
	}
}

// R5: a visitor's resolved query drains; one with no resolved condition leaves the set.
func TestVisitorDrainException(t *testing.T) {
	e := newRVEnv(t)
	v := e.video(t, "/m/v.mkv", tag("Director", "Ann Lee"))
	dirty := func() int {
		var n int
		if err := e.db.QueryRow(`SELECT COUNT(*) FROM completeness_dirty WHERE entity_type = 'video'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := dirty()
	if before == 0 {
		t.Fatal("seed left nothing dirty")
	}
	e.ids(t, `alien`, "")
	if got := dirty(); got != before {
		t.Errorf("a plain visitor query drained: %d, want %d", got, before)
	}
	if got := e.ids(t, `director:"Ann Lee"`, ""); !slices.Equal(got, []int64{v}) {
		t.Errorf("visitor director = %v", got)
	}
	if got := dirty(); got != 0 {
		t.Errorf("a resolved visitor query left %d dirty", got)
	}
}

// A6, SEC-8: a hidden field is refused for a visitor, never offered, and can't be public.
func TestHiddenFieldForVisitors(t *testing.T) {
	e := newRVEnv(t)
	v := e.video(t, "/m/v.mkv", tag("Format", "matroska"), tag("Director", "Ann Lee"))
	base := e.srv.URL + "/api/v1"

	e.want(t, `container:matroska`, v)
	if code, _ := getJSONTok(t, base+"/media/ids?expr="+url.QueryEscape(`container:matroska`), ""); code != http.StatusUnauthorized {
		t.Errorf("visitor hidden field: %d", code)
	}
	if code, _ := getJSONTok(t, base+"/query/parse?expr="+url.QueryEscape(`-container:matroska`), ""); code != http.StatusUnauthorized {
		t.Errorf("visitor parse: %d", code)
	}

	names := func(token string) []string {
		_, body := getJSONTok(t, base+"/query/fields", token)
		var out []string
		for _, f := range body["fields"].([]any) {
			out = append(out, f.(map[string]any)["field"].(string))
		}
		return out
	}
	if slices.Contains(names(""), "container") {
		t.Error("visitor offered a hidden field")
	}
	if got := names("s3cret"); !slices.Contains(got, "container") || !slices.Contains(got, "director") {
		t.Errorf("owner fields = %v", got)
	}

	values := func(field, token string) []any {
		_, body := getJSONTok(t, base+"/query/values?field="+field+"&q=", token)
		return body["values"].([]any)
	}
	if got := values("container", ""); len(got) != 0 {
		t.Errorf("visitor values of a hidden field = %v", got)
	}
	if got := values("container", "s3cret"); len(got) != 1 {
		t.Errorf("owner values = %v", got)
	}
	if got := values("director", ""); len(got) != 1 || got[0] != "Ann Lee" {
		t.Errorf("visitor values = %v", got)
	}
	if got := values("nope", "s3cret"); len(got) != 0 {
		t.Errorf("unknown field values = %v", got)
	}

	// The chip gets the key from the field's visibility.
	_, parsed := getJSONTok(t, base+"/query/parse?expr="+url.QueryEscape(`container:matroska`), "s3cret")
	if tree := parsed["tree"].(map[string]any); tree["owner_only"] != true || tree["field_label"] != "Container" {
		t.Errorf("display = %v", tree)
	}

	id := namedSmart(t, base, "Hidden", `container:matroska`, nil)
	code, body := patchTok(t, base+"/playlists/"+itoa(id), "s3cret", map[string]any{"visibility": "public"})
	if code != http.StatusBadRequest || body["owner_only"] != true {
		t.Errorf("going public: %d %v", code, body)
	}
	namedSmart(t, base, "Plain", `director:"Ann Lee"`, map[string]any{"visibility": "public"})
}

// Suggestions never name a value only a video browse hides holds (SEC-8).
func TestFieldValuesSkipHiddenVideos(t *testing.T) {
	e := newRVEnv(t)
	gone := e.video(t, "/m/gone.mkv", tag("Director", "Del Moss"))
	film := e.video(t, "/m/film.mkv", tag("Director", "Fay Lund"))
	e.video(t, "/m/keep.mkv", tag("Director", "Ann Lee"))
	e.want(t, `director:"ann lee" OR director:"del moss" OR director:"fay lund"`, gone, film, film+1)

	res, err := e.db.Exec(`INSERT INTO films (name) VALUES ('A film')`)
	if err != nil {
		t.Fatal(err)
	}
	filmID, _ := res.LastInsertId()
	if _, err := e.db.Exec(`INSERT INTO film_videos (film_id, video_id, is_full_film, created_at) VALUES (?, ?, 1, '2026-01-01T00:00:00Z')`, filmID, film); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE videos SET deleted_at = '2026-01-01T00:00:00Z' WHERE id = ?`, gone); err != nil {
		t.Fatal(err)
	}
	_, body := getJSONTok(t, e.srv.URL+"/api/v1/query/values?field=director&q=", "")
	if got := body["values"].([]any); len(got) != 1 || got[0] != "Ann Lee" {
		t.Errorf("values = %v, want only Ann Lee", got)
	}
}

// A10, P0-6: a field removed from the mapping makes a playlist stale, naming it, and the
// editor can still read the definition.
func TestFieldRemovedFromMapping(t *testing.T) {
	e := newRVEnv(t)
	v := e.video(t, "/m/v.mkv", tag("Director", "Ann Lee"))
	base := e.srv.URL + "/api/v1"
	id := namedSmart(t, base, "Directed", `director:"Ann Lee"`, nil)
	if got, _ := plIDs(t, base, id, "s3cret"); !slices.Equal(got, []int64{v}) {
		t.Fatalf("before = %v", got)
	}

	const without = "fields:\n  - canonical: title\n    label: Title\n    sources: [file:title]\n"
	if err := os.WriteFile(e.mpath, []byte(without), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.store.Reload(); err != nil {
		t.Fatal(err)
	}

	got, body := plIDs(t, base, id, "s3cret")
	if len(got) != 0 {
		t.Errorf("stale playlist returned %v", got)
	}
	refs, _ := body["stale_refs"].([]any)
	if len(refs) != 1 || refs[0].(map[string]any)["kind"] != "unknown_key" || refs[0].(map[string]any)["key"] != "director" {
		t.Errorf("stale_refs = %v", body["stale_refs"])
	}
	_, parsed := getJSONTok(t, base+"/query/parse?stored=1&expr="+url.QueryEscape(`director:"Ann Lee"`), "s3cret")
	if tree := parsed["tree"].(map[string]any); tree["missing"] != true || tree["expr"] != `director:"Ann Lee"` {
		t.Errorf("stored display = %v", tree)
	}
	// Typed fresh, it's the ordinary unknown field.
	if code, _ := getJSONTok(t, base+"/media/ids?expr="+url.QueryEscape(`director:x`), "s3cret"); code != http.StatusBadRequest {
		t.Errorf("unknown field: %d", code)
	}
}

// RD2: changed_at moves on a real change and only then. A first fingerprint moves nothing.
func TestChangeStamp(t *testing.T) {
	e := newRVEnv(t)
	ctx := context.Background()
	v := e.video(t, "/m/v.mkv", tag("Director", "Ann Lee"))
	other := e.video(t, "/m/o.mkv", tag("Director", "Bea Ford"))
	const long = "2020-01-01T00:00:00Z"
	if _, err := e.db.Exec(`UPDATE videos SET changed_at = ?`, long); err != nil {
		t.Fatal(err)
	}
	stamp := func(id int64) string {
		e.ids(t, `updated:<1d`, "s3cret") // drains
		var s string
		if err := e.db.QueryRow(`SELECT changed_at FROM videos WHERE id = ?`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	reset := func() {
		if _, err := e.db.Exec(`UPDATE videos SET changed_at = ? WHERE id = ?`, long, v); err != nil {
			t.Fatal(err)
		}
	}
	if got := stamp(v); got != long {
		t.Fatalf("the first fingerprint moved changed_at to %s", got)
	}

	// No change: an unchanged rescan that rewrites its rows, and an identical refresh.
	e.video(t, "/m/v.mkv", tag("Director", "Ann Lee"))
	if err := e.repo.UpsertEnrichment(ctx, "video", v, "fake", "x", map[string][]string{"director": {"Ann Lee"}}); err != nil {
		t.Fatal(err)
	}
	reset()
	stamp(v)
	reset()
	if err := e.repo.UpsertEnrichment(ctx, "video", v, "fake", "x", map[string][]string{"director": {"Ann Lee"}}); err != nil {
		t.Fatal(err)
	}
	e.video(t, "/m/v.mkv", tag("Director", "Ann Lee"))
	if got := stamp(v); got != long {
		t.Errorf("an identical rescan and refresh moved changed_at to %s", got)
	}

	// Reading more out of an unchanged file refreshes the index, and is no change (F12.7).
	e.video(t, "/m/v.mkv", tag("Director", "Ann Lee"), tag("Cast", "Hal Roy"))
	if got := stamp(v); got != long {
		t.Errorf("a new tag from an unchanged file moved changed_at to %s", got)
	}
	e.want(t, `actors:"hal roy"`, v)

	moved := func(name string, write func() error) {
		t.Helper()
		reset()
		if err := write(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := stamp(v); got == long {
			t.Errorf("%s didn't move changed_at", name)
		}
	}
	moved("a Tag", func() error {
		if _, err := e.db.Exec(`INSERT INTO tags (name) VALUES ('noir')`); err != nil {
			return err
		}
		_, err := e.db.Exec(`INSERT INTO video_tags (video_id, tag_id) SELECT ?, id FROM tags WHERE name = 'noir'`, v)
		return err
	})
	moved("a decision", func() error { return e.repo.SetDecision(ctx, "video", v, "title", "file", "") })
	moved("a losing provider value", func() error {
		return e.repo.UpsertEnrichment(ctx, "video", v, "fake", "x", map[string][]string{"title": {"Losing"}})
	})
	moved("a changed file", func() error {
		_, err := e.repo.UpsertVideo(ctx, &model.Video{FilePath: "/m/v.mkv", FileSize: 2, Title: "v", Duration: 60,
			FileMtime: time.Unix(1_700_000_100, 0).UTC()}, []model.ExtraMetadata{tag("Director", "Ann Lee")})
		return err
	})

	if got := stamp(other); got != long {
		t.Errorf("another video moved to %s", got)
	}
	e.want(t, `updated:<1d`, v)
	e.want(t, `updated:>1d`, other)

	// The sort follows, a visitor's included.
	code, body := getJSONTok(t, e.srv.URL+"/api/v1/media/ids?sort=updated_desc", "")
	if ids := numIDs(body["ids"]); code != http.StatusOK || len(ids) != 2 || ids[0] != v {
		t.Errorf("updated_desc = %d %v", code, ids)
	}
}

// The MCP tool has no drain, so it refuses what would read stale values.
func TestExprFieldsCatalogueIsOptional(t *testing.T) {
	e := newRVEnv(t)
	if _, err := api.ParseExpr(context.Background(), e.repo, `director:x`, nil, true); err == nil {
		t.Error("no catalogue: a mapped field parsed")
	}
	q, err := api.ParseExpr(context.Background(), e.repo, `updated:<1d`, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !query.NeedsIndex(q.Root) {
		t.Error("updated: doesn't report needing the index")
	}
}

// R3: every event that changes what a field resolves to marks the whole library dirty, so
// the next resolved query reflects it. The mapping's sources stand in for any change of
// precedence; a moved boot fingerprint is cmd/holodex's completeness_boot_test.
func TestMarkAllEventsReachResolvedQueries(t *testing.T) {
	e := newRVEnv(t)
	a := e.video(t, "/m/a.mkv", tag("Director", "Ann Lee"), tag("Other", "Del Moss"))
	b := e.video(t, "/m/b.mkv", tag("Director", "Ann Lee"), tag("Other", "Fay Lund"))
	e.want(t, `director:"Ann Lee"`, a, b)
	dirty := func() (n int) {
		t.Helper()
		if err := e.db.QueryRow(`SELECT COUNT(*) FROM completeness_dirty WHERE entity_type = 'video'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := dirty(); n != 0 {
		t.Fatalf("%d dirty after a drain", n)
	}

	const now = "'2026-01-01T00:00:00Z'"
	for _, event := range []struct{ name, write string }{
		{"promotion", `INSERT INTO field_promotions (entity_type, field_key, created_at, updated_at) VALUES ('video', 'height', ` + now + `, ` + now + `)`},
		{"claim", `INSERT INTO field_claims (entity_type, provider, field_key, canonical, created_at, updated_at) VALUES ('video', 'fake', 'hq', 'country', ` + now + `, ` + now + `)`},
		{"hint", `INSERT INTO provider_field_hints (provider, field_key, updated_at) VALUES ('fake', 'height', ` + now + `)`},
	} {
		if _, err := e.db.Exec(event.write); err != nil {
			t.Fatalf("%s: %v", event.name, err)
		}
		if n := dirty(); n != 2 {
			t.Errorf("%s left %d videos dirty, want all 2", event.name, n)
		}
		e.want(t, `director:"Ann Lee"`, a, b) // drains
	}

	// A mapping reload: the field now reads another tag, and the query follows at once.
	yaml := strings.Replace(rvMappings, "sources: [fake:director, file:Director]", "sources: [fake:director, file:Other]", 1)
	if yaml == rvMappings {
		t.Fatal("the mapping edit changed nothing")
	}
	if err := os.WriteFile(e.mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	code, resp := reqTokBody(t, http.MethodPost, e.srv.URL+"/api/v1/admin/reload-config", "s3cret", nil)
	if code != http.StatusOK {
		t.Fatalf("reload: %d %v", code, resp)
	}
	if n := dirty(); n != 2 {
		t.Errorf("reload left %d videos dirty, want all 2", n)
	}
	e.want(t, `director:"Ann Lee"`)
	e.want(t, `director:"Del Moss"`, a)
	e.want(t, `director:"Fay Lund"`, b)
}
