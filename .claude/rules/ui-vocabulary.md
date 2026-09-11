---
paths:
  - "web/**/*.svelte"
  - "web/**/*.css"
  - "web/**/*.ts"
---

# UI vocabulary: translate, link, record

The shared words for owner/visitor rendering, field controls, and the ways they drift live in
[`docs/reference/ui-vocabulary.md`](../../docs/reference/ui-vocabulary.md) — owner/visitor
branch, value vs. affordance (chrome vs. content), value-owning control, knob, parity, drift,
long prose, chip row / pencil + modal / docked pencil, Tier-1/Tier-2, adoption vs. precedence.
Use them when describing a change, and read the doc before coining a new one.

The owner describes changes in plain words; the doc holds the mechanism names. When a request
maps onto a term — "add a way for the component to be configured so the text can be more of a
grey than white" is *a knob for tone* — do three things, in the same turn:

1. **Name the term** in the reply, so the translation is visible and can be corrected before any
   code moves. Do not silently act on the mechanism you inferred.
2. **Link it.** Any UX term from the doc that appears in a reply carries a link to
   `docs/reference/ui-vocabulary.md` (deep-link the section when there is one) the first time it
   is used in that reply, so the owner can check the meaning rather than infer it from context.
3. **Record the translation.** If the plain phrasing is not already in the doc's *Translations*
   table, add it — the phrasing the owner actually used, the term it landed on, and what that
   changes about the work — in the same change as the code. A translation that only happened in
   chat is lost with the session. Same rule when a request needs a term the doc does not have
   yet: coin it, define it, add it, and say so.

This is the mechanism by which the vocabulary stays the owner's rather than the agent's: every
entry traces to a phrasing the owner said.
