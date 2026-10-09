package metadata

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"slices"
	"testing"
)

// ---- EBML builders (also used by the integration test) ----

func ebmlEl(id uint64, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	var idb []byte
	for v := id; v > 0; v >>= 8 {
		idb = append([]byte{byte(v)}, idb...)
	}
	size := make([]byte, 8)
	binary.BigEndian.PutUint64(size, uint64(len(body)))
	size[0] = 0x01 // 8-byte size marker
	return slices.Concat(idb, size, body)
}

func ebmlUint(id, v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return ebmlEl(id, b)
}

func ebmlStr(id uint64, s string) []byte { return ebmlEl(id, []byte(s)) }

type testTag struct {
	level int    // 0 = no TargetTypeValue
	uidID uint64 // a Tag*UID element ID, 0 = none
	uid   uint64
	pairs [][2]string
}

func ebmlTags(tags ...testTag) []byte {
	var out [][]byte
	for _, t := range tags {
		var targets [][]byte
		if t.level != 0 {
			targets = append(targets, ebmlUint(ebmlIDTargetType, uint64(t.level)))
		}
		if t.uidID != 0 {
			targets = append(targets, ebmlUint(t.uidID, t.uid))
		}
		body := [][]byte{ebmlEl(ebmlIDTargets, targets...)}
		for _, p := range t.pairs {
			body = append(body, ebmlEl(ebmlIDSimpleTag, ebmlStr(ebmlIDTagName, p[0]), ebmlStr(ebmlIDTagString, p[1])))
		}
		out = append(out, ebmlEl(ebmlIDTag, body...))
	}
	return ebmlEl(ebmlIDTags, out...)
}

func mkvFile(segment ...[]byte) []byte {
	header := ebmlEl(ebmlIDHeader, ebmlStr(0x4282, "matroska"))
	return slices.Concat(header, ebmlEl(ebmlIDSegment, segment...))
}

const ebmlIDCluster = 0x1F43B675

func TestParseMatroskaTags_LevelsAndBinding(t *testing.T) {
	file := mkvFile(
		ebmlEl(ebmlIDInfo, ebmlUint(0x2AD7B1, 1000000), ebmlStr(0x7BA9, "Info Title")),
		// A Cluster full of bytes that would mis-parse as elements: it must be skipped by size.
		ebmlEl(ebmlIDCluster, bytes.Repeat([]byte{0x73, 0x73, 0x81}, 50)),
		ebmlTags(
			testTag{level: 30, uidID: ebmlIDTrackUID, uid: 42, pairs: [][2]string{{"TITLE", "track"}}},
			testTag{level: 70, pairs: [][2]string{{"TITLE", "collection"}}},
			testTag{pairs: [][2]string{{"TITLE", "movie"}, {"ARTIST", "a"}}},
			testTag{level: 50, uidID: ebmlIDTrackUID, uid: 0, pairs: [][2]string{{"GENRE", "all-tracks"}}},
			testTag{level: 30, uidID: ebmlIDChapterUID, uid: 7, pairs: [][2]string{{"TITLE", "chapter"}}},
		),
	)
	got, err := parseMatroskaTags(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	if !got.InfoKeys["Title"] || got.InfoKeys["DateTimeOriginal"] {
		t.Errorf("InfoKeys = %v, want only Title", got.InfoKeys)
	}
	want := []mkvSimpleTag{
		{"TITLE", "track", 30, true},
		{"TITLE", "collection", 70, false},
		{"TITLE", "movie", 50, false},
		{"ARTIST", "a", 50, false},
		{"GENRE", "all-tracks", 50, false}, // UID 0 = every track, not bound to one
		{"TITLE", "chapter", 30, true},
	}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Errorf("Tags =\n %v\nwant\n %v", got.Tags, want)
	}
}

func TestParseMatroskaTags_UnknownSizeSegmentIsReadToEOF(t *testing.T) {
	header := ebmlEl(ebmlIDHeader)
	seg := slices.Concat(unknownSizedSegment(), ebmlTags(testTag{pairs: [][2]string{{"TITLE", "x"}}}))
	file := slices.Concat(header, seg)
	got, err := parseMatroskaTags(bytes.NewReader(file), int64(len(file)))
	if err != nil || len(got.Tags) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func unknownSizedSegment() []byte {
	return []byte{0x18, 0x53, 0x80, 0x67, 0xFF} // Segment ID, 1-byte unknown size
}

func TestParseMatroskaTags_Errors(t *testing.T) {
	good := mkvFile(ebmlTags(testTag{pairs: [][2]string{{"TITLE", "x"}}}))
	cases := map[string][]byte{
		"not EBML":  []byte("not a matroska file at all"),
		"truncated": good[:len(good)-4],
		// An unknown-size Cluster can't be skipped, so Tags after it are unreachable.
		"unknown-size cluster": slices.Concat(ebmlEl(ebmlIDHeader), unknownSizedSegment(),
			[]byte{0x1F, 0x43, 0xB6, 0x75, 0xFF}, ebmlTags(testTag{pairs: [][2]string{{"TITLE", "x"}}})),
	}
	for name, file := range cases {
		if _, err := parseMatroskaTags(bytes.NewReader(file), int64(len(file))); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestSelectMatroskaTags(t *testing.T) {
	track := func(pairs ...[2]string) testTag {
		return testTag{level: 30, uidID: ebmlIDTrackUID, uid: 1, pairs: pairs}
	}
	tag := func(level int, pairs ...[2]string) testTag { return testTag{level: level, pairs: pairs} }
	cases := []struct {
		name string
		raw  map[string]any // exiftool's flattened output (first occurrence wins)
		info []string
		tags []testTag
		want map[string]any
	}{
		{
			name: "track-level tag before the movie-level one",
			raw:  map[string]any{"Title": "TRACK", "Artist": "Track Artist", "VideoFrameRate": 25},
			tags: []testTag{track([2]string{"TITLE", "TRACK"}, [2]string{"ARTIST", "Track Artist"}),
				tag(50, [2]string{"TITLE", "MOVIE"}, [2]string{"ARTIST", "Movie Artist"})},
			want: map[string]any{"Title": "MOVIE", "Artist": "Movie Artist", "VideoFrameRate": 25},
		},
		{
			name: "collection before movie: no series smear",
			raw:  map[string]any{"Title": "SERIES", "Actor": "Series Cast"},
			tags: []testTag{tag(70, [2]string{"TITLE", "SERIES"}, [2]string{"ACTOR", "Series Cast"}),
				tag(0, [2]string{"TITLE", "EPISODE"}, [2]string{"ACTOR", "Guest"})},
			want: map[string]any{"Title": "EPISODE", "Actor": "Guest"},
		},
		{
			name: "single-value falls back 60 then 70; multi-value never inherits",
			raw:  map[string]any{"Title": "SERIES", "PartNumber": "3", "Genre": "Drama"},
			tags: []testTag{tag(70, [2]string{"TITLE", "SERIES"}, [2]string{"GENRE", "Drama"}),
				tag(60, [2]string{"PART_NUMBER", "3"})},
			want: map[string]any{"Title": "SERIES", "PartNumber": "3"},
		},
		{
			name: "season beats collection",
			raw:  map[string]any{"Title": "COLLECTION"},
			tags: []testTag{tag(70, [2]string{"TITLE", "COLLECTION"}), tag(60, [2]string{"TITLE", "SEASON"})},
			want: map[string]any{"Title": "SEASON"},
		},
		{
			name: "every movie-level value of a multi-value key is kept",
			raw:  map[string]any{"Actor": "A"},
			tags: []testTag{tag(50, [2]string{"ACTOR", "A"}, [2]string{"ACTOR", "B"}), tag(50, [2]string{"ACTOR", "C"})},
			want: map[string]any{"Actor": "A, B, C"},
		},
		{
			name: "only track-level: the key is dropped",
			raw:  map[string]any{"Encoder": "x264", "Bps": "900"},
			tags: []testTag{track([2]string{"BPS", "900"})},
			want: map[string]any{"Encoder": "x264"},
		},
		{
			name: "segment Info title and date outrank tags",
			raw:  map[string]any{"Title": "INFO", "DateTimeOriginal": "2020:01:01 00:00:00Z"},
			info: []string{"Title", "DateTimeOriginal"},
			tags: []testTag{tag(50, [2]string{"TITLE", "TAG"}, [2]string{"DATE_RECORDED", "1999"})},
			want: map[string]any{"Title": "INFO", "DateTimeOriginal": "2020:01:01 00:00:00Z"},
		},
		{
			name: "substituted dates take exiftool's colon form",
			raw:  map[string]any{"DateTimeOriginal": "1999:01:01 00:00:00"},
			tags: []testTag{tag(70, [2]string{"DATE_RECORDED", "1999-01-01 00:00:00"}),
				tag(50, [2]string{"DATE_RECORDED", "2001-04-25 12:00:00"})},
			want: map[string]any{"DateTimeOriginal": "2001:04:25 12:00:00"},
		},
		{
			name: "renamed standard tag and language variants collapse to one key",
			raw:  map[string]any{"DateTimeOriginal": "2001", "Title-eng": "COLL", "Title-fre": "COLL-FR"},
			tags: []testTag{tag(70, [2]string{"DATE_RECORDED", "2001"}, [2]string{"TITLE", "COLL"}),
				tag(50, [2]string{"DATE_RECORDED", "2002"}, [2]string{"TITLE", "EP"})},
			want: map[string]any{"DateTimeOriginal": "2002", "Title": "EP"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file := mkvFile(ebmlTags(c.tags...))
			mt, err := parseMatroskaTags(bytes.NewReader(file), int64(len(file)))
			if err != nil {
				t.Fatal(err)
			}
			for _, k := range c.info {
				if mt.InfoKeys == nil {
					mt.InfoKeys = map[string]bool{}
				}
				mt.InfoKeys[k] = true
			}
			if got := selectMatroskaTags(c.raw, mt); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got  %v\nwant %v", got, c.want)
			}
		})
	}
}

func TestSelectMatroskaTags_NoTagsIsIdentity(t *testing.T) {
	raw := map[string]any{"Title": "x"}
	if got := selectMatroskaTags(raw, mkvTags{}); !reflect.DeepEqual(got, raw) {
		t.Errorf("got %v", got)
	}
}
