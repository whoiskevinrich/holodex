# Rollout plan: Per-field source-of-truth (F36 / ADR-051)

**Living coordination doc** — update the graph + table as sessions complete (see [Update protocol](#update-protocol)).
**Design package**: [spec F36](../specs/field-source-of-truth.md) · [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md) · [handoff](../design/field-source-of-truth-handoff.md) · [QA checklist](../design/field-source-of-truth-qa-checklist.md) · [testing-strategy §9](../testing-strategy.md). Design is **complete**; this plan sequences the **implementation** across sessions.

> **Highest-leverage decision (baked in):** do **S0 BaselineSource** first and build the resolver on it
> *video-first but entity-generic* — that realizes fast-follow **① (entity-agnostic resolver)** as a
> byproduct, so People (②) and Studio (③) are unblocked the moment F36 merges, with no resolver rework.

## Session graph

```mermaid
flowchart TB
  subgraph CP["Critical path"]
    direction LR
    S0["S0 · BaselineSource<br/>architecture addendum"] --> S1["S1 · F36 backend<br/>migration 0016 · resolver · API<br/>(realizes ① resolver)"]
    S1 --> S3["S3 · review + QA<br/>/security-review · 3 skins"]
    S3 --> M(("F36<br/>merged"))
  end
  subgraph PAR["In parallel — freeze the API/types contract first"]
    S2["S2 · F36 frontend<br/>SourceSelect · starts now"]
  end
  S2 -. joins .-> S3
  subgraph FF["Fast-follows after merge — ② &amp; ③ need ① (delivered by S1)"]
    direction LR
    S4["S4 · multi-provider<br/>P1 · optional"]
    S5["S5 · People ②<br/>person pages"]
    S6["S6 · Studio ③<br/>new entity · parallel w/ S5"]
  end
  M --> S4
  M --> S5
  M --> S6

  classDef done fill:#1d9e75,stroke:#0f6e56,color:#ffffff;
  classDef active fill:#378add,stroke:#185fa5,color:#ffffff;
  classDef pending fill:#f1efe8,stroke:#888780,color:#2c2c2a;
  classDef gate fill:#faeeda,stroke:#ba7517,color:#412402;

  class S0,S1,S3,S4,S5,S6 pending;
  class S2 active;
  class M gate;
```

**Legend** — `pending` (grey) · `active` (blue, in progress) · `done` (green, merged) · `gate` (amber, the merge milestone).
**Flow:** critical path `S0 → S1 → S3 → merge`; `S2` runs in parallel and joins at review; `S4/S5/S6` are post-merge, with `S5`/`S6` parallel to each other.

## Sessions

| ID | Scope | Depends on | Parallel with | Size | PR | Status |
|----|-------|-----------|---------------|------|----|--------|
| **S0** | `/architecture` addendum: pin the `BaselineSource` contract (video impl reads file cols + `video_metadata`); video-first but entity-generic | — | — | XS | fold into S1's PR | ☐ not started |
| **S1** | F36 backend: migration 0016 `field_source_decisions`, repo CRUD, resolver decision short-circuit + file-first default + `default_source` config, `ResolvedField.decision` + sync compute, owner-gated `PUT/DELETE …/decision`, writeback writes decided value (one `WriteBatch`/file) | S0 | S2 | L | PR (backend) | ☐ not started |
| **S2** | F36 frontend: `SourceSelect` radiogroup, wire into replace fields, candidates, sync indicators, "Write decisions to file" rename, 3-skin QA | frozen API/types contract (spec + handoff) | S1 | M | [#71](https://github.com/whoiskevinrich/holodex/pull/71) (draft) | ◐ in review (HOLODEX-7 — `SourceSelect.svelte`, `f36.ts`, typed payload in `types.ts`, dev mock `f36mock.ts`; svelte-check + token-guard + unit tests green; joins S1 at S3) |
| **S3** | Integration + `/security-review` (owner gate, untrusted `manual_value`) + execute the QA checklist (smoke/agent/human-3-skin) | S1 + S2 | — | M | gates the merge | ☐ not started |
| **S4** | Multi-provider (P1): inter-provider trust order config, one `Adopt` per matched provider, "providers differ" hint | F36 merged | S5/S6 | M | fast-follow | ☐ not started |
| **S5** | ② People refactor: person detail through resolver + curation + decisions | ① (from S1) | S6 | L | fast-follow | ☐ not started |
| **S6** | ③ Studio entity: table, scan resolve-or-create, FTS, page, facet → inherits decisions | ① (from S1) | S5 | L | fast-follow | ☐ not started |

## Why this sequence (speed · tokens · progress)

- **Speed** — S2 (frontend) starts immediately, parallel to S1: the API shape and `ResolvedField.decision` / `in_sync` payload are already frozen in the spec + handoff, so `SourceSelect` builds against a typed stub. Frontend and backend touch **disjoint files** → no merge contention. Post-merge, S5 + S6 run in parallel.
- **Token utilization** — each session is one-PR-sized and uses a **doc section as its contract** instead of re-deriving: S1 → spec P0-1…5 + testing-strategy §9; S2 → handoff + QA §3/§4; S3 → the QA checklist verbatim. Use **separate worktrees** for parallel sessions (S1/S2, later S5/S6) so none reloads another's churn. `/security-review` runs **once** (S3, backend diff only).
- **Progress** — folding ① into S1 means the F36 merge instantly unblocks ② and ③; the resolver refactor is paid for once.

## The S0/S1 fork
**Pure video-first** (hardcode `*model.Video`) is marginally faster for S1 alone but forces ②/③ to reopen `resolver.Resolve`. **Fold-in-① (recommended):** S0 defines `BaselineSource` as the seam; S1 implements only the *video* `BaselineSource` — ~same cost, but the resolver comes out entity-generic and ① is done. Net faster across the whole plan.

## Launch-ready session briefs

- **S0 + S1:** "Implement F36 backend per `docs/specs/field-source-of-truth.md` (P0-1…P0-8) and `docs/architecture/ADR-051` §2–5. Start with a short ADR addendum pinning a `BaselineSource` contract (video-first but entity-generic), then: migration 0016 `field_source_decisions`, repo CRUD, resolver decision short-circuit + file-first default + `default_source` config, `ResolvedField.decision` + per-field sync, owner-gated `PUT/DELETE /media/{id}/fields/{canonical}/decision`, writeback writes the decided value. Satisfy testing-strategy §9 F36 rows — especially the **one-`WriteBatch`-per-file** assertion and decisions-are-DB-only. Honor the atomic-batched-writes constraint. New worktree."
- **S2 (parallel):** "Build the F36 frontend per `docs/design/field-source-of-truth-handoff.md`: the `SourceSelect` radiogroup on replace fields, candidates, per-field + section sync indicators, 'Write decisions to file' rename. Build against the typed payload in the spec (`ResolvedField.decision`, `in_sync`) + the decision endpoints; mock the API until S1 lands. Pass QA §3/§4, tokens-only, 3 skins. Separate worktree from the backend."
- **S3 (gate):** "Run `/security-review` on the F36 backend diff (owner gate, untrusted `manual_value` → file write) and execute `docs/design/field-source-of-truth-qa-checklist.md` §2–§4."

## Update protocol

When a session changes state, update **both** the graph and the table here:

1. **Graph** — move the node's id between the `class … done|active|pending` lines at the bottom of the Mermaid block. E.g. when S1 starts: move `S1` to the `active` class; when its PR merges: move it to `done`. Flip `M` (`F36 merged`) to `done` once S3 merges F36.
2. **Table** — set the Status cell (`☐ not started` → `◐ in progress` → `☑ done`) and drop the PR link in the PR column.
3. **Briefs** — strike a brief once its session has merged; add the actual PR URL.

Keep this file as the single source of truth for *where the rollout stands*; the spec/ADR/handoff/QA stay the source of truth for *what to build*.
