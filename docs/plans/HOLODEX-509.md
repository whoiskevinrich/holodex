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
- [x] backend → `internal/api/genre_writeback.go`
- [x] testing `testing-strategy` — unattached-alias + case-variant cases in genre_writeback_test.go

## Up next — ordered (position = priority)

1. [ ] Squash-merge the PR on green CI, then confirm CI moved HOLODEX-509 to Done

## Session log — newest first (cap: last 8 sessions; older → archive/)
