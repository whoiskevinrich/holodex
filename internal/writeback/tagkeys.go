package writeback

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"holodex/internal/mapping"
	"holodex/internal/metadata"
)

// ExtraTagKeys are the container keys the scanner reads tags from besides
// Genre (internal/metadata tagKeys minus Genre). A genres writeback filters each
// of them that is present on the file down to the written set, so a tag removed
// in the UI can't come back from one of them on rescan (ADR-110 D2). Keep in step
// with internal/metadata's tagKeys — TestExtraTagKeysAreReadAsTags pins it.
var ExtraTagKeys = []string{"Genres", "Keywords", "Category", "Categories"}

// TagKeyFieldPrefix marks a write-job field that targets one container tag key
// directly rather than a canonical field: a derived write of a genres job, or
// its revert (ADR-110 D3). The rest of the field is the tag name as written —
// validated by ValidTagKeyName before it ever reaches a writer.
const TagKeyFieldPrefix = "tagkey:"

// tagKeyGroup is the shape of an exiftool group name (e.g. "Keys", "ItemList").
var tagKeyGroup = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// bareTagKeys reports whether the container's writer addresses tags by bare
// name (ffmpeg -metadata / mkvpropedit Simple) rather than exiftool Group:Name.
func bareTagKeys(container string) bool {
	return container == "Matroska" || container == "WebM"
}

// ValidTagKeyName reports whether tagName may be written as a derived tag-key
// write for the container: its name is one of ExtraTagKeys, and it carries an
// exiftool group only where the container's writer is exiftool. A job payload
// is data (security C2), so this is the whole allowlist for TagKeyFieldPrefix
// fields — nothing else reaches a writer by name.
func ValidTagKeyName(container, tagName string) bool {
	group, name, qualified := strings.Cut(tagName, ":")
	if !qualified {
		name, group = group, ""
	}
	if bareTagKeys(container) == qualified {
		return false
	}
	if qualified && !tagKeyGroup.MatchString(group) {
		return false
	}
	for _, k := range ExtraTagKeys {
		if strings.EqualFold(name, k) {
			return true
		}
	}
	return false
}

// ReadTagKeys reads every ExtraTagKeys instance on the file, keyed by the tag
// name to write it back under (Group:Name on exiftool containers, where the
// same key can sit in more than one group, e.g. MP4 Keys:Keywords; the bare
// name on Matroska/WebM) and split into values the way the scanner splits them.
func ReadTagKeys(ctx context.Context, path, container string) (map[string][]string, error) {
	// Same reach as the scanner (HOLODEX-505): a tag key this filter can't see is
	// never trimmed, so a removed tag would return on the next rescan.
	args := append(metadata.MatroskaSeekArgs(path), "-j", "-G1", "-a", "-api", "largefilesupport=1")
	for _, k := range ExtraTagKeys {
		args = append(args, "-"+k)
	}
	raw, err := exec.CommandContext(ctx, "exiftool", append(args, path)...).Output()
	if err != nil {
		return nil, fmt.Errorf("writeback read tag keys: %w", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("writeback read tag keys: parse json: %w", err)
	}
	out := map[string][]string{}
	if len(arr) == 0 {
		return out, nil
	}
	for key, v := range arr[0] {
		if key == "SourceFile" {
			continue
		}
		name := key
		if bareTagKeys(container) {
			_, name, _ = strings.Cut(key, ":")
		}
		out[name] = append(out[name], splitTagValue(v)...)
	}
	return out, nil
}

// splitTagValue flattens one exiftool JSON value (a string, or a list when
// exiftool itself splits it) into scanner-style values.
func splitTagValue(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, e := range t {
			out = append(out, splitTagValue(e)...)
		}
		return out
	case nil:
		return nil
	default:
		return mapping.SplitMulti(fmt.Sprint(t))
	}
}

// FilterTagKeys derives a genres job's extra writes (ADR-110 D2): each present
// key keeps only the values keep accepts, a key left with none is deleted, and
// an unchanged key is not written at all. It never adds a value or creates a
// key. Results are ordered by tag name so a batch is deterministic.
func FilterTagKeys(present map[string][]string, keep func(string) bool, source string) []Mapped {
	names := make([]string, 0, len(present))
	for name := range present {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []Mapped
	for _, name := range names {
		values := present[name]
		kept := make([]string, 0, len(values))
		for _, v := range values {
			if keep(v) {
				kept = append(kept, v)
			}
		}
		if len(kept) == len(values) {
			continue
		}
		out = append(out, Mapped{
			Field:   TagKeyFieldPrefix + name,
			TagName: name,
			Source:  source,
			Values:  kept,
			Delete:  len(kept) == 0,
		})
	}
	return out
}
