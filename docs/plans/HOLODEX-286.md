---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-286                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: [HOLODEX-280]        # stacked on the still-open PR #255 branch, which is where Film's image code lives
release_note: Internal refactor only — no user-facing behavior change.
---

# HOLODEX-286 · Generalize the entity-image pipeline (Person → Studio → Film)

Studio (F51/ADR-079) and Film (F56/HOLODEX-280/ADR-086) mechanically duplicated Person's
(F25/ADR-038) self-hosted-image pattern a third time — disk path/store/remove, an API
upload/serve handler pair, and an upload/replace/remove Svelte control were each rewritten
per entity. Per the project's "promote to shared code on the third real use" convention
(ADR-079's own closing note anticipated this), this ticket generalizes what's safely
generalizable without forcing a fit where the entities have genuinely diverged. Done means:
`go build`/`go vet`/`go test ./...` clean, frontend `check`/`test` clean, no behavior change
(proven by the pre-existing Studio/Film adversarial test suites passing unchanged, not new
scenarios — this is a structural refactor, not a new capability).

**Scope drawn (see session log for the reasoning):**
- **Disk layer — fully generalized.** New `internal/entityimage` package; `personimage`/
  `studioimage`/`filmimage`'s `ImagePath`/`Store`/`Remove` become thin delegating wrappers,
  exact same exported signatures, zero call-site changes.
- **Go API-handler layer — generalized for all three named entities, split by
  upload vs. serve.** New `internal/api/entity_images.go` (`parseImageUpload`/
  `serveEntityImageFile`). `serveEntityImageFile` is used by all three —
  `servePersonImageFile`, `serveStudioImage`, `serveFilmImage` — since the on-disk-JPEG-
  with-immutable-cache serve tail is byte-identical across Person/Studio/Film with none of
  Person's gallery-specific logic in the way. `parseImageUpload` is used by Studio+Film
  only: Person's `uploadPersonImage` additionally handles the gallery cap, over-cap
  override, and content-hash dedup, a genuinely different shape. Deliberately NOT applied
  to `video_poster.go`/`provider_icon.go` (same low-level multipart/ServeContent pattern,
  but unrelated features outside this ticket's Person→Studio→Film entity-image precedent
  chain — folding them in would be scope creep, not the duplication this ticket names).
- **Frontend — fully generalized.** `StudioImageSlot.svelte`/`FilmImageSlot.svelte` (byte-
  identical apart from prop names) replaced by one `entity/EntityImageSlot.svelte`, generic
  over the role type (Svelte 5 `generics="TRole extends string"`), taking `upload`/`remove`
  as function props. Small `uploadEntityImage` helper extracted in `api.ts` for the
  duplicated FormData-building.
- **NOT touched: the repo/SQL CRUD layer.** Studio's `UNIQUE(studio_id, role)` vs Film's
  `UNIQUE(film_id, role, source)` (ADR-086 — a role can hold both an uploaded AND a
  provider-sourced row, reserved for HOLODEX-284) is a genuine schema-driven behavioral
  difference: Studio's queries correctly omit a `source` filter that Film's require. Forcing
  one shared function would add an optional param used by only one caller, or risk a subtle
  correctness change, for no real duplication payoff.
- **NOT touched: Person's repo/API layer at all.** Materially different feature set (capped
  gallery, content-hash dedup, owner-deletion suppression, promote-to-core, hash-backfill
  migration) — out of scope, low value, real risk.

**Design package:** no new spec/ADR/design — this is a structural refactor of existing,
already-specified behavior (Person F25/ADR-038, Studio F51/ADR-079, Film F56/ADR-086); the
scope reasoning above is this ticket's own record. Testing: no new
`docs/testing-strategy.md` scenarios — the existing Studio §1704 / Film §1947 adversarial
blocks already prove the behavior this refactor must preserve, and they pass unchanged.

## Gates — definition of done

- [x] spec `write-spec` → not needed; no behavior/requirement change
- [x] architecture `architecture` → not needed; no data-model/contract change, see scope note above
- [x] design `design-handoff` → not needed; UI is visually/behaviorally identical, only the component's file/props changed
- [x] backend
- [x] frontend
- [x] testing `testing-strategy` → no new scenarios needed (see Design package); existing Studio/Film suites pass unchanged
- [x] security `security-review` → clean; no new attack surface, same normalize/owner-gating spine reused verbatim, see session log

## Up next — ordered (position = priority)

1. [x] [backend] `internal/entityimage` package + delegate personimage/studioimage/filmimage — `internal/entityimage/entityimage.go`
2. [x] [backend] shared upload-parse helper (Studio+Film) + shared serve-file helper (Person+Studio+Film) — `internal/api/entity_images.go`
3. [x] [frontend] `EntityImageSlot.svelte` + call-site updates, delete old components — `web/src/lib/components/entity/EntityImageSlot.svelte`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-25 · full implementation + simplify + verification
- skills: simplify, security-review
- handoff: HOLODEX-286 is implementation-complete. `/simplify`'s 4-agent pass found and fixed
  two issues: a name collision (`serveImageFile` vs. a pre-existing unrelated method — renamed
  to `serveEntityImageFile`) and duplicate deep test coverage in `filmimage_test.go`/
  `studioimage_test.go` now redundant with `entityimage_test.go` (trimmed both to delegation-
  only checks). The altitude pass then found the API-handler-layer split was slightly
  under-reached: `servePersonImageFile`'s serve tail had no gallery-specific logic and was
  byte-identical to the new shared helper, so it was folded into `serveEntityImageFile` too
  (`parseImageUpload` stays Studio+Film only — Person's upload path has real gallery-cap/
  dedup logic the other two don't). `provider_icon.go`/`video_poster.go` were deliberately
  left alone — same low-level shape, but outside this ticket's named Person→Studio→Film
  entity-image chain; folding those in would be scope creep. `/security-review` re-run
  against the final diff (entityimage package, entity_images.go, all three delegation
  wrappers, EntityImageSlot.svelte) came back clean — no path-traversal/authZ/normalization/
  header regressions, route gating unchanged. `go build`/`vet`/`test ./...` and frontend
  `check`/`test` all clean, zero behavior change. Live browser verification was skipped this
  session — port 5173/7800 were contended by another concurrent Claude session (see PR
  description for the incident note); the change is markup/logic-identical to what was
  already live-verified under HOLODEX-280, so risk is low, but a human should eyeball the
  three skins once before merge if that hasn't happened elsewhere. This branch is stacked on
  the still-open PR #255 (`worktree-HOLODEX-280-film-poster-pipeline`) — its own PR must
  target that branch, not `main`, until #255 merges.
