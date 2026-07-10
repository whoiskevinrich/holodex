# Design handoff: Derived / calculated person fields — Age & Age at death (F45)

**Spec**: [derived-person-fields.md](../specs/derived-person-fields.md) (F45, HOLODEX-73) ·
**ADR**: [ADR-063](../architecture/ADR-063-derived-computed-fields.md) ·
**Epic**: [HOLODEX-18](https://whoiskevinrich.atlassian.net/browse/HOLODEX-18)

This is an **addendum** to the [F37 people source-of-truth handoff](people-source-of-truth-handoff.md) and the
[F39 provider render-hints handoff](provider-render-hints-handoff.md). The person Details section — the `Details`
card, the `dt`/`dd` two-column grid, `SourceSelect`, `ProvenanceBadge`, the compact/long/merge field buckets —
is **inherited unchanged**. This document specifies only what is new for F45: a **read-only derived row** placed
directly under Birthdate, and a **computed treatment on `ProvenanceBadge`**. Everything is **tokens-only**
(no literal palette/radius/font — see [theming.md](theming.md)) and must be QA'd in all three skins.

Derived fields are a **distinct genre**: computed-on-read, source-less, **read-only for owner and visitor
alike** (ADR-063 §D2/D3). They are never adoptable, never curatable — no source chips, no promote pill, no
`SourceSelect`. They are the fact a human reads *off* the data (how old is this person?), shown well, never
edited.

---

## Overview

When a person has an enriched `birthdate`, an **Age** row appears directly beneath **Born**, showing a bare
integer (e.g. `36`) with a small muted **computed provenance icon** whose hover/label reads *"calculated from
Birthdate."* When the person also has a `deathdate`, the running Age is **replaced** by an **Age at death** row
(never both). When `birthdate` is missing or unparseable, **nothing renders** — no placeholder, no "—", no
nudge, for owner and visitor alike (spec D3). The row is identical for owner and visitor: a derived value has
no owner affordances.

---

## Resolved decisions (from spec + this handoff)

| # | Decision | Source |
|---|---|---|
| D1 | Value is a **bare integer** whole years (`36`); the label carries the noun ("Age" / "Age at death"). No months/days, no unit suffix. | spec D1 |
| D2 | The row sits **directly under Birthdate** in the primary bio group — **not** the "Additional details" auto-field block. | spec D2 |
| D3 | Missing/unparseable input → the row is **absent** for everyone. No placeholder, no "—", no enrichment nudge. | spec D3 |
| D4 | `deathdate` present → **Age at death** replaces the running Age. Exactly one of the two ever renders. | spec FR3 |
| **D5** | **Provenance treatment: icon-only** (this handoff, chosen 2026-07-10). A single muted glyph mirroring the ADR-059 provider brand-icon badge; the transitive phrase *"calculated from Birthdate"* lives in `title` + `aria-label`, never inline text. **Not** a provider brand icon, **not** the "file" text pill. | this doc §2 |

---

## 1. Placement & the row itself

The derived row reuses the person page's existing compact `dt`/`dd` grid row — same shape as a visitor's
read-only replace field, minus every control.

```
Details
  Name: Maya Rodriguez            ← canonical / curatable (unchanged)
  Born: 1990-03-14                ← birthdate (unchanged)
  Age: 36  ⨏                      ← NEW derived row, directly under Born; ⨏ = computed icon
  Nationality: American
  Bio: …
```

- **Directly under Birthdate.** The backend positions `age` / `age_at_death` immediately after `birthdate` in
  the `resolved[]` payload (spec FR5). The SPA renders in received order, so adjacency is a **backend
  ordering** guarantee — the SPA adds no client sort.
- **Row markup** matches the visitor compact branch already in
  [`people/[id]/+page.svelte`](../../web/src/routes/people/[id]/+page.svelte):
  `<dt class="inline text-muted">{f.label}:</dt>` + `<dd class="inline text-ink">{f.values[0]}</dd>`, followed
  by the computed `ProvenanceBadge` (§2). Single-column (not `sm:col-span-2`) — it's a compact vital, not prose.
- **Value** is `f.values[0]` verbatim — the backend emits the bare integer string; the SPA does no arithmetic
  and no formatting.

---

## 2. Provenance — computed treatment on `ProvenanceBadge` (D5, chosen)

`ProvenanceBadge.svelte` gains a **third branch**, alongside the existing provider-icon and file-pill branches.
It renders **icon-only**, exactly the shape of the ADR-059 provider brand icon (a 16px inline mark with the
long form on `title`/`aria-label`), but with a **generic "calculated" glyph** instead of a provider logo and
**no** monogram fallback.

**New props:**

```svelte
let {
    provider = '',
    label = '',
    computed = false,            // NEW — render the derived treatment
    derivedFrom = []             // NEW — dependency field LABELS, e.g. ["Birthdate"]
}: { provider?: string; label?: string; computed?: boolean; derivedFrom?: string[] } = $props();
```

- When `computed` is true, render neither the `ProviderIcon` nor the file pill. Instead render a single inline
  **SVG glyph** (a small function/calculator mark — see below), `size 16`, `text-muted`, `ml-2 inline-flex
  align-middle`, with:
  - `title` **and** `aria-label` = **`calculated from {derivedFrom joined}`** — e.g. `"calculated from
    Birthdate"`, or for age-at-death `"calculated from Birthdate and Death date"`. Join with `", "` for 2 and a
    serial-comma "and" for the last (there are at most two today).
- **Glyph**: an inline SVG in the existing convention (cf. the promote arrow in `AutoFieldRows.svelte`),
  `class="h-4 w-4"`, `stroke="currentColor"` / `fill="none"`, `aria-hidden="true"`, so it inherits `text-muted`
  and reacts to every skin. Use a simple **function-curve / ƒ(x)** or **calculator** mark — nothing branded.
  (Do not use a raster or an emoji.)
- **Tone**: `text-muted` only — never `--accent` (which reads as active/selected) and never `--warn`. It must
  read as a quiet, secondary annotation, like the "file" pill's muted tone.

The transitive **labels** (`derivedFrom`) come from the payload, not a client-side registry lookup: the backend
emits each dependency's **registry label** on the derived row (see §4), so the copy stays in sync with whatever
the Birthdate row is actually labeled. The spec's example copy assumes that label is "Birthdate".

> **Why icon-only (D5).** The provider badge already hides its long form ("from tmdb") behind a 16px icon +
> `title` (ADR-059) so the value isn't out-shouted down the column; the computed badge matches that exact
> restraint. Inline text ("calculated from Birthdate" as a visible pill) was considered and rejected — it
> competes with the one-character value ("36") and crowds the compact vitals row. The phrase is preserved for
> hover and screen readers.

---

## 3. The `computed:` winning-source gotcha (must-fix, or the badge breaks)

A derived row carries `winning_source = "computed:age"`. The shared helper
[`providerFromWinningSource`](../../web/src/lib/format.ts) only strips the `record`/`file`/`manual` baseline
namespaces:

```ts
// returns "computed" for "computed:age" — NOT "" — because computed isn't in the baseline set
export function providerFromWinningSource(winningSource?: string): string {
    const ns = (winningSource ?? '').split(':')[0];
    return ns === 'record' || ns === 'file' || ns === 'manual' ? '' : ns;
}
```

Left unchanged, both the person page's `winnerProvider(f)` and `AutoFieldRows`' `provider(f)` would treat a
computed row as **provider-sourced** and render `<ProvenanceBadge provider="computed" />` — which shows a
`ProviderIcon` monogram bubble reading **"C"**. That is the wrong badge.

**Fix (two parts):**

1. **Never route a computed row through the provider-badge path.** The SPA branches on `f.computed` *first*
   (see §5) and passes `computed`/`derivedFrom` to `ProvenanceBadge` — it does **not** compute a `provider`
   for it.
2. **Belt-and-suspenders:** add `computed` to the baseline-namespace guard in `providerFromWinningSource` so a
   `computed:*` winning source can never resolve to a phantom provider name anywhere:
   `ns === 'record' || ns === 'file' || ns === 'manual' || ns === 'computed' ? '' : ns`.

---

## 4. Payload contract (`ResolvedField`)

Add to the TS type in [`web/src/lib/types.ts`](../../web/src/lib/types.ts), mirroring the backend
`resolver.ResolvedField` additions (ADR-063 §D2):

```ts
// F45 (ADR-063) — a computed-on-read, source-less, read-only derived field. Renders a bare
// value + the muted "calculated" provenance icon; carries NO decision/candidates/in_sync and
// is never adoptable/curatable. winning_source is "computed:<canonical>".
computed?: boolean;
// F45 — the human LABELS of the inputs this value was derived from (e.g. ["Birthdate"]),
// for the "calculated from …" provenance copy. Backend-supplied so the SPA needs no registry.
derived_from?: string[];
```

- A computed row: `computed: true`, `winning_source: "computed:age"`, `multi: false`, `auto_registered:
  false`, `values: ["36"]`, `derived_from: ["Birthdate"]`, and **`decision` / `candidates` / `in_sync` all
  absent** (structurally non-adoptable).
- `age` vs `age_at_death` are mutually exclusive in the payload — only one is ever present on a person.

---

## 5. SPA integration — one branch in the compact loop

The derived rows stay **inside** the existing `compactFields` iteration (so backend ordering keeps them under
Birthdate for free), with a new `{#if f.computed}` branch **ahead of** the owner/visitor split so a computed
field never reaches `SourceSelect` or `promotedEdit`:

```svelte
{#each compactFields as f (f.canonical)}
    {#if f.computed}
        <!-- F45: derived read-only row — identical for owner and visitor, no controls. -->
        <div>
            <dt class="inline text-muted">{f.label}:</dt>
            <dd class="inline text-ink">{f.values[0]}</dd>
            <ProvenanceBadge computed derivedFrom={f.derived_from ?? []} />
        </div>
    {:else if isOwner}
        <!-- …existing SourceSelect owner branch, unchanged… -->
    {:else}
        <!-- …existing visitor read-only branch, unchanged… -->
    {/if}
    {#if !f.computed}{@render promotedEdit(f)}{/if}
{/each}
```

- `compactFields` already keeps a computed field (it is `!multi`, `!auto_registered`, and for both owner and
  visitor `values.length > 0`), so **no partition change is required** — only the leading `{#if f.computed}`
  branch and skipping `promotedEdit` for it.
- **Do not** add a computed field to `mergeFields`, `longFields`, or `extraFields` — it is none of those.
- Same one-branch pattern is available to the media/studio pages later, but F45 seeds **person only**.

---

## 6. States

| State | Render |
|---|---|
| Birthdate present, no deathdate | **Age** row (bare integer) directly under Born + muted computed icon. |
| Birthdate **and** deathdate present | **Age at death** row instead; **no** running Age row (exactly one). |
| No birthdate | **Neither** row — for owner **and** visitor. No placeholder, no "—", no nudge (D3). |
| Birthdate unparseable (e.g. `"unknown"`) | Same as "no birthdate" — the row is absent, no error, no partial value. |
| Owner vs visitor | **Identical** — derived rows have no owner controls (no `SourceSelect`, no promote, no chips). |
| Deceased with unparseable birthdate | No Age-at-death row (requires both inputs parseable). |

---

## 7. Copy

- **Field labels** (from the registry, ADR-063 §D1): `Age` and `Age at death`. Sentence case, no trailing
  punctuation beyond the `dt`'s `:`.
- **Provenance label** (`title` + `aria-label`): `calculated from Birthdate` (Age) /
  `calculated from Birthdate and Death date` (Age at death). Lower-case verb, the input names carried as
  the dependency **registry labels** so they track the visible row labels. Serial comma if a future formula has
  three inputs.
- No visible pill text — the phrase is icon-hover / SR only (D5).

---

## 8. Accessibility

- The computed glyph is **not** an interactive control — no tab stop, no button semantics. It is an annotation:
  `aria-hidden="true"` on the inner SVG, and the **wrapping `<span>` carries the `aria-label`** (`calculated
  from Birthdate`), so a screen reader announces the row as e.g. *"Age: 36, calculated from Birthdate."*
- Focus order is unchanged — a derived row adds no focusable elements (contrast the owner curatable rows, which
  have the `SourceSelect` radiogroup).
- The value must never be conveyed by color alone; it is plain `text-ink`.
- Hover/`title` is a progressive enhancement over the `aria-label`, not the only channel for the provenance
  phrase.

---

## 9. Three-skin QA (required)

Render in **Cinémathèque, Broadcast, and Brutalist** (header picker), tokens only
(`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` stays empty):

1. The **Age** row reads correctly against `bg-surface`, value in `text-ink`, label in `text-muted`, sitting
   flush directly under **Born** in all three skins.
2. The computed **glyph** renders in `text-muted` (never `--accent`, never `--warn`), aligned with the value
   baseline (`align-middle`), and does **not** collide with the value or the next row in any skin — the
   badge-vs-value collision class from F36/F39.
3. Hovering the glyph shows **"calculated from Birthdate"**; a screen reader announces it from the wrapping
   span's `aria-label`.
4. A **deceased** person shows **Age at death** and **no** running Age; its glyph title reads "…from Birthdate
   and Death date".
5. A person with **no birthdate** shows **neither** row (owner and visitor) — confirm nothing shifts or leaves
   a gap.
6. The derived row shows **no** owner controls in Admin mode — no `SourceSelect` chips, no promote pill, no
   Custom entry — and is byte-identical between owner and visitor.
7. No phantom **"C"** provider bubble anywhere on the row (the §3 gotcha) in any skin.

See [derived-person-fields-qa-checklist.md](derived-person-fields-qa-checklist.md) for the numbered, tagged QA
items.

---

## 10. What is explicitly not in this handoff

- **Owner controls of any kind.** Derived fields are non-adoptable and non-curatable (ADR-063 §D3) — no
  `SourceSelect`, no promote pill, no manual override. Adding one would contradict the genre.
- **A missing-input enrichment nudge.** Cut in spec D3 — the row is simply absent. No "add a birthdate to
  calculate age" prompt.
- **Sub-year precision / locale formatting** (age in months/days, "36 yrs") — spec non-goal; the value is a
  bare integer.
- **Video / studio derived rows.** The genre is entity-generic, but F45 renders **person** only; a future
  computed field is a backend formula registration + this same one-branch render, not a new design.
- **A new skin token or `[data-theme]` flourish.** F45 is pure token reuse — `text-muted` + an inline SVG.
