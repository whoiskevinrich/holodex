package repo_test

import (
	"context"
	"testing"

	"holodex/internal/model"
)

// TestClaims_SetListClear covers the store's grain (F49, ADR-074 D1): claims are keyed
// per (entity_type, provider, field_key), listed in append order, upserted by canonical,
// and independent across entity types.
func TestClaims_SetListClear(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if rows, err := r.ClaimsForEntityType(ctx, model.EnrichEntityPerson); err != nil || len(rows) != 0 {
		t.Fatalf("want no claims, got %v err=%v", rows, err)
	}

	// Two providers spelling one field differently, plus one same-named key on another
	// provider that must stay independent.
	if err := r.SetClaim(ctx, model.EnrichEntityPerson, "tmdb", "biography", "bio"); err != nil {
		t.Fatalf("set claim: %v", err)
	}
	if err := r.SetClaim(ctx, model.EnrichEntityPerson, "provb", "life_story", "bio"); err != nil {
		t.Fatalf("set claim: %v", err)
	}
	if err := r.SetClaim(ctx, model.EnrichEntityStudio, "tmdb", "biography", "description"); err != nil {
		t.Fatalf("set studio claim: %v", err)
	}

	rows, err := r.ClaimsForEntityType(ctx, model.EnrichEntityPerson)
	if err != nil || len(rows) != 2 {
		t.Fatalf("want 2 person claims, got %v err=%v", rows, err)
	}
	// Listed in (provider, field_key) order — the order they append in, so resolution is
	// reproducible from the table's contents rather than from edit history (D3).
	if rows[0].Provider != "provb" || rows[1].Provider != "tmdb" {
		t.Errorf("claims must list in provider order, got %+v", rows)
	}
	if rows[1].FieldKey != "biography" || rows[1].Canonical != "bio" {
		t.Errorf("claim payload = %+v", rows[1])
	}
	if studio, _ := r.ClaimsForEntityType(ctx, model.EnrichEntityStudio); len(studio) != 1 {
		t.Errorf("entity types are independent stores, got %+v", studio)
	}

	// Re-pointing a claim is an upsert on the PK, not a second row.
	if err := r.SetClaim(ctx, model.EnrichEntityPerson, "tmdb", "biography", "aliases"); err != nil {
		t.Fatalf("re-point claim: %v", err)
	}
	rows, _ = r.ClaimsForEntityType(ctx, model.EnrichEntityPerson)
	if len(rows) != 2 || rows[1].Canonical != "aliases" {
		t.Errorf("re-pointing must upsert in place, got %+v", rows)
	}

	// Clearing is per (provider, field_key) and idempotent.
	if n, err := r.ClearClaim(ctx, model.EnrichEntityPerson, "tmdb", "biography"); err != nil || n != 1 {
		t.Fatalf("clear claim = %d err=%v", n, err)
	}
	if n, err := r.ClearClaim(ctx, model.EnrichEntityPerson, "tmdb", "biography"); err != nil || n != 0 {
		t.Fatalf("idempotent clear = %d err=%v", n, err)
	}
	rows, _ = r.ClaimsForEntityType(ctx, model.EnrichEntityPerson)
	if len(rows) != 1 || rows[0].Provider != "provb" {
		t.Errorf("clearing one provider must leave the other, got %+v", rows)
	}
}

// TestClaims_SetClearsPromotionInSameWrite pins RD3/D5 at the store level: a key may be
// promoted or claimed, never both, and the two writes are one transaction so a claim can
// never land beside a surviving promotion of the same key. The clear is provider-blind by
// design — field_promotions has no provider column, and any provider's claim of the key
// retires the promoted row that would otherwise render it a second time.
func TestClaims_SetClearsPromotionInSameWrite(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.SetPromotion(ctx, model.EnrichEntityPerson, "biography", "Life", "long_text", "primary", 1); err != nil {
		t.Fatalf("set promotion: %v", err)
	}
	if err := r.SetPromotion(ctx, model.EnrichEntityPerson, "handedness", "", "", "", 0); err != nil {
		t.Fatalf("set unrelated promotion: %v", err)
	}
	if err := r.SetClaim(ctx, model.EnrichEntityPerson, "tmdb", "biography", "bio"); err != nil {
		t.Fatalf("set claim: %v", err)
	}

	promos, err := r.PromotionsForEntityType(ctx, model.EnrichEntityPerson)
	if err != nil || len(promos) != 1 || promos[0].FieldKey != "handedness" {
		t.Fatalf("claiming must clear only that key's promotion, got %+v err=%v", promos, err)
	}

	// Un-claiming does not bring it back: the clear is a delete, not a suspension.
	if _, err := r.ClearClaim(ctx, model.EnrichEntityPerson, "tmdb", "biography"); err != nil {
		t.Fatalf("clear claim: %v", err)
	}
	if promos, _ := r.PromotionsForEntityType(ctx, model.EnrichEntityPerson); len(promos) != 1 {
		t.Errorf("removing a claim must not resurrect the promotion, got %+v", promos)
	}
}
