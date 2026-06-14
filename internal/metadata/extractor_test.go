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
	    {"codec_type":"audio","codec_name":"aac","width":0,"height":0},
	    {"codec_type":"video","codec_name":"h264","width":3840,"height":1606}
	  ],
	  "format": {"duration":"3661.5","bit_rate":"8500000","format_name":"mov,mp4,m4a,3gp,3g2,mj2"}
	}`
	var p ffprobeOut
	if err := json.Unmarshal([]byte(sample), &p); err != nil {
		t.Fatal(err)
	}
	r := mapFfprobe(p)
	if r.width != 3840 || r.height != 1606 {
		t.Errorf("dims = %dx%d", r.width, r.height)
	}
	if r.durationSec != 3662 { // 3661.5 rounds up
		t.Errorf("duration = %d", r.durationSec)
	}
	if r.videoCodec != "h264" || r.audioCodec != "aac" {
		t.Errorf("codecs = %q / %q", r.videoCodec, r.audioCodec)
	}
	if r.bitrateKbps != 8500 { // 8500000 bps / 1000
		t.Errorf("bitrate = %d kbps", r.bitrateKbps)
	}
	if r.container != "MP4" {
		t.Errorf("container = %q", r.container)
	}
}

func TestNormalizeContainer(t *testing.T) {
	cases := map[string]string{
		"matroska,webm":             "Matroska",
		"mov,mp4,m4a,3gp,3g2,mj2":   "MP4",
		"webm":                      "WebM",
		"":                          "",
		"avi":                       "avi",
	}
	for in, want := range cases {
		if got := normalizeContainer(in); got != want {
			t.Errorf("normalizeContainer(%q) = %q, want %q", in, got, want)
		}
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
