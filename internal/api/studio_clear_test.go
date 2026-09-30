package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// Clearing a studio (F74, ADR-120): the decision API, the writeback request and
// the film-studio cascade, driven end to end through a real queue whose writer
// records the tag deletes instead of touching a file.

type clearEnv struct {
	srv *httptest.Server
	r   *repo.Repo

	mu      sync.Mutex
	written map[string][]writeback.FieldWrite // file path → the batch written to it
}

func (e *clearEnv) writes(path string) []writeback.FieldWrite {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.written[path]
}

func (e *clearEnv) drain(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if n, _ := e.r.PendingWritebackCount(context.Background()); n == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("writeback queue did not drain")
}

// studioClearServer: studio reads Publisher then Label (two file tags), a live
// queue with the clear-sources hook wired as cmd/holodex does, films enabled.
func studioClearServer(t *testing.T) *clearEnv {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [tmdb:title, file:title]\n" +
		"  - canonical: studio\n    label: Studio\n    sources: [file:Publisher, file:Label, tmdb:studio]\n" +
		"  - canonical: overview\n    label: Overview\n    sources: [file:Comment]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}

	env := &clearEnv{r: r, written: map[string][]writeback.FieldWrite{}}
	write := func(_ context.Context, path string, fs []writeback.FieldWrite) error {
		env.mu.Lock()
		defer env.mu.Unlock()
		env.written[path] = fs
		return nil
	}
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	h.SetFilmsEnabled(true)
	h.SetAuth(api.NewAuth(""), false)

	q := writequeue.New(r, write, log, 1, "")
	// The pre-write snapshot read fails on these fake paths; that's logged and
	// non-fatal by design, so the recorded write still happens.
	q.SetClearSources(func(c string) []string {
		f, ok := store.Current().ByCanonical(c)
		if !ok {
			return nil
		}
		return f.FileTagSources()
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	q.Start(ctx)
	h.SetWriteQueue(q)

	env.srv = httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(env.srv.Close)
	return env
}

// seedTaggedStudioVideo seeds a Matroska video whose file carries studio under tag,
// and links it the way a scan would.
func seedTaggedStudioVideo(t *testing.T, r *repo.Repo, path, title, tag, studio string) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: path, FileSize: 1, Title: title, Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, []model.ExtraMetadata{{SourceKey: tag, Value: studio}})
	if err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
	if err := r.ReconcileVideoStudios(ctx, id, []string{studio}, nil); err != nil {
		t.Fatalf("link %s: %v", path, err)
	}
	return id
}

func studiosOf(t *testing.T, r *repo.Repo, id int64) []string {
	t.Helper()
	m, err := r.StudiosForVideos(context.Background(), []int64{id})
	if err != nil {
		t.Fatalf("studios: %v", err)
	}
	var out []string
	for _, s := range m[id] {
		out = append(out, s.Name)
	}
	return out
}

// R1: only an explicit clear on an allowlisted field stores a cleared decision.
func TestStudioClear_DecisionValidation(t *testing.T) {
	env := studioClearServer(t)
	id := seedTaggedStudioVideo(t, env.r, "/m/a.mkv", "A", "Publisher", "Acme")
	url := func(c string) string { return env.srv.URL + "/api/v1/media/" + itoa(id) + "/fields/" + c + "/decision" }

	bad := []struct {
		name, canonical string
		body            map[string]any
	}{
		{"clear with a value", "studio", map[string]any{"source": "manual", "clear": true, "manual_value": "x"}},
		{"clear with file", "studio", map[string]any{"source": "file", "clear": true}},
		{"clear with provider", "studio", map[string]any{"source": "provider:tmdb", "clear": true}},
		{"clear on title", "title", map[string]any{"source": "manual", "clear": true}},
		{"clear on overview", "overview", map[string]any{"source": "manual", "clear": true}},
		{"bare empty manual (guard kept)", "studio", map[string]any{"source": "manual", "manual_value": ""}},
		{"studio_id on the media PUT", "studio", map[string]any{"source": "manual", "clear": true, "studio_id": 1}},
	}
	for _, c := range bad {
		if code := sendDecision(t, http.MethodPut, url(c.canonical), "", c.body); code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", c.name, code)
		}
	}
	if got := studiosOf(t, env.r, id); !slices.Equal(got, []string{"Acme"}) {
		t.Fatalf("refused requests must change nothing, studios = %v", got)
	}
}

// R1–R2, D4: a clear unlinks at once, keeps a standing decision the owner can
// see and write, survives a re-scan, and DELETE undoes it.
func TestStudioClear_MediaClearLifecycle(t *testing.T) {
	env := studioClearServer(t)
	id := seedTaggedStudioVideo(t, env.r, "/m/a.mkv", "A", "Label", "Acme") // the mis-parse sits under Label
	url := env.srv.URL + "/api/v1/media/" + itoa(id) + "/fields/studio/decision"

	if code := sendDecision(t, http.MethodPut, url, "", map[string]any{"source": "manual", "clear": true}); code != http.StatusNoContent {
		t.Fatalf("clear: want 204, got %d", code)
	}
	if got := studiosOf(t, env.r, id); len(got) != 0 {
		t.Fatalf("studios after clear = %v, want none", got)
	}
	f := resolvedField(t, env.srv, id, "studio")
	dec, _ := f["decision"].(map[string]any)
	if dec["source"] != "manual" || dec["standing"] != true || dec["manual_value"] != nil {
		t.Fatalf("decision = %v, want standing manual with no manual_value", dec)
	}
	if vals, _ := f["values"].([]any); len(vals) != 0 {
		t.Fatalf("values = %v, want none", vals)
	}
	if f["in_sync"] != false {
		t.Fatalf("in_sync = %v, want false (the file still says Acme)", f["in_sync"])
	}
	if f["write_target"] != "Publisher" {
		t.Fatalf("write_target = %v, want Publisher — the cockpit must be able to write a clear", f["write_target"])
	}

	// A re-scan of the same file leaves it cleared: the standing decision wins.
	if _, err := env.r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A", Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, []model.ExtraMetadata{{SourceKey: "Label", Value: "Acme"}}); err != nil {
		t.Fatal(err)
	}
	if vals, _ := resolvedField(t, env.srv, id, "studio")["values"].([]any); len(vals) != 0 {
		t.Fatalf("re-scan brought the studio back: %v", vals)
	}

	// DELETE is the undo: back to the file value, relinked.
	if code := sendDecision(t, http.MethodDelete, url, "", nil); code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", code)
	}
	if got := studiosOf(t, env.r, id); !slices.Equal(got, []string{"Acme"}) {
		t.Fatalf("studios after undo = %v, want [Acme]", got)
	}
}

// A blank pin (a file pin to an empty layer) is NOT a clear: it keeps the
// unwritable stamp (ADR-113), so a clear can't leak onto other empty decisions.
func TestStudioClear_BlankPinStaysUnwritable(t *testing.T) {
	env := studioClearServer(t)
	ctx := context.Background()
	id, err := env.r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/empty.mkv", FileSize: 1, Title: "Empty", Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	url := env.srv.URL + "/api/v1/media/" + itoa(id) + "/fields/studio/decision"
	if code := sendDecision(t, http.MethodPut, url, "", map[string]any{"source": "file"}); code != http.StatusNoContent {
		t.Fatalf("blank pin: want 204, got %d", code)
	}
	if wt := resolvedField(t, env.srv, id, "studio")["write_target"]; wt != "" && wt != nil {
		t.Fatalf("blank pin write_target = %v, want none", wt)
	}
}

// R3: the composite-key gate runs for a clear, and override stores it.
func TestStudioClear_CollisionGate(t *testing.T) {
	env := studioClearServer(t)
	ctx := context.Background()
	// Same title/people/date; one already has no studio.
	noStudio, err := env.r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/none.mkv", FileSize: 1, Title: "Same", Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	withStudio := seedTaggedStudioVideo(t, env.r, "/m/acme.mkv", "Same", "Publisher", "Acme")
	url := env.srv.URL + "/api/v1/media/" + itoa(withStudio) + "/fields/studio/decision"

	code, body := putDecisionRaw(t, url, map[string]any{"source": "manual", "clear": true})
	if code != http.StatusConflict {
		t.Fatalf("clear into a collision: want 409, got %d", code)
	}
	if c, _ := body["conflict"].(map[string]any); c == nil || int64(c["id"].(float64)) != noStudio {
		t.Fatalf("conflict = %v, want video #%d", body["conflict"], noStudio)
	}
	if got := studiosOf(t, env.r, withStudio); !slices.Equal(got, []string{"Acme"}) {
		t.Fatalf("a refused clear must not persist, studios = %v", got)
	}
	if code := sendDecision(t, http.MethodPut, url, "", map[string]any{"source": "manual", "clear": true, "override": true}); code != http.StatusNoContent {
		t.Fatalf("override: want 204, got %d", code)
	}
	if got := studiosOf(t, env.r, withStudio); len(got) != 0 {
		t.Fatalf("override clear should unlink, studios = %v", got)
	}
}

// R7: an HTTP clear is accepted only against a standing cleared decision, and
// deletes both of studio's file tags.
func TestStudioClear_WritebackRequest(t *testing.T) {
	env := studioClearServer(t)
	id := seedTaggedStudioVideo(t, env.r, "/m/a.mkv", "A", "Label", "Acme")
	wb := env.srv.URL + "/api/v1/media/" + itoa(id) + "/writeback"
	clearEntry := map[string]any{"fields": []map[string]any{{"field": "studio", "clear": true}}}

	// Undecided: nothing to write as a clear.
	if code, _ := rawRequest(t, http.MethodPost, wb, clearEntry); code != http.StatusBadRequest {
		t.Fatalf("clear with no cleared decision: want 400, got %d", code)
	}
	// A manual:"Other" decision is not a clear either.
	dec := env.srv.URL + "/api/v1/media/" + itoa(id) + "/fields/studio/decision"
	sendDecision(t, http.MethodPut, dec, "", map[string]any{"source": "manual", "manual_value": "Other"})
	if code, _ := rawRequest(t, http.MethodPost, wb, clearEntry); code != http.StatusBadRequest {
		t.Fatalf("clear against manual:Other: want 400, got %d", code)
	}
	// Off-allowlist and clear-with-values are refused outright.
	for _, body := range []map[string]any{
		{"fields": []map[string]any{{"field": "title", "clear": true}}},
		{"fields": []map[string]any{{"field": "studio", "clear": true, "values": []string{"x"}}}},
	} {
		if code, _ := rawRequest(t, http.MethodPost, wb, body); code != http.StatusBadRequest {
			t.Fatalf("%v: want 400, got %d", body, code)
		}
	}

	sendDecision(t, http.MethodPut, dec, "", map[string]any{"source": "manual", "clear": true})
	// Clearing a field and writing it a value in one batch is refused.
	for _, body := range []map[string]any{
		{"fields": []map[string]any{{"field": "studio", "clear": true}, {"field": "studio", "values": []string{"X"}}}},
	} {
		if code, _ := rawRequest(t, http.MethodPost, wb, body); code != http.StatusBadRequest {
			t.Fatalf("%v: want 400, got %d", body, code)
		}
	}

	sendDecision(t, http.MethodPut, dec, "", map[string]any{"source": "manual", "clear": true})
	if code, _ := rawRequest(t, http.MethodPost, wb, clearEntry); code != http.StatusAccepted {
		t.Fatalf("clear write: want 202, got %d", code)
	}
	env.drain(t)
	var got []string
	for _, w := range env.writes("/m/a.mkv") {
		if !w.Delete {
			t.Fatalf("clear wrote a value: %+v", w)
		}
		got = append(got, w.TagName)
	}
	if !slices.Equal(got, []string{"Publisher", "Label"}) {
		t.Fatalf("deleted tags = %v, want [Publisher Label]", got)
	}
}

// R5, D6: a cascade clear touches only the film's videos carrying that studio,
// and enqueues a delete — not an empty-values job — for each.
func TestStudioClear_FilmCascadeScopedToStudio(t *testing.T) {
	env := studioClearServer(t)
	ctx := context.Background()
	filmID, err := env.r.CreateFilm(ctx, "Mixed", 2020)
	if err != nil {
		t.Fatal(err)
	}
	p1 := seedTaggedStudioVideo(t, env.r, "/m/p1.mkv", "Part One", "Publisher", "Acme")
	p2 := seedTaggedStudioVideo(t, env.r, "/m/p2.mkv", "Part Two", "Publisher", "Acme")
	p3 := seedTaggedStudioVideo(t, env.r, "/m/p3.mkv", "Part Three", "Publisher", "Beta")
	for _, v := range []int64{p1, p2, p3} {
		if _, err := env.r.AttachFilmVideo(ctx, filmID, v, nil, false); err != nil {
			t.Fatal(err)
		}
	}
	acme, err := env.r.StudiosForVideos(ctx, []int64{p1})
	if err != nil {
		t.Fatal(err)
	}
	acmeID := acme[p1][0].ID
	cascade := env.srv.URL + "/api/v1/films/" + itoa(filmID) + "/studio/cascade"

	// The pairing rule: clear needs a positive studio_id and studio_id needs
	// clear. A negative id must not slip through as an unscoped clear.
	for _, body := range []map[string]any{
		{"source": "manual", "clear": true},
		{"source": "manual", "clear": true, "studio_id": -1},
		{"source": "manual", "manual_value": "Acme", "studio_id": acmeID},
	} {
		if code, _ := rawRequest(t, http.MethodPost, cascade, body); code != http.StatusBadRequest {
			t.Fatalf("%v: want 400, got %d", body, code)
		}
	}

	code, body := rawRequest(t, http.MethodPost, cascade, map[string]any{"source": "manual", "clear": true, "studio_id": acmeID})
	if code != http.StatusAccepted {
		t.Fatalf("cascade clear: want 202, got %d (%v)", code, body)
	}
	results, _ := body["results"].([]any)
	var cleared []int64
	for _, raw := range results {
		row := raw.(map[string]any)
		if row["status"] != "enqueued" {
			t.Fatalf("unexpected result %v", row)
		}
		cleared = append(cleared, int64(row["video_id"].(float64)))
	}
	slices.Sort(cleared)
	if want := []int64{p1, p2}; !slices.Equal(cleared, want) {
		t.Fatalf("cleared videos = %v, want %v (part three carries Beta)", cleared, want)
	}

	env.drain(t)
	for _, p := range []string{"/m/p1.mkv", "/m/p2.mkv"} {
		ws := env.writes(p)
		if len(ws) == 0 || !ws[0].Delete || ws[0].TagName != "Publisher" {
			t.Fatalf("%s: want a Publisher delete, got %+v", p, ws)
		}
	}
	if ws := env.writes("/m/p3.mkv"); ws != nil {
		t.Fatalf("part three must be untouched, got %+v", ws)
	}
	if got := studiosOf(t, env.r, p3); !slices.Equal(got, []string{"Beta"}) {
		t.Fatalf("part three studios = %v, want [Beta]", got)
	}

	// A studio the film doesn't carry is a clean no-op.
	code, body = rawRequest(t, http.MethodPost, cascade, map[string]any{"source": "manual", "clear": true, "studio_id": 999999})
	if code != http.StatusAccepted || body["batch_id"] != "" {
		t.Fatalf("unknown studio_id: want 202 with no batch, got %d %v", code, body)
	}
}
