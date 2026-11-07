---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-630
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: backend             # a rename-token bug with no UI change
depends-on: []
release_note: Renamed files now name each cast member once, by their name, instead of listing an alias beside it.
---

# HOLODEX-630 · {people} lists a person's alias beside their name (F86 bug)

Done means the `{people}` rename token names each linked person once, by their canonical name,
whichever spelling (name or alias) the file tags and sources carried.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. `file-rename.md` token table: `{people}` is each person once, by their name, never an alias.
- [x] backend → `{cmd,internal,providers}/**`. `canonicalPeople` in `internal/api/rename.go` maps each actors value through the video's linked people (names and aliases, any role) to the canonical name, deduplicated, in field order; unlinked values pass through.
- [x] testing `testing-strategy`. `TestRenamePeopleUseCanonicalName` covers an alias from one file tag plus the name from another, and a role-less (pre-F40) link; `internal/api` and `internal/rename` suites pass after merging main.

## Up next — ordered (position = priority)

1. [ ] [—] On merge, check HOLODEX-630 moved to Done (CI moves the branch's own key).

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-07 · canonical people names
- skills: code-review high --fix (3 fixed: role-less links, deduped alias IN list, spec row; 1 skipped: Go Unicode vs SQLite ASCII case fold, unreachable after the resolver's own fold)
- handoff: Fix, spec row and test landed with every gate settled, so the PR opens ready for review; next is merging it. Files already named with an alias will show as Out of date in File names.
