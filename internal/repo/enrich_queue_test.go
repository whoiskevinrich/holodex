package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

// --- GET /owner/enrich-queue backing query (F47, ADR-066 RD2/RD9/P0-1) ---------------

func TestEnrichQueue_MembershipAndStates(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	idA, _ := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", []string{"Alice"}, nil), nil)
	linkPeople(t, r, idA, "Alice")
	idB, _ := r.UpsertVideo(ctx, sampleVideo("/m/b.mkv", "B", []string{"Bob"}, nil), nil)
	linkPeople(t, r, idB, "Bob")
	alice := personIDByName(t, r, "Alice")
	bob := personIDByName(t, r, "Bob")

	providers := map[string][]string{model.EnrichEntityPerson: {"tmdb", "other"}}

	// Neither person has any enrichment or dismissal yet — both appear, each carrying
	// both providers as 'unreviewed'.
	rows, err := r.EnrichQueue(ctx, providers)
	if err != nil {
		t.Fatalf("enrich queue: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2", rows)
	}
	for _, row := range rows {
		if len(row.Providers) != 2 || row.Providers[0].State != "unreviewed" || row.Providers[1].State != "unreviewed" {
			t.Errorf("row %+v, want both providers unreviewed", row)
		}
	}

	// Linking one provider for Alice removes it from her row (nothing to review), but
	// her row stays (the other provider is still outstanding).
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityPerson, alice, "tmdb", "ext-1", map[string][]string{"bio": {"x"}}); err != nil {
		t.Fatalf("upsert enrichment: %v", err)
	}
	rows, err = r.EnrichQueue(ctx, providers)
	if err != nil {
		t.Fatalf("enrich queue: %v", err)
	}
	foundAlice := false
	for _, row := range rows {
		if row.EntityID == alice {
			foundAlice = true
			if len(row.Providers) != 1 || row.Providers[0].Provider != "other" {
				t.Fatalf("alice row after linking tmdb = %+v, want only 'other'", row)
			}
		}
	}
	if !foundAlice {
		t.Fatalf("alice must still be a member (one outstanding provider): %+v", rows)
	}

	// Dismissing Alice's remaining provider ('other') drops her from the queue entirely
	// — zero non-dismissed outstanding providers means no membership (testing-strategy's
	// stated rule), even though the row would still show a 'not_matched' chip if she
	// stayed a member via some other provider.
	if err := r.DismissEnrichment(ctx, model.EnrichEntityPerson, alice, "other"); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	rows, err = r.EnrichQueue(ctx, providers)
	if err != nil {
		t.Fatalf("enrich queue: %v", err)
	}
	if len(rows) != 1 || rows[0].EntityID != bob {
		t.Fatalf("after fully dismissing alice, only bob should remain: %+v", rows)
	}

	// Dismissing only ONE of Bob's two providers keeps him a member (the other is still
	// unreviewed) and shows both chips — 'not_matched' for the dismissed one.
	if err := r.DismissEnrichment(ctx, model.EnrichEntityPerson, bob, "tmdb"); err != nil {
		t.Fatalf("dismiss bob tmdb: %v", err)
	}
	rows, err = r.EnrichQueue(ctx, providers)
	if err != nil {
		t.Fatalf("enrich queue: %v", err)
	}
	if len(rows) != 1 || rows[0].EntityID != bob || len(rows[0].Providers) != 2 {
		t.Fatalf("bob row = %+v, want both providers listed", rows)
	}
	states := map[string]string{}
	for _, p := range rows[0].Providers {
		states[p.Provider] = p.State
	}
	if states["tmdb"] != "not_matched" || states["other"] != "unreviewed" {
		t.Errorf("bob provider states = %+v, want tmdb=not_matched other=unreviewed", states)
	}
}

// Grouping is fixed nav order (spec Q3): person, studio, then video — asserted via a
// mixed-type providers map with one qualifying entity of each type.
func TestEnrichQueue_EntityTypeOrder(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A Video", []string{"Alice"}, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	linkPeople(t, r, vid, "Alice")
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Acme Studio"}, nil); err != nil {
		t.Fatalf("seed studio: %v", err)
	}

	providers := map[string][]string{
		model.EnrichEntityVideo:  {"tmdb"},
		model.EnrichEntityStudio: {"tmdb"},
		model.EnrichEntityPerson: {"tmdb"},
	}
	rows, err := r.EnrichQueue(ctx, providers)
	if err != nil {
		t.Fatalf("enrich queue: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3", rows)
	}
	want := []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EnrichEntityVideo}
	for i, et := range want {
		if rows[i].EntityType != et {
			t.Errorf("rows[%d].EntityType = %q, want %q (order: %+v)", i, rows[i].EntityType, et, rows)
		}
	}
}

// A trashed (soft-deleted) video has no business in the owner's backlog.
func TestEnrichQueue_ExcludesSoftDeletedVideo(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, sampleVideo("/m/a.mkv", "A", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.SoftDelete(ctx, vid); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	rows, err := r.EnrichQueue(ctx, map[string][]string{model.EnrichEntityVideo: {"tmdb"}})
	if err != nil {
		t.Fatalf("enrich queue: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %+v, want none (video is trashed)", rows)
	}
}
