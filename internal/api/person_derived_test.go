package api_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"holodex/internal/api"
	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// derivedFixedNow is the deterministic read-path clock the derived-field API tests
// inject via Handlers.SetNow so Age never depends on the wall clock (AC-8).
var derivedFixedNow = time.Date(2026, time.July, 8, 12, 0, 0, 0, time.UTC)

// personDerivedServer wires a person ("Maya") with a matched `tmdb` provider that
// supplies the given enrichment fields, over a real repo with the read-path clock
// pinned to derivedFixedNow. token="" leaves the owner gate open.
func personDerivedServer(t *testing.T, token string, fields map[string][]string) (*httptest.Server, int64) {
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
	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/x.mkv", FileSize: 1, Title: "Clip", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Maya"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	linkPeople(t, r, vid, "Maya")
	pid, _, err := r.PersonIDByName(ctx, "Maya")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	if len(fields) > 0 {
		if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, pid, "tmdb", "ext-1", fields); err != nil {
			t.Fatalf("seed enrichment: %v", err)
		}
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	h.SetNow(func() time.Time { return derivedFixedNow })
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, pid
}

// TestPersonDerived_AgeUnderBirthdate covers AC-1/AC-2/AC-6/AC-7 at the API layer: a
// living person with a birthdate gets an Age row, positioned directly under birthdate,
// stamped computed with the computed: source, nil decision/candidates, and the
// transitive derived_from label — identical for owner and visitor (AC-4/D3).
func TestPersonDerived_AgeUnderBirthdate(t *testing.T) {
	for _, token := range []string{"", "secret"} { // "" = owner-open, "secret" = visitor (no header)
		srv, pid := personDerivedServer(t, token, map[string][]string{"birthdate": {"1990-03-14"}})
		resolved := getResolved(t, srv, pid)

		age, ok := findField(resolved, "age")
		if !ok {
			t.Fatalf("token=%q: want an age row", token)
		}
		if v := age["values"].([]any); len(v) != 1 || v[0] != "36" {
			t.Fatalf("token=%q: age values = %v, want [36]", token, age["values"])
		}
		if age["computed"] != true {
			t.Errorf("token=%q: age must carry computed:true", token)
		}
		if age["winning_source"] != "computed:age" {
			t.Errorf("token=%q: winning_source = %v, want computed:age", token, age["winning_source"])
		}
		if _, has := age["decision"]; has {
			t.Errorf("token=%q: a computed row must carry no decision", token)
		}
		if _, has := age["candidates"]; has {
			t.Errorf("token=%q: a computed row must carry no candidates", token)
		}
		df, _ := age["derived_from"].([]any)
		if len(df) != 1 || df[0] != "Born" {
			t.Errorf("token=%q: derived_from = %v, want [Born]", token, age["derived_from"])
		}
		// Positioned immediately after birthdate.
		if bd, age := indexOf(resolved, "birthdate"), indexOf(resolved, "age"); age != bd+1 {
			t.Errorf("token=%q: age at %d, birthdate at %d — want adjacency", token, age, bd)
		}
		if _, has := findField(resolved, "age_at_death"); has {
			t.Errorf("token=%q: a living person must not have age_at_death", token)
		}
	}
}

// TestPersonDerived_AgeAtDeathReplacesAge covers AC-3: a deceased person shows
// age_at_death and no running age.
func TestPersonDerived_AgeAtDeathReplacesAge(t *testing.T) {
	srv, pid := personDerivedServer(t, "", map[string][]string{
		"birthdate": {"1950-01-01"}, "deathdate": {"1999-06-15"},
	})
	resolved := getResolved(t, srv, pid)

	if _, ok := findField(resolved, "age"); ok {
		t.Error("a deceased person must not have a running age row")
	}
	aad, ok := findField(resolved, "age_at_death")
	if !ok {
		t.Fatal("want an age_at_death row")
	}
	if v := aad["values"].([]any); v[0] != "49" {
		t.Errorf("age_at_death = %v, want 49", aad["values"])
	}
	df, _ := aad["derived_from"].([]any)
	if len(df) != 2 || df[0] != "Born" || df[1] != "Died" {
		t.Errorf("derived_from = %v, want [Born Died]", aad["derived_from"])
	}
}

// TestPersonDerived_MissingBirthdateNoRow covers AC-4/AC-5: without a (parseable)
// birthdate, neither derived row appears — for owner and visitor alike.
func TestPersonDerived_MissingBirthdateNoRow(t *testing.T) {
	for _, bd := range []map[string][]string{nil, {"birthdate": {"unknown"}}} {
		srv, pid := personDerivedServer(t, "", bd)
		resolved := getResolved(t, srv, pid)
		if _, ok := findField(resolved, "age"); ok {
			t.Errorf("fields=%v: want no age row", bd)
		}
		if _, ok := findField(resolved, "age_at_death"); ok {
			t.Errorf("fields=%v: want no age_at_death row", bd)
		}
	}
}

// TestPersonDerived_ComputedDecisionRejected covers ADR-063 §D3: a decision naming a
// computed canonical is rejected 400 — a derived field is never adoptable.
func TestPersonDerived_ComputedDecisionRejected(t *testing.T) {
	srv, pid := personDerivedServer(t, "", map[string][]string{"birthdate": {"1990-03-14"}})
	url := srv.URL + "/api/v1/people/" + itoa(pid) + "/fields/age/decision"
	if code := sendDecision(t, http.MethodPut, url, "", map[string]string{"source": "record"}); code != http.StatusBadRequest {
		t.Fatalf("pin computed field: want 400, got %d", code)
	}
}

// getResolved returns GET /people/{id}'s resolved[] as a slice of field maps.
func getResolved(t *testing.T, srv *httptest.Server, id int64) []map[string]any {
	t.Helper()
	code, body := getJSON(t, srv.URL+"/api/v1/people/"+itoa(id))
	if code != http.StatusOK {
		t.Fatalf("get person = %d", code)
	}
	raw, _ := body["resolved"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, f := range raw {
		if m, ok := f.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func findField(fields []map[string]any, canonical string) (map[string]any, bool) {
	for _, f := range fields {
		if f["canonical"] == canonical {
			return f, true
		}
	}
	return nil, false
}

func indexOf(fields []map[string]any, canonical string) int {
	for i, f := range fields {
		if f["canonical"] == canonical {
			return i
		}
	}
	return -1
}
