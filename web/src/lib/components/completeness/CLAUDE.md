# Completeness components

Entity Completeness Score UI (F55, ADR-081/082): the facet-first remediation queue and the
per-entity breakdown panel — both consumed across video/person/studio, so they live here rather
than in any single entity-type folder (see the parent `components/CLAUDE.md` classification rule).

| File | Purpose |
|---|---|
| `CompletenessQueueRow.svelte` | One (entity, missing facet) row on `owner/completeness`. Candidate-ready rows show a `ProvenanceBadge` + Apply button that pins the field to the cached candidate's provider; needs-research rows show a Search link (and, for image facets, an Upload link) that navigates to the entity page anchored at that facet's control. |
| `CompletenessPanel.svelte` | Per-entity breakdown panel card on video/person/studio detail pages (F55.13-15) — headline is the required band (`score`, null for a type with no required band → extras is the number) with `extras` beside it, never blended (F65), a bar + actionability line, facets grouped Critical/Nice to have with a status pill per tier (Curated/Provider via `ProvenanceBadge`/Missing/Not applicable), and the video-only not-applicable toggle on the `external_provider_id` row. |
| `CompletenessRing.svelte` | The ring badge (F65.4, HOLODEX-412): `required` fills an accent arc over a `muted` track; once required is 100, `extras` draws as a second lap in ink (the overfill). Props `required`/`extras` (`number \| null`) + `size` `'card'` (14 px) / `'row'` (12 px). Owner-only **by payload, not by prop** — callers mount it iff the item carries `completeness`; the API strips that for visitors. Mounted by `video/VideoCard` (bottom-left chip, before the part badge), `person/PersonPosterCard` and the `/people` + `/studios` rows (trailing, before the count). **F65.8 (HOLODEX-435): a `<button>`** — press = the single-entity sweep step (`api.enrichRefreshAll`, sweep semantics: no picker, no toast) then `api.entityCompleteness` to redraw itself; `entity: { kind, id }` is required, `onrefreshed(bands)` optional. Busy = spinning quarter arc, `aria-busy`, disabled. |
| `ringRefresh.svelte.ts` | Per-entity refresh state for the ring (F65.8): `busy` and the post-refresh bands keyed `kind:id`, so a page that mounts the same entity twice (landing shelves + grid) spins and redraws every ring for it, and a second press cannot double-fire. `currentBands` lets a later list fetch with *different* bands win again; a fetch returning exactly the pre-refresh bands is indistinguishable from "not re-fetched yet" and keeps the refreshed bands — known, accepted. |
| `ring.ts` | `ringReading` / `arc` — the ring's one reading (accent arc, ink overfill gated on required === 100, aria-label, both-null = mount nothing) and the 44-unit dasharray maths, pure so `ring.test.ts` pins the handoff's state table. |

## Rules

- **`facets[].curatable` and `criticality: 'optional'` (F60, HOLODEX-377).** `curatable` is true
  for a plain-text replace field — the shape the media page can hand to an empty `SourceBadge` row
  when a `#field-<canonical>` deep link targets a facet the resolver dropped for being empty and
  undecided; image, long-text and merge fields are never `curatable`. An `optional` facet is
  listed for exactly that reason but is **not** a completeness concern: `CompletenessPanel` groups
  only `critical` / `nice_to_have`, so it never renders one, and the queue API skips it — don't add
  an "Optional" group or count it in "missing".
- **The ring reads `completeness.required` / `.extras` only** (the list item's `CompletenessSummary`).
  Never hand it the detail page's `Completeness` object — that one keeps `score` as the required
  band's key for the panel (ADR-099 D5) and carries `facets`; the two shapes are deliberately
  different so a card can't accidentally depend on a detail payload.
- **The ring is a button, so it is never inside a link or a label (F65.8).** Every mount hoists it
  as a sibling of the card/row `<a>` (or the select-mode `<label>`): `VideoCard` via the
  `.video-frame-overlay` sibling that mirrors the frame's aspect, the poster card and the index rows
  via a stretched link (`after:absolute after:inset-0` on an unpositioned `<a>` inside a `relative`
  wrapper) with the ring at `relative z-[1]`. `document.querySelectorAll('a button.completeness-ring')`
  must stay empty on every surface — it is the live-QA assertion for any new mount.
- **Sweep semantics on a press.** `needs_review` and `rate_limited` are swallowed on purpose; the ring
  returns to idle with the bands it had (`bandsAfterRefresh`) and the entity page is where review
  happens. Do not add a picker, a toast or a confirm here — the ring keeps the F66 sweep's
  unattended contract (ADR-103 D7), one entity at a time.
