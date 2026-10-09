package metadata

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Matroska tag target levels (TargetTypeValue). Only the levels the selection
// rule names are listed; an absent value means 50.
const (
	mkvLevelMovie      = 50 // MOVIE / EPISODE / ALBUM — the video itself
	mkvLevelSeason     = 60 // SEASON / EDITION
	mkvLevelCollection = 70 // COLLECTION
)

// EBML element IDs the reader needs (IDs keep their length-marker bits).
const (
	ebmlIDHeader     = 0x1A45DFA3
	ebmlIDSegment    = 0x18538067
	ebmlIDInfo       = 0x1549A966
	ebmlIDTags       = 0x1254C367
	ebmlIDTag        = 0x7373
	ebmlIDTargets    = 0x63C0
	ebmlIDTargetType = 0x68CA
	ebmlIDTrackUID   = 0x63C5
	ebmlIDEditionUID = 0x63C9
	ebmlIDChapterUID = 0x63C4
	ebmlIDAttachUID  = 0x63C6
	ebmlIDSimpleTag  = 0x67C8
	ebmlIDTagName    = 0x45A3
	ebmlIDTagString  = 0x4487
)

// ebmlMaxReadLength caps an Info or Tags payload; a larger one is treated as corrupt.
const ebmlMaxReadLength = 16 << 20

// mkvSimpleTag is one top-level SimpleTag with the target it was declared under.
type mkvSimpleTag struct {
	Name  string // TagName as written (conventionally upper snake case)
	Value string
	Level int  // TargetTypeValue, 50 when absent
	Bound bool // targets a specific track, edition, chapter or attachment
}

// mkvTags is what the reader extracts from a Matroska file: which segment Info
// elements that share a key with a standard tag are present, and every
// top-level SimpleTag, in file order.
type mkvTags struct {
	InfoKeys map[string]bool // exiftool key, e.g. "Title"
	Tags     []mkvSimpleTag
}

// mkvInfoKeys are the segment Info elements exiftool names like a standard tag
// (Title; DateUTC as DateTimeOriginal, the name it gives DATE_RECORDED). Info
// tags outrank standard tags in exiftool, so when one is present exiftool's
// value for that key is already the Info one.
var mkvInfoKeys = map[uint64]string{
	0x7BA9: "Title",
	0x4461: "DateTimeOriginal",
}

// isMatroska reports whether path has a Matroska-family extension.
func isMatroska(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv", ".mka", ".mks", ".webm":
		return true
	}
	return false
}

// readMatroskaTags reads the segment Info title and all Tags elements of a
// Matroska file. It walks the Segment's level-1 elements, skipping each by its
// size, so Clusters are never read. exiftool flattens Tags without regard to
// target level (HOLODEX-536); this reader keeps the level so the selection rule
// can be applied. An unknown-size element other than the Segment cannot be
// skipped and ends the walk with an error.
func readMatroskaTags(path string) (mkvTags, error) {
	f, err := os.Open(path)
	if err != nil {
		return mkvTags{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return mkvTags{}, err
	}
	return parseMatroskaTags(f, st.Size())
}

func parseMatroskaTags(r io.ReaderAt, fileSize int64) (mkvTags, error) {
	var out mkvTags
	id, size, unknown, hl, err := readElementHeader(r, 0)
	if err != nil {
		return out, err
	}
	if id != ebmlIDHeader || unknown {
		return out, errors.New("matroska: missing EBML header")
	}
	pos := int64(hl) + int64(size)

	id, size, unknown, hl, err = readElementHeader(r, pos)
	if err != nil {
		return out, err
	}
	if id != ebmlIDSegment {
		return out, errors.New("matroska: missing Segment")
	}
	pos += int64(hl)
	end := fileSize
	if !unknown && pos+int64(size) < end {
		end = pos + int64(size)
	}

	for pos < end {
		id, size, unknown, hl, err = readElementHeader(r, pos)
		if errors.Is(err, io.EOF) {
			break // a truncated tail after the last complete element
		}
		if err != nil {
			return out, err
		}
		if unknown {
			// Typically a live-muxed Cluster: its end can only be found by parsing it.
			return out, fmt.Errorf("matroska: unknown-size element %#x at %d", id, pos)
		}
		data := pos + int64(hl)
		if id == ebmlIDInfo || id == ebmlIDTags {
			payload, err := readPayload(r, data, size)
			if err != nil {
				return out, err
			}
			if id == ebmlIDInfo {
				for _, c := range children(payload) {
					if key, ok := mkvInfoKeys[c.id]; ok {
						if out.InfoKeys == nil {
							out.InfoKeys = map[string]bool{}
						}
						out.InfoKeys[key] = true
					}
				}
			} else {
				out.Tags = append(out.Tags, parseTags(payload)...)
			}
		}
		pos = data + int64(size)
	}
	return out, nil
}

// parseTags decodes a Tags payload into its top-level SimpleTags. Nested
// SimpleTags (a sub-tag of another tag) and binary values are not collected.
func parseTags(b []byte) []mkvSimpleTag {
	var out []mkvSimpleTag
	for _, tag := range children(b) {
		if tag.id != ebmlIDTag {
			continue
		}
		level, bound := mkvLevelMovie, false
		var simple []mkvSimpleTag
		for _, c := range children(tag.data) {
			switch c.id {
			case ebmlIDTargets:
				for _, t := range children(c.data) {
					switch t.id {
					case ebmlIDTargetType:
						level = int(readUint(t.data))
					case ebmlIDTrackUID, ebmlIDEditionUID, ebmlIDChapterUID, ebmlIDAttachUID:
						// UID 0 means "all of them", i.e. not bound to one.
						if readUint(t.data) != 0 {
							bound = true
						}
					}
				}
			case ebmlIDSimpleTag:
				name, okName := firstChild(c.data, ebmlIDTagName)
				value, okValue := firstChild(c.data, ebmlIDTagString)
				if okName && okValue {
					simple = append(simple, mkvSimpleTag{Name: string(name), Value: string(value)})
				}
			}
		}
		for i := range simple {
			simple[i].Level, simple[i].Bound = level, bound
		}
		out = append(out, simple...)
	}
	return out
}

// selectMatroskaTags rewrites exiftool's flattened output so each Matroska tag
// key carries the value that describes the video, not whichever target level
// happened to come first in the file:
//
//   - a tag bound to a track, edition, chapter or attachment never counts;
//   - single-value keys take the first level-50 value, falling back to 60, then 70;
//   - multi-value keys (people, tags) take every level-50 value and never inherit
//     from season or collection levels;
//   - a segment Info element exiftool names the same (Title, DateTimeOriginal)
//     keeps exiftool's value, which is already the Info one.
//
// Keys with no eligible value are removed. Keys exiftool emitted that match no
// SimpleTag (other Info fields, attachments, track facts) pass through untouched.
func selectMatroskaTags(raw map[string]any, mt mkvTags) map[string]any {
	byKey := map[string]map[int][]string{} // normalized key -> level -> values
	for _, t := range mt.Tags {
		k := matroskaTagKey(t.Name)
		if byKey[k] == nil {
			byKey[k] = map[int][]string{}
		}
		if !t.Bound {
			v := t.Value
			if strings.HasPrefix(strings.ToUpper(t.Name), "DATE_") {
				v = mkvDateSep.ReplaceAllString(v, "$1:$2:") // as exiftool renders it
			}
			byKey[k][t.Level] = append(byKey[k][t.Level], v)
		}
	}
	if len(byKey) == 0 {
		return raw
	}

	out := make(map[string]any, len(raw))
	names := map[string]string{} // normalized key -> exiftool's canonical spelling
	for k, v := range raw {
		ck := canonicalKey(k)
		nk := normalizeTagKey(ck)
		if _, ok := byKey[nk]; !ok {
			out[k] = v
			continue
		}
		if mt.InfoKeys[k] {
			out[k] = v
		}
		if prev, seen := names[nk]; !seen || ck < prev {
			names[nk] = ck // deterministic when several language variants exist
		}
	}
	for nk, name := range names {
		if mt.InfoKeys[name] {
			continue
		}
		levels := byKey[nk]
		var value string
		if peopleKeys.has(nk) || tagKeys.has(nk) {
			value = strings.Join(levels[mkvLevelMovie], ", ")
		} else {
			for _, lvl := range []int{mkvLevelMovie, mkvLevelSeason, mkvLevelCollection} {
				if vs := levels[lvl]; len(vs) > 0 {
					value = vs[0]
					break
				}
			}
		}
		if value != "" {
			out[name] = value
		}
	}
	return out
}

// mkvDateSep matches the date separators of a Matroska DATE_* value
// ("2001-04-25 ..."), which exiftool rewrites to its own "2001:04:25 ..." form
// (Image::ExifTool::Matroska %dateInfo); substituted values must match it.
var mkvDateSep = regexp.MustCompile(`^(\d{4})-(\d{2})-`)

// matroskaTagAliases are the standard tag names exiftool renames rather than
// camel-casing (Image::ExifTool::Matroska::StdTag).
var matroskaTagAliases = map[string]string{
	"daterecorded":  "datetimeoriginal",
	"datedigitized": "createdate",
}

// matroskaTagKey maps a SimpleTag TagName (e.g. "PART_NUMBER") to the
// normalized form of the key exiftool emits for it ("PartNumber").
func matroskaTagKey(name string) string {
	k := normalizeTagKey(name)
	if alias, ok := matroskaTagAliases[k]; ok {
		return alias
	}
	return k
}

// normalizeTagKey lowercases a key and drops everything but letters and digits.
func normalizeTagKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---- EBML primitives ----

type ebmlElement struct {
	id   uint64
	data []byte
}

// children splits a master element's payload into its child elements. A
// malformed child ends the list; what was decoded before it is kept.
func children(b []byte) []ebmlElement {
	var out []ebmlElement
	for len(b) > 0 {
		id, idLen, ok := readVint(b, false)
		if !ok {
			break
		}
		size, sizeLen, ok := readVint(b[idLen:], true)
		hl := idLen + sizeLen
		if !ok || size == unknownSize(sizeLen) || size > uint64(len(b)-hl) {
			break
		}
		out = append(out, ebmlElement{id: id, data: b[hl : hl+int(size)]})
		b = b[hl+int(size):]
	}
	return out
}

func firstChild(b []byte, id uint64) ([]byte, bool) {
	for _, c := range children(b) {
		if c.id == id {
			return c.data, true
		}
	}
	return nil, false
}

func readUint(b []byte) uint64 {
	var v uint64
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	return v
}

// readElementHeader reads the ID and size at off, returning the header length.
func readElementHeader(r io.ReaderAt, off int64) (id, size uint64, unknown bool, hl int, err error) {
	var buf [12]byte // 4-byte ID + 8-byte size at most
	n, err := r.ReadAt(buf[:], off)
	if n == 0 {
		if err == nil {
			err = io.EOF
		}
		return 0, 0, false, 0, err
	}
	b := buf[:n]
	id, idLen, ok := readVint(b, false)
	if !ok || idLen > 4 {
		return 0, 0, false, 0, fmt.Errorf("matroska: bad element ID at %d", off)
	}
	size, sizeLen, ok := readVint(b[idLen:], true)
	if !ok {
		if n < len(buf) {
			return 0, 0, false, 0, io.EOF
		}
		return 0, 0, false, 0, fmt.Errorf("matroska: bad element size at %d", off)
	}
	return id, size, size == unknownSize(sizeLen), idLen + sizeLen, nil
}

func readPayload(r io.ReaderAt, off int64, size uint64) ([]byte, error) {
	if size > ebmlMaxReadLength {
		return nil, fmt.Errorf("matroska: element at %d too large (%d bytes)", off, size)
	}
	buf := make([]byte, size)
	if n, err := r.ReadAt(buf, off); n < len(buf) {
		return nil, fmt.Errorf("matroska: element at %d truncated: %w", off, err)
	}
	return buf, nil
}

// readVint decodes an EBML variable-length integer. With strip set the length
// marker is removed (sizes and values); without it the marker is kept (IDs).
func readVint(b []byte, strip bool) (uint64, int, bool) {
	if len(b) == 0 || b[0] == 0 {
		return 0, 0, false
	}
	n := 1
	for mask := byte(0x80); b[0]&mask == 0; mask >>= 1 {
		n++
	}
	if n > 8 || len(b) < n {
		return 0, 0, false
	}
	v := uint64(b[0])
	if strip {
		v &= uint64(0xFF >> n)
	}
	for _, x := range b[1:n] {
		v = v<<8 | uint64(x)
	}
	return v, n, true
}

// unknownSize is the reserved all-ones size value for an n-byte size field.
func unknownSize(n int) uint64 {
	return 1<<(7*uint(n)) - 1
}
