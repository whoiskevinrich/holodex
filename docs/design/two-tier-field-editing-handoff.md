# Design Handoff: Two-Tier Field Editing Model (F56)

**Spec**: [Two-Tier Field Editing Model (F56)](../specs/two-tier-field-editing.md) · **Issue**: [HOLODEX-268](https://whoiskevinrich.atlassian.net/browse/HOLODEX-268)
**Supersedes** (for the fields in scope): [Per-field source-of-truth decisions (F36) handoff](field-source-of-truth-handoff.md) — the always-on chip radiogroup this spec replaces with a collapsed badge + explicit Confirm. F36's underlying decision model, API, and RD1–RD5 rules are **unchanged**; only the presentation for non-Tier-1 replace fields changes.
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

---

## Overview

Every Tier-2 replace field (any scalar field with 2+ candidate sources, excluding Title/People/
Studio on Video and name on Person/Studio — see spec §Non-Goals) drops the always-on `SourceSelect`
chip radiogroup. In its place:

- **At rest**, the field renders **exactly** what a visitor sees today: the resolved value + a
  `ProvenanceBadge`. The owner gets a click affordance layered on the *same* badge element — no
  extra chrome, no permanent radiogroup.
- **On click** (owner only), the badge expands the field in place into a `CurationChip` radio-mode
  chip row with explicit **Confirm** / **Cancel** actions. Clicking a chip **stages** a selection;
  nothing commits until Confirm.
- **Confirm** calls the existing F36 decision API (`PUT`/`DELETE .../fields/{canonical}/decision`)
  — no API change. **Cancel**, Escape, or clicking outside discards the staged selection and
  collapses back to the badge with zero network calls.

This is the structural fix for the RD6 pending-chip bug ([field-source-of-truth-handoff.md
§States and Interactions](field-source-of-truth-handoff.md#states-and-interactions), row "decided
adopt-provider" vs the un-fixable pending case): there is no longer a chip whose click both *looks*
like a selection and silently does nothing, because no chip click commits by itself anymore.

### Design-system fit (the `/design-system` check)

**One new component, zero new primitives.** `SourceBadge.svelte` (new, `curation/`) wraps
`ProvenanceBadge` for the at-rest state and `CurationChip` (existing `radio` mode) for the expanded
state — the same chip shell F36 already ships, just gated behind a click instead of always-rendered.
No new color, no new radius, no new interaction primitive beyond a plain expand/collapse + two
buttons (`.btn-accent` reused for Confirm, `.btn-ghost` for Cancel — see `app.css`).

`SourceSelect.svelte` is **not extended** to support staging — its whole design is "select = commit
immediately" (roving radiogroup, debounced arrow-key commit), and retrofitting a staged mode onto it
would risk reintroducing exactly the ambiguity this spec removes. `SourceBadge` builds its own small
staged-selection state directly against `CurationChip` + `f36.ts`'s `sourceChips`/`resolveSelection`,
independent of `SourceSelect`. `SourceSelect` stays alive **only** for Person's `onadopt`-intercepted
name field (Tier-1, out of scope here — HOLODEX-269 retires it for that last caller).

---

## Layout

Two existing `dd` shapes host Tier-2 fields today (see `people/[id]/+page.svelte:572-649` for the
canonical pattern, reused verbatim by Video/Studio) — `SourceBadge` replaces the owner branch of
`compactFields` (short-inline field) and the owner-`<dd class="block">` beneath a `longtextFields`
value; the **visitor** branch of both is untouched.

**Compact field, at rest (both visitor and owner look identical):**

```
Runtime:  118 min [●]
                    ↑ ProvenanceBadge (provider brand icon) or a muted "file" pill.
                      Owner-only: cursor pointer + hover/focus ring appear ON HOVER ONLY —
                      nothing renders differently until the pointer arrives (Resolved Decision below).
```

**Compact field, expanded (owner clicked the badge):**

```
Runtime:  118 min [●]
          [● 118 min ·tmdb, pending]  [○ — ·file]        [Confirm]  [Cancel]
           ↑ the RD6-pending case: dashed ring + hollow dot (existing CurationChip `pending` styling,
             unchanged from F36) — Confirm here is what actually creates the standing decision.
```

**Long-text field (Tagline-shaped), at rest and expanded — same pattern, chip row appears on its own
line beneath the value, matching F36's existing `<dd class="block">` placement:**

```
Tagline:  A story about waiting. [●]
          [● A story about waiting. ·file]  [○ Every second counts. ·tmdb]   [Confirm]  [Cancel]
```

**Single-source field (nothing to decide) — no badge at all, owner or visitor:**

```
Original language:  English
```

- **The chip row** on expand is the identical `CurationChip` radio-mode row F36 already renders
  (same tokens, same fold/dedup rules, same `—` em-dash empty-baseline chip, same `·provenance`
  suffix) — only the surrounding shell (badge → expand → Confirm/Cancel) is new.
- **Custom** stays available as the trailing opener chip, unchanged inline-edit idiom (Enter
  commits the *staged* value — still gated behind Confirm — Escape cancels, blur commits into the
  staged slot).
- **Only one field expanded at a time** (P1, F56.9): expanding a second field's badge collapses any
  other currently-expanded field, discarding its staged (uncommitted) selection.

---

## The `SourceBadge` component

Props mirror `SourceSelect`'s entity-agnostic shape so Video/Person/Studio call sites are
interchangeable:

```ts
{
  field: ResolvedField;
  decide: (source: DecisionSource, manualValue?: string) => Promise<void>;
  baselineKey?: string; // 'file' (video) | 'record' (person/studio)
}
```

No `onadopt` — `SourceBadge` never intercepts into a rename/collision flow (that's Tier-1's
mechanism, HOLODEX-269/270/271); every Tier-2 Confirm always calls `decide` directly.

| State | Renders |
|---|---|
| Single candidate source | Value only — no badge, not interactive (owner or visitor identical). |
| 2+ sources, collapsed | `ProvenanceBadge` for the resolved winner; owner gets `cursor: pointer` + hover/focus ring on that same badge. |
| 2+ sources, expanded | `CurationChip` radio row (reusing `sourceChips`/`resolveSelection` from `f36.ts`) + Confirm/Cancel. Selecting a chip updates *local* staged state only. |
| RD6 pending, collapsed | Badge renders for the pending provider exactly as `ProvenanceBadge` does today (no different visual — the pending-ness is invisible until expanded, matching current F36 behavior). |
| RD6 pending, expanded | The pending chip renders with its existing dashed-ring/hollow-dot `pending` treatment (`CurationChip.svelte:83-107`, unchanged). Confirm commits it — **this is the bug fix.** |
| Confirming | Confirm button shows the existing busy treatment (`opacity-60`/`aria-busy`, reused from `SourceSelect`); on success, collapses to the new badge state. On failure, stays expanded with the staged selection intact + inline error (reuse `SourceSelect`'s `toMessage`/error-paragraph pattern). |
| Cancel / Escape / click-away | Collapses immediately, no request, staged selection discarded, badge unchanged. |

---

## Design Tokens Used

| Token | Usage |
|---|---|
| `bg-surface-2` | chip background (unchanged from F36); badge hover/focus background |
| `border-rule` | idle chip border; badge idle (no visible border at rest — see Resolved Decision) |
| `border-accent` / `text-accent` | selected chip; badge hover/focus ring; Confirm button (`.btn-accent`) |
| `text-ink` | selected chip value, resolved value text |
| `text-muted` | field label, idle chip value, Cancel button (`.btn-ghost`/`.btn-quiet`) |
| `text-warn` / `border-warn` | out-of-sync pill (unchanged, still renders independent of expand state) |
| `rounded-full` | chips, badge, buttons (intentional shape) |

No `zinc-/sky-/emerald-/amber-`, no hex, no fixed `rounded-lg/md/sm/xl`, no named fonts, no new
animation system (reuse the existing `transition` token-color fade).

---

## States and Interactions

| Element | State | Behavior |
|---|---|---|
| `SourceBadge` | at rest, 2+ sources | Renders `ProvenanceBadge` exactly as the visitor sees it. Owner: `cursor: pointer`; hover/focus reveals a faint accent ring (Resolved Decision — hover-only, not persistent). |
| `SourceBadge` | at rest, 1 source | No badge; plain value, non-interactive, identical to visitor. |
| `SourceBadge` | click (owner) | Expands in place: renders the `CurationChip` radio row + Confirm/Cancel beneath (compact fields) or on the existing chip-row line (long-text fields). |
| chip row | chip click | Stages that chip's source locally — **no** `decide()` call, no `PUT`/`DELETE`. Visual selection updates (accent border + dot) exactly like `SourceSelect`'s selected-chip styling. |
| chip row | RD6-pending chip staged, unchanged | Dashed-ring/hollow-dot `pending` chip (unchanged `CurationChip` styling) — this is the case F56.4 fixes: Confirm here **does** commit. |
| Confirm | click | Calls `decide(stagedSource, manualValue?)`; busy state (`opacity-60`/`aria-busy`) on the row; on success collapses to badge with the new resolved value/provenance; on failure stays expanded with an inline `text-warn` error line, staged selection intact for retry. |
| Cancel | click / Escape / click-away | Collapses immediately; no request; staged selection discarded; field's resolved value is unchanged (still whatever `decide` last committed, or the resolver default). |
| Custom chip | staged (expanded) | Opens the existing inline input; committing (Enter/blur) sets the staged value to `manual` + the literal — **still requires Confirm** to actually decide. Escape/empty cancels the input only, returns focus to the Custom chip, expanded state persists. |
| Out-of-sync pill | any | Unchanged — renders next to the resolved value at rest (not gated behind expand), exactly as F36 does today. |
| Second field expands | while field A is expanded | Field A collapses first, discarding A's staged (uncommitted) selection (F56.9). |
| Visitor / non-owner | any | `SourceBadge` never renders interactively; field is `ProvenanceBadge` + value, byte-identical to today. |

---

## Responsive Behavior

| Breakpoint | Changes |
|---|---|
| ≥ `sm` (two-column `dl`) | Expanded chip row `flex flex-wrap` wraps within the field's grid cell, same as F36 today. |
| < `sm` (single column) | Unchanged; chips wrap freely; Confirm/Cancel wrap to their own line if the row is tight. |

No new breakpoint. The `dl` grid itself is untouched.

---

## Edge Cases

- **RD6 pending field with no baseline value at all** (empty file/record, one provider) — badge
  still renders (there is a decision worth surfacing), chip row on expand shows the file/record `—`
  chip plus the pending provider chip; Confirm on the pending chip is the bug-fix path (F56.4).
- **Field with exactly one candidate value across all sources** (they all agree) — folds to one
  chip on expand exactly as F36 does today (`·file + tmdb`); still counts as "2+ sources" for badge
  purposes since more than one source contributes, even though there's only one *value* to confirm.
- **Confirm fails (network/API error)** — stays expanded, staged selection preserved so the owner
  can retry without re-selecting; error text reuses `SourceSelect`'s existing `toMessage` pattern.
- **Owner navigates away / component unmounts mid-expand** — no special handling needed; nothing
  was committed, so an abandoned expand is a no-op by construction.
- **Long candidate values** — same `max-w-[14rem] truncate` clamp as F36's existing chips, full
  value on `title`/`aria-label`.
- **Provider match cleared after a decision was made** (existing F31 edge case) — unchanged
  fallback to the baseline chip; harmless stored decision, same as F36 today.

---

## Animation / Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| badge | hover/focus | border/ring token-color transition | ~150ms | default `transition` |
| expand/collapse | click / Confirm / Cancel | none required — chip row appears/disappears in place, no slide/fade (avoids layout-shift jank in a dense `dl`) | — | — |
| chip select (staged) | click | color/border token transition, reused from F36 | ~150ms | default `transition` |
| Confirm button | submitting | reuse existing `opacity-60`/`aria-busy` treatment (no new spinner) | — | — |

No animation on expand/collapse itself — the chip row's appearance is instant, consistent with the
project's "no layout-shifting animation" rule from the F36 handoff.

---

## Accessibility Notes

- `SourceBadge`'s at-rest badge is a `<button>` for the owner (visitor gets the existing
  `ProvenanceBadge`'s plain `<span>`) — `aria-expanded` reflects collapsed/expanded state,
  `aria-label` names the field + "click to change source" affordance.
- The expanded chip row reuses `role="radiogroup"` + roving-tabindex `role="radio"` chips exactly as
  `SourceSelect` does today (`CurationChip.svelte:89-112`) — Left/Right/Up/Down rove focus and
  *stage* a selection (no debounced auto-commit, since nothing commits without Confirm).
  Space/Enter on a chip stages it; Space/Enter on **Confirm**/**Cancel** activates that button.
- Escape anywhere in the expanded row collapses (same as clicking Cancel) and returns focus to the
  badge button.
- Focus-visible ring (`focus-visible:ring-accent`) on the badge, each chip, Confirm, and Cancel.
- Colour is never the only signal — selection is dot + `aria-checked` + border, matching F36
  unchanged; the RD6-pending state keeps its dashed-ring (shape), not colour alone.

---

## Resolved Decision — badge discoverability

Two options were mocked up and compared (hover-only reveal vs. a persistent subtle accent ring) —
**hover-only reveal** is the direction: the badge is byte-for-byte identical to the visitor's
`ProvenanceBadge` at rest; the interactive affordance (cursor + ring) appears only on hover/focus.
This keeps the owner/visitor visual parity from spec requirement F56.1 as literal as possible, at
the cost of slightly lower discoverability than a persistent indicator — acceptable since the owner
already knows these are their own curation fields.

## Open Question carried to implementation

- **[engineering, non-blocking]** Whether `SourceSelect.svelte` is deleted in this story or kept
  alive solely for Person's `onadopt`-intercepted name field until HOLODEX-269 replaces it too — see
  spec §Open Questions. This handoff assumes it stays, scoped down to that one remaining caller.
