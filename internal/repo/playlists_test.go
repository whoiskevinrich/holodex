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
	// F90 P0-6 (testing-strategy §42.3 Q3): a stored Completeness sort reads as the default,
	// and a new one can't be saved.
	for _, retired := range []string{"completeness_asc", "completeness_desc"} {
		if ValidSort(retired) {
			t.Errorf("ValidSort(%q) = true after the score was retired", retired)
		}
		if clause, _ := (VideoFilter{Sort: retired}).orderBy(); clause != defaultClause {
			t.Errorf("orderBy(%q) = %q, want the default", retired, clause)
		}
		if clause, _ := (NamedListFilter{Sort: retired}).orderBy("person", "e.id"); clause != "b.lbl COLLATE NOCASE ASC, e.id ASC" {
			t.Errorf("named orderBy(%q) = %q, want name order", retired, clause)
		}
	}
	for _, key := range []string{
		"added_asc", "updated_desc", "updated_asc", "title_asc", "title_desc", "duration_desc", "duration_asc",
		"resolution_desc", "resolution_asc", "random", SortEnrichedDesc, SortEnrichedAsc,
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
