package api_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"holodex/internal/extract"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/thumbnail"
)

// F90 slice 1 (testing-strategy §42): has: over the drain's entity_field_state cache.

// fsMappings adds the poster, and maps director from a provider that is never declared.
var fsMappings = strings.Replace(rvMappings, "[fake:director, file:Director]", "[fake:director, gone:director, file:Director]", 1) +
	"  - canonical: poster_url\n    label: Poster\n    display: image_url\n    sources: [fake:poster_url]\n" +
	"  - canonical: studio\n    label: Studio\n    sources: [file:Publisher]\n"

func newFSEnv(t *testing.T) *rvEnv {
	t.Helper()
	e := newRVEnv(t)
	if err := os.WriteFile(e.mpath, []byte(fsMappings), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.store.Reload(); err != nil {
		t.Fatal(err)
	}
	e.repo.SetDeclaredProviders([]string{"fake"})
	return e
}

// holdSlot puts a local poster slot on disk and in video_poster_sources, and makes it the
// video's poster decision, as the chooser does.
func (e *rvEnv) holdSlot(t *testing.T, id int64, kind string) {
	t.Helper()
	slot, ok := thumbnail.SlotForKind(kind)
	if !ok {
		t.Fatalf("no slot for %s", kind)
	}
	thumbs := filepath.Join(filepath.Dir(e.mpath), "thumbnails")
	path := thumbnail.SlotPath(thumbs, id, slot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("jpeg-"+kind), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`INSERT INTO video_poster_sources (video_id, kind, hash, updated_at) VALUES (?, ?, ?, '2026-11-11T00:00:00Z')`,
		id, kind, thumbnail.ContentHash([]byte("jpeg-"+kind))); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.SetDecision(context.Background(), "video", id, "poster_url", "local:"+kind, ""); err != nil {
		t.Fatal(err)
	}
}

// §42 A3, P0-2: has: is what the page shows, from any source, and composes with the rest.
func TestHasMatchesWhatThePageShows(t *testing.T) {
	e := newFSEnv(t)
	ctx := context.Background()
	fileDir := e.video(t, "/m/fd.mkv", tag("Director", "Ann Lee"), tag("Cast", "Gil Tan"))
	provDir := e.video(t, "/m/pd.mkv")
	if err := e.repo.UpsertEnrichment(ctx, "video", provDir, "fake", "x", map[string][]string{"director": {"Ann Lee"}}); err != nil {
		t.Fatal(err)
	}
	dated := e.video(t, "/m/rd.mkv", tag("Year", "2001"))
	junk := e.video(t, "/m/jd.mkv", tag("Year", "sometime")) // displays, but isn't a date
	removed := e.video(t, "/m/rm.mkv", tag("Cast", "Hal Roy"))
	if err := e.repo.SetCuration(ctx, "video", removed, "actors", "Hal Roy", repo.CurationSuppress); err != nil {
		t.Fatal(err)
	}
	cleared := e.video(t, "/m/cl.mkv", tag("Year", "1999"))
	if err := e.repo.SetDecision(ctx, "video", cleared, "release_date", "manual", ""); err != nil {
		t.Fatal(err)
	}
	// A provider's director the owner removed is stored, so enriched, but not shown.
	hidden := e.video(t, "/m/hd.mkv")
	if err := e.repo.UpsertEnrichment(ctx, "video", hidden, "fake", "x", map[string][]string{"director": {"Del Moss"}}); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.SetCuration(ctx, "video", hidden, "director", "Del Moss", repo.CurationSuppress); err != nil {
		t.Fatal(err)
	}
	all := []int64{fileDir, provDir, dated, junk, removed, cleared, hidden}
	without := func(ids ...int64) []int64 {
		return slices.DeleteFunc(slices.Clone(all), func(id int64) bool { return slices.Contains(ids, id) })
	}

	e.want(t, `has:director`, fileDir, provDir)
	e.want(t, `-has:director`, without(fileDir, provDir)...)
	e.want(t, `has:actors`, fileDir) // the owner removed the only value of the other
	e.want(t, `has:released`, dated, junk)
	e.want(t, `has:release_date`, dated, junk)
	e.want(t, `released:2001`, dated) // every released: match also has the field
	e.want(t, `has:director -has:actors`, provDir)
	e.want(t, `has:actors OR has:released`, fileDir, dated, junk)
	e.want(t, `-(has:director OR has:released)`, removed, cleared, hidden)
	e.want(t, `HAS:Director`, fileDir, provDir) // field names ignore case

	// R4, RD4: not applicable keeps a video out of the queue, not out of -has:.
	if _, err := e.db.Exec(`INSERT INTO facet_not_applicable (entity_type, entity_id, canonical_field, created_at) VALUES ('video', ?, 'director', '2026-11-11T00:00:00Z')`, dated); err != nil {
		t.Fatal(err)
	}
	e.want(t, `-has:director`, without(fileDir, provDir)...)

	// The count and the page agree with the id list.
	code, body := getJSONTok(t, e.srv.URL+"/api/v1/media?limit=1&expr="+url.QueryEscape(`has:director`), "s3cret")
	if code != http.StatusOK || body["total"].(float64) != 2 {
		t.Errorf("media total: %d %v", code, body["total"])
	}
}

// P0-2: an automatic thumbnail fills the card but isn't a poster; an upload is.
func TestHasPosterIgnoresTheAutomaticThumbnail(t *testing.T) {
	e := newFSEnv(t)
	auto := e.video(t, "/m/auto.mkv")
	e.holdSlot(t, auto, "automatic")
	upload := e.video(t, "/m/up.mkv")
	e.holdSlot(t, upload, "upload")
	bare := e.video(t, "/m/bare.mkv")

	e.want(t, `has:poster`, upload)
	e.want(t, `-has:poster`, auto, bare)
}

// P0-2: only queryable fields, released and poster may be named; anything else is F87's
// unknown-field refusal.
func TestHasRefusesFieldsItCannotCheck(t *testing.T) {
	e := newFSEnv(t)
	for _, expr := range []string{`has:poster_url`, `has:_x`, `has:nonesuch`} {
		code, body := getJSONTok(t, e.srv.URL+"/api/v1/media/ids?expr="+url.QueryEscape(expr), "s3cret")
		if code != http.StatusBadRequest {
			t.Errorf("%s: %d %v, want 400", expr, code, body)
		}
	}
	if code, _ := getJSONTok(t, e.srv.URL+"/api/v1/media/ids?expr="+url.QueryEscape(`has:director`), ""); code != http.StatusUnauthorized {
		t.Errorf("visitor has: = %d, want 401", code)
	}
}

// §42 R1/R5 (the drain's half): what the cache holds for each entity type. enriched is a
// declared provider's stored data whether or not it won; filename, `_` keys and unmapped
// keys add nothing, and images are never enriched.
func TestDrainWritesFieldStateForEveryEntityType(t *testing.T) {
	e := newFSEnv(t)
	ctx := context.Background()
	v := e.video(t, "/m/v.mkv", tag("Director", "Ann Lee"))
	for provider, fields := range map[string]map[string][]string{
		"fake":           {"title": {"Lost"}, "_internal": {"x"}, "tagline": {"x"}},
		extract.Provider: {"director": {"Del Moss"}},
		"gone":           {"director": {"Del Moss"}}, // mapped, but not declared
	} {
		if err := e.repo.UpsertEnrichment(ctx, "video", v, provider, "x", fields); err != nil {
			t.Fatal(err)
		}
	}
	var person int64
	if err := e.db.QueryRow(`INSERT INTO people (name) VALUES ('Ann Lee') RETURNING id`).Scan(&person); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`INSERT INTO video_people (video_id, person_id, role) VALUES (?, ?, 'director')`, v, person); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.UpsertEnrichment(ctx, model.EnrichEntityPerson, person, "fake", "p1", map[string][]string{"bio": {"Born somewhere."}}); err != nil {
		t.Fatal(err)
	}
	e.want(t, `has:director`, v) // drains

	type st struct{ shown, enriched bool }
	state := func(entityType string, id int64) map[string]st {
		rows, err := e.db.Query(`SELECT field_key, shown, enriched FROM entity_field_state WHERE entity_type = ? AND entity_id = ?`, entityType, id)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]st{}
		for rows.Next() {
			var k string
			var s st
			if err := rows.Scan(&k, &s.shown, &s.enriched); err != nil {
				t.Fatal(err)
			}
			out[k] = s
		}
		return out
	}
	got := state("video", v)
	want := map[string]st{
		"title":    {shown: true, enriched: true}, // the provider's title lost to the file's
		"director": {shown: true},                 // filename's and an undeclared provider's never count
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("video %s = %+v, want %+v", k, got[k], w)
		}
	}
	for _, k := range []string{"_internal", "tagline", "poster", "poster_url"} {
		if _, ok := got[k]; ok {
			t.Errorf("video has a %s row: %+v", k, got[k])
		}
	}
	if p := state("person", person); p["bio"] != (st{true, true}) || p["name"] != (st{shown: true}) {
		t.Errorf("person state = %+v", p)
	}
	if _, ok := state("person", person)["photo"]; ok {
		t.Error("a person with no headshot has a photo row")
	}
}

// P0-2 (owner, 2026-11-12): a mapped field named like a built-in condition is still a
// field has: can check. studio: stays the built-in condition, and its values are never
// put in the value index.
func TestHasStudioWhileStudioStaysBuiltin(t *testing.T) {
	e := newFSEnv(t)
	with := e.video(t, "/m/s.mkv", tag("Publisher", "Acme"))
	without := e.video(t, "/m/n.mkv")
	e.want(t, `has:studio`, with)
	e.want(t, `-has:studio`, without)

	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM resolved_values WHERE field_key = 'studio'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("resolved_values holds %d studio rows", n)
	}
	// studio: names a studio entity, so an unknown name is refused as one, not compared as text.
	if code, body := getJSONTok(t, e.srv.URL+"/api/v1/media/ids?expr="+url.QueryEscape(`studio:Acme`), "s3cret"); code != http.StatusBadRequest {
		t.Errorf("studio:Acme with no such studio = %d %v, want 400", code, body)
	}
	code, body := getJSONTok(t, e.srv.URL+"/api/v1/query/fields", "s3cret")
	if code != http.StatusOK {
		t.Fatalf("query fields: %d", code)
	}
	for _, f := range body["fields"].([]any) {
		if f.(map[string]any)["field"] == "studio" {
			t.Error("studio is offered as a value field")
		}
	}
}
