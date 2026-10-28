package api_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"holodex/internal/api"
	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/repo"
	"holodex/internal/thumbnail"
)

// F85 (HOLODEX-599) poster chooser: the frame and upload routes, upload removal,
// local poster decisions and the owner's poster_url payload
// (testing-strategy §37.2 G8, G9, G13).

const chooserToken = "secret"

type chooser struct {
	srv      *httptest.Server
	repo     *repo.Repo
	thumbDir string
	id       int64
}

func chooserServer(t *testing.T) chooser {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	thumbDir := filepath.Join(dir, "thumbnails")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mpath := filepath.Join(dir, "metadata-mappings.yaml")
	yaml := "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [file:title]\n" +
		"  - canonical: poster_url\n    label: Poster\n    sources: [provider-1:poster_url]\n"
	if err := os.WriteFile(mpath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}
	h := api.NewHandlers(r, log, thumbnail.New(thumbnail.Config{Dir: thumbDir}, log, r), thumbDir, nil, nil)
	h.SetMetadataFields(store, cache.Noop{})
	h.SetAuth(api.NewAuth(chooserToken), false)
	h.SetPersonImages(filepath.Join(dir, "person-images"), 10<<20, 2000)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return chooser{srv: srv, repo: r, thumbDir: thumbDir, id: seedThumbVideo(t, r, "/m/a.mkv")}
}

func (c chooser) url(path string) string { return c.srv.URL + "/api/v1/media/" + itoa(c.id) + path }

func (c chooser) do(t *testing.T, method, url, token string, body io.Reader, contentType string) int {
	t.Helper()
	req, _ := http.NewRequest(method, url, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set(api.AdminTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func (c chooser) postImage(t *testing.T, path, token string) int {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "frame.png")
	_, _ = fw.Write(postersPNG(t, 40, 60))
	_ = mw.Close()
	return c.do(t, http.MethodPost, c.url(path), token, &body, mw.FormDataContentType())
}

func (c chooser) putDecision(t *testing.T, source string) int {
	t.Helper()
	return c.do(t, http.MethodPut, c.url("/fields/poster_url/decision"), chooserToken,
		bytes.NewReader([]byte(`{"source":"`+source+`"}`)), "application/json")
}

func (c chooser) slot(t *testing.T, slot string) []byte {
	t.Helper()
	b, err := os.ReadFile(thumbnail.SlotPath(c.thumbDir, c.id, slot))
	if err != nil {
		t.Fatalf("slot %s: %v", slot, err)
	}
	return b
}

// served returns the presented poster's bytes, nil when there is none.
func (c chooser) served(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(thumbnail.PosterPath(c.thumbDir, c.id))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// posterField returns the detail payload's poster_url row, nil when absent.
func (c chooser) posterField(t *testing.T, token string) map[string]any {
	t.Helper()
	code, body := getJSONTok(t, c.url(""), token)
	if code != http.StatusOK {
		t.Fatalf("detail = %d", code)
	}
	rows, _ := body["resolved"].([]any)
	for _, row := range rows {
		if m, _ := row.(map[string]any); m["canonical"] == "poster_url" {
			return m
		}
	}
	return nil
}

func candidates(row map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	cands, _ := row["candidates"].([]any)
	for _, c := range cands {
		m, _ := c.(map[string]any)
		out[m["source"].(string)] = m
	}
	return out
}

// G8 + G13: the frame and upload routes each store their slot, become the decision,
// and are presented in one request; each keeps its own tile; visitors are refused.
func TestPosterChooser_FrameAndUploadStageTheirSlots(t *testing.T) {
	c := chooserServer(t)
	for _, path := range []string{"/poster/frame?at=760.5", "/poster"} {
		if code := c.postImage(t, path, ""); code != http.StatusUnauthorized && code != http.StatusForbidden {
			t.Fatalf("visitor POST %s = %d", path, code)
		}
	}
	if code := c.postImage(t, "/poster/frame?at=-1", chooserToken); code != http.StatusBadRequest {
		t.Fatalf("negative at = %d", code)
	}

	if code := c.postImage(t, "/poster/frame?at=760.5", chooserToken); code != http.StatusCreated {
		t.Fatalf("frame = %d", code)
	}
	if !bytes.Equal(c.served(t), c.slot(t, thumbnail.SlotFrame)) {
		t.Fatal("the page does not show the picked frame")
	}
	row := c.posterField(t, chooserToken)
	if row == nil {
		t.Fatal("no Poster row for the owner")
	}
	if dec, _ := row["decision"].(map[string]any); dec["source"] != "local:frame" || dec["standing"] != true {
		t.Fatalf("decision = %v", row["decision"])
	}
	frame := candidates(row)["local:frame"]
	if frame == nil || frame["frame_at"] != 760.5 || frame["image_url"] != "/api/v1/media/"+itoa(c.id)+"/poster/sources/frame?v="+thumbnail.ContentHash(c.slot(t, thumbnail.SlotFrame)) {
		t.Fatalf("frame candidate = %v", frame)
	}

	if code := c.postImage(t, "/poster", chooserToken); code != http.StatusCreated {
		t.Fatalf("upload = %d", code)
	}
	if !bytes.Equal(c.served(t), c.slot(t, thumbnail.SlotUpload)) {
		t.Fatal("the page does not show the upload")
	}
	cands := candidates(c.posterField(t, chooserToken))
	if cands["local:frame"] == nil || cands["local:upload"] == nil {
		t.Fatalf("both owner tiles should remain: %v", cands)
	}
	if dec, _ := c.repo.PosterDecision(context.Background(), c.id); dec != "local:upload" {
		t.Fatalf("decision = %q", dec)
	}

	// A visitor gets no local tiles, and can't load a slot image.
	if row := c.posterField(t, ""); row != nil {
		for src := range candidates(row) {
			if src == "local:frame" || src == "local:upload" {
				t.Fatalf("visitor sees %s", src)
			}
		}
	}
	if code := c.do(t, http.MethodGet, c.url("/poster/sources/upload"), "", nil, ""); code == http.StatusOK {
		t.Fatal("visitor loaded a poster source")
	}
	if code := c.do(t, http.MethodGet, c.url("/poster/sources/upload"), chooserToken, nil, ""); code != http.StatusOK {
		t.Fatalf("owner slot GET = %d", code)
	}
	if code := c.do(t, http.MethodGet, c.url("/poster/sources/..%2F1"), chooserToken, nil, ""); code != http.StatusNotFound {
		t.Fatalf("unknown kind = %d", code)
	}
}

// G9: removing the chosen upload clears the decision and falls back; removing one
// that isn't chosen leaves the decision.
func TestPosterChooser_RemoveUpload(t *testing.T) {
	c := chooserServer(t)
	if err := os.WriteFile(thumbnail.SlotPath(c.thumbDir, c.id, thumbnail.SlotFile), []byte("FILE ART"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.postImage(t, "/poster", chooserToken)
	if code := c.do(t, http.MethodDelete, c.url("/poster"), chooserToken, nil, ""); code != http.StatusNoContent {
		t.Fatalf("remove = %d", code)
	}
	if dec, _ := c.repo.PosterDecision(context.Background(), c.id); dec != "" {
		t.Fatalf("decision left = %q", dec)
	}
	if string(c.served(t)) != "FILE ART" {
		t.Fatalf("served = %q, want the file's art", c.served(t))
	}

	c.postImage(t, "/poster", chooserToken)
	c.postImage(t, "/poster/frame", chooserToken)
	c.do(t, http.MethodDelete, c.url("/poster"), chooserToken, nil, "")
	if dec, _ := c.repo.PosterDecision(context.Background(), c.id); dec != "local:frame" {
		t.Fatalf("decision = %q, want the frame kept", dec)
	}
	if _, err := os.Stat(thumbnail.SlotPath(c.thumbDir, c.id, thumbnail.SlotUpload)); !os.IsNotExist(err) {
		t.Fatal("upload slot not removed")
	}
}

// A file with cover art and no decision: the owner's Poster row reads `·file`, in
// sync — the file candidate carries the art's token, never "" (F85, design state C
// applies only to a file with no cover art).
func TestPosterChooser_FileCoverIsUndecidedWinner(t *testing.T) {
	c := chooserServer(t)
	art := []byte("FILE ART")
	if err := os.WriteFile(thumbnail.SlotPath(c.thumbDir, c.id, thumbnail.SlotFile), art, 0o644); err != nil {
		t.Fatal(err)
	}
	row := c.posterField(t, chooserToken)
	if row == nil {
		t.Fatal("no Poster row")
	}
	token := "file#" + thumbnail.ContentHash(art)
	if file := candidates(row)["file"]; file["value"] != token || file["image_url"] == nil {
		t.Fatalf("file candidate = %v", file)
	}
	if dec, _ := row["decision"].(map[string]any); dec["source"] != "file" || dec["standing"] == true {
		t.Fatalf("decision = %v", row["decision"])
	}
	if row["in_sync"] != true {
		t.Fatalf("in_sync = %v", row["in_sync"])
	}
}

// A local decision is set only on poster_url and only for a held slot, and every
// poster_url decision change re-presents the page's poster.
func TestPosterChooser_LocalDecisions(t *testing.T) {
	c := chooserServer(t)
	if err := os.WriteFile(thumbnail.SlotPath(c.thumbDir, c.id, thumbnail.SlotFile), []byte("FILE ART"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := c.putDecision(t, "local:automatic"); code != http.StatusBadRequest {
		t.Fatalf("unheld local:automatic = %d", code)
	}
	if code := c.putDecision(t, "local:../x"); code != http.StatusBadRequest {
		t.Fatalf("path-shaped kind = %d", code)
	}
	if code := c.do(t, http.MethodPut, c.url("/fields/title/decision"), chooserToken,
		bytes.NewReader([]byte(`{"source":"local:frame"}`)), "application/json"); code != http.StatusBadRequest {
		t.Fatalf("local on title = %d", code)
	}

	c.postImage(t, "/poster/frame", chooserToken)
	if code := c.putDecision(t, "file"); code != http.StatusNoContent {
		t.Fatalf("file decision = %d", code)
	}
	if string(c.served(t)) != "FILE ART" {
		t.Fatalf("file decision serves %q", c.served(t))
	}
	if code := c.putDecision(t, "local:frame"); code != http.StatusNoContent {
		t.Fatalf("local:frame = %d", code)
	}
	if !bytes.Equal(c.served(t), c.slot(t, thumbnail.SlotFrame)) {
		t.Fatal("local:frame decision did not present the frame")
	}
	if code := c.do(t, http.MethodDelete, c.url("/fields/poster_url/decision"), chooserToken, nil, ""); code != http.StatusNoContent {
		t.Fatalf("clear = %d", code)
	}
	if string(c.served(t)) != "FILE ART" {
		t.Fatalf("cleared decision serves %q", c.served(t))
	}
	// The file tile carries its slot's image.
	if file := candidates(c.posterField(t, chooserToken))["file"]; file == nil || file["image_url"] == nil {
		t.Fatalf("file candidate = %v", file)
	}
}
