package api

import (
	"math"
	"net/url"
	"testing"

	"holodex/internal/repo"
)

// pageFromQuery clamps like GET /media (HOLODEX-587): default 50, capped at
// repo.MaxListLimit, never a negative offset.
func TestPageFromQuery(t *testing.T) {
	cases := []struct {
		query         string
		limit, offset int
	}{
		{"", defaultPageLimit, 0},
		{"limit=10&offset=20", 10, 20},
		{"limit=0", defaultPageLimit, 0},
		{"limit=-3", defaultPageLimit, 0},
		{"limit=abc&offset=xyz", defaultPageLimit, 0},
		{"limit=999999", repo.MaxListLimit, 0},
		{"offset=-5", defaultPageLimit, 0},
	}
	for _, c := range cases {
		q, _ := url.ParseQuery(c.query)
		if got := pageFromQuery(q); got.Limit != c.limit || got.Offset != c.offset {
			t.Errorf("pageFromQuery(%q) = %+v, want {%d %d}", c.query, got, c.limit, c.offset)
		}
	}
}

func TestPageOf(t *testing.T) {
	all := []int{1, 2, 3, 4, 5}
	cases := []struct {
		p    page
		want []int
	}{
		{page{Limit: 2, Offset: 0}, []int{1, 2}},
		{page{Limit: 2, Offset: 4}, []int{5}},
		{page{Limit: 2, Offset: 5}, []int{}},
		{page{Limit: 2, Offset: 99}, []int{}},
	}
	for _, c := range cases {
		got := pageOf(all, c.p)
		if got == nil || len(got) != len(c.want) {
			t.Fatalf("pageOf(%+v) = %v, want %v", c.p, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("pageOf(%+v) = %v, want %v", c.p, got, c.want)
			}
		}
	}
	if got := pageOf([]int(nil), page{Limit: 5}); got == nil {
		t.Error("pageOf(nil) = nil, want an empty non-nil slice")
	}
}

// A completeness group pages over candidate-ready then needs-research, and Total is
// always the whole group's count.
func TestFacetGroupWindow(t *testing.T) {
	g := FacetGroup{
		CandidateReady: []QueueRow{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		NeedsResearch:  []QueueRow{{Name: "d"}, {Name: "e"}},
	}
	names := func(g FacetGroup) (out string) {
		for _, r := range g.CandidateReady {
			out += r.Name
		}
		out += "|"
		for _, r := range g.NeedsResearch {
			out += r.Name
		}
		return out
	}
	cases := []struct {
		p    page
		want string
	}{
		{page{Limit: 2, Offset: 0}, "ab|"},
		{page{Limit: 2, Offset: 2}, "c|d"},
		{page{Limit: 2, Offset: 4}, "|e"},
		{page{Limit: 50, Offset: 0}, "abc|de"},
		{page{Limit: 2, Offset: 9}, "|"},
		{page{Limit: 50, Offset: math.MaxInt}, "|"},
	}
	for _, c := range cases {
		w := g.window(c.p)
		if got := names(w); got != c.want || w.Total != 5 {
			t.Errorf("window(%+v) = %q total %d, want %q total 5", c.p, got, w.Total, c.want)
		}
		if w.CandidateReady == nil || w.NeedsResearch == nil {
			t.Errorf("window(%+v) left a nil list; it must marshal as []", c.p)
		}
	}
}

// The extraction queue pages by video: most pending fields first, then title, and a
// video's rows always stay together.
func TestExtractionGroups(t *testing.T) {
	row := func(video int64, title string) repo.ExtractionQueueRow {
		return repo.ExtractionQueueRow{VideoID: video, VideoTitle: title}
	}
	rows := []repo.ExtractionQueueRow{
		row(1, "beta"), // 1 row
		row(2, "Zulu"), row(2, "Zulu"), // 2 rows
		row(3, "alpha"), // 1 row, sorts before beta by title
		row(4, "Mike"), row(4, "Mike"), row(4, "Mike"), // 3 rows
	}
	groups := extractionGroups(rows)
	var order []int64
	for _, g := range groups {
		for _, r := range g {
			if r.VideoID != g[0].VideoID {
				t.Fatalf("group mixes videos: %+v", g)
			}
		}
		order = append(order, g[0].VideoID)
	}
	want := []int64{4, 2, 3, 1}
	if len(order) != len(want) {
		t.Fatalf("groups = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("groups = %v, want %v", order, want)
		}
	}
}
