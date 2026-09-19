---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-425
status: in-progress
release_note: The skin is now the instance's identity — the owner picks it once on the new Appearance tab and every viewer sees it; an owner can also declare a custom palette (five colors on top of Cinémathèque) in holodex.yaml.
---

# HOLODEX-425 · F66 Instance skin — owner-set skin on an Appearance tab + custom palette

The skin stops being a per-browser `localStorage` preference and becomes instance identity: the
header picker goes, an **Appearance** tab on `/owner` selects a shipped skin or the owner's
`theme.custom` palette, the choice persists in a new `settings` KV table (the first UI-set
operator setting) and ships to everyone in `/capabilities`. Custom = `base: cinematheque` + five
primaries; everything else is derived in CSS so the contrast pairs stay structurally safe.
Brainstormed 2026-09-16 (ladder S/L1/L2/L3 → S + L1; the in-app editor waits for an operator to
ask). Spec: [`docs/specs/instance-skin.md`](../specs/instance-skin.md).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/instance-skin.md` (F66), RD1–RD10 locked
- [x] architecture `architecture` — [ADR-102](../architecture/ADR-102-instance-skin-and-settings-store.md)
  D1–D6; supersedes ADR-021 §5 only; index row + ADR-021 annotation landed
- [x] design `design-handoff` — [instance-skin-handoff.md](../design/instance-skin-handoff.md) +
  [mockup SVG](../design/instance-skin-mockup.svg); option B (miniature browse preview) chosen;
  active = accent border + outlined chip, never solid fill
- [/] backend — **S1 done**: migration 0048 `settings`, `repo.GetSetting/PutSetting`, `PUT /admin/theme`,
  `/capabilities.theme` (theme.go + tests); S3 still open: `theme.custom` parse/validate, contrast WARN
- [/] frontend — **S1 done**: `theme.svelte.ts` server-applied + paint cache, preference removed,
  **header picker deleted here** (it called the removed `theme.set`; moved up from S2); S2 still open:
  `/owner/appearance`; S3: derivation layer in `app.css`
- [ ] testing `testing-strategy` — R1–R3 handler/repo tests, R11 derivation gate, R12 contrast
  unit test, three-skin + custom QA matrix
- [/] security `security-review` — S1 reviewed clean 2026-09-19 (CSRF/authz/SQL/DOM-CSS/cache/exposure);
  re-run after S3 lands the YAML → CSS surface

## Up next — ordered (position = priority)

1. [ ] [HOLODEX-427] S2 Appearance tab (`/owner/appearance`, option B cards) — picker already gone
2. [ ] [HOLODEX-428] S3 custom palette, derivation (R11 gate), contrast WARN, docs
3. [ ] [HOLODEX-425] on ready-for-review sweep 426/427/428 to In Review by hand; on merge sweep
   all four to Done by hand (epic-keyed branch → CI fires nothing)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 · brainstorm → epic → spec → ADR → design → S1
- skills: product-brainstorming, write-spec, architecture, design-handoff, code-review, security-review
- handoff: epic HOLODEX-425 + stories 426/427/428 filed, branch renamed
  `HOLODEX-425-instance-skin`, epic In Progress; spec + ADR-102 (D2 = ownership test) + design
  handoff (option B) committed; **S1 shipped** (backend + SPA plumbing, picker removed, live-verified:
  owner PUT → visitor reload lands Broadcast tokens, no picker). Code-review 4 findings fixed, security
  review clean. Draft PR #365. Next is S2 (HOLODEX-427): `/owner/appearance` with option B cards.
