# Spec: Candidate thumbnail in the resolve picker (F64)

**Status**: Draft
**Phase**: Phase 3 follow-up (enrichment quality)
**Owner**: Project owner
**Date**: 2026-09-17
**Feature block**: **F64** — a `/resolve` candidate may carry `image_url`, a small thumbnail the
picker renders in a fixed 2:3 box at the left of every row so the owner can tell same-named
candidates apart at a glance. Hot-linked from an allowlisted provider host through the render-time
gate ADR-056 already established for `image_url` field hints; never fetched, never stored.

**Issue**: [HOLODEX-406](https://whoiskevinrich.atlassian.net/browse/HOLODEX-406)
**ADR**: none — additive optional response key under [§2.3](metadata-provider-contract.md#23-post-resolve--identity-match-disambiguation)'s
"Holodex ignores unknown response keys" (the `profile_url` / `detail` posture), and the browser-side
image gate is [ADR-056](../architecture/ADR-056-provider-field-render-hints.md)'s `render: image_url`
rule applied to one more surface. No new decision, no new perimeter.
**Contract amendment**: [metadata-provider-contract.md](metadata-provider-contract.md) §2.3
(response example + field row), §5 (caps row), §6 (S7 note), §8 (example) — amended in the same
change as this spec.
**Design handoff**: [candidates-image-handoff.md](../design/candidates-image-handoff.md) +
[candidates-image-mockup.svg](../design/candidates-image-mockup.svg) +
[candidates-image-qa-checklist.md](../design/candidates-image-qa-checklist.md).
**Supersedes**: the F61 non-goal "an inline thumbnail / `thumbnail_url` on candidates"
([candidates-detail.md](candidates-detail.md) Non-Goals, P2-c). F61 set it aside because it "would
be the first candidate-level field Holodex has to *fetch* through the SSRF perimeter". This spec
does not fetch: the thumb is *rendered* through the existing allowlist gate, exactly like an
`image_url` field hint. The perimeter cost F61 priced in does not apply.

**Depends on** (all shipped):

- F22 / [ADR-033](../architecture/ADR-033-metadata-source-plugins.md) — the `/resolve` contract and
  the shared `EnrichPicker.svelte`, mounted by the person, media, film and studio detail pages.
- [ADR-039](../architecture/ADR-039-provider-asset-urls.md) — `asset_hosts` allowlist
  (`{base_url host} ∪ operator-listed hosts`), the single `checkHost` gate.
- [ADR-056](../architecture/ADR-056-provider-field-render-hints.md) — `render: image_url` renders an
  `<img>` to an allowlisted host; a value on any other host degrades to text. `Service.ImageURLAllowed`
  is the check.
- F61 — `candidates[].detail` (the row's expand/collapse behaviour the thumb must coexist with).

## Problem Statement

When a `/resolve` returns several candidates with the same label — homonym people ("Chris Evans"
the actor vs. the presenter), same-title films across years and remakes, a video matched against
a film catalogue — the owner has only `label` + `disambiguation` (+ F61 `detail` on reveal) to
choose from. For people especially, the headshot is the fastest signal there is, and the provider
already has it on the search result (TMDB `profile_path`, `poster_path`, `logo_path`). The
candidate contract carries no image, so the owner reads three text fields or clicks through
`view source ↗` to see a face.

## Goals

- **G1** — A provider can attach one small thumbnail to a candidate; Holodex shows it in the picker
  row for every entity type that mounts the picker (person, video/media, film, studio) with **no
  entity-kind branching** in the component.
- **G2** — The thumb is served straight from the provider's allowlisted host with the gate Holodex
  already trusts for `image_url` fields; nothing new is downloaded, normalized, or stored.
- **G3** — Rows align whether or not a thumb arrives: the column is always present; a missing or
  refused image shows the entity monogram on the plate.
- **G4** — Every existing row affordance — match strength, `disambiguation`, `view source ↗`, the
  F61 `details` toggle, roving tabindex, auto-expand on label collision — is unchanged.

## Non-Goals

- **A proxy or cache for candidate images.** 25 candidates × one 300 ms debounce = transient. A
  `serveProviderIcon`-style fetch-normalize-store endpoint is a new perimeter for an image that
  lives seconds. If the owner-IP-to-CDN exposure ever matters, that is the change to bring back —
  not here.
- **Size negotiation between core and sidecar.** The contract says "small thumbnail suitable for a
  list row"; the sidecar picks the rendition (TMDB `w185`). Core does not send a size hint.
- **Hover / zoom preview of the thumb.** The row is a chooser, not a viewer; `view source ↗` is the
  path to a bigger picture.
- **The entity's own current image in the picker header for side-by-side comparison.** Cheap via
  `PersonAvatar`, but it is a second feature and a comparison — layer 1 (ADR-090) judges a candidate
  against the entity's baseline by *identity*, not by image match.
- **Treating the thumb as a poster / headshot adoption.** It is candidate identity evidence, like
  `label`. It promises nothing about which image lands in the shadow store on `/enrich` — that stays
  `assets[]` (§4.3), download-and-store, aspect-guarded (HOLODEX-386).
- **A grid or card layout for the picker.** Drops match strength, `detail`, and the roving-tabindex
  list; 25 tiles is nine rows of scrolling to find a "Possible" the list shows in one screen.
- **Per-entity-kind thumb shapes** (circle for people, 2:3 for posters, wide for logos). One 2:3
  box, `object-contain` on the logo plate, covers all three without the component knowing which
  entity it is serving.

## Users & Value

- **Owner, resolving a person.** Opens Enrich on "Chris Evans", sees three same-label rows. Today:
  reads `Actor · Captain America` / `Presenter · Top Gear` / `Crew`. With F64: sees the face, clicks
  the right row without reading. The value is speed and confidence on the entity type where text
  disambiguation is weakest.
- **Owner, resolving a film or a video against a film.** Year already does most of the work in
  `disambiguation`; the poster confirms it and separates a remake or an alternate cut at a glance.
- **Owner, resolving a studio.** Recognition more than disambiguation — the logo on the plate.
  Rides along because the component is shared; costs nothing extra.
- **Provider implementer.** One optional string per candidate, drawn from data the search result
  already returns. Emit it before Holodex reads it; an older Holodex ignores the key.

## Functional Requirements

### Must-Have (P0)

#### FR1 — Contract: `candidates[].image_url` (§2.3, §5, §6, §8)

A `/resolve` candidate MAY carry `image_url: string` — an absolute `https` (or `http`) URL to a
**raster thumbnail suitable for a list row** (a person portrait, a poster, a logo). One URL, one
rendition, chosen by the provider; the ~2:3 portrait renditions providers already serve for search
results are the intended shape, but any raster works because the row uses `object-contain`.
Entity-agnostic. Additive. Omit when absent — never send `""`. Not stored, not written back, not
returned from `/enrich`.

- **Given** a candidate carries `image_url` on a host that is the source's `base_url` host or one
  the operator listed in `asset_hosts`, **when** Holodex decodes the response, **then** the field
  reaches the client unchanged.
- **Given** a candidate carries `image_url` on any other host, **when** decoded, **then** the field
  is **dropped** before the response leaves the server — the candidate itself is still usable, no
  error, same silent posture as a malformed `profile_url`.
- **Given** a candidate carries `image_url` with a non-`http(s)` scheme, a malformed URL, or an
  empty string, **when** decoded, **then** it is dropped the same way.
- **Given** a provider predating this spec, **when** any resolve happens, **then** every row renders
  with the monogram plate and nothing else changes.

#### FR2 — Sanitization: host gate in `sanitizeCandidates`

`sanitizeCandidates` (`internal/enrich/service.go`) gains an `image_url` step beside the
`profile_url` scheme check: parse → scheme `http`/`https` → `Service.ImageURLAllowed(host)` (the
ADR-056 check, itself `assetHostAllowed` over the source's allowlist) → keep or clear. It is the
**same function** the field-render gate calls; F64 adds a caller, not a rule. The `Fake` provider
gains an `ImageURL` per candidate so API tests can drive it.

- **Given** the allowlist for source `acme` is `{acme.example, cdn.acme.example}`, **when** a
  candidate's `image_url` host is `cdn.acme.example`, **then** it is kept; `img.other.example` →
  cleared; `acme.example.evil.example` → cleared (exact host match, as today).
- **Given** the sanitizer runs, **when** any other candidate key is inspected, **then** its
  treatment is unchanged (regression on the existing candidate-sanitizer table).

#### FR3 — Picker row: fixed 2:3 thumb column

Every `<li role="option">` in `EnrichPicker.svelte` gains a leading, fixed-size image slot —
**40 × 60 CSS px** (2:3) — before the existing text block; the text block keeps its current
structure (label + match strength, `disambiguation`, actions line, F61 `detail` list). The slot:

- renders `<img src={c.image_url} alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer">`
  with `object-contain` on the **logo plate** background (`bg-logo-plate`), so a 2:3 portrait fills
  the box and a wide logo letterboxes inside it — one rule for every entity kind;
- shows the **monogram** of `c.label` (the `monogram()` helper in `web/src/lib/format.ts`, the
  `ProviderIcon` / `FilmsRow` idiom) on the same plate when `image_url` is absent, and swaps to it
  on `<img>` `error` (a 404 or a refused decode must not leave a broken-image glyph);
- is `aria-hidden` / `alt=""`: the label already carries the name, the thumb is decoration to a
  screen reader;
- is **top-aligned** (`items-start`) so an F61-expanded row grows downward with the thumb pinned to
  the label line, and a collapsed row is exactly one thumb tall plus padding.

- **Given** a candidate with an allowed `image_url`, **when** the picker renders, **then** the row
  shows the image at 40 × 60 with no layout shift on load (the box is sized before the bytes arrive).
- **Given** a candidate without `image_url`, **when** the picker renders, **then** the row shows the
  monogram plate in the same 40 × 60 slot and the text block starts at the same x as every other row.
- **Given** a row whose image 404s, **when** the browser fires `error`, **then** the slot falls back
  to the monogram — never the broken-image icon.
- **Given** a row with `detail` expanded, **when** it grows, **then** the thumb stays top-aligned
  and the expanded lines start under the label, not under the thumb.
- **Given** 25 candidates each with an image, **when** the list renders, **then** images below the
  fold are not requested until scrolled near (`loading="lazy"`), and the listbox scroll container
  is unchanged.
- **Given** any candidate row, **when** the owner uses ↑/↓, Tab, Enter, Space, click, or the
  `details` toggle, **then** behaviour is identical to F61 — the slot is not focusable and not a
  click target of its own (clicking it selects the row, as clicking the label does).

#### FR4 — TMDB sidecar emits `image_url`

`providers/tmdb` maps the search result's image path to `image_url` with the existing
`tmdbImageURL` builder at the **`w185`** rendition (not `original`): person `profile_path`, movie
`poster_path`, company `logo_path`. Omitted when the path is null. The provider's docs for
operators already tell them to allowlist `image.tmdb.org`; F64 adds a sentence noting the picker
now renders from it too.

- **Given** a TMDB person search hit with a `profile_path`, **when** the sidecar builds the
  candidate, **then** `image_url` is `https://image.tmdb.org/t/p/w185/<path>`.
- **Given** a hit with a null image path, **when** built, **then** the key is absent.

### Nice-to-Have (P1)

None. The scope is deliberately one column and one field.

### Future Considerations (P2)

- **P2-a — Proxy for the owner-IP-to-CDN case.** If a deployment needs the owner's browser never
  to touch a provider CDN, add a transient fetch-and-stream endpoint behind the same allowlist.
  Not needed for the local / Tailscale deployments this project targets.
- **P2-b — Entity's current image in the picker header.** Bring back only if owners ask for a
  side-by-side; `PersonAvatar` makes it a small change.

## Acceptance Criteria

1. A `/resolve` candidate with an allowlisted `image_url` reaches the picker unchanged; one on a
   non-allowlisted host, with a bad scheme, malformed, or empty, reaches the picker with the key
   absent; a pre-F64 provider's response renders byte-for-byte as today except for the monogram
   column.
2. The gate is `Service.ImageURLAllowed` — the same function ADR-056's `gateImageURL` calls —
   invoked from `sanitizeCandidates`; no second allowlist, no new config key.
3. The contract carries the new §2.3 row and example, the §5 row, the §6 S7 note, and the §8
   example, and states: entity-agnostic, additive, omit-when-absent, one small raster rendition,
   rendered not fetched, not stored.
4. Every picker row has a 40 × 60 slot at the left; rows with and without an image have identical
   text-block x-offset and identical collapsed `offsetHeight`.
5. An absent or failed image shows the label monogram on the plate; a broken-image glyph never
   appears.
6. The `<img>` carries `alt=""`, `loading="lazy"`, `referrerpolicy="no-referrer"`; it is not
   focusable and adds no tab stop inside the modal's focus trap.
7. Studio candidates (wide logos) letterbox inside the same box on the plate — no entity-kind
   conditional in `EnrichPicker.svelte`.
8. F61 behaviour (toggle, inline expansion, auto-expand on collision, per-row state across ↑/↓,
   reset on new response) is unchanged; an expanded row's lines sit under the label with the thumb
   top-aligned.
9. The TMDB sidecar emits `image_url` at `w185` for person / movie / company hits with an image
   path and omits it otherwise.
10. No `image_url` value is stored in `entity_enrichment`, returned from `/enrich`, sent to
    writeback, or written to the activity log.
11. All three skins: the plate, the monogram, and the image frame use tokens only
    (`bg-logo-plate`, `text-muted`, `border-*`, `rounded-theme`) and read correctly in each skin.

## Test Notes (for `/testing-strategy`)

- **Sanitizer (`internal/enrich`)** — table test for the `image_url` step of `sanitizeCandidates`:
  allowlisted base host kept; allowlisted `asset_hosts` entry kept; foreign host cleared;
  suffix-spoof host (`base.example.evil.example`) cleared; `ftp:` cleared; malformed cleared; `""`
  cleared; absent stays absent. Regression: the existing table for `label` / `disambiguation` /
  `profile_url` / `detail` unchanged. This is the **riskiest-assumption test** — that the allowlist
  gate is the whole perimeter — and it is one table.
- **API (`internal/api` resolve handlers)** — with the `Fake`: a candidate whose `ImageURL` is on
  the Fake's allowlisted host round-trips to the JSON response; one on another host arrives without
  the key. One test each for the person and film resolve handlers (they pass `res.Candidates`
  through; the assertion is that the gate ran upstream of both).
- **Sidecar (`providers/tmdb`)** — unit test on the candidate builders: `profile_path` /
  `poster_path` / `logo_path` → `w185` URL; null path → no key.
- **Picker — pure helper (vitest)** — no component harness in this repo (F61 precedent): the
  "show image vs monogram" decision and the monogram derivation live in a pure module with unit
  tests; DOM behaviour is verified live against the stub per the QA checklist (slot present on
  every row; `error` → monogram; no extra tab stop; F61 toggle still works on an image row).
- **Geometry** — rows with and without an image share `offsetHeight` when collapsed and share the
  text block's `getBoundingClientRect().x`; three-skin contrast of the monogram on
  `bg-logo-plate` for both the resting and the active (`bg-surface-2`) row — computed-style
  approach, screenshots time out on this picker.
- **Contract stub (`testdata/enrich-stub/`)** — a person candidate set mixing: image on the
  stub's own host, image on a foreign host (must be stripped), no image, and an image path that
  404s — so the QA checklist exercises all four slot states against a real sidecar.

## Resolved Decisions

Folded from the brainstorm (2026-09-16):

| # | Question | Decision | Why |
|---|---|---|---|
| 1 | Row shape? | **2:3 thumb list** (40 × 60) | Keeps match strength, `detail`, roving list; a circle crops posters; a grid loses the list model |
| 2 | Hot-link or proxy? | **Hot-link via the ADR-056 allowlist gate** | Transient image, existing gate, owner-only surface; proxy is P2-a if a deployment ever needs it |
| 3 | Studios? | **Ride along, same box** | Shared component; `object-contain` on the plate makes logos work with zero branching |
| 4 | Who picks the rendition? | **The sidecar** | "Small thumbnail for a list row" is contract language; core sends no size hint |
| 5 | Is the thumb an image adoption? | **No — identity evidence** | Layer 1 (ADR-090) evidence like `label`; `assets[]` on `/enrich` stays the only image path into the store |
| 6 | ADR? | **No** | Additive optional key + an existing gate applied to one more caller |

## Open Questions

- ~~(design) Exact plate treatment for the monogram — `ProviderIcon` or `FilmsRow` idiom?~~ —
  **settled 2026-09-17** in the design handoff: the `FilmsRow` 2:3 tile (`aspect-[2/3]
  rounded-theme bg-logo-plate` + `font-display text-sm font-semibold text-logo-plate-ink`
  monogram) at `w-10`, with `object-contain` in place of `FilmsRow`'s `object-cover` because the
  candidate image's aspect is not gated upstream (`entity/CLAUDE.md` rule). `ProviderIcon`'s
  plate is a square, size-driven inline icon — the wrong shape.
- **(engineering, non-blocking)** Whether the `error` → monogram swap is a `$state` flag per row or
  a CSS `:has()` trick — implementation's call; AC5 fixes the outcome.

## Timeline Considerations

- No hard deadline. The TMDB sidecar can emit `image_url` **before** core reads it (unknown key,
  ignored); either side can ship first.
- Sequence: spec + contract amendment (this change, Draft PR) → design handoff (SVG committed) →
  backend FR1/FR2 + tests → sidecar FR4 + test → frontend FR3 + tests + three-skin QA → testing
  strategy → security review → mark ready. One story, one PR.
