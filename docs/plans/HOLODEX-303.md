---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-303                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Person bio now shows in the header next to the name, and edits open a clearer picker.
---

# HOLODEX-303 · Person detail: move bio into the name/photo header row

Bio moves out of the Details card and into the hero header row (third column beside name/meta,
line-clamped so it never grows the row on desktop; stacks full-width on mobile). Owner editing
switches from SourceBadge's inline chip row to a new pencil-icon → "Edit bio" modal pattern,
documented as the standard for `long_text` tier-2 fields going forward.

**Design package:** — · — · [person-detail-bio-header-handoff.md](../design/person-detail-bio-header-handoff.md) · —

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [~] spec `write-spec` → `docs/specs/**` — until: judged behavior-only (layout + interaction pattern change on an existing field), no new spec needed; revisit if scope grows
- [~] architecture `architecture` → `docs/architecture/ADR-*` — until: no cross-cutting/data-model change identified; the new modal pattern is a component convention, not an ADR-worthy decision
- [x] design `design-handoff` → `docs/design/**`
- [~] backend — until: zero-candidate resolver omission worked around in the frontend (bio-specific placeholder), general fix filed as [HOLODEX-304](https://whoiskevinrich.atlassian.net/browse/HOLODEX-304); no backend change in this diff
- [x] frontend
- [~] testing `testing-strategy` — until: no pure-logic surface added (`SourceEditModal` is markup over already-tested `f36.ts` helpers — `resolveSelection`/`sourceChips` already cover the bio/record-baseline/empty-value shapes it exercises); repo has no Svelte component-test harness (`@testing-library/svelte` not installed) to add coverage for the modal/header markup itself without a separate infra decision; verified instead via manual QA across owner/visitor, empty/short/long bio, and all three skins
- [~] security `security-review` — until: no new endpoint or auth surface; `SourceEditModal` calls the same owner-gated `decideField`/`api.setPersonFieldDecision` that `SourceBadge` already calls on this page — revisit if that changes

## Up next — ordered (position = priority)

1. [x] [frontend] Add Bio as a third hero-row column in `hero()`, with line-clamp truncation — `web/src/routes/people/[id]/+page.svelte`
2. [x] [frontend] Remove the `bio`-specific rendering from the `longFields` loop in `detail()` (Details section) — `web/src/routes/people/[id]/+page.svelte`
3. [x] [frontend] Build the "Edit bio" modal component (radio source list + inline Custom textarea, `ConfirmDialog` chrome) — `web/src/lib/components/curation/SourceEditModal.svelte`
4. [x] [frontend] Wire pencil icon (owner mode) + modal into the new Bio header column — `web/src/routes/people/[id]/+page.svelte`
5. [x] [testing] Manual QA for truncation, empty bio, modal Save/Cancel/error states, three-skin QA — no component/e2e added (repo has no Svelte component-test harness; underlying logic already unit-tested in `f36.test.ts`)
6. [ ] [—] Video `overview` adoption of the same modal pattern → HOLODEX (candidate follow-up issue, out of scope here)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-01 · code-review high --fix
- skills: code-review
- 8-angle review (5 agents) + 1-vote verification found 3 confirmed correctness bugs the
  implementation session missed, all fixed:
  - `bioField` picked `longFields[0]` unconditionally — an owner-promoted second `long_text`
    field vanished from the whole page (value, provenance, promotion edit/demote control all
    gone), and could even displace `bio` itself under the "Bio" label if `bio` had no
    resolvable value. Fixed: `bioField` now matches `canonical === 'bio'` specifically; a new
    `extraLongFields` derivation restores Details-section rendering (SourceBadge/visitor row +
    `promotedEdit`) for any other `long_text` field — `web/src/routes/people/[id]/+page.svelte`.
  - The owner's edit pencil was a trailing inline child of the `sm:line-clamp-4` div, so
    WebKit/Chromium clipped it off-screen for any bio whose text alone filled all 4 lines —
    removing the only entry point to `SourceEditModal` for exactly the long bios it exists to
    handle. Fixed: pencil moved next to the "BIO" eyebrow label (matching the approved mockup's
    layout), outside the clamped container entirely.
  - `SourceEditModal`'s `stagedKey` is seeded once at mount, but the modal doesn't remount when
    `bioField` changes reference underneath it (only `bioEditOpen` toggling mounts/unmounts) —
    an unrelated `reloadDetail()` elsewhere on the page while the modal stays open could leave
    `stagedKey` pointing at a chip no longer in `chips`, and `save()` silently no-op'd. Fixed:
    `save()` now surfaces a visible error instead of returning silently; comment corrected (the
    "mounted fresh" assumption was wrong).
  - Also fixed: redundant triple `winnerProvider(bioField)` call collapsed to one `{@const}`.
  - Skipped (documented, not blocking): hardcoded `'bio'`/`'Bio'` placeholder literal (real fix
    is the resolver-level change tracked as HOLODEX-304); `SourceEditModal`'s hand-rolled radio
    row vs. `CurationChip` (verified structurally unfit — `CurationChip`'s radio mode truncates,
    doesn't support full-width paragraph rows; matches `MergeCanonicalDialog`'s own precedent).
- verified: `npm run check` clean (0 errors) after fixes (one new type error surfaced by the
  `bioField` fix — placeholder's `display` field needed `as const` — caught and fixed same pass).
- next: push, update PR #284 with the fix commit.

### 2026-08-31 · session
- skills: testing-strategy

### 2026-09-01 · implementation landed
- skills: simplify
- impl: Bio moved into the hero row as a third column (`web/src/routes/people/[id]/+page.svelte`) —
  divider + `sm:line-clamp-4` truncation on desktop/tablet, full-width unclamped stack on mobile;
  removed from the Details card. Owner editing switched to the new `SourceEditModal.svelte`
  (pencil → modal, radio rows per candidate source + Custom textarea), documented in
  `curation/CLAUDE.md` as the standard pattern for `long_text` tier-2 fields going forward.
- bug found+fixed: a person with zero bio candidates anywhere got no `bio` entry in the API's
  `resolved[]` at all (resolver omits zero-candidate replace fields — general, pre-existing,
  entity-agnostic behavior, `internal/resolver/resolver.go` ~L313-333), so the owner silently lost
  the "Bio" label + pencil. Worked around with a frontend-only placeholder `ResolvedField`
  synthesized when nothing resolved and the viewer is owner; filed the general resolver-level fix
  as [HOLODEX-304](https://whoiskevinrich.atlassian.net/browse/HOLODEX-304) rather than expanding
  this story's scope.
- judgment call: visitors see nothing (not even the "Bio" label) when a person has no bio anywhere —
  a deliberate deviation from the handoff's literal text (which implies the label renders for
  visitors too), chosen for consistency with how every other empty field behaves app-wide; worth
  confirming with the requester.
- `/simplify` run: deduped `bioEditBtn()` render call across branches into one snippet-render site,
  extracted a shared `pencilIcon` snippet (was duplicated inline in two places), removed a redundant
  `sourceChips()` recomputation in `SourceEditModal`'s `stagedKey` initializer, fixed pre-existing
  hero-row wrapper indentation. Altitude finding (resolver-level generalization) intentionally left
  for HOLODEX-304 rather than fixed inline.
- verified: `npm run check` clean (0 errors); manual QA across owner/visitor, empty/short/long bio,
  all three skins, mobile stacking, and modal open/cancel/save/error flows.
- next: testing-strategy pass (component/e2e coverage), security-review judgment call (reuses
  existing `decideField`/`api.setPersonFieldDecision` — likely no new surface), then push + mark
  Draft PR #284 ready once those gates close.

### 2026-08-31 · design-handoff artifacts committed
- skills: design-handoff, simplify
- handoff: Mockup + handoff spec landed (`docs/design/person-detail-bio-header-mockup.svg`,
  `person-detail-bio-header-handoff.md`); design converged after 3 rounds of user review — header
  owns editing via a new pencil+modal pattern (not SourceBadge chips), bio drops from Details,
  mobile stacks full-width. Next session picks up implementation per "Up next" above.
