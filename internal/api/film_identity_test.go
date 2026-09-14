package api_test

import (
	"context"
	"net/http"
	"testing"

	"holodex/internal/model"
)

// Film alias / merge / rename / near-miss over the shared identity spine (HOLODEX-376,
// ADR-096 D3): the studio/tag route config's film branch, mounted under the films
// gate, with the composite (title, year) key behind the rename verdict.

func TestFilmIdentityEndpoints(t *testing.T) {
	srv, r, v1, v2 := filmServer(t, "s3cret")
	ctx := context.Background()
	s1980, err := r.CreateFilm(ctx, "Superman II", 1980)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	s2006, err := r.CreateFilm(ctx, "Superman II", 2006)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	base := srv.URL + "/api/v1/films/" + itoa(s1980)

	// Gating, then a provider-free alias add that the detail page carries.
	if code, _ := postTok(t, base+"/aliases", "", map[string]string{"alias": "Superman 2"}); code != http.StatusUnauthorized {
		t.Errorf("no-token add = %d, want 401", code)
	}
	code, body := postTok(t, base+"/aliases", "s3cret", map[string]string{"alias": "  Superman 2 "})
	if code != http.StatusOK {
		t.Fatalf("add alias = %d %v, want 200", code, body)
	}
	if list := aliasList(t, body); len(list) != 1 || list[0]["alias"] != "Superman 2" {
		t.Fatalf("aliases = %v", body["aliases"])
	}
	code, detail := getJSON(t, base)
	if code != http.StatusOK {
		t.Fatalf("detail = %d", code)
	}
	film, _ := detail["film"].(map[string]any)
	if aliases, _ := film["aliases"].([]any); len(aliases) != 1 {
		t.Errorf("detail film.aliases = %v, want the added alias", film["aliases"])
	}
	// An alias that is another film's title is a conflict the owner must resolve.
	if code, body := postTok(t, base+"/aliases", "s3cret", map[string]string{"alias": "superman ii"}); code != http.StatusConflict || body["conflict"] == nil {
		t.Errorf("colliding alias = %d %v, want 409 with conflict", code, body)
	}

	// Rename: same title under the other film's year is free; the same year collides
	// with a 409 naming the occupant (spec RD5 — the year itself is untouched).
	if code, _ := postTok(t, base+"/rename", "s3cret", map[string]string{"name": "Superman II: The Richard Donner Cut"}); code != http.StatusNoContent {
		t.Fatalf("rename = %d, want 204", code)
	}
	if code, _ := postTok(t, srv.URL+"/api/v1/films/"+itoa(s2006)+"/rename", "s3cret", map[string]string{"name": "Superman 2"}); code != http.StatusNoContent {
		t.Fatalf("rename onto an alias of a same-title/other-year film = %d, want 204 (composite key)", code)
	}
	free, err := r.CreateFilm(ctx, "Superman Returns", 2006)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	code, body = postTok(t, srv.URL+"/api/v1/films/"+itoa(free)+"/rename", "s3cret", map[string]string{"name": "superman 2"})
	if code != http.StatusConflict {
		t.Fatalf("rename onto (Superman 2, 2006) = %d, want 409", code)
	}
	conflict, _ := body["conflict"].(map[string]any)
	if int64(conflict["id"].(float64)) != s2006 {
		t.Errorf("conflict = %v, want the 2006 film", body["conflict"])
	}
	_, detail = getJSON(t, base)
	film, _ = detail["film"].(map[string]any)
	if film["name"] != "Superman II: The Richard Donner Cut" || film["year"] != float64(1980) {
		t.Errorf("after rename = %v/%v, want the new title with the year untouched", film["name"], film["year"])
	}

	// Near-miss look-alike for the owner's title control (case-fold/punctuation).
	if code, body := getJSONTok(t, base+"/near-miss?name=superman%20returns!", "s3cret"); code != http.StatusOK || body["near_miss"] == nil {
		t.Errorf("near-miss = %d %v, want 200 with a match", code, body)
	}

	// Merge: the loser's scenes and title follow the survivor.
	one, two := int64(1), int64(2)
	if _, err := r.AttachFilmVideo(ctx, s1980, v1, &one, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, free, v2, &two, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	code, body = postTok(t, base+"/merge", "s3cret", map[string]int64{"from_id": free})
	if code != http.StatusOK {
		t.Fatalf("merge = %d %v, want 200", code, body)
	}
	film, _ = body["film"].(map[string]any)
	if film["name"] != "Superman II: The Richard Donner Cut" {
		t.Errorf("merge returned %v, want the survivor", body["film"])
	}
	fvs, err := r.FilmVideos(ctx, s1980)
	if err != nil || len(fvs) != 2 {
		t.Errorf("survivor videos after merge = %d (%v), want 2", len(fvs), err)
	}
	// Added alias + the pre-rename title + the loser's title.
	if aliases := aliasList(t, map[string]any{"aliases": film["aliases"]}); len(aliases) != 3 || aliases[2]["alias"] != "Superman Returns" {
		t.Errorf("survivor aliases after merge = %v, want the loser's title alongside the two earlier ones", film["aliases"])
	}

	// The Duplicates tab accepts film pairs (dismiss = keep separate).
	code, _ = postTok(t, srv.URL+"/api/v1/owner/duplicates/dismiss", "s3cret",
		map[string]any{"entity_type": model.EnrichEntityFilm, "id_a": s1980, "id_b": s2006})
	if code != http.StatusNoContent {
		t.Errorf("dismiss film pair = %d, want 204", code)
	}
	if kept, err := r.IsKeptSeparate(ctx, model.EnrichEntityFilm, s1980, s2006); err != nil || !kept {
		t.Errorf("kept separate = (%v, %v), want true", kept, err)
	}
}

// The film identity routes live under the films gate like every other film route.
func TestFilmIdentityRoutes_GatedByFilmsEnabled(t *testing.T) {
	srv, _ := identityServer(t, "s3cret") // films_enabled left off
	if code, _ := postTok(t, srv.URL+"/api/v1/films/1/rename", "s3cret", map[string]string{"name": "x"}); code != http.StatusNotFound {
		t.Errorf("rename with films_enabled=false = %d, want 404", code)
	}
	if code, _ := getJSONTok(t, srv.URL+"/api/v1/films/1/near-miss?name=x", "s3cret"); code != http.StatusNotFound {
		t.Errorf("near-miss with films_enabled=false = %d, want 404", code)
	}
}
