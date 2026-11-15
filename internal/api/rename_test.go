package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/rename"
	"holodex/internal/repo"
	"holodex/internal/settings"
)

type renameFixture struct {
	srv   *httptest.Server
	r     *repo.Repo
	lib   string
	ids   map[string]int64
	kick  *kickCounter
	sqldb *sql.DB
}

// renameServer seeds a real temp library: each file exists on disk and as a video row whose title
// and studio come from its file tags.
func renameServer(t *testing.T, token string) *renameFixture {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [file:title]\n" +
		"  - canonical: studio\n    label: Studio\n    sources: [Publisher]\n" +
		"  - canonical: actors\n    label: Actors\n    sources: [Artist, Performer]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	h.SetAuth(api.NewAuth(token), false)
	h.SetFilmsEnabled(true)
	st := settings.New(r, rename.SettingKeys(h.RenameProviders)...)
	if err := st.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.SetSettings(st)
	kick := &kickCounter{}
	h.SetRenamer(kick)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	lib := filepath.Join(dir, "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	f := &renameFixture{srv: srv, r: r, lib: lib, ids: map[string]int64{}, kick: kick, sqldb: database}
	seed := func(file, title, studio string) {
		path := filepath.Join(lib, file)
		if err := os.WriteFile(path, []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
		var extra []model.ExtraMetadata
		if studio != "" {
			extra = append(extra, model.ExtraMetadata{SourceKey: "Publisher", Value: studio})
		}
		id, err := r.UpsertVideo(context.Background(), &model.Video{FilePath: path, Title: title,
			FileMtime: time.Now().UTC().Truncate(time.Second)}, extra)
		if err != nil {
			t.Fatal(err)
		}
		f.ids[file] = id
	}
	seed("junk1.mkv", "Ada Lovelace", "Acme")
	seed("junk2.mkv", "Bob", "")
	seed("[Acme] Cy.mkv", "Cy", "Acme")
	seed("junk3.mkv", "Ada - Redux", "Acme")
	seed("junk4.mkv", "Taken", "")
	if err := os.WriteFile(filepath.Join(lib, "Taken.mkv"), []byte("untracked"), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func doJSON(t *testing.T, method, url, token string, body any) (int, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rd = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, url, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (f *renameFixture) put(t *testing.T, key, value string) (int, map[string]any) {
	return doJSON(t, http.MethodPut, f.srv.URL+"/api/v1/admin/settings/"+key, "secret", map[string]string{"value": value})
}

// A1: every rename and settings route refuses a visitor. A new route must join this table.
func TestRenameRoutesOwnerGated(t *testing.T) {
	f := renameServer(t, "secret")
	id := itoa(f.ids["junk1.mkv"])
	for _, rt := range []struct{ method, path string }{
		{http.MethodGet, "/admin/settings"},
		{http.MethodPut, "/admin/settings/" + rename.KeyTemplates},
		{http.MethodGet, "/rename/plan"},
		{http.MethodPost, "/rename/plan/preview"},
		{http.MethodPut, "/media/" + id + "/name-lock"},
		{http.MethodGet, "/media/" + id + "/renames"},
		{http.MethodPost, "/rename/apply"},
		{http.MethodGet, "/media/" + id + "/rename/status"},
		{http.MethodPost, "/media/" + id + "/rename/undo"},
		{http.MethodPost, "/media/" + id + "/rename/retry"},
		{http.MethodPost, "/media/" + id + "/rename/dismiss"},
	} {
		if code, _ := doJSON(t, rt.method, f.srv.URL+"/api/v1"+rt.path, "", map[string]any{}); code != http.StatusUnauthorized {
			t.Errorf("visitor %s %s = %d, want 401", rt.method, rt.path, code)
		}
	}
}

// A3: templates are validated per row on the generic settings write; mode and settle window are on
// the allowlist and range-checked.
func TestRenameSettingsValidated(t *testing.T) {
	f := renameServer(t, "secret")
	code, body := f.put(t, rename.KeyTemplates, `["{title}", "{title} {title}", "{studio}"]`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid list = %d %v, want 422", code, body)
	}
	rows, _ := body["rows"].([]any)
	if len(rows) != 3 || rows[0] != "" || rows[1] == "" || rows[2] == "" {
		t.Errorf("per-row errors = %v", body["rows"])
	}
	if v, ok, _ := f.r.GetSetting(context.Background(), rename.KeyTemplates); ok {
		t.Errorf("an invalid list was stored: %q", v)
	}
	if code, _ := f.put(t, rename.KeyTemplates, `["[{studio}] {title}", "{title}"]`); code != http.StatusOK {
		t.Errorf("valid list = %d", code)
	}
	if v, _, _ := f.r.GetSetting(context.Background(), rename.KeyTemplates); v != `["[{studio}] {title}","{title}"]` {
		t.Errorf("stored = %q", v)
	}
	for _, c := range []struct {
		key, value string
		want       int
	}{
		{rename.KeyMode, "manual", 200}, {rename.KeyMode, "sometimes", 422},
		{rename.KeySettle, "7", 200}, {rename.KeySettle, "0", 422}, {rename.KeySettle, "366", 422},
		{"not.a.key", "x", 404},
	} {
		if code, body := f.put(t, c.key, c.value); code != c.want {
			t.Errorf("PUT %s=%s = %d %v, want %d", c.key, c.value, code, body, c.want)
		}
	}
}

func (f *renameFixture) plan(t *testing.T, query string) map[string]any {
	t.Helper()
	code, body := getTok(t, f.srv.URL+"/api/v1/rename/plan"+query, "secret")
	if code != http.StatusOK {
		t.Fatalf("plan%s = %d %v", query, code, body)
	}
	return body
}

// A5: reasons and counts are server totals over the whole library; a filter and paging narrow the
// rows only. Plan mode touches no file (A4's Plan half).
func TestRenamePlanReasonsAndCounts(t *testing.T) {
	f := renameServer(t, "secret")
	f.put(t, rename.KeyTemplates, `["{title} - {studio}", "[{studio}] {title}", "{title}"]`)
	f.put(t, rename.KeyMode, rename.ModePlan)
	before := treeOf(t, f.lib)

	body := f.plan(t, "?limit=1")
	counts := body["counts"].(map[string]any)
	want := map[string]float64{rename.ReasonWillRename: 3, rename.ReasonWontReadBack: 1, rename.ReasonClash: 1, rename.ReasonAlreadyNamed: 0}
	for k, v := range want {
		if counts[k] != v {
			t.Errorf("count %s = %v, want %v (all %v)", k, counts[k], v, counts)
		}
	}
	if body["total"] != float64(5) || len(body["rows"].([]any)) != 1 || body["filtered"] != float64(5) {
		t.Errorf("total/filtered/rows = %v/%v/%d", body["total"], body["filtered"], len(body["rows"].([]any)))
	}
	clash := f.plan(t, "?reason=clash")
	rows := clash["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["video_id"] != float64(f.ids["junk4.mkv"]) {
		t.Errorf("clash rows = %v", rows)
	}
	if clash["counts"].(map[string]any)[rename.ReasonClash] != float64(1) || clash["total"] != float64(5) {
		t.Errorf("a filter changed the totals: %v", clash["counts"])
	}
	tc := body["template_counts"].([]any)
	if tc[0].(map[string]any)["wont_read_back"] != float64(1) {
		t.Errorf("template counts = %v", tc)
	}
	wrb := f.plan(t, "?reason="+rename.ReasonWontReadBack)["rows"].([]any)
	if len(wrb) != 1 || wrb[0].(map[string]any)["written"] == nil || wrb[0].(map[string]any)["parsed"] == nil {
		t.Errorf("won't-read-back row = %v", wrb)
	}
	if _, ok := body["providers"].([]any); !ok {
		t.Errorf("providers = %v", body["providers"])
	}
	if code, _ := getTok(t, f.srv.URL+"/api/v1/rename/plan?reason=nope", "secret"); code != http.StatusBadRequest {
		t.Errorf("unknown reason = %d", code)
	}
	if after := treeOf(t, f.lib); !bytes.Equal(before, after) {
		t.Error("the plan touched the library")
	}
}

// {people} names each linked person once, by their canonical name: a file tag carrying an alias
// and a source carrying the name survive the actors merge as two values, but they are one person.
func TestRenamePeopleUseCanonicalName(t *testing.T) {
	f := renameServer(t, "secret")
	ctx := context.Background()
	path := filepath.Join(f.lib, "junk5.mkv")
	if err := os.WriteFile(path, []byte("junk5"), 0o644); err != nil {
		t.Fatal(err)
	}
	vid, err := f.r.UpsertVideo(ctx, &model.Video{FilePath: path, Title: "Gala",
		FileMtime: time.Now().UTC().Truncate(time.Second)}, []model.ExtraMetadata{
		{SourceKey: "Artist", Value: "J. Doe"}, {SourceKey: "Artist", Value: "R. Roe"},
		{SourceKey: "Performer", Value: "Jane Doe"}, {SourceKey: "Performer", Value: "Ray Roe"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Ray's link has no role, as a link made before roles existed does.
	if err := f.r.ReconcileVideoPeople(ctx, vid, []repo.PersonRoleName{
		{Name: "Jane Doe", Role: "actor"}, {Name: "Ray Roe", Role: ""}}, nil); err != nil {
		t.Fatal(err)
	}
	for name, alias := range map[string]string{"Jane Doe": "J. Doe", "Ray Roe": "R. Roe"} {
		pid, ok, err := f.r.PersonIDByName(ctx, name)
		if err != nil || !ok {
			t.Fatalf("person %s = %v %v", name, ok, err)
		}
		if _, err := f.r.AddPersonAlias(ctx, pid, alias); err != nil {
			t.Fatal(err)
		}
	}
	f.put(t, rename.KeyTemplates, `["{title} - {people}"]`)
	f.put(t, rename.KeyMode, rename.ModePlan)

	rows := f.plan(t, "?reason=all&video_id="+itoa(vid))["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %v", rows)
	}
	if name := rows[0].(map[string]any)["name"]; name != "Gala - Jane Doe, Ray Roe.mkv" {
		t.Errorf("name = %v, want Gala - Jane Doe, Ray Roe.mkv", name)
	}
}

// {film} renders the first linked film by name as a {film-<name>} marker; a video with no film
// drops an optional {film?}.
func TestRenameFilmMarker(t *testing.T) {
	f := renameServer(t, "secret")
	ctx := context.Background()
	vid := f.ids["junk2.mkv"]
	for _, name := range []string{"Zed", "Ada Lovelace"} {
		fid, err := f.r.CreateFilm(ctx, name, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.r.AttachFilmVideo(ctx, fid, vid, nil, true); err != nil {
			t.Fatal(err)
		}
	}
	f.put(t, rename.KeyTemplates, `["{title} {film?}"]`)
	f.put(t, rename.KeyMode, rename.ModePlan)

	for id, want := range map[int64]string{vid: "Bob {film-Ada Lovelace}.mkv", f.ids["junk1.mkv"]: "Ada Lovelace.mkv"} {
		rows := f.plan(t, "?reason=all&video_id="+itoa(id))["rows"].([]any)
		if len(rows) != 1 || rows[0].(map[string]any)["name"] != want {
			t.Errorf("video %d rows = %v, want %s", id, rows, want)
		}
	}
}

// A6: a preview computes the plan from an unsaved list and saves nothing.
func TestRenamePlanPreview(t *testing.T) {
	f := renameServer(t, "secret")
	code, body := doJSON(t, http.MethodPost, f.srv.URL+"/api/v1/rename/plan/preview?reason=already-named", "secret",
		map[string]any{"templates": []string{"[{studio}] {title}"}})
	if code != http.StatusOK || body["counts"].(map[string]any)[rename.ReasonAlreadyNamed] != float64(1) {
		t.Fatalf("preview = %d %v", code, body)
	}
	if _, ok, _ := f.r.GetSetting(context.Background(), rename.KeyTemplates); ok {
		t.Error("preview saved the templates")
	}
	if code, _ := doJSON(t, http.MethodPost, f.srv.URL+"/api/v1/rename/plan/preview", "secret",
		map[string]any{"templates": []string{"{studio}"}}); code != http.StatusUnprocessableEntity {
		t.Errorf("invalid preview = %d, want 422", code)
	}
}

// HOLODEX-633: a preview carries an unsaved character table and renders with it; a refused table
// names each bad row; the saved table is what the plan uses after it's stored.
func TestRenameCharacterTable(t *testing.T) {
	f := renameServer(t, "secret")
	table := map[string]string{":": "-", "\"": "'", "/": "+", "\\": "+", "|": "-", "*": "-", "?": "", "<": "", ">": "", "-": "+"}
	names := func(body map[string]any) map[string]bool {
		out := map[string]bool{}
		for _, row := range body["rows"].([]any) {
			if n, _ := row.(map[string]any)["name"].(string); n != "" {
				out[n] = true
			}
		}
		return out
	}

	code, body := doJSON(t, http.MethodPost, f.srv.URL+"/api/v1/rename/plan/preview", "secret",
		map[string]any{"templates": []string{"{title}"}, "characters": table})
	if code != http.StatusOK || !names(body)["Ada + Redux.mkv"] {
		t.Fatalf("preview with a '-' row = %d, names %v; want Ada + Redux.mkv", code, names(body))
	}
	if _, ok, _ := f.r.GetSetting(context.Background(), rename.KeyCharacters); ok {
		t.Error("preview saved the character table")
	}

	code, body = doJSON(t, http.MethodPost, f.srv.URL+"/api/v1/rename/plan/preview", "secret",
		map[string]any{"templates": []string{"{title}"}, "characters": map[string]string{":": "/", "a": "b"}})
	rows, _ := body["characters"].(map[string]any)
	if code != http.StatusUnprocessableEntity || rows[":"] == nil || rows["a"] == nil || rows["?"] == nil {
		t.Fatalf("refused table = %d %v; want 422 naming :, a and the missing ?", code, body)
	}

	if code, body := f.put(t, rename.KeyCharacters, `{"a":"b"}`); code != http.StatusUnprocessableEntity || body["characters"] == nil {
		t.Errorf("PUT refused table = %d %v, want 422 with characters", code, body)
	}
	raw, _ := json.Marshal(table)
	if code, _ := f.put(t, rename.KeyCharacters, string(raw)); code != http.StatusOK {
		t.Fatalf("PUT valid table = %d", code)
	}
	if code, _ := f.put(t, rename.KeyTemplates, `["{title}"]`); code != http.StatusOK {
		t.Fatal("PUT templates")
	}
	if !names(f.plan(t, ""))["Ada + Redux.mkv"] {
		t.Error("the saved table didn't reach the plan")
	}
}

// A7: lock and unlock change only the name state; history lists done renames oldest first.
func TestRenameLockAndHistory(t *testing.T) {
	f := renameServer(t, "secret")
	id := f.ids["junk1.mkv"]
	url := f.srv.URL + "/api/v1/media/" + itoa(id) + "/name-lock"
	if code, _ := doJSON(t, http.MethodPut, url, "secret", map[string]bool{"locked": true}); code != http.StatusOK {
		t.Fatalf("lock = %d", code)
	}
	st, _ := f.r.FileNameStates(context.Background())
	if !st[id].Locked || st[id].LockSource != repo.LockOwner {
		t.Errorf("state = %+v", st[id])
	}
	f.put(t, rename.KeyTemplates, `["{title}"]`)
	body := f.plan(t, "?reason=locked")
	if body["filtered"] != float64(1) {
		t.Errorf("locked rows = %v", body["filtered"])
	}
	if row := body["rows"].([]any)[0].(map[string]any); row["lock_source"] != repo.LockOwner || row["locked_at"] == nil {
		t.Errorf("locked row = %v", row)
	}
	doJSON(t, http.MethodPut, url, "secret", map[string]bool{"locked": false})
	st, _ = f.r.FileNameStates(context.Background())
	if st[id].Locked || st[id].LockSource != "" {
		t.Errorf("unlocked state = %+v", st[id])
	}
	code, _ := getTok(t, f.srv.URL+"/api/v1/media/"+itoa(id)+"/renames", "secret")
	if code != http.StatusOK {
		t.Errorf("history = %d", code)
	}
	if code, _ := doJSON(t, http.MethodPut, f.srv.URL+"/api/v1/media/999999/name-lock", "secret", map[string]bool{"locked": true}); code != http.StatusNotFound {
		t.Errorf("unknown video lock = %d", code)
	}
}

// treeOf hashes a folder's names and bytes, to prove a step touched nothing.
func treeOf(t *testing.T, dir string) []byte {
	t.Helper()
	var b bytes.Buffer
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		b.WriteString(e.Name() + "\x00")
		b.Write(data)
		b.WriteByte(0)
	}
	return b.Bytes()
}
