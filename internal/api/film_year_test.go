package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/enrich"
	"holodex/internal/repo"
)

// filmYearServer mirrors filmEnrichServer (enrich_test.go) but seeds the film with
// NO year, which is the only state the F59/ADR-089 D3 fill acts on. It returns the
// repo so a test can assert on films.year directly rather than only through the API.
func filmYearServer(t *testing.T, name string) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)

	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte("sources:\n  - name: fake\n    base_url: http://fake:9100\n    entity_types: [person, film]\n    enabled: true\n"), 0o644); err != nil {
		t.Fatalf("write sources: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := enrich.NewStore(sp, log)
	if err != nil {
		t.Fatalf("sources store: %v", err)
	}
	svc := enrich.NewServiceWithClient(store, r, log, func(enrich.Source) enrich.ProviderClient { return enrich.NewFake("fake") })

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetEnrichment(svc)
	h.SetAuth(api.NewAuth(""), false)
	h.SetFilmsEnabled(true)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)

	fid, err := r.CreateFilm(context.Background(), name, 0) // 0 => year IS NULL
	if err != nil {
		t.Fatalf("create yearless film: %v", err)
	}
	return srv, r, fid
}

func filmYear(t *testing.T, r *repo.Repo, id int64) int {
	t.Helper()
	f, err := r.GetFilm(context.Background(), id)
	if err != nil {
		t.Fatalf("get film %d: %v", id, err)
	}
	return f.Year
}

// The happy path: a film created without a year picks one up from the provider's
// release_date. The fake's canned film carries "2001-07-20", so the assertion is on
// the *parsed* year, not on the date string being copied somewhere.
func TestFilmEnrichApply_FillsYearFromReleaseDate(t *testing.T) {
	srv, r, fid := filmYearServer(t, "Spirited Away")
	base := srv.URL + "/api/v1/films/" + itoa(fid)

	if got := filmYear(t, r, fid); got != 0 {
		t.Fatalf("precondition: seeded film already has year %d", got)
	}

	code, body := postTok(t, base+"/enrich", "", map[string]string{"provider": "fake", "external_id": "tmdb:129"})
	if code != http.StatusOK {
		t.Fatalf("apply code = %d", code)
	}
	if _, ok := body["year_collision"]; ok {
		t.Fatalf("unexpected collision on a clean fill: %v", body["year_collision"])
	}
	if got := filmYear(t, r, fid); got != 2001 {
		t.Fatalf("films.year after enrich = %d, want 2001", got)
	}

	// The detail payload must show it too — the header reads this, not the column.
	code, fbody := getJSON(t, base)
	if code != http.StatusOK {
		t.Fatalf("film detail code = %d", code)
	}
	film, _ := fbody["film"].(map[string]any)
	if year, _ := film["year"].(float64); int(year) != 2001 {
		t.Fatalf("detail film.year = %v, want 2001", film["year"])
	}
}

// The fill is one-way by design: it fills a blank, never rewrites an owner-asserted
// year. Rewriting would silently change half the (name, year) identity key, and could
// not be undone on clear because no prior value is stored anywhere.
func TestFilmEnrichApply_NeverOverwritesAnExistingYear(t *testing.T) {
	srv, r, _ := filmYearServer(t, "Placeholder")
	ctx := context.Background()

	// A deliberately WRONG year the owner asserted: the provider says 2001.
	fid, err := r.CreateFilm(ctx, "Spirited Away", 1999)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	base := srv.URL + "/api/v1/films/" + itoa(fid)

	code, body := postTok(t, base+"/enrich", "", map[string]string{"provider": "fake", "external_id": "tmdb:129"})
	if code != http.StatusOK {
		t.Fatalf("apply code = %d", code)
	}
	if _, ok := body["year_collision"]; ok {
		t.Fatalf("a skipped fill is not a collision: %v", body["year_collision"])
	}
	if got := filmYear(t, r, fid); got != 1999 {
		t.Fatalf("films.year = %d, want the owner's 1999 left intact", got)
	}
}

// The load-bearing test. A collision must leave BOTH films byte-identical on the
// identity column and name the occupant — no silent swap, no auto-bump (the same
// posture AttachFilmVideo takes for scene numbers).
//
// It must equally prove the enrich itself still succeeded: per ADR-089 D3 (as
// amended) only the identity write is gated, not the shadow store, which ADR-033
// makes deliberately additive. Asserting only "collision reported" would pass
// against an implementation that had thrown the whole enrich away.
func TestFilmEnrichApply_YearCollisionWithheldAndNamed(t *testing.T) {
	srv, r, fid := filmYearServer(t, "Spirited Away")
	ctx := context.Background()

	// Same name, already holding the year the provider will supply. Legal to create:
	// (name, NULL) and (name, 2001) are distinct under UNIQUE(name, year).
	occupantID, err := r.CreateFilm(ctx, "Spirited Away", 2001)
	if err != nil {
		t.Fatalf("create occupant: %v", err)
	}
	base := srv.URL + "/api/v1/films/" + itoa(fid)

	code, body := postTok(t, base+"/enrich", "", map[string]string{"provider": "fake", "external_id": "tmdb:129"})
	if code != http.StatusOK {
		t.Fatalf("apply code = %d, want 200 — the enrich is not rolled back by a year collision", code)
	}

	collision, _ := body["year_collision"].(map[string]any)
	if collision == nil {
		t.Fatal("no year_collision reported; the owner would never learn the year was withheld")
	}
	if got, _ := collision["film_id"].(float64); int64(got) != occupantID {
		t.Errorf("collision film_id = %v, want the occupant %d", collision["film_id"], occupantID)
	}
	if collision["film_name"] != "Spirited Away" {
		t.Errorf("collision film_name = %v, want the occupant's name", collision["film_name"])
	}
	if got, _ := collision["year"].(float64); int(got) != 2001 {
		t.Errorf("collision year = %v, want 2001", collision["year"])
	}

	// Neither film's identity moved.
	if got := filmYear(t, r, fid); got != 0 {
		t.Errorf("target film year = %d, want it left unset", got)
	}
	if got := filmYear(t, r, occupantID); got != 2001 {
		t.Errorf("occupant film year = %d, want an untouched 2001", got)
	}

	// ...and the enrich still landed. This is the half that distinguishes "gate the
	// identity write" from "reject the apply".
	code, fbody := getJSON(t, base)
	if code != http.StatusOK {
		t.Fatalf("film detail code = %d", code)
	}
	resolved, _ := fbody["resolved"].([]any)
	var gotDescription bool
	for _, f := range resolved {
		m, _ := f.(map[string]any)
		if m["canonical"] != "description" {
			continue
		}
		if vals, _ := m["values"].([]any); len(vals) > 0 && vals[0] != "" {
			gotDescription = true
		}
	}
	if !gotDescription {
		t.Error("description did not resolve — the collision wrongly discarded the whole enrich")
	}
}

// Repo-level guards for the states the API path cannot easily reach.
func TestFillFilmYear(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	r := repo.New(database)
	ctx := context.Background()

	blank, err := r.CreateFilm(ctx, "Akira", 0)
	if err != nil {
		t.Fatalf("create blank: %v", err)
	}

	// A non-positive year is "nothing to do", never a write of 0 or a negative.
	for _, y := range []int{0, -1} {
		if c, err := r.FillFilmYear(ctx, blank, y); err != nil || c != nil {
			t.Fatalf("FillFilmYear(%d) = (%v, %v), want (nil, nil)", y, c, err)
		}
		if got := filmYear(t, r, blank); got != 0 {
			t.Fatalf("year after FillFilmYear(%d) = %d, want 0", y, got)
		}
	}

	if c, err := r.FillFilmYear(ctx, blank, 1988); err != nil || c != nil {
		t.Fatalf("clean fill = (%v, %v)", c, err)
	}
	if got := filmYear(t, r, blank); got != 1988 {
		t.Fatalf("year after clean fill = %d, want 1988", got)
	}

	// Second fill is a no-op, not an overwrite — this is what makes the operation
	// idempotent across a re-enrich or a refresh-all.
	if c, err := r.FillFilmYear(ctx, blank, 2016); err != nil || c != nil {
		t.Fatalf("second fill = (%v, %v), want a silent no-op", c, err)
	}
	if got := filmYear(t, r, blank); got != 1988 {
		t.Fatalf("year after second fill = %d, want an unchanged 1988", got)
	}

	// A different film with the same name collides only on the taken year.
	other, err := r.CreateFilm(ctx, "Akira", 0)
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	c, err := r.FillFilmYear(ctx, other, 1988)
	if err != nil {
		t.Fatalf("collision fill errored: %v", err)
	}
	if c == nil || c.FilmID != blank || c.Year != 1988 {
		t.Fatalf("collision = %+v, want the 1988 Akira (%d)", c, blank)
	}
	if got := filmYear(t, r, other); got != 0 {
		t.Fatalf("year after collision = %d, want it left unset", got)
	}
	// A free year on the same name is fine — the constraint is the pair, not the name.
	if c, err := r.FillFilmYear(ctx, other, 2016); err != nil || c != nil {
		t.Fatalf("non-colliding same-name fill = (%v, %v)", c, err)
	}
	if got := filmYear(t, r, other); got != 2016 {
		t.Fatalf("year = %d, want 2016", got)
	}

	if _, err := r.FillFilmYear(ctx, 999999, 2001); err == nil {
		t.Error("FillFilmYear on a missing film returned no error")
	}
}
