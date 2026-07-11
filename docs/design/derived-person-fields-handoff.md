# Design handoff: Derived / calculated person fields — Age & Age at death (F45)

**Spec**: [derived-person-fields.md](../specs/derived-person-fields.md) (F45, HOLODEX-73) ·
**ADR**: [ADR-063](../architecture/ADR-063-derived-computed-fields.md) ·
**Epic**: [HOLODEX-18](https://whoiskevinrich.atlassian.net/browse/HOLODEX-18)

This is an **addendum** to the [F37 people source-of-truth handoff](people-source-of-truth-handoff.md) and the
[F39 provider render-hints handoff](provider-render-hints-handoff.md). The person Details section — the `Details`
card, the `dt`/`dd` two-column grid, `SourceSelect`, `ProvenanceBadge`, the compact/long/merge field buckets —
is **inherited unchanged**. This document specifies only what is new for F45: a **read-only derived row** placed
directly under Birthdate, whose provenance is a **hover tooltip on the value itself** (no icon/badge).
Everything is **tokens-only** (no literal palette/radius/font — see [theming.md](theming.md)).

> **D5 revised (2026-07-10, owner):** the earlier icon-only computed badge is **cut**. A derived value carries
> **no visible provenance mark**; the "calculated from …" phrase is surfaced only as a **hover tooltip
> (`title`) on the data point**, with an `aria-label` on the value for screen readers. Sections below are
> updated to match; the badge-treatment history is retained for context but is **not** the shipped behavior.

Derived fields are a **distinct genre**: computed-on-read, source-less, **read-only for owner and visitor
alike** (ADR-063 §D2/D3). They are never adoptable, never curatable — no source chips, no promote pill, no
`SourceSelect`. They are the fact a human reads *off* the data (how old is this person?), shown well, never
edited.

---

## Overview

When a person has an enriched `birthdate`, an **Age** row appears directly beneath **Born**, showing a bare
integer (e.g. `36`). Hovering the value shows a **tooltip** reading *"calculated from Birthdate"* — there is
**no icon or badge** next to it. When the person also has a `deathdate`, the running Age is **replaced** by an
**Age at death** row (never both). When `birthdate` is missing or unparseable, **nothing renders** — no
placeholder, no "—", no nudge, for owner and visitor alike (spec D3). The row is identical for owner and
visitor: a derived value has no owner affordances.

---

## Resolved decisions (from spec + this handoff)

| # | Decision | Source |
|---|---|---|
| D1 | Value is a **bare integer** whole years (`36`); the label carries the noun ("Age" / "Age at death"). No months/days, no unit suffix. | spec D1 |
| D2 | The row sits **directly under Birthdate** in the primary bio group — **not** the "Additional details" auto-field block. | spec D2 |
| D3 | Missing/unparseable input → the row is **absent** for everyone. No placeholder, no "—", no enrichment nudge. | spec D3 |
| D4 | `deathdate` present → **Age at death** replaces the running Age. Exactly one of the two ever renders. | spec FR3 |
| **D5** | **Provenance treatment: tooltip-only, no icon** (revised 2026-07-10, owner). The value carries the transitive phrase *"calculated from Birthdate"* as a hover `title`, plus an `aria-label` for screen readers. **No** icon, badge, provider brand icon, or "file" pill on the row. *(Supersedes the earlier "icon-only glyph" choice.)* | this doc §2 |

---

## 1. Placement & the row itself

The derived row reuses the person page's existing compact `dt`/`dd` grid row — same shape as a visitor's
read-only replace field, minus every control.

```
Details
  Name: Maya Rodriguez            ← canonical / curatable (unchanged)
  Born: 1990-03-14                ← birthdate (unchanged)
  Age: 36                         ← NEW derived row, directly under Born; hover "36" → tooltip
  Nationality: American
  Bio: …
```

- **Directly under Birthdate.** The backend positions `age` / `age_at_death` immediately after `birthdate` in
  the `resolved[]` payload (spec FR5). The SPA renders in received order, so adjacency is a **backend
  ordering** guarantee — the SPA adds no client sort.
- **Row markup** matches the visitor compact branch already in
  [`people/[id]/+page.svelte`](../../web/src/routes/people/[id]/+page.svelte):
  `<dt class="inline text-muted">{f.label}:</dt>` + `<dd class="inline text-ink">{f.values[0]}</dd>` — with the
  provenance tooltip on the `dd` (§2). Single-column (not `sm:col-span-2`) — it's a compact vital, not prose.
- **Value** is `f.values[0]` verbatim — the backend emits the bare integer string; the SPA does no arithmetic
  and no formatting.

---

## 2. Provenance — tooltip on the value (D5 revised, shipped)

There is **no badge or icon** on a derived row. The "calculated from …" phrase is surfaced as a **hover
tooltip on the value itself** (`title` on the `dd`), plus an `aria-label` restating value + provenance for
screen readers. `ProvenanceBadge` is **not** used for computed rows (it keeps only its provider-icon and
file-pill branches).

- The phrase is built by the shared [`calculatedFrom(labels)`](../../web/src/lib/format.ts) helper:
  `["Born"]` → `"calculated from Born"`, `["Born","Died"]` → `"calculated from Born and Died"` (serial "and"
  for the last of 3+).
- The transitive **labels** come from the payload's `derived_from` (each dependency's **registry label**, see
  §4), not a client lookup — so the copy tracks whatever the Birthdate row is actually labeled (today "Born").
- The value stays plain `text-ink` — no tone change, no color. Provenance is a progressive-disclosure
  affordance (hover / SR), never a visible mark competing with the one- or two-character value.

```svelte
{@const provenance = calculatedFrom(f.derived_from ?? [])}
<dd class="inline text-ink" title={provenance} aria-label={`${f.values[0]}, ${provenance}`}>{f.values[0]}</dd>
```

> **Why tooltip-only (D5 revised).** The icon-only badge (prior choice) still put a mark in the vitals column
> next to a one-character value. The owner cut it entirely: the derived value should read like any other vital,
> with the "it's calculated" fact available on demand (hover / screen reader) but never claiming visual space.

---

## 3. The `computed:` winning-source gotcha (belt-and-suspenders)

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
   (see §5); the computed branch renders no `ProvenanceBadge` at all, so it never computes a `provider` for a
   derived row.
2. **Belt-and-suspenders:** add `computed` to the baseline-namespace guard in `providerFromWinningSource` so a
   `computed:*` winning source can never resolve to a phantom provider name anywhere:
   `ns === 'record' || ns === 'file' || ns === 'manual' || ns === 'computed' ? '' : ns`.

---

## 4. Payload contract (`ResolvedField`)

Add to the TS type in [`web/src/lib/types.ts`](../../web/src/lib/types.ts), mirroring the backend
`resolver.ResolvedField` additions (ADR-063 §D2):

```ts
// F45 (ADR-063) — a computed-on-read, source-less, read-only derived field. Renders a bare
// value with a "calculated from …" hover tooltip (no icon); carries NO decision/candidates/
// in_sync and is never adoptable/curatable. winning_source is "computed:<canonical>".
computed?: boolean;
// F45 — the human LABELS of the inputs this value was derived from (e.g. ["Born"]),
// for the "calculated from …" provenance copy. Backend-supplied so the SPA needs no registry.
derived_from?: string[];
```

- A computed row: `computed: true`, `winning_source: "computed:age"`, `multi: false`, `auto_registered:
  false`, `values: ["36"]`, `derived_from: ["Born"]`, and **`decision` / `candidates` / `in_sync` all
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
        {@const provenance = calculatedFrom(f.derived_from ?? [])}
        <div>
            <dt class="inline text-muted">{f.label}:</dt>
            <dd class="inline text-ink" title={provenance} aria-label={`${f.values[0]}, ${provenance}`}>
                {f.values[0]}
            </dd>
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
| Birthdate present, no deathdate | **Age** row (bare integer) directly under Born; value hover-tooltip "calculated from Born". |
| Birthdate **and** deathdate present | **Age at death** row instead; **no** running Age row (exactly one). |
| No birthdate | **Neither** row — for owner **and** visitor. No placeholder, no "—", no nudge (D3). |
| Birthdate unparseable (e.g. `"unknown"`) | Same as "no birthdate" — the row is absent, no error, no partial value. |
| Owner vs visitor | **Identical** — derived rows have no owner controls (no `SourceSelect`, no promote, no chips). |
| Deceased with unparseable birthdate | No Age-at-death row (requires both inputs parseable). |

---

## 7. Copy

- **Field labels** (from the registry, ADR-063 §D1): `Age` and `Age at death`. Sentence case, no trailing
  punctuation beyond the `dt`'s `:`.
- **Provenance tooltip** (`title` on the value + `aria-label`): `calculated from Born` (Age) /
  `calculated from Born and Died` (Age at death) — the dependency **registry labels** (today "Born"/"Died"), so
  the copy tracks the visible row labels. Serial comma if a future formula has three inputs. Built by
  `calculatedFrom()`.
- No visible text beyond the value — the phrase is hover-tooltip / SR only (D5 revised).

---

## 8. Accessibility

- The provenance carries **no interactive control** and adds **no focusable element** — it is an annotation on
  the value: `title` for the sighted hover tooltip and `aria-label` = `"{value}, {phrase}"` on the `dd`, so a
  screen reader announces the row as e.g. *"Age: 36, calculated from Born."*
- Focus order is unchanged — a derived row adds no focusable elements (contrast the owner curatable rows, which
  have the `SourceSelect` radiogroup).
- The value must never be conveyed by color alone; it is plain `text-ink`.
- Hover/`title` is a progressive enhancement over the `aria-label`, not the only channel for the provenance
  phrase.

---

## 9. QA

The row has no skin-dependent styling (no icon/badge — just a plain `text-ink` value), so the three-skin
matrix is trivial; still keep the token discipline
(`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` stays empty):

1. The **Age** row reads correctly, value in `text-ink`, label in `text-muted`, sitting flush directly under
   **Born**. No icon, badge, or extra mark next to the value.
2. **Hovering the value** ("36") shows the tooltip **"calculated from Born"**; a screen reader announces the
   row as *"Age: 36, calculated from Born."*
3. A **deceased** person shows **Age at death** and **no** running Age; its value tooltip reads "calculated
   from Born and Died".
4. A person with **no birthdate** shows **neither** row (owner and visitor) — confirm nothing shifts or leaves
   a gap.
5. The derived row shows **no** owner controls in Admin mode — no `SourceSelect` chips, no promote pill, no
   Custom entry — and is byte-identical between owner and visitor.
6. No phantom **"C"** provider bubble anywhere on the row (the §3 gotcha).

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
- **A new skin token or `[data-theme]` flourish.** F45 adds no styling at all — the value is a plain
  `text-ink` vital; provenance lives in `title`/`aria-label`.
- **A visible provenance mark** (icon, badge, pill, or inline text). Cut in D5-revised — provenance is
  hover-tooltip / SR only.
