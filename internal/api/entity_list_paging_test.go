package api_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/repo"
)

// walkPages reads list (a URL ending in "?..." or "&") page by page at size limit and
// returns the item names in order, failing on a total that disagrees between pages
// or a name seen twice. It is the F80 "every entry reachable, none repeated" check.
func walkPages(t *testing.T, list string, limit int) ([]string, int) {
	t.Helper()
	var names []string
	seen := map[string]bool{}
	total := -1
	for off := 0; ; off += limit {
		code, body := getJSON(t, fmt.Sprintf("%slimit=%d&offset=%d", list, limit, off))
		if code != 200 {
			t.Fatalf("%s offset %d = %d", list, off, code)
		}
		if total >= 0 && num(body, "total") != total {
			t.Fatalf("total changed between pages: %d then %d", total, num(body, "total"))
		}
		total = num(body, "total")
		if num(body, "limit") != limit || num(body, "offset") != off {
			t.Fatalf("envelope echo = limit %v offset %v, want %d %d", body["limit"], body["offset"], limit, off)
		}
		items := body["items"].([]any)
		for _, it := range items {
			n := it.(map[string]any)["name"].(string)
			if seen[n] {
				t.Fatalf("%q on two pages of %s", n, list)
			}
			seen[n] = true
			names = append(names, n)
		}
		if len(items) < limit {
			return names, total
		}
	}
}

func seedPeople(t *testing.T, r *repo.Repo, names ...string) {
	t.Helper()
	vid := seedVideo(t, r, "/m/people.mkv", "People")
	linkPeople(t, r, vid, names...)
}

// TestPeopleListPaging pins F80 on GET /people: name-order pages that tile, a total
// that counts the library (not the page), search across every row, and the default
// page size applying when no limit is sent.
func TestPeopleListPaging(t *testing.T) {
	srv, r, _ := newServer(t)
	seedPeople(t, r, "zed", "Bob", "mae", "Max", "9 Lives", "bea", "Carl")
	list := srv.URL + "/api/v1/people?"

	names, total := walkPages(t, list, 3)
	want := []string{"9 Lives", "bea", "Bob", "Carl", "mae", "Max", "zed"}
	if !reflect.DeepEqual(names, want) || total != len(want) {
		t.Errorf("paged names = %v (total %d), want %v", names, total, want)
	}

	names, total = walkPages(t, list+"q=MA&", 1)
	if !reflect.DeepEqual(names, []string{"mae", "Max"}) || total != 2 {
		t.Errorf("q=MA = %v (total %d), want [mae Max] (2)", names, total)
	}

	_, body := getJSON(t, list)
	if num(body, "limit") != 50 || num(body, "total") != len(want) || body["letters"] != nil {
		t.Errorf("unparameterised read = %v, want the default limit 50 and no letters", body)
	}
}

// TestPeopleLetterOffsets: letters=1 maps each index letter to the name-order offset
// of its first row, "#" covering names that don't start with a letter, and a letter
// restart (offset = letters[L]) lands on that letter's first name (F80 EL6).
func TestPeopleLetterOffsets(t *testing.T) {
	srv, r, _ := newServer(t)
	seedPeople(t, r, "zed", "Bob", "mae", "Max", "9 Lives", "bea")
	_, body := getJSON(t, srv.URL+"/api/v1/people?letters=1&limit=2&sort=count")
	letters, _ := body["letters"].(map[string]any)
	// Name order: 9 Lives, bea, Bob, mae, Max, zed.
	want := map[string]any{"#": 0.0, "B": 1.0, "M": 3.0, "Z": 5.0}
	if !reflect.DeepEqual(letters, want) {
		t.Fatalf("letters = %v, want %v (name-order offsets, whatever the page's sort)", letters, want)
	}
	_, body = getJSON(t, srv.URL+"/api/v1/people?limit=1&offset=3")
	if got := body["items"].([]any)[0].(map[string]any)["name"]; got != "mae" {
		t.Errorf("restart at M = %v, want mae", got)
	}
}

// TestPeopleRandomPagingTiles: one seed is one shuffle across pages, with every person
// exactly once; the same seed reproduces it (ADR-045, F80 EL4).
func TestPeopleRandomPagingTiles(t *testing.T) {
	srv, r, _ := newServer(t)
	seedPeople(t, r, "a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9")
	list := srv.URL + "/api/v1/people?sort=random&seed=42&"
	first, total := walkPages(t, list, 4)
	again, _ := walkPages(t, list, 3)
	if len(first) != total || total != 9 {
		t.Fatalf("random pages covered %d of %d, want all 9", len(first), total)
	}
	if !reflect.DeepEqual(first, again) {
		t.Errorf("seed 42 at page size 4 = %v, at 3 = %v: one seed must be one order", first, again)
	}
}

func TestStudiosAndTagsListPaging(t *testing.T) {
	srv, r := identityServer(t, "")
	for i, n := range []string{"Zeta", "alpha", "Beta"} {
		seedStudioVideo(t, r, fmt.Sprintf("/m/s%d.mkv", i), n)
	}
	seedTagVideo(t, r, "/m/t.mkv", "noir", "drama", "zombie")

	names, total := walkPages(t, srv.URL+"/api/v1/studios?", 2)
	if !reflect.DeepEqual(names, []string{"alpha", "Beta", "Zeta"}) || total != 3 {
		t.Errorf("studios = %v (total %d)", names, total)
	}
	_, body := getJSON(t, srv.URL+"/api/v1/studios?letters=1")
	if letters, _ := body["letters"].(map[string]any); len(letters) != 3 || letters["Z"] != 2.0 {
		t.Errorf("studio letters = %v, want A, B and Z, with Z at 2", letters)
	}

	names, total = walkPages(t, srv.URL+"/api/v1/tags?", 2)
	if !reflect.DeepEqual(names, []string{"drama", "noir", "zombie"}) || total != 3 {
		t.Errorf("tags = %v (total %d)", names, total)
	}
	names, _ = walkPages(t, srv.URL+"/api/v1/tags?q=o&sort=completeness_asc&", 5)
	if !reflect.DeepEqual(names, []string{"noir", "zombie"}) {
		t.Errorf("tags q=o with an unsupported sort = %v, want name order [noir zombie]", names)
	}
}

// TestCategoriesPagedOnlyWithLimit: ?limit= pages /categories in name order with the
// envelope (the /tags list, F80); without it the HOLODEX-586 shape is unchanged.
func TestCategoriesPagedOnlyWithLimit(t *testing.T) {
	srv, r := identityServer(t, "")
	for _, n := range []string{"Mood", "era", "Genre"} {
		if _, err := r.CreateCategory(context.Background(), n); err != nil {
			t.Fatalf("create category: %v", err)
		}
	}
	names, total := walkPages(t, srv.URL+"/api/v1/categories?", 2)
	if !reflect.DeepEqual(names, []string{"era", "Genre", "Mood"}) || total != 3 {
		t.Errorf("paged categories = %v (total %d)", names, total)
	}
	_, body := getJSON(t, srv.URL+"/api/v1/categories")
	if _, has := body["total"]; has || len(body["items"].([]any)) != 3 {
		t.Errorf("unpaged categories = %v, want the bare {items} of all 3", body)
	}
}

func TestFilmsListPaging(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetFilmsEnabled(true) // the /films routes don't exist otherwise
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	for _, n := range []string{"Vertigo", "alien", "Brazil", "Ran"} {
		if _, err := r.CreateFilm(context.Background(), n, 1980); err != nil {
			t.Fatalf("create film: %v", err)
		}
	}
	names, total := walkPages(t, srv.URL+"/api/v1/films?", 3)
	if !reflect.DeepEqual(names, []string{"alien", "Brazil", "Ran", "Vertigo"}) || total != 4 {
		t.Errorf("films = %v (total %d)", names, total)
	}
	shuffled, _ := walkPages(t, srv.URL+"/api/v1/films?sort=random&seed=7&", 3)
	if len(shuffled) != 4 {
		t.Errorf("random films covered %v, want all 4 once", shuffled)
	}
}
