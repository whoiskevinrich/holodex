package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

func TestPromotions_SetListClear(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	// No promotions initially.
	if rows, err := r.PromotionsForEntityType(ctx, model.EnrichEntityPerson); err != nil || len(rows) != 0 {
		t.Fatalf("want no promotions, got %v err=%v", rows, err)
	}

	// Set a promotion with full presentation, and one label-only ("make curatable").
	if err := r.SetPromotion(ctx, model.EnrichEntityPerson, "measurements", "Measurements", "chips", "attributes", 5); err != nil {
		t.Fatalf("set promotion: %v", err)
	}
	if err := r.SetPromotion(ctx, model.EnrichEntityPerson, "handedness", "", "", "", 0); err != nil {
		t.Fatalf("set inherit promotion: %v", err)
	}

	rows, err := r.PromotionsForEntityType(ctx, model.EnrichEntityPerson)
	if err != nil || len(rows) != 2 {
		t.Fatalf("want 2 promotions, got %v err=%v", rows, err)
	}
	byKey := map[string]struct {
		label, render, group string
		order                int
	}{}
	for _, p := range rows {
		byKey[p.FieldKey] = struct {
			label, render, group string
			order                int
		}{p.Label, p.Render, p.Group, p.Order}
	}
	if got := byKey["measurements"]; got.label != "Measurements" || got.render != "chips" || got.group != "attributes" || got.order != 5 {
		t.Errorf("measurements promotion = %+v", got)
	}
	if got := byKey["handedness"]; got.label != "" || got.render != "" || got.group != "" || got.order != 0 {
		t.Errorf("handedness promotion should inherit (all empty), got %+v", got)
	}

	// Promotions are scoped per entity type — none leak to studio.
	if rows, _ := r.PromotionsForEntityType(ctx, model.EnrichEntityStudio); len(rows) != 0 {
		t.Errorf("studio should have no promotions, got %v", rows)
	}

	// Upsert the same key updates presentation in place (still 2 rows).
	if err := r.SetPromotion(ctx, model.EnrichEntityPerson, "measurements", "Vitals", "text", "primary", 1); err != nil {
		t.Fatalf("re-set: %v", err)
	}
	rows, _ = r.PromotionsForEntityType(ctx, model.EnrichEntityPerson)
	if len(rows) != 2 {
		t.Fatalf("upsert should not add a row, got %d", len(rows))
	}
	for _, p := range rows {
		if p.FieldKey == "measurements" && (p.Label != "Vitals" || p.Render != "text" || p.Group != "primary" || p.Order != 1) {
			t.Errorf("upsert should replace presentation, got %+v", p)
		}
	}

	// Clear removes the row; clearing again is an idempotent no-op.
	n, err := r.ClearPromotion(ctx, model.EnrichEntityPerson, "measurements")
	if err != nil || n != 1 {
		t.Fatalf("clear: n=%d err=%v", n, err)
	}
	n, _ = r.ClearPromotion(ctx, model.EnrichEntityPerson, "measurements")
	if n != 0 {
		t.Errorf("second clear should affect 0 rows, got %d", n)
	}
	if rows, _ := r.PromotionsForEntityType(ctx, model.EnrichEntityPerson); len(rows) != 1 {
		t.Errorf("want 1 promotion left, got %d", len(rows))
	}
}
