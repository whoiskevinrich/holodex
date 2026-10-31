# Duplicates components

The Duplicates review queue (F43 S5, ADR-061): possible-duplicate entity pairs the owner merges
or dismisses.

| File | Purpose |
|---|---|
| `DuplicateComparePanel.svelte` | The F70 compare panel a **person** pair expands into: the match-kind label, then two columns of evidence side by side, then the row's Keep separate as a footer. **Each column is `PersonHoverCard`'s anatomy laid flat** — same fields, same order, same absent-is-absent rule — under a 240 px lead image with a strip of five 44 px `PersonImageFrame` buttons (headshot + up to 4 non-headshot gallery images, no `+N`; no strip for one image) that swap the lead, per side, reset on close. Name, Videos and Films open in a new tab (↗); an alias-less side says "No other names"; the row's `mergeInto` snippet sits at the foot of each column. Composes `GET /people/{id}/card` (through F68's `loadPersonCard` cache, so a person already hovered costs nothing) + `GET /people/{id}/images`; per-side failure is inline with its own Retry and never disables a verdict. |
| `DuplicateStudioPanel.svelte` | The F70 compare panel a **studio** pair expands into (598, P0-10): the person panel's well, grid, column shell and footer over `GET /owner/duplicates/studios/{id}` — per side the name (new tab ↗), its count, **"Name came from"** (file tags or each provider, with video counts), up to 3 lazy posters captioned with the source that won each, "Also known as" with each alias's origin ("you" when it has no source), and the provider badges. Describes provenance only: no verdict word, no precedence control. A session cache (module-level, failures dropped) means a reopened panel issues no request. Renders the row's `verdicts` and `mergeInto`. |
| `DuplicatePairRow.svelte` | One pair row: both entities (name · count), the variation kind, and the verdicts. **Two grammars on `showsComparePanel`:** a tag or film row keeps accent Keep separate + ghost two-step Merge (RD7); a person or studio row offers only ghost Keep separate, its names link to the entity page in a new tab, the whole row toggles the panel (the chevron stays the keyboard control), and it hands the panel a `mergeInto` snippet ("Merge into this one", accent pill) that opens the row's merge `ConfirmDialog` — with the Show as radio for a studio (P0-11). The page above owns which panel is open. |
| `DuplicatesBanner.svelte` | Owner-only "N possible duplicates" notice above an entity list, deep-linking the Owner hub's Duplicates tab. Self-gating and self-fetching. `inline` renders it as a " · N possible duplicates" link inside a list's count line (F73), which is how People and Studios mount it. |
| `VideoPairRow.svelte` | One **video** pair (F76): the shared title once, a per-side `4K · 34:14` summary, "same provider match", and Keep both. **Owns every video verdict and `busy`**: Keep both, the `keepThis` snippet (Keep this one), the keep-one `ConfirmDialog` with its carry-over list, and the `label` call. |
| `VideoComparePanel.svelte` | The panel a video pair expands into: ONE fact table, a column per file, so every difference sits on one row (option B of the signed-off handoff). A row renders when either side has a value; a side without one leaves its cell blank. Owns its fetch and the edition/part editors' UI state only; renders the row's snippets and calls its `label`. |
| `videoPairs.ts` | The Videos group's rules: video pair ids, the focus ladder, `sideSummary`, `factRows` (row presence + media-page formats), `workSummary`/`carryLines`, and `labelError` (the editors' validation copy). Pinned by `videoPairs.test.ts`. |
| `queue.ts` | The queue's rules, in one testable place: the per-pair and per-lane id strings (`pairKey`/`disclosureId`/`panelId`/`laneTabId`/`lanePanelId`/`QUEUE_ID`), the lanes (`LANES` in fixed tab order, `LANE_LABEL`, `parseLane` for `?type=`, `defaultLane` — first lane with pairs, else Videos), `showsComparePanel` (person and studio, RD10), `profileHref`, `sourceLabel` (studio evidence namespaces in words), `matchKindLabel`, `labelPlacement` (a person or studio pair's label goes in the **panel**, tag and film keep it in the **row** — OQ3), and `focusLandingIds` (next disclosure → the lane's tabpanel → the queue). Pinned by `queue.test.ts`. |

## Rules

- **Derive an id through `queue.ts`, never by hand.** The page, the row and the panel all need
  the same per-pair strings; a fourth spelling is how `aria-controls` ends up pointing at a panel
  that never renders and the focus ladder silently misses every rung.
- **Gate the label on entity type, not on "is a panel open".** Only person and studio pairs
  have a panel, so moving the match-kind label into it without the type gate deletes it from
  tag/film rows — and then two unalike names are paired with nothing explaining why
  (`labelPlacement`).
- **The panel never owns a verdict** — the row hands over `verdicts` and `mergeInto` (entity
  pairs) or `verdicts`, `keepThis` and `label` (video pairs). It renders those snippets, which is what makes
  a per-side fetch failure unable to disable a verdict and keeps the collapsed row and the open
  panel from disagreeing about `busy`. `verdictOwnership.test.ts` fails if that stops being true.
