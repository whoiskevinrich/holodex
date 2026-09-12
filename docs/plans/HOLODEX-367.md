---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-367
status: in-progress
release_note: Providers that ask for it now receive the video's resolved fields and its raw filename alongside the search query, so they can match a release directly and fall back to a performers + studio search when the title misses; the search query no longer repeats the studio and cast when the title is only those words; and the picker can show what a provider actually searched.
---

# HOLODEX-367 · Structured resolve hints (ADR-080 D1 revisit)

ADR-080 D1 kept `/resolve`'s hint one flattened string and said: revisit when a second provider
wants finer control. The partner video provider brought the evidence on 2026-09-11 — 6 of 9
zero-hit resolves are reachable with a performers + studio query the undelimited blob cannot
express, and the raw bracketed basename matches 3/3 through the upstream's filename matcher where
free-text gets 0/3. Design converged over three rounds; the epic is ADR + contract text + two
stories. HOLODEX-370 (see and undo a written-back wrong match) landed first on purpose.

**Decisions that were mine, not the ticket's** (flag on review): a residue-dropped `{title}` is
*rendered-empty*, not *missing* — it must not fail a required token through to the sanitized-title
floor, which would resend exactly the duplication the rule prevents (ADR-095 D5). `hint.fields` is
`/enrich`-shaped (`{key: [values]}`) so the vocabulary is one the provider already speaks (D2).
The operator deny is a per-source boolean sketched as `send_filename: false`; the name is the
story's to settle.

## Gates — definition of done

- [x] spec `write-spec` — contract text (`metadata-provider-contract.md` §2.2 row + example, §2.3
  three `hint.*` rows + video example + `searched[]`, §4.9 residue row, **new §4.10** deep-dive
  with the opt-in table / miss definition / provider obligations, §5 caps row) **and** the F54 spec
  amendment (FR6 residue rule incl. the honest all-or-nothing example, FR7 `query_source`, FR8
  structured hints on both paths, FR9 "Searched: …" caption replacing P1-a; AC-12–18; AC-10
  narrowed; P2-a promoted; test notes per FR)
- [/] architecture `architecture` — ADR-095 drafted (D1–D8), indexed in `README.md`; number was
  reserved via `adr-claims.mjs` before drafting
- [ ] design `design-handoff` — the picker caption (HOLODEX-369): one muted line under the input,
  stressed state at 10 entries, three skins, SVG committed
- [ ] backend — HOLODEX-368: `Manifest.ResolveHints`, `Source` deny flag, `Hint{Fields, Filename,
  QuerySource}`, residue rule in `query.go`, `query_source` derived in `enrichVideoResolve`,
  per-provider hints inside `refreshOneProvider`, `searched[]` decoded → `job_runs.detail`
- [ ] frontend — HOLODEX-369: caption from `searched[]`
- [ ] testing `testing-strategy` — residue lossless invariant + rendered-empty-not-missing;
  `query_source` race → `"user"`; byte-identical request for a provider without `resolve_hints`
  (golden); deny flag; `Base()` only; batch per-provider; `searched[]` caps
- [ ] security `security-review` — raw basename leaves the box for opted-in providers: opt-in +
  deny layering, `filepath.Base` only, `fields` ⊆ what the blob already sent, `searched[]` ingest
  sanitized/capped, `job_runs.detail` no-path invariant

## Up next — ordered (position = priority)

1. [x] [M] Contract text — §2.2 / §2.3 / §4.9 / new §4.10 / §5, in PR #327.
2. [x] [S] F54 spec edit — FR6–FR9, AC-12–18, in PR #327.
3. [ ] [M] `/design-handoff` for HOLODEX-369 (gate 3) — can run in parallel with 368.
4. [ ] [—] When the PR is marked ready: sweep HOLODEX-368/369 with the epic (CI moves only the
   branch's key).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-11 · epic refreshed from the filename probe, ADR-095 drafted
- skills: architecture
- Folded the provider's filename-matcher probe into the epic (verbatim basename, default-allow
  load-bearing, batch path too), marked HOLODEX-370 landed, fired In Progress, branched off main
  as `HOLODEX-367-structured-resolve-hints`. Drafted ADR-095 (D1–D8) and the README row.
- Handoff: ADR-095 is on the branch in Draft PR #327; contract text (§2.2/2.3/4.9/5) is the next
  thing to write, in the same PR.

### 2026-09-11 · contract text written
- skills: —
- Wrote the provider-contract text for ADR-095: rows in §2.2/§2.3/§4.9/§5 and a new §4.10
  deep-dive in the §4.7–4.9 house style (a §4.10 was my call — the miss definition and the
  provider's obligations wanted a home, and §5 is a caps table). Ticked ADR-095 AI 1–2.
- Handoff: PR #327 now carries ADR + contract text; next is the F54 spec edit (caption slot →
  "Searched: …", ACs for `query_source` + residue), then `/design-handoff` for HOLODEX-369.

### 2026-09-11 · F54 spec amended
- skills: write-spec (by hand — the spec exists; amended in place rather than a new doc)
- Added FR6–FR9 + AC-12–18 to F54, narrowed AC-10, struck the superseded Non-Goal / P1-a / P2-a
  with pointers. Caught my own wrong example while writing: the residue rule is all-or-nothing, so
  a title *with* residue keeps its duplication — the spec now shows that render honestly and says
  why (decomposition is FR8's job, not a cleverer blob). Spec gate closed.
- Handoff: PR #327 = ADR-095 + contract §2.2/2.3/4.9/4.10/5 + F54 FR6–FR9. Next:
  `/design-handoff` for HOLODEX-369 (caption), then testing strategy + security review, then
  mark ready. Backend story HOLODEX-368 can start any time — its spec is complete.
