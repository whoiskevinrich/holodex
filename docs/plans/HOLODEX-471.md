---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-471                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
approved:
  design:
    on: 2026-09-27
    at: ffb4ba25
release_note: Owners can add an Overview to a video that has none, from the page or from Write metadata to file.
---

# HOLODEX-471 · Owner can add an Overview when neither the file nor a provider supplies one

Done means an owner viewing a video with no overview sees **+ Add overview** in the rail. It opens
the existing Edit Overview dialog with Custom selected, and the Write metadata to file dialog lists
an Overview row they can fill and write. Visitors see no change, and no other empty field changes.

**Design package:** [ADR-113](../architecture/ADR-113-owner-offered-empty-fields.md) (owner-offered
empty fields, adopted per field) · [spec F36 P1-5](../specs/field-source-of-truth.md) ·
[handoff](../design/overview-add-handoff.md) + [mockup](../design/overview-add-mockup.svg)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F36 P1-5 + Resolution step 4 in `field-source-of-truth.md`
- [x] architecture `architecture` → `docs/architecture/ADR-*` — ADR-113 (owner-offered empty fields, adopted per field)
- [x] design `design-handoff` → `docs/design/**` — `overview-add-handoff.md` + `overview-add-mockup.svg` (option B; signed off at ffb4ba25)
- [x] backend → `{cmd,internal,providers}/**` — `resolver.Options.Offer`, registry `OfferWhenEmpty` on `overview`, owner-only in `getMedia`, offered rows stamped writable (blank pins excluded)
- [x] frontend → `web/src/**` — "+ Add overview" pill in the rail; `SourceEditModal` opens on Custom with focus (`startsOnCustom`); QA'd live on backend-amv in all three skins
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [testing] Add the owner-offered empty-field rows to `docs/testing-strategy.md` — `docs/testing-strategy.md`
2. [ ] [security] `/security-review`: ADR-113 D3 is an owner/visitor gate on `resolved[]`
3. [ ] [design] Re-confirm the design sign-off: handoff behaviour 4 was corrected after QA (the empty writeback row sits in the `=` tier, not ○) — `docs/design/overview-add-handoff.md`

## Session log — newest first

### 2026-09-27 · session
- skills: handoff, implement, code-review
- Mocked the empty-Overview affordance (A/B/C); Kevin picked B (dashed **+ Add overview** pill → existing Edit Overview dialog) and Overview-only scope with an ADR for per-field adoption. Found the cause: the resolver drops empty undecided replace fields, so neither the page nor the writeback dialog had a row. Wrote ADR-113, spec F36 P1-5, and the handoff + three-skin SVG. Linked HOLODEX-304 as the rollout backlog.
- Backend: `ResolveFields` keeps an empty replace field when `opts.Offer` names it; `getMedia` sets `Offer = registry.OffersWhenEmpty` for the owner only. `ResolveForContainer` skips valueless fields, so `markWriteTargets` asks where an offered row's value would land (ADR-113 D1 amended). code-review high found that this also stamped blank-pinned rows; fixed and mutation-tested.
- Frontend: the **+ Add overview** pill (the "+ Set part" idiom) and `startsOnCustom` in f36.ts (vitest). `SourceEditModal` focuses the Custom textarea after ConfirmDialog's initial focus. Live QA on backend-amv: the pill, Custom checked and focused, the empty-save validation line, and a save that replaces the pill with the text. In the writeback row, picking Custom gives "Will be written → QuickTime:Comment / Write 1 field to file". Pill tokens were checked in all three skins. code-review high: 4 findings, no code change.
- handoff: Backend and frontend are done on Draft PR #402. Left: testing-strategy, security review, and a re-confirm of the design sign-off (the handoff's writeback-row wording was corrected after QA).

## Dropped — newest first (the reason is the point)
