package writeback

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// FieldWrite is one tag assignment for WriteBatch.
type FieldWrite struct {
	TagName string   // format-specific tag name from TagForField / ImageTagForField
	Values  []string // one or more values; for IsImage fields Values[0] is a URL
	IsImage bool     // when true, Values[0] is an https:// URL to download+embed as cover art
}

// WriteBatch embeds all tag values into the file at path in a single tool
// invocation (ADR-041 §file-safety). The write tool is chosen by extension:
//
//   - .mkv / .mka / .mks / .webm → mkvpropedit if available, else ffmpeg
//   - everything else             → exiftool
//
// Every backend merges: a tag named in fields is replaced with the incoming
// values, and every other tag, attachment, and stream on the file is preserved.
// How that is achieved differs per tool — exiftool by construction (-TAG=VALUE
// touches only named tags), ffmpeg via -map 0 -map_metadata 0, mkvpropedit by
// reading the existing tags back and splicing them (mergeTagsXML) because
// --tags global: replaces the whole element. A new or edited backend must
// uphold this contract; both tools that default to wholesale replacement have
// silently destroyed metadata here before.
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
//
// The mkvpropedit path needs mkvextract too: mkvpropedit replaces the whole
// global TAGS element rather than merging into it, so the existing tags have to
// be read back and folded in. Without mkvextract we cannot do that merge, and
// ffmpeg (which carries tags forward via -map_metadata 0) is the safe choice.
func writeMKVBatch(ctx context.Context, path string, fields []FieldWrite) error {
	_, propErr := exec.LookPath("mkvpropedit")
	_, extrErr := exec.LookPath("mkvextract")
	if propErr == nil && extrErr == nil {
		return writeMKVWithMkvpropedit(ctx, path, fields)
	}
	return writeMKVWithFFmpeg(ctx, path, fields)
}

// writeExiftoolBatch writes all fields in one exiftool invocation. Text fields
// use -TAG=VALUE; image fields use -TAG<=file (binary read from a temp download).
func writeExiftoolBatch(ctx context.Context, path string, fields []FieldWrite) error {
	tmp := path + ".holodex-tmp"
	if err := copyFile(path, tmp); err != nil {
		return fmt.Errorf("writeback copy: %w", err)
	}

	// One -TAG=VALUE per value for multi-valued fields (genres). -m suppresses
	// minor-error exits so exiftool writes to imperfect-but-valid user files.
	args := make([]string, 0, len(fields)*2+3)
	for _, f := range fields {
		if f.IsImage {
			imgPath, cleanup, err := downloadImageToTemp(ctx, f.Values[0])
			if err != nil {
				_ = os.Remove(tmp)
				return fmt.Errorf("writeback: %w", err)
			}
			defer cleanup()
			// exiftool binary-write syntax: -TAG<=filepath reads the file content
			args = append(args, fmt.Sprintf("-%s<=%s", f.TagName, imgPath))
		} else {
			for _, v := range f.Values {
				args = append(args, fmt.Sprintf("-%s=%s", f.TagName, v))
			}
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
// Image fields are attached as cover art via separate mkvpropedit invocations.
// Uses the same copy→write→rename safety model as the exiftool path.
func writeMKVWithMkvpropedit(ctx context.Context, path string, fields []FieldWrite) error {
	var args []string
	var xmlTags []FieldWrite
	var imgFields []FieldWrite

	for _, f := range fields {
		if f.IsImage {
			imgFields = append(imgFields, f)
		} else if strings.EqualFold(f.TagName, "Title") {
			args = append(args, "--edit", "info", "--set", "title="+f.Values[0])
		} else {
			xmlTags = append(xmlTags, f)
		}
	}

	// --tags global: replaces the entire TAGS element, so read what the file
	// already carries and merge our fields into it. Both steps only touch the
	// original, so they run before the copy — a failure here then costs one cheap
	// subprocess rather than a discarded full-file copy.
	var mergedTags string
	if len(xmlTags) > 0 {
		existing, err := existingTagsXML(ctx, path)
		if err != nil {
			return fmt.Errorf("writeback: %w", err)
		}
		if mergedTags, err = mergeTagsXML(existing, xmlTags); err != nil {
			return fmt.Errorf("writeback: %w", err)
		}
	}

	tmp := path + ".holodex-tmp"
	if err := copyFile(path, tmp); err != nil {
		return fmt.Errorf("writeback copy: %w", err)
	}

	xmlPath := tmp + ".tags.xml"
	if len(xmlTags) > 0 || len(args) > 0 {
		if len(xmlTags) > 0 {
			if err := os.WriteFile(xmlPath, []byte(mergedTags), 0o600); err != nil {
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
	}

	// Handle cover art attachments. Each image field downloads its URL to a temp
	// file, then mkvpropedit replaces (or adds) the named attachment on the temp copy.
	for _, f := range imgFields {
		imgPath, cleanup, err := downloadImageToTemp(ctx, f.Values[0])
		if err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("writeback: %w", err)
		}
		defer cleanup()

		// Remove any existing attachment with this name (ignore exit code — it
		// may not exist; mkvpropedit exits 2 for warnings).
		exec.CommandContext(ctx, "mkvpropedit", tmp, "--delete-attachment", "name:"+f.TagName).Run() //nolint:errcheck

		// Add the new attachment.
		addOut, addErr := exec.CommandContext(ctx, "mkvpropedit", tmp,
			"--attachment-name", f.TagName,
			"--attachment-mime-type", "image/jpeg",
			"--add-attachment", imgPath,
		).CombinedOutput()
		if addErr != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("writeback mkvpropedit cover art: %w — %s", addErr, strings.TrimSpace(string(addOut)))
		}
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writeback rename: %w", err)
	}
	return nil
}

// ffmpegImgEntry is a downloaded image field ready to attach via ffmpeg.
type ffmpegImgEntry struct{ tagName, localPath string }

// buildFFmpegArgs builds the ffmpeg argument list for a writeback remux. Pure
// (no I/O) so the stream-preservation and metadata-merge behavior can be unit
// tested without shelling out to ffmpeg.
//
// -map 0 is unconditional: it carries forward every existing stream (video,
// audio, subtitles, and attachments such as an embedded cover art image).
// Without it, ffmpeg's automatic stream selection drops attachment streams
// entirely — a writeback that only touched text fields would silently erase
// any existing embedded poster.
func buildFFmpegArgs(path, newPath, format string, fields []FieldWrite, imgEntries []ffmpegImgEntry) []string {
	// -y: overwrite output; -map 0: keep every stream; -map_metadata 0: carry
	// existing container tags forward (unlisted -metadata keys are untouched).
	args := []string{"-y", "-i", path, "-map", "0", "-c", "copy", "-map_metadata", "0", "-f", format}

	for _, f := range fields {
		if f.IsImage {
			continue
		}
		key := ffmpegMetadataKey(f.TagName)
		args = append(args, "-metadata", key+"="+strings.Join(f.Values, ", "))
	}
	for i, ie := range imgEntries {
		args = append(args,
			"-attach", ie.localPath,
			fmt.Sprintf("-metadata:s:t:%d", i), "mimetype=image/jpeg",
			fmt.Sprintf("-metadata:s:t:%d", i), "filename="+ie.tagName,
		)
	}
	args = append(args, newPath)
	return args
}

// writeMKVWithFFmpeg remuxes the file with updated tags using ffmpeg (-c copy
// keeps all streams byte-for-byte; only the container header is rebuilt).
// ffmpeg is already required by the project (thumbnail pipeline), so this
// path adds no extra dependency. Multi-value fields are joined with ", ".
// Image fields are attached using ffmpeg's -attach option.
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

	// Separate image fields from text fields and download images upfront.
	var imgEntries []ffmpegImgEntry
	for _, f := range fields {
		if !f.IsImage {
			continue
		}
		imgPath, cleanup, err := downloadImageToTemp(ctx, f.Values[0])
		if err != nil {
			return fmt.Errorf("writeback: %w", err)
		}
		defer cleanup()
		imgEntries = append(imgEntries, ffmpegImgEntry{f.TagName, imgPath})
	}

	args := buildFFmpegArgs(path, newPath, format, fields, imgEntries)

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

// ffmpegMetadataKey converts our tag name to the key ffmpeg expects for
// -metadata. ffmpeg's built-in Matroska codec mapping uses lowercase standard
// keys; passing them capitalised causes them to be stored as custom tags with
// the wrong name instead of the expected COMMENT/ARTIST/… element.
// "Year" is intentionally left as-is so it lands in a YEAR custom tag rather
// than being remapped to DATE_RELEASED via ffmpeg's "date" alias.
func ffmpegMetadataKey(tagName string) string {
	switch strings.ToLower(tagName) {
	case "title", "comment", "artist", "genre", "publisher", "subtitle":
		return strings.ToLower(tagName)
	}
	return tagName
}

// mkvTagsDoc mirrors the Matroska tags XML that mkvextract emits and
// mkvpropedit consumes. Each <Simple> keeps its raw inner XML so elements we do
// not model (TagLanguage, Binary, nested Simple) survive the round trip.
type mkvTagsDoc struct {
	XMLName xml.Name `xml:"Tags"`
	Tags    []mkvTag `xml:"Tag"`
}

type mkvTag struct {
	Targets mkvRawEl    `xml:"Targets"`
	Simples []mkvSimple `xml:"Simple"`
}

type mkvRawEl struct {
	Inner string `xml:",innerxml"`
}

type mkvSimple struct {
	Name  string `xml:"Name"`
	Inner string `xml:",innerxml"`
}

// existingTagsXML returns the file's current Matroska tags document, or "" when
// it carries none.
func existingTagsXML(ctx context.Context, path string) (string, error) {
	out, err := exec.CommandContext(ctx, "mkvextract", path, "tags").Output()
	if err != nil {
		// mkvextract exits 1 for warnings (a file with no tags at all can land
		// here) but 2+ for real errors. Only a real error is fatal — treating one
		// as "no tags" would write a document that erases what we failed to read.
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() >= 2 {
			return "", fmt.Errorf("read existing tags: %w", err)
		}
	}
	return strings.TrimSpace(string(out)), nil
}

// mergeTagsXML folds fields into an existing Matroska tags document, rendering
// the result for mkvpropedit's --tags global:. That option REPLACES the whole
// TAGS element rather than merging into it, so passing only the current batch
// would erase every tag an earlier batch had written. Simple elements whose
// Name matches an incoming field are dropped in favour of the new values;
// everything else is carried through verbatim. Multi-value fields (genres)
// produce one <Simple> per value, and names are uppercased per Matroska
// convention. An empty existing document yields a fresh single-Tag document.
func mergeTagsXML(existing string, fields []FieldWrite) (string, error) {
	var doc mkvTagsDoc
	if existing != "" {
		if err := xml.Unmarshal([]byte(existing), &doc); err != nil {
			return "", fmt.Errorf("parse existing tags: %w", err)
		}
	}

	replaced := make(map[string]bool, len(fields))
	for _, f := range fields {
		replaced[strings.ToUpper(strings.TrimSpace(f.TagName))] = true
	}

	// The incoming fields belong on an untargeted (whole-file) Tag; make sure the
	// document has one for them to land on, so the render loop below is the only
	// thing that emits a Tag.
	untargeted := func(t mkvTag) bool { return strings.TrimSpace(t.Targets.Inner) == "" }
	if !slices.ContainsFunc(doc.Tags, untargeted) {
		doc.Tags = append(doc.Tags, mkvTag{})
	}

	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\"?>\n")
	sb.WriteString("<!DOCTYPE Tags SYSTEM \"matroskatags.dtd\">\n")
	sb.WriteString("<Tags>\n")

	added := false
	for _, tag := range doc.Tags {
		kept := make([]mkvSimple, 0, len(tag.Simples))
		for _, s := range tag.Simples {
			if !replaced[strings.ToUpper(strings.TrimSpace(s.Name))] {
				kept = append(kept, s)
			}
		}
		// The first untargeted Tag takes our fields.
		addHere := !added && untargeted(tag)
		if len(kept) == 0 && !addHere {
			// Matroska requires every Tag to carry at least one Simple.
			continue
		}

		sb.WriteString("<Tag>\n")
		if untargeted(tag) {
			sb.WriteString("<Targets />\n")
		} else {
			sb.WriteString("<Targets>" + tag.Targets.Inner + "</Targets>\n")
		}
		for _, s := range kept {
			sb.WriteString("<Simple>" + s.Inner + "</Simple>\n")
		}
		if addHere {
			writeSimples(&sb, fields)
			added = true
		}
		sb.WriteString("</Tag>\n")
	}

	sb.WriteString("</Tags>\n")
	return sb.String(), nil
}

// writeSimples renders one <Simple> element per value.
func writeSimples(sb *strings.Builder, fields []FieldWrite) {
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

// ImageFetcher downloads an image URL under an SSRF-guarded transport (host
// allowlist, cross-host-redirect refusal, size/time caps — ADR-039) and returns
// the raw bytes. Satisfied by enrich.Service.FetchAllowedImage in production.
type ImageFetcher func(ctx context.Context, rawURL string) ([]byte, error)

// imageFetch is the guarded downloader downloadImageToTemp uses. Wired once at
// startup via SetImageFetcher; left nil, every image-field write is refused
// rather than falling back to an unguarded fetch (HOLODEX-212 — fail closed,
// not open).
var imageFetch ImageFetcher

// SetImageFetcher wires the SSRF-guarded image download used by an IsImage
// FieldWrite (HOLODEX-212, ADR-039). Mirrors the SetImageSink/SetWriteback
// startup-wiring idiom. Must be called before any writeback with an image field
// runs — production wires enrich.Service.FetchAllowedImage.
func SetImageFetcher(fn ImageFetcher) { imageFetch = fn }

// downloadImageToTemp downloads an https:// image URL to a temp file through
// the guarded imageFetch and returns the path plus a cleanup function. Only
// https is accepted; the caller must call cleanup() when done. The fetch itself
// is size/host capped by imageFetch (ADR-039) — a nil imageFetch or a host
// outside every enabled provider's allowlist refuses rather than downloading.
func downloadImageToTemp(ctx context.Context, rawURL string) (path string, cleanup func(), err error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return "", nil, fmt.Errorf("cover image: only https URLs are supported")
	}
	if imageFetch == nil {
		return "", nil, fmt.Errorf("cover image: no allowlisted image fetcher configured")
	}
	data, err := imageFetch(ctx, rawURL)
	if err != nil {
		return "", nil, fmt.Errorf("cover image: %w", err)
	}

	ext := ".jpg"
	if ct := http.DetectContentType(data); strings.Contains(ct, "png") {
		ext = ".png"
	}
	tmp, err := os.CreateTemp("", "holodex-cover-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("cover image: create temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("cover image: write temp: %w", err)
	}
	tmp.Close()
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
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
