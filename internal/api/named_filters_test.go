package api_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"holodex/internal/model"
)

// F90 P0-3, P0-4, P0-8 (testing-strategy §42.5 A4, A5; §42.4 R5): the People and Studios
// filters and the enriched sorts, over the drain's field-state cache.

type namedEnv struct {
	*rvEnv
	person map[string]int64
	studio map[string]int64
}

func newNamedEnv(t *testing.T) *namedEnv {
	t.Helper()
	e := &namedEnv{rvEnv: newFSEnv(t), person: map[string]int64{}, studio: map[string]int64{}}
	ctx := context.Background()
	v := e.video(t, "/m/n.mkv")
	enrich := func(entityType string, id int64, provider string, fields map[string][]string) {
		if err := e.repo.UpsertEnrichment(ctx, entityType, id, provider, "x", fields); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"Ann Lee", "Bo Diaz", "Cy Moss", "Di Park"} {
		var id int64
		if err := e.db.QueryRow(`INSERT INTO people (name) VALUES (?) RETURNING id`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.Exec(`INSERT INTO video_people (video_id, person_id, role) VALUES (?, ?, 'actor')`, v, id); err != nil {
			t.Fatal(err)
		}
		e.person[name] = id
	}
	// Ann: two fields from a declared provider. Di: one. Cy: a birthdate from a provider
	// that isn't declared, shown but neither linked nor enriched. Bo: nothing.
	enrich(model.EnrichEntityPerson, e.person["Ann Lee"], "fake", map[string][]string{"birthdate": {"1970-01-01"}, "bio": {"An actor."}})
	enrich(model.EnrichEntityPerson, e.person["Di Park"], "fake", map[string][]string{"bio": {"Another."}})
	enrich(model.EnrichEntityPerson, e.person["Cy Moss"], "gone", map[string][]string{"birthdate": {"1980-01-01"}})

	for _, name := range []string{"Acme", "Brio"} {
		var id int64
		if err := e.db.QueryRow(`INSERT INTO studios (name) VALUES (?) RETURNING id`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.Exec(`INSERT INTO video_studios (video_id, studio_id) VALUES (?, ?)`, v, id); err != nil {
			t.Fatal(err)
		}
		e.studio[name] = id
	}
	enrich(model.EnrichEntityStudio, e.studio["Acme"], "fake", map[string][]string{"country": {"US"}})
	return e
}

// list GETs a People or Studios list and returns the item names in order and the total.
func (e *namedEnv) list(t *testing.T, path, token string) ([]string, int) {
	t.Helper()
	code, body := getJSONTok(t, e.srv.URL+"/api/v1/"+path, token)
	if code != http.StatusOK {
		t.Fatalf("%s: %d %v", path, code, body)
	}
	var names []string
	for _, it := range body["items"].([]any) {
		names = append(names, it.(map[string]any)["name"].(string))
	}
	return names, int(body["total"].(float64))
}

func (e *namedEnv) wantNames(t *testing.T, path string, want ...string) {
	t.Helper()
	got, total := e.list(t, path, "s3cret")
	if !slices.Equal(got, want) && (len(got) != 0 || len(want) != 0) || total != len(want) {
		t.Errorf("%s = %v (total %d), want %v", path, got, total, want)
	}
}

// A4: each filter returns exactly the entities whose page matches, and they AND together.
func TestPeopleAndStudiosFilters(t *testing.T) {
	e := newNamedEnv(t)
	e.wantNames(t, "people?missing=birthdate", "Bo Diaz", "Di Park")
	e.wantNames(t, "people?has=birthdate", "Ann Lee", "Cy Moss") // Cy's page shows the undeclared provider's
	e.wantNames(t, "people?link=linked", "Ann Lee", "Di Park")
	e.wantNames(t, "people?link=unlinked", "Bo Diaz", "Cy Moss")
	e.wantNames(t, "people?link=linked&missing=birthdate", "Di Park")
	e.wantNames(t, "people?has=bio&has=birthdate", "Ann Lee")
	e.wantNames(t, "people?missing=photo", "Ann Lee", "Bo Diaz", "Cy Moss", "Di Park")
	e.wantNames(t, "studios?link=unlinked", "Brio")
	e.wantNames(t, "studios?has=country", "Acme")
	e.wantNames(t, "studios?missing=branding", "Acme", "Brio")
}

// R5, P0-4: Most and Least enriched order by the count of provider-contributed fields, ties
// in name order, and paging returns every entity exactly once.
func TestEnrichedSortPagesWithoutRepeats(t *testing.T) {
	e := newNamedEnv(t)
	e.wantNames(t, "people?sort=enriched_desc", "Ann Lee", "Di Park", "Bo Diaz", "Cy Moss")
	e.wantNames(t, "people?sort=enriched_asc", "Bo Diaz", "Cy Moss", "Di Park", "Ann Lee")
	var paged []string
	for _, off := range []string{"0", "3"} {
		got, total := e.list(t, "people?sort=enriched_desc&limit=3&offset="+off, "s3cret")
		if total != 4 {
			t.Errorf("total = %d", total)
		}
		paged = append(paged, got...)
	}
	if !slices.Equal(paged, []string{"Ann Lee", "Di Park", "Bo Diaz", "Cy Moss"}) {
		t.Errorf("paged = %v", paged)
	}
	e.wantNames(t, "studios?sort=enriched_desc", "Acme", "Brio")
}

// A5, SEC-13: every new parameter is refused for a visitor with missing_facet's 401, never
// ignored; the owner's unknown values are 400s; the old lists are unchanged for a visitor.
func TestNamedListFiltersAreOwnerOnly(t *testing.T) {
	e := newNamedEnv(t)
	for _, path := range []string{
		"people?link=linked", "people?has=bio", "people?missing=birthdate", "people?sort=enriched_desc",
		"studios?link=unlinked", "studios?has=country", "studios?sort=enriched_asc",
		"media?sort=enriched_desc", "media/ids?sort=enriched_asc",
	} {
		if code, _ := getJSONTok(t, e.srv.URL+"/api/v1/"+path, ""); code != http.StatusUnauthorized {
			t.Errorf("visitor %s = %d, want 401", path, code)
		}
	}
	for _, path := range []string{"people?link=maybe", "people?has=tagline", "people?missing=branding", "studios?has=photo", "studios?missing=_x"} {
		if code, _ := getJSONTok(t, e.srv.URL+"/api/v1/"+path, "s3cret"); code != http.StatusBadRequest {
			t.Errorf("owner %s = %d, want 400", path, code)
		}
	}
	if got, total := e.list(t, "people", ""); total != 4 || len(got) != 4 {
		t.Errorf("visitor people = %v (%d)", got, total)
	}
	// Tags share the parameter parser but not the filters: they never narrow a Tags list.
	if _, err := e.db.Exec(`INSERT INTO tags (name) VALUES ('noir')`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"tags?has=bio", "tags?link=linked", "tags?missing=name"} {
		if got, total := e.list(t, path, ""); total != 1 || len(got) != 1 {
			t.Errorf("visitor %s = %v (%d), want the one tag", path, got, total)
		}
	}
}
