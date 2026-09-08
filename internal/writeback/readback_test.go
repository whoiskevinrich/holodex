package writeback

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"holodex/internal/mapping"
)

// TestReadbackGaps_DetectsWrongTag pins the property that makes the check worth having:
// declaring SOME file tag is not enough. A field reading `Description` while writeback
// writes `Comment` reproduces HOLODEX-335 exactly and must still be reported.
func TestReadbackGaps_DetectsWrongTag(t *testing.T) {
	wrong := mapping.Field{
		Canonical:     "overview",
		ParsedSources: []mapping.Source{{Namespace: "file", Key: "Description"}, {Namespace: "tmdb", Key: "overview"}},
	}
	right := mapping.Field{
		Canonical:     "overview",
		ParsedSources: []mapping.Source{{Namespace: "file", Key: "Comment"}, {Namespace: "tmdb", Key: "overview"}},
	}
	merge := mapping.Field{
		Canonical:     "genres",
		Multi:         true,
		ParsedSources: []mapping.Source{{Namespace: "tmdb", Key: "genres"}},
	}

	gaps := ReadbackGaps([]mapping.Field{wrong, merge})
	if len(gaps) != 1 || gaps[0].Canonical != "overview" {
		t.Fatalf("a file source naming the wrong tag must still be a gap (and a merge field must not), got %+v", gaps)
	}
	if got := gaps[0].WantKeys; len(got) != 1 || got[0] != "comment" {
		t.Errorf("the gap should name the key that closes it; want [comment], got %v", got)
	}
	if gaps := ReadbackGaps([]mapping.Field{right}); len(gaps) != 0 {
		t.Errorf("a file source naming the written tag closes the gap, got %+v", gaps)
	}
}

// TestLogReadbackGaps_MessageIsActionable pins that the runtime warning carries what an
// operator needs to act — which field, and which key closes it. A warning that only says
// "something is wrong" would leave them exactly where the HOLODEX-335 reporter started.
func TestLogReadbackGaps_MessageIsActionable(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	LogReadbackGaps(log, []mapping.Field{{
		Canonical:     "release_date",
		ParsedSources: []mapping.Source{{Namespace: "tmdb", Key: "release_date"}},
	}})

	out := buf.String()
	for _, want := range []string{"release_date", "year", "canonical-fields"} {
		if !strings.Contains(out, want) {
			t.Errorf("warning should name %q so the operator can act on it; got: %s", want, out)
		}
	}
}

// TestLogReadbackGaps_SilentWhenClean keeps the warning meaningful: a mapping with no gap
// must log nothing at all. A line on every start would train the operator to ignore it.
func TestLogReadbackGaps_SilentWhenClean(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	LogReadbackGaps(log, []mapping.Field{
		{Canonical: "release_date", ParsedSources: []mapping.Source{{Namespace: "file", Key: "Year"}}},
		// Exempt by UnreadableWriteTargets — no source can close it, so warning is noise.
		{Canonical: "tagline", ParsedSources: []mapping.Source{{Namespace: "tmdb", Key: "tagline"}}},
	})

	if buf.Len() != 0 {
		t.Errorf("a mapping with no actionable gap must log nothing, got: %s", buf.String())
	}
}
