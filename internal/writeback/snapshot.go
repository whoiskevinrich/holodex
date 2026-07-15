package writeback

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"holodex/internal/metadata"
)

// ReadCurrentValues reads each mapped field's current on-disk tag value via
// exiftool, keyed by canonical field (F48.9, ADR-067) — the pre-write
// snapshot the caller records immediately before invoking WriteBatch. A tag
// with no current value (or that exiftool doesn't report at all) reads back
// as "" — the field simply had nothing before this write, matching the
// snapshot table's "'' if previously absent" contract.
//
// Image fields (cover art / attachments) are skipped: there is no text value
// to snapshot for a binary tag, and cover art already has its own detection
// path (ADR-009).
//
// This mirrors internal/metadata.Extractor's exiftool invocation, but reads
// the exact target tag names the write is about to overwrite rather than the
// broader canonical field set — the two serve different purposes and share
// no code (metadata.Extract is unavailable outside its own package).
func ReadCurrentValues(ctx context.Context, path string, mapped []Mapped) (map[string]string, error) {
	out := make(map[string]string, len(mapped))
	tagArgs := make([]string, 0, len(mapped))
	for _, m := range mapped {
		if !m.IsImage {
			tagArgs = append(tagArgs, "-"+m.TagName)
		}
	}
	if len(tagArgs) == 0 {
		return out, nil
	}

	args := append(tagArgs, "-j", "-api", "largefilesupport=1", path)
	raw, err := exec.CommandContext(ctx, "exiftool", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("writeback read current values: %w", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("writeback read current values: parse json: %w", err)
	}
	var current map[string]any
	if len(arr) > 0 {
		current = arr[0]
	}

	for _, m := range mapped {
		if m.IsImage {
			continue
		}
		out[m.Field] = currentTagValue(current, m.TagName)
	}
	return out, nil
}

// currentTagValue looks up tagName in exiftool's JSON output. exiftool's JSON
// keys are the bare tag name even when the request used a group-qualified
// name (e.g. "QuickTime:Title") to disambiguate a write, so any "Group:"
// prefix is stripped before the lookup. Multi-valued tags come back as a JSON
// array; joined with "\n" to match how WriteBatch's own multi-value fields
// are audited (see writequeue.process's strings.Join(m.Values, "\n")), so a
// round-trip snapshot → revert splits back into the same values.
func currentTagValue(m map[string]any, tagName string) string {
	if m == nil {
		return ""
	}
	key := tagName
	if i := strings.LastIndex(key, ":"); i >= 0 {
		key = key[i+1:]
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	return snapshotValueToString(v)
}

// snapshotValueToString handles the one case metadata.ToString doesn't need
// (a multi-valued tag's JSON array); every scalar case defers to it so the
// two exiftool-JSON converters don't drift.
func snapshotValueToString(v any) string {
	arr, ok := v.([]any)
	if !ok {
		return metadata.ToString(v)
	}
	parts := make([]string, len(arr))
	for i, e := range arr {
		parts[i] = snapshotValueToString(e)
	}
	return strings.Join(parts, "\n")
}
