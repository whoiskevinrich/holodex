package writeback

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FieldWrite is one tag assignment for WriteBatch.
type FieldWrite struct {
	TagName string   // format-specific tag name from TagForField
	Values  []string // one or more values; multi-valued tags get one entry per value
}

// WriteBatch embeds all tag values into the file at path in a single tool
// invocation (ADR-041 §file-safety). The write tool is chosen by extension:
//
//   - .mkv / .mka / .mks / .webm → mkvpropedit if available, else ffmpeg
//   - everything else             → exiftool
//
// On any failure the original is untouched; temp files are cleaned up.
// All FieldWrite entries must have a non-empty TagName and at least one value.
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

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mkv", ".mka", ".mks", ".webm":
		return writeMKVBatch(ctx, path, fields)
	default:
		return writeExiftoolBatch(ctx, path, fields)
	}
}

// Write embeds a single tag into the file at path. Delegates to WriteBatch.
func Write(ctx context.Context, path, tagName string, values []string) error {
	return WriteBatch(ctx, path, []FieldWrite{{TagName: tagName, Values: values}})
}

// writeMKVBatch dispatches to mkvpropedit (fast, in-place) when available,
// falling back to ffmpeg remux (already a required project dependency).
func writeMKVBatch(ctx context.Context, path string, fields []FieldWrite) error {
	if _, err := exec.LookPath("mkvpropedit"); err == nil {
		return writeMKVWithMkvpropedit(ctx, path, fields)
	}
	return writeMKVWithFFmpeg(ctx, path, fields)
}

// writeExiftoolBatch writes all fields in one exiftool invocation.
func writeExiftoolBatch(ctx context.Context, path string, fields []FieldWrite) error {
	tmp := path + ".holodex-tmp"
	if err := copyFile(path, tmp); err != nil {
		return fmt.Errorf("writeback copy: %w", err)
	}

	// One -TAG=VALUE per value for multi-valued fields (genres). -m suppresses
	// minor-error exits so exiftool writes to imperfect-but-valid user files.
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

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writeback rename: %w", err)
	}
	return nil
}

// writeMKVWithMkvpropedit uses mkvpropedit (MKVToolNix) for fast in-place
// Matroska tag writes. "Title" maps to the Segment Info title property;
// all other tags go into the global TAGS element via a temp XML file.
// Uses the same copy→write→rename safety model as the exiftool path.
func writeMKVWithMkvpropedit(ctx context.Context, path string, fields []FieldWrite) error {
	tmp := path + ".holodex-tmp"
	if err := copyFile(path, tmp); err != nil {
		return fmt.Errorf("writeback copy: %w", err)
	}

	xmlPath := tmp + ".tags.xml"
	var args []string
	var xmlTags []FieldWrite

	for _, f := range fields {
		if strings.EqualFold(f.TagName, "Title") {
			args = append(args, "--edit", "info", "--set", "title="+f.Values[0])
		} else {
			xmlTags = append(xmlTags, f)
		}
	}

	if len(xmlTags) > 0 {
		if err := os.WriteFile(xmlPath, []byte(buildTagsXML(xmlTags)), 0o600); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("writeback: write tags XML: %w", err)
		}
		args = append(args, "--tags", "global:"+xmlPath)
	}
	args = append(args, tmp)

	cmd := exec.CommandContext(ctx, "mkvpropedit", args...)
	out, err := cmd.CombinedOutput()
	_ = os.Remove(xmlPath)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writeback mkvpropedit: %w — %s", err, strings.TrimSpace(string(out)))
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writeback rename: %w", err)
	}
	return nil
}

// writeMKVWithFFmpeg remuxes the file with updated tags using ffmpeg (-c copy
// keeps all streams byte-for-byte; only the container header is rebuilt).
// ffmpeg is already required by the project (thumbnail pipeline), so this
// path adds no extra dependency. Multi-value fields are joined with " / ".
//
// ffmpeg reads from the original and writes to a temp path; rename is atomic.
func writeMKVWithFFmpeg(ctx context.Context, path string, fields []FieldWrite) error {
	newPath := path + ".holodex-new"

	// ffmpeg determines the output muxer from the file extension by default.
	// Our temp file ends in ".holodex-new" which is unrecognised, so we must
	// pass -f explicitly. webm and matroska are distinct muxers in ffmpeg.
	format := "matroska"
	if strings.ToLower(filepath.Ext(path)) == ".webm" {
		format = "webm"
	}

	// -y: overwrite output; -map_metadata 0: carry existing tags forward
	// (individual -metadata flags then add/override the specific keys).
	args := []string{"-y", "-i", path, "-c", "copy", "-map_metadata", "0", "-f", format}
	for _, f := range fields {
		key := ffmpegMetadataKey(f.TagName)
		// ffmpeg doesn't support duplicate keys for multi-value; join with " / "
		// which is the common separator in Matroska tooling.
		args = append(args, "-metadata", key+"="+strings.Join(f.Values, " / "))
	}
	args = append(args, newPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(newPath)
		if isNotFound(err) {
			return fmt.Errorf("writeback: neither mkvpropedit nor ffmpeg found — install MKVToolNix or ffmpeg")
		}
		return fmt.Errorf("writeback ffmpeg: %w — %s", err, strings.TrimSpace(string(out)))
	}

	if err := os.Rename(newPath, path); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("writeback rename: %w", err)
	}
	return nil
}

// ffmpegMetadataKey converts our Matroska/ExifTool tag name to the key ffmpeg
// expects for -metadata. Only "Title" is special: ffmpeg maps lowercase "title"
// to both segment info and the TITLE tag; other names pass through unchanged.
func ffmpegMetadataKey(tagName string) string {
	if strings.EqualFold(tagName, "Title") {
		return "title"
	}
	return tagName
}

// buildTagsXML renders a minimal Matroska Tags XML document for mkvpropedit.
// Multi-value fields (genres) produce one <Simple> element per value.
// Tag names are uppercased per Matroska convention.
func buildTagsXML(fields []FieldWrite) string {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\"?>\n")
	sb.WriteString("<!DOCTYPE Tags SYSTEM \"matroskatags.dtd\">\n")
	sb.WriteString("<Tags>\n<Tag>\n<Targets />\n")
	for _, f := range fields {
		name := strings.ToUpper(f.TagName)
		for _, v := range f.Values {
			sb.WriteString("<Simple><Name>")
			sb.WriteString(name)
			sb.WriteString("</Name><String>")
			sb.WriteString(xmlEscape(v))
			sb.WriteString("</String></Simple>\n")
		}
	}
	sb.WriteString("</Tag>\n</Tags>\n")
	return sb.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// isNotFound reports whether err came from exec failing to find the binary.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "executable file not found")
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
