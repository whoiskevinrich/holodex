package resolver_test

import (
	"strings"
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/resolver"
)

// F85 (HOLODEX-599): a poster Holodex holds is a local:<kind> decision source over a
// fixed slot (field-resolution.md "A poster Holodex holds").

var (
	hashA = strings.Repeat("a", 64)
	hashB = strings.Repeat("b", 64)
)

const providerPoster = "https://img.provider-1.example/poster.jpg"

var localEnrich = resolver.Enrichment{"provider-1": {"poster_url": {providerPoster}}}

func resolvePoster(t *testing.T, opts resolver.Options) resolver.ResolvedField {
	t.Helper()
	opts.ImageURLAllowed = func(string, string) bool { return true }
	got := resolver.ResolveFields(resolver.NewVideoBaseline(testVideo, nil), localEnrich, nil,
		[]mapping.Field{imageField("poster_url", "provider-1:poster_url")}, opts)
	if len(got) != 1 {
		t.Fatalf("want 1 field, got %d", len(got))
	}
	return got[0]
}

// G2: a local decision resolves to its slot's content token, and a new hash moves
// the token with no decision change.
func TestLocalPosterDecisionResolvesToToken(t *testing.T) {
	dec := resolver.Decisions{"poster_url": {Source: "local:frame"}}
	for _, h := range []string{hashA, hashB} {
		rf := resolvePoster(t, resolver.Options{Decisions: dec,
			LocalPosters: map[string]resolver.LocalPoster{"frame": {Hash: h}}})
		if want := "local:frame#" + h; rf.Values[0] != want {
			t.Fatalf("value = %q, want %q", rf.Values[0], want)
		}
		if rf.Display != "image_url" {
			t.Fatalf("a local poster must not degrade the display: %q", rf.Display)
		}
		if rf.Decision == nil || rf.Decision.Source != "local:frame" || !rf.Decision.Standing {
			t.Fatalf("decision = %+v", rf.Decision)
		}
	}
}

// G3: undecided, a held slot is a candidate but never wins; a decision whose slot
// is empty resolves through the undecided default.
func TestLocalPosterNeverWinsUndecided(t *testing.T) {
	locals := map[string]resolver.LocalPoster{"upload": {Hash: hashA}, "automatic": {Hash: hashB}}
	rf := resolvePoster(t, resolver.Options{LocalPosters: locals})
	if rf.Values[0] != providerPoster {
		t.Fatalf("undecided value = %q, want the provider default", rf.Values[0])
	}
	var srcs []string
	for _, c := range rf.Candidates {
		srcs = append(srcs, c.Source)
	}
	if got := strings.Join(srcs, ","); got != "file,provider:provider-1,local:upload,local:automatic" {
		t.Fatalf("candidates = %s", got)
	}

	rf = resolvePoster(t, resolver.Options{LocalPosters: locals,
		Decisions: resolver.Decisions{"poster_url": {Source: "local:frame"}}})
	if rf.Values[0] != providerPoster {
		t.Fatalf("decided empty slot = %q, want the undecided default", rf.Values[0])
	}
}

// A visitor read loads no slots: no local candidate, and the decision falls through.
func TestLocalPosterHiddenWithoutSlots(t *testing.T) {
	rf := resolvePoster(t, resolver.Options{Decisions: resolver.Decisions{"poster_url": {Source: "local:upload"}}})
	for _, c := range rf.Candidates {
		if strings.HasPrefix(c.Source, "local:") {
			t.Fatalf("local candidate without slots: %+v", c)
		}
	}
}

// G4: the ledger witnesses a local decision by its token — same hash in sync, a
// newer image in the slot reads will-write.
func TestLocalPosterSyncFromLedger(t *testing.T) {
	dec := resolver.Decisions{"poster_url": {Source: "local:upload"}}
	locals := map[string]resolver.LocalPoster{"upload": {Hash: hashB}}
	for _, tc := range []struct {
		last string
		want bool
	}{{"local:upload#" + hashB, true}, {"local:upload#" + hashA, false}} {
		rf := resolvePoster(t, resolver.Options{Decisions: dec, LocalPosters: locals,
			LastWritten: map[string]string{"poster_url": tc.last}})
		if rf.InSync == nil || *rf.InSync != tc.want {
			t.Fatalf("last %q: in_sync = %v, want %v", tc.last, rf.InSync, tc.want)
		}
	}
}

// G13: the frame candidate carries its picked time for the tile's title.
func TestLocalFrameCandidateCarriesTime(t *testing.T) {
	at := 760.0
	rf := resolvePoster(t, resolver.Options{LocalPosters: map[string]resolver.LocalPoster{"frame": {Hash: hashA, FrameAt: &at}}})
	for _, c := range rf.Candidates {
		if c.Source == "local:frame" {
			if c.FrameAt == nil || *c.FrameAt != at || c.Value != "local:frame#"+hashA {
				t.Fatalf("frame candidate = %+v", c)
			}
			return
		}
	}
	t.Fatal("no frame candidate")
}

// A local source on any other field is not a poster and resolves undecided.
func TestLocalDecisionOnlyForPoster(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewVideoBaseline(testVideo, nil), nil, nil,
		[]mapping.Field{stubField("title", false, "file:Title")},
		resolver.Options{Decisions: resolver.Decisions{"title": {Source: "local:frame"}},
			LocalPosters: map[string]resolver.LocalPoster{"frame": {Hash: hashA}}})
	if len(got) != 1 || strings.HasPrefix(got[0].Values[0], "local:") {
		t.Fatalf("title = %+v", got)
	}
}
