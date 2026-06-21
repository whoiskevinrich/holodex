package writeback

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Write embeds tag values into the file at path using exiftool, via an atomic
// copy → write → rename sequence so the original is never touched on failure
// (ADR-041 §file-safety model).
//
// tagName is an exiftool tag target (e.g. "QuickTime:Title", "GENRE") from the
// format-mapping table; values is the ordered list of values to write (one for
// scalar fields, many for multi-valued ones like genres). An empty tagName or
// empty values slice is an error.
//
// The caller is responsible for resolving canonical → tagName via TagForField
// and for inserting the audit row on success.
func Write(ctx context.Context, path, tagName string, values []string) error {
	if tagName == "" {
		return fmt.Errorf("writeback: empty tag name")
	}
	if len(values) == 0 {
		return fmt.Errorf("writeback: no values for tag %q", tagName)
	}

	tmp := path + ".holodex-tmp"

	// Step 1: copy the original to a temp file in the same directory (same FS
	// partition guarantees the later rename is atomic).
	if err := copyFile(path, tmp); err != nil {
		return fmt.Errorf("writeback copy: %w", err)
	}

	// Step 2: write the tag on the temp copy. Build discrete exiftool args —
	// one -TAG=VALUE per value for multi-valued fields (e.g. genres).
	// -m suppresses minor-error exits (e.g. incomplete file structure) so
	// exiftool writes to imperfect-but-valid user files; the copy→rename ensures
	// the original is never touched on any hard failure.
	args := make([]string, 0, len(values)+3)
	for _, v := range values {
		args = append(args, fmt.Sprintf("-%s=%s", tagName, v))
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
