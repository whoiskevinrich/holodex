# Spec: Candidate thumbnail in the resolve picker (F64)

**Status**: Draft
**Phase**: Phase 3 follow-up (enrichment quality)
**Owner**: Project owner
**Date**: 2026-09-17
**Feature block**: **F64** — a `/resolve` candidate may carry `image_url`, a small thumbnail the
picker renders in a fixed 2:3 box at the left of every row so the owner can tell same-named
candidates apart at a glance. Hot-linked from an allowlisted provider host through the render-time
gate ADR-056 already established for `image_url` field hints; never fetched, never stored.

**Issue**: [HOLODEX-406](https://whoiskevinrich.atlassian.net/browse/HOLODEX-406) ·
**Amended 2026-09-18** by [HOLODEX-414](https://whoiskevinrich.atlassian.net/browse/HOLODEX-414)
(PR #352): the slot's box is **kind-shaped** — portrait for person/film, landscape (backdrop) for
media, logo for studio — always 60 px tall. FR3, FR4, AC4/7/9 and Resolved Decisions 7–9 carry
the amendment; the original "one 2:3 box, no branching" rule is retired, not reworded.
**ADR**: none — additive optional response key under [§2.3](metadata-provider-contract.md#23-post-resolve--identity-match-disambiguation)'s
"Holodex ignores unknown response keys" (the `profile_url` / `detail` posture), and the browser-side
image gate is [ADR-056](../architecture/ADR-056-provider-field-render-hints.md)'s `render: image_url`
rule applied to one more surface. No new decision, no new perimeter.
**Contract amendment**: [metadata-provider-contract.md](metadata-provider-contract.md) §2.3
(response example + field row), §5 (caps row), §6 (S7 note), §8 (example) — amended in the same
change as this spec.
**Design handoff**: [candidates-image-handoff.md](../design/candidates-image-handoff.md) +
[candidates-image-mockup.svg](../design/candidates-image-mockup.svg) +
[candidates-image-qa-checklist.md](../design/candidates-image-qa-checklist.md); the per-kind
amendment in [candidates-image-per-kind-handoff.md](../design/candidates-image-per-kind-handoff.md) +
[candidates-image-per-kind-mockup.svg](../design/candidates-image-per-kind-mockup.svg) +
[candidates-image-per-kind-qa-checklist.md](../design/candidates-image-per-kind-qa-checklist.md).
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

#### FR3 — Picker row: kind-shaped thumb column, 60 px tall

Every `<li role="option">` in `EnrichPicker.svelte` gains a leading, fixed-size image slot —
**always 60 CSS px tall**, with a width set by the entity kind the picker was opened for — before
the existing text block; the text block keeps its current structure (label + match strength,
`disambiguation`, actions line, F61 `detail` list). *(Amended by HOLODEX-414; F64 as merged used
one 40 × 60 box for every kind.)*

| `entityType` | Box (≥ `sm`) | Box (< `sm`) | Classes | Intended image |
|---|---|---|---|---|
| `person` | 40 × 60 portrait | same | `w-10 h-15` | profile / headshot |
| `film` | 40 × 60 portrait | same | `w-10 h-15` | poster |
| `video` | 108 × 60 landscape | 80 × 45 | `w-20 h-11.25 sm:w-27 sm:h-15` | backdrop / still (16:9); a poster letterboxes when that is all the provider has |
| `studio` | 120 × 60 logo | 80 × 40 | `w-20 h-10 sm:w-30 sm:h-15` | logo — a wordmark spans the width, a symbol fills the height |

Below `sm` the two wide boxes narrow to 80 at the same aspect and the row's match-strength text
stacks under the name (every kind), because a 315 px dialog left a studio name ~30 px beside
"Strong match" (QA §4.4, Resolved Decision 10). The 60 px height — and therefore the 76 px row
floor — holds from `sm` up, which is where the geometry harness measures.

The picker takes a **required `entityType` prop** (`'person' | 'film' | 'video' | 'studio'`); its
five mounts (the four detail pages and `EnrichQueueRow` via the queue row's `entity_type`) pass
it. The mapping lives in a pure `slotShape(entityType)` + `SLOT_CLASS` in `candidateImage.ts`,
not in the template — adding a kind means adding a shape, never a `{#if}`. Both axes are explicit
(`w-* h-15`, not `aspect-*`) so the `<img>` can never widen the box to its natural width. Within
one open picker the shape is constant, so every row's text block starts at the same x. The slot:

- renders `<img src={c.image_url} alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer">`
  with `object-contain` on the **logo plate** background (`bg-logo-plate`), so the intended image
  fills its kind's box and anything else letterboxes inside it — never cropped;
- shows the **monogram** of `c.label` (the `monogram()` helper in `web/src/lib/format.ts`, the
  `ProviderIcon` / `FilmsRow` idiom) on the same plate when `image_url` is absent, and swaps to it
  on `<img>` `error` (a 404 or a refused decode must not leave a broken-image glyph);
- is `aria-hidden` / `alt=""`: the label already carries the name, the thumb is decoration to a
  screen reader;
- is **top-aligned** (`items-start`) so an F61-expanded row grows downward with the thumb pinned to
  the label line, and a collapsed row is exactly one thumb tall plus padding.

- **Given** a candidate with an allowed `image_url`, **when** the picker renders, **then** the row
  shows the image in the kind's box with no layout shift on load (the box is sized before the bytes
  arrive).
- **Given** a candidate without `image_url`, **when** the picker renders, **then** the row shows the
  monogram plate in the same kind's box and the text block starts at the same x as every other row.
- **Given** a media picker and a candidate whose provider sent a poster (no backdrop upstream),
  **when** the row renders, **then** the poster sits centred in the 108 × 60 box with plate either
  side — letterboxed, not cropped, not a monogram.
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

#### FR4 — TMDB sidecar emits `image_url`, rendition per entity kind

`providers/tmdb` maps a hit's image path to `image_url` with a list-row rendition (never
`original`), choosing the path by the `entity_type` Holodex sent *(amended by HOLODEX-414)*:

| `entity_type` | Path | Rendition |
|---|---|---|
| `person` | `profile_path` | `w185` |
| `film` | `poster_path` | `w185` |
| `video` | **`backdrop_path`**; `poster_path` when the hit has no backdrop | `w300` for the backdrop, `w185` for the poster |
| `studio` | `logo_path` | `w185` |

Omitted when the chosen path is null. `resolveMovie` therefore takes the `entityType` its caller
already switched on, and the search / find result structs decode `backdrop_path` (the details
struct already does). The provider's docs for operators already tell them to allowlist
`image.tmdb.org`; F64 adds a sentence noting the picker now renders from it too — backdrops are
on the same host, so the amendment widens nothing.

- **Given** a TMDB person search hit with a `profile_path`, **when** the sidecar builds the
  candidate, **then** `image_url` is `https://image.tmdb.org/t/p/w185/<path>`.
- **Given** a `video` resolve and a movie hit with a `backdrop_path`, **when** built, **then**
  `image_url` is `https://image.tmdb.org/t/p/w300/<backdrop>` — never the poster.
- **Given** a `video` resolve and a hit with a `poster_path` but no `backdrop_path`, **when**
  built, **then** `image_url` is the `w185` poster.
- **Given** a `film` resolve and a hit with both paths, **when** built, **then** `image_url` is the
  `w185` poster — never the backdrop.
- **Given** a hit with no usable image path, **when** built, **then** the key is absent.

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
4. Every picker row has a 60 px-tall slot at the left whose width is the kind's (40 / 108 / 120);
   within one picker, rows with and without an image have identical text-block x-offset and
   identical collapsed `offsetHeight`.
5. An absent or failed image shows the label monogram on the plate; a broken-image glyph never
   appears.
6. The `<img>` carries `alt=""`, `loading="lazy"`, `referrerpolicy="no-referrer"`; it is not
   focusable and adds no tab stop inside the modal's focus trap.
7. The slot's box comes from `slotShape(entityType)` / `SLOT_CLASS` in `candidateImage.ts`; the
   template has no entity-kind conditional and no `aspect-*` class; `entityType` is required (a
   mount that omits it fails `npm run check`).
8. F61 behaviour (toggle, inline expansion, auto-expand on collision, per-row state across ↑/↓,
   reset on new response) is unchanged; an expanded row's lines sit under the label with the thumb
   top-aligned.
9. The TMDB sidecar emits `image_url` per FR4's table — `w185` profile / poster / logo, and for a
   `video` resolve the `w300` backdrop with the `w185` poster as the fallback — and omits it when
   no path applies.
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
  `poster_path` / `logo_path` → `w185` URL; null path → no key; `video` with `backdrop_path` →
  `w300` backdrop, without → `w185` poster, with neither → no key; `film` with both → poster.
  The search and find fixtures carry `backdrop_path` so the decode is proven, not assumed.
- **Picker — pure helper (vitest)** — no component harness in this repo (F61 precedent): the
  "show image vs monogram" decision and the monogram derivation live in a pure module with unit
  tests; DOM behaviour is verified live against the stub per the QA checklist (slot present on
  every row; `error` → monogram; no extra tab stop; F61 toggle still works on an image row).
- **Geometry** — rows with and without an image share `offsetHeight` when collapsed and share the
  text block's `getBoundingClientRect().x`; the slot's `offsetWidth` equals the kind's px exactly
  (120 on the studio page the `flood` persona runs on) so an `aspect-*` regression is caught; three-skin contrast of the monogram on
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

Amended 2026-09-18 (HOLODEX-414, from a two-option inline mockup):

| # | Question | Decision | Why |
|---|---|---|---|
| 7 | One box for every kind, or kind-shaped? | **Kind-shaped** — portrait / landscape / logo — retiring #1's "one box" and #3's "zero branching" (the branch is a pure `slotShape`, not a template conditional) | A video row is compared against a landscape file thumbnail; a studio row against a logo. A poster in a 2:3 box was the wrong evidence for both |
| 8 | Height locked or width locked? | **Height locked at 60** (A): 40 / 108 / 120 wide. B (80 wide, rows shrink to the text stack) rejected | HOLODEX-406 §4.6 already ruled against handing row height to the text stack and thinning a logo's letterbox; 108 × 60 is the smallest recognisable still |
| 9 | Media hit with no backdrop? | **Poster fallback**, letterboxed in the landscape box | Better identity evidence than a monogram; `object-contain` means it can never crop |
| 10 | Narrow dialogs (QA §4.4, a 315 px dialog showed "Six P…")? | **Both** below `sm`: wide boxes to 80 at the same aspect **and** match strength under the name. Shrinking alone (~70 px for the name) and stacking alone (~124 px, clips a 128 px name) rejected from a four-panel mockup | The name is what the owner reads; 80 keeps a wordmark legible; nothing changes at ≥ 640 |

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
