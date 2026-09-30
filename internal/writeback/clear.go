package writeback

import (
	"regexp"
	"strings"
)

// Cleared-field tag deletes (ADR-120 D4). An owner-cleared replace field is
// removed from the file by deleting every file tag the field reads from, not
// just the one it writes: studio writes Publisher but reads Publisher, Label,
// Studio and ProductionCompany, so deleting only Publisher would leave the
// mis-parsed value behind in Label.
//
// The names reach exiftool as "-NAME=" and ffmpeg as "NAME=", so a name like
// "all" or "QuickTime:all" would strip every tag from the file. Every delete
// name therefore passes ValidClearTagName — deliberately NOT ValidTagKeyName,
// which is ADR-110's genres tag-key allowlist and must not be widened.

// clearTagName is the shape of the name part: letters and digits, starting
// with a letter. No '-', '=', '<', '>', ':', whitespace or wildcard can pass.
var clearTagName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// ValidClearTagName reports whether tagName may be deleted from a file of the
// given container when clearing the canonical field. It holds only when:
//   - the name part matches clearTagName and is not "all" (any case);
//   - a group prefix appears only on a container whose writer takes one
//     (never Matroska/WebM), and matches the exiftool group shape;
//   - the name part is the field's own write target for this container, or
//     one of sources (the field's un-namespaced file sources from the live
//     mapping) — so no mapping edit can reach a tag outside the field.
func ValidClearTagName(container, canonical, tagName string, sources []string) bool {
	target, ok := TagForField(canonical, container)
	if !ok {
		return false
	}
	group, name, qualified := strings.Cut(tagName, ":")
	if !qualified {
		name, group = group, ""
	}
	if !clearTagName.MatchString(name) || strings.EqualFold(name, "all") {
		return false
	}
	if qualified && (bareTagKeys(container) || !tagKeyGroup.MatchString(group)) {
		return false
	}
	if strings.EqualFold(name, tagPart(target)) {
		return true
	}
	for _, s := range sources {
		if strings.EqualFold(name, s) {
			return true
		}
	}
	return false
}

// ClearTagNames lists the tags to delete to clear canonical on a container: the
// name part of its write target, then each source — all BARE, on every
// container. The scanner reads a tag by bare name from any group, and exiftool
// deletes a bare name from every group, so a bare delete removes exactly what
// the reader would find: an MP4 whose studio sits in XMP:Label is left intact
// by "-QuickTime:Label=" but cleared by "-Label=" (probed 2026-09-29). This is
// deliberately unlike ADR-110's tag-key filter, which must leave other groups
// alone. Only names ValidClearTagName accepts are returned, de-duplicated
// case-insensitively; rejected names come back separately so the caller can
// name them in the job detail. An unmapped field returns nothing.
func ClearTagNames(container, canonical string, sources []string) (names, rejected []string) {
	target, ok := TagForField(canonical, container)
	if !ok {
		return nil, nil
	}
	seen := map[string]bool{}
	add := func(n string) {
		key := strings.ToLower(n)
		if seen[key] {
			return
		}
		seen[key] = true
		if ValidClearTagName(container, canonical, n, sources) {
			names = append(names, n)
		} else {
			rejected = append(rejected, n)
		}
	}
	add(tagPart(target))
	for _, s := range sources {
		add(s)
	}
	return names, rejected
}

// tagPart is the name part of an optionally group-qualified tag name.
func tagPart(tag string) string {
	if _, name, ok := strings.Cut(tag, ":"); ok {
		return name
	}
	return tag
}
