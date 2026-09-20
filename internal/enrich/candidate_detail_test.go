package enrich

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"holodex/internal/model"
)

// sanitizeDetail (F61 FR2, contract §5): first maxDetail entries, each cleaned like
// a label then cut to maxDetailLen on a rune boundary, empties dropped, nil/[] ⇒ nil.
func TestSanitizeDetail(t *testing.T) {
	long := strings.Repeat("x", maxDetailLen+144)
	// A multi-byte rune straddling the byte cap must not be split into invalid UTF-8.
	straddle := strings.Repeat("y", maxDetailLen-1) + "é" + "tail"
	var twelve []string
	for i := 0; i < 12; i++ {
		twelve = append(twelve, "l"+string(rune('a'+i)))
	}

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil in ⇒ nil out", nil, nil},
		{"empty list ⇒ nil", []string{}, nil},
		{"all-blank ⇒ nil", []string{"   ", "\t"}, nil},
		{"under the caps passes verbatim, in order", []string{"Studio: A › B", "Record: 28 tags · synopsis"}, []string{"Studio: A › B", "Record: 28 tags · synopsis"}},
		{"keeps the first 8 of 12", twelve, twelve[:8]},
		{"cuts a long line at the byte cap", []string{long}, []string{long[:maxDetailLen]}},
		{"cut lands on a rune boundary", []string{straddle}, []string{strings.Repeat("y", maxDetailLen-1)}},
		{"control chars stripped, newlines collapsed to one line", []string{"line\none\x07", "\x1b[31mred\x1b[0m", "a\r\nb"}, []string{"line one", "[31mred[0m", "a  b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeDetail(c.in)
			if c.want == nil {
				if got != nil {
					t.Fatalf("got %#v, want nil", got)
				}
				return
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %d entries %q, want %d %q", len(got), got, len(c.want), c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], c.want[i])
				}
				if !utf8.ValidString(got[i]) || len(got[i]) > maxDetailLen || strings.ContainsAny(got[i], "\n\r") {
					t.Errorf("[%d] violates the line invariant: %q", i, got[i])
				}
			}
		})
	}
}

// Over the wire (F61 FR1): detail on one candidate reaches the caller sanitized and
// in provider order, a candidate without it stays nil, and every other candidate
// key is untouched. `[]` and a missing key are indistinguishable after ingest.
func TestServiceResolve_DetailIngest(t *testing.T) {
	reply := `{"candidates":[
		{"external_id":"t:1","namespace":"t","label":"Harbor Lights","confidence":0.7,"disambiguation":"Outlet B","profile_url":"https://t.example/1",
		 "detail":["Studio: Outlet B › Network X","Record: 28 tags · synopsis","  ","Id: t:1\n"]},
		{"external_id":"t:2","namespace":"t","label":"Harbor Lights","confidence":0.7,"detail":[]},
		{"external_id":"t:3","namespace":"t","label":"Harbor Lights","confidence":0.7}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/describe":
			_, _ = w.Write([]byte(`{"provider":"t","protocol_version":1,"entity_types":["video"]}`))
		case "/resolve":
			_, _ = w.Write([]byte(reply))
		}
	}))
	defer srv.Close()
	svc := wireSvc(t, "sources:\n  - name: t\n    base_url: "+srv.URL+"\n    entity_types: [video]\n    enabled: true\n")
	res, err := svc.Resolve(context.Background(), "t", model.EnrichEntityVideo, Hint{Query: "x"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(res.Candidates) != 3 {
		t.Fatalf("candidates = %d, want 3", len(res.Candidates))
	}
	c := res.Candidates[0]
	want := []string{"Studio: Outlet B › Network X", "Record: 28 tags · synopsis", "Id: t:1"}
	if strings.Join(c.Detail, "|") != strings.Join(want, "|") {
		t.Errorf("Detail = %q, want %q", c.Detail, want)
	}
	if c.Label != "Harbor Lights" || c.Disambiguation != "Outlet B" || c.ProfileURL != "https://t.example/1" || c.Confidence != 0.7 {
		t.Errorf("sibling keys disturbed: %+v", c)
	}
	for i := 1; i < 3; i++ {
		if res.Candidates[i].Detail != nil {
			t.Errorf("candidate %d Detail = %#v, want nil", i, res.Candidates[i].Detail)
		}
	}
	// What the API hands the client: the key is absent, never `[]`.
	raw, _ := json.Marshal(res.Candidates[1])
	if strings.Contains(string(raw), `"detail"`) {
		t.Errorf("empty detail leaked to the wire: %s", raw)
	}
}

// RecordSearched with an applied candidate (F61 FR5): the entry carries the
// auto-applied candidate's lines after searched[], is written on detail alone when
// the provider sent no searched[], and is unchanged when the applied candidate had
// no detail. The interactive path never passes applied — that stays nil.
func TestServiceRecordSearched_AppliedDetail(t *testing.T) {
	svc, r := newSvc(t, NewFake("fake"))
	ctx := context.Background()
	searched := []string{"Harbor Lights 2023"}
	withDetail := &Candidate{ExternalID: "x:1", Label: "Harbor Lights",
		Detail: []string{"Studio: Outlet B › Network X", "Record: 28 tags · synopsis · 3 images"}}
	noDetail := &Candidate{ExternalID: "x:1", Label: "Harbor Lights"}

	// applied without detail and nothing searched ⇒ no entry (unchanged from today).
	svc.RecordSearched(time.Now(), "acme", model.EnrichEntityVideo, 412, ResolveResult{Candidates: []Candidate{*noDetail}}, noDetail, "")
	if runs, _ := r.ListJobRuns(ctx, 10); len(runs) != 0 {
		t.Fatalf("applied without detail + no searched ⇒ no entry, got %d", len(runs))
	}

	// searched + applied without detail ⇒ exactly today's line.
	svc.RecordSearched(time.Now(), "acme", model.EnrichEntityVideo, 412, ResolveResult{Candidates: []Candidate{*noDetail}, Searched: searched}, noDetail, "")
	// searched + applied with detail ⇒ the applied segment follows searched.
	svc.RecordSearched(time.Now(), "acme", model.EnrichEntityVideo, 412, ResolveResult{Candidates: []Candidate{*withDetail}, Searched: searched}, withDetail, "")
	// no searched + applied with detail ⇒ written on detail alone.
	svc.RecordSearched(time.Now(), "acme", model.EnrichEntityVideo, 412, ResolveResult{Candidates: []Candidate{*withDetail}}, withDetail, "")

	runs, err := r.ListJobRuns(ctx, 10)
	if err != nil || len(runs) != 3 {
		t.Fatalf("runs = %d err=%v", len(runs), err)
	}
	// ListJobRuns is newest-first.
	wants := []string{
		"acme → video #412 (1 candidates) · applied: Harbor Lights — Studio: Outlet B › Network X · Record: 28 tags · synopsis · 3 images",
		"acme → video #412 (1 candidates) · searched: Harbor Lights 2023 · applied: Harbor Lights — Studio: Outlet B › Network X · Record: 28 tags · synopsis · 3 images",
		"acme → video #412 (1 candidates) · searched: Harbor Lights 2023",
	}
	for i, w := range wants {
		if runs[i].Detail != w {
			t.Errorf("run[%d]\n got %q\nwant %q", i, runs[i].Detail, w)
		}
		if strings.ContainsAny(runs[i].Detail, `/\`) {
			t.Errorf("run[%d] carries a path separator: %q", i, runs[i].Detail)
		}
	}
}
