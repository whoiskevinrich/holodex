package maintenance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"holodex/internal/writeback"
)

// Tools names the binaries the pipeline runs. Paths, not shell strings; every call is argv.
type Tools struct {
	FFmpeg, FFprobe, Exiftool, Mkvmerge string
}

// Demuxer is the ffmpeg/ffprobe input format forced for a container family.
func Demuxer(family string) string {
	if family == FamilyMKV {
		return "matroska"
	}
	return "mov"
}

// CollectSnapshot gathers everything Verify compares about path: the probe, a digest per
// non-attachment stream, and the container facts ffprobe can't see (XMP for MP4, the
// attachment list for Matroska).
func CollectSnapshot(ctx context.Context, t Tools, family, path string, p Probe) (Snapshot, error) {
	hashes, err := streamHashes(ctx, t.FFmpeg, family, path, p)
	if err != nil {
		return Snapshot{}, err
	}
	s := Snapshot{Family: family, Probe: p, StreamHashes: hashes}
	switch family {
	case FamilyMP4:
		out, err := run(ctx, t.Exiftool, "-j", "-G1", "-XMP:all", toolArg(path))
		if err != nil {
			return Snapshot{}, fmt.Errorf("read XMP: %w", err)
		}
		if s.Extra, err = parseXMP(out); err != nil {
			return Snapshot{}, err
		}
	case FamilyMKV:
		id, err := identifyMKV(ctx, t.Mkvmerge, path)
		if err != nil {
			return Snapshot{}, err
		}
		s.Extra = id.attachmentFacts()
	}
	return s, nil
}

// streamHashes digests every non-attachment stream through ffmpeg's streamhash muxer, keyed by
// input stream index. Attachments aren't packet streams; Matroska's are compared through the
// mkvmerge listing instead.
func streamHashes(ctx context.Context, ffmpeg, family, path string, p Probe) (map[int]string, error) {
	args := inputArgs(Demuxer(family), path)
	var outToIn []int
	for _, s := range p.Streams {
		if s.CodecType == "attachment" {
			continue
		}
		args = append(args, "-map", "0:"+strconv.Itoa(s.Index))
		outToIn = append(outToIn, s.Index)
	}
	args = append(args, "-c", "copy", "-f", "streamhash", "-hash", "sha256", "-")
	out, err := run(ctx, ffmpeg, args...)
	if err != nil {
		return nil, fmt.Errorf("stream digests: %w", err)
	}
	return parseStreamHash(out, outToIn)
}

// parseStreamHash reads streamhash lines ("<out>,<type>,SHA256=<hex>") into input indexes.
func parseStreamHash(out []byte, outToIn []int) (map[int]string, error) {
	hashes := map[int]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ",", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("stream digests: unexpected line %q", line)
		}
		n, err := strconv.Atoi(parts[0])
		if err != nil || n < 0 || n >= len(outToIn) {
			return nil, fmt.Errorf("stream digests: bad stream %q", parts[0])
		}
		hashes[outToIn[n]] = parts[2]
	}
	if len(hashes) != len(outToIn) {
		return nil, fmt.Errorf("stream digests: %d of %d streams", len(hashes), len(outToIn))
	}
	return hashes, nil
}

// parseXMP flattens exiftool's JSON for one file into "xmp:<Group:Tag>" keys. Values are kept
// as their JSON text so lists and structs compare exactly.
func parseXMP(out []byte) (map[string]string, error) {
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(out, &rows); err != nil || len(rows) != 1 {
		return nil, fmt.Errorf("read XMP: unexpected exiftool output")
	}
	facts := map[string]string{}
	for k, v := range rows[0] {
		if k == "SourceFile" {
			continue
		}
		facts["xmp:"+k] = string(v)
	}
	return facts, nil
}

// mkvIdentity is the part of `mkvmerge -J` maintenance reads.
type mkvIdentity struct {
	Tracks []struct {
		ID         int `json:"id"`
		Properties struct {
			UID json.Number `json:"uid"`
		} `json:"properties"`
	} `json:"tracks"`
	Attachments []struct {
		ID          int    `json:"id"`
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		Description string `json:"description"`
		Size        int64  `json:"size"`
	} `json:"attachments"`
}

func identifyMKV(ctx context.Context, mkvmerge, path string) (mkvIdentity, error) {
	out, err := run(ctx, mkvmerge, "-J", toolArg(path))
	if err != nil {
		return mkvIdentity{}, fmt.Errorf("identify: %w", err)
	}
	return parseMkvIdentity(out)
}

func parseMkvIdentity(out []byte) (mkvIdentity, error) {
	var id mkvIdentity
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber() // a track UID is a uint64; a float64 would round it
	if err := dec.Decode(&id); err != nil {
		return mkvIdentity{}, fmt.Errorf("identify: %w", err)
	}
	return id, nil
}

// attachmentFacts keys each attachment by position and name, so a reordered, renamed,
// retyped or resized attachment all differ.
func (id mkvIdentity) attachmentFacts() map[string]string {
	facts := map[string]string{}
	for i, a := range id.Attachments {
		facts["attachment:"+strconv.Itoa(i)+":"+a.FileName] = a.ContentType + "|" + strconv.FormatInt(a.Size, 10) + "|" + a.Description
	}
	return facts
}

// trackUID returns the UID of the track with mkvmerge id tid.
func (id mkvIdentity) trackUID(tid int) (string, bool) {
	for _, t := range id.Tracks {
		if t.ID == tid {
			return t.Properties.UID.String(), true
		}
	}
	return "", false
}

// Matroska tags XML, as mkvextract emits it. Targets keeps its raw XML (TargetTypeValue and
// friends survive verbatim) and also exposes the TrackUIDs it names.
type mkvTagsDoc struct {
	XMLName xml.Name `xml:"Tags"`
	Tags    []mkvTag `xml:"Tag"`
}

type mkvTag struct {
	Targets struct {
		Inner     string   `xml:",innerxml"`
		TrackUIDs []string `xml:"TrackUID"`
	} `xml:"Targets"`
	Simples []struct {
		Name  string `xml:"Name"`
		Inner string `xml:",innerxml"`
	} `xml:"Simple"`
}

var trackUIDEl = regexp.MustCompile(`(?s)<TrackUID>.*?</TrackUID>\s*`)

// trackTagsXML extracts the tags the original file attaches to track uid, ready for
// `mkvmerge --tags 0:<file>` on the re-encoded track: TrackUID targets removed (mkvmerge
// assigns the new track), statistics and mux stamps dropped (mkvmerge regenerates them).
// ok is false when the track has no tags of its own to carry.
func trackTagsXML(doc, uid string) (out string, ok bool, err error) {
	if strings.TrimSpace(doc) == "" {
		return "", false, nil
	}
	var parsed mkvTagsDoc
	if err := xml.Unmarshal([]byte(doc), &parsed); err != nil {
		return "", false, fmt.Errorf("parse tags: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Tags>\n")
	for _, t := range parsed.Tags {
		if !slices.Contains(t.Targets.TrackUIDs, uid) {
			continue
		}
		var simples []string
		for _, s := range t.Simples {
			if !isMuxStamp(s.Name) && !isEncodedStat(s.Name) {
				simples = append(simples, s.Inner)
			}
		}
		if len(simples) == 0 {
			continue
		}
		ok = true
		sb.WriteString("<Tag><Targets>")
		sb.WriteString(trackUIDEl.ReplaceAllString(t.Targets.Inner, ""))
		sb.WriteString("</Targets>")
		for _, s := range simples {
			sb.WriteString("<Simple>" + s + "</Simple>")
		}
		sb.WriteString("</Tag>\n")
	}
	sb.WriteString("</Tags>\n")
	return sb.String(), ok, nil
}

// PrepareTrackTags writes the original main video track's own tags to a temp file for
// MKVMergeArgs and returns its path, or "" when the track has none. The caller removes it.
func PrepareTrackTags(ctx context.Context, t Tools, src string, p Probe) (string, error) {
	v, ok := p.MainVideo()
	if !ok {
		return "", refuse(ReasonNoVideo, "")
	}
	id, err := identifyMKV(ctx, t.Mkvmerge, src)
	if err != nil {
		return "", err
	}
	uid, ok := id.trackUID(mkvTrackID(p, v))
	if !ok {
		return "", fmt.Errorf("track tags: no mkvmerge track for stream %d", v.Index)
	}
	doc, err := writeback.ExistingTagsXML(ctx, src)
	if err != nil {
		return "", fmt.Errorf("track tags: %w", err)
	}
	xmlDoc, has, err := trackTagsXML(doc, uid)
	if err != nil || !has {
		return "", err
	}
	f, err := os.CreateTemp("", "holodex-track-tags-*.xml")
	if err != nil {
		return "", fmt.Errorf("track tags: %w", err)
	}
	if _, err := f.WriteString(xmlDoc); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("track tags: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("track tags: %w", err)
	}
	return f.Name(), nil
}

// run executes a tool and returns stdout; stderr is folded into the error.
func run(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			// MKVToolNix reports its errors on stdout.
			msg = strings.TrimSpace(string(out))
		}
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		return out, fmt.Errorf("%s: %w: %s", bin, err, msg)
	}
	return out, nil
}
