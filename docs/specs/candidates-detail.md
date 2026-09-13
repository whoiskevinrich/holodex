# Spec: Revealable per-candidate record summary in the resolve picker (F61)

**Status**: Draft
**Phase**: Phase 3 follow-up (enrichment quality)
**Owner**: Project owner
**Date**: 2026-09-13
**Feature block**: **F61** — a `/resolve` candidate may carry `detail: string[]`, short lines the
owner can **reveal** to choose between candidates the one-line `disambiguation` cannot separate.
Collapsed by default; expands inline on a toggle; auto-expands when two or more candidates share a
label; the auto-applied candidate's lines reach the activity log on the unattended path.

**Issue**: [HOLODEX-380](https://whoiskevinrich.atlassian.net/browse/HOLODEX-380)
**ADR**: none — additive optional response key, same posture as `profile_url` (F47/RD6) and
`searched[]` ([ADR-095](../architecture/ADR-095-structured-resolve-hints.md) D6). The wire promise
that makes this safe is [§2.3](metadata-provider-contract.md#23-post-resolve--identity-match-disambiguation)'s
"Holodex ignores unknown response keys".
**Contract amendment**: [metadata-provider-contract.md](metadata-provider-contract.md) §2.3
(response example + field row) and §5 (caps row) — amended in the same change as this spec.
**Design handoff**: [candidates-detail-handoff.md](../design/candidates-detail-handoff.md) +
[mockup](../design/candidates-detail-mockup.svg) + [QA checklist](../design/candidates-detail-qa-checklist.md)
(2026-09-13: text toggle chosen over an info glyph — the picker's existing `.btn-quiet`
dotted-underline idiom, zero new components or icons)
**Origin**: a proposal from the partner video provider's implementer, reviewed and accepted
2026-09-13. The provider already emits the studio chain in `disambiguation` when duplicates
collide and tie-breaks equal confidences on catalogue richness; this spec gives that invisible
signal a place to show.

**Depends on** (all shipped):
- the provider sidecar contract, `POST /resolve` ([ADR-033](../architecture/ADR-033-metadata-source-plugins.md))
- the picker's `searched[]` caption and its caps/sanitization pattern
  (`internal/enrich/resolve_hints.go`, [ADR-095](../architecture/ADR-095-structured-resolve-hints.md) D6)
- `profile_url` on candidates and the `view source ↗` in-row link pattern (F47/RD6,
  `web/src/lib/components/enrichment/EnrichPicker.svelte`)
- auto-apply of a lone strong candidate and the unattended refresh-all path
  ([ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md) D1,
  `internal/api/enrich_review.go`)

## Problem Statement

A resolve candidate is rendered from `label`, `disambiguation`, a confidence badge, and a
`profile_url` link. That density separates same-named *entities*. It does not separate the case an
upstream catalogue produces systematically for `video`: **one release catalogued several times,
once per distribution outlet** — identical title, date, and cast; only the outlet, the genre-tag
count, and the image set differ. The owner's decision is not "which of these is the video" (they
all are) but **which record to bind to**, and that choice has consequences: the record's outlet
chain becomes the video's `_studio_external_ids` (§4.6), its tags become `genres` (F50/ADR-075),
its synopsis and poster become the video's. Today the picker shows four rows at the same confidence
in an order the owner can't explain, and the only way to compare is four `view source ↗` tabs.

The provider holds exactly the information needed and has nowhere to put it: `disambiguation` is
one line and already carries `studio · date · cast`; stuffing it makes every candidate on every
resolve longer to serve a case most resolves never hit.

## Goals

1. **The duplicate-release case is decidable in the picker.** For a response where ≥ 2 candidates
   share a label, the owner can see each record's distinguishing summary without leaving the modal.
2. **Zero cost when unneeded.** A provider that emits `detail` on every candidate changes nothing
   about a row's default height or reading order on a resolve that has no collision.
3. **Provider-agnostic.** Holodex renders lines; it does not learn any provider's notion of
   "completeness". A new provider can use the slot without a vocabulary negotiation.
4. **Auditable when unattended.** An auto-applied match's `detail` lines are in the activity log,
   so a refresh-all pass that bound the "wrong" duplicate can be diagnosed after the fact.
5. **Same safety envelope as `searched[]`.** Untrusted provider text is capped, sanitized, never
   stored in `entity_enrichment`, never written back.

## Non-Goals

- **A typed candidate schema** (`{studio, tag_count, has_synopsis}`). It would need a vocabulary,
  labels, and precedence entries the way `fields` does — heavy machinery for a reveal — and would
  encode one provider's notion of completeness into the contract. Lines keep Holodex agnostic.
- **Parsing the `Key:` prefix.** Lines render verbatim. A muted-prefix rendering is a client-only
  nicety that can come later with no wire change (P2-a) — it is not needed to make the case
  decidable.
- **An inline thumbnail / `thumbnail_url` on candidates.** On the duplicate-release case the
  records often share one cover render, so an image confirms "right video" but does not separate
  rows — and it would be the first candidate-level field Holodex has to *fetch* through the SSRF
  perimeter rather than merely render. Revisit as its own proposal if owners still click through.
- **Changing `disambiguation`, `confidence`, or `profile_url` semantics.** `detail` supplements the
  one-line summary; it may *explain* an order but never changes it; `profile_url` stays the deep link.
- **Storing `detail`, or surfacing it anywhere but the picker and the activity log.** It is not a
  field, does not enter the shadow store, and never reaches writeback or the MCP surface.
- **Holodex rendering `external_id` itself.** The provider's `Id:` line duplicates a value already
  on the wire; that is the provider's line to spend, not a Holodex affordance. Bounded on purpose.

## Users & Value

- **Owner, interactive resolve (primary).** Opens Enrich on a video, sees four "Harbor Lights" rows
  at "Possible match", and can now read `Record: 28 tags · synopsis · 3 images` against
  `Record: 16 tags · synopsis · 1 image` without opening four tabs. Binds to the richer record on
  purpose.
- **Owner, after a refresh-all.** A video came back with an unexpected studio chain. The activity
  entry for that resolve shows what was searched *and* the applied candidate's `Studio:` line, so
  the owner can tell "provider bound the Network X outlet" from "provider matched the wrong video".
- **Provider implementer.** Has a contract-defined slot for a per-record summary, with caps that
  make the "just stuff `disambiguation`" workaround unnecessary. Can ship before Holodex reads it.

## Functional Requirements

### Must-Have (P0)

#### FR1 — Contract: `candidates[].detail` (§2.3) and its caps (§5)

`POST /resolve` response candidates gain one optional key, `detail: string[]`. Each entry is one
display line, conventionally `Key: value`. Entity-agnostic (allowed on `person`, `studio`, `video`,
`film` candidates alike — unlike `searched[]`, which is `video`-only). Additive: an older Holodex
ignores it; a provider that omits it renders exactly as today. No `protocol_version` bump.

Caps, mirroring the `searched[]` row in §5:

| Constraint | Value |
|---|---|
| Entries per candidate | ≤ **8** — Holodex keeps the first 8 |
| Chars per entry | ≤ **256** — truncated beyond |
| Newlines | none — control characters stripped, same treatment as `label` |
| Emptiness | omit the key; `[]` is a provider bug, tolerated as absent |

- **Given** a candidate carries `detail` with 3 entries under the caps, **when** Holodex decodes
  the response, **then** the candidate reaches the client with those 3 lines in provider order.
- **Given** a candidate carries 12 entries, one of 400 chars containing `\n`, **when** decoded,
  **then** the client receives 8 lines, the long one truncated to 256 chars with the newline
  stripped, and the candidate is otherwise unaffected (label, confidence, `profile_url` intact).
- **Given** a candidate carries `"detail": []`, **when** decoded, **then** it is treated as absent:
  no toggle, no `detail` key on the wire to the client.
- **Given** a provider predating this spec (no `detail` anywhere), **when** any resolve happens,
  **then** the response and the picker are byte-for-byte what they are today.

#### FR2 — Sanitization sibling of `sanitizeSearched`

`internal/enrich` bounds `detail` at the same boundary that bounds `searched[]` (the resolve
sanitizer that already runs `label`/`disambiguation`/`profile_url` through `SanitizeValue`), with
its **own** per-entry cap — `SanitizeValue`'s 4096 is the field-value cap, not this one.

- **Given** `detail` entries containing control characters (`\x00`, `\x1b[31m`, `\r\n`), **when**
  sanitized, **then** the control characters are stripped and no entry can break the line it renders on.
- **Given** the sanitizer runs, **when** any other candidate key is inspected, **then** its
  treatment is unchanged (regression guard on the existing loop).

#### FR3 — Picker reveal: text toggle, inline expansion

A candidate row with a non-empty `detail` shows a **toggle button** — the text `details` /
`hide details` in the picker's existing `.btn-quiet` dotted-underline idiom (the Searched
caption's `+N more`) — trailing the `view source ↗` link on the row's actions line, so the row's
existing reading order is preserved. Activating it expands the lines **inline beneath the row**
(the row grows; nothing floats, nothing is clipped by the listbox's scroll container). Activating
again collapses. The toggle is a real button: reachable by Tab inside the existing focus trap,
toggled by Enter/Space, tapped on touch, and it stops propagation so toggling never confirms the
candidate. Its `aria-expanded` tracks the state and `aria-controls` names the lines' container;
its visible text is its accessible name. Hover is *not* a mechanism — reveal is by activation only.

A row with no `detail` shows no toggle and is unchanged.

- **Given** a candidate with `detail`, **when** the picker renders, **then** the row is the same
  height as a row without `detail`, plus one `details` toggle on the actions line.
- **Given** the owner activates the toggle (click, tap, Enter, or Space), **when** the lines expand,
  **then** they render beneath the row as one visual line per entry, verbatim, in provider order,
  muted against the row's `label`, and the candidate is **not** confirmed.
- **Given** the lines are expanded, **when** the owner activates the toggle again (now `hide details`), **then** they collapse.
- **Given** the lines are expanded on row 2, **when** the owner uses ↑/↓ to move the active row,
  **then** row 2's expansion state is preserved (state is per row, per response).
- **Given** a new search runs, **when** new candidates arrive, **then** every row starts collapsed
  (subject to FR4) — expansion state does not leak across responses.
- **Given** a row is expanded, **when** the owner presses Enter with the *row* focused (not the
  toggle), **then** the candidate confirms exactly as today — the toggle does not intercept row keys.

#### FR4 — Auto-expand on label collision

When **two or more** candidates in one response share a normalized `label` (case-folded,
whitespace-collapsed, trimmed), every candidate in that collision group that carries `detail`
renders **expanded by default**. Candidates outside any collision group start collapsed. The
toggle still works on auto-expanded rows.

- **Given** four candidates labelled "Harbor Lights" and one "Harbor Lights II", each with
  `detail`, **when** the picker renders, **then** the four are expanded and the fifth is collapsed.
- **Given** two candidates "harbor lights" and "Harbor  Lights", **when** the picker renders,
  **then** they count as a collision (normalized) and both expand.
- **Given** two same-labelled candidates where only one carries `detail`, **when** the picker
  renders, **then** that one expands and the other has no toggle.
- **Given** all candidates have distinct labels, **when** the picker renders, **then** nothing is
  auto-expanded even if every candidate carries `detail`.

#### FR5 — Unattended path: the auto-applied candidate's `detail` reaches the activity log

On refresh-all, the activity entry that records the resolve (today: `… (N candidates) ·
searched: …`, written by `RecordSearched`) also carries the **auto-applied** candidate's `detail`
when one applied and it carried lines: `· applied: <label> — <line> · <line> …`. The entry is
written when *either* `searched[]` or an applied candidate's `detail` is present (today it is a
no-op when `searched[]` is empty). Implementation note: `RecordSearched` currently runs *before*
`SingleStrongMatch` — it needs the applied candidate (or nil) as an input, or to move after it.

The interactive picker path does **not** log `detail` — the owner saw it and chose.

- **Given** a refresh-all resolve returns one candidate at 0.91 with `detail`, **when** it
  auto-applies, **then** the activity entry contains `applied: <label> — ` followed by the lines
  joined with ` · `.
- **Given** a refresh-all resolve returns four equal-confidence candidates, **when** it stops at
  `needs_review`, **then** the entry is what it is today — no `applied:` segment (nothing applied).
- **Given** a refresh-all resolve returns a lone strong candidate **without** `detail` and the
  provider sent no `searched[]`, **when** it auto-applies, **then** no resolve entry is written
  (unchanged from today; the apply itself is logged as it is now).
- **Given** a refresh-all resolve returns a lone strong candidate with `detail` **and the apply
  then fails** (provider error on `/enrich`), **when** the entry is written, **then** it carries
  `searched[]` only — no `applied:` segment — and the enrich job's own `(failed)` entry records
  the failure. The audit line never claims a binding that didn't happen.
- **Given** the entry is written, **when** rendered in System Activity, **then** it contains no
  file path (F22.6b no-path invariant holds — `detail` is provider text about the provider's
  record, and the sanitizer has already run).

#### FR6 — Never stored, never written back

`detail` is not persisted to `entity_enrichment`, does not appear in `/enrich` field results, and
is not a writeback input. The only durable trace is the FR5 activity-log line.

- **Given** the owner confirms a candidate that carried `detail`, **when** the enrichment is
  stored, **then** no `detail` text is in the shadow store, the resolved field list, or the
  writeback payload.

### Nice-to-Have (P1)

#### P1-a — Line count in the collapsed toggle

The closed toggle may read `details (3)` so the owner knows how much is behind it before opening.
The design handoff settled the base copy as `details` / `hide details`; the count is an optional
refinement, cheap to add with FR3 — skip it if it reads as noise in the monospace skins.

### Future Considerations (P2)

- **P2-a — Muted `Key:` prefix.** Split each line on its first `: ` and render the key muted.
  Client-only; lines without `: ` fall back to verbatim. Not a wire change — deliberately deferred
  so Holodex ships with no parsing of provider strings.
- **P2-b — Person / studio emitters.** The partner provider emits `detail` for `video` only; the
  contract is entity-agnostic so no change is needed when a person/studio duplicate pattern appears.
- **P2-c — Candidate thumbnail.** Explicitly set aside (Non-Goals). If, with `detail` and
  `profile_url` both present, owners are still clicking through to compare, that is the evidence
  to bring it back as its own proposal — with the SSRF-perimeter cost priced in.

## Acceptance Criteria

1. A `/resolve` response candidate with `detail` reaches the picker with its lines in provider
   order; a candidate without `detail`, and any response from a pre-F61 provider, renders
   byte-for-byte as today.
2. `detail` is capped at 8 entries and 256 chars/entry, control characters stripped, no entry can
   contain a newline; `[]` and a missing key are indistinguishable to the client.
3. The contract carries the new §2.3 row (with the example JSON showing `detail`) and the new §5
   caps row, and states: entity-agnostic, additive, omit-when-empty, verbatim, not stored.
4. A row with `detail` is the same height as one without until revealed; the only visible
   addition is one `details` text toggle trailing the `view source ↗` link (alone on the actions
   line when there is no link).
5. The toggle is a button: Tab reaches it inside the modal's focus trap, Enter/Space/click/tap
   toggle it, toggling never confirms the candidate, `aria-expanded` and `aria-controls` are
   correct, and the row's own Enter/Space still confirm.
6. Expanded lines render inline beneath the row, verbatim, one per entry, and are never clipped by
   the listbox's scroll container at 25 candidates.
7. Expansion state is per row and survives ↑/↓ navigation; it resets when a new response arrives.
8. When ≥ 2 candidates share a normalized label, each of them that carries `detail` is expanded on
   first render; candidates outside the collision group are collapsed; with all-distinct labels
   nothing auto-expands.
9. On refresh-all, an auto-applied candidate's `detail` appears in the resolve activity entry as
   `applied: <label> — <lines · joined>`; a `needs_review` or `no_candidates` outcome adds nothing;
   the entry still contains no file path.
10. No `detail` text is stored in `entity_enrichment`, returned from `/enrich`, or sent to writeback.
11. All three skins: the toggle, the expanded lines, and the auto-expanded state use tokens only
    (`text-muted`, `text-accent`, `bg-surface-2`, `border-*`) and read correctly in each skin.

## Test Notes (for `/testing-strategy`)

- **Decoder / sanitizer (`internal/enrich`)** — table test for `sanitizeDetail`: under-cap
  passthrough; 12→8 truncation; 400→256 truncation; control-char and CRLF stripping; `[]`→nil;
  nil→nil. Regression: the existing candidate-sanitizer test still covers `label`, `disambiguation`,
  `profile_url` unchanged. `Fake` provider gains a `Detail []string` per candidate so API tests can
  drive it.
- **Unattended path (`internal/api` refresh-all)** — with the `Fake`: lone strong candidate with
  `detail` → activity entry contains `applied: … — …`; four equal candidates → entry unchanged;
  lone strong candidate without `detail` and no `searched[]` → no resolve entry; assert the
  no-path invariant on the rendered detail string.
- **Picker (`EnrichPicker.svelte`, vitest + testing-library)** — toggle absent without `detail`;
  toggle present, `aria-expanded=false`, lines not in DOM; click/Enter/Space toggle; toggle does not
  call `confirm`; row Enter still confirms with lines expanded; state preserved across ↑/↓; reset on
  new response; collision normalization (`harbor lights` vs `Harbor  Lights`); mixed group (one
  with `detail`, one without); all-distinct → none expanded. The existing roving-tabindex and
  focus-trap tests must still pass with the toggle as an extra tab stop.
- **Geometry** — a row with `detail` collapsed has the same `offsetHeight` as one without;
  expanded lines at 25 candidates are inside the `<ul>`'s scroll box (no clipping); three-skin
  contrast on the muted lines against `bg-surface-2` for the active row (use the computed-style
  approach — screenshots time out on this picker).
- **Contract stub (`testdata/enrich-stub/`)** — a `video` candidate set with four same-label
  records carrying `detail`, so the QA checklist can exercise auto-expand against a real sidecar.

## Resolved Decisions

Folded from the maintainer review of the provider's proposal (2026-09-13):

| # | Question | Decision | Why |
|---|---|---|---|
| 1 | Is `Key: value` parsed or verbatim? | **Verbatim** | Holodex never parses provider strings; muted prefix is P2-a with no wire change |
| 2 | Touch / keyboard affordance? | **Toggle button, inline expansion** | Hover-only fails touch *and* keyboard in a keyboard-first listbox; a floating tooltip inside a scrolling modal gets clipped |
| 3 | Unattended path? | **In v1** (FR5) | Data is already on the wire; a refresh-all that bound the wrong duplicate is otherwise undiagnosable. Provider implication: emit `detail` on every candidate, not only on collisions — auto-apply is by definition a single candidate |
| 4 | Auto-expand on collision? | **Yes** (FR4) | The exact case the field exists for; the provider already mirrors this (expands the studio chain only on duplicates); cost is bounded — collapsed everywhere else, toggle still works |
| 5 | Entity scope? | **Entity-agnostic** | The decoder is shared; `searched[]`'s `video`-only posture came from `hint.*` being video-shaped, which doesn't apply here |
| 6 | ADR? | **No** | Additive optional response key under the existing unknown-key promise — `profile_url` precedent |

## Open Questions

- ~~(design) Toggle choice and expanded-lines typography~~ — **settled 2026-09-13** in the design
  handoff: text toggle in the existing `.btn-quiet` idiom; lines `text-xs text-muted` behind a
  `border-l border-rule` indent.
- **(engineering, non-blocking)** Whether `RecordSearched` takes the applied candidate as a
  parameter or is split into resolve-entry + applied-segment — implementation's call; FR5 fixes
  the entry's content and when it is written.

## Timeline Considerations

- No hard deadline. The partner provider can emit `detail` **before** this lands (unknown key,
  ignored) and has said it will once the merged contract text matches; either side can ship first.
- Sequence: spec + contract amendment (this change, Draft PR) → design handoff (SVG mockup
  committed) → backend FR1/FR2/FR5 + tests → frontend FR3/FR4 + tests + three-skin QA → testing
  strategy → mark ready. One story, one PR.
