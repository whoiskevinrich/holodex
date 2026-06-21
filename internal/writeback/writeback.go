package writeback

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// FieldWrite is one tag assignment for WriteBatch.
type FieldWrite struct {
	TagName string   // exiftool tag name (e.g. "Title", "QuickTime:Genre")
	Values  []string // one or more values; multi-valued tags get one -TAG=V per value
}

// WriteBatch embeds all tag values into the file at path in a single exiftool
// invocation — one copy → one write → one atomic rename regardless of how many
// fields are written (ADR-041 §file-safety model).
//
// All FieldWrite entries must have a non-empty TagName and at least one value;
// the call returns an error before touching the file if any entry is invalid.
// On any exiftool failure the original is untouched.
//
// The caller is responsible for resolving canonical → TagName via TagForField
// and for inserting audit rows on success.
func WriteBatch(ctx context.Context, path string, fields []FieldWrite) error {
	if len(fields) == 0 {
		return fmt.Errorf("writeback: no fields to write")
	}
	for _, f := range fields {
		if f.TagName == "" {
			return fmt.Errorf("writeback: empty tag name in batch")
		}
		if len(f.Values) == 0 {
			return fmt.Errorf("writeback: no values for tag %q", f.TagName)
		}
	}

	tmp := path + ".holodex-tmp"

	// Step 1: copy original to a temp in the same directory (same FS partition
	// guarantees the later rename is atomic).
	if err := copyFile(path, tmp); err != nil {
		return fmt.Errorf("writeback copy: %w", err)
	}

	// Step 2: write all tags in one exiftool call. One -TAG=VALUE per value for
	// multi-valued fields (genres). -m suppresses minor-error exits so exiftool
	// writes to imperfect-but-valid user files without aborting.
	args := make([]string, 0, len(fields)*2+3)
	for _, f := range fields {
		for _, v := range f.Values {
			args = append(args, fmt.Sprintf("-%s=%s", f.TagName, v))
		}
	}
	args = append(args, "-m", "-overwrite_original", tmp)

	cmd := exec.CommandContext(ctx, "exiftool", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writeback exiftool: %w — %s", err, strings.TrimSpace(string(out)))
	}

	// Step 3: atomic rename — replaces the original only after a successful write.
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writeback rename: %w", err)
	}
	return nil
}

// Write embeds a single tag into the file at path. Delegates to WriteBatch so
// the atomicity and file-safety invariants are shared with the batch path.
func Write(ctx context.Context, path, tagName string, values []string) error {
	return WriteBatch(ctx, path, []FieldWrite{{TagName: tagName, Values: values}})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
