package thumbnail

import (
	"os/exec"
	"strings"
	"testing"
)

// TestStillsNeverDependOnMontages pins F83 RD5: the hover-montage pipeline sits
// beside the still pipeline, and the dependency runs one way only. If this
// package (or anything it imports) ever imports internal/montage, a later change
// to stills would have to account for montages.
func TestStillsNeverDependOnMontages(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not on PATH")
	}
	out, err := exec.Command("go", "list", "-deps", "holodex/internal/thumbnail").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v: %s", err, out)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "holodex/internal/montage" {
			t.Fatal("internal/thumbnail depends on internal/montage (F83 RD5)")
		}
	}
}
