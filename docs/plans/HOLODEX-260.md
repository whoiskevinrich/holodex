---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-260                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Owner mode can now sort/filter entities by how complete their metadata is, and work a remediation queue to fill in the gaps.
---

# HOLODEX-260 · Entity Completeness Score (F55)

A per-entity completeness score (weighted, tiered by source trust) plus a separate actionability
signal, surfaced as an owner-mode browse sort/filter, a facet-first remediation queue, and a
per-entity breakdown panel — done when the owner can find and fix metadata gaps without scrolling
the library by eye. Ships as **one release**, not phased (explicit owner call during brainstorming).

**Design package:** [entity-completeness-score.md](../specs/entity-completeness-score.md) · ADR TBD · design TBD · testing-strategy TBD

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → `docs/specs/entity-completeness-score.md`
- [ ] architecture `architecture` → facet-criticality data model, tri-state not-applicable persistence, score-computation seam
- [ ] design `design-handoff` → remediation queue, breakdown panel, browse filter/sort, all three skins
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review` — new owner-gated not-applicable mutation

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [architecture] ADR: facet criticality metadata, tri-state `Standing` value (or equivalent), score/actionability computation seam — `docs/architecture/ADR-0NN-entity-completeness-score.md`
2. [ ] [backend] registry criticality metadata + tri-state facet status + score/actionability computation — `internal/registry/registry.go`, `internal/resolver/`
3. [ ] [backend] `imdb_id` → `external_provider_id` generalization + not-applicable mutation — `internal/registry/registry.go`, `internal/api/`  ⛔ blocked on #1 (ADR settles the persistence shape)
4. [ ] [frontend] browse "Completeness" sort + "Missing facet" filter chip (reuse `FacetFilter.svelte`, `SortDropdown`) — `web/src/routes/+page.svelte`, `web/src/routes/people/+page.svelte`, `web/src/routes/studios/+page.svelte`
5. [ ] [backend+frontend] facet-first remediation queue (candidate-ready vs needs-research, individual apply/search/upload) — new `web/src/routes/owner/completeness/+page.svelte`, backend predicate shared with #4
6. [ ] [frontend] per-entity completeness breakdown panel — video/person/studio detail pages
7. [ ] [testing] F55 block in `docs/testing-strategy.md`
8. [ ] [security] `/security-review` on the not-applicable mutation

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-07 · Spec written, Jira epic + stories created, branch/issue wired up
- skills: product-brainstorming, write-spec
- handoff: brainstormed the completeness-score design end to end (two-metric split —
  completeness score vs. a non-score-affecting actionability signal; tri-state
  resolved/missing/not-applicable facet status; critical/nice-to-have weighting; generalizing
  `imdb_id` to a provider-agnostic `external_provider_id`; individual-apply-only remediation
  queue per the HOLODEX-199 precedent), then wrote the full spec at
  `docs/specs/entity-completeness-score.md` (facet tables per entity type, scoring formula +
  worked 65% example, F55.1–F55.18 requirements, data/frontend/security sections, ships as one
  release per an explicit owner override of my phased-rollout recommendation). Created Jira epic
  HOLODEX-260 plus five child stories (HOLODEX-261–265: scoring engine, browse sort/filter,
  remediation queue, breakdown panel, not-applicable affordance) with `needs-adr` /
  `needs-design` / `needs-security-review` / `needs-testing-strategy` labels on the epic. Renamed
  the worktree branch to `HOLODEX-260-entity-completeness-score` and fired the Jira In Progress
  transition. Added the F55 line to `docs/architecture/README.md`'s Phase specs index. Next:
  open the Draft PR with the spec (first pre-implementation gate landing, ADR-069), then start
  the ADR — it blocks the `external_provider_id` generalization and the not-applicable
  persistence shape before any backend work.
