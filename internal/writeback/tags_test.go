package writeback

import "testing"

func TestTagForField(t *testing.T) {
	cases := []struct {
		canonical string
		container string
		wantTag   string
		wantOK    bool
	}{
		// Matroska
		{"title", "Matroska", "Title", true},
		{"overview", "Matroska", "Summary", true},
		{"genres", "Matroska", "GENRE", true},
		{"release_date", "Matroska", "DATE_RELEASED", true},
		// WebM (subset of Matroska mappings)
		{"title", "WebM", "Title", true},
		{"genres", "WebM", "GENRE", true},
		// MP4 / QuickTime atoms
		{"title", "MP4", "QuickTime:Title", true},
		{"overview", "MP4", "QuickTime:Comment", true},
		{"genres", "MP4", "QuickTime:Genre", true},
		// MP3 / ID3
		{"title", "mp3", "Title", true},
		{"genres", "mp3", "Genre", true},
		// FLAC
		{"title", "flac", "Title", true},
		// No mapping: unknown canonical
		{"nonexistent_field", "Matroska", "", false},
		{"nonexistent_field", "MP4", "", false},
		// No mapping: unsupported container
		{"title", "avi", "", false},
		{"title", "", "", false},
		// image_url fields are intentionally absent — no tag target for URLs
		{"poster_url", "Matroska", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.canonical+"/"+tc.container, func(t *testing.T) {
			got, ok := TagForField(tc.canonical, tc.container)
			if ok != tc.wantOK || got != tc.wantTag {
				t.Errorf("TagForField(%q, %q) = (%q, %v), want (%q, %v)",
					tc.canonical, tc.container, got, ok, tc.wantTag, tc.wantOK)
			}
		})
	}
}
