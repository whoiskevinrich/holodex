---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-632
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: backend             # a rename-sanitiser bug with no UI change
depends-on: []
release_note: Renamed files keep a readable stand-in for characters a file name can't hold (`Star Wars: Episode IV` becomes `Star Wars- Episode IV`, `AC/DC` becomes `AC+DC`) instead of dropping them, and drop `!` and swap `;` for `,`.
---

# HOLODEX-632 · File rename deletes illegal characters instead of substituting (F86 bug)

Done means P0-4 substitutes readable stand-ins for illegal file-name characters, with a hardcoded
table. The configurable table is the follow-up story HOLODEX-633 (Relates).

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. `file-rename.md` P0-4: the substitution table, the `-` glue rule (joins the word before it, dropped with none), and the `;` → `,` people-delimiter caveat. Template text may hold `!` and `;`.
- [x] backend → `{cmd,internal,providers}/**`. `substitutes` in `internal/rename/name.go`, applied by `SanitizeValue` before removal.
- [x] testing `testing-strategy`. N8 row updated. `TestSanitizeValueSubstitutes` has one subtest per stand-in, and the whitespace table covers the glue rule, a kept literal ` - ` and a leading stand-in.

## Up next — ordered (position = priority)

1. [ ] [—] On merge, check that HOLODEX-632 moved to Done (CI moves the branch's own key).
2. [ ] [—] HOLODEX-633: make the substitution table a rename setting (second PR).
3. [ ] [—] Fold trim's `SanitizeEditionName` (`internal/maintenance/trim.go`) into the same table. It still deletes these characters, so a trimmed edition's file name disagrees with rename's spelling.

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-07 · substitute instead of delete
- skills: code-review high --fix (1 fixed: a leading stand-in `-` is now dropped; 1 skipped: `;` → `,` against the `, ` people delimiter is accepted and documented in P0-4; 1 skipped: trim's edition sanitiser, left for HOLODEX-633; 1 no change: renamed files going out of date on rollout is intended)
- handoff: The hardcoded substitution and its spec and tests landed with every gate settled, so the PR opens ready for review. After deploy, files renamed under the old rules show as Out of date in File names, and the next run renames them.
