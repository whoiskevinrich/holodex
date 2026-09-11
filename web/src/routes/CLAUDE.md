# Routes

SvelteKit pages. The two **detail pages** — `media/[id]` and `films/[id]` — share the `stage-grid`
shell from `app.css`, so they share a column contract. It is written here because it was answered
once per page, differently, before anyone noticed they were the same question.

## The two-zone column contract

`stage-grid` is one column below `lg` and `minmax(0, 1.4fr) minmax(320px, 1fr)` at or above it.
The split is not "important things left, less important things right":

| Zone | Holds |
|---|---|
| **Subject** (1.4fr) | the media object itself — player or poster, title, the resolution/duration/year meta line, studio |
| **Rail** (1fr, min 320px) | everything *about* the object — **overview/description**, tags, films, people, the resolved-field list, and the owner-only management blocks |

**Reader prose belongs in the rail, not the header.** The synopsis is a resolved canonical field
whose owner rendering *is* a `SourceBadge`, and HOLODEX-331 picked the 320px rail floor as "where a
field label, its value and its `SourceBadge` chip row still fit on one line" — the rail was already
sized for it. This is unconditional: no viewport branch, no role branch. Below `lg` the rail stacks
under the subject in DOM order, so the synopsis reads after the studio card; that is the accepted
cost of one rule rather than two.

**It stays its own block above Tags — it does not rejoin the resolved-field list.** The
`media-detail-entity-ux` reasoning that the synopsis reads as page content rather than a
data-management row still holds; only its column changed.

**Why not a viewport-conditional move.** It was specced that way first. `#field-overview` is a
deep-link anchor and the codebase already guards this hazard for `#field-actors` ("rendered in
exactly one branch below, so the id is never duplicated"), so a media-query branch cannot be a
second render — it would have to be CSS placement, collapsing the two column wrappers into one flat
grid with named areas. That restructures a primitive both detail pages share, for one block. Not
worth it; the unconditional rule costs one line of DOM order on phones instead.

## Visitor vs. owner is a control gate, not a content gate

A data point is **visible** to a visitor whenever a value exists, and **editable** for the owner.
Gate the affordance, never the value: `{#if isOwner || hasValue}`, not a bare `{#if isOwner}`
wrapped round the whole section. Visitors get values plus their `ProvenanceBadge`; owners
additionally get `SourceBadge`, the enrichment controls, writeback and the edit affordances.

Blocks that are wholly owner machinery — Manage, Completeness, File, the `Enrichment data:`
payload disclosures — are the exception and stay fully gated.

**The value's rendering sits outside the owner branch; only the affordance goes inside one.**
Co-location is not enough — the media Overview had both views five lines apart in one file and
still diverged (HOLODEX-365), because the owner branch delegated the *value* to `SourceBadge`,
which renders it at rest in its own typography (ink, unclamped, no chevron) while the visitor
branch used `ExpandableText`. So: one rendering of the value, unconditional; the owner-only
part is the pencil / badge / chip row beside it. A control that renders the value itself
(`SourceBadge` does) cannot be the owner branch of a field a visitor also sees — on a
visitor-visible block use the `SourceEditModal` pattern, which separates the decision from the
display. Long prose in particular is always `ExpandableText`
(`web/src/lib/components/shared/CLAUDE.md`).

## Page-bottom audit group

`File` and the `Enrichment data:` disclosures are one owner-only group at the very bottom, in that
order, and never separate. They sit inside `max-w-stage` so `field-grid` gets real width for the
`col-span-full` `Path:` row.

## Width scope

Which rows break `max-w-stage` and how they justify is a separate decision, recorded with the
components that implement it: [`../lib/components/video/CLAUDE.md`](../lib/components/video/CLAUDE.md).
Read it before adding any full-width row.

## Where the reasoning lives

`docs/design/media-detail-stage-layout-handoff.md` (HOLODEX-363) is the source for all of the above.
The words used above — owner/visitor branch, value vs. affordance, value-owning control, knob,
parity, drift — are defined in [`docs/reference/ui-vocabulary.md`](../../../docs/reference/ui-vocabulary.md);
use them when asking for or describing a change.
The film page had the opposite arrangement and converges on it via HOLODEX-364 — if you are reading
`films/[id]` and it still renders the description in the header, that ticket is why.
