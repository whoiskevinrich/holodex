package resolver_test

import (
	"reflect"
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/resolver"
)

// F30.3g: the owner's detail read lists each merge field's removals (testing-strategy §34.1).

func suppressed(stored ...string) resolver.FieldCuration {
	fc := resolver.FieldCuration{Suppress: map[string]bool{}, SuppressedAs: map[string]string{}}
	for _, s := range stored {
		nk := resolver.NormKey(s)
		fc.Suppress[nk] = true
		fc.SuppressedAs[nk] = s
	}
	return fc
}

func TestRemoved_ListsStoredSpellingAndNotSent(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"genres": {"Drama", "Action"}}}
	cur := resolver.Curation{"genres": suppressed("Drama", "Western")}
	got := resolver.Resolve(testVideo, nil, enr, cur,
		[]mapping.Field{mergeField("genres", "tmdb:genres")}, resolver.Options{ListRemoved: true})
	want := []resolver.RemovedValue{{Value: "Drama"}, {Value: "Western", NotSent: true}}
	if !reflect.DeepEqual(got[0].Removed, want) {
		t.Fatalf("removed = %+v, want %+v", got[0].Removed, want)
	}
	if !reflect.DeepEqual(got[0].Values, []string{"Action"}) {
		t.Errorf("values = %v, want [Action]", got[0].Values)
	}
}

func TestRemoved_ManualAddCountsAsSent(t *testing.T) {
	// A manual add of a removed value stays hidden (suppress wins), but lifting the
	// removal would bring it back, so it is not "not sent now".
	fc := suppressed("Western")
	fc.Add = []string{"Western"}
	got := resolver.Resolve(testVideo, nil, resolver.Enrichment{"tmdb": {"genres": {"Drama"}}},
		resolver.Curation{"genres": fc}, []mapping.Field{mergeField("genres", "tmdb:genres")},
		resolver.Options{ListRemoved: true})
	if !reflect.DeepEqual(got[0].Values, []string{"Drama"}) {
		t.Errorf("values = %v, want [Drama]: suppress beats a manual add", got[0].Values)
	}
	if want := []resolver.RemovedValue{{Value: "Western"}}; !reflect.DeepEqual(got[0].Removed, want) {
		t.Errorf("removed = %+v, want %+v", got[0].Removed, want)
	}
}

func TestRemoved_EveryValueRemovedKeepsFieldForOwnerOnly(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"genres": {"Drama"}}}
	cur := resolver.Curation{"genres": suppressed("Drama")}
	fields := []mapping.Field{mergeField("genres", "tmdb:genres")}

	owner := resolver.Resolve(testVideo, nil, enr, cur, fields, resolver.Options{ListRemoved: true})
	if len(owner) != 1 || len(owner[0].Values) != 0 || len(owner[0].Removed) != 1 {
		t.Fatalf("owner read should keep the empty field with one removal, got %+v", owner)
	}
	if visitor := resolver.Resolve(testVideo, nil, enr, cur, fields, resolver.Options{}); len(visitor) != 0 {
		t.Errorf("visitor read should drop the empty field, got %+v", visitor)
	}
}

func TestRemoved_OptionOffIsUnchanged(t *testing.T) {
	enr := resolver.Enrichment{"tmdb": {"genres": {"Drama", "Action"}}}
	cur := resolver.Curation{"genres": suppressed("Drama")}
	fields := []mapping.Field{mergeField("genres", "tmdb:genres"), stubField("title", false, "tmdb:title", "file:title")}
	off := resolver.Resolve(testVideo, nil, enr, cur, fields, resolver.Options{})
	for _, rf := range off {
		if rf.Removed != nil {
			t.Fatalf("%s carries removals without ListRemoved: %+v", rf.Canonical, rf.Removed)
		}
	}
	on := resolver.Resolve(testVideo, nil, enr, cur, fields, resolver.Options{ListRemoved: true})
	for i := range on {
		on[i].Removed = nil
	}
	if !reflect.DeepEqual(on, off) {
		t.Errorf("ListRemoved changed more than Removed:\n on=%+v\noff=%+v", on, off)
	}
}

func TestRemoved_NotOnReplaceFieldsOrOtherActions(t *testing.T) {
	fc := suppressed("Old Title")
	fc.NoWrite = map[string]bool{"action": true}
	got := resolver.Resolve(testVideo, nil, resolver.Enrichment{"tmdb": {"title": {"Old Title"}, "genres": {"Action"}}},
		resolver.Curation{"title": fc, "genres": {NoWrite: map[string]bool{"action": true}, Add: []string{"Noir"}}},
		[]mapping.Field{stubField("title", false, "tmdb:title"), mergeField("genres", "tmdb:genres")},
		resolver.Options{ListRemoved: true})
	for _, rf := range got {
		if rf.Removed != nil {
			t.Errorf("%s: only suppress rows on merge fields are removals, got %+v", rf.Canonical, rf.Removed)
		}
	}
}

func TestRemoved_PersonBaseline(t *testing.T) {
	got := resolver.ResolveFields(resolver.NewPersonBaseline(testPerson),
		resolver.Enrichment{"tmdb": {"genres": {"Drama"}}},
		resolver.Curation{"genres": suppressed("Drama")},
		[]mapping.Field{mergeField("genres", "tmdb:genres")}, resolver.Options{ListRemoved: true})
	if len(got) != 1 || !reflect.DeepEqual(got[0].Removed, []resolver.RemovedValue{{Value: "Drama"}}) {
		t.Fatalf("person read should list the removal, got %+v", got)
	}
}
