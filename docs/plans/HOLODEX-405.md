---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-405
status: in-progress
profile: feature
release_note: Press `e` on a detail page to refresh enrichment and `f` on a media page to open the write-to-file dialog; `?` lists the keys that work on the current page.
---

# HOLODEX-405 · Page-scoped hotkeys (F62)

Single-key hotkeys for the two owner actions used most on a detail page — `e` Refresh all,
`f` Write decisions to file — plus a `?` sheet. Built as a `use:hotkey` Svelte action **on the
button**, so the button's existence is the gate and the next hotkey is one attribute. Spec:
[docs/specs/hotkeys.md](../specs/hotkeys.md).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/hotkeys.md` (F62), RD1–RD8 locked from the 2026-09-16 brainstorm
- [x] design `design-handoff` — `docs/design/hotkeys-handoff.md` + `hotkeys-sheet.svg`: centered modal (A) on the ConfirmDialog surface, empty + 9-key two-column states, numbered three-skin QA
- [~] backend — n/a
- [x] frontend — `lib/actions/hotkey.svelte.ts` (action + registry + guard) · `shared/HotkeySheet.svelte` · listener in `+layout.svelte` · `use:hotkey={'e'}` on Refresh all, `use:hotkey={'f'}` on media writeback; P1-1 clickable rows included
- [x] testing `testing-strategy` — §5 rows added; `hotkey.test.ts` (14 tests: registry, RD6 guard matrix, fire order); browser QA per handoff §QA on backend-films (three skins via computed tokens, empty + 9-key states, video/dialog guards, launcher rows)

## Up next — ordered (position = priority)

1. [x] [—] `/design-handoff` for the `?` sheet — one mockup, three-skin, committed as SVG
2. [x] [—] Implement per spec P0-1…P0-5; P1-1 (live sheet entries) if cheap
3. [x] [—] Three-skin QA + `/code-review high --fix` (2 findings, both fixed: sheet Esc leaked to browse clearAll; row launcher focus-restore)
4. [ ] [—] Mark PR #343 ready → CI fires Jira In Review; human pass of handoff §QA 3.1–3.3 on your prod skins

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · brainstorm → story → spec
- skills: product-brainstorming, write-spec, design-handoff, code-review
- handoff: HOLODEX-405 filed + In Progress; branch renamed `HOLODEX-405-ux-hotkeys`; spec + design
  handoff landed (sheet = centered modal, A), Draft PR #343 open. Next is the code — no code yet.
