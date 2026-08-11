---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-269                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
---

# HOLODEX-269 · Unified name-edit mechanism across Video Title, Person, Studio, and Tag

One shared component — docked-pencil affordance + pluggable collision/merge-offer check —
replacing three currently-divergent rename implementations (Person's `SourceSelect`/`onadopt`
intercept, Studio's `AliasPanel`, Tag's list-page rename/merge) plus adding a rename affordance
where none exists today (Video Title). Person/Studio/Tag collision = identity-spine lookup with
a keep-separate/merge-into-existing verdict (reusing the ADR-061 merge primitive); Video Title's
collision surface is the separate composite-key story, HOLODEX-270.

**Design package:** spec [unified-name-edit.md](../specs/unified-name-edit.md) · design [unified-name-edit-handoff.md](../design/unified-name-edit-handoff.md) · `docs/testing-strategy.md` TBD

## Gates — definition of done

- [x] spec `write-spec` → [docs/specs/unified-name-edit.md](../specs/unified-name-edit.md)
- [~] architecture `architecture` — not applicable; no new ADR. ADR-061 D7 (tags don't get the
  field-decision model) is untouched; the "no detail page" line in its spec twin (RD7) was
  already stale before this story (Phase 1 shipped `tags/[id]`, HOLODEX-259 expanded it) and
  needed no edit. Tag's detail page, `GET /tags/{id}`, and the merge/rename primitives all
  already exist — this story is a consumer-side frontend consolidation, no new data model or seam
  (same "no new ADR required" call HOLODEX-259 made for the same page)
- [x] design `design-handoff` → [docs/design/unified-name-edit-handoff.md](../design/unified-name-edit-handoff.md)
- [~] backend — no new Go code needed. Audit confirmed `POST /{people|studios|tags}/{id}/rename`
  already return byte-identical `{error, conflict: <entity>}` on 409 / empty `204` on success
  (`entity_identity.go:237-247`, `person_decisions.go:220-229`), all behind the same
  `requireOwner` gate. Video Title's `PUT /media/{id}/fields/title/decision` has no
  collision branch at all, matching the spec's no-op-checker plan. One bug found (frontend-only):
  `renamePerson` in `web/src/lib/api.ts:938-959` doesn't unwrap `body.conflict`, unlike the generic
  `renameEntity`, masked by an incorrectly-mocked unit test — fixed by retiring it in favor of
  `renameEntity('person', ...)` during the frontend gate (parity requirement, not scope creep)
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review` — rename/merge mutations, likely required

## Up next — ordered (position = priority)

1. [x] [spec] `/write-spec` for HOLODEX-269 — `docs/specs/unified-name-edit.md`
2. [x] [design] `/design-handoff` for the docked-pencil + verdict panel — `docs/design/unified-name-edit-handoff.md`
3. [x] [backend] audited existing rename/merge/collision endpoints — no new Go code needed (see gate note)
4. [ ] [frontend] `MergeOfferCard` + `NameEditControl` + per-entity wiring (Person, Studio, Tag, Video Title), retire buggy `renamePerson` — `web/src/lib/components/entity/`, `web/src/lib/api.ts`
5. [ ] [testing] `/testing-strategy` update
6. [ ] [security] `/security-review`
7. [x] [—] push Draft PR now that the spec (first gate artifact) has landed; sync Jira first — [#229](https://github.com/whoiskevinrich/holodex/pull/229)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-10 · Started HOLODEX-269, worktree + Jira set up, research dispatched
- skills: (none yet — spec pass next), design-handoff, graphify
- handoff: fresh worktree `HOLODEX-269-name-edit-mechanism` branched off origin/main (which already
  includes the merged HOLODEX-273 fix). Jira transitioned to In Progress. Dispatched an Explore
  agent to ground the spec in the four current rename implementations (Person/Studio/Tag/Video
  Title) plus the ADR-061 merge primitive and identity-spine lookup before running `/write-spec`.

### 2026-08-10 · Wrote the spec, then caught and fixed a wrong premise in it
- skills: write-spec
- handoff: `/write-spec` produced `docs/specs/unified-name-edit.md`. Its first draft claimed Tag
  had no detail page and needed one built, reversing F43's RD7 non-goal
  (`docs/specs/entity-identity.md`) — asked the user via `AskUserQuestion`, who chose to build the
  page. Before touching entity-identity.md, went back to verify the "no detail page" premise
  directly and found it false: `tags/[id]/+page.svelte` has existed since Phase 1 and was
  substantially expanded 3 days ago (HOLODEX-259, PR #221 — parent/children/categories/writeback/
  sync). The real gap is just that page having no rename/merge control, not a missing page.
  Reverted the entity-identity.md edits (RD7 needed no change — it was never actually about page
  existence in a load-bearing way) and rewrote unified-name-edit.md's Existing State/Goals/
  Non-Goals/Resolved Decisions/Requirements/Open Questions to drop the "build a page" framing in
  favor of "add `NameEditControl` to the existing header." Net effect: smaller scope than
  originally spec'd, and the user's approval is superseded by this factual correction rather than
  contradicted (the pencil docks on the page they already have). Architecture gate confirmed N/A —
  no new ADR needed, same call HOLODEX-259 made for the same page.
  Next: `/design-handoff` for the shared `NameEditControl` + `MergeOfferCard` components, then
  push a Draft PR (spec is the first landed gate, per ADR-069).

### 2026-08-10 · Design handoff landed
- skills: design-handoff
- handoff: wrote `docs/design/unified-name-edit-handoff.md`, grounded in the real `AliasPanel.svelte`
  (`web/src/lib/components/person/AliasPanel.svelte` — corrected from a wrong `entity/` path
  assumed in the skill prompt), the always-visible pencil precedent on `categories/[id]`, and the
  existing `.curation-actions` hover/focus-reveal CSS mechanism in `app.css` (recommended as a
  pattern to mirror with new, non-chip-specific class names rather than reusing the literal
  `.curation-chip`/`.curation-actions` classes on a header row). Resolved the spec's one open
  question: `NameEditControl`'s conflict state is a `verdict` snippet prop, not a sibling
  component, so Video Title can omit it cleanly while Person/Studio/Tag all wire the same
  `MergeOfferCard`. `MergeOfferCard` extracted contract matches `AliasPanel`'s existing
  `conflict`/`onmerge`/`onkeepseparate` shape exactly — no behavior change, just relocation.
  Next: push a Draft PR (spec is the first landed gate, per ADR-069) — sync Jira first, then
  backend/frontend implementation.

### 2026-08-10 · Backend audit: no new Go code needed
- skills: (none — direct research + worklog update)
- handoff: dispatched an Explore agent to map every rename/merge/collision/near-miss endpoint
  across Person, Studio, Tag, and Video Title before writing anything. Confirmed the spec's claim
  precisely: Person/Studio/Tag rename endpoints return byte-identical JSON shapes on both success
  (204) and collision (409, `{error, conflict: <entity>}`), all gated by the same `requireOwner`
  middleware; merge only differs in its response-wrapper key (`person`/`studio`/`tag`); near-miss
  is legitimately studio/tag-only (route omission, not a Go-level type restriction); Video Title's
  field-decision endpoint has no collision branch at all, matching the planned no-op checker.
  Found one real, preexisting bug while there: the frontend's dedicated `renamePerson` (api.ts)
  doesn't unwrap `body.conflict` the way the generic `renameEntity` does, masked by a unit test
  that mocks the wrong response shape. Marked the backend gate `[~]` — no Go changes required.
  Next: frontend — extract `MergeOfferCard`, build `NameEditControl`, wire into all four pages,
  retire `renamePerson` in favor of `renameEntity` (fixes the bug as a parity side effect).
