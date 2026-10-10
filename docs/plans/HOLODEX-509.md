---
key: HOLODEX-509
status: in-progress
profile: backend
depends-on: []
release_note: "Genre writeback now writes your tag's own spelling for any genre value that matches a tag by alias, even when that tag isn't attached to the video, instead of the provider's spelling."
---

# HOLODEX-509 · Unattached-tag aliases written canonically in genre writeback

Spun out of HOLODEX-507. P0-10 collapsed a raw genre value into its canonical tag only when the
video carries that tag; an alias of an unattached tag (reachable after the owner detaches the tag
while a provider still supplies it, or when best-effort materialization fails) was written in its
alias spelling. Owner's pick (2026-10-09): **any raw value that resolves to a tag is written as
that tag's canonical name**, attached or not.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — widen P0-10 in entity-identity.md
- [ ] backend → `internal/api/genre_writeback.go`
- [ ] testing `testing-strategy` — unattached-alias case in genre_writeback_test.go

## Up next — ordered (position = priority)

1. [ ] Spec P0-10 edit, push, `/implement`
2. [ ] Write the failing test, then canonicalize raw values in `genreWritebackItemsFrom`

## Session log — newest first (cap: last 8 sessions; older → archive/)
