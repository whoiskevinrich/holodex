# Manual QA Checklist: Two-Tier Field Editing Model (F56)

**Spec**: [Two-Tier Field Editing Model (F56)](../specs/two-tier-field-editing.md) · **Issue**: [HOLODEX-268](https://whoiskevinrich.atlassian.net/browse/HOLODEX-268) · **Design**: [handoff](two-tier-field-editing-handoff.md)
**Builds on**: [field-source-of-truth-qa-checklist.md](field-source-of-truth-qa-checklist.md) — F36's invariants (DB-only decisions, one batched write, file-first default, server-gate authority) are **unchanged**; this checklist covers only the new badge/expand/Confirm presentation layer.
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. Items are grouped **by verifier** — each actor runs only their section:
> - **§2 Smoke** — automated test / build gate (`svelte-check`, token-guard `rg`, unit tests). Green = pass; pre-check `[x]` with the test named.
> - **§3 Agent** — an AI agent drives the running app (DOM/ARIA/Network/computed-style). Deterministic, no human judgment.
> - **§4 Human** — needs a human's eye (per-skin "look", legibility, the at-a-glance read).
>
> §1 is one-time **setup** §3/§4 depend on. Every item is numbered `section.item` — cite the number when filing a miss.
>
> **Core invariants for every item:**
> 1. **Nothing commits without Confirm.** Clicking a chip in the expanded row must **never** issue a `PUT`/`DELETE` decision call by itself — only the Confirm button does.
> 2. **The RD6 bug is actually fixed.** Confirming a pending implicit-winner chip with no further chip click **must** create a standing decision (`decision.standing === true` on the next fetch) — this was the original no-op bug.
> 3. **Visitor parity at rest.** A visitor and an owner see byte-identical rendering for a Tier-2 field until the owner interacts (hover/click) with the badge.
> 4. **The F36 decision model is untouched.** Same API, same `sourceChips`/`resolveSelection` semantics, same writeback batching — confirm no regression against [field-source-of-truth-qa-checklist.md](field-source-of-truth-qa-checklist.md) §2–3.

---

## 1. Setup / preconditions

> **Who does this:** whoever sets up the session (developer or agent) — *not* the §4 human. **Quick "is it ready?" check:** open a Person page as owner; a Tier-2 field (e.g. Nationality) with a file/record value **and** a matched provider value shows a `ProvenanceBadge` next to the value, with **no** permanent chip row beneath it.

- [ ] 1.1 App running with a Person (or Video/Studio) record that has, on at least one Tier-2 field, an **empty baseline** and a **matched provider value** — this is the RD6-pending case §3.2/§3.3 need. A second Tier-2 field with both a baseline value **and** a differing provider value covers the ordinary multi-source case.
- [ ] 1.2 At least one Tier-1 field (Video Title, or Person/Studio name) present, to confirm it renders **unchanged** by this story (§3.7).
- [ ] 1.3 At least one single-source field (only file/record, no matched provider) present, to confirm no badge appears (§3.1).
- [ ] 1.4 Owner setup: `ADMIN_TOKEN` unset (open/owner) or set then unlocked; know how to simulate a **non-owner** for §3.8.
- [ ] 1.5 Devtools open — Console (errors), Network (watch the decision `PUT`/`DELETE`, confirm chip clicks alone never fire one).

---

## 2. Smoke — automated (green in CI)

- [ ] 2.1 **`svelte-check`** passes with the new `SourceBadge.svelte` component and its call sites on Video/Person/Studio detail pages.
- [ ] 2.2 **Token-discipline guard** empty on the changed markup: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`.
- [ ] 2.3 **RD6-confirm regression test (F56.4, the bug fix).** Unit/integration: given a field with `resolveSelection(...).pending === true`, staging that same chip and calling the component's Confirm handler issues exactly one `decide()` call for that source, and the resulting field reports `decision.standing === true`.
- [ ] 2.4 **Staged, not committed (F56.2/F56.3).** Unit: clicking a chip in the expanded row updates only local component state — zero calls to `decide`/`onedit`/the decision API — until Confirm is invoked.
- [ ] 2.5 **Cancel is a true no-op.** Unit: Cancel/Escape after staging a different chip results in zero API calls and the field's rendered value/badge unchanged from before expand.
- [ ] 2.6 **F36 regression guard.** Existing `field-source-of-truth-qa-checklist.md` §2 smoke items (resolver decision short-circuit, one-batch writeback, DB-only decision, API auth/status, sync recompute) still pass unmodified — this story touches presentation only.

---

## 3. Agent — drive the running app

**Badge presence & at-rest parity (F56.1/F56.5/F56.6)**

- [ ] 3.1 A Tier-2 field with **only one** candidate source renders the value with **no** `SourceBadge`/click target — identical DOM shape for owner and visitor.
- [ ] 3.2 A Tier-2 field with **2+** candidate sources renders `ProvenanceBadge` + value at rest for **both** owner and visitor, with no chip row visible until the owner clicks.
- [ ] 3.3 As **owner**, the badge is a `<button>` with `aria-expanded="false"`; as **visitor**, the same slot renders the existing read-only `ProvenanceBadge` `<span>`.

**Expand / stage / commit (F56.2/F56.3/F56.4 — the core mechanism)**

- [ ] 3.4 Clicking the badge sets `aria-expanded="true"` and renders a `role="radiogroup"` chip row (reusing the existing `CurationChip` radio shell) plus **Confirm** and **Cancel** controls. **No** network request fires from this click alone.
- [ ] 3.5 Clicking a non-selected chip changes its visual selection (accent border + dot) but issues **zero** `PUT`/`DELETE` calls (Network tab empty for decision endpoints).
- [ ] 3.6 **The bug fix, verified live:** on the RD6-pending field from §1.1, expand it — the pending provider chip renders with its dashed-ring/hollow-dot styling and is already staged/selected. Click **Confirm** with no other chip click. Network shows exactly **one** `PUT .../decision` for that provider. Refetch confirms `decision.standing === true` — previously (pre-F56) this exact sequence was a no-op.
- [ ] 3.7 Clicking **Cancel**, pressing **Escape**, or clicking outside the expanded row collapses it with **zero** network calls; the field's resolved value and badge are unchanged from before the expand.
- [ ] 3.8 Custom chip: opening the inline input, typing a value, and committing (Enter/blur) stages a `manual` selection **without** calling `decide` — Confirm is still required to actually commit it.

**Tier-1 fields unaffected (F56.6/F56.7)**

- [ ] 3.9 Video Title, Person/Studio name, and every image field render exactly as they did before this story — no `SourceBadge` wrapping, no behavior change.
- [ ] 3.10 A merge field (actors/genres/aliases/tags) still renders via the unchanged `CurationFieldRow` — no radio dot, no badge, `✕`-per-chip on hover as before.

**Single-expansion (F56.9, P1)**

- [ ] 3.11 With field A expanded and a chip staged (not yet confirmed), clicking field B's badge collapses A (discarding its staged selection) and expands B.

**Owner gating (invariant 3)**

- [ ] 3.12 As a **non-owner**, no `SourceBadge` is interactive anywhere on the page — every Tier-2 field renders read-only value + `ProvenanceBadge` exactly as a visitor would, matching pre-F56 non-owner rendering byte-for-byte.

**Keyboard / a11y**

- [ ] 3.13 The badge is reachable and activatable via Tab + Enter/Space. Once expanded, the chip radiogroup is roving-tabindex (Left/Right/Up/Down move focus + stage selection, matching F36's existing pattern); Tab from the last chip reaches Confirm then Cancel.
- [ ] 3.14 Escape anywhere in the expanded row collapses it and returns focus to the badge button (no keyboard trap).

---

## 4. Human — needs an eye (run in **all three skins**: Cinémathèque · Broadcast · Brutalist)

> **Nav:** open a Video, Person, and Studio detail page as **owner**, each with at least one Tier-2 field that has a matched provider. Switch skins via the header picker.

- [ ] 4.1 **At rest, it looks like the visitor view.** Load the same page as owner and as visitor side by side (or toggle) — the Tier-2 fields are visually indistinguishable until you hover/click. No permanent radiogroup clutter anywhere on the page.
- [ ] 4.2 **The click affordance is discoverable enough on hover, not sooner.** Hovering/focusing a badge reveals a subtle ring/cursor change; nothing shows before that interaction (confirms the Resolved Decision in the handoff — hover-only, not persistent).
- [ ] 4.3 **Expand reads as a natural continuation, not a jarring pop-in.** Clicking a badge should feel like the field "opens up" in place — no layout jump elsewhere on the page, chip row wraps cleanly at both `sm` and mobile widths.
- [ ] 4.4 **The RD6-pending chip is visually distinct pre-confirm.** The dashed ring / hollow dot on the pending chip reads clearly as "not yet decided" in every skin — confirm it doesn't get lost against Brutalist's high-contrast palette or blend into Cinémathèque's warm tones.
- [ ] 4.5 **Confirm/Cancel read as a clear pair, not ambiguous twins.** Confirm (`.btn-accent`) is visually the affirmative action; Cancel (`.btn-ghost`/`.btn-quiet`) reads as neutral/dismissive — no confusion about which does what, in every skin.
- [ ] 4.6 **The fix, felt.** As the owner, find a field where a provider suggestion is showing but nothing has been decided (the old bug case). Expand it, click Confirm without touching anything else, and confirm the value now shows as a real decision (e.g. reload the page — the choice persisted). This should feel obviously different from the old "nothing happens" bug.
- [ ] 4.7 **Empty/edge states themed.** A single-source field shows no badge at all; a long candidate value truncates gracefully in the expanded row; nothing overflows the card in any skin.
