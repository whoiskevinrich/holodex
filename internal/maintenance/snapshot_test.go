package maintenance

import (
	"strings"
	"testing"
)

func TestParseStreamHash_MapsOutputToInputIndexes(t *testing.T) {
	// Attachment at input 2 was not mapped, so output 2 is input 3.
	out := []byte("0,a,SHA256=aa\n1,v,SHA256=vv\n2,s,SHA256=ss\n")
	got, err := parseStreamHash(out, []int{0, 1, 3})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "SHA256=aa" || got[1] != "SHA256=vv" || got[3] != "SHA256=ss" || len(got) != 3 {
		t.Errorf("hashes = %v", got)
	}
}

func TestParseStreamHash_RefusesShortOrGarbledOutput(t *testing.T) {
	for name, out := range map[string]string{
		"a stream missing": "0,a,SHA256=aa\n",
		"garbled line":     "0 a aa\n1,v,SHA256=vv\n",
		"out of range":     "0,a,SHA256=aa\n7,v,SHA256=vv\n",
	} {
		if _, err := parseStreamHash([]byte(out), []int{0, 1}); err == nil {
			t.Errorf("%s: accepted %q", name, out)
		}
	}
}

func TestParseXMP_DropsSourceFileKeepsJSONValues(t *testing.T) {
	out := []byte(`[{"SourceFile":"/m/a.mp4","XMP-prism:Edition":"Director's Cut","XMP-dc:Subject":["a","b"]}]`)
	got, err := parseXMP(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, has := got["xmp:SourceFile"]; has {
		t.Error("SourceFile compared: it differs between original and candidate by construction")
	}
	if got["xmp:XMP-prism:Edition"] != `"Director's Cut"` || got["xmp:XMP-dc:Subject"] != `["a","b"]` {
		t.Errorf("facts = %v", got)
	}
}

func TestParseMkvIdentity_KeepsFullTrackUID(t *testing.T) {
	// 18446744073709551000 does not survive a float64 round trip.
	out := []byte(`{"tracks":[{"id":0,"properties":{"uid":18446744073709551000}}],
		"attachments":[{"id":1,"file_name":"font.ttf","content_type":"font/ttf","description":"","size":1234}]}`)
	id, err := parseMkvIdentity(out)
	if err != nil {
		t.Fatal(err)
	}
	if uid, ok := id.trackUID(0); !ok || uid != "18446744073709551000" {
		t.Errorf("uid = %q, %v", uid, ok)
	}
	if got := id.attachmentFacts()["attachment:0:font.ttf"]; got != "font/ttf|1234|" {
		t.Errorf("attachment facts = %v", id.attachmentFacts())
	}
}

const tagsDoc = `<?xml version="1.0"?>
<Tags>
  <Tag><Targets><TargetTypeValue>50</TargetTypeValue></Targets>
    <Simple><Name>TITLE</Name><String>Film</String></Simple></Tag>
  <Tag><Targets><TargetTypeValue>50</TargetTypeValue><TrackUID>111</TrackUID></Targets>
    <Simple><Name>BPS</Name><String>8000000</String></Simple>
    <Simple><Name>_STATISTICS_WRITING_APP</Name><String>mkvmerge</String></Simple></Tag>
  <Tag><Targets><TargetTypeValue>30</TargetTypeValue><TrackUID>111</TrackUID></Targets>
    <Simple><Name>ENCODER_SETTINGS</Name><String>crf=18 &amp; slow</String></Simple>
    <Simple><Name>NUMBER_OF_BYTES</Name><String>99</String></Simple></Tag>
  <Tag><Targets><TrackUID>222</TrackUID></Targets>
    <Simple><Name>LANGUAGE_NOTE</Name><String>audio</String></Simple></Tag>
</Tags>`

func TestTrackTagsXML_CarriesOnlyTheTracksOwnNonStatTags(t *testing.T) {
	out, ok, err := trackTagsXML(tagsDoc, "111")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	for _, want := range []string{"<TargetTypeValue>30</TargetTypeValue>", "ENCODER_SETTINGS", "crf=18 &amp; slow"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"TrackUID", "BPS", "NUMBER_OF_BYTES", "_STATISTICS", "TITLE", "LANGUAGE_NOTE"} {
		if strings.Contains(out, bad) {
			t.Errorf("output carries %q:\n%s", bad, out)
		}
	}
	if strings.Count(out, "<Tag>") != 1 {
		t.Errorf("a statistics-only tag survived as an empty <Tag>:\n%s", out)
	}
}

func TestTrackTagsXML_NothingToCarry(t *testing.T) {
	for name, doc := range map[string]string{"no tags at all": "", "only statistics": tagsDoc} {
		uid := "111"
		if name == "only statistics" {
			uid = "333" // no tag targets this track
		}
		if _, ok, err := trackTagsXML(doc, uid); ok || err != nil {
			t.Errorf("%s: ok=%v err=%v", name, ok, err)
		}
	}
}
