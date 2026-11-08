---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-633
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: feature             # a new owner setting with a UI surface; no technology fork, no auth change
depends-on: [HOLODEX-632]
approved:
  design:
    on: 2026-11-07
    at: 99884aa9
release_note: You can now choose what each character a file name can't hold becomes, and add your own, from the Characters section of File names.
---

# HOLODEX-633 · Configurable rename character substitutions (F86 follow-up)

Done means the owner edits P0-4's character table in the Renaming card, previews its effect on the
plan, and saves it; the rename builder uses the saved table everywhere it builds a name.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. `file-rename.md` P0-4: "The owner edits the character table" (fixed rows, added rows, stand-in limits, one pass, preview then save, reset).
- [x] design `design-handoff` → `docs/design/**`. `file-rename-handoff.md` "Characters (HOLODEX-633)" + `file-rename-characters-mockup.svg`. The owner picked layout A (a row per character) over B (one-line field), and Preview-then-save, on 2026-11-07.
- [x] backend → `{cmd,internal,providers}/**`. `rename.characters` key with `ParseCharacters` validation (`internal/rename/settings.go`); `Characters` threaded `Plan` → `Build` → `SanitizeValue`; preview body and `PUT /admin/settings` return 422 `characters` per row. A stored table that no longer validates falls back to the default.
- [x] frontend → `web/src/**`. `rename/CharacterTable.svelte` in `RenamingCard`: one draft and footer with templates, Save templates / characters / changes, browser example helper in `rename.ts`. Browser QA on the open stress fixture (:9700/:5176): a refused stand-in maps to its row, Preview renders names with the draft table, Save writes both settings and the fold counts characters, tokens resolve to Cinémathèque, no overflow at 375 px. Screenshots timed out (pane behind another window), so the visual pass was by computed style.
- [x] testing `testing-strategy`. N8b row. Go: `TestSanitizeValueOwnerTable`, `TestParseCharacters`, `TestCharactersRoundTrip`, `TestRenameCharacterTable`. Web: `CharacterTable.svelte.test.ts`, `RenamingCard` cases, `rename.test.ts`. `go test ./internal/...`, `npx vitest run` (1064) and `npm run check` (0 errors) pass.

## Up next — ordered (position = priority)

1. [ ] [—] Once #587 (HOLODEX-632) merges, the PR retargets to main: merge `origin/main` in (never rebase) and re-run the suites.
2. [ ] [—] On merge, move HOLODEX-633 to Done if CI doesn't (stacked branch).
3. [ ] [—] Fold trim's `SanitizeEditionName` (`internal/maintenance/trim.go`) into the same table (spun out of HOLODEX-632's review).

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-07 · build
- skills: code-review high --fix (1 fixed: the browser isSpace port compared to an invisible BOM literal; 2 skipped: stand-in maxlength counts UTF-16 units, and Save changes is two writes; 1 no change: a corrupt stored table falls back to the default)
- handoff: Backend, frontend and tests landed with every gate settled, so the PR is marked ready; it waits on #587 merging first.

### 2026-11-07 · design
- skills: implement (spec and handoff written in place; mockups rendered inline and approved), code-review
- handoff: Crossed into build — design signed off at 99884aa9; draft PR open. Start at Up next 1 (backend setting).
