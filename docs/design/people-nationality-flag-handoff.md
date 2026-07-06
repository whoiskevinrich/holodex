# Design Handoff: Nationality flag beside the person name (HOLODEX-139)

**Status**: Implemented (developer handoff)
**Date**: 2026-07-05
**Spec**: [`docs/specs/people-nationality-flag.md`](../specs/people-nationality-flag.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[`theming.md`](theming.md) — **tokens only, QA all three skins**

A small country flag sits to the **right of the person's name** in the hero, derived from the existing
`nationality` value. The flag itself is imagery; every piece of chrome around it uses semantic tokens.

## Placement & measurements

**Files:** [`web/src/lib/components/NationalityFlags.svelte`](../../web/src/lib/components/NationalityFlags.svelte),
[`web/src/routes/people/[id]/+page.svelte`](../../web/src/routes/people/[id]/+page.svelte) (hero name row).

| Property | Value | Token / source |
|---|---|---|
| Position | Inline, right of the `<h1>` name, vertically centered | `flex items-center gap-2` row wrapping the name + flag |
| Name truncation | The name keeps `truncate` + `min-w-0`; the flag is `shrink-0` so a long name ellipsizes **before** the flag is pushed out | — |
| Flag height | `h-4` (16px), width auto (~21px at the SVG's 4:3) | `h-4 w-auto` |
| Flag corner | `rounded-theme` (2px Cinémathèque, 0 Broadcast/Brutalist) | `--radius` |
| Flag border | 1px hairline so a white flag reads on the surface | `border border-rule` (`--rule`) |
| Multi count | Muted `+N` after the primary flag when >1 country resolves | `text-xs text-muted` (`--muted`) |
| Gap flag↔count | 4px | `gap-1` |

## States

- **Known country** → primary flag; `alt`/`title` = country name (tooltip + screen reader).
- **Multiple** → primary (first) flag + muted `+N`; `alt`/`title` lists all countries, comma-separated.
  The `+N` is `aria-hidden` (the img alt already carries the full list).
- **Unknown / no data** → the component renders **nothing** (no `<span>`, no `<img>`); the name row has
  a single child so `gap-2` produces no visual gap. No broken-image placeholder.

## Theming notes (what bites these surfaces)

- **Tokens only.** The only literals are the flag image and its `width`/`height` sizing attributes
  (imagery, not styling). Border, radius, and the `+N` color are all tokens, so they react to the skin.
- **Broadcast & Brutalist** set `--radius: 0` → the flag is square; **Cinémathèque** rounds it 2px.
- The `--rule` hairline keeps a light flag (e.g. Japan) from bleeding into `--surface`. Verified the
  border color tracks each skin's `--rule` (Cinémathèque `#2a2622`, Broadcast `#1a2240`, Brutalist
  `#333333`).

## Derivation (see the spec for detail)

`nationality` free text → country → flag, entirely client-side:
`web/src/lib/nationality.ts` (value→ISO country, unit-tested) + `web/src/lib/flags.ts` (ISO→bundled
flag-icons SVG). Flags are self-hosted (MIT), bundled by Vite as on-demand files (not inlined), so they
load offline and ship inside the Go binary.
