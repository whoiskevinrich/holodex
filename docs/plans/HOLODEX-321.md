---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-321                 # the tracker key; must match the branch key regex
status: in-review                   # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: ~
---

# HOLODEX-321 · Broken in-page anchors in the provider hand-off specs

Three `](#anchor)` links in `metadata-provider-contract.md` and `tmdb-provider.md` point at slugs no
heading generates, so they are dead on GitHub and in every Markdown renderer. These two documents are
what an **external** provider team reads with no access to the Holodex tree, so a dead
cross-reference is a navigation failure, not cosmetic. Done means a GitHub-slug checker reports zero
broken anchors across both files, with no prose or section-numbering change.

**Design package:** none — link-target corrections only. No spec, ADR, design or testing artifact is
implicated; nothing about the contract's meaning changes.

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → not needed; corrects link targets, not content
- [x] architecture `architecture` → not needed
- [x] design `design-handoff` → not needed
- [x] testing `testing-strategy` → not needed, no behaviour change
- [~] security `security-review` → not touched (no auth/access/infra change) — deferred, not applicable

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [spec] §4.1's §4.5 reference — `docs/specs/metadata-provider-contract.md`
2. [x] [spec] The `§8`→`§10` deliverable reference and the §4.3 photo reference — `docs/specs/tmdb-provider.md`
3. [ ] [—] Consider a CI link-check over `docs/**` so this class of rot is caught at PR time rather than by eye → not filed; see the note below before promoting it

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-05 · three dead anchors fixed; found by writing the checker properly the second time

- skills: (none — docs)
- Spotted while reviewing [HOLODEX-319](https://whoiskevinrich.atlassian.net/browse/HOLODEX-319).
  Reported there as **one** stale anchor and deliberately left out of that PR as unrelated to films.
- **There were three, and the first pass only found one because my slug function was wrong.** It
  stripped `_`, which GitHub keeps, so four correct references to headings like
  `### 4.6 Studio external IDs (\`_studio_external_ids\`)` reported as broken while two genuinely
  broken ones in `tmdb-provider.md` were never scanned. Rewritten to GitHub's actual rule —
  lowercase → strip markdown → keep `[a-z0-9 _-]` → spaces to hyphens — which then reports
  45 headings / 95 refs in the contract and 40 / 23 in the TMDB spec with exactly three failures.
  Worth remembering: a false positive in a link checker is not harmless, because it hides the true
  positives behind noise you learn to skip.
- The three, each a heading retitled without its inbound links following:
  - contract §4.1 → `#45-video-credits-people`, where the heading is
    `### 4.5 Video credits — per-person cast/crew with headshots`. A lone straggler — the other four
    references to that section already use the correct slug.
  - `tmdb-provider.md:264` → `[§8](#8-deliverable--operator-wiring)`. **The section number in the
    link text was wrong too**: §8 is "Reference contract examples"; the target is
    `## 10. Deliverable & operator wiring`, which line 245 already links correctly as §10. Fixing
    only the anchor would have left the visible "§8" pointing a reader at the wrong section number.
  - `tmdb-provider.md:321` → `#43-photo--profile_path--asset-url`, where the heading is
    `### 4.3 Person photos → asset URLs`.
- Three lines changed, no prose touched. Re-ran the checker: zero broken across both files.
- On Up-next #3 — a CI link check is the obvious durable fix, but it should be scoped before being
  filed. Naïvely checking every `](#…)` and relative path across `docs/**` would light up on the ADR
  corpus's historical cross-references, and a check that is red on arrival gets disabled. Whoever
  picks it up should measure the current failure count first and decide whether to gate only the two
  provider hand-off specs (the external-facing ones, where a dead link actually costs someone) rather
  than the whole tree.
- handoff: HOLODEX-321 is complete — docs only, three link targets, verified by checker. This PR
  touches only `docs/**`, so per the `jira-sync` guardrail the merge will **not** fire the `Done`
  transition; move it by hand.
