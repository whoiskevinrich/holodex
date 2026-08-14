package repo_test

import (
	"context"
	"testing"

	"holodex/internal/repo"
)

func TestProviderLinkTemplates_ReplaceAndRead(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.ReplaceProviderLinkTemplates(ctx, "tmdb", []repo.ProviderLinkTemplate{
		{Namespace: "imdb", EntityType: "video", Template: "https://www.imdb.com/title/{id}/"},
		{Namespace: "imdb", EntityType: "person", Template: "https://www.imdb.com/name/{id}/"},
		{Namespace: "tmdb", EntityType: "video", Template: "https://www.themoviedb.org/movie/{id}"},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}

	all, err := r.ProviderLinkTemplates(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := all["imdb"]["video"]; got != "https://www.imdb.com/title/{id}/" {
		t.Fatalf("imdb/video wrong: %q", got)
	}
	if got := all["imdb"]["person"]; got != "https://www.imdb.com/name/{id}/" {
		t.Fatalf("imdb/person wrong: %q", got)
	}
	if got := all["tmdb"]["video"]; got != "https://www.themoviedb.org/movie/{id}" {
		t.Fatalf("tmdb/video wrong: %q", got)
	}
}

// A namespace is a shared identity space across providers (ADR-055 D2), so the row
// for (namespace, entity_type) is owned by whichever provider most recently
// advertised a template for it — not scoped per-provider like field hints.
func TestProviderLinkTemplates_LastDescribeWinsAcrossProviders(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.ReplaceProviderLinkTemplates(ctx, "acme", []repo.ProviderLinkTemplate{
		{Namespace: "imdb", EntityType: "video", Template: "https://acme.example/{id}"},
	}); err != nil {
		t.Fatal(err)
	}
	// tmdb's /describe also claims the imdb namespace — its row wins.
	if err := r.ReplaceProviderLinkTemplates(ctx, "tmdb", []repo.ProviderLinkTemplate{
		{Namespace: "imdb", EntityType: "video", Template: "https://www.imdb.com/title/{id}/"},
	}); err != nil {
		t.Fatal(err)
	}
	all, err := r.ProviderLinkTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := all["imdb"]["video"]; got != "https://www.imdb.com/title/{id}/" {
		t.Fatalf("most recently described provider should own the row, got %q", got)
	}
}

func TestProviderLinkTemplates_ReplaceOnlyClearsOwnProviderRows(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if err := r.ReplaceProviderLinkTemplates(ctx, "tmdb", []repo.ProviderLinkTemplate{
		{Namespace: "tmdb", EntityType: "video", Template: "https://www.themoviedb.org/movie/{id}"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceProviderLinkTemplates(ctx, "acme", []repo.ProviderLinkTemplate{
		{Namespace: "acme", EntityType: "video", Template: "https://acme.example/{id}"},
	}); err != nil {
		t.Fatal(err)
	}
	// acme's re-describe stops advertising anything — only its own row is dropped.
	if err := r.ReplaceProviderLinkTemplates(ctx, "acme", nil); err != nil {
		t.Fatal(err)
	}
	all, err := r.ProviderLinkTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all["acme"]; ok {
		t.Fatalf("acme's dropped namespace should be gone, got %+v", all["acme"])
	}
	if got := all["tmdb"]["video"]; got == "" {
		t.Fatal("tmdb's row must survive acme's unrelated replace")
	}
}
