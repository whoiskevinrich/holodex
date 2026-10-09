---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-547
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: backend             # a purge-cleanup bug: no UI, no auth or perimeter change
depends-on: []
release_note: "Permanently deleting a video now also removes its field choices, hidden values and other edits, and a one-time cleanup clears those left behind by earlier deletes."
---

# HOLODEX-547 · Permanent delete leaves a video's edits behind

Done when purging a video removes every per-video row that has no foreign key to it (enrichment,
field decisions, curation, not-applicable marks; dismissals already had a trigger), and the orphans
earlier purges left are swept once. Spun out of HOLODEX-521.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F24.4 in `delete-media.md` now names the owner's work among what a purge removes
- [x] backend → `{cmd,internal,providers}/**` — migration 0060: `videos_ad_entity_rows` trigger (the 0024/0049 precedent, so every delete path is covered) + one-time orphan sweep
- [x] testing `testing-strategy` — `TestMigration0060VideoEntityRows`; dropping the trigger or the sweep each turns it red; §24.8/§24.9 updated

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge once CI is green

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-09 · session
- skills: code-review
- handoff: Trigger + sweep in migration 0060, tested and mutation-checked; the PR is ready for review.

## Dropped — newest first (the reason is the point)
