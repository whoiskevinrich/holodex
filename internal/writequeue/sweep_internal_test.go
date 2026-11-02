package writequeue

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// R11 (§39.4): the orphan sweep removes only the writeback temps; a held original and a rename's
// via name both survive it.
func TestSweepOrphansKeepsHeldAndViaNames(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"a.mkv.holodex-tmp", "a.mkv.holodex-new", "a.mkv.holodex-orig", "hx-7.holodex-rename"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	q := &Queue{mediaRoot: root, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	q.sweepOrphans()
	for n, want := range map[string]bool{"a.mkv.holodex-tmp": false, "a.mkv.holodex-new": false,
		"a.mkv.holodex-orig": true, "hx-7.holodex-rename": true} {
		if _, err := os.Stat(filepath.Join(root, n)); (err == nil) != want {
			t.Errorf("%s present = %v, want %v", n, err == nil, want)
		}
	}
}
