# Studio relationship-edit popover (F56.4, HOLODEX-271)

Parent epic: [HOLODEX-267](https://whoiskevinrich.atlassian.net/browse/HOLODEX-267) — Entity
decision editing overhaul. Sibling: [HOLODEX-270](https://whoiskevinrich.atlassian.net/browse/HOLODEX-270)
(composite-key collision check, shipped — this story wires its already-generic collision
machinery into the Studio commit path, per that spec's own forward-compat note).

## Problem Statement

Studio-on-video is a relationship — a link to a Studio entity — not free text, but the only way to
change it today is `SourceSelect`'s radiogroup, which only offers the file- and provider-declared
candidate *values* already resolved for that field (`web/src/routes/media/[id]/+page.svelte:690`,
`web/src/lib/components/curation/SourceSelect.svelte`). That conflates two distinct owner intents:
picking which already-known candidate should win, versus reassigning the video to a *different*
Studio entity entirely — including one no source ever suggested (a typo fix, a studio the file
metadata never mentioned, or consolidating onto a studio's canonical name). There is no search-any-
studio affordance and no way to create a studio inline; an owner who wants either has no path in
the UI today. The cost of not solving this is either a wrong studio staying wrong (no candidate
matches what the owner actually wants) or an owner reaching for a workaround outside the app
(editing file metadata directly, then re-scanning) to get the value they need into the candidate
list first.

## Existing State (grounded in code, this session)

- **No `videos.studio` column.** Studio is a join table, `video_studios`
  (`0017_studios.up.sql:15-19`, whose own comment says "no column on videos" — deliberate).
  `internal/repo/studios.go` `StudiosForVideos(ctx, ids []int64) (map[int64][]model.Studio, error)`
  (:209-232) is the sole reader; `ReconcileVideoStudios(ctx, videoID, names []string,
  extIDByName map[string]string) error` (:43-118) is the sole writer, called from
  `RelinkVideoStudios` (`internal/api/studios.go:244-293`) whenever the resolved `studio` field
  value changes.
- **Today's picker.** `media/[id]/+page.svelte:687-707` — owner view renders
  `<SourceSelect field={studioField} decide={(s, mv) => decideField('studio', s, mv)} />` (:690)
  plus anchor links to each currently-linked studio's detail page. `SourceSelect.svelte` (303
  lines) is a radiogroup of source-tagged chips built from `field.values` (baseline + one per
  distinct provider value) with a "Custom" free-text chip fallback — no search, no list of studios
  beyond what's already resolved for this field.
- **Commit path already generic.** `decideField('studio', source, manualValue?)` →
  `api.setFieldDecision(id, 'studio', {...})` → `PUT /media/{id}/fields/studio/decision` →
  `internal/api/decisions.go` `setFieldDecision` (:41-104), the same endpoint HOLODEX-269/270 use
  for Title. Studio creation is **implicit**, not a separate endpoint: any manual value committed
  through this path flows into `ReconcileVideoStudios` → `resolveOrCreateStudio` →
  `resolveOrCreateByName` (`internal/repo/studios.go:29-31`), which creates the studio row if the
  name doesn't already resolve to one. No `POST /studios` create route exists or is needed.
- **Collision gate already wired for reuse, not yet firing for Studio.** `decisions.go:81-82`
  calls `h.repo.FindTitleCollision` only when `field.Canonical == "title"`; the function's own
  comment (`internal/repo/video_collision.go:27`) says "Studio and People triggers reuse this same
  check once HOLODEX-271/272 land." `FindTitleCollision` already reads the video's *current*
  people+studio links unconditionally (:48-57) and only varies title/date on the candidate filter
  (:59-65) — for Studio, the roles invert: title/date/people stay fixed from the current row, and
  the *proposed* studio is what varies against candidates. `VideoCollision`'s response shape
  (:16-22) and the 409 envelope (`decisions.go:86-93`) are already entity-agnostic; no frontend
  change is needed there — `NameEditControl`'s conflict slot and `CollisionOfferCard` (HOLODEX-270)
  work for any canonical field, not just Title.
- **Reusable search-and-create-fallback pattern already exists**, just not wired to Studio today:
  `web/src/lib/components/entity/EntityPickerDialog.svelte` (248 lines) — built for the Extraction
  tab's People/Studio "Edit…" action — debounces a query (300ms, :67-76) against `GET /search`
  (`api.search(q)`, `internal/api/handlers.go:1166`), and shows an inline **"Use "{query}" as a
  new {kind}"** row (:216-225) when no exact match exists. `onselect(name, existing)` (:19-27)
  only *hands back a name* — it does not decide or commit anything itself, so it composes cleanly
  with `decideField`. `PickerShell.svelte` (101 lines) is the shared dialog chrome (backdrop,
  focus trap, Escape-to-close, entry animation) this and `CategoryPicker.svelte` already build on.
  `CategoryPicker.svelte:72-86` (`createAndAssign()`) is the closest precedent for "search, or
  create inline, then commit" as one flow, though its create step calls a dedicated
  `api.createCategory` endpoint that Studio doesn't need (creation is implicit via `decideField`,
  above).

## Goals

1. An owner can reassign a video's Studio to *any* studio in the library, not just the file/
   provider-declared candidates — closing the gap where the right value exists but was never
   discoverable through today's picker.
2. An owner can create a new studio inline, in the same flow, when the studio they want isn't in
   the library yet — no round-trip through file-metadata editing and a re-scan.
3. Reassigning Studio runs the same composite-key collision check Title already has (HOLODEX-270),
   so a Studio edit that would produce a duplicate-looking video gets the same one-click "view
   existing / save anyway" resolution Title edits already have — zero new silent-duplicate paths.
4. The existing quick-select-a-known-candidate flow stays at least as fast as today's — this is a
   superset of `SourceSelect`'s current capability for this field, not a regression wrapped in an
   extra click.

## Non-Goals

- **A dedicated `POST /studios` create endpoint** — creation stays implicit through
  `decideField('studio', 'manual', name)` → `ReconcileVideoStudios`, exactly as it already works
  for any manual studio value today. The popover's "create" row is a UI affordance over the
  existing commit path, not new backend surface.
- **Replacing `SourceSelect` for other fields** — this story's popover is Studio-specific (behind
  the shared docked-pencil affordance per the epic's two-tier model); Title/Person/Tag renames
  keep using `NameEditControl` (HOLODEX-269), and other resolved fields keep `SourceSelect`.
- **People trigger-point wiring** — HOLODEX-272 owns that; this story only closes the Studio leg
  of HOLODEX-270's three trigger points.
- **Bulk studio reassignment across multiple videos** — this is a single-video, single-field edit
  surface, matching the scope of every other field-decision UI in the app today.
- **Studio merge/dedup UI from inside this popover** — if search surfaces a near-duplicate studio
  name, resolving that duplication is the existing F43/ADR-061 Duplicates system's job, not this
  popover's; the popover only picks or creates, it doesn't merge.

## User Stories

- As an owner, I want to search the full list of studios (not just today's candidate chips) when
  reassigning a video's Studio, so I can pick the correct entity even if no source ever suggested
  it.
- As an owner, I want to create a new studio inline when the one I want doesn't exist yet, so I
  don't have to leave the video page to fix file metadata and re-scan first.
- As an owner, I want the known candidates (today's file/provider values) to still be one click
  away, so reassigning to an already-suggested value doesn't get slower than it is today.
- As an owner, I want to be warned if reassigning Studio would make this video an exact
  composite-key duplicate of another active video, so I get the same protection Title edits
  already have.

## Requirements

### Must-Have (P0)

- **Popover behind the shared docked-pencil affordance**, consistent with the epic's two-tier
  editing model (ADR/spec precedent: HOLODEX-269's `NameEditControl`) — at rest, identical to the
  non-owner view; interaction is opt-in, not always-visible chrome. Acceptance: a non-owner or an
  owner who hasn't interacted sees the same Studio display as today.
- **Known candidates as quick-select options**, preserving today's `SourceSelect` capability for
  this field (file-declared baseline + provider-declared values). Acceptance: selecting a known
  candidate commits in the same number of interactions as today's radiogroup (one click on the
  candidate, no extra confirmation step).
- **Search-any-studio field**, reusing `EntityPickerDialog`'s debounced `GET /search` pattern.
  Acceptance: typing a studio name that exists in the library but was never a candidate for this
  video surfaces it as a selectable result.
- **Inline "create studio '{query}'" fallback**, shown when search finds no exact match — required,
  not optional, per the story's own framing: a picker with no answer for "the studio isn't in the
  list" is a dead end. Acceptance: searching a name with no match shows a create row; selecting it
  commits that name as a new manual studio value (creating the studio row via the existing implicit
  create-on-relink path).
- **Any selection (chip, searched, or created) commits through the existing `decideField('studio',
  ...)` path** — no new endpoint. Acceptance: network trace of any of the three selection paths
  shows the same `PUT /media/{id}/fields/studio/decision` call shape already used today.
- **Studio reassignment triggers the shared collision check** (HOLODEX-270's mechanism, extended
  with a studio-varying sibling query) before committing. On a hit, the same verdict panel
  (`CollisionOfferCard`, HOLODEX-270) renders in the same conflict slot, with the same two choices.
  Acceptance: reassigning Studio into an exact composite-key collision with another active video
  returns 409 and does not persist; "Save anyway" commits with an override exactly as it does for
  Title.

### Nice-to-Have (P1)

- **Recently-used studios surfaced above search results** when the popover opens with an empty
  query, to shortcut the common "reassign to a studio I picked recently" case.
- **Keyboard-only flow** (open popover, arrow through candidates/results, Enter to select) matching
  the roving-tabindex precedent already used elsewhere (`EnrichPicker.svelte`) for consistency
  across the app's picker surfaces.

### Future Considerations (P2)

- Surfacing a lightweight "did you mean the existing studio '{name}'?" nudge inline in the create
  row if the searched name is a near-miss (not exact) match of an existing studio — deferred to the
  existing Duplicates system rather than duplicating near-miss logic here, per this spec's own
  non-goal on merge/dedup.

## Success Metrics

**Leading:** Studio field-decision commits via the search or create path (vs. the known-candidate
chip path), sampled post-launch to confirm the new affordances get used, not just theoretically
available. Studio-trigger collision-panel fire rate, mirroring HOLODEX-270's own leading metric.

**Lagging:** no new duplicate-looking video pairs traceable to a Studio edit, verified by spot
check a month after launch (same cadence as HOLODEX-270's lagging metric, now covering the second
of three trigger points).

## Open Questions

- **Exact popover layout (modal-style `PickerShell` vs. an inline-anchored popover under the
  pencil)** (design, non-blocking for this spec — resolved in the design-handoff pass, grounded in
  the real `PickerShell`/`EntityPickerDialog`/`CategoryPicker` components above).
- **Whether the studio-collision sibling query needs its own repo function
  (`FindStudioCollision`) or `FindTitleCollision` should be generalized to a single
  `FindCompositeKeyCollision(ctx, videoID, proposed{Title,StudioID,...})` call** (engineering,
  non-blocking — resolved during implementation; the research pass above confirms either shape is
  a small, low-risk change over the existing candidate-filter-then-compare structure).
