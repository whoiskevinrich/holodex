# ADR-088: Frontend component-reuse discipline

**Status:** Proposed
**Date:** 2026-08-25
**Deciders:** Project owner

**Relates to:** [ADR-021](ADR-021-frontend-theming-and-skins.md) (the rule file this ADR
extends carries ADR-021's theming rules today) · `web/src/lib/components/CLAUDE.md` /
`web/src/lib/components/entity/CLAUDE.md` (the existing classification + inventory system
this ADR cross-links rather than replaces). Adjacent but distinct: HOLODEX-267 (entity
decision editing overhaul, unified name-edit mechanism) — not blocking, not folded in here.
No spec — this is a discipline/tooling change, not a product-behavior change; the one concrete
UX delta (§Decision 2) routes through `/design-handoff` as an action item, not this ADR.

---

## Context

A product brainstorm (2026-08-25) started from an observation: People/Studio/Tag
relationship-editing UI looks and behaves "wildly different" between the Video detail page
and the Film detail page. Investigation narrowed the actual complaint: it isn't end-user
confusion, it's that UX/component decisions don't survive across agent sessions — a new
entity page gets built and a fresh session re-derives a bespoke interaction pattern instead of
finding and extending whatever the last page already settled on. That's a discovery/recall
failure specific to long or context-degraded sessions, not a missing concept — asked in
isolation, an agent correctly identifies that two similar components should probably share a
base.

**The premise this ADR almost got built on was wrong, and the correction is itself
informative.** The working assumption going in was "no shared component, no inventory, no
rule" — treat `StudioPicker`/`PersonPicker` as two accidental near-duplicates and merge them,
and treat the whole reuse problem as needing new documentation from scratch. Closer inspection
of the current tree overturned both halves:

- **A classification + inventory system already exists and is well-maintained.**
  `web/src/lib/components/CLAUDE.md` states a two-axis folder-classification rule
  (consumer-based when genuinely cross-entity-type, function-based otherwise, `shared/` only
  for components with zero domain knowledge) and instructs updating the folder's own
  `CLAUDE.md` table when adding a component. Each folder's `CLAUDE.md`
  (e.g. `entity/CLAUDE.md`) then documents every file with its purpose and explicit
  sibling relationships — `PersonPicker` is noted as "the multi-select sibling of
  `StudioPicker`," `EntityPicker` as "generalized from F23's `PersonPicker`," and so on. This
  is materially the inventory this ADR was about to propose inventing.
- **`StudioPicker`/`PersonPicker` are not an accidental duplicate.** Per that same
  documentation, `StudioPicker` backs a single decided field (one studio, commits through the
  decision endpoint, runs a collision/verdict flow) while `PersonPicker` backs a multi-valued
  curation list (many people, stays open across commits, requires a per-row Actor/Director
  role choice, commits through curation attach/detach, no verdict prop). Different data shape,
  different commit semantics, both composed from the shared `PickerShell` chrome — arguably
  the correct composition pattern already, not the copy-paste it looked like from the
  outside. This ADR does **not** propose merging them.

**What the correction leaves as the actual gap:**

1. `.claude/rules/frontend-theming.md` is the one file in this system that is *auto-loaded*
   by path match (`web/**/*.svelte`, `web/**/*.css`, `web/**/*.ts`) regardless of what a
   session remembers. It says nothing about checking `components/CLAUDE.md` before creating a
   new component. The inventory exists; nothing pulls a fresh or context-degraded session
   toward it. A nested `CLAUDE.md` only gets read if a session already knows to open that
   specific directory — which is exactly the discovery failure in question.
2. Tags has no entry in `entity/CLAUDE.md` at all. Tag attach/detach on the Video detail page
   is a hand-rolled inline pill-list + plain `<input>` form living directly in the route file
   (`web/src/routes/media/[id]/+page.svelte`), including a custom near-miss suggestion card —
   real, non-trivial logic with no shared home, unlike Person/Studio which each got a proper
   `entity/` component.

This ADR is scoped to those two concrete gaps. It does not re-open whether `StudioPicker`/
`PersonPicker` should be consolidated (the documented semantic difference stands unless
disproven in practice — see Consequences), does not extend to the Go backend (a deliberate,
separate scope decision), and does not build any new tooling (lint, AST-similarity checks, a
component showcase route) — all explicitly out of scope to avoid turning a two-part fix into a
platform project.

---

## Decision

### 1 — Cross-link the existing classification system from the auto-loading rule

Add one bullet to `.claude/rules/frontend-theming.md`, in the same style as its existing
tokens/buttons/skins bullets: before creating a new component, read
`web/src/lib/components/CLAUDE.md` (folder classification) and the target folder's own
`CLAUDE.md` for an existing base to extend via props/slots/composition (Svelte has no
classical inheritance); only add a new sibling when the classification doc's own criteria
say a new one is warranted (a genuinely different data shape or commit path, not "this
session doesn't remember one already exists"); update that folder's `CLAUDE.md` table in the
same change, per the rule the folder doc already states but nothing currently enforces.

This is deliberately **not** a new rule file — the existing file's title already frames itself
as "Frontend theming (component discipline)," its path trigger already matches every
`.svelte`/`.ts`/`.css` touch, and a one-bullet addition is the smallest change that closes the
signposting gap without adding a fourth file to a system that currently has three
(`frontend-theming.md`, `migrations.md`, `provider-sidecar.md`).

### 2 — Give Tags a real component, following the existing precedent

Build `TagPicker.svelte` in `web/src/lib/components/entity/` (it belongs there by the folder's
own function-based criterion — an entity-relationship-editing widget, the same reasoning that
already placed `StudioPicker`/`PersonPicker` there despite each having one caller today).
Shape it after `PersonPicker`, not `StudioPicker`: tags are multi-valued with no per-row role
requirement, closer to person-attach than studio-decide. It must preserve the existing
near-miss suggestion behavior currently embedded in the Video route — that logic is a real
feature, not scaffolding to discard. Replace the Video detail page's hand-rolled inline
form with it. Add its entry to `entity/CLAUDE.md` in the same change (per Decision 1's own
rule, applied to itself).

The component's concrete interaction spec (exact layout, states, whether/how the near-miss
card mounts) is a `/design-handoff` deliverable, not decided here — this ADR fixes the
component *boundary* (a proper `entity/` sibling, not a route-local hand-roll), not its pixel
-level design.

---

## Options Considered

### Decision 1 — where the reuse-before-duplicate instruction lives

#### A — One bullet added to the existing `frontend-theming.md` (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Minimal — one bullet, no new file |
| Consistency | High — reuses a path trigger that already covers every relevant file |
| Cost | Near zero |
| Risk | The file accumulates a second concern (styling + component reuse) alongside its original one |

**Pros:** smallest possible diff; the file's own title already claims "component discipline"
as its scope; no new moving part to maintain. **Cons:** if the file keeps growing, styling and
reuse concerns become harder to scan independently.

#### B — A new dedicated rule file (`.claude/rules/component-reuse.md`)

**Pros:** clean separation, can evolve independently of theming rules. **Cons:** a fourth rule
file for a one-bullet instruction, in a system that has kept each file to one clear topic
because there are few of them — adds a file to maintain for marginal separation-of-concerns
benefit right now. Rejected for now; revisit if Option A's file becomes unwieldy (see
Consequences).

#### C — Rely on the existing per-folder `CLAUDE.md` system unchanged, no auto-load pointer

**Pros:** zero new content anywhere. **Cons:** this is the status quo that already produced
the problem — nothing about a nested `CLAUDE.md` is discovered without already knowing to look
for it. Rejected: doesn't fix anything.

### Decision 2 — how Tags gets relationship-editing parity

#### A — Dedicated `TagPicker.svelte`, `PersonPicker`-shaped (chosen)

**Pros:** matches the existing precedent exactly (a genuinely different data shape gets its
own sibling, composed from the shared shell); preserves and properly homes the near-miss
suggestion logic; gives any future entity that needs tag-attach (e.g. Film, if that ever
becomes editable) something to reuse instead of a fourth hand-roll. **Cons:** real
implementation work, not just documentation.

#### B — Retrofit `PersonPicker` with an `entityKind` prop to also serve tags

**Pros:** one fewer file. **Cons:** `PersonPicker`'s per-row Actor/Director role requirement
is person-specific; forcing tags through the same shape means either bloating it with
tag-specific conditionals or an awkward optional-role UI for tags that don't have roles —
trading "duplicate siblings" for "one component secretly branching on entity kind," which is
the anti-pattern this ADR is trying to move away from, just relocated. Rejected.

#### C — Leave Tags hand-rolled, document why

**Pros:** zero engineering cost. **Cons:** this is the literal status quo the brainstorm
identified as broken; the near-miss suggestion logic stays orphaned in a route file with no
path to reuse. Rejected.

---

## Trade-off Analysis

**The corrected scope is smaller than the brainstorm's opening framing, and that is the
point, not a compromise.** "Build a design library" would have meant inventing documentation
that already exists in a more specific, better-maintained form than a generic library would
have produced. The actual gap — a good inventory that nothing auto-loading points at, plus one
entity type that never got its sibling built — is narrow enough to fix in one rule-file bullet
and one new component, without new tooling or a new artifact class to maintain going forward.

**Not consolidating `StudioPicker`/`PersonPicker` is a deliberate non-decision, not an
oversight.** Their documented semantic difference (single decided field vs. multi-valued
curation list with per-row roles) is a real architectural distinction, and forcing them into
one config-driven component risks recreating Decision 2 Option B's problem in reverse — one
component with two commit paths branching inside it instead of two components sharing a
shell. This ADR leaves that separation in place; if it later proves wrong in practice (e.g.
a third relationship type turns out to need the same shape as both), that's a future ADR's
job, not a silent scope addition to this one.

---

## Consequences

**What becomes easier**
- A fresh or context-degraded session touching any `.svelte` file gets pointed at the
  existing classification system automatically, instead of needing to already know it exists.
- Tags reaches parity with Person/Studio's relationship-editing pattern — three siblings
  sharing `PickerShell`, not two siblings plus one orphaned hand-roll.
- The near-miss tag-suggestion logic gets a proper, reusable home instead of being trapped in
  a route file.

**What becomes harder**
- `frontend-theming.md` now carries two related-but-distinct concerns (styling tokens, component
  reuse) under one file — acceptable at one bullet's worth of addition, worth revisiting
  (Option B) if it grows further.
- `TagPicker`'s actual interaction design is still undecided — this ADR fixes the component
  boundary, not the pixel-level design; `/design-handoff` must close that gap before or
  alongside implementation.

**What we'll need to revisit**
- Whether `StudioPicker`/`PersonPicker` should ever be consolidated — deliberately left open,
  not resolved here (see Trade-off Analysis).
- Whether the reuse-before-duplicate instruction needs to become codebase-wide (including Go)
  — explicitly out of scope now; the user's own reasoning for cutting scope here was avoiding
  a project that never finishes, so backend scope is a separate future decision, not an
  oversight.
- If `frontend-theming.md` accumulates further unrelated concerns, split component-reuse into
  its own rule file (Decision 1 Option B).

---

## Action Items

1. [ ] Add the "reuse before you create" bullet to `.claude/rules/frontend-theming.md`,
   cross-referencing `web/src/lib/components/CLAUDE.md` and per-folder `CLAUDE.md` files.
2. [ ] `/design-handoff` for `TagPicker`'s concrete interaction spec, grounded in
   `PickerShell`/`PersonPicker` as the base to extend, preserving the existing near-miss
   suggestion behavior.
3. [ ] Build `TagPicker.svelte` in `web/src/lib/components/entity/`; add its row to
   `entity/CLAUDE.md` in the same change.
4. [ ] Wire `TagPicker` into `web/src/routes/media/[id]/+page.svelte`, replacing the
   hand-rolled inline pill/input form; confirm the near-miss suggestion still surfaces.
5. [ ] Add this ADR's row to `docs/architecture/README.md`.
6. [ ] `/testing-strategy`: component-level coverage for `TagPicker` attach/detach parity with
   `PersonPicker`, and the near-miss suggestion path specifically (easy to silently drop in
   the port).
7. [ ] No new mutation surface or auth boundary is introduced — `TagPicker` reuses the Video
   detail page's existing tag attach/detach endpoints unchanged; confirm this holds once built
   and skip `/security-review` if so.
