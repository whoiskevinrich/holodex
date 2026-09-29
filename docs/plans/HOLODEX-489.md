---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-489
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: full                # sync contract (ADR) + a new owner-gated endpoint + two UI surfaces
depends-on: []
release_note: A title or release date that Holodex writes to a file but can't read back no longer shows as out of sync or rewrites on every open, and System Activity now lists which fields can't be read back and the one-line mapping change that fixes each.
approved:
  design:
    on: 2026-09-28
    at: b29d3e75
---

# HOLODEX-489 · Read-back gap: ledger witness + owner-visible gap

This is done when a decided text field in `writeback.ReadbackGaps` has its sync witnessed by the write
ledger (written once, then `=`; re-decided, then out of sync), and the gap is visible to the owner in
two places: a muted hint on a never-written row in the writeback dialog, and a *Mapping checks* block
on System Activity (plus a count in the reload-config response). Relates to HOLODEX-488.

**Design package:** [ADR-119](../architecture/ADR-119-ledger-witness-for-readback-gaps.md) ·
[handoff](../design/readback-gap-handoff.md) + [mockup](../design/readback-gap-mockup.svg) ·
spec [field-source-of-truth.md §Sync state](../specs/field-source-of-truth.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — two bullets under §Sync state (the ledger witness for gap fields, and owner visibility)
- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-119 (Proposed); extends ADR-093/101; rejects an implicit read-back source
- [x] design `design-handoff` → `docs/design/**` — handoff + SVG; hint A and diagnostics A picked by the owner 2026-09-28; signed off via `/implement` the same day
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review` — new owner-gated read endpoint exposing mapping config

## Up next — ordered (position = priority)

1. [ ] [backend] `Options.ReadbackGaps` set + ledger witness in `replaceMarkers` for gap fields — `internal/resolver/resolver.go`
2. [ ] [backend] `GET /owner/readback-gaps`, reload-config `readback_gaps`, per-field `readback_gap` on the owner detail read — `internal/api`
3. [ ] [frontend] Dialog hint line (`WritebackFormDialog.svelte`) + Mapping checks block and toast (`routes/owner/status`)
4. [ ] [testing] Resolver table (gap × decided × ledger row), API 401/200, `writebackCockpit` hint predicate; `docs/testing-strategy.md` row

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-28 · session
- skills: architecture, design-handoff, implement
- handoff: Crossed into build: design signed off at b29d3e75, and the draft PR is open. Start at Up next item 1, the resolver ledger witness for gap fields.

## Dropped — newest first (the reason is the point)

- [~] [backend] Implicit read-back source (treat the written tag as a lowest-priority `file:` source) — dropped 2026-09-28: bends ADR-013's explicit mapping and changes precedence (ADR-119 D5)
- [~] [frontend] Hint folded into the file chip (option B) — dropped 2026-09-28: owner chose the muted line (A)
- [~] [frontend] Gap list on Owner › Fields — dropped 2026-09-28: owner chose System Activity, beside Reload config
