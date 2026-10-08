//go:build integration

// Integration test for clearing an edition (F76, HOLODEX-521) against the real
// binaries — the proof edition needs before it joins the clearable allowlist. Run
// with `make test-image`, or locally with
//
//	go test -tags integration ./internal/writeback/ -run EditionClear
//
// Edition writes Matroska/WebM EDITION and MP4 XMP-prism:Edition, and is read back
// by bare name, so a bare "Edition" delete must remove it from either place and
// leave every other tag alone.
package writeback

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

var editionClearSources = []string{"Edition"}

func TestEditionClear_RealFiles(t *testing.T) {
	requireStudioClearTools(t)
	backends := []struct {
		name, container, ext, seedTag string
		write                         func(context.Context, string, []FieldWrite) error
		needs                         []string
	}{
		{"MP4/exiftool", "MP4", "mp4", "XMP-prism:Edition", WriteBatch, nil},
		{"Matroska/mkvpropedit", "Matroska", "mkv", "Edition", writeMKVWithMkvpropedit, []string{"mkvpropedit", "mkvextract", "mkvmerge"}},
		{"Matroska/ffmpeg", "Matroska", "mkv", "Edition", writeMKVWithFFmpeg, nil},
	}
	for _, b := range backends {
		t.Run(b.name, func(t *testing.T) {
			for _, tool := range b.needs {
				if _, err := exec.LookPath(tool); err != nil {
					t.Skipf("%s not on PATH", tool)
				}
			}
			path, ok := seedStudioClip(t, b.ext, []string{b.seedTag})
			if !ok {
				t.Fatalf("%s: could not seed %s", b.name, b.seedTag)
			}
			names, rejected := ClearTagNames(b.container, "edition", editionClearSources)
			if len(rejected) != 0 || len(names) == 0 {
				t.Fatalf("clear names on %s = %v, rejected %v", b.container, names, rejected)
			}
			batch := make([]FieldWrite, len(names))
			for i, n := range names {
				batch[i] = FieldWrite{TagName: n, Delete: true}
			}
			if err := b.write(context.Background(), path, batch); err != nil {
				t.Fatalf("clear %v: %v", names, err)
			}
			if v := exifValue(t, path, "Edition"); v != "" {
				t.Errorf("Edition still on file after clear: %q", v)
			}
			if v := exifValue(t, path, "Title"); v != "Keep Me" {
				t.Errorf("Title = %q after clear, want it untouched", v)
			}
			out, _ := exec.Command("exiftool", "-a", "-G1", "-s", path).Output()
			for _, bad := range []string{": none", ": —", `: ""`} {
				if strings.Contains(string(out), bad) {
					t.Errorf("a placeholder %q reached the file:\n%s", bad, out)
				}
			}
		})
	}
}
