package resolver_test

import (
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/resolver"
)

// TestResolveFields_ReadbackGapSyncFromLedger pins ADR-119 D1/D2: a text field named
// in Options.ReadbackGaps takes its in_sync from the write ledger exactly as an image
// field does (ADR-101) — including when some other `file:` source is declared, which
// reads a different tag than the one written — and a field outside the set keeps the
// file read-back.
func TestResolveFields_ReadbackGapSyncFromLedger(t *testing.T) {
	decided := resolver.Decisions{"title": {Source: "provider:tmdb"}}
	gap := map[string]bool{"title": true}

	cases := []struct {
		name   string
		fields []mapping.Field
		last   map[string]string
		want   *bool // nil = unknown
	}{
		// Provider/filename only: before ADR-119 this was unknown for ever.
		{"no file source, never written", []mapping.Field{stubField("title", true, "tmdb:title")}, map[string]string{}, nil},
		{"no file source, newest write is the decided value", []mapping.Field{stubField("title", true, "tmdb:title")}, map[string]string{"title": "TMDB Title"}, ptr(true)},
		{"no file source, newest write is another value", []mapping.Field{stubField("title", true, "tmdb:title")}, map[string]string{"title": "Old Title"}, ptr(false)},
		// A declared file source reading some other tag: before ADR-119 this compared
		// against that tag and read false for ever; the ledger now decides.
		{"other file source, newest write is the decided value", []mapping.Field{stubField("title", true, "tmdb:title", "file:SomeOtherTag")}, map[string]string{"title": "TMDB Title"}, ptr(true)},
		{"no ledger loaded", []mapping.Field{stubField("title", true, "tmdb:title")}, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, tc.fields,
				resolver.Options{Decisions: decided, LastWritten: tc.last, ReadbackGaps: gap})
			if len(got) != 1 || got[0].Values[0] != "TMDB Title" {
				t.Fatalf("decided provider must win, got %+v", got)
			}
			switch {
			case tc.want == nil && got[0].InSync != nil:
				t.Errorf("want unknown in_sync, got %v", *got[0].InSync)
			case tc.want != nil && (got[0].InSync == nil || *got[0].InSync != *tc.want):
				t.Errorf("want in_sync %v, got %v", *tc.want, got[0].InSync)
			}
		})
	}

	t.Run("field outside the gap set keeps the file read-back", func(t *testing.T) {
		// Same as image_sync_test's text case: the ledger says "TMDB Title" was written,
		// but the file reads "filename_title", and only the gap set routes to the ledger.
		got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, titleField(),
			resolver.Options{Decisions: decided, LastWritten: map[string]string{"title": "TMDB Title"}, ReadbackGaps: map[string]bool{"overview": true}})
		if got[0].InSync == nil || *got[0].InSync {
			t.Errorf("non-gap field must use the file read-back (out of sync), got %v", got[0].InSync)
		}
	})

	t.Run("undecided gap field stays in sync by construction", func(t *testing.T) {
		got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, []mapping.Field{stubField("title", true, "tmdb:title")},
			resolver.Options{LastWritten: map[string]string{"title": "Old Title"}, ReadbackGaps: gap})
		if got[0].InSync == nil || !*got[0].InSync {
			t.Errorf("undecided must not consult the ledger, got %v", got[0].InSync)
		}
	})
}
