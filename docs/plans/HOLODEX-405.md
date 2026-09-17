---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-405
status: in-progress
release_note: Press `e` on a detail page to refresh enrichment and `f` on a media page to open the write-to-file dialog; `?` lists the keys that work on the current page.
---

# HOLODEX-405 · Page-scoped hotkeys (F62)

Single-key hotkeys for the two owner actions used most on a detail page — `e` Refresh all,
`f` Write decisions to file — plus a `?` sheet. Built as a `use:hotkey` Svelte action **on the
button**, so the button's existence is the gate and the next hotkey is one attribute. Spec:
[docs/specs/hotkeys.md](../specs/hotkeys.md).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/hotkeys.md` (F62), RD1–RD8 locked from the 2026-09-16 brainstorm
- [~] architecture `architecture` — n/a: frontend-local action + module store; no data model or cross-cutting decision
- [ ] design `design-handoff` — the `?` sheet (layout, empty state, 8+ stressed state, dismissal) + committed SVG
- [~] backend — n/a
- [ ] frontend — `lib/actions/hotkey.svelte.ts` + listener/sheet in `+layout.svelte` + two `use:hotkey` attributes
- [ ] testing `testing-strategy` — §5 rows: registry + RD6 guard matrix (Vitest), sheet component row
- [~] security `security-review` — n/a: no auth/access/infra change; no new endpoint

## Up next — ordered (position = priority)

1. [ ] [—] `/design-handoff` for the `?` sheet — one mockup, three-skin, committed as SVG
2. [ ] [—] Implement per spec P0-1…P0-5; P1-1 (live sheet entries) if cheap
3. [ ] [—] Three-skin QA + `/code-review high --fix`; mark PR ready → Jira In Review

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · brainstorm → story → spec
- skills: product-brainstorming, write-spec
- handoff: HOLODEX-405 filed + In Progress; branch renamed `HOLODEX-405-ux-hotkeys`; spec landed,
  Draft PR open. Next is the design handoff for the `?` sheet — no code yet.
