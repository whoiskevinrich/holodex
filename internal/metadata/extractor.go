// Package metadata extracts a file's embedded tags (the source of truth, ADR-004)
// using exiftool for container tags and ffprobe for authoritative stream
// dimensions and duration. Subprocess invocation (Extractor) is kept separate
// from the pure mapping functions so the mapping is unit-testable with fixture
// JSON and needs no external binaries.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"holodex/internal/model"
)

// Extracted is the normalized metadata for one media file.
type Extracted struct {
	Title       string
	People      []string
	Tags        []string
	DurationSec int
	Width       int
	Height      int
	VideoCodec  string
	AudioCodec  string
	BitrateKbps int
	Container   string
	RecordedAt  *time.Time
	Extra       []model.ExtraMetadata
	// HasCoverArt reports that the container carries an embedded cover image, so
	// the thumbnail pipeline can extract it (Tier 1) rather than generating a
	// frame (ADR-009). The blob itself is not captured here.
	HasCoverArt bool
}

// Extractor runs exiftool + ffprobe. Binary paths default to the names on PATH.
type Extractor struct {
	ExiftoolPath string
	FfprobePath  string
}

// NewExtractor returns an extractor using `exiftool` and `ffprobe` from PATH.
func NewExtractor() *Extractor {
	return &Extractor{ExiftoolPath: "exiftool", FfprobePath: "ffprobe"}
}

// Available reports whether both required binaries resolve on PATH.
func (e *Extractor) Available() error {
	for _, bin := range []string{e.ExiftoolPath, e.FfprobePath} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("required binary %q not found: %w", bin, err)
		}
	}
	return nil
}

// Extract reads embedded metadata from path. ffprobe values (width/height/
// duration) take precedence over exiftool's for those fields (ADR-004).
func (e *Extractor) Extract(ctx context.Context, path string) (Extracted, error) {
	// Pass an absolute path to the subprocesses: an absolute path always begins
	// with a separator (or drive letter), so a filename can never be mis-parsed
	// as a CLI option (argv flag smuggling). Defense-in-depth — the scanner only
	// ever feeds filesystem paths under MEDIA_PATH, never untrusted input.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	exifRaw, err := e.runExiftool(ctx, path)
	if err != nil {
		return Extracted{}, err
	}
	ex := mapExiftool(exifRaw)

	if probe, err := e.runFfprobe(ctx, path); err == nil {
		p := mapFfprobe(probe)
		if p.width > 0 {
			ex.Width, ex.Height = p.width, p.height
		}
		if p.durationSec > 0 {
			ex.DurationSec = p.durationSec
		}
		ex.VideoCodec, ex.AudioCodec = p.videoCodec, p.audioCodec
		ex.BitrateKbps, ex.Container = p.bitrateKbps, p.container
	}
	return ex, nil
}

func (e *Extractor) runExiftool(ctx context.Context, path string) (map[string]any, error) {
	// -j JSON, -n numeric (unformatted) values, -api largefilesupport for big files.
	out, err := exec.CommandContext(ctx, e.ExiftoolPath, "-j", "-api", "largefilesupport=1", path).Output()
	if err != nil {
		return nil, fmt.Errorf("exiftool: %w", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil {
		return nil, fmt.Errorf("parse exiftool json: %w", err)
	}
	if len(arr) == 0 {
		return map[string]any{}, nil
	}
	return arr[0], nil
}

type ffprobeOut struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
	Format struct {
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
		FormatName string `json:"format_name"`
	} `json:"format"`
}

func (e *Extractor) runFfprobe(ctx context.Context, path string) (ffprobeOut, error) {
	var out ffprobeOut
	raw, err := exec.CommandContext(ctx, e.FfprobePath,
		"-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		return out, fmt.Errorf("ffprobe: %w", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("parse ffprobe json: %w", err)
	}
	return out, nil
}

// probeResult is the subset of ffprobe data Holodex stores (dimensions,
// duration, codecs, bitrate, container).
type probeResult struct {
	width, height, durationSec, bitrateKbps int
	videoCodec, audioCodec, container       string
}

func mapFfprobe(p ffprobeOut) probeResult {
	var r probeResult
	for _, s := range p.Streams {
		switch s.CodecType {
		case "video":
			if r.videoCodec == "" {
				r.videoCodec = s.CodecName
			}
			if r.width == 0 && s.Width > 0 {
				r.width, r.height = s.Width, s.Height
			}
		case "audio":
			if r.audioCodec == "" {
				r.audioCodec = s.CodecName
			}
		}
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(p.Format.Duration), 64); err == nil {
		r.durationSec = int(f + 0.5)
	}
	if b, err := strconv.ParseInt(strings.TrimSpace(p.Format.BitRate), 10, 64); err == nil {
		r.bitrateKbps = int(b / 1000)
	}
	r.container = normalizeContainer(p.Format.FormatName)
	return r
}

// normalizeContainer turns ffprobe's comma-joined format_name (e.g.
// "mov,mp4,m4a,3gp,3g2,mj2" or "matroska,webm") into one friendly label.
func normalizeContainer(formatName string) string {
	f := strings.ToLower(strings.TrimSpace(formatName))
	switch {
	case strings.Contains(f, "matroska"):
		return "Matroska"
	case strings.Contains(f, "webm"):
		return "WebM"
	case strings.Contains(f, "mp4"):
		return "MP4"
	default:
		// "" and single-token names fall through unchanged; comma-joined lists
		// (none of the above) keep their first element.
		if i := strings.IndexByte(f, ','); i > 0 {
			return f[:i]
		}
		return f
	}
}

// Field-key classification (case-insensitive). exiftool surfaces MP4 atoms and
// MKV tags under friendly names; we match a known set per category (ADR-004/013).
var (
	titleKeys  = newKeySet("Title")
	peopleKeys = newKeySet("Artist", "AlbumArtist", "Cast", "Actor", "Actors", "Performer", "Director", "Producer")
	tagKeys    = newKeySet("Genre", "Genres", "Keywords", "Category", "Categories")
	dateKeys   = newKeySet("RecordedDate", "DateTimeOriginal", "CreationDate", "ContentCreateDate",
		"MediaCreateDate", "CreateDate", "DateRecorded", "DateTagged", "Date")
)

// coverArtKeys are the embedded cover-image tags exiftool surfaces. Their
// presence flags Tier-1 art availability (ADR-009); the bytes are extracted
// separately by the thumbnail pipeline via `exiftool -b`.
//
// Exiftool uses different names across versions / container groups:
//   - "CoverArt"  — QuickTime/MP4 (older exiftool)
//   - "Cover Art" — QuickTime/MP4 (newer exiftool prints with a space in JSON)
//   - "Artwork"   — QuickTime/MP4 (exiftool 12+ renamed the covr atom tag)
//   - "Picture"   — ID3v2 (embedded in MP3/FLAC) and generic fallback
var coverArtKeys = newKeySet("CoverArt", "Cover Art", "Artwork", "Picture")

// attachedFileMIMETypeKeys is the exiftool key for a Matroska file attachment's
// MIME type. Unlike coverArtKeys (presence alone flags art), the attachment may be
// a font/subtitle/etc., so the value must start with "image/" to count as cover art.
var attachedFileMIMETypeKeys = newKeySet("AttachedFileMIMEType")

// excludedKeys are filesystem/tool/binary keys never captured as human metadata
// (ADR-013 excludes cover-art blobs, core-six source keys, and noise).
var excludedKeys = newKeySet(
	"SourceFile", "ExifToolVersion", "FileName", "Directory", "FileSize",
	"FileModifyDate", "FileAccessDate", "FileCreateDate", "FileInodeChangeDate",
	"FilePermissions", "FileType", "FileTypeExtension", "MIMEType",
	"ImageWidth", "ImageHeight", "ImageSize", "Megapixels", "Duration",
	"ThumbnailImage", "PreviewImage",
	// Matroska attachment fields — consumed by the cover-art path, not human metadata.
	"AttachedFileName", "AttachedFileMIMEType", "AttachedFileData", "AttachedFileUID",
	"AttachedFileDescription",
)

// mkvLangSuffix matches the Matroska SimpleTag language code exiftool appends to
// each localizable tag name (e.g. "Title-und", "Artist-eng"). "-und" =
// "undetermined"; otherwise a 2-3 letter ISO-639 code. MP4/QuickTime atoms have
// no such suffix. The "[a-z]{2,3}" guard (letters, not "\w") matches "und" and
// every ISO-639 code while leaving keys like "CRC-32" and MP4 atom names alone.
var mkvLangSuffix = regexp.MustCompile(`(?i)-[a-z]{2,3}$`)

// canonicalKey strips a trailing Matroska language suffix so a tag is addressed
// the same whether it came from an MKV (suffixed) or an MP4 atom (bare) — both
// for classification and as the Extra SourceKey persisted to extra_metadata, so a
// single mapping source (e.g. "file:SeasonNumber") matches across containers.
func canonicalKey(key string) string {
	return mkvLangSuffix.ReplaceAllString(strings.TrimSpace(key), "")
}

// mapExiftool converts exiftool's flat JSON object into normalized fields,
// capturing every other human-meaningful tag into Extra (F2.9).
func mapExiftool(m map[string]any) Extracted {
	var ex Extracted
	for key, raw := range m {
		val := ToString(raw)
		// Canonicalize the key (strip any MKV language suffix) so MKV SimpleTags like
		// "Title-und"/"Artist-und" classify — and are captured into Extra — the same
		// as bare MP4 atoms (issue #63). Unmapped tags (PartNumber, SeasonNumber,
		// EpisodeSort, …) thus land in extra_metadata under a container-agnostic key,
		// ready for a `file:<Key>` mapping to consume.
		ck := canonicalKey(key)
		switch {
		case titleKeys.has(ck):
			if ex.Title == "" {
				ex.Title = val
			}
		case peopleKeys.has(ck):
			ex.People = append(ex.People, splitMulti(val)...)
		case tagKeys.has(ck):
			ex.Tags = append(ex.Tags, splitMulti(val)...)
		case dateKeys.has(ck):
			if ex.RecordedAt == nil {
				if t := parseDate(val); t != nil {
					ex.RecordedAt = t
				}
			}
		case coverArtKeys.has(ck):
			ex.HasCoverArt = true
		case attachedFileMIMETypeKeys.has(ck):
			// Matroska attachment: flag cover art when the attachment is an image.
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(val)), "image/") {
				ex.HasCoverArt = true
			}
		case excludedKeys.has(ck) || isBinaryValue(val):
			// skip
		default:
			if v := strings.TrimSpace(val); v != "" {
				ex.Extra = append(ex.Extra, model.ExtraMetadata{SourceKey: ck, Value: v})
			}
		}
	}
	ex.People = dedupe(ex.People)
	ex.Tags = dedupe(ex.Tags)
	return ex
}

// ---- value helpers ----

type keySet map[string]struct{}

func newKeySet(keys ...string) keySet {
	s := make(keySet, len(keys))
	for _, k := range keys {
		s[strings.ToLower(k)] = struct{}{}
	}
	return s
}

func (s keySet) has(key string) bool {
	_, ok := s[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// ToString converts one exiftool JSON scalar value to its string form (shared
// with internal/writeback's pre-write snapshot reader, F48.9 ADR-067, which
// wraps this for its own multi-value-array case).
func ToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// splitMulti splits a multi-value tag on common separators and trims each.
func splitMulti(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '/' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, v := range in {
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

// dateLayouts covers exiftool's common date renderings (and bare year).
var dateLayouts = []string{
	"2006:01:02 15:04:05-07:00",
	"2006:01:02 15:04:05",
	"2006:01:02",
	time.RFC3339,
	"2006-01-02",
	"2006",
}

func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "0000") {
		return nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

// isBinaryValue detects exiftool's placeholder for binary blobs it didn't dump.
func isBinaryValue(v string) bool {
	return strings.HasPrefix(v, "(Binary data") || strings.Contains(v, "use -b option")
}
