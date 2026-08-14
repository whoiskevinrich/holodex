package writeback

import (
	"context"
	"errors"
	"os"
	"testing"
)

// withImageFetcher installs fn as the package's guarded downloader for the
// duration of the test and restores the prior value afterward, so a test can't
// leak a fetcher into an unrelated test in this package (HOLODEX-212).
func withImageFetcher(t *testing.T, fn ImageFetcher) {
	t.Helper()
	prev := imageFetch
	imageFetch = fn
	t.Cleanup(func() { imageFetch = prev })
}

// TestDownloadImageToTemp_RefusesWithNoFetcherConfigured is the fail-closed
// contract (HOLODEX-212): an unwired imageFetch must refuse the download, never
// fall back to an unguarded fetch.
func TestDownloadImageToTemp_RefusesWithNoFetcherConfigured(t *testing.T) {
	withImageFetcher(t, nil)
	if _, _, err := downloadImageToTemp(context.Background(), "https://cdn.example.com/poster.jpg"); err == nil {
		t.Fatal("want error when no image fetcher is configured, got nil")
	}
}

// TestDownloadImageToTemp_RefusesNonHTTPS confirms the scheme check still runs
// before the guarded fetcher is even consulted.
func TestDownloadImageToTemp_RefusesNonHTTPS(t *testing.T) {
	withImageFetcher(t, func(ctx context.Context, rawURL string) ([]byte, error) {
		t.Fatal("fetcher must not be called for a non-https URL")
		return nil, nil
	})
	if _, _, err := downloadImageToTemp(context.Background(), "http://cdn.example.com/poster.jpg"); err == nil {
		t.Fatal("want error for a non-https URL, got nil")
	}
}

// TestDownloadImageToTemp_PropagatesFetcherRefusal confirms a host the guarded
// fetcher refuses (not allowlisted by any enabled provider) surfaces as an
// error rather than downloading anyway.
func TestDownloadImageToTemp_PropagatesFetcherRefusal(t *testing.T) {
	refused := errors.New("image host not allowlisted by any enabled provider")
	withImageFetcher(t, func(ctx context.Context, rawURL string) ([]byte, error) {
		return nil, refused
	})
	_, _, err := downloadImageToTemp(context.Background(), "https://evil.example.com/poster.jpg")
	if err == nil || !errors.Is(err, refused) {
		t.Fatalf("want the fetcher's refusal to propagate, got %v", err)
	}
}

// TestDownloadImageToTemp_WritesAllowedBytesToTemp confirms an allowed fetch
// writes the returned bytes to a cleanup-able temp file untouched.
func TestDownloadImageToTemp_WritesAllowedBytesToTemp(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	var gotURL string
	withImageFetcher(t, func(ctx context.Context, rawURL string) ([]byte, error) {
		gotURL = rawURL
		return png, nil
	})

	path, cleanup, err := downloadImageToTemp(context.Background(), "https://cdn.example.com/poster.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if gotURL != "https://cdn.example.com/poster.png" {
		t.Errorf("fetcher called with %q, want the original URL", gotURL)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	if string(data) != string(png) {
		t.Errorf("temp file contents = %v, want %v", data, png)
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("want temp file removed after cleanup, stat err = %v", err)
	}
}
