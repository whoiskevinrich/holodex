package extract

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewPatternStore_MissingFileIsEmpty mirrors mapping.Load's "missing file
// is fine" contract (F48.1b): no patterns configured, every filename falls
// through to tag-only resolution unchanged.
func TestNewPatternStore_MissingFileIsEmpty(t *testing.T) {
	s, err := NewPatternStore(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("NewPatternStore: %v", err)
	}
	patterns, delimiter := s.Current()
	if len(patterns) != 0 || delimiter != "" {
		t.Fatalf("Current() = %v, %q; want empty", patterns, delimiter)
	}
}

// TestNewPatternStore_EmptyPathIsEmpty mirrors mapping.Load's "" path
// shortcut.
func TestNewPatternStore_EmptyPathIsEmpty(t *testing.T) {
	s, err := NewPatternStore("")
	if err != nil {
		t.Fatalf("NewPatternStore: %v", err)
	}
	if patterns, _ := s.Current(); len(patterns) != 0 {
		t.Fatalf("Current() = %v, want empty", patterns)
	}
}

// TestNewPatternStore_LoadsAndCompiles proves a real file loads its ordered
// pattern list and delimiter, ready for MatchFirst.
func TestNewPatternStore_LoadsAndCompiles(t *testing.T) {
	path := writePatternFile(t, `
delimiter: "; "
patterns:
  - "[{studio}] {title} ({people}, {year})"
  - "{title} ({year})"
`)
	s, err := NewPatternStore(path)
	if err != nil {
		t.Fatalf("NewPatternStore: %v", err)
	}
	patterns, delimiter := s.Current()
	if len(patterns) != 2 {
		t.Fatalf("got %d patterns, want 2", len(patterns))
	}
	if delimiter != "; " {
		t.Fatalf("delimiter = %q, want %q", delimiter, "; ")
	}
}

// TestNewPatternStore_InvalidPatternFails proves F48.1a's save-time
// validation: an unparseable pattern fails the load rather than silently
// dropping it.
func TestNewPatternStore_InvalidPatternFails(t *testing.T) {
	path := writePatternFile(t, `
patterns:
  - "no tokens here"
`)
	if _, err := NewPatternStore(path); err == nil {
		t.Fatal("want an error for a pattern with no {token} placeholders")
	}
}

// TestPatternStore_Reload proves a runtime reload (POST /admin/reload-config,
// F48.1a) swaps the pattern list in atomically, and a malformed reload leaves
// the previous known-good list in place.
func TestPatternStore_Reload(t *testing.T) {
	path := writePatternFile(t, `
patterns:
  - "{title} ({year})"
`)
	s, err := NewPatternStore(path)
	if err != nil {
		t.Fatalf("NewPatternStore: %v", err)
	}
	if patterns, _ := s.Current(); len(patterns) != 1 {
		t.Fatalf("got %d patterns, want 1", len(patterns))
	}

	if err := os.WriteFile(path, []byte(`
patterns:
  - "{title} ({year})"
  - "[{studio}] {title}"
`), 0o644); err != nil {
		t.Fatalf("rewrite pattern file: %v", err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if patterns, _ := s.Current(); len(patterns) != 2 {
		t.Fatalf("after reload got %d patterns, want 2", len(patterns))
	}

	if err := os.WriteFile(path, []byte(`
patterns:
  - "no tokens here"
`), 0o644); err != nil {
		t.Fatalf("rewrite pattern file: %v", err)
	}
	if err := s.Reload(); err == nil {
		t.Fatal("want an error reloading a malformed pattern file")
	}
	if patterns, _ := s.Current(); len(patterns) != 2 {
		t.Fatalf("after a failed reload got %d patterns, want the previous 2", len(patterns))
	}
}

func writePatternFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metadata-patterns.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write pattern file: %v", err)
	}
	return path
}
