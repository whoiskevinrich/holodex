---
key: HOLODEX-511
status: in-progress
profile: backend
depends-on: []
release_note: "Searching a studio or tag, by its name or any alias, now lists its videos too, after title and people matches."
---

# HOLODEX-511 · Global search returns a matched studio's and tag's media

The "+ its media" half of F43 P0-9 (spun out of HOLODEX-508). Only people folded their videos into
search's Videos group; studio and tag matches never did. Done = the spec states the ordering, plus repo
tests that a studio alias search and a tag alias search each return that entity's videos.

**Design package:** spec [entity-identity.md](../specs/entity-identity.md) P0-9 — ordering settled with
the owner: specific → broad (title, person, studio, tag), and a tag's media includes its sub-tags like
the tag page (F50) · no ADR (completes ADR-061 / ADR-036) · no UI.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — P0-9 "+ its media" ordering + three acceptance cases
- [x] backend → `{cmd,internal,providers}/**` — `VideoFilter.StudioIDsAny`/`TagIDsAny`, Search top-up loop
- [x] testing `testing-strategy` — `TestSearchReturnsStudioAliasMedia`, `TestSearchReturnsTagAliasMedia`, `TestSearchVideoOrdering`; all three failed before the fix

## Up next — ordered (position = priority)

1. [ ] [—] Review and merge the PR; CI moves HOLODEX-511 to Done

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-02 · ordering decided, fix built and tested in one session
- skills: code-review (high --fix)
- note: code-review fixed one real issue — `TagIDsAny` first OR'd one recursive CTE per tag (SQLite
  expression-depth risk at limit≈1000); now one `tagSubtreesQuery` seeded with every root. Skipped:
  the discarded `CountVideos` per top-up (pre-existing ListVideos shape) and pickers paying for video
  folds they ignore — neither is a correctness bug.
- note: security — parameterized read-only queries, no auth/access/infra change, so no `/security-review`.
- handoff: Built and green; PR open. Start at: review and merge it.

## Dropped — newest first (the reason is the point)
