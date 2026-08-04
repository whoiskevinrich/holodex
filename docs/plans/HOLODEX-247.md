---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-247                 # the tracker key; must match the branch key regex
status: done                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Studios now support icon, logo, and poster images — sourced automatically from enrichment providers, or added, replaced, and removed directly on the studio page.
---

# HOLODEX-247 · Studio image roles: icon, logo, poster (F51)

Studios gain three owner-editable image roles — icon (studios list well), logo (detail page
header), and poster (schema/API/UI shipped now, reserved for a future consumer) — replacing the
single enrichment-only logo cache (ADR-057) with a generalized asset-slot model matching Person's
provenance-locked images (ADR-049, generalized by ADR-079). Done means the migration, the
entity-generic `imagesink` package, the owner-gated CRUD API, the TMDB asset switch, and the
frontend controls are all merged, with every lockstep gate (spec/ADR/design/testing/security)
landed.

**Design package:** [spec](../specs/studio-images.md) · [ADR-079](../architecture/ADR-079-studio-image-roles.md) · [handoff](../design/studio-images-handoff.md) · [testing-strategy §10](../testing-strategy.md)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → [docs/specs/studio-images.md](../specs/studio-images.md)
- [x] architecture `architecture` → [ADR-079](../architecture/ADR-079-studio-image-roles.md) (supersedes ADR-057)
- [x] design `design-handoff` → [docs/design/studio-images-handoff.md](../design/studio-images-handoff.md)
- [x] backend → migration 0036, `internal/imagesink` (entity-generic `enrich.ImageSink`), `internal/api/studio_images.go`, `providers/tmdb` asset switch
- [x] frontend → `StudioImageSlot.svelte`, studios list + detail pages
- [x] testing `testing-strategy` → [docs/testing-strategy.md §10](../testing-strategy.md)
- [x] security `security-review` → clean sign-off (no findings), recorded in the spec's Timeline section

## Up next — ordered (position = priority)

_None — epic complete, merged to `main` via [PR #206](https://github.com/whoiskevinrich/holodex/pull/206) (`1ef23b8`)._

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-04 · Full epic delivered end-to-end and merged
- skills: write-spec, architecture, design-handoff, testing-strategy, security-review
- handoff: Epic complete — PR #206 merged to `main` (`1ef23b8`), Jira already shows Done. Poster
  ships with full CRUD but no consuming view yet, by design (spec RD1). Nothing further planned
  for HOLODEX-247.
