package writeback

import (
	"context"
	"strings"
	"testing"
)

func TestValidTagKeyName(t *testing.T) {
	for _, tc := range []struct {
		container, name string
		want            bool
	}{
		{"MP4", "Keys:Keywords", true},
		{"MP4", "ItemList:Category", true},
		{"MP4", "XMP-dc:Keywords", true},
		{"MP4", "Keywords", false},         // exiftool containers name the group
		{"MP4", "Keys:Genre", false},       // Genre is the canonical field's, not a derived key
		{"MP4", "System:FileName", false},  // anything else is refused outright
		{"MP4", "Keys;rm:Keywords", false}, // group must be a plain group name
		{"Matroska", "Keywords", true},     // Matroska/WebM write by bare name
		{"WebM", "categories", true},       // case-insensitive, like the scanner
		{"Matroska", "Matroska:Keywords", false},
	} {
		if got := ValidTagKeyName(tc.container, tc.name); got != tc.want {
			t.Errorf("ValidTagKeyName(%q, %q) = %v, want %v", tc.container, tc.name, got, tc.want)
		}
	}
}

func TestFilterTagKeys(t *testing.T) {
	keep := func(v string) bool { return strings.EqualFold(v, "drama") }
	got := FilterTagKeys(map[string][]string{
		"Keys:Keywords":     {"Drama", "Heist"},
		"ItemList:Category": {"Heist"},
		"Keys:Genres":       {"Drama"},
	}, keep, "manual")

	if len(got) != 2 {
		t.Fatalf("got %d writes, want 2 (the unchanged key is skipped): %+v", len(got), got)
	}
	// Sorted by tag name.
	if c := got[0]; c.TagName != "ItemList:Category" || !c.Delete || len(c.Values) != 0 ||
		c.Field != TagKeyFieldPrefix+"ItemList:Category" {
		t.Errorf("Category = %+v, want a delete", c)
	}
	if k := got[1]; k.TagName != "Keys:Keywords" || k.Delete || len(k.Values) != 1 || k.Values[0] != "Drama" {
		t.Errorf("Keywords = %+v, want [Drama]", k)
	}
}

func TestSplitTagValue(t *testing.T) {
	got := splitTagValue([]any{"Drama", "Crime, Heist", 1999})
	want := []string{"Drama", "Crime", "Heist", "1999"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("splitTagValue = %q, want %q", got, want)
	}
}

func TestWriteBatch_DeleteMustCarryNoValues(t *testing.T) {
	for _, f := range []FieldWrite{
		{TagName: "Genre", Values: []string{"x"}, Delete: true},
		{TagName: "Genre"},
		{TagName: "cover.jpg", IsImage: true, Delete: true},
	} {
		if err := WriteBatch(context.Background(), "clip.mp4", []FieldWrite{f}); err == nil {
			t.Errorf("WriteBatch accepted %+v", f)
		}
	}
}

func TestMergeTagsXML_Delete(t *testing.T) {
	const existing = `<?xml version="1.0"?>
<Tags>
<Tag>
<Targets />
<Simple><Name>GENRE</Name><String>Drama</String></Simple>
<Simple><Name>KEYWORDS</Name><String>Heist</String></Simple>
</Tag>
</Tags>`

	got, err := mergeTagsXML(existing, []FieldWrite{{TagName: "Keywords", Delete: true}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "KEYWORDS") || !strings.Contains(got, "<Name>GENRE</Name><String>Drama</String>") {
		t.Errorf("want KEYWORDS gone and GENRE kept:\n%s", got)
	}

	// A batch of only deletes that empties the Tag emits no Tag: Matroska
	// requires every Tag to carry a Simple.
	got, err = mergeTagsXML(existing, []FieldWrite{{TagName: "Genre", Delete: true}, {TagName: "Keywords", Delete: true}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<Tag>") {
		t.Errorf("an emptied Tag was emitted:\n%s", got)
	}
}
