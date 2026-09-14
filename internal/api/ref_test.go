package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// Reference handles (F60 RD1, HOLODEX-374): `kind:id` is accepted wherever a bare
// id is, a mismatched kind is 400 naming the expected kind, and every entity
// payload carries `ref`.

func TestParseRef(t *testing.T) {
	cases := []struct {
		kind    model.Kind
		in      string
		want    int64
		wantErr bool
		kindErr bool
	}{
		{model.KindPerson, "42", 42, false, false},
		{model.KindPerson, " person:42 ", 42, false, false},
		{model.KindPerson, "film:42", 0, true, true},
		{model.KindPerson, "person:0", 0, true, false},
		{model.KindPerson, "person:abc", 0, true, false},
		{model.KindPerson, "person:", 0, true, false},
		{model.KindPerson, ":42", 0, true, true},
		{model.KindPerson, "abc", 0, true, false},
		{model.KindPerson, "-1", 0, true, false},
		{"", "7", 7, false, false},
		{"", "video:7", 0, true, false},
	}
	for _, tc := range cases {
		got, err := api.ParseRef(tc.kind, tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("ParseRef(%q, %q) = %d, %v; want %d, err=%v", tc.kind, tc.in, got, err, tc.want, tc.wantErr)
		}
		var kindErr *api.RefKindError
		if errors.As(err, &kindErr) != tc.kindErr {
			t.Errorf("ParseRef(%q, %q): kind error = %v, want %v", tc.kind, tc.in, err, tc.kindErr)
		}
	}
	if got := (&api.RefKindError{Expected: model.KindPerson, Ref: "film:42"}).Error(); got != "expected a person ref, got film:42" {
		t.Errorf("RefKindError message = %q", got)
	}
}

// refServer seeds one of each entity kind with films enabled and returns the
// server plus the numeric ids keyed by API collection.
func refServer(t *testing.T) (*httptest.Server, map[string]int64) {
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
	vid := seedVideo(t, r, "/m/ref.mkv", "Ref clip")
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Acme"}, nil); err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	fid, err := r.CreateFilm(ctx, "Ref film", 2020)
	if err != nil {
		t.Fatalf("seed film: %v", err)
	}
	people, err := r.ListPeople(ctx, false)
	if err != nil || len(people) != 1 {
		t.Fatalf("list people: %v (%d)", err, len(people))
	}
	tags, err := r.ListTags(ctx, false)
	if err != nil || len(tags) != 1 {
		t.Fatalf("list tags: %v (%d)", err, len(tags))
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil || len(studios) != 1 {
		t.Fatalf("list studios: %v (%d)", err, len(studios))
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetFilmsEnabled(true)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, map[string]int64{
		"media": vid, "people": people[0].ID, "tags": tags[0].ID, "studios": studios[0].ID, "films": fid,
	}
}

var refKinds = map[string]model.Kind{
	"media": model.KindVideo, "people": model.KindPerson, "tags": model.KindTag,
	"studios": model.KindStudio, "films": model.KindFilm,
}

func getBody(t *testing.T, url string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func TestEntityRoutesAcceptRef(t *testing.T) {
	srv, ids := refServer(t)
	for coll, id := range ids {
		kind := refKinds[coll]
		base := fmt.Sprintf("%s/api/v1/%s/", srv.URL, coll)

		bareStatus, bare := getBody(t, fmt.Sprintf("%s%d", base, id))
		refStatus, ref := getBody(t, fmt.Sprintf("%s%s:%d", base, kind, id))
		if bareStatus != http.StatusOK || refStatus != http.StatusOK {
			t.Errorf("%s: bare=%d ref=%d, want 200/200", coll, bareStatus, refStatus)
			continue
		}
		if string(bare) != string(ref) {
			t.Errorf("%s: ref body differs from bare body\nbare: %s\nref:  %s", coll, bare, ref)
		}
		// The top-level entity object carries ref = kind:id.
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(bare, &payload); err != nil {
			t.Fatalf("%s: decode: %v", coll, err)
		}
		entityKey := map[string]string{"media": "video", "people": "person", "tags": "tag", "studios": "studio", "films": "film"}[coll]
		var entity struct {
			ID  int64  `json:"id"`
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal(payload[entityKey], &entity); err != nil {
			t.Fatalf("%s: decode %q: %v", coll, entityKey, err)
		}
		if want := model.Ref(kind, id); entity.Ref != want || entity.ID != id {
			t.Errorf("%s: entity = %+v, want ref %q", coll, entity, want)
		}

		// Wrong kind is 400 (the row exists — the request is malformed), naming the expected kind.
		wrong := model.KindFilm
		if kind == model.KindFilm {
			wrong = model.KindPerson
		}
		status, body := getBody(t, fmt.Sprintf("%s%s:%d", base, wrong, id))
		if status != http.StatusBadRequest {
			t.Errorf("%s: %s:%d status = %d, want 400", coll, wrong, id, status)
		}
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &errBody)
		if want := fmt.Sprintf("expected a %s ref, got %s:%d", kind, wrong, id); errBody.Error != want {
			t.Errorf("%s: error = %q, want %q", coll, errBody.Error, want)
		}
	}
}

func TestListItemsCarryRef(t *testing.T) {
	srv, ids := refServer(t)
	for coll, id := range ids {
		status, body := getBody(t, srv.URL+"/api/v1/"+coll)
		if status != http.StatusOK {
			t.Fatalf("%s list: %d", coll, status)
		}
		var list struct {
			Items []struct {
				ID  int64  `json:"id"`
				Ref string `json:"ref"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &list); err != nil {
			t.Fatalf("%s list decode: %v", coll, err)
		}
		if len(list.Items) != 1 || list.Items[0].Ref != model.Ref(refKinds[coll], id) {
			t.Errorf("%s list items = %+v, want one item with ref %q", coll, list.Items, model.Ref(refKinds[coll], id))
		}
	}
}

// Nested entities (Video.People / Video.Tags) ride the same MarshalJSON, so a
// video's people carry person refs without the handler doing anything.
func TestNestedEntitiesCarryRef(t *testing.T) {
	srv, ids := refServer(t)
	_, body := getBody(t, fmt.Sprintf("%s/api/v1/media/%d", srv.URL, ids["media"]))
	if !strings.Contains(string(body), fmt.Sprintf(`"ref":"person:%d"`, ids["people"])) {
		t.Errorf("video payload lacks nested person ref: %s", body)
	}
}

// Nested routes read the kind off the same route pattern.
func TestNestedRouteAcceptsRef(t *testing.T) {
	srv, ids := refServer(t)
	if status, _ := getBody(t, fmt.Sprintf("%s/api/v1/media/video:%d/related", srv.URL, ids["media"])); status != http.StatusOK {
		t.Errorf("media/video:N/related = %d, want 200", status)
	}
	if status, _ := getBody(t, fmt.Sprintf("%s/api/v1/media/tag:%d/related", srv.URL, ids["media"])); status != http.StatusBadRequest {
		t.Errorf("media/tag:N/related = %d, want 400", status)
	}
}

// Routes with no entity kind (categories, writeback jobs) keep bare ids only —
// a ref there is a plain 400, not a kind-mismatch.
func TestNonEntityRouteRejectsRef(t *testing.T) {
	srv, _, _ := newServer(t)
	status, body := getBody(t, srv.URL+"/api/v1/categories/tag:1")
	if status != http.StatusBadRequest || !strings.Contains(string(body), `"invalid id"`) {
		t.Errorf("categories/tag:1 = %d %s, want 400 invalid id", status, body)
	}
}

func TestModelMarshalRef(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, tc := range []struct {
		v    any
		want string
	}{
		{model.Video{ID: 1, Title: "t", IndexedAt: now}, `"ref":"video:1"`},
		{&model.Video{ID: 2, IndexedAt: now}, `"ref":"video:2"`},
		{model.Person{ID: 3}, `"ref":"person:3"`},
		{model.Studio{ID: 4}, `"ref":"studio:4"`},
		{model.Tag{ID: 5}, `"ref":"tag:5"`},
		{model.Film{ID: 6}, `"ref":"film:6"`},
	} {
		b, err := json.Marshal(tc.v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), tc.want) || !strings.Contains(string(b), `"id":`) {
			t.Errorf("%T marshals to %s, want %s alongside id", tc.v, b, tc.want)
		}
	}
}
