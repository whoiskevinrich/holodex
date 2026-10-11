package maintenance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A planted symlink at either working name stops the job before any tool runs, and the link's
// target is untouched (security review, design check 3).
func TestRebuild_RefusesSymlinkedWorkingFiles(t *testing.T) {
	for _, suffix := range []string{".holodex-new", ".holodex-tmp"} {
		t.Run(suffix, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "a.mkv")
			target := filepath.Join(dir, "precious.db")
			if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, src+suffix); err != nil {
				t.Skipf("symlinks unavailable here: %v", err)
			}
			err := Rebuild(context.Background(), Tools{FFmpeg: "no-such-ffmpeg"}, FamilyMKV, OpHFlip, nil,
				src, src+".holodex-new", Probe{})
			if !errors.Is(err, ErrSymlinkedSibling) {
				t.Fatalf("err = %v, want ErrSymlinkedSibling", err)
			}
			if b, _ := os.ReadFile(target); string(b) != "keep" {
				t.Errorf("link target changed to %q", b)
			}
			if _, err := os.Lstat(src + suffix); err != nil {
				t.Errorf("the planted link itself was removed: %v", err)
			}
		})
	}
}
