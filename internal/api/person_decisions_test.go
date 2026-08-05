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

// personDecisionServer wires the F37 person source-of-truth surface over a real
// repo: a person ("Alice") with a matched `tmdb` provider supplying name, the
// scalar fields, and the aliases merge field. token="" leaves the gate open.
func personDecisionServer(t *testing.T, token string) (*httptest.Server, *repo.Repo, int64) {
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
		People:    []model.Person{{Name: "Alice"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	linkPeople(t, r, vid, "Alice")
	pid, _, err := r.PersonIDByName(ctx, "Alice")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, pid, "tmdb", "ext-9", map[string][]string{
		"name":        {"Alicia Example"},
		"bio":         {"A biography."},
		"nationality": {"Utopia"},
		"aliases":     {"Ally", "Al"},
	}); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}

	h := api.NewHandlers(r, log, nil, filepath.Join(dir, "thumbnails"), nil, nil)
	h.SetAuth(api.NewAuth(token), false)
	srv := httptest.NewServer(api.Router(log, api.NewHealth(), h, nil))
	t.Cleanup(srv.Close)
	return srv, r, pid
}

// personResolvedField pulls one canonical field out of GET /people/{id}'s
// resolved array; ok=false when the field is absent (a decided-empty field drops).
func personResolvedField(t *testing.T, srv *httptest.Server, id int64, canonical string) (map[string]any, bool) {
	t.Helper()
	body := getPersonBody(t, srv, id)
	raw, _ := body["resolved"].([]any)
	for _, f := range raw {
		m, _ := f.(map[string]any)
		if m["canonical"] == canonical {
			return m, true
		}
	}
	return nil, false
}

func getPersonBody(t *testing.T, srv *httptest.Server, id int64) map[string]any {
	t.Helper()
	code, body := getJSON(t, srv.URL+"/api/v1/people/"+itoa(id))
	if code != http.StatusOK {
		t.Fatalf("get person = %d", code)
	}
	return body
}

// --- Payload shape (P0-2 / QA 2.7) --------------------------------------------------

func TestPersonResolvedPayloadShape(t *testing.T) {
	srv, _, pid := personDecisionServer(t, "")
	body := getPersonBody(t, srv, pid)

	// enriched[] is retired; resolved[] replaces it.
	if _, ok := body["enriched"]; ok {
		t.Error("person payload must not carry enriched[] (retired by F37)")
	}
	raw, _ := body["resolved"].([]any)
	if len(raw) == 0 {
		t.Fatal("person payload must carry resolved[]")
	}
	for _, f := range raw {
		m, _ := f.(map[string]any)
		if _, ok := m["in_sync"]; ok {
			t.Errorf("person resolved field %v must not carry in_sync", m["canonical"])
		}
	}

	// The record vocabulary: name resolves from the record, baseline candidate
	// first, provider spelling as an adoptable candidate.
	name, _ := personResolvedField(t, srv, pid, "name")
	if name["values"].([]any)[0] != "Alice" || name["winning_source"] != "record:name" {
		t.Errorf("name should resolve from the record: %v", name)
	}
	if dec := name["decision"].(map[string]any); dec["source"] != "record" || dec["standing"] != false {
		t.Errorf("undecided name marker should be non-standing record, got %v", dec)
	}
	cands := name["candidates"].([]any)
	if first := cands[0].(map[string]any); first["source"] != "record" || first["value"] != "Alice" {
		t.Errorf("record candidate should anchor first: %v", cands)
	}

	// RD6 additivity at the API layer: undecided enrichment-only fields show the
	// raw provider values.
	bio, _ := personResolvedField(t, srv, pid, "bio")
	if bio["values"].([]any)[0] != "A biography." || bio["winning_source"] != "tmdb:bio" {
		t.Errorf("undecided bio must show the provider value: %v", bio)
	}
	aliases, _ := personResolvedField(t, srv, pid, "aliases")
	if vals := aliases["values"].([]any); len(vals) != 2 || aliases["multi"] != true {
		t.Errorf("aliases should be the provider union merge field: %v", aliases)
	}
}

// --- Decisions (P0-3 / QA 2.3, 2.4) --------------------------------------------------

func TestPersonDecisionAPI_RecordManualRoundTrip(t *testing.T) {
	srv, r, pid := personDecisionServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid) + "/fields/bio/decision"

	// Record blank-pin: the payload says "record", the store keeps the internal
	// "file" token (the RD4 edge mapping), and the provider bio is suppressed —
	// but the field stays in the resolved set with no values (RD3), carrying the
	// standing record decision and its candidates so the pin can be re-decided.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": "record"}); code != 204 {
		t.Fatalf("record pin: want 204, got %d", code)
	}
	rows, err := r.DecisionsForEntity(context.Background(), model.EnrichEntityPerson, pid)
	if err != nil || len(rows) != 1 || rows[0].Source != "file" || rows[0].FieldKey != "bio" {
		t.Fatalf("stored decision should keep the internal file token: %+v (%v)", rows, err)
	}
	if f, ok := personResolvedField(t, srv, pid, "bio"); !ok {
		t.Fatal("record blank-pin must keep the field visible")
	} else {
		if vals := f["values"].([]any); len(vals) != 0 {
			t.Fatalf("blank-pinned bio must carry no values, got %v", vals)
		}
		dec := f["decision"].(map[string]any)
		if dec["source"] != "record" || dec["standing"] != true {
			t.Errorf("want standing record decision in person vocabulary, got %v", dec)
		}
		if len(f["candidates"].([]any)) == 0 {
			t.Errorf("blank-pinned bio must keep candidates, got %v", f)
		}
	}

	// The literal internal token is not person vocabulary.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": "file"}); code != 400 {
		t.Errorf("literal 'file': want 400, got %d", code)
	}

	// Manual literal round-trip.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": "manual", "manual_value": "My words."}); code != 204 {
		t.Fatalf("manual: want 204, got %d", code)
	}
	f, _ := personResolvedField(t, srv, pid, "bio")
	if f["values"].([]any)[0] != "My words." {
		t.Errorf("manual: want My words., got %v", f["values"])
	}
	if dec := f["decision"].(map[string]any); dec["source"] != "manual" || dec["manual_value"] != "My words." {
		t.Errorf("decision marker = %v", dec)
	}

	// Adopt the provider, then clear back to the record-first default.
	if code := sendDecision(t, http.MethodPut, base, "", map[string]string{"source": "provider:tmdb"}); code != 204 {
		t.Fatalf("adopt provider: want 204, got %d", code)
	}
	f, _ = personResolvedField(t, srv, pid, "bio")
	if dec := f["decision"].(map[string]any); dec["source"] != "provider:tmdb" || dec["standing"] != true {
		t.Errorf("standing provider marker = %v", dec)
	}
	if code := sendDecision(t, http.MethodDelete, base, "", nil); code != 204 {
		t.Fatalf("clear: want 204, got %d", code)
	}
	f, _ = personResolvedField(t, srv, pid, "bio")
	if f["values"].([]any)[0] != "A biography." || f["decision"].(map[string]any)["standing"] != false {
		t.Errorf("after clear: want undecided provider value, got %v", f)
	}
}

func TestPersonDecisionAPI_Validation(t *testing.T) {
	srv, _, pid := personDecisionServer(t, "")
	fields := srv.URL + "/api/v1/people/" + itoa(pid) + "/fields/"

	// name never pins (RD1) — the rename flow is the only name mutation.
	if code := sendDecision(t, http.MethodPut, fields+"name/decision", "", map[string]string{"source": "provider:tmdb"}); code != 400 {
		t.Errorf("name decision: want 400, got %d", code)
	}
	if code := sendDecision(t, http.MethodDelete, fields+"name/decision", "", nil); code != 400 {
		t.Errorf("name decision clear: want 400, got %d", code)
	}
	// aliases is merge-only.
	if code := sendDecision(t, http.MethodPut, fields+"aliases/decision", "", map[string]string{"source": "provider:tmdb"}); code != 400 {
		t.Errorf("merge field: want 400, got %d", code)
	}
	// Bad source shape / manual without a value.
	if code := sendDecision(t, http.MethodPut, fields+"bio/decision", "", map[string]string{"source": "bogus"}); code != 400 {
		t.Errorf("bad source: want 400, got %d", code)
	}
	if code := sendDecision(t, http.MethodPut, fields+"bio/decision", "", map[string]string{"source": "manual"}); code != 400 {
		t.Errorf("manual w/o value: want 400, got %d", code)
	}
	// Unmatched provider.
	if code := sendDecision(t, http.MethodPut, fields+"bio/decision", "", map[string]string{"source": "provider:imdb"}); code != 400 {
		t.Errorf("unmatched provider: want 400, got %d", code)
	}
	// Unknown field / unknown person.
	if code := sendDecision(t, http.MethodPut, fields+"nope/decision", "", map[string]string{"source": "record"}); code != 404 {
		t.Errorf("unknown field: want 404, got %d", code)
	}
	if code := sendDecision(t, http.MethodPut, srv.URL+"/api/v1/people/99999/fields/bio/decision", "", map[string]string{"source": "record"}); code != 404 {
		t.Errorf("unknown person: want 404, got %d", code)
	}
}

func TestPersonDecisionAPI_OwnerGated(t *testing.T) {
	srv, _, pid := personDecisionServer(t, "s3cret")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	// Every F37 mutation 401s without the owner token.
	if code := sendDecision(t, http.MethodPut, base+"/fields/bio/decision", "", map[string]string{"source": "record"}); code != 401 {
		t.Errorf("PUT decision without token: want 401, got %d", code)
	}
	if code := sendDecision(t, http.MethodDelete, base+"/fields/bio/decision", "", nil); code != 401 {
		t.Errorf("DELETE decision without token: want 401, got %d", code)
	}
	if code, _ := postTok(t, base+"/curation", "", map[string]string{"field": "aliases", "value": "X", "action": "suppress"}); code != 401 {
		t.Errorf("curation without token: want 401, got %d", code)
	}
	if code, _ := postTok(t, base+"/rename", "", map[string]string{"name": "Eve"}); code != 401 {
		t.Errorf("rename without token: want 401, got %d", code)
	}
	// With the token they succeed.
	if code := sendDecision(t, http.MethodPut, base+"/fields/bio/decision", "s3cret", map[string]string{"source": "record"}); code != 204 {
		t.Errorf("PUT decision with token: want 204, got %d", code)
	}
}

// --- Curation (P0-4 / RD2) -----------------------------------------------------------

func TestPersonCurationAPI_AliasesMergeField(t *testing.T) {
	srv, _, pid := personDecisionServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	// Suppress a provider alias, add a manual one.
	if code, _ := postTok(t, base+"/curation", "", map[string]string{"field": "aliases", "value": "Al", "action": "suppress"}); code != 204 {
		t.Fatalf("suppress: want 204, got %d", code)
	}
	if code, _ := postTok(t, base+"/curation", "", map[string]string{"field": "aliases", "value": "Ace", "action": "add"}); code != 204 {
		t.Fatalf("add: want 204, got %d", code)
	}
	f, _ := personResolvedField(t, srv, pid, "aliases")
	var vals []string
	for _, v := range f["values"].([]any) {
		vals = append(vals, v.(string))
	}
	if len(vals) != 2 || vals[0] != "Ally" || vals[1] != "Ace" {
		t.Errorf("curated aliases = %v, want [Ally Ace]", vals)
	}

	// Clear restores the suppressed value.
	if code, _ := postTok(t, base+"/curation/clear", "", map[string]string{"field": "aliases", "value": "Al", "action": "suppress"}); code != 204 {
		t.Fatalf("clear: want 204, got %d", code)
	}
	f, _ = personResolvedField(t, srv, pid, "aliases")
	if got := f["values"].([]any); len(got) != 3 {
		t.Errorf("after clear aliases = %v, want 3 values", got)
	}

	// Validation mirrors the media handler; unknown person 404s.
	if code, _ := postTok(t, base+"/curation", "", map[string]string{"field": "", "value": "x", "action": "add"}); code != 400 {
		t.Errorf("missing field: want 400, got %d", code)
	}
	if code, _ := postTok(t, srv.URL+"/api/v1/people/99999/curation", "", map[string]string{"field": "aliases", "value": "x", "action": "add"}); code != 404 {
		t.Errorf("unknown person: want 404, got %d", code)
	}
}

// --- Rename (P0-5 / RD1 / QA 2.5) ----------------------------------------------------

func TestPersonRename_HappyPath(t *testing.T) {
	srv, r, pid := personDecisionServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	if code, _ := postTok(t, base+"/rename", "", map[string]string{"name": "  Alicia Example  "}); code != 204 {
		t.Fatalf("rename: want 204, got %d", code)
	}
	body := getPersonBody(t, srv, pid)
	person, _ := body["person"].(map[string]any)
	if person["name"] != "Alicia Example" {
		t.Errorf("renamed person = %v", person["name"])
	}
	// The old name is kept as an F23 alias…
	aliases, _ := person["aliases"].([]any)
	if len(aliases) != 1 || aliases[0].(map[string]any)["alias"] != "Alice" {
		t.Errorf("old name should be an alias: %v", person["aliases"])
	}
	// …and stays searchable (alias FTS routing).
	res, err := r.Search(context.Background(), "Alice", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, p := range res.People {
		if p.ID == pid {
			found = true
		}
	}
	if !found {
		t.Error("old name must remain search-matchable after rename")
	}
	// The record chip carries the new name; the provider chip now matches it too.
	name, _ := personResolvedField(t, srv, pid, "name")
	if name["values"].([]any)[0] != "Alicia Example" {
		t.Errorf("resolved name = %v", name["values"])
	}
}

func TestPersonRename_CollisionAndNoOp(t *testing.T) {
	srv, r, pid := personDecisionServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/y.mkv", FileSize: 1, Title: "Y", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Bob"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed second person: %v", err)
	}
	linkPeople(t, r, vid, "Bob")
	bob, _, _ := r.PersonIDByName(ctx, "Bob")

	// Collision: 409 with the existing person (never an auto-merge), no mutation.
	code, body := postTok(t, base+"/rename", "", map[string]string{"name": "Bob"})
	if code != http.StatusConflict {
		t.Fatalf("colliding rename = %d, want 409", code)
	}
	conflict, _ := body["conflict"].(map[string]any)
	if conflict == nil || int64(conflict["id"].(float64)) != bob || conflict["name"] != "Bob" {
		t.Errorf("409 conflict payload = %v, want Bob (#%d)", body["conflict"], bob)
	}
	if _, ok := conflict["video_count"]; !ok {
		t.Errorf("conflict must carry the video count for the merge offer: %v", conflict)
	}
	person, _ := getPersonBody(t, srv, pid)["person"].(map[string]any)
	aliases, _ := person["aliases"].([]any)
	if person["name"] != "Alice" || len(aliases) != 0 {
		t.Errorf("failed rename must not mutate: %v", person)
	}

	// No-op: renaming to the current name succeeds without creating an alias.
	if code, _ := postTok(t, base+"/rename", "", map[string]string{"name": "Alice"}); code != 204 {
		t.Errorf("no-op rename = %d, want 204", code)
	}
	person, _ = getPersonBody(t, srv, pid)["person"].(map[string]any)
	if aliases, _ := person["aliases"].([]any); len(aliases) != 0 {
		t.Errorf("no-op rename must not add an alias: %v", person["aliases"])
	}

	// Validation: empty name.
	if code, _ := postTok(t, base+"/rename", "", map[string]string{"name": "   "}); code != 400 {
		t.Errorf("empty rename = %d, want 400", code)
	}
	// Unknown person.
	if code, _ := postTok(t, srv.URL+"/api/v1/people/99999/rename", "", map[string]string{"name": "Zed"}); code != 404 {
		t.Errorf("unknown person rename = %d, want 404", code)
	}
}

// --- Merge cleanup (P0-6 / RD5 / QA 2.6) ---------------------------------------------

func TestPersonMerge_DropsDecisionsAndCuration(t *testing.T) {
	srv, r, alice := personDecisionServer(t, "")
	ctx := context.Background()

	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/y.mkv", FileSize: 1, Title: "Y", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Bob"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed second person: %v", err)
	}
	linkPeople(t, r, vid, "Bob")
	bob, _, _ := r.PersonIDByName(ctx, "Bob")

	// Rows on both sides: the canonical (Alice) keeps hers, Bob's are dropped.
	for _, pid := range []int64{alice, bob} {
		if err := r.SetDecision(ctx, model.EnrichEntityPerson, pid, "bio", "manual", "kept?"); err != nil {
			t.Fatalf("seed decision: %v", err)
		}
		if err := r.SetCuration(ctx, model.EnrichEntityPerson, pid, "aliases", "Zed", "suppress"); err != nil {
			t.Fatalf("seed curation: %v", err)
		}
	}

	if code, _ := postTok(t, srv.URL+"/api/v1/people/"+itoa(alice)+"/merge", "", map[string]int64{"from_id": bob}); code != 200 {
		t.Fatalf("merge: want 200, got %d", code)
	}

	if rows, _ := r.DecisionsForEntity(ctx, model.EnrichEntityPerson, bob); len(rows) != 0 {
		t.Errorf("merged-away decisions must be dropped: %+v", rows)
	}
	if rows, _ := r.CurationForEntity(ctx, model.EnrichEntityPerson, bob); len(rows) != 0 {
		t.Errorf("merged-away curation must be dropped: %+v", rows)
	}
	if rows, _ := r.DecisionsForEntity(ctx, model.EnrichEntityPerson, alice); len(rows) != 1 {
		t.Errorf("canonical decisions must survive: %+v", rows)
	}
	if rows, _ := r.CurationForEntity(ctx, model.EnrichEntityPerson, alice); len(rows) != 1 {
		t.Errorf("canonical curation must survive: %+v", rows)
	}
}
