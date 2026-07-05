package repo_test

import (
	"context"
	"testing"

	"holodex/internal/repo"
)

func TestProviderFieldHints_ReplaceAndRead(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.ReplaceProviderFieldHints(ctx, "tmdb", []repo.ProviderFieldHint{
		{FieldKey: "gender", Label: "Gender", Render: "text", Group: "attributes", Order: 3},
		{FieldKey: "trivia", Label: "Trivia", Render: "long_text", Group: "extended"},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := r.ReplaceProviderFieldHints(ctx, "acme", []repo.ProviderFieldHint{
		{FieldKey: "gender", Label: "Sex", Render: "text"},
	}); err != nil {
		t.Fatalf("replace acme: %v", err)
	}

	all, err := r.ProviderFieldHints(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := all["tmdb"]["gender"]; got.Label != "Gender" || got.Group != "attributes" || got.Order != 3 {
		t.Fatalf("tmdb/gender wrong: %+v", got)
	}
	if got := all["tmdb"]["trivia"]; got.Render != "long_text" {
		t.Fatalf("tmdb/trivia wrong: %+v", got)
	}
	// Providers are isolated: acme's gender is its own row.
	if got := all["acme"]["gender"]; got.Label != "Sex" {
		t.Fatalf("acme/gender wrong: %+v", got)
	}
}

func TestProviderFieldHints_ReplaceDropsRemovedKeys(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.ReplaceProviderFieldHints(ctx, "tmdb", []repo.ProviderFieldHint{
		{FieldKey: "gender", Label: "Gender"},
		{FieldKey: "trivia", Label: "Trivia"},
	}); err != nil {
		t.Fatal(err)
	}
	// Re-describe now advertises only gender → trivia must be dropped.
	if err := r.ReplaceProviderFieldHints(ctx, "tmdb", []repo.ProviderFieldHint{
		{FieldKey: "gender", Label: "Gender v2"},
	}); err != nil {
		t.Fatal(err)
	}
	all, err := r.ProviderFieldHints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all["tmdb"]["trivia"]; ok {
		t.Fatal("a key the provider stopped advertising must be dropped on refresh")
	}
	if got := all["tmdb"]["gender"]; got.Label != "Gender v2" {
		t.Fatalf("gender should be updated: %+v", got)
	}
}

func TestProviderFieldHints_EmptyClears(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.ReplaceProviderFieldHints(ctx, "tmdb", []repo.ProviderFieldHint{{FieldKey: "gender"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceProviderFieldHints(ctx, "tmdb", nil); err != nil {
		t.Fatal(err)
	}
	all, err := r.ProviderFieldHints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all["tmdb"]; ok {
		t.Fatalf("empty replace should clear the provider's rows, got %+v", all["tmdb"])
	}
}
