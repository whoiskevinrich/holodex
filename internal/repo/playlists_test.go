package repo

import "testing"

// TestValidSortMatchesOrderBy pins ValidSort to orderBy's explicit cases (F69,
// ADR-104 D2): every key ValidSort accepts must get its own ORDER BY clause, and
// every key orderBy handles must be accepted — otherwise a persisted playlist
// sort could silently fall to the added_desc default, or browse could honour a
// key the playlist API rejects.
func TestValidSortMatchesOrderBy(t *testing.T) {
	defaultClause, _ := VideoFilter{Sort: "definitely-unknown"}.orderBy()
	addedDesc, _ := VideoFilter{Sort: "added_desc"}.orderBy()
	if defaultClause != addedDesc {
		t.Fatalf("orderBy default %q is not added_desc %q", defaultClause, addedDesc)
	}
	for _, key := range []string{
		"added_asc", "title_asc", "title_desc", "duration_desc", "duration_asc",
		"resolution_desc", "resolution_asc", "random", SortCompletenessAsc, SortCompletenessDesc,
	} {
		if !ValidSort(key) {
			t.Errorf("ValidSort(%q) = false, but orderBy handles it", key)
		}
		if clause, _ := (VideoFilter{Sort: key}).orderBy(); clause == defaultClause {
			t.Errorf("orderBy(%q) fell to the default clause — ValidSort accepts a key orderBy ignores", key)
		}
	}
	if !ValidSort("added_desc") {
		t.Error("ValidSort(added_desc) = false")
	}
	for _, bad := range []string{"", "manual", "sideways"} {
		if ValidSort(bad) {
			t.Errorf("ValidSort(%q) = true", bad)
		}
	}
}
