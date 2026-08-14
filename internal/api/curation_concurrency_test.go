package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"holodex/internal/repo"
)

// TestCurationAPI_PeopleConcurrentDifferentFields_NoLostUpdate is the regression
// test for HOLODEX-277 (ADR-084): two concurrent curation adds to different
// person-typed fields (actors, director) on the same video must both survive in
// video_people.
//
// Before ADR-084, proposedPeopleLinks (curation.go) computed each request's
// resulting link set from a PeopleForVideos snapshot taken inside
// SetCurationChecked's writeMu lock, but relinkPeopleWithContext committed that
// snapshot via ReconcileVideoPeople (a separate, later writeMu acquisition) —
// leaving an unlocked gap in which a concurrent request's own snapshot-then-commit
// could interleave. Whichever request's full-replace ReconcileVideoPeople write
// landed second silently dropped the other's link, because its snapshot was
// captured before the other's write ran (see ADR-084's Context for the exact
// interleaving). SetCurationChecked's commit callback now runs the relink write
// inside the same lock as the curation write, serializing each request's entire
// check-write-relink cycle against every other request's — so no interleaving is
// possible regardless of goroutine scheduling. Run over many rounds of genuine
// concurrent HTTP requests: the fix makes every round deterministic, not merely
// likely, to preserve both links.
func TestCurationAPI_PeopleConcurrentDifferentFields_NoLostUpdate(t *testing.T) {
	srv, r, id := actorsAndDirectorServer(t)
	ctx := context.Background()
	base := srv.URL + "/api/v1/media/" + itoa(id) + "/curation"

	const rounds = 20
	for round := 0; round < rounds; round++ {
		type result struct {
			code int
			err  error
		}
		results := make(chan result, 2)
		go func() {
			code, err := postCurationNoFatal(base, map[string]any{"field": "actors", "value": "Alice", "action": "add"})
			results <- result{code, err}
		}()
		go func() {
			code, err := postCurationNoFatal(base, map[string]any{"field": "director", "value": "Bob", "action": "add"})
			results <- result{code, err}
		}()
		for i := 0; i < 2; i++ {
			res := <-results
			if res.err != nil {
				t.Fatalf("round %d: request error: %v", round, res.err)
			}
			if res.code != http.StatusNoContent {
				t.Fatalf("round %d: want 204, got %d", round, res.code)
			}
		}

		people, err := r.PeopleForVideos(ctx, []int64{id})
		if err != nil {
			t.Fatalf("round %d: people for video: %v", round, err)
		}
		byRole := map[string]string{}
		for _, p := range people[id] {
			byRole[p.Role] = p.Name
		}
		if byRole["actor"] != "Alice" || byRole["director"] != "Bob" {
			t.Fatalf("round %d: concurrent adds to different person-typed fields lost an update, got %v", round, people[id])
		}

		// Reset to the empty baseline before the next round.
		if code, err := postCurationNoFatal(base, map[string]any{"field": "actors", "value": "Alice", "action": "suppress"}); err != nil || code != http.StatusNoContent {
			t.Fatalf("round %d: cleanup suppress actors: code=%d err=%v", round, code, err)
		}
		if code, err := postCurationNoFatal(base, map[string]any{"field": "director", "value": "Bob", "action": "suppress"}); err != nil || code != http.StatusNoContent {
			t.Fatalf("round %d: cleanup suppress director: code=%d err=%v", round, code, err)
		}
	}
}

// postCurationNoFatal is doJSONRequest (decisions_test.go) specialized to an
// unauthenticated POST — this test's fixture leaves the owner-token gate open.
func postCurationNoFatal(url string, body any) (int, error) {
	return doJSONRequest(http.MethodPost, url, "", body)
}

// actorsAndDirectorServer is peopleDecisionServerWithFields (curation_collision_test.go)
// with both person-typed fields (actors, director) mapped, so a concurrent add to
// each can be raced against the other — peopleDecisionServer's default fixture
// deliberately maps only actors (needed by TestCurationAPI_PersonFieldNotMapped's
// unmapped-field coverage).
func actorsAndDirectorServer(t *testing.T) (*httptest.Server, *repo.Repo, int64) {
	t.Helper()
	return peopleDecisionServerWithFields(t, "fields:\n"+
		"  - canonical: title\n    label: Title\n    sources: [tmdb:title, file:title]\n"+
		"  - canonical: actors\n    label: Actors\n    multi: true\n    sources: [tmdb:actors, file:Artist]\n"+
		"  - canonical: director\n    label: Director\n    sources: [tmdb:director, file:director]\n")
}
