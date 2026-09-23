# Duplicates pair evidence — expand a person pair into a symmetric compare panel — design handoff

**Ticket:** [HOLODEX-451](https://whoiskevinrich.atlassian.net/browse/HOLODEX-451) · F70 ·
**Status:** Drafted 2026-09-22, amended 2026-09-23 (OQ3 reversed — see "The label moved"),
awaiting the owner's sign-off at `/implement` ·
**Date:** 2026-09-23 ·
**Spec:** [`duplicates-pair-evidence.md`](../specs/duplicates-pair-evidence.md) P0-1…P0-7, RD1–RD12 ·
**ADR:** none — the epic's `architecture` gate is `[~]` n/a (no endpoint, no migration, no
cross-cutting decision). Rides ADR-061 (the queue), F26 (person images), F68 (`/card`), ADR-102
(skin = instance identity).

## Decision

**The column *is* the F68 hover card, laid flat.** Same fields, same order, same
segment-dropping rule, same `ProviderLinkBadge` link row — with a strip of five 44 px image
frames where the card's single 48 px headshot was. That is the concrete answer to the spec's
RD2: the hover card was never the wrong *content*, only the wrong *container* — one at a time,
hover-gated, hidden on touch. F70 keeps the content and changes the container to two of them,
side by side, in the row you are deciding.

![Duplicates pair evidence: the collapsed queue as single-line 40px rows with the verdict emphasis swapped and the weak-signal label moved out, the expanded two-column compare panel opening with that label and carrying an image strip per side, its loading, sparse and error states, the narrow stacked layout, and the same panel in the Brutalist skin](duplicates-pair-evidence-mockup.svg)

| Panel | Surface | Idiom reused |
|---|---|---|
| 1 | collapsed queue rows | today's `DuplicatePairRow` + the `CompletenessPanel` disclosure recipe |
| 2 | the expanded panel | two `PersonHoverCard`-shaped cards on a `bg-surface-2` well; `field-grid` |
| 3 | loading / sparse / error | F68's "the card should feel like it *was* there"; absent-is-absent |
| 4 | narrow / stacked | `field-grid`'s own auto-fit — no breakpoint written by hand |
| 5 | Brutalist | `--radius: 0` — the mockup's only non-Cinémathèque panel, as a QA reminder |

### Open questions closed by this handoff

| # | Question | Answer | Why |
|---|---|---|---|
| OQ2 | `+N` opens a modal in place, or navigates? | **Neither — there is no `+N`.** Five frames are the whole strip. | Kevin, 2026-09-22. The strip is a *sample*, not an index: if five faces don't settle it, the panel has failed and the profile is the next move anyway. Removing `+N` also removes a second dismissable layer competing with Escape inside an expanded row. |
| OQ3 | Keep `via alias` / `alias match only — weak signal`? | **Keep both — but on a person pair they live in the panel, not the row.** The row keeps `· {variation}` only. A non-person pair keeps its label in the row, unchanged. | Kevin, 2026-09-23, reversing the 2026-09-22 answer. See "The label moved" below — at real type sizes the label cost every person row a second line, and 100 % of the queue carries it. They still explain why the *detector* flagged the pair, so they belong with the evidence rather than in a scan line. |
| OQ4 | Is 5 the right strip cap? | **5 frames at 44 px (`w-11`), `gap-1`.** | Measured, not guessed: `5 × 44 + 4 × 4 = 236 px` inside a 296 px column (the `field-grid` 320 px minimum less `p-3` both sides). 4 × 52 px wastes 43 px of column; 6 × 40 px leaves 6 px of slack and drops each face below the 44–48 px the F68 card already proved recognisable. |

### Design calls made here (not in the spec)

1. **The verdict swap is a literal class swap, not a demotion to `btn-quiet`.** RD7 says Merge
   "becomes the quiet action", but `.btn-quiet` is documented in `app.css:300–306` as *"Borderless
   neutral — a UI-only toggle with no side effect (Cancel, Undo)"*, and Merge is the most
   consequential, least reversible action on the page. Merge takes `.btn-ghost` — the class
   Keep separate vacates — so both verdicts stay bordered and equally hit-targetable.
   - **Build note:** `app.css:281–292` and `:313–324` name these two buttons *by name* in their
     doc comments (`btn-ghost` → "Keep separate", `btn-accent` → "Merge"). Those comments become
     wrong in this change and must be updated in the same commit, or the role vocabulary drifts
     for `ExtractionQueueRow` and `EnrichQueueRow`, which share the classes.
2. **The link row keeps F68's `Videos` and `Films` anchors.** They are free (the `#videos` /
   `#films` ids already exist on the profile from F68) and they close what would otherwise be a
   dead end: with `+N` gone and the pair-row names still `<span>`s (P2-1), the panel would have
   had no in-app path to a profile at all. This delivers P2-1's benefit now without making the
   names links.
3. **Each column is its own bordered card; there is no divider between the columns.**
   `--surface-2` differs from `--surface` by roughly 2 % in Cinémathèque (`#181310` vs `#15110e`),
   so a fill change cannot carry the separation — the border does. A per-column border also means
   the stacked layout (panel 4) needs no `border-left` → `border-top` flip, which `field-grid`'s
   auto-fit could not tell us to do anyway.
4. **No summary line, no co-appearance marker, no conflict chip in the footer.** An earlier
   draft put a one-line précis under the columns ("different ages, different countries…"); that
   is exactly the cross-side adjudication RD3/RD4 forbid. The divider carries verdicts and
   nothing else.
5. **The label moved — the row stays one 40px line.** *(2026-09-23, reversing OQ3.)*
   Measured, not eyeballed: at the component's real type sizes (`text-sm` names, `text-xs`
   meta) a person row carrying `· alias match only — weak signal` needs **~836 px** to stay on
   one line. The queue renders far narrower, so `DuplicatePairRow.svelte:77`'s `flex-wrap`
   wrapped it — and since the probe found **100 % of the queue is `alias`**, that second line
   was the *default* row, not an edge case, on a surface whose whole job is scanning 31 of them.
   The first mockup had drawn the label straddling the `Keep separate` pill instead of wrapping,
   which is why this went unnoticed until now.
   - **Where it went:** the panel well's first line, above both columns —
     `<p class="mb-3 text-xs text-warn">` — because it describes *the pair* (why the detector
     fired), not either side, so it cannot sit in a column, and RD3/RD4 keep it out of the
     footer. It frames the evidence rather than trailing it.
   - **Drop the leading `· `.** In the row the middot was a list separator; as a standalone
     line it is a typo. The string itself is unchanged — reuse `matchKindLabel`, don't fork a
     second copy.
   - **Non-person rows keep the label exactly where it is.** The panel is person-only (RD10),
     so a studio/tag/film pair has nowhere to move it to, and `matchKindLabel` at
     `DuplicatePairRow.svelte:84–92` is **not** gated on entity type today — both `alias` and
     `mixed` (`via alias`, `text-muted`) render for every kind. Gate the row's label on
     `pair.entity_type !== 'person'` rather than deleting it, or non-person pairs silently lose
     the only thing explaining why two unalike names are paired.
   - **`flex-wrap` on the inner block must go, or the 40 px is a wish.** With `flex-wrap` the
     browser wraps before it shrinks, so a long name pair re-wraps even with the label gone.
     Drop `flex-wrap` from the inner text block (`:77`) and let `min-w-0` + the existing
     `truncate` on the two name spans absorb the overflow; the `shrink-0` siblings already
     ensure only the names compress. **Keep `flex-wrap` on the row root (`:73`)** — that is the
     genuinely narrow-viewport escape hatch, where the verdict block drops below the text.

## Layout

```
DuplicatePairRow  (div, role="group")                     ← unchanged shell
├── button.disclosure          h-7 w-7   person pairs only
├── div.flex-1                 names · counts · variation   ← NO flex-wrap; names truncate
│                              · match-kind  ONLY when entity_type !== 'person'
└── div.verdicts               [Keep separate] [Merge]

DuplicateComparePanel  (div, id=`dup-panel-{type}-{aId}-{bId}`)   ← sibling, when expanded
├── div  border-t border-rule bg-surface-2 px-3 py-3
│   ├── p  mb-3 text-xs {text-warn|text-muted}   ← match-kind label, when there is one
│   │                                     warn for `alias`, muted for `mixed` — same
│   │                                     conditional the row uses today (:87–88)
│   └── div.field-grid  gap-4
│       ├── section  rounded-theme border border-rule bg-surface p-3    ← side A
│       └── section  rounded-theme border border-rule bg-surface p-3    ← side B
└── div  mt-3 border-t border-rule pt-3  flex justify-end gap-2         ← verdicts, repeated
```

**Why the verdicts repeat.** They stay in the collapsed row *and* appear in the panel footer.
The row's pair are the one-click path for the common answer (spec Goal 2); the footer's are where
your eye lands after reading the evidence. Both call the same handlers; `busy` is shared, so
pressing either disables all four.

### The column, top to bottom — F68's order exactly

| # | Element | Markup | Rule |
|---|---|---|---|
| 1 | Image strip | `<div class="flex gap-1">` of `PersonImageFrame` | 5 frames, `frameClass="portrait-frame--1x1 w-11"`. Slot 1 = `role="headshot"` with `version={card.headshot_version}`; slots 2–5 = `gallery` filtered to `img.role !== 'headshot'`, in `sort_order`. No `+N`. |
| 2 | Name + flags | `<span class="flex items-center gap-1.5">` → `<span class="skin-title truncate font-display text-sm font-semibold">` + `NationalityFlags values={card.nationality}` | `display_name ?? name ?? pair.X.name`. Flags render nothing at all when none resolve. |
| 3 | Meta line | `<span class="block text-xs text-muted">{meta.join(' · ')}</span>`, only `{#if meta.length}` | Copy `PersonHoverCard.svelte:34–42` verbatim: `age` xor `†age_at_death`; `videoCount(video_count)` unconditional; `N film(s)` only when `film_count > 0`. **Absent segments are never pushed**, so the ` · ` separator can never orphan. |
| 4 | Aliases | `<span class="block truncate text-xs italic text-muted">also credited as {aliases.join(', ')}</span>` | `card.aliases?.slice(0, 3)`; omitted entirely when empty. |
| 5 | Link row | `<div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-rule pt-2 text-xs">` | `Videos` → `/people/{id}#videos` (always) · `Films` → `/people/{id}#films` (only when `film_count > 0`) · one `ProviderLinkBadge` per `sortExternalLinks(card.external_links)`. |

**Not rendered in P0:** `CompletenessRing`. `card.completeness` *will* be in the payload (this is
an owner surface), and "I can't decide — go fetch more" is a genuinely apt action here. It is held
to **P1** because it adds a second interactive control per column competing with the verdicts, and
F65.8's sweep-with-redraw has never run inside a list row. Just don't render it.

### The disclosure

Copy `CompletenessPanel.svelte:79–99` exactly — it and `ExpandableText.svelte` are the house
pattern, confirmed a third time at `media/[id]/+page.svelte:2041`.

```svelte
<button
  type="button"
  onclick={() => (expanded = !expanded)}
  aria-expanded={expanded}
  aria-controls={panelId}
  aria-label={expanded ? `Hide evidence for ${a} and ${b}` : `Compare ${a} and ${b}`}
  title={expanded ? 'Hide evidence' : 'Compare'}
  class="btn-quiet flex h-7 w-7 shrink-0 items-center justify-center rounded-theme hover:bg-surface-2"
>
  <svg class="h-4 w-4 transition-transform duration-200 motion-reduce:transition-none"
       class:rotate-180={expanded} viewBox="0 0 24 24" fill="none"
       stroke="currentColor" stroke-width="2" aria-hidden="true">
    <path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6" />
  </svg>
</button>
```

`panelId` must be unique per row. Use the `{#each}` key's own source:
`` `dup-panel-${pair.entity_type}-${pair.a.id}-${pair.b.id}` ``.

**Rendered only when `pair.entity_type === 'person'`** (RD10) — and this is also the
`ExpandableText` rule (`:5–7`) applied: *"a control that cannot change what you see is one the
reader learns to distrust."* A studio, tag or film row has no panel, so it gets no chevron and
keeps today's left edge.

## Design tokens used

| Token / utility | Where | Note |
|---|---|---|
| `bg-surface` | section shell, both column cards | |
| `bg-surface-2` | the panel well, disclosure `hover:` | ~2 % from `--surface` in Cinémathèque — decorative only, never load-bearing |
| `border-rule` | every hairline: row `border-t`, card borders, in-card dividers | |
| `rounded-theme` | card corners, disclosure square | `2px` Cinémathèque, **`0px` Broadcast and Brutalist** — never assume a visible corner |
| `text-ink` / `font-display` / `skin-title` | names | |
| `text-muted` | counts, meta, aliases, provider badge text, chevron | |
| `text-accent` / `border-accent` | `Videos` / `Films` links; the Keep separate pill | |
| `text-warn` | the `alias match only — weak signal` label — **the panel well's first line** on a person pair, still the row on any other kind; per-side fetch error | |
| `.btn-row .btn-pill .btn-accent` | **Keep separate** (was Merge) | outlined accent, `rounded-full` |
| `.btn-row .btn-ghost px-2` | **Merge** (was Keep separate) | bordered neutral |
| `.btn-row .btn-pill .btn-accent` | the two survivor buttons, step 2 | unchanged — in that step Keep separate isn't on screen, so accent is unambiguous |
| `.btn-row .btn-quiet` | `Cancel` in step 2; the disclosure | unchanged |
| `.portrait-frame .portrait-frame--1x1 w-11` | every strip frame | `object-position: center 28%` face-bias comes free |
| `field-grid` | the two-column grid | `repeat(auto-fit, minmax(min(320px,100%),1fr))` — **use the utility, not a hand-written `grid-cols-[…]`**; the theming rule forbids the latter |
| `gap-1` / `gap-4` / `p-3` / `mt-2 pt-2` | strip gap / column gap / card padding / in-card divider | matches `PersonHoverCard`'s own spacing |

Nothing hardcoded. The mockup's literal hexes are the Cinémathèque tokens at `app.css:107–123`
and the Brutalist tokens at `:145–161`, so it renders faithfully on GitHub without the webfonts.

## Data

Zero new endpoints (RD8). Per side, on first expand:

| Call | Supplies | Note |
|---|---|---|
| `loadPersonCard(id)` — `person/personCard.svelte.ts` | name, `headshot_version`, counts, age, nationality, aliases, `external_links` | **Use the lease wrapper, not `api.personCard` directly** — it shares the F68 hover card's session cache, so a person you already hovered costs nothing. One request per person; failures are not cached. |
| `api.getPersonImages(id)` | `{ roles, gallery }` for the strip | `PersonImage` carries `role`, so the headshot is excluded from the tail by `img.role !== 'headshot'` — the ids are not otherwise comparable. |

Image URLs: `api.personImageURL(id, 'headshot', { version, skin })` for slot 1,
`api.personGalleryImageURL(id, imageId, { version, skin })` for slots 2–5 — both already inside
`PersonImageFrame`, which `$derived`s on `theme.current` so the strip re-themes live on a skin flip.

**Hard rule from `components/person/CLAUDE.md`, repeated here because it is easy to get wrong:**
*"Card facts come from `GET /people/{id}/card` only — never assemble age/nationality client-side,
and never feed the card the detail read."* `api.getPerson` ships up to 500 videos; it is not an
option.

> **Build question to settle in code, not here.** `loadPersonCard` returns a `CardLease`
> (`{ promise, release }`). P0-5 requires that reopening a panel issues no second request. Check
> whether `release()` evicts the entry or merely decrements a refcount — if it evicts, hold the
> lease for the page's lifetime rather than releasing on collapse, and say so in the component's
> header comment.

## States and interactions

| Element | State | Behaviour |
|---|---|---|
| Disclosure | default | chevron down, `text-muted` |
| Disclosure | hover / focus | `bg-surface-2`; `.btn-quiet` adds `text-ink` + underline on the (absent) label — the square is icon-only, so this reads as the background change alone |
| Disclosure | expanded | chevron `rotate-180`, 200 ms, `motion-reduce:transition-none` |
| Panel | opening | **no height animation.** `{#if expanded}` toggles it in, matching `CompletenessPanel`. The chevron rotation is the only motion. |
| Column | loading (`card === null`) | five **empty** `<span class="portrait-frame portrait-frame--1x1 w-11" aria-hidden="true">` frames + the name from `pair.a.name` / `pair.b.name`, which the row already has. No spinner, no skeleton bars. Meta, aliases and link row are simply absent until data lands. |
| Column | sparse | absent facts leave no trace — no age segment, no flag, no alias line, no `Films` link, no badges. Never `—`, never "unknown". |
| Column | fetch failed | `<p class="text-xs text-warn" role="alert">Couldn't load this side.</p>` + a `btn-row btn-quiet` **Retry** inside that column only. The other column still renders; **both verdicts stay enabled**; the failure is not cached, so Retry re-requests. |
| Verdicts | `busy` | all four buttons disabled via the existing `busy` state. `.btn-accent` drops to `border-rule`/`text-muted` and `.btn-ghost` drops its border — **do not add `disabled:opacity-60`**, which lands at 2.4–2.9:1 on `text-muted` (theming rule). |
| Verdicts | Merge pressed | unchanged two-step: `Keep:` + both full names as `btn-pill btn-accent` + `Cancel`. If the panel is open it **stays** open — the survivor choice is exactly when you want the evidence on screen. |
| Row | resolved | the page's `resolve()` removes it from `pairs`. |

### Keyboard and focus

| Key | Result |
|---|---|
| `Enter` / `Space` on the disclosure | toggles — native `<button>` behaviour, nothing to write |
| `Escape` while a panel is open | collapses it; focus returns to **its** disclosure |
| `Tab` from the disclosure | into the panel in DOM order: side A's `Videos` → `Films` → badges → side B's → footer Keep separate → footer Merge → next row's disclosure |
| another disclosure pressed | the open panel collapses first (RD12) — a module-level `openId`, same shape as `PersonHoverCard`'s single-open state |

**Do not use `use:dismissable` here.** It closes on outside click, which on this page would
collapse your panel the moment you reach for another row's verdict. The panel is in-flow, not
floating: bind a plain `onkeydown` for `Escape` on the panel container and nothing else.

**Two things the page does not have today and this change introduces.** Both are new work in
`routes/owner/duplicates/+page.svelte`, not just in the components:

1. **The first keyboard handling on the page.** There is currently none — no `onkeydown`, no
   focus management, buttons are the only tab stops.
2. **Focus after a resolve.** `resolve()` is `pairs = pairs.filter(p => p !== pair)`, an instant
   array removal — focus would fall to `<body>`. Before removing, move focus to the next row's
   disclosure, or to the group heading when it was the last row in the group.

> **Spec correction.** Spec P0-6 says *"when the row fades out"* and `DuplicatePairRow.svelte:7`
> says *"the row fades out"* — **there is no fade.** `+page.svelte:47–49` is an unanimated filter
> and always has been. This handoff keeps the instant removal (a transition is scope this epic
> didn't ask for) and the stale comment at `DuplicatePairRow.svelte:7` should be corrected in the
> same commit.

## Responsive behaviour

| Width | Layout |
|---|---|
| Column ≥ 320 px each (the common desktop case) | two columns, `field-grid` auto-fit |
| Below that | one column: side A above side B, each keeping its own border. No breakpoint is written by hand and no divider orientation flips — this falls out of `field-grid`. |
| Row text | **one 40 px line, always** — the label is gone and the inner block no longer wraps, so a long pair truncates its names instead of growing the row. Measured at real type sizes: the widest sample row ends at `x=489` against a verdict pill starting at `x=540`. Only at genuinely narrow viewports does the row root's `flex-wrap` drop the verdict block below the text |

The strip does **not** reflow — five 44 px frames fit the 320 px minimum with 60 px to spare.

## Edge cases

- **No images at all on a side** — render exactly **one** frame (the backend always serves a real
  or themed placeholder for `role="headshot"`, so `PersonImageFrame` never shows a broken glyph),
  not five empty wells. The column keeps its shape without pretending to evidence it doesn't have.
- **Fewer than 5 images** — render what exists. Three frames is three frames; no filler.
- **A side with no `external_links`** — the link row still renders (`Videos` is unconditional),
  just without badges.
- **A provider with no URL** — `ProviderLinkBadge` already degrades to a non-interactive `<span>`
  labelled `Known to {label}` (ADR-083 D2). Nothing extra to do.
- **Long names** — `truncate` on the name line inside a `min-w-0` column, as F68 does. The full
  name is in the row above and in the Merge survivor buttons, which never truncate.
- **Both sides are the same person id** — impossible; the detector pairs distinct ids.
- **A pair resolved in another tab** — out of scope; the queue is a single-owner surface and the
  read is on page load.

## Accessibility

- Disclosure: `<button type="button">` with `aria-expanded`, `aria-controls={panelId}`, a
  state-dependent `aria-label` and a plain `title`. **Never nest it in another control** — the row
  root is a `<div role="group">`, so there is nothing to nest inside (P0-1's third criterion).
- Panel: `<div id={panelId} role="group" aria-label={`Compare ${a} and ${b}`}>`. **Not**
  `role="dialog"`, **not** `aria-modal` — it is in-flow supplementary content, and the rest of the
  queue stays reachable behind it.
- Each column: `<section aria-label={shownName}>` so a screen reader can tell the two apart
  without counting.
- Strip images: the first frame carries the person's name as `alt` (via `PersonImageFrame`'s
  default); frames 2–5 pass `alt=""` — they are the same person, and five identical announcements
  is noise. This is the documented use of that prop.
- Per-side error: `role="alert"`, so the failure is announced without moving focus.
- The chevron is `aria-hidden`; the button's `aria-label` carries the meaning.
- Contrast: every pairing here is an existing token pairing already QA'd in the three skins. The
  one new one is `text-accent` on `bg-surface` for the Keep-separate pill, which is the existing
  `.btn-accent` treatment moved to a different button — same colours, same surface.

## Three-skin QA

Per `.claude/rules/frontend-theming.md`, QA Cinémathèque, Broadcast and Brutalist **plus the
custom palette if one is configured**. Skin is set at Owner › Appearance (ADR-102); there is no
header picker, so switching means changing the instance setting.

What actually differs, and what to look for:

1. **`--radius: 0` in Broadcast and Brutalist** (mockup panel 5). Column cards, image frames and
   the disclosure square all go square. The Keep-separate pill stays round — `rounded-full` is
   literal and sanctioned.
2. **Broadcast's scanline `::after` on `.portrait-frame`** (`app.css:553`) now lands on five
   frames per column, ten per panel. Check it doesn't read as a moiré at 44 px.
3. **`--surface-2` vs `--surface`** differs by more in Broadcast/Brutalist than in Cinémathèque —
   confirm the well doesn't start reading as a raised card instead of a recess.
4. **`text-warn`** on the weak-signal label — now against `--surface-2` (the panel well) on a
   person pair, and still against `--surface` (the row) on any other kind. **Check both**: it is
   a token pairing the row never had, and `--surface-2` differs from `--surface` by more in
   Broadcast and Brutalist than in Cinémathèque.

Greps that must stay empty for this change:
`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` ·
`rg 'text-muted[^"]*disabled:opacity' web/src --glob '*.svelte'`

## Build checklist

- [ ] `web/src/lib/components/duplicates/DuplicateComparePanel.svelte` — new
- [ ] `web/src/lib/components/duplicates/CLAUDE.md` — **add the row for it in the same commit**
      (the parent `components/CLAUDE.md` mandates this)
- [ ] `DuplicatePairRow.svelte` — disclosure (person only), verdict class swap, panel mount,
      fix the stale "fades out" comment at line 7, **drop `flex-wrap` from the inner text block
      (`:77`) but keep it on the row root (`:73`)**, and **gate the match-kind label (`:84–92`)
      on `pair.entity_type !== 'person'`** — a person pair's label is rendered by the panel
      instead. Export `matchKindLabel` (or lift it) so the panel reuses the same strings.
- [ ] `routes/owner/duplicates/+page.svelte` — single-open state, Escape, focus-after-resolve
- [ ] `app.css:281–292` / `:313–324` — update the `.btn-ghost` / `.btn-accent` doc comments that
      name Keep separate and Merge
- [ ] `/testing-strategy`, `/security-review` (owner-gated surface — re-confirm the existing gate
      covers `/people/{id}/card` and `/people/{id}/images` reached from this page),
      `/code-review high --fix`

## Spec deltas this handoff introduced — **applied 2026-09-22 (approved by the owner)**

| Spec | Was | Now |
|---|---|---|
| P0-3 | "capped at 5 visible with a `+N` affordance that links to the person's gallery" | 5 visible, **no `+N`** (OQ2). Its two `+N` acceptance criteria are replaced by: *given a person with more than 5 images, 5 render and nothing indicates there are more*; *given a person with no images, exactly one placeholder frame renders*. |
| P0-4 / RD7 | Merge "becomes the quiet action" | Merge takes `.btn-ghost`, not `.btn-quiet` — `btn-quiet` is reserved for no-side-effect controls |
| P0-6 | "when the row fades out" | "when the row is removed" — there is no fade and never was |
| UI | reuses `PersonImageFrame`, `NationalityFlags`, `ProviderLinkBadge` | …and the `Videos` / `Films` profile anchors from F68 |

All four are now in `docs/specs/duplicates-pair-evidence.md`, along with OQ2/OQ3/OQ4 struck
through as closed and RD7/RD12 reworded. The spec and this handoff no longer disagree.

### Second amendment — 2026-09-23 (approved by the owner)

| Spec | Was | Now |
|---|---|---|
| OQ3 / RD10 | the match-kind label sits in the collapsed row for every pair kind | on a **person** pair it moves to the compare panel's first line; every other kind keeps it in the row. The row is one 40 px line at every desktop width |
| P0-1 | (silent on row height) | new acceptance criterion: *given the widest person pair in the queue, the collapsed row renders on a single line with the verdicts on that same line* |

**Why this is not a return to design.** The 2026-09-22 answer to OQ3 was given against a mockup
that drew the label fitting on one line. It does not fit — the mockup had drawn it overlapping
the `Keep separate` pill rather than wrapping, so the cost of keeping it was invisible when the
question was asked. This amendment prices it correctly and the owner re-answered.
