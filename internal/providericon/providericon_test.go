package providericon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStoreRoundTrip covers the disk layout (ADR-059): Store creates the dir and writes
// the id-named JPEG, ImagePath resolves it, and Remove is idempotent. The filename is
// the server-assigned row id only — the provider name never appears in the path.
func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	const id int64 = 7
	data := []byte("not-a-real-jpeg-but-bytes-are-opaque-here")

	if err := Store(dir, id, data); err != nil {
		t.Fatalf("store: %v", err)
	}
	path := ImagePath(dir, id)
	if !strings.HasSuffix(path, filepath.FromSlash("/7.jpg")) {
		t.Fatalf("path = %q, want .../7.jpg", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stored: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("stored bytes mismatch")
	}
	// No leftover temp file.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file lingered: %v", err)
	}

	if err := Remove(dir, id); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still present after remove")
	}
	// Idempotent — removing an absent file is a no-op success.
	if err := Remove(dir, id); err != nil {
		t.Fatalf("remove idempotent: %v", err)
	}
}
