# UI Vocabulary

Shared words for talking about owner/visitor rendering, field controls, and the ways they
drift apart. Use these when asking for a change so the request names the mechanism, not just
the symptom — "owner and visitor should have parity on the bio" says what to check; "make the
bio look the same" does not.

Each term links to where it is load-bearing. If a term here and the rule it points at ever
disagree, the rule wins — fix the entry.

## Audiences and branches

**Owner / visitor.** The two audiences every detail page serves. A *visitor* sees a data point
whenever a value exists; the *owner* additionally gets the controls to change it. The gate is
`{#if isOwner || hasValue}`, never a bare `{#if isOwner}` around content.
→ [`web/src/routes/CLAUDE.md`](../../web/src/routes/CLAUDE.md) § "Visitor vs. owner is a
control gate, not a content gate"

**Owner branch / visitor branch.** The two arms of an `{#if isOwner}`. The rule: the *value's
rendering sits outside the owner branch*; only chrome goes inside one.
→ same section

**Parity.** Same field, same computed rendering — font, line-height, colour, clamp — for both
audiences. A testable claim, not a vibe: the harness assertion is
[HOLODEX-366](https://whoiskevinrich.atlassian.net/browse/HOLODEX-366).

**Drift.** Two renderings of one thing that were once the same and quietly stopped being.
Co-location does **not** prevent it — the media Overview had owner and visitor five lines apart
in one file and still diverged (HOLODEX-365). What prevents it is one rendering of the value,
unconditional.

**Deliberate split vs. drift.** Before "fixing" a divergence, look for the written decision. The
film page's description was a deliberate split (visitor in the header, owner in Details, reasoned
in a comment) that *became* drift the moment the sibling media page decided the opposite
(HOLODEX-363), and was converged in HOLODEX-364.

## Content and chrome

**Value vs. affordance** (industry: *content vs. chrome*). The value is what the field says; the
affordance is the control that lets the owner change it — pencil, badge, chip row, picker.
"Chrome" is the furniture around the content, as in "browser chrome". *Gate the affordance,
never the value.*
→ [`web/src/routes/CLAUDE.md`](../../web/src/routes/CLAUDE.md)

**Value-owning control.** A control that renders the value itself at rest — `SourceBadge` does
(value span + `ProvenanceBadge`, then the chip row on click). By construction it gives the owner
a private typography, so it cannot be the owner branch of anything a visitor also sees. Fine on
the Metadata list, where both audiences see the same rows.
→ [`web/src/lib/components/curation/CLAUDE.md`](../../web/src/lib/components/curation/CLAUDE.md)
`SourceBadge` row

**Decision / display separation.** The opposite shape: the value renders once, unconditionally,
and the decision control is a separate thing a pencil opens — `SourceEditModal`. Use it on any
visitor-visible block.
→ `curation/CLAUDE.md` `SourceEditModal` row;
[person-detail-bio-header-handoff.md](../design/person-detail-bio-header-handoff.md)

**Knob.** A styling prop that makes a rule opt-in per call site. `ExpandableText` had a `tone`
knob defaulting to ink; one of three call sites set it, so "muted prose" was true on one page.
*Enforcement by subtraction*: a component with no knob **is** the rule; one with a knob is a
suggestion. Layout-fit props (`lines` 4/5) are not knobs — they answer "how much space is
there", not "what does this look like".
→ [`web/src/lib/components/shared/CLAUDE.md`](../../web/src/lib/components/shared/CLAUDE.md)
`ExpandableText` row

**Long prose** / `long_text`. `long_text` is the field type in the mapping registry
([canonical-fields.md](canonical-fields.md)); *long prose* is the UX word for what it looks
like — bio, overview, description, comments. One look, everywhere, for everyone: muted
`text-sm leading-relaxed`, clamped, with the expand chevron. It is `ExpandableText`, no
exceptions.
→ `shared/CLAUDE.md`

## Control patterns (by name)

**Chip row.** `SourceBadge`'s inline expansion: a roving-tabindex radio row of `CurationChip`s
with staged selection + Confirm/Cancel. The Tier-2 default for scalar fields.
→ [two-tier-field-editing-handoff.md](../design/two-tier-field-editing-handoff.md)

**Pencil + modal.** A pencil in the field's heading opens `SourceEditModal` — one full-width
radio row per candidate source plus a Custom textarea. The pattern for `long_text` (Person bio,
media Overview).
→ [person-detail-bio-header-handoff.md](../design/person-detail-bio-header-handoff.md)

**Docked pencil.** `NameEditControl`: at rest identical to the visitor's view; on owner
hover/focus a low-opacity pencil brightens and opens an inline edit. Page headings, the Video
title, the Film year.
→ [`web/src/lib/components/entity/CLAUDE.md`](../../web/src/lib/components/entity/CLAUDE.md)
`NameEditControl` row; [unified-name-edit-handoff.md](../design/unified-name-edit-handoff.md)

**Deep-link anchor.** `id="field-<canonical>"` on the block that renders a field, so the
completeness queue can jump to it. Must be unique on the page and must exist whenever the queue
could point at it — a viewport-keyed second render is a bug, not a layout choice.
→ `media/[id]/+page.svelte` `hasPageAnchor`; HOLODEX-363 worklog

## Field model

**Tier-1 / Tier-2 field.** Tier-1 = identity fields with their own flow and collision handling
(name/title via `NameEditControl`, studio via `StudioPicker`, people via `PersonPicker`).
Tier-2 = every other replace field, handled by the standard decision control (chip row, or
pencil + modal for `long_text`).
→ [two-tier-field-editing-handoff.md](../design/two-tier-field-editing-handoff.md);
[ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)

**Adoption vs. precedence.** ADR-090's two layers. *Adoption*: should this candidate enter the
shadow store at all (transient, review queues). *Precedence*: which stored source wins this
field (standing, the decision controls above). Every control in this document is a precedence
control. Never put a competing provider value in an adoption row.
→ [ADR-090](../architecture/ADR-090-two-layer-entity-metadata-management.md)

**Baseline.** The entity's own record — the file layer for videos (`baselineKey='file'`), the
row for persons/studios (`'record'`). Enrichment is an additive shadow over it; the resolver is
the only merge point.
→ [ADR-033](../architecture/ADR-033-metadata-source-plugins.md), ADR-051

## Translations

Plain phrasings the owner has used, and the term each landed on. Append here whenever a request
is translated into a term (`.claude/rules/ui-vocabulary.md`, translate / link / record);
the phrasing is kept as said so the next reading of it is consistent.

| Owner said | Term | What that changes about the work |
|---|---|---|
| "add a way for the component to be configured so the text can be more of a grey than white" | **a knob for tone** | it is a styling prop, so the question is whether the look should be the component's rule instead — `ExpandableText` had exactly this knob and lost it |
| "the owner view and the visitor view should look the same" | **parity** | check computed typography of the value, not which component is named; the harness assertion is HOLODEX-366 |
| "include the collapsible chevron" | **long prose** (`ExpandableText`) | the chevron is not a feature to add, it is part of the one rendering long prose gets |
| "are the owner and visitor view using the same component now?" | **decision / display separation** | the answer is about *where the value renders*, not which control the owner has |
| "co-locate owner and visitor views in the same file to reduce divergence" | **drift**, and the *owner branch* rule | co-location does not prevent drift; one unconditional rendering of the value does |
| "a reuse mechanism for long prose — biographies, comments, descriptions" | **long prose** + **enforcement by subtraction** | the mechanism existed; it had a knob — the fix is removing the knob, not adding a component |

## Saying it

| You say | It means |
|---|---|
| "Owner and visitor should have parity on X" | check computed typography of the value, not just which component is named |
| "That's a value-owning control on a visitor-visible block" | the fix is decision/display separation, not restyling the control |
| "Don't add a knob for that" | bake the rule into the component; no prop |
| "Gate the chrome, not the content" | only the affordance goes inside `{#if isOwner}` |
| "Is this drift or a deliberate split?" | find the written decision before fixing it |
| "Long prose" | `ExpandableText`, muted, clamped, chevron — end of discussion |
