package metadata

import (
	"encoding/json"
	"testing"
)

func TestMapExiftool(t *testing.T) {
	raw := map[string]any{
		"SourceFile":   "/m/a.mkv",
		"Title":        "Amélie",
		"Artist":       "Audrey Tautou, Mathieu Kassovitz",
		"Genre":        "Comedy; Romance",
		"RecordedDate": "2001:04:25 12:00:00",
		"Publisher":    "UGC",
		"FileSize":     "700 MB",
		"CoverArt":     "(Binary data 12345 bytes, use -b option to extract)",
	}
	ex := mapExiftool(raw)

	if ex.Title != "Amélie" {
		t.Errorf("title = %q", ex.Title)
	}
	if len(ex.People) != 2 || ex.People[0] != "Audrey Tautou" {
		t.Errorf("people = %v", ex.People)
	}
	if len(ex.Tags) != 2 || ex.Tags[1] != "Romance" {
		t.Errorf("tags = %v", ex.Tags)
	}
	if ex.RecordedAt == nil || ex.RecordedAt.Year() != 2001 {
		t.Errorf("recordedAt = %v", ex.RecordedAt)
	}
	// Publisher is captured; FileSize/CoverArt are excluded.
	if len(ex.Extra) != 1 || ex.Extra[0].SourceKey != "Publisher" || ex.Extra[0].Value != "UGC" {
		t.Errorf("extra = %+v", ex.Extra)
	}
}

func TestMapFfprobe(t *testing.T) {
	const sample = `{
	  "streams": [
	    {"codec_type":"audio","width":0,"height":0},
	    {"codec_type":"video","width":3840,"height":1606}
	  ],
	  "format": {"duration":"3661.5"}
	}`
	var p ffprobeOut
	if err := json.Unmarshal([]byte(sample), &p); err != nil {
		t.Fatal(err)
	}
	w, h, dur := mapFfprobe(p)
	if w != 3840 || h != 1606 {
		t.Errorf("dims = %dx%d", w, h)
	}
	if dur != 3662 { // 3661.5 rounds up
		t.Errorf("duration = %d", dur)
	}
}

func TestParseDate(t *testing.T) {
	cases := map[string]bool{
		"2001:04:25 12:00:00": true,
		"2001:04:25":          true,
		"2001":                true,
		"0000:00:00 00:00:00": false,
		"":                    false,
		"garbage":             false,
	}
	for in, wantOK := range cases {
		got := parseDate(in)
		if (got != nil) != wantOK {
			t.Errorf("parseDate(%q) ok=%v, want %v", in, got != nil, wantOK)
		}
	}
}
