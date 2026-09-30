//go:build integration

// Integration test for clearing a studio (ADR-120 D4, testing-strategy §22.4)
// against the real binaries. Run with `make test-image`, which runs it inside the
// built image (trixie exiftool / mkvpropedit / ffmpeg), or locally with
//
//	go test -tags integration ./internal/writeback/ -run StudioClear
//
// A mis-parsed studio can sit under any of the tags the field reads from, not
// only the Publisher tag it writes. A clear must remove it wherever it is, leave
// every other tag alone, and never write a placeholder.
package writeback

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var studioClearSources = []string{"Publisher", "Label", "Studio", "ProductionCompany"}

func requireStudioClearTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"ffmpeg", "exiftool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
}

// exifValue reads one tag with exiftool (-s3: the bare value), "" when absent.
func exifValue(t *testing.T, path, tag string) string {
	t.Helper()
	out, err := exec.Command("exiftool", "-s3", "-"+tag, path).Output()
	if err != nil {
		t.Fatalf("exiftool -%s: %v", tag, err)
	}
	return strings.TrimSpace(string(out))
}

// seedStudioClip makes a 1 s clip carrying Title "Keep Me" plus studio "Acme"
// under each of tags. An MP4 tag is group-qualified ("XMP:Label") and written
// with exiftool; an MKV tag is bare and written by ffmpeg. It reports false when
// the container can't hold one of them — that source can't be on such a file.
func seedStudioClip(t *testing.T, ext string, tags []string) (string, bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip."+ext)
	args := []string{"-y", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=5",
		"-metadata", "title=Keep Me"}
	if ext == "mkv" {
		for _, tag := range tags {
			args = append(args, "-metadata", strings.ToUpper(tag)+"=Acme")
		}
	}
	if out, err := exec.Command("ffmpeg", append(args, path)...).CombinedOutput(); err != nil {
		t.Skipf("could not synthesize clip: %v: %s", err, out)
	}
	if ext == "mp4" {
		exArgs := []string{"-m", "-overwrite_original"}
		for _, tag := range tags {
			exArgs = append(exArgs, "-"+tag+"=Acme")
		}
		if out, err := exec.Command("exiftool", append(exArgs, path)...).CombinedOutput(); err != nil {
			t.Logf("seed %v on mp4: %v: %s", tags, err, out)
			return path, false
		}
	}
	for _, tag := range tags {
		if exifValue(t, path, tag) != "Acme" {
			return path, false
		}
	}
	return path, true
}

// clearAndCheck runs the clear through write, then asserts no studio tag is
// left, the unrelated Title survived, and nothing wrote a placeholder.
func clearAndCheck(t *testing.T, path, container string, write func(context.Context, string, []FieldWrite) error) {
	t.Helper()
	names, rejected := ClearTagNames(container, "studio", studioClearSources)
	if len(rejected) != 0 {
		t.Fatalf("real studio sources rejected on %s: %v", container, rejected)
	}
	batch := make([]FieldWrite, len(names))
	for i, n := range names {
		batch[i] = FieldWrite{TagName: n, Delete: true}
	}
	if err := write(context.Background(), path, batch); err != nil {
		t.Fatalf("clear %v: %v", names, err)
	}
	for _, tag := range studioClearSources {
		if v := exifValue(t, path, tag); v != "" {
			t.Errorf("%s still on file after clear: %q", tag, v)
		}
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
}

// Each case plants the mis-parse somewhere the field reads from — the write
// target, a source it never writes, another group, or two at once — and clears
// it. The MP4 XMP cases are why deletes are bare: "-QuickTime:Label=" leaves an
// XMP:Label the scanner still reads as the studio.
func TestStudioClear_RealFiles(t *testing.T) {
	requireStudioClearTools(t)
	mkvSeeds := [][]string{{"Publisher"}, {"Label"}, {"Studio"}, {"Label", "ProductionCompany"}}
	mp4Seeds := [][]string{{"QuickTime:Publisher"}, {"XMP:Publisher"}, {"XMP:Label"}, {"QuickTime:Publisher", "XMP:Label"}}

	type backend struct {
		name, container, ext string
		write                func(context.Context, string, []FieldWrite) error
		needs                []string
	}
	backends := []backend{
		{"MP4/exiftool", "MP4", "mp4", WriteBatch, nil},
		{"Matroska/mkvpropedit", "Matroska", "mkv", writeMKVWithMkvpropedit, []string{"mkvpropedit", "mkvextract", "mkvmerge"}},
		{"Matroska/ffmpeg", "Matroska", "mkv", writeMKVWithFFmpeg, nil},
	}
	for _, b := range backends {
		t.Run(b.name, func(t *testing.T) {
			for _, tool := range b.needs {
				if _, err := exec.LookPath(tool); err != nil {
					t.Skipf("%s not on PATH", tool)
				}
			}
			seeds := mkvSeeds
			if b.ext == "mp4" {
				seeds = mp4Seeds
			}
			seeded := 0
			for _, tags := range seeds {
				t.Run(strings.Join(tags, "+"), func(t *testing.T) {
					path, ok := seedStudioClip(t, b.ext, tags)
					if !ok {
						t.Skipf("%s can't hold %v", b.container, tags)
					}
					seeded++
					clearAndCheck(t, path, b.container, b.write)
				})
			}
			if seeded == 0 {
				t.Fatalf("%s: no studio source could be seeded; the clear was never exercised", b.name)
			}
		})
	}
}
