package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// partsFixture is the HOLODEX-389 triplet: three files of one media whose only
// part source is, respectively, a container tag, a filename candidate and a manual
// decision — plus one file with no part at all. Each source lives in a different
// table, and the list path historically loaded none of the first (video_metadata),
// so a card could only ever show a part that came through the filename or a decision.
type partsFixture struct {
	srv                                *httptest.Server
	r                                  *repo.Repo
	tagOnly, filenameOnly, decided, no int64
}

func partsServer(t *testing.T) partsFixture {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    browse: true\n    sources: [fake:title, file:title]\n" +
		"  - canonical: part\n    label: Part\n    sources: [PartNumber, DiskNumber, filename:part]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}
	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte("sources:\n  - name: fake\n    base_url: http://fake:9100\n    entity_types: [video]\n    enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	estore, err := enrich.NewStore(sp, log)
	if err != nil {
		t.Fatal(err)
	}
	svc := enrich.NewServiceWithClient(estore, r, log, func(enrich.Source) enrich.ProviderClient { return enrich.NewFake("fake") })

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	h.SetEnrichment(svc)
	h.SetFilmsEnabled(true)
	h.SetAuth(api.NewAuth("tok"), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	seed := func(path string, extra ...model.ExtraMetadata) int64 {
		id, err := r.UpsertVideo(ctx, &model.Video{
			FilePath: path, FileSize: 1, Title: "Live at Budokan",
			FileMtime: time.Now().UTC().Truncate(time.Second),
		}, extra)
		if err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
		linkPeopleAs(t, r, id, "actor", "Alice")
		return id
	}
	f := partsFixture{srv: srv, r: r}
	f.tagOnly = seed("/m/budokan-1.mkv", model.ExtraMetadata{SourceKey: "PartNumber", Value: "1"})
	f.filenameOnly = seed("/m/budokan {part-2}.mkv")
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, f.filenameOnly, "filename", "", map[string][]string{"part": {"2"}}); err != nil {
		t.Fatalf("seed filename candidate: %v", err)
	}
	f.decided = seed("/m/budokan-3.mkv")
	if err := r.SetDecision(ctx, model.EnrichEntityVideo, f.decided, "part", "manual", "3"); err != nil {
		t.Fatalf("seed decision: %v", err)
	}
	f.no = seed("/m/solo.mkv")
	return f
}

// partsByID reads {id → part} out of any list payload whose rows carry `id`/`part`.
func partsByID(t *testing.T, rows []any) map[int64]string {
	t.Helper()
	out := map[int64]string{}
	for _, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("row is not an object: %v", row)
		}
		id, _ := m["id"].(float64)
		part, _ := m["part"].(string)
		if _, present := m["part"]; present && part == "" {
			t.Errorf("row %v carries an empty part key; want omitted", m["id"])
		}
		out[int64(id)] = part
	}
	return out
}

func assertTriplet(t *testing.T, got map[int64]string, f partsFixture) {
	t.Helper()
	for id, want := range map[int64]string{f.tagOnly: "1", f.filenameOnly: "2", f.decided: "3", f.no: ""} {
		if got[id] != want {
			t.Errorf("video %d part = %q, want %q (all: %v)", id, got[id], want, got)
		}
	}
}

// TestListMedia_CarriesPart is the container-tag-only regression: the browse list
// passes no file layer to the resolver (applyBrowseTitles), so a part that exists
// only as PartNumber in video_metadata was invisible on the card.
func TestListMedia_CarriesPart(t *testing.T) {
	f := partsServer(t)
	code, body := getJSONTok(t, f.srv.URL+"/api/v1/media", "")
	if code != 200 {
		t.Fatalf("GET /media = %d", code)
	}
	items, _ := body["items"].([]any)
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}
	assertTriplet(t, partsByID(t, items), f)
}

// TestListSurfaces_CarryPart covers the list paths that never went through
// applyBrowseTitles at all — search and the entity page's video list — as a visitor,
// since the pill is visible to everyone (owner/visitor gating rule).
func TestListSurfaces_CarryPart(t *testing.T) {
	f := partsServer(t)
	code, body := getJSONTok(t, f.srv.URL+"/api/v1/search?q=Budokan", "")
	if code != 200 {
		t.Fatalf("GET /search = %d", code)
	}
	videos, _ := body["videos"].([]any)
	assertTriplet(t, partsByID(t, videos), f)

	pid, ok, err := f.r.PersonIDByName(context.Background(), "Alice")
	if err != nil || !ok {
		t.Fatalf("person id: %v (found=%v)", err, ok)
	}
	code, body = getJSONTok(t, f.srv.URL+"/api/v1/people/"+itoa(pid), "")
	if code != 200 {
		t.Fatalf("GET /people/{id} = %d", code)
	}
	items, _ := body["items"].([]any)
	assertTriplet(t, partsByID(t, items), f)
}

// TestGetMedia_VideoCarriesPart: the detail's `video` object is the same model.Video
// the lists stamp, so it carries `part` too — a client must not see the field flip
// present/absent depending on which endpoint produced the object.
func TestGetMedia_VideoCarriesPart(t *testing.T) {
	f := partsServer(t)
	for id, want := range map[int64]string{f.tagOnly: "1", f.decided: "3", f.no: ""} {
		code, body := getJSONTok(t, f.srv.URL+"/api/v1/media/"+itoa(id), "")
		if code != 200 {
			t.Fatalf("GET /media/%d = %d", id, code)
		}
		video, _ := body["video"].(map[string]any)
		if got, _ := video["part"].(string); got != want {
			t.Errorf("video %d detail part = %q, want %q", id, got, want)
		}
	}
}

// TestGetFilm_CarriesPartOnScenesAndFullFilms: unlike edition (full-film rows only),
// part is stamped on scene rows too — the scenes grid renders the same VideoCard as
// the browse grid, and that card's corner badge is the whole point of the field.
func TestGetFilm_CarriesPartOnScenesAndFullFilms(t *testing.T) {
	f := partsServer(t)
	ctx := context.Background()
	filmID, err := f.r.CreateFilm(ctx, "Budokan", 1978)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	if _, err := f.r.AttachFilmVideo(ctx, filmID, f.tagOnly, nil, false); err != nil {
		t.Fatalf("attach scene: %v", err)
	}
	if _, err := f.r.AttachFilmVideo(ctx, filmID, f.filenameOnly, nil, true); err != nil {
		t.Fatalf("attach full film: %v", err)
	}
	code, body := getJSONTok(t, f.srv.URL+"/api/v1/films/"+itoa(filmID), "")
	if code != 200 {
		t.Fatalf("GET /films/{id} = %d", code)
	}
	for _, tc := range []struct {
		key  string
		id   int64
		want string
	}{{"scenes", f.tagOnly, "1"}, {"full_films", f.filenameOnly, "2"}} {
		rows, _ := body[tc.key].([]any)
		if len(rows) != 1 {
			t.Fatalf("%s = %d rows, want 1", tc.key, len(rows))
		}
		video, _ := rows[0].(map[string]any)["video"].(map[string]any)
		if got, _ := video["part"].(string); got != tc.want {
			t.Errorf("%s[0].video.part = %q, want %q", tc.key, got, tc.want)
		}
	}
}

// TestQueues_CarryPart covers the two owner queue rows that name a video by title
// alone — the surface the design handoff calls most likely to be forgotten.
func TestQueues_CarryPart(t *testing.T) {
	f := partsServer(t)
	ctx := context.Background()

	code, body := getJSONTok(t, f.srv.URL+"/api/v1/owner/enrich-queue", "tok")
	if code != 200 {
		t.Fatalf("GET /owner/enrich-queue = %d", code)
	}
	rows, _ := body["rows"].([]any)
	got := map[int64]string{}
	for _, row := range rows {
		m := row.(map[string]any)
		if m["entity_type"] != model.EnrichEntityVideo {
			continue
		}
		id, _ := m["entity_id"].(float64)
		got[int64(id)], _ = m["part"].(string)
	}
	assertTriplet(t, got, f)

	for _, id := range []int64{f.tagOnly, f.no} {
		if err := f.r.UpsertExtractionReview(ctx, id, "title", "New", "Old", 0.4, 0); err != nil {
			t.Fatalf("seed review: %v", err)
		}
	}
	code, body = getJSONTok(t, f.srv.URL+"/api/v1/owner/extraction-queue", "tok")
	if code != 200 {
		t.Fatalf("GET /owner/extraction-queue = %d", code)
	}
	rows, _ = body["rows"].([]any)
	got = map[int64]string{}
	for _, row := range rows {
		m := row.(map[string]any)
		id, _ := m["video_id"].(float64)
		got[int64(id)], _ = m["part"].(string)
	}
	if got[f.tagOnly] != "1" || got[f.no] != "" {
		t.Errorf("extraction queue parts = %v, want %d→1 and %d→\"\"", got, f.tagOnly, f.no)
	}
}

// TestEnrichment_NeverWritesPart is the RD10 triplet invariance: the same provider
// record adopted on all three parts, with the provider's title decided as the
// precedence winner (ADR-051 — the file layer stays the default until a decision
// says otherwise), resolves the provider fields identically while each part is
// untouched — even though the provider's payload carries a `part` key, because the
// mapping has no provider source for it (the loader forbids one), so the stored
// row is inert.
func TestEnrichment_NeverWritesPart(t *testing.T) {
	f := partsServer(t)
	ctx := context.Background()
	for _, id := range []int64{f.tagOnly, f.filenameOnly, f.decided} {
		if err := f.r.UpsertEnrichment(ctx, model.EnrichEntityVideo, id, "fake", "ext-1", map[string][]string{
			"title": {"Cheap Trick at Budokan"},
			"part":  {"9"},
		}); err != nil {
			t.Fatalf("apply provider record: %v", err)
		}
		if err := f.r.SetDecision(ctx, model.EnrichEntityVideo, id, "title", "provider:fake", ""); err != nil {
			t.Fatalf("decide provider title: %v", err)
		}
	}
	_, body := getJSONTok(t, f.srv.URL+"/api/v1/media", "")
	items, _ := body["items"].([]any)
	assertTriplet(t, partsByID(t, items), f)
	for _, row := range items {
		m := row.(map[string]any)
		id, _ := m["id"].(float64)
		if int64(id) == f.no {
			continue
		}
		if m["title"] != "Cheap Trick at Budokan" {
			t.Errorf("video %v title = %q, want the provider title on every part", m["id"], m["title"])
		}
	}
}
