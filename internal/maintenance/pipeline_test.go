package maintenance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A tool that fails mid-rebuild leaves no candidate behind and names the step, for each family.
func TestRebuild_ToolFailureLeavesNoCandidate(t *testing.T) {
	p := Probe{Streams: []Stream{{Index: 0, CodecType: "video", CodecName: "h264"}}}
	broken := Tools{FFmpeg: "holodex-no-such-ffmpeg", FFprobe: "ffprobe", Exiftool: "exiftool", Mkvmerge: "holodex-no-such-mkvmerge"}
	for _, family := range []string{FamilyMP4, FamilyMKV} {
		dir := t.TempDir()
		src := filepath.Join(dir, "a."+family)
		write(t, src, "original")
		dst := NewPath(src)
		write(t, dst, "stale candidate")
		err := Rebuild(context.Background(), broken, family, OpHFlip, allEncoders, src, dst, p)
		if err == nil {
			t.Fatalf("%s: rebuild with a missing tool succeeded", family)
		}
		if read(dst) != "<missing>" || read(src+".holodex-tmp") != "<missing>" {
			t.Errorf("%s: left dst=%q tmp=%q", family, read(dst), read(src+".holodex-tmp"))
		}
		if read(src) != "original" {
			t.Errorf("%s: original changed", family)
		}
	}
	if err := Rebuild(context.Background(), broken, "avi", OpHFlip, allEncoders, "x.avi", "x.avi.holodex-new", p); err == nil {
		t.Error("an unknown family rebuilt")
	}
}

func TestCollectSnapshot_ToolFailureIsAnError(t *testing.T) {
	p := Probe{Streams: []Stream{{Index: 0, CodecType: "video"}}}
	_, err := CollectSnapshot(context.Background(), Tools{FFmpeg: "holodex-no-such-ffmpeg"}, FamilyMP4, "x.mp4", p)
	if err == nil || !strings.Contains(err.Error(), "stream digests") {
		t.Errorf("err = %v", err)
	}
}

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
