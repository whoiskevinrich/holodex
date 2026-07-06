# Spec: Nationality flag on the person page (HOLODEX-139)

**Status**: Implemented
**Date**: 2026-07-05
**Feature**: A small country flag beside a person's name in the hero, derived from the existing
`nationality` field.
**Design handoff**: [`docs/design/people-nationality-flag-handoff.md`](../design/people-nationality-flag-handoff.md)
· QA [`people-nationality-flag-qa-checklist.md`](../design/people-nationality-flag-qa-checklist.md)
**Provider contract**: [`metadata-provider-contract.md`](metadata-provider-contract.md) §4.2 (`nationality`
flag hint)

## Problem

The person hero shows only the name. A nationality flag is a fast, scannable signal of who a
person is. The `nationality` canonical field **already exists** (registry `nationality`, TMDB maps
`place_of_birth` → `nationality`, and it renders as a text row in Details) — so this feature adds a
**flag rendering** over that existing value, not a new field.

## Approach — client-side derivation (no contract/provider/migration change)

The `nationality` value is free text: TMDB supplies a place of birth (`"London, England, United
Kingdom"`); the contract also permits a plain nationality word (`"British"`). The flag is derived in
the SPA at render time:

1. **Value → country.** For a comma-separated place of birth, the country is the **last segment**
   (`"…, United Kingdom"` → `gb`). A single token is matched as a country name or a **demonym**
   (`"British"` → `gb`). Matching folds diacritics/periods and maps common synonyms
   (`USA`/`UK`/`England`/`Türkiye`/`West Germany`/…). Unresolvable values yield **no flag**.
   (`web/src/lib/nationality.ts`, unit-tested in `nationality.test.ts`.)
2. **Country → flag image.** ISO 3166-1 alpha-2 code → a bundled **flag-icons** (MIT) SVG, served
   locally and embedded in the Go binary via `web/dist` — no CDN, works offline
   (`web/src/lib/flags.ts`; Vite `assetsInlineLimit` keeps the flags as on-demand files, not inlined
   into the page chunk).
3. **Render.** `NationalityFlags.svelte` shows the **primary** flag with a muted **"+N"** when more
   than one nationality resolves; the full country list is the `alt`/`title` (tooltip + screen
   reader). It sits beside the name in the hero (`people/[id]/+page.svelte`).

Rationale: the field and its TMDB mapping already ship, so a render-layer derivation works on
already-enriched people with zero contract change, no new migration, and no re-enrich. See the design
handoff for the placement/size/multi/fallback decisions.

## Acceptance

- A person with a known country shows a correctly-mapped flag beside the name, with the country named
  in the tooltip/`aria-label`. ✅ (Austria, USA verified end-to-end.)
- A person with no resolvable country shows **no flag and no layout gap**. ✅
- Multiple nationalities render one primary flag plus a `+N`; the tooltip lists all. ✅
- Tokens only around the flag (imagery aside); QA'd in Cinémathèque, Broadcast, and Brutalist. ✅

## Non-goals / known limits

- **Not a curation surface.** The flag reflects the resolved `nationality` value; it is not separately
  editable. Changing the value (via the existing Details source-decision UI) changes the flag.
- **Place-of-birth ≠ nationality.** TMDB's `place_of_birth` is used as the nationality proxy (the
  field's existing behavior). A person born abroad may show their birth country. Accepted.
- **Rare mis-parse.** A place of birth ending in a token that collides with a country name (e.g. a US
  state "Georgia" with no trailing country) can resolve to the wrong flag. Uncommon in provider data
  (TMDB includes the country); accepted degrade.
