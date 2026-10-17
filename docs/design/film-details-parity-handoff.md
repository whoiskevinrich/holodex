# Handoff Spec: Film Details parity (HOLODEX-95)

**Story:** HOLODEX-95 (epic HOLODEX-20, Unified metadata presentation)
**Spec:** [`docs/specs/film-provider-enrichment-ux.md`](../specs/film-provider-enrichment-ux.md) P0-13
· reference pattern: [studio Details §2c](studio-entity-handoff.md)
**Date:** 2026-10-16 · **Status:** Option A approved by the owner, 2026-10-16

Decided with the owner:
- **The film page's Details section is the studio page's Details section**, and visitors see it.
  The film field set does not change. Frontend only.
- **Rejected: widening the film field set** (runtime, tagline, genres as film fields) and wiring
  promotion, claims and additional-detail rows for films. That design was drawn from TMDB's movie
  payload. The production provider sends only `release_date`, `description`, `actors`,
  `external_provider_id` and an internal source URL for films (owner's probe, 2026-10-16), so none
  of it would ever render. Deferred to
  [HOLODEX-579](https://whoiskevinrich.atlassian.net/browse/HOLODEX-579), to be picked up when a
  production provider sends more.
- **Visitors get Details without the box, on film and studio pages.** For a visitor the section
  is a bare heading plus rows, in the same rhythm as Description and Cast. The owner keeps the
  bordered box, because there it frames controls. Rejected: the box for visitors too. With the
  production norm of one row, it spent about 86px of padding, heading and an empty second column on
  one 20px fact, and it made the smallest fact in the rail its heaviest block (design critique,
  2026-10-16).

## Mockup

![Owner and visitor film pages with release date in Details, plus the mixed-source and empty states](film-details-parity-mockup.svg)

## Layout

Only the rail's Details section changes. Header, Description, Aliases, Tags, Cast, billed-cast
coverage, Full film, Manage and the Scenes grid keep their markup and order.

Details keeps its slot (after billed-cast coverage, before Full film). Its container depends on who
is looking:

- **Owner:** the box, as today — `section space-y-3 rounded-theme border border-rule bg-surface p-4`.
- **Visitor:** no box — `section space-y-2`, with no border, surface or padding. Its heading is the
  rail's section heading (`h2 text-xs uppercase tracking-wide text-muted`, as on Description), with
  the provider note on the same line, right-aligned. It is separated from its neighbours by the
  rail's normal section spacing only.

Inside either one, the same `<dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">` holds the
Details fields, built the studio page's way: compact rows first, then long rows spanning both
columns. Today that is one compact row, **Released**, for almost every film. Unboxed, the empty
second column is plain rail space, not a visible hole.

**The studio page's visitor Details drops its box the same way** (see
[studio Details §2c](studio-entity-handoff.md)), so the two pages stay identical. The studio
owner's box is unchanged.

Name and description stay out of Details; they keep their own header and rail sections.

## Components

| Component | Where | Props / notes |
|---|---|---|
| `SourceBadge` | owner rows | Same call as studio (`baselineKey="record"`); unchanged from today's film Details |
| `ProvenanceBadge` | visitor rows, mixed-source case only | Follows the inline value; omitted when the section note is shown |
| `EnrichProviderChips` / `NoProviderHint` | owner header | Unchanged |
| Visitor header note | visitor header, single provider | `text-xs text-muted` "Enriched from **{Provider}**", provider name `text-accent` — the studio page's markup |

Row markup is the studio page's:
- Owner compact row: `dt.mb-1 text-muted "Label:"` over a `dd` holding the `SourceBadge`.
- Visitor compact row: inline `dt.text-muted`, `dd.inline text-ink`.
- Visitor long row: `dd.mt-1 block leading-relaxed text-ink`.

Not used on films in this change: `CurationFieldRow` (films have no multi-value Details field) and
`AutoFieldRows` (film resolution produces no additional-detail rows). Adding them would render
nothing.

## States and interactions

| Element | State | Behaviour |
|---|---|---|
| Details section | owner | As today: shown when a field has a value or sources are loaded |
| Details section | visitor, at least one value | Shown, unboxed. Previously never shown to visitors |
| Details section | visitor, no values | Hidden (mockup panel 3b) |
| Section note | visitor, every row from one provider | "Enriched from {Provider}" in the header; rows carry no badge |
| Section note | visitor, a row with a custom value or from a second provider | No note; each row carries its `ProvenanceBadge` (panel 3a) |
| Released row | owner, badge click | Source picker expands in place — `SourceBadge`'s existing behaviour |
| Released row | visitor | Read-only value; no badge click, no pencil |

## Responsive behaviour

| Breakpoint | Changes |
|---|---|
| `lg` and up | `stage-grid` two columns; Details sits in the rail |
| below `lg` | Single column; Details follows in rail order |
| 375px | One column inside the `dl`; the header note wraps under the heading if it must |

## Edge cases

- **Only Released** (the production norm): the visitor sees a heading with the note and one row,
  about 38px; the owner sees one row in the box.
- **Film not enriched** (8 of 103 in production): visitor sees no Details; owner sees today's
  owner state.
- **Release date set as a custom value:** no section note; the row's badge says
  where it came from.
- **Loading:** no skeleton; the section appears with the detail payload, as today.

## Animation / motion

None added.

## Accessibility

- `dt`/`dd` pairs keep the list semantic; the visitor note is plain text in the section header.
- Visitor rows have no focus stops. Owner focus order is unchanged.

## Cinémathèque QA checklist

1. [agent] Visitor, enriched film: Details shows Released with "Enriched from {Provider}", no badge, pencil or chip, and no box (computed border width 0, background transparent, no padding).
2. [agent] Visitor, unenriched film: no Details section.
3. [agent] Visitor, release date set as a custom value: no section note; the row carries its badge.
4. [agent] Owner, film and studio: Details unchanged, boxed (same rows, picker, enrich chips).
5. [agent] Visitor, studio page: Details unboxed, the same as on a film.
6. [human] Visitor, film and studio side by side: Details sits in the rail's rhythm, with the same spacing above and below as Description and Cast.
7. [smoke] 375px: no horizontal scroll; the note wraps cleanly.
