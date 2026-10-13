package scanner

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// seededBytes returns n deterministic pseudo-random bytes (testing-strategy §26.2).
func seededBytes(seed int64, n int) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(seed)).Read(b) //nolint:gosec // test fixture, not crypto
	return b
}

func writeBytes(t *testing.T, dir, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustFP(t *testing.T, path string) string {
	t.Helper()
	fp, err := contentFingerprint(path)
	if err != nil {
		t.Fatalf("contentFingerprint(%s): %v", path, err)
	}
	if fp == "" {
		t.Fatalf("contentFingerprint(%s) returned empty", path)
	}
	return fp
}

func TestContentFingerprint_StableAndSurvivesRename(t *testing.T) {
	dir := t.TempDir()
	p := writeBytes(t, dir, "a.mp4", seededBytes(1, 3<<20))
	first := mustFP(t, p)
	if again := mustFP(t, p); again != first {
		t.Fatalf("second read differs: %s vs %s", again, first)
	}
	moved := filepath.Join(dir, "sub-b.mkv")
	if err := os.Rename(p, moved); err != nil {
		t.Fatal(err)
	}
	if got := mustFP(t, moved); got != first {
		t.Fatalf("renamed file differs: %s vs %s", got, first)
	}
}

// The near twins defeat weaker evidence: each pair has the same size, and a
// fingerprint that ignored the differing region would merge two videos (P0-1).
func TestContentFingerprint_NearTwinsDiffer(t *testing.T) {
	dir := t.TempDir()
	const size = 4 << 20
	base := seededBytes(2, size)
	mutate := func(off int) []byte {
		b := append([]byte(nil), base...)
		b[off] ^= 0xFF
		return b
	}
	cases := map[string][]byte{
		"different bytes": seededBytes(3, size),
		"middle only":     mutate(size/2 + 10),
		"start only":      mutate(10),
		"end only":        mutate(size - 10),
	}
	want := mustFP(t, writeBytes(t, dir, "base.mp4", base))
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			if got := mustFP(t, writeBytes(t, dir, name+".mp4", b)); got == want {
				t.Fatalf("near twin %q matched the base fingerprint", name)
			}
		})
	}
}

func TestContentFingerprint_SizeIsPartOfTheHash(t *testing.T) {
	dir := t.TempDir()
	// All-zero files read identical samples, so only the size can tell them apart.
	a := mustFP(t, writeBytes(t, dir, "a.mp4", make([]byte, 1<<20)))
	b := mustFP(t, writeBytes(t, dir, "b.mp4", make([]byte, 1<<20+4096)))
	if a == b {
		t.Fatal("files differing only in size share a fingerprint")
	}
}

func TestContentFingerprint_SmallFilesHashedWhole(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []int{0, 1, 100 << 10, fpSampleSize * 3} {
		b := seededBytes(4, n)
		fp := mustFP(t, writeBytes(t, dir, "s.mp4", b))
		if n == 0 {
			continue
		}
		// A one-byte change anywhere in a small file changes the hash.
		b[n-1] ^= 0xFF
		if got := mustFP(t, writeBytes(t, dir, "s.mp4", b)); got == fp {
			t.Fatalf("size %d: last-byte change not detected", n)
		}
	}
}

func TestContentFingerprint_MissingFileErrors(t *testing.T) {
	if _, err := contentFingerprint(filepath.Join(t.TempDir(), "nope.mp4")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
