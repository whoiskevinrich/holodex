package enrich

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// fullHint is what internal/api builds before the gate: every structured key set.
func fullHint() Hint {
	return Hint{
		Query:       "Acme Pictures Ada Lovelace",
		ExternalIDs: []string{"imdb:tt1"},
		Fields: map[string][]string{
			"title":  {"[Acme Pictures] Ada Lovelace (2023-08-01) 1080p"},
			"studio": {"Acme Pictures"}, "actors": {"Ada Lovelace", "Grace Hopper"},
			"director": {"Alan Turing"}, "release_date": {"2023-08-01"},
			// Never on the wire, however the provider advertises (security review):
			"overview": {"the owner's own Comment tag"}, "homepage": {"https://x"},
			"external_provider_id": {"other:1"}, "poster_url": {"https://p"},
		},
		Filename:    "[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4",
		QuerySource: QuerySourcePattern,
	}
}

// ADR-095 D1: the manifest opt-in decides which structured keys survive.
func TestGateHint_ManifestOptIn(t *testing.T) {
	allFive := []string{"title", "studio", "actors", "director", "release_date"}
	cases := []struct {
		name         string
		hints        []string
		advertised   []string
		src          Source
		wantFields   []string // keys expected in Fields, nil = key absent
		wantFilename bool
		wantSource   bool
	}{
		{"no resolve_hints: nothing structured", nil, allFive, Source{}, nil, false, false},
		{"fields only", []string{"fields"}, allFive, Source{}, allFive, false, true},
		{"filename only", []string{"filename"}, allFive, Source{}, nil, true, true},
		{"both", []string{"fields", "filename"}, allFive, Source{}, allFive, true, true},
		{"unknown entries ignored, known honored", []string{"person", "FILENAME "}, allFive, Source{}, nil, true, true},
		{"fields ∩ advertised: a video with actors, a provider advertising title+studio", []string{"fields"}, []string{"Title", "studio"}, Source{}, []string{"title", "studio"}, false, true},
		{"fields bounded to the five even when the provider advertises all of §4.2a", []string{"fields"}, append(allFive, "overview", "homepage", "external_provider_id", "poster_url", "tagline"), Source{}, allFive, false, true},
		{"advertises nothing in the vocabulary: fields key absent, query_source still rides the opt-in", []string{"fields"}, []string{"overview"}, Source{}, nil, false, true},
		{"operator deny withholds filename, fields unaffected", []string{"fields", "filename"}, allFive, Source{SendFilename: boolPtr(false)}, allFive, false, true},
		{"explicit send_filename: true is the same as unset", []string{"filename"}, allFive, Source{SendFilename: boolPtr(true)}, nil, true, true},
		{"deny is filename-only: a fields opt-in still sends fields", []string{"fields"}, allFive, Source{SendFilename: boolPtr(false)}, allFive, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := gateHint(fullHint(), c.src, Manifest{ResolveHints: c.hints, Fields: c.advertised})
			if got.Query != "Acme Pictures Ada Lovelace" || len(got.ExternalIDs) != 1 {
				t.Errorf("query/external_ids must pass through untouched: %+v", got)
			}
			if c.wantFields == nil {
				if got.Fields != nil {
					t.Errorf("Fields = %v, want absent", got.Fields)
				}
			} else {
				if len(got.Fields) != len(c.wantFields) {
					t.Errorf("Fields keys = %v, want %v", keys(got.Fields), c.wantFields)
				}
				for _, k := range c.wantFields {
					if _, ok := got.Fields[k]; !ok {
						t.Errorf("Fields missing %q: %v", k, got.Fields)
					}
				}
				for _, k := range []string{"overview", "homepage", "external_provider_id", "poster_url", "tagline"} {
					if _, ok := got.Fields[k]; ok {
						t.Errorf("Fields must never carry %q", k)
					}
				}
			}
			if (got.Filename != "") != c.wantFilename {
				t.Errorf("Filename = %q, want present=%v", got.Filename, c.wantFilename)
			}
			if (got.QuerySource != "") != c.wantSource {
				t.Errorf("QuerySource = %q, want present=%v", got.QuerySource, c.wantSource)
			}
		})
	}
}

// Values are sent AS-IS: no sanitizer on fields (brackets survive), every value of
// a multi-valued field (no performersCap), and the filename verbatim with extension,
// brackets, parens and commas intact.
func TestGateHint_ValuesAsIs(t *testing.T) {
	got := gateHint(fullHint(), Source{}, Manifest{
		ResolveHints: []string{"fields", "filename"},
		Fields:       []string{"title", "actors"},
	})
	if want := "[Acme Pictures] Ada Lovelace (2023-08-01) 1080p"; got.Fields["title"][0] != want {
		t.Errorf("fields.title = %q, want the unsanitized %q", got.Fields["title"][0], want)
	}
	if len(got.Fields["actors"]) != 2 {
		t.Errorf("fields.actors = %v, want every value", got.Fields["actors"])
	}
	if want := "[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4"; got.Filename != want {
		t.Errorf("filename = %q, want verbatim %q", got.Filename, want)
	}
}

// Golden (ADR-095 D1, F54 AC-15): a provider whose /describe has no resolve_hints
// receives a POST /resolve body byte-identical to pre-ADR-095, even though the
// caller handed the Service every structured key. The ADR-080 golden this must
// match is the {"entity_type","hint":{"query","external_ids"}} shape — nothing more.
func TestServiceResolve_WireGolden(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		src      string // extra YAML under the source
		wantBody string
	}{
		{
			"no resolve_hints: byte-identical to ADR-080",
			`{"provider":"t","protocol_version":1,"entity_types":["video"],"fields":["title","studio"]}`,
			"",
			`{"entity_type":"video","hint":{"query":"Acme Pictures Ada Lovelace","external_ids":["imdb:tt1"]}}`,
		},
		{
			"fields: only advertised search keys, plus query_source, no filename",
			`{"provider":"t","protocol_version":1,"entity_types":["video"],"fields":["title","studio","overview"],"resolve_hints":["fields"]}`,
			"",
			`{"entity_type":"video","hint":{"query":"Acme Pictures Ada Lovelace","external_ids":["imdb:tt1"],"fields":{"studio":["Acme Pictures"],"title":["[Acme Pictures] Ada Lovelace (2023-08-01) 1080p"]},"query_source":"pattern"}}`,
		},
		{
			"filename: the basename verbatim, plus query_source, no fields",
			`{"provider":"t","protocol_version":1,"entity_types":["video"],"fields":["title"],"resolve_hints":["filename"]}`,
			"",
			`{"entity_type":"video","hint":{"query":"Acme Pictures Ada Lovelace","external_ids":["imdb:tt1"],"filename":"[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4","query_source":"pattern"}}`,
		},
		{
			"filename opted in but operator-denied: key absent, not empty, no error",
			`{"provider":"t","protocol_version":1,"entity_types":["video"],"fields":["title"],"resolve_hints":["filename"]}`,
			"    send_filename: false\n",
			`{"entity_type":"video","hint":{"query":"Acme Pictures Ada Lovelace","external_ids":["imdb:tt1"],"query_source":"pattern"}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var gotBody string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/describe":
					_, _ = w.Write([]byte(c.manifest))
				case "/resolve":
					b, _ := io.ReadAll(r.Body)
					gotBody = strings.TrimSpace(string(b))
					_, _ = w.Write([]byte(`{"candidates":[]}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			svc := wireSvc(t, "sources:\n  - name: t\n    base_url: "+srv.URL+"\n    entity_types: [video]\n    enabled: true\n"+c.src)
			if _, err := svc.Resolve(context.Background(), "t", model.EnrichEntityVideo, fullHint()); err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if gotBody != c.wantBody {
				t.Errorf("request body\n got %s\nwant %s", gotBody, c.wantBody)
			}
		})
	}
}

// ADR-095 D6 / contract §5: searched[] ingest — first 10 kept, each capped at
// maxFieldLen, control characters stripped and newlines collapsed (the
// candidates[].label treatment), empties dropped, and a missing key decodes to nil.
func TestServiceResolve_SearchedIngest(t *testing.T) {
	long := strings.Repeat("x", maxFieldLen+100)
	var entries []string
	for i := 0; i < 12; i++ {
		entries = append(entries, "q"+string(rune('a'+i)))
	}
	entries[1] = long
	entries[2] = "line\none\x07"
	entries[3] = "   "
	reply, _ := json.Marshal(map[string]any{"candidates": []Candidate{}, "searched": entries})

	cases := []struct {
		name  string
		reply string
		want  []string
	}{
		{"caps and sanitizes", string(reply), []string{"qa", long[:maxFieldLen], "line one", "qe", "qf", "qg", "qh", "qi", "qj"}},
		{"missing key decodes to nil", `{"candidates":[]}`, nil},
		{"empty list decodes to nil", `{"candidates":[],"searched":[]}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/describe":
					_, _ = w.Write([]byte(`{"provider":"t","protocol_version":1,"entity_types":["video"]}`))
				case "/resolve":
					_, _ = w.Write([]byte(c.reply))
				}
			}))
			defer srv.Close()
			svc := wireSvc(t, "sources:\n  - name: t\n    base_url: "+srv.URL+"\n    entity_types: [video]\n    enabled: true\n")
			res, err := svc.Resolve(context.Background(), "t", model.EnrichEntityVideo, Hint{Query: "x"})
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if c.want == nil {
				if res.Searched != nil {
					t.Errorf("Searched = %#v, want nil", res.Searched)
				}
				return
			}
			if len(res.Searched) != len(c.want) {
				t.Fatalf("Searched has %d entries, want %d: %q", len(res.Searched), len(c.want), res.Searched)
			}
			for i := range c.want {
				if res.Searched[i] != c.want[i] {
					t.Errorf("Searched[%d] = %q, want %q", i, res.Searched[i], c.want[i])
				}
			}
		})
	}
}

// RecordSearched writes the batch-path activity entry in the handoff's format and
// is a no-op when the provider reported nothing; the detail carries a basename a
// provider echoed back but never a path separator (F22.6b).
func TestServiceRecordSearched(t *testing.T) {
	svc, r := newSvc(t, NewFake("fake"))
	ctx := context.Background()

	svc.RecordSearched(time.Now(), "acme", model.EnrichEntityVideo, 412, ResolveResult{})
	if runs, _ := r.ListJobRuns(ctx, 10); len(runs) != 0 {
		t.Fatalf("no searched ⇒ no entry, got %d", len(runs))
	}

	svc.RecordSearched(time.Now(), "acme", model.EnrichEntityVideo, 412, ResolveResult{
		Candidates: []Candidate{{ExternalID: "x:1"}},
		Searched:   []string{"[Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4", "Acme Pictures Ada Lovelace"},
	})
	runs, err := r.ListJobRuns(ctx, 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %v err=%v", runs, err)
	}
	want := "acme → video #412 (1 candidates) · searched: [Acme Pictures] Ada Lovelace (2023-08-01) 1080p.mp4 · Acme Pictures Ada Lovelace"
	if runs[0].Detail != want {
		t.Errorf("detail\n got %q\nwant %q", runs[0].Detail, want)
	}
	if runs[0].Kind != model.JobKindEnrich || runs[0].EntityType != model.EnrichEntityVideo || runs[0].EntityID != 412 {
		t.Errorf("run attribution = %+v", runs[0])
	}
	if strings.ContainsAny(runs[0].Detail, `/\`) {
		t.Errorf("detail carries a path separator: %q", runs[0].Detail)
	}
}

func wireSvc(t *testing.T, sourcesYAML string) *Service {
	t.Helper()
	dir := t.TempDir()
	sp := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(sp, []byte(sourcesYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := NewStore(sp, log)
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return NewService(store, repo.New(database), log)
}

func boolPtr(b bool) *bool { return &b }

func keys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
