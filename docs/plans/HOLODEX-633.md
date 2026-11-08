---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-633
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: feature             # a new owner setting with a UI surface; no technology fork, no auth change
depends-on: [HOLODEX-632]
release_note: You can now choose what each character a file name can't hold becomes, and add your own, from the Characters section of File names.
---

# HOLODEX-633 · Configurable rename character substitutions (F86 follow-up)

Done means the owner edits P0-4's character table in the Renaming card, previews its effect on the
plan, and saves it; the rename builder uses the saved table everywhere it builds a name.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. `file-rename.md` P0-4: "The owner edits the character table" (fixed rows, added rows, stand-in limits, one pass, preview then save, reset).
- [x] design `design-handoff` → `docs/design/**`. `file-rename-handoff.md` "Characters (HOLODEX-633)" + `file-rename-characters-mockup.svg`. The owner picked layout A (a row per character) over B (one-line field), and Preview-then-save, on 2026-11-07.
- [ ] backend → `{cmd,internal,providers}/**`. `rename.characters` setting key, validation, threaded through `Plan`/`Build` to `SanitizeValue`; preview body carries draft characters.
- [ ] frontend → `web/src/**`. `rename/CharacterTable.svelte` in `RenamingCard`, shared draft/footer, preview carries characters.
- [ ] testing `testing-strategy`.

## Up next — ordered (position = priority)

1. [ ] [backend] Setting key + validation + threading, with tests.
2. [ ] [frontend] CharacterTable + card/plan wiring, with component tests; Cinémathèque QA.
3. [ ] [—] Rebase onto main once HOLODEX-632 (#587) merges; this branch is stacked on it.
4. [ ] [—] Fold trim's `SanitizeEditionName` (`internal/maintenance/trim.go`) into the same table (spun out of HOLODEX-632's review).

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-07 · design
- skills: none (spec and handoff written in place; mockups rendered inline and approved)
- handoff: Spec and design are settled and owner-approved; next is /implement, then the backend setting.
