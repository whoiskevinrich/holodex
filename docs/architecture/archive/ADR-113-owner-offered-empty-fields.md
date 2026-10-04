# ADR-113: Offer empty replace fields to the owner, one field at a time, through the resolver

> **Archived — not current truth.** This numbered ADR was retired by HOLODEX-528. Its architecture now lives in [field-resolution.md](../field-resolution.md). Its product and UI rules now live in the owning spec and design docs (`docs/specs/`, `docs/design/`). Read those; this file is kept only as history.


**Status:** Proposed
**Date:** 2026-09-27
**Deciders:** Project owner

**Extends:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field source decisions;
the resolver's output shape). **Relates to:**
[ADR-090](ADR-090-two-layer-entity-metadata-management.md) (layer 2 precedence UI),
[ADR-085](ADR-085-films-entity.md) §4 (the film-candidate exception to the same drop),
[ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md) (`in_sync`). HOLODEX-471 (first
adopter: `overview`) · HOLODEX-304 (the remaining fields).

---

## Context

`ResolveFields` (`internal/resolver/resolver.go`, the `len(items) == 0` branch) leaves a replace
field out of `resolved[]` when three things are all true: no layer has a value, there is no standing
decision, and there is no film candidate. Two exceptions were added later: a standing blank pin (F37
RD3) and an undecided film candidate (ADR-085 §4). No ADR decided the drop itself. It dates from
before per-field decisions existed, when a row without a value had nothing to show.

With ADR-051 a row *can* do something without a value: it is where the owner types a Custom value.
Dropping it strands the owner in two places:

- **The detail page.** The Overview block is gated on the field existing, so an overview no source
  supplies has no pencil, and the owner can't add one (HOLODEX-471).
- **The writeback dialog.** It lists `resolved[]`, so it has no row for the field. `markWriteTargets`
  only stamps rows that exist, so a row added client-side would also be unwritable.

The page already works around this in one place. `deepLinkedMissing` builds the missing row
client-side for completeness deep links, but only for `curatable` facets, and `long_text` fields
such as `overview` are never curatable. A second client-side synthesis for overview would mean two
builders of the same row, and neither would carry `write_target`.

HOLODEX-304 proposed the global fix: stop dropping zero-candidate replace fields. That changes every
surface at once:
- The Metadata list would grow a "—" row for every empty field.
- The writeback dialog would grow a row for every empty field.
- Visitors would receive rows they then have to filter out.
- The completeness and `visibleResolved` filters written against the drop would need re-checking.

None of those surfaces has been designed for it.

## Decision

### D1: The resolver offers an empty field when asked to; it doesn't decide which fields

`resolver.Options` gains an `Offer` predicate (`func(canonical string) bool`, nil = offer nothing).
When a replace field would be dropped and `Offer(canonical)` is true, the resolver keeps it, with
the same shape it already builds for an undecided field:
- `values: []`
- `candidates: [{source: "file", value: ""}]`, since `replaceMarkers` always emits the file candidate
- a non-standing `decision` whose source falls back to `file`

No new field and no new shape: `sourceChips()` already renders this as File "No value" + Custom. The
resolver stays pure, and it stays the one place a resolved row is built (ADR-033).

Being built before `markWriteTargets` is not enough on its own to make an offered row writable.
`writeback.ResolveForContainer` skips a field with no values, so an empty row would get no
`write_target` and the dialog would disable it. `markWriteTargets` therefore asks the mapper where
an offered field's value *would* land (it passes one empty value for a valueless offered row). An
empty row that the owner blank-pinned keeps today's unwritable stamp. The write path is unchanged:
by the time anything is written, the row carries the Custom value.

Merge, multi, entity-link and identity fields are never offered. Offering means a place to type a
Custom value, and ADR-051 RD1 limits Custom to replace fields.

### D2: Which fields are offered is per-field registry metadata, adopted one component at a time

`registry` field metadata gains `OfferWhenEmpty bool`. The API builds `Offer` from it. A field
gets the flag **in the same change that designs its empty state**: the surface that renders it has
to show an intentional empty affordance, and the writeback row has to be checked. Until then the
field keeps today's behaviour. Nothing flips globally.

First adopter: `overview` (HOLODEX-471). The rail shows **+ Add overview**, and the writeback dialog
gains the row; see [overview-add-handoff](../../design/overview-add-handoff.md). HOLODEX-304 becomes
the backlog of later adopters, each carrying its own design. The Metadata list (tagline and the rest)
is the next candidate and needs a design for a list of empty rows before any of its fields flip.

### D3: Offered rows are owner-only, gated at the API

`getMedia` passes `Offer` only when the request is `authorized`. Visitors get the same `resolved[]`
they get today, so no visitor surface needs a filter, and an empty offered row can't leak an
owner-only affordance through a surface that forgot to gate it. The same predicate applies to the
person and studio resolvers when they adopt a field; until then they pass nil.

### D4: Once a field adopts this, the client never synthesizes its row

For an adopted field, the page and dialog use the server row. `deepLinkedMissing` keeps covering
curatable facets that haven't adopted. When a curatable field adopts, its server row exists and the
synthesis stands down on its own, because it only fires for fields missing from `canonicalResolved`.

## Consequences

- **Good:**
  - A missing value is added in the same editor as a present one: one dialog, one decision endpoint,
    one writeback row.
  - Each surface decides for itself when it's ready. No surface changes before it's designed.
  - The resolver stays the one merge point, and visitors are unaffected.
- **Cost:**
  - Each surface needs one registry flag and one design pass when it adopts.
  - `deepLinkedMissing` and D1 coexist while the curatable facets migrate.
- **Completeness:** unaffected. An offered row has no value, so its facet is still `missing`, and
  the score reads the facet, not the presence of the row.
- **Writeback:** unaffected until the owner picks Custom. An offered, undecided row with no value
  has `in_sync` true or unknown, so `needsWriteback()` doesn't count it and the dialog doesn't
  preselect it. Its place is "Not yet decided".

## Alternatives considered

- **Stop dropping empty replace fields everywhere (HOLODEX-304 as filed).** Rejected for now
  because it changes several undesigned surfaces at once (see Context). D2 gets there one field
  at a time, and the flag can become the default once every surface has adopted.
- **Build the empty overview row in the page.** Rejected: that is a second builder of a resolver row
  with no `write_target`, so the writeback dialog would still be missing it. It is the
  `deepLinkedMissing` problem again.
- **A hard-coded offer list in the API handler.** Rejected: which fields are offered is per-field
  metadata, and the registry is where per-field metadata lives (label, display, criticality).
- **Offer to visitors and filter in the UI.** Rejected: it moves an owner gate to every consuming
  component (see "Visitor vs. owner is a control gate" in `routes/CLAUDE.md`).
