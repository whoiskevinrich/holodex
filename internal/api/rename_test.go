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
		"  - canonical: studio\n    label: Studio\n    sources: [Publisher]\n"
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
