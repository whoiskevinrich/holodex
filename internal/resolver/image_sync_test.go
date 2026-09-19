package resolver_test

import (
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/resolver"
)

// TestResolveFields_ImageSyncFromLedger pins ADR-101 D1: an image_url field with a
// standing decision takes its in_sync from the write ledger (Options.LastWritten),
// never from the file read-back — a matching newest write is in sync, a different
// one is out of sync, no row is unknown — and a text field ignores the ledger even
// when it carries an entry for it.
func TestResolveFields_ImageSyncFromLedger(t *testing.T) {
	allowAll := func(provider, rawURL string) bool { return true }
	const posterURL = "https://image.tmdb.org/poster.jpg"
	decided := resolver.Decisions{"poster_url": {Source: "provider:tmdb"}}

	cases := []struct {
		name string
		last map[string]string
		want *bool // nil = unknown
	}{
		{"no ledger loaded", nil, nil},
		{"no row for the field", map[string]string{"title": "x"}, nil},
		{"newest write is the decided URL", map[string]string{"poster_url": posterURL}, ptr(true)},
		{"newest write is another URL", map[string]string{"poster_url": "https://image.tmdb.org/old.jpg"}, ptr(false)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolver.ResolveFields(resolver.NewVideoBaseline(testVideo, nil), imageEnrich, nil,
				[]mapping.Field{imageField("poster_url", "tmdb:poster_url")},
				resolver.Options{Decisions: decided, ImageURLAllowed: allowAll, LastWritten: tc.last})
			if len(got) != 1 {
				t.Fatalf("want 1 field, got %d", len(got))
			}
			if got[0].Values[0] != posterURL {
				t.Fatalf("decided provider must win, got %q", got[0].Values[0])
			}
			switch {
			case tc.want == nil && got[0].InSync != nil:
				t.Errorf("want unknown in_sync, got %v", *got[0].InSync)
			case tc.want != nil && (got[0].InSync == nil || *got[0].InSync != *tc.want):
				t.Errorf("want in_sync %v, got %v", *tc.want, got[0].InSync)
			}
		})
	}

	t.Run("undecided image field stays in sync by construction", func(t *testing.T) {
		got := resolver.ResolveFields(resolver.NewVideoBaseline(testVideo, nil), imageEnrich, nil,
			[]mapping.Field{imageField("poster_url", "tmdb:poster_url")},
			resolver.Options{ImageURLAllowed: allowAll, LastWritten: map[string]string{"poster_url": "https://image.tmdb.org/old.jpg"}})
		if got[0].InSync == nil || !*got[0].InSync {
			t.Errorf("undecided must not consult the ledger, got %v", got[0].InSync)
		}
	})

	t.Run("text field ignores the ledger", func(t *testing.T) {
		// title decided to tmdb with a file value present: the file read-back (not the
		// ledger's stale value) decides — decided "TMDB Title" != file "filename_title".
		got := resolver.Resolve(testVideo, testExtra, testEnrich, nil, titleField(),
			resolver.Options{
				Decisions:   resolver.Decisions{"title": {Source: "provider:tmdb"}},
				LastWritten: map[string]string{"title": "TMDB Title"},
			})
		if got[0].InSync == nil || *got[0].InSync {
			t.Errorf("text field must use the file read-back (out of sync), got %v", got[0].InSync)
		}
	})
}

func ptr(b bool) *bool { return &b }
