package api_test

import (
	"context"
	"net/http"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// billedNames pulls the billed-absent list and total out of GET /films/{id}.
func billedCast(t *testing.T, srv string, fid int64) ([]string, []float64, int) {
	t.Helper()
	code, body := getJSON(t, srv+"/api/v1/films/"+itoa(fid))
	if code != http.StatusOK {
		t.Fatalf("film detail code = %d", code)
	}
	names := []string{}
	ids := []float64{}
	for _, raw := range body["billed_absent"].([]any) {
		m := raw.(map[string]any)
		names = append(names, m["name"].(string))
		if v, ok := m["person_id"].(float64); ok {
			ids = append(ids, v)
		} else {
			ids = append(ids, 0)
		}
	}
	total, _ := body["billed_total"].(float64)
	return names, ids, int(total)
}

// seedBilled writes a provider `actors` row straight into the enrichment shadow — the
// same place a real film enrich lands it (verified against the live testbed). Going
// through the shadow rather than a fake provider keeps these tests about the difference
// rule itself rather than about the enrich plumbing, which enrich_test.go already covers.
func seedBilled(t *testing.T, r *repo.Repo, fid int64, names ...string) {
	t.Helper()
	if err := r.UpsertEnrichment(context.Background(), model.EnrichEntityFilm, fid, "fake", "tmdb:1",
		map[string][]string{"actors": names}); err != nil {
		t.Fatalf("seed billed cast: %v", err)
	}
}

// The core rule (ADR-089 D2): the page shows the union, then only its COMPLEMENT.
// The arithmetic IS the assertion — a naive implementation returns the whole billed
// list, and that bug is invisible until a real film renders.
func TestFilmBilledCast_OnlyTheComplement(t *testing.T) {
	srv, r, fid := filmYearServer(t, "Dune")
	ctx := context.Background()

	// Two performers who really are in a scene of this film...
	vid, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/dune-s1.mkv", FileSize: 1, Title: "Dune scene"}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	linkPeopleAs(t, r, vid, "actor", "Timothée Chalamet", "Rebecca Ferguson")
	if _, err := r.AttachFilmVideo(ctx, fid, vid, nil, false); err != nil {
		t.Fatalf("attach: %v", err)
	}

	// ...against a billed list that also names two who are not.
	seedBilled(t, r, fid, "Timothée Chalamet", "Rebecca Ferguson", "Oscar Isaac", "Jason Momoa")

	names, _, total := billedCast(t, srv.URL, fid)
	if total != 4 {
		t.Fatalf("billed_total = %d, want 4", total)
	}
	if len(names) != 2 {
		t.Fatalf("billed_absent = %v, want exactly the 2 not in any scene — not the whole billed list", names)
	}
	got := map[string]bool{names[0]: true, names[1]: true}
	if !got["Oscar Isaac"] || !got["Jason Momoa"] {
		t.Errorf("billed_absent = %v, want Oscar Isaac + Jason Momoa", names)
	}
	for _, n := range names {
		if n == "Timothée Chalamet" || n == "Rebecca Ferguson" {
			t.Errorf("%q is in a scene and must never appear twice on the page", n)
		}
	}
}

// D2's false-positive guard, and the reason matching goes through the identity spine
// rather than comparing strings: telling an owner their COMPLETE rip is incomplete is
// worse than quietly omitting one genuinely missing name.
func TestFilmBilledCast_MatchesByIdentityNotString(t *testing.T) {
	srv, r, fid := filmYearServer(t, "Dune")
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/dune-s2.mkv", FileSize: 1, Title: "Dune scene"}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	linkPeopleAs(t, r, vid, "actor", "Timothée Chalamet")
	if _, err := r.AttachFilmVideo(ctx, fid, vid, nil, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	pid, ok, err := r.PersonIDByName(ctx, "Timothée Chalamet")
	if err != nil || !ok {
		t.Fatalf("person id: %v ok=%v", err, ok)
	}
	if _, err := r.AddPersonAlias(ctx, pid, "Timmy C"); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// A case variant and an alias must BOTH count as covered.
	seedBilled(t, r, fid, "timothée chalamet", "Timmy C", "Oscar Isaac")

	names, _, total := billedCast(t, srv.URL, fid)
	// Two of those three names are the SAME person reached two ways, so they collapse
	// to one billed credit — the identity dedupe TestFilmBilledCast_DegenerateShapes
	// pins directly. Counting 3 here would mean the spine had failed to connect them.
	if total != 2 {
		t.Fatalf("billed_total = %d, want 2 — the case variant and the alias are one person", total)
	}
	if len(names) != 1 || names[0] != "Oscar Isaac" {
		t.Fatalf("billed_absent = %v, want only Oscar Isaac — a case variant or alias reported as "+
			"missing tells the owner a complete rip is incomplete", names)
	}
}

// A billed name already in the library (just not in THIS film's scenes) gets a link;
// one that names nobody stays inert text. The second half is the load-bearing one:
// D1 was amended specifically so that provider cast creates no Person rows.
func TestFilmBilledCast_LinksKnownPeopleAndCreatesNone(t *testing.T) {
	srv, r, fid := filmYearServer(t, "Dune")
	ctx := context.Background()

	// Someone in the library via a different, unattached video.
	other, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/other.mkv", FileSize: 1, Title: "Other"}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	linkPeopleAs(t, r, other, "actor", "Oscar Isaac")
	oscarID, ok, err := r.PersonIDByName(ctx, "Oscar Isaac")
	if err != nil || !ok {
		t.Fatalf("oscar id: %v ok=%v", err, ok)
	}

	before, err := r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list people: %v", err)
	}

	seedBilled(t, r, fid, "Oscar Isaac", "Nobody At All")

	names, ids, total := billedCast(t, srv.URL, fid)
	if total != 2 || len(names) != 2 {
		t.Fatalf("billed = %v (total %d), want both absent", names, total)
	}
	for i, n := range names {
		switch n {
		case "Oscar Isaac":
			if int64(ids[i]) != oscarID {
				t.Errorf("Oscar Isaac person_id = %v, want %d so the chip can link", ids[i], oscarID)
			}
		case "Nobody At All":
			if ids[i] != 0 {
				t.Errorf("unknown name got person_id %v — it must stay inert text", ids[i])
			}
		}
	}

	// The invariant D1 was amended for: reading the page creates nobody.
	after, err := r.ListPeople(ctx, false)
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("people count %d → %d: rendering billed cast must create no Person rows, or "+
			"/people fills with performers the library holds no footage of", len(before), len(after))
	}
}

// Degenerate shapes, each asserted so a later reader cannot conflate them.
func TestFilmBilledCast_DegenerateShapes(t *testing.T) {
	srv, r, fid := filmYearServer(t, "Dune")
	ctx := context.Background()

	// No provider cast at all → the whole block is absent from the payload.
	if names, _, total := billedCast(t, srv.URL, fid); len(names) != 0 || total != 0 {
		t.Fatalf("unenriched film reported billed cast %v (total %d) — its Cast section must "+
			"render exactly as it did before this feature", names, total)
	}

	// Every billed name covered → an empty difference, and a total that still counts.
	vid, err := r.UpsertVideo(ctx, &model.Video{FilePath: "/m/all.mkv", FileSize: 1, Title: "All"}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	linkPeopleAs(t, r, vid, "actor", "Oscar Isaac")
	if _, err := r.AttachFilmVideo(ctx, fid, vid, nil, false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	seedBilled(t, r, fid, "Oscar Isaac")
	if names, _, total := billedCast(t, srv.URL, fid); len(names) != 0 || total != 1 {
		t.Fatalf("fully covered film = %v (total %d), want no absences but a total of 1 so the "+
			"page can say \"all 1 billed cast\"", names, total)
	}

	// The same person billed twice under two spellings collapses to one entry.
	seedBilled(t, r, fid, "Oscar Isaac", "oscar isaac", "Jason Momoa")
	names, _, total := billedCast(t, srv.URL, fid)
	if total != 2 {
		t.Fatalf("billed_total = %d, want 2 — two spellings of one person are one billed credit", total)
	}
	if len(names) != 1 || names[0] != "Jason Momoa" {
		t.Fatalf("billed_absent = %v, want only Jason Momoa", names)
	}
}
