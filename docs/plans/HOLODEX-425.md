---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-425
status: in-review
release_note: The skin is now the instance's identity — the owner picks it once on the new Appearance tab and every viewer sees it; an owner can also declare a custom palette (five colors on top of Cinémathèque) in holodex.yaml.
---

# HOLODEX-425 · F67 Instance skin — owner-set skin on an Appearance tab + custom palette

The skin stops being a per-browser `localStorage` preference and becomes instance identity: the
header picker goes, an **Appearance** tab on `/owner` selects a shipped skin or the owner's
`theme.custom` palette, the choice persists in a new `settings` KV table (the first UI-set
operator setting) and ships to everyone in `/capabilities`. Custom = `base: cinematheque` + five
primaries; everything else is derived in CSS so the contrast pairs stay structurally safe.
Brainstormed 2026-09-16 (ladder S/L1/L2/L3 → S + L1; the in-app editor waits for an operator to
ask). Spec: [`docs/specs/instance-skin.md`](../specs/instance-skin.md).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/instance-skin.md` (F67), RD1–RD10 locked
- [x] architecture `architecture` — [ADR-102](../architecture/ADR-102-instance-skin-and-settings-store.md)
  D1–D6; supersedes ADR-021 §5 only; index row + ADR-021 annotation landed
- [x] design `design-handoff` — [instance-skin-handoff.md](../design/instance-skin-handoff.md) +
  [mockup SVG](../design/instance-skin-mockup.svg); option B (miniature browse preview) chosen;
  active = accent border + outlined chip, never solid fill
- [x] backend — S1: migration 0048 `settings`, `repo.GetSetting/PutSetting`, `PUT /admin/theme`,
  `/capabilities.theme`; S3: `internal/theme` (Parse / Derive / Contrast + the R11 gate
  `TestDeriveMatchesCinematheque`), `config.Theme`, boot wiring with contrast WARNs
- [x] frontend — S1: `theme.svelte.ts` server-applied + paint cache, preference removed, header picker
  deleted; S2: `/owner/appearance` option B cards (`.skin-card` two-branch fence in `app.css`); S3:
  `[data-palette='custom']` derivation block, `data-palette` on `<html>` and the custom card, R15
  contrast readout
- [x] testing `testing-strategy` — §13 added: Go gate + contrast/parse tests, repo + API tests, store
  tests, agent/human rows; CSS ↔ Go cross-check done live (7/7 tokens match to the hex)
- [x] security `security-review` — S1 and S3 reviewed clean 2026-09-19; the YAML → CSS path is hex-only
  by construction (regex + re-emit), the paint cache now holds the same shape

## Up next — ordered (position = priority)

1. [x] [HOLODEX-425] human QA 3.1–3.4 passed (Kevin, 2026-09-19); PR #365 marked ready, 425–428 swept
   to In Review by hand (epic-keyed branch → CI fires nothing)
2. [ ] [HOLODEX-429] luminance-switched ink partners (spec OQ3) — a mid-tone accent derives a dark ink
   that fails AA (the sample red lands at 3.91:1); follow-up, not a v1 blocker
3. [ ] [HOLODEX-425] on merge sweep 425/426/427/428 to Done by hand (epic-keyed branch → CI fires
   nothing)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 · brainstorm → epic → spec → ADR → design → S1 → S2 → S3
- skills: product-brainstorming, write-spec, architecture, design-handoff, code-review, security-review
- handoff: epic HOLODEX-425 + stories 426/427/428 filed, branch renamed
  `HOLODEX-425-instance-skin`, epic In Progress; spec + ADR-102 (D2 = ownership test) + design
  handoff (option B) committed; **S1 shipped** (backend + SPA plumbing, picker removed, live-verified:
  owner PUT → visitor reload lands Broadcast tokens, no picker). Code-review 4 findings fixed, security
  review clean. **S2 shipped**: Appearance tab live-verified (cards in their own tokens + flourishes,
  click → server persisted → page re-skinned, keyboard roving, 403 revert + alert, phone two-up, no
  picker). Found and fixed the descendant-selector bleed with the `.skin-card` fence. **S3 shipped**:
  `theme.custom` config → `internal/theme` → boot WARNs → `/capabilities`; `app.css` derivation block
  mirrored in Go with the R11 gate green (all seven tokens ΔE ≤ 2, browser and Go agree to the hex
  live); docs + example + testing-strategy §13. Every gate is green; PR #365 stays Draft only for
  Kevin's human QA. Filed HOLODEX-429 (mid-tone accent → dark ink fails AA).

### 2026-09-19 · human QA → ready for review
- skills: —
- handoff: 3.1–3.4 passed on the dev testbed with the sample palette (the one hiccup was my stopped
  backend, not code). PR #365 marked **ready**; 425–428 In Review. Nothing open on the branch; on
  merge sweep all four to Done by hand, then HOLODEX-429 is the next piece of F67.
