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
		{"overview", "Matroska", "Comment", true},
		{"release_date", "Matroska", "Year", true},
		{"genres", "Matroska", "Genre", true},
		{"actors", "Matroska", "Artist", true},
		{"studio", "Matroska", "Publisher", true},
		// WebM
		{"title", "WebM", "Title", true},
		{"overview", "WebM", "Comment", true},
		{"release_date", "WebM", "Year", true},
		{"genres", "WebM", "Genre", true},
		{"actors", "WebM", "Artist", true},
		{"studio", "WebM", "Publisher", true},
		// MP4 / QuickTime atoms
		{"title", "MP4", "QuickTime:Title", true},
		{"overview", "MP4", "QuickTime:Comment", true},
		{"release_date", "MP4", "QuickTime:Year", true},
		{"genres", "MP4", "QuickTime:Genre", true},
		{"actors", "MP4", "QuickTime:Artist", true},
		{"studio", "MP4", "QuickTime:Publisher", true},
		// MP3 / ID3
		{"title", "mp3", "Title", true},
		{"overview", "mp3", "Comment", true},
		{"release_date", "mp3", "Year", true},
		{"genres", "mp3", "Genre", true},
		{"actors", "mp3", "Artist", true},
		{"studio", "mp3", "Publisher", true},
		// FLAC
		{"title", "flac", "Title", true},
		{"overview", "flac", "Comment", true},
		{"release_date", "flac", "Year", true},
		{"actors", "flac", "Artist", true},
		{"studio", "flac", "Publisher", true},
		// No mapping: unknown canonical
		{"nonexistent_field", "Matroska", "", false},
		{"nonexistent_field", "MP4", "", false},
		// No mapping: unsupported container
		{"title", "avi", "", false},
		{"title", "", "", false},
		// image_url fields are intentionally absent from formatMap — handled by ImageTagForField
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

func TestImageTagForField(t *testing.T) {
	cases := []struct {
		canonical string
		container string
		wantTag   string
		wantOK    bool
	}{
		{"poster_url", "Matroska", "cover.jpg", true},
		{"poster_url", "WebM", "cover.jpg", true},
		{"poster_url", "MP4", "QuickTime:CoverArt", true},
		{"poster_url", "mp3", "Picture", true},
		{"poster_url", "flac", "Picture", true},
		// Non-image fields return false
		{"title", "Matroska", "", false},
		{"overview", "MP4", "", false},
		// Unsupported container
		{"poster_url", "avi", "", false},
		{"poster_url", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.canonical+"/"+tc.container, func(t *testing.T) {
			got, ok := ImageTagForField(tc.canonical, tc.container)
			if ok != tc.wantOK || got != tc.wantTag {
				t.Errorf("ImageTagForField(%q, %q) = (%q, %v), want (%q, %v)",
					tc.canonical, tc.container, got, ok, tc.wantTag, tc.wantOK)
			}
		})
	}
}
