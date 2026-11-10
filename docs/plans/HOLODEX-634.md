---
key: HOLODEX-634
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: feature             # a new template token with a UI chip; no technology fork, no auth change
depends-on: []
release_note: Rename templates can name a video's linked film with {film}, written into the file name as {film-Film Name}.
---

# HOLODEX-634 · Rename templates: {film} token for the linked film

Done means a rename template can carry `{film}`, which names the video's linked film as a
`{film-<name>}` marker that reads back exactly and never makes or changes a film link.

**Design package:** [spec F86 P0-1](../specs/file-rename.md) · [testing-strategy §39 N22](../testing-strategy.md#39-rename-media-files-from-a-template-f86-holodex-602)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. `file-rename.md` P0-1: the `{film}` row and "The film token" rule (first linked film by name, display spelling, no fit with no film or films off, never a link from a name, not an identity token). The owner picked the marker form and first-by-name on 2026-11-10.
- [~] design `design-handoff` → `docs/design/**`. Deliberately skipped with the owner's yes (2026-11-10): the only UI change is one `{film}` chip in the existing token chip row and a help sentence. `file-rename-handoff.md`'s chip list doesn't name it yet.
- [x] backend → `{cmd,internal,providers}/**`. `TokFilm` + `liftFilm` in `internal/rename/template.go`; the value from `FilmsForVideos`' first attachment in `loadRenameState`.
- [x] frontend → `web/src/**`. `{film}` chip in `TemplateList.svelte`, help line in `RenamingCard.svelte`. Browser QA on the open stress fixture (:9600/:5174): the chip shows; a preview of `{title} {film?}` gives 2,077 renames, 0 won't read back.
- [x] testing `testing-strategy`. N22 row; `TestBuildFilmMarker`, `TestRenameFilmMarker`. `go test ./internal/rename ./internal/api ./internal/scanner` and `npm run check` (0 errors) pass.

## Up next — ordered (position = priority)

1. [ ] [—] On merge, check HOLODEX-634 moved to Done (CI moves the branch's own key).
2. [ ] [—] Name `{film}` in `file-rename-handoff.md`'s chip list the next time the design is re-confirmed.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-11-10 · {film} token
- skills: code-review
- handoff: `{film}` is built, tested and QA'd; it moved here from the epic-keyed PR #590 (whose gate read the epic's squashed-away sign-off), and design is a deliberate skip for one chip.
