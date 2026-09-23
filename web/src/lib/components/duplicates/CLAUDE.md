# Duplicates components

The Duplicates review queue (F43 S5, ADR-061): possible-duplicate entity pairs the owner merges
or dismisses.

| File | Purpose |
|---|---|
| `DuplicateComparePanel.svelte` | The F70 compare panel a **person** pair expands into: the match-kind label, then two columns of evidence side by side, then the row's own verdict snippet repeated as a footer. **Each column is `PersonHoverCard`'s anatomy laid flat** — same fields, same order, same absent-is-absent rule — with a strip of five 44 px `PersonImageFrame`s (headshot + up to 4 non-headshot gallery images, no `+N`) where the card's single 48 px headshot was. Composes `GET /people/{id}/card` (through F68's `loadPersonCard` cache, so a person already hovered costs nothing) + `GET /people/{id}/images`; per-side failure is inline with its own Retry and never disables a verdict. |
| `DuplicatePairRow.svelte` | One pair row: both entities (name · count), the variation kind, and the Keep-separate / Merge verdicts. A person pair also carries the disclosure that opens `DuplicateComparePanel`; the page above owns which panel is open. |
| `DuplicatesBanner.svelte` | Owner-only "N possible duplicates" notice above an entity list, deep-linking the Owner hub's Duplicates tab. Self-gating and self-fetching. |
| `queue.ts` | The queue's rules, in one testable place: the per-pair id strings (`pairKey`/`disclosureId`/`panelId`/`groupId`/`QUEUE_ID`), `showsComparePanel` (person only, RD10), `matchKindLabel`, `labelPlacement` (a person pair's label goes in the **panel**, every other kind keeps it in the **row** — OQ3), and `focusLandingIds` (the three-rung ladder focus takes when a row is removed). Pinned by `queue.test.ts`. |

## Rules

- **Derive an id through `queue.ts`, never by hand.** The page, the row and the panel all need
  the same per-pair strings; a fourth spelling is how `aria-controls` ends up pointing at a panel
  that never renders and the focus ladder silently misses every rung.
- **Gate the label on entity type, not on "is a panel open".** The panel is person-only, so
  moving the match-kind label into it without the type gate deletes it from studio/tag/film rows
  — and then two unalike names are paired with nothing explaining why (`labelPlacement`).
- **The panel never owns a verdict.** It repeats the row's `verdicts` snippet, which is what makes
  a per-side fetch failure unable to disable a verdict and keeps the collapsed row and the open
  panel from disagreeing about `busy`. `verdictOwnership.test.ts` fails if that stops being true.
