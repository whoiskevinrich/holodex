---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-418
status: in-progress
release_note: "Re-match…" on a linked provider now always shows the candidate list instead of silently re-applying a lone strong match — the match the owner just said was wrong.
---

# HOLODEX-418 · "Re-match…" auto-applies a lone strong candidate instead of showing the picker

`EnrichPicker` ran every entity-seeded open with F47 RD1 auto-apply on, and `EnrichProviderChips`
fired the same `onenrich(p)` for a first "Enrich" and for ⋯ → "Re-match…" (RD7 relabel), so a
re-match against a provider that still returned exactly one `auto_apply` candidate applied it and
closed without a list — re-linking the match the owner was trying to replace. Reproduced on the
films testbed (media 6, Commando → `tmdb:10999`) before the fix.

## Gates — definition of done

- [x] spec `write-spec` — F47 RD1 gains a scope note (auto-apply is for an *unattended first match*
  on an unlinked provider; a Re-match never auto-applies), RD7 cross-references it, P0-2 gains the
  re-match Given/When/Then. Queue-row "Try again" decided: **unchanged** — the row isn't linked, RD1
  is the right outcome there.
- [~] architecture `architecture` — n/a: no data-model / API change; ADR-066 D1's threshold is
  untouched, only *when* the client asks for it
- [x] design `design-handoff` — `enrichment-review-workflow-handoff.md` §3 chip table, the
  interaction row and a new QA item **3.5a**; no visual change, no mockup needed
- [x] frontend — `EnrichPicker` takes `autoApply` (default `true`); chips' Re-match fires
  `onenrich(p, { rematch: true })`; the four detail pages share an `openPicker(p, opts)` that sets
  `pickerRematch` (also the Refresh-all needs-review hand-off, so the flag never lingers) and pass
  `autoApply={!pickerRematch}`; `EnrichQueueRow` unchanged. The RD1 decision is extracted to the
  pure `autoApplyPick(candidates, allowed)` in `web/src/lib/autoApply.ts`
- [x] testing `testing-strategy` — `autoApply.test.ts` (lone strong picks; weak siblings don't
  block; zero / 2+ strong fall through; disallowed never picks); auto-apply routing row updated in
  `docs/testing-strategy.md`; live: first Enrich still auto-applies (resolve + enrich), Re-match →
  resolve only, dialog open with "Commando · Strong match", nothing applied until clicked (then one
  enrich, dialog closes); no console errors. Skins n/a — no styling touched
- [~] security `security-review` — n/a
- [x] `code-review high --fix` — no findings

## Up next — ordered (position = priority)

1. [ ] [—] On merge, HOLODEX-418 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · diagnosed, fixed, verified
- skills: code-review high --fix (no findings)
- Branch renamed to the key, Jira → In Progress. graphify + targeted reads located the two
  components and the four page call sites; fix shaped exactly as the ticket proposed.
- handoff: fix + spec/handoff/testing edits shipped; PR opened ready for review (all gates green or
  n/a); nothing pending but the merge.
