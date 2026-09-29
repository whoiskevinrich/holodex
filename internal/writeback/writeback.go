package writeback

import (
	"context"
	"encoding/json"
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
	Delete  bool     // when true, Values is empty and the tag is removed from the file (ADR-110)
}

// fileValue is the single string a text field is stored as on file. Container
// genre/artist tags are one comma-delimited value, not a list — the reader
// splits it back apart (metadata.splitMulti) — and every backend must write that
// same shape: exiftool keeps only the LAST of repeated -TAG=VALUE assignments to
// a non-list tag, which silently cut MP4 genres to one (HOLODEX-464).
//
// A Delete field yields "", which is itself the delete on both text backends:
// exiftool's -TAG= and ffmpeg's -metadata key= remove the tag (ADR-110).
func fileValue(f FieldWrite) string {
	return strings.Join(f.Values, ", ")
}

// WriteBatch embeds all tag values into the file at path in a single tool
// invocation (ADR-041 §file-safety). The write tool is chosen by extension:
//
//   - .mkv / .mka / .mks / .webm → mkvpropedit if available, else ffmpeg
//   - everything else             → exiftool (a fragmented .mp4/.m4v/.mov is
//     remuxed into the temp copy first, since exiftool can't write it; when
//     that's impossible the write fails with ErrFragmentedMP4 — ADR-116)
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
// All FieldWrite entries must have a non-empty TagName and either at least one
// value or Delete set.
func WriteBatch(ctx context.Context, path string, fields []FieldWrite) error {
	if len(fields) == 0 {
		return fmt.Errorf("writeback: no fields to write")
	}
	for _, f := range fields {
		if f.TagName == "" {
			return fmt.Errorf("writeback: empty tag name in batch")
		}
		if f.Delete != (len(f.Values) == 0) {
			return fmt.Errorf("writeback: tag %q must carry values or be a delete, not both or neither", f.TagName)
		}
		if f.Delete && f.IsImage {
			return fmt.Errorf("writeback: cannot delete image tag %q", f.TagName)
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
// mkvmerge lists the attachments a cover write replaces; all three ship together.
func writeMKVBatch(ctx context.Context, path string, fields []FieldWrite) error {
	_, propErr := exec.LookPath("mkvpropedit")
	_, extrErr := exec.LookPath("mkvextract")
	_, mergeErr := exec.LookPath("mkvmerge")
	if propErr == nil && extrErr == nil && mergeErr == nil {
		return writeMKVWithMkvpropedit(ctx, path, fields)
	}
	return writeMKVWithFFmpeg(ctx, path, fields)
}

// writeExiftoolBatch writes all fields in one exiftool invocation. Text fields
// use -TAG=VALUE; image fields use -TAG<=file (binary read from a temp download).
func writeExiftoolBatch(ctx context.Context, path string, fields []FieldWrite) error {
	tmp := path + ".holodex-tmp"
	remuxed, err := stageTemp(ctx, path, tmp)
	if err != nil {
		return err
	}

	// -m suppresses minor-error exits so exiftool writes to imperfect-but-valid
	// user files.
	args := make([]string, 0, len(fields)*2+6)
	if remuxed {
		// ADR-116 D2: the remux drops XMP (where edition lives), so restore every
		// tag from the original first; the batch's assignments below, processed
		// after it, override. Still one exiftool invocation per batch.
		args = append(args, "-TagsFromFile", path, "-all:all")
	}
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
			args = append(args, fmt.Sprintf("-%s=%s", f.TagName, fileValue(f)))
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
		} else if strings.EqualFold(f.TagName, "Title") && f.Delete {
			// An ADR-110 delete carries no Values (ADR-117 D5).
			args = append(args, "--edit", "info", "--delete", "title")
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

		// Replace every cover in the same role (cover.webp too, not just an exact
		// cover.jpg — HOLODEX-485), then add the new one, in one invocation.
		existing, err := listMKVAttachments(ctx, tmp)
		if err != nil {
			_ = os.Remove(tmp)
			return err
		}
		coverArgs := append([]string{tmp}, coverDeleteArgs(existing, f.TagName)...)
		coverArgs = append(coverArgs,
			"--attachment-name", f.TagName,
			"--attachment-mime-type", coverMIME(imgPath),
			"--add-attachment", imgPath,
		)
		addOut, addErr := exec.CommandContext(ctx, "mkvpropedit", coverArgs...).CombinedOutput()
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

// mkvAttachment is one Matroska attachment as mkvmerge -J reports it.
type mkvAttachment struct {
	uid      uint64
	fileName string
}

// listMKVAttachments reads the file's attachments with mkvmerge (shipped with
// mkvpropedit in every MKVToolNix package) so a cover write can find the
// same-role covers to replace.
func listMKVAttachments(ctx context.Context, path string) ([]mkvAttachment, error) {
	// Exit 1 is warnings only — the identification JSON is still complete, and
	// refusing it would fail a cover write the old blind delete let through.
	out, err := exec.CommandContext(ctx, "mkvmerge", "-J", path).Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		err = nil
	}
	if err != nil {
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("writeback mkvmerge: %w — %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("writeback mkvmerge: %w", err)
	}
	var doc struct {
		Attachments []struct {
			FileName   string `json:"file_name"`
			Properties struct {
				UID uint64 `json:"uid"`
			} `json:"properties"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("writeback mkvmerge: %w", err)
	}
	atts := make([]mkvAttachment, len(doc.Attachments))
	for i, a := range doc.Attachments {
		atts[i] = mkvAttachment{a.Properties.UID, a.FileName}
	}
	return atts, nil
}

// coverDeleteArgs returns mkvpropedit --delete-attachment actions for every
// attachment in tagName's cover role. Selecting by UID (=N) is exact — no name
// parsing, and no position shift as earlier deletes in the same run land. An
// mkvmerge that reports no UID falls back to the exact name.
func coverDeleteArgs(atts []mkvAttachment, tagName string) []string {
	var args []string
	for _, a := range atts {
		if !sameCoverRole(a.fileName, tagName) {
			continue
		}
		sel := fmt.Sprintf("=%d", a.uid)
		if a.uid == 0 {
			sel = "name:" + a.fileName
		}
		args = append(args, "--delete-attachment", sel)
	}
	return args
}

// ffmpegImgEntry is a downloaded image field ready to attach via ffmpeg.
type ffmpegImgEntry struct{ tagName, localPath, mime string }

// buildFFmpegArgs builds the ffmpeg argument list for a writeback remux. Pure
// (no I/O) so the stream-preservation and metadata-merge behavior can be unit
// tested without shelling out to ffmpeg.
//
// -map 0 is unconditional: it carries forward every existing stream (video,
// audio, subtitles, and attachments such as an embedded cover art image).
// Without it, ffmpeg's automatic stream selection drops attachment streams
// entirely — a writeback that only touched text fields would silently erase
// any existing embedded poster.
//
// When the batch writes a cover, every existing stream in the same cover role
// is excluded (-map -0:m:filename:NAME) so the new one replaces it rather than
// stacking beside it. The role is the filename stem, case-insensitively: writing
// cover.jpg drops cover.webp and COVER.PNG too (players pick a cover by that
// naming convention, so two would be ambiguous), while fonts and other roles
// (small_cover.*) survive (HOLODEX-484).
// This also matters for recovery: ffmpeg exposes image attachments as
// attached-pic video streams and refuses to copy one it cannot decode
// ("dimensions not set"), so an undecodable cover blocks every writeback on
// that file until a cover writeback drops it.
//
// existing is the input's streams as probeStreams reports them. -metadata:s:t:N
// addresses OUTPUT attachment streams, and the ones copied by -map 0 come first,
// so the i-th new attachment sits at (surviving copied attachments + i).
func buildFFmpegArgs(path, newPath, format string, fields []FieldWrite, imgEntries []ffmpegImgEntry, existing []probedStream) []string {
	// -y: overwrite output; -map 0: keep every stream; -map_metadata 0: carry
	// existing container tags forward (unlisted -metadata keys are untouched).
	args := []string{"-y", "-i", path, "-map", "0"}
	// One classification drives both the drop and the count, so they cannot
	// disagree: a stream is either excluded by name or counted if it is a t:.
	dropped := map[string]bool{}
	keptAttachments := 0
	for _, s := range existing {
		replaced := ffmpegSpecifierSafe(s.filename) && slices.ContainsFunc(imgEntries, func(ie ffmpegImgEntry) bool {
			return sameCoverRole(s.filename, ie.tagName)
		})
		switch {
		case replaced && !dropped[s.filename]:
			dropped[s.filename] = true
			args = append(args, "-map", "-0:m:filename:"+s.filename)
		case !replaced && s.codecType == "attachment":
			keptAttachments++
		}
	}
	args = append(args, "-c", "copy", "-map_metadata", "0", "-f", format)

	for _, f := range fields {
		if f.IsImage {
			continue
		}
		key := ffmpegMetadataKey(f.TagName)
		args = append(args, "-metadata", key+"="+fileValue(f))
	}
	for i, ie := range imgEntries {
		spec := fmt.Sprintf("-metadata:s:t:%d", keptAttachments+i)
		args = append(args,
			"-attach", ie.localPath,
			spec, "mimetype="+ie.mime,
			spec, "filename="+ie.tagName,
		)
	}
	args = append(args, newPath)
	return args
}

// sameCoverRole reports whether an existing attachment filename fills the same
// cover role as tagName: equal stems, case-insensitively (cover.webp vs
// cover.jpg). An empty filename never matches.
func sameCoverRole(filename, tagName string) bool {
	stem := func(n string) string { return strings.TrimSuffix(n, filepath.Ext(n)) }
	return filename != "" && strings.EqualFold(stem(filename), stem(tagName))
}

// ffmpegSpecifierSafe reports whether ffmpeg's stream-specifier tokenizer takes
// filename verbatim in -map -0:m:filename:NAME. One with ':', '\' or a quote
// could silently miss, leaving the new cover's labels on the wrong stream again
// — better to keep that old cover than to break the remux.
func ffmpegSpecifierSafe(filename string) bool {
	return !strings.ContainsAny(filename, `:\'"`)
}

// probedStream is one input stream as ffprobe reports it: its type (an
// "attachment", or "video" for an image attachment ffmpeg exposes as an
// attached pic) and its Matroska attachment filename, if any.
type probedStream struct{ codecType, filename string }

// probeStreams lists the file's streams so buildFFmpegArgs can find the covers
// to replace and index the new attachment past the ones it keeps.
func probeStreams(ctx context.Context, path string) ([]probedStream, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-show_entries", "stream=codec_type:stream_tags=filename", "-of", "json", path).Output()
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("writeback: ffprobe not found — install MKVToolNix or ffmpeg")
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("writeback ffprobe: %w — %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("writeback ffprobe: %w", err)
	}
	var doc struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Tags      struct {
				Filename string `json:"filename"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("writeback ffprobe: %w", err)
	}
	streams := make([]probedStream, len(doc.Streams))
	for i, s := range doc.Streams {
		streams[i] = probedStream{s.CodecType, s.Tags.Filename}
	}
	return streams, nil
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
		imgEntries = append(imgEntries, ffmpegImgEntry{f.TagName, imgPath, coverMIME(imgPath)})
	}

	// Only a cover write needs the input's streams: a text-only remux keeps
	// every attachment and adds none.
	var existing []probedStream
	if len(imgEntries) > 0 {
		var err error
		if existing, err = probeStreams(ctx, path); err != nil {
			return err
		}
	}

	args := buildFFmpegArgs(path, newPath, format, fields, imgEntries, existing)

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
// everything else is carried through verbatim. Each field is one <Simple>
// holding fileValue (multi-value genres comma-joined, as the ffmpeg path writes
// them), and names are uppercased per Matroska convention. An empty existing document yields a fresh single-Tag document.
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
		if addHere {
			added = true
		}
		if len(kept) == 0 && !(addHere && hasWrites(fields)) {
			// Matroska requires every Tag to carry at least one Simple — and a
			// batch of only deletes (ADR-110) adds none.
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
		}
		sb.WriteString("</Tag>\n")
	}

	sb.WriteString("</Tags>\n")
	return sb.String(), nil
}

// hasWrites reports whether any field adds a value rather than deleting its tag.
func hasWrites(fields []FieldWrite) bool {
	return slices.ContainsFunc(fields, func(f FieldWrite) bool { return !f.Delete })
}

// writeSimples renders one <Simple> element per field. A Delete field renders
// nothing: mergeTagsXML has already dropped its existing Simples.
func writeSimples(sb *strings.Builder, fields []FieldWrite) {
	for _, f := range fields {
		if f.Delete {
			continue
		}
		sb.WriteString("<Simple><Name>")
		sb.WriteString(xmlEscape(strings.ToUpper(f.TagName)))
		sb.WriteString("</Name><String>")
		sb.WriteString(xmlEscape(fileValue(f)))
		sb.WriteString("</String></Simple>\n")
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

// coverMIME returns the attachment mimetype for a temp cover written by
// downloadImageToTemp, whose extension is the sniffed content type. Matroska
// readers trust this label — ffmpeg decodes an image attachment with the codec
// the mimetype names, so a PNG labelled image/jpeg becomes an undecodable
// attached-pic stream that later blocks remuxing the file.
func coverMIME(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return "image/png"
	}
	return "image/jpeg"
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
