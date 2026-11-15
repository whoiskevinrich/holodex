package api_test

import (
	"context"
	"testing"
	"time"
)

// ADR-112 D2: the boot drain empties the dirty set off the request path, and
// the owner's first list read then finds the missing facets it stored.
func TestDrainCompletenessInBackground_FillsStoreBeforeFirstRead(t *testing.T) {
	srv, r, h := completenessBrowseFixture(t, "secret")
	ctx := context.Background()

	done := h.DrainCompletenessInBackground(ctx)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("background drain did not finish")
	}
	if dirty, err := r.DirtyCompleteness(ctx); err != nil || len(dirty["video"]) != 0 {
		t.Fatalf("dirty after background drain = %v, %v; want empty", dirty, err)
	}

	_, body := getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	got := itemsByTitle(t, body, "title")
	for title, want := range map[string]int{"Bare": 3, "Half": 2, "Full": 0} {
		if n := len(storedMissing(t, r, int64(got[title]["id"].(float64)))); n != want {
			t.Errorf("%s missing %d facets, want %d", title, n, want)
		}
	}
}

// ADR-112 D2: while the boot drain runs, an owner read serves the store as it
// stands instead of queueing behind it; once it is done, reads drain again.
func TestListMedia_SkipsDrainWhileBootDrainRuns(t *testing.T) {
	srv, r, h := completenessBrowseFixture(t, "secret")
	ctx := context.Background()

	h.SetBootDrain(true)
	getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	if dirty, _ := r.DirtyCompleteness(ctx); len(dirty["video"]) != 3 {
		t.Fatalf("dirty during boot drain = %v; want the 3 seeded videos untouched", dirty)
	}

	h.SetBootDrain(false)
	getJSONTok(t, srv.URL+"/api/v1/media", "secret")
	if dirty, _ := r.DirtyCompleteness(ctx); len(dirty["video"]) != 0 {
		t.Fatalf("dirty after boot drain = %v; want the owner read to drain", dirty)
	}
}
