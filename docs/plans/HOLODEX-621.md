---
# Flightplan worklog — one epic, one worklog, one definition of done.
key: HOLODEX-621
status: in-progress          # DERIVED from the Gates below; only `done` and `released` are read from here.
profile: feature             # new user-facing behavior: spec, design, backend, frontend, testing
depends-on: []
release_note: Rename templates can mark a token optional with `?`, such as `[{studio?}]`, so one template names videos with or without that value, and the template help explains it.
approved:
  design:
    on: 2026-11-01
    at: e452a36f
---

# HOLODEX-621 · Optional rename-template tokens (F86 follow-up)

Done means one template with `{token?}` tokens replaces a ladder of fallback templates. An empty
optional token drops along with the literal text touching it, and every built name still reads back
exactly.

**Design package:** [spec F86 P0-1/P0-2](../specs/file-rename.md) · [handoff, TEMPLATES block](../design/file-rename-handoff.md) · [testing-strategy §39.2 N20–N21](../testing-strategy.md#39-rename-media-files-from-a-template-f86-holodex-602)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**`. P0-1 has the optional-token rules and worked example, and P0-2 says "every required token". The owner decided both rules on 2026-11-01 (an empty token drops its whole word, and any token may take `?` as long as title or an identity token stays required).
- [x] design `design-handoff` → `docs/design/**`. The handoff's TEMPLATES block gains the second help line on token syntax.
- [x] backend → `{cmd,internal,providers}/**`. `internal/rename/template.go`: each template compiles into its plain variants, most tokens kept first, and render and read-back go through them. At most six tokens can be optional. Variants that differ only in markers share one compiled pattern.
- [x] frontend → `web/src/**`. `RenamingCard.svelte` has the help line. In the stress-fixture preview the spec's template named 169 videos.
- [x] testing `testing-strategy`. N20 and N21 are in §39.2, with `TestBuildOptionalTokens`, `TestDropOptional` and `TestParseTemplateOptionalRefuses`. `go test ./internal/rename ./internal/api ./internal/extract` and the rename vitest suite pass.

## Up next — ordered (position = priority)

1. [ ] [—] On merge, move HOLODEX-621 to Done by hand (a child of HOLODEX-602; CI moves only the branch's own key, so check it moved)

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-11-01 · optional tokens
- skills: code-review high --fix, handoff, implement
- handoff: Crossed into build: design signed off at `e452a36f` and every gate is settled, so the PR opens ready for review; next is merging it and moving HOLODEX-621 to Done.

## Dropped — newest first (the reason is the point)
