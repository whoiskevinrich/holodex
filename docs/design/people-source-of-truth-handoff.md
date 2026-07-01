# Design Handoff: People on the unified source-of-truth model (F37)

**Spec**: [People source-of-truth (F37)](../specs/people-source-of-truth.md) · **Jira**: HOLODEX-10
**Addendum to**: [F36 handoff](field-source-of-truth-handoff.md) — the chip radiogroup, fold/dedup,
tokens, a11y semantics, and motion rules there apply **unchanged**; this document specifies only what
differs on the person page. **QA**: [people-source-of-truth-qa-checklist.md](people-source-of-truth-qa-checklist.md).
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

---

## Overview

The person detail page (`/people/[id]`) Details section moves from a raw enrichment list to the F36
chip vocabulary: each **replace** field (bio, born, died, nationality, website) renders the
`SourceSelect` radiogroup — the person's **record** baseline anchored first (tagged `·record`), one
chip per distinct provider value, a trailing Custom chip. The **aliases** enrichment field becomes a
**merge** field on `CurationFieldRow` (✕-chips). Three deliberate absences vs. the media page:

- **No "Write decisions to file" button and no out-of-sync pill** — a person has no file; decisions
  are display-truth only (`in_sync` is absent from person resolved fields, so `outOfSync()` is never
  true and the pill simply doesn't render — no special-casing needed).
- **No decision chips on Name** — the name row uses chips as a *rename* affordance (RD1, below),
  never a standing pin.
- **The baseline provenance is `·record`**, not `·file`.

## Entity-generic baseline label (dev notes)

The person payload uses `record` as the baseline source key (`candidates[].source`,
`decision.source`, and the `PUT …/decision` body). Two shared modules currently hardcode `file`:

- **`web/src/lib/f36.ts`** — `decidedSource()` default, `fileCandidateValue()`, and the anchored
  first chip in `sourceChips()` all use the `'file'` literal. Generalize with a `baselineKey`
  parameter (default `'file'` so the media page is untouched); `selectedChipKey()` follows.
- **`CurationChip.svelte`** — `isProvider` treats any source ≠ `file`/`manual` as a provider
  (accent). Add `record` to the muted baseline set. The `·provenance` suffix needs no change — it
  prints whatever the sources array carries (`·record`, `·record + tmdb` when folded).

`SourceSelect.svelte` itself needs **no change** — it is already entity-agnostic (`field` + `decide`).

## Layout

Same `dl` grid idiom as the media Metadata card. Owner view:

```
Jennifer Lawrence                                    12 videos
┌ DETAILS ──────────────────────────────  [Enrich ▾] [Clear tmdb] ┐
│ Name:     [● Jennifer Lawrence ·record] [○ J. S. Lawrence ·tmdb] [＋ Custom]
│ Bio:      Jennifer Shrader Lawrence is an American actress …(full prose)
│           [○ — ·record] [● (truncated…) ·tmdb] [＋ Custom]
│ Born:     [○ — ·record] [● 1990-08-15 ·tmdb] [＋ Custom]
│ Website:  [○ — ·record] [● example.com ·tmdb] [＋ Custom]
│ Also known as:  [ J Law ·tmdb ✕ ] [ + Add ]          ← merge field (CurationFieldRow)
└──────────────────────────────────────────────────────────────────┘
Aliases (search & scan routing)  ← the F23 section, UNCHANGED, below Details
```

- **Field order**: Name first, then registry order (bio, born, died, nationality, website), the
  provider-aliases merge row last within Details.
- **Empty baseline (RD3)**: every enrichment-only field anchors a selectable `[— ·record]` chip
  first. Selecting it pins the field **blank** (a standing `record` decision) — the field then
  renders the em-dash as its value until cleared or re-decided.
- **Undecided default (RD6)**: with no decision, the selected chip is the provider value (record is
  empty) — display identical to today's enrichment list.
- **A provider with no value for a field** contributes no chip; a field with no value **and** no
  candidates doesn't render (same as media).
- **Visitor / non-owner**: read-only resolved values (prose/inline, `ProvenanceBadge`-free chips not
  needed — render exactly like today's read-only rows, driven by `resolved[]`).

### Long-text fields (`bio`) — P1-1 resolved

For fields whose registry display is `long_text`, the **resolved value renders as full prose in the
`dd`** (readable, wraps), and the chip radiogroup sits **beneath** it as the source selector. Chips
keep the `max-w-[14rem] truncate` clamp — they are pickers here, not the reading surface. All other
replace fields keep the media-page treatment (the selected chip *is* the value).

### Two alias systems, kept visually apart (RD2)

- The Details merge row is labeled **"Also known as"** (display label for the person `aliases`
  canonical field) — provider-sourced, curatable (✕ suppress / + Add manual), **display-only**:
  kept chips never route scans or search.
- The **F23 section keeps its own card** ("Aliases", with the add input + delete + merge entry
  point) below Details, unchanged. Its explanatory copy already frames it as routing
  ("searching either name finds this person").
- Do **not** interleave them; the separation *is* the design. A per-chip "promote to routing alias"
  affordance is deliberately deferred (spec P2-1).

## The Name row (RD1 — materialize, never pin)

The name renders the same chip radiogroup — record chip (live `people.name`) anchored, one chip per
distinct provider name, `＋ Custom` — but selecting a **non-record** chip does *not* persist a
decision. It opens a **confirm dialog**; on confirm the SPA calls `POST /people/{id}/rename` and
refetches. The record chip is always the selected one at rest (there is never a standing name
decision to render).

**Confirm dialog** (reuse the F23 merge-confirm modal idiom — `bg-surface border border-rule
rounded-theme p-4`, overlay, focus-trapped):

> **Rename to "Jennifer Shrader Lawrence"?**
> "Jennifer Lawrence" is kept as an alias — search and future scans still match it.
> `[Rename]` (accent: `bg-accent text-accent-ink`) `[Cancel]` (ghost)

- Escape / Cancel closes and **returns focus to the chip that opened it** (`await tick()` — same
  a11y rule as the Custom input; cf. `EnrichPicker` focus handling).
- The Custom chip opens the existing inline input; **committing a custom name opens this same
  confirm dialog** (with the typed name) instead of issuing a decision.
- Busy state: dialog buttons disabled + `aria-busy` during the request; no page spinner.

**Name-collision (409) state** — the API returns the existing person; the dialog swaps to the F23
merge offer, never auto-merging:

> **"Jennifer Shrader Lawrence" already exists** (14 videos).
> Renaming would collide with that person. You can merge this person (3 videos) into them instead —
> videos combine and "Jennifer Lawrence" becomes an alias.
> `[Merge into "Jennifer Shrader Lawrence"…]` (opens the existing F23 merge confirmation)
> `[Keep separate]` (ghost, closes)

**After a successful rename**: the record chip carries the new name; a provider chip remains only if
its value still differs; the F23 section now lists the old name as an alias (refetch covers all
three).

## States and Interactions (delta to F36)

| Element | State | Behavior |
|---|---|---|
| any replace row | undecided | provider-value chip selected when record is empty (RD6); `[— ·record]` idle |
| any replace row | decided `record` (blank-pin) | `— ·record` chip selected; field value renders `—` |
| bio row | any | full prose above, chip radiogroup beneath (long_text treatment) |
| name row | at rest | record chip selected; never a standing decision |
| name row | non-record chip activated | confirm dialog opens; **no** PUT/DELETE decision call fires |
| rename dialog | confirm | `POST /people/{id}/rename` → refetch; focus to the name row |
| rename dialog | 409 collision | swaps to the merge-offer state (above); merge routes to the existing F23 flow |
| rename dialog | escape / cancel | closes, focus returns to the opening chip |
| Also-known-as row | owner | merge chips: ✕ = F30 suppress, `+ Add` = F30 manual add (`nowrite` toggle hidden — no writeback for persons) |
| whole Details card | visitor | read-only resolved values; no radiogroups, no dialogs |
| out-of-sync pill / Write button | — | **never rendered** on person pages |

## Design tokens

No new tokens. The dialog reuses the modal treatment (`bg-surface`, `border-rule`, `rounded-theme`);
buttons: accent primary (`bg-accent text-accent-ink`) for Rename / Merge, ghost for Cancel / Keep
separate. `text-warn` appears **nowhere** on this page (no sync state; the collision dialog is
informational, not an error). Quick check stays green:
`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`.

## Accessibility

- Radiogroup semantics unchanged from F36 (roving tabindex, arrow debounce, `aria-checked`).
- Chip aria-labels read the person vocabulary: `Jennifer Lawrence, from record`,
  `no value, from record` (the `—` chip), `1990-08-15, from tmdb`.
- The rename dialog: `role="dialog"` + `aria-modal="true"`, labelled by its heading, focus trapped,
  Escape cancels, focus returns to the opener ([[feedback-keyboard-list-roving-tabindex]] modal rule).
- The name radiogroup announces its consequence: group `aria-label="Name — selecting a source renames
  this person"` so SR users aren't surprised by the dialog.
