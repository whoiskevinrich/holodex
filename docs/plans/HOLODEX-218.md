---
key: HOLODEX-218
status: in-progress
depends-on: []
release_note: A canonical field can now claim a provider's differently-named key, so one value stops rendering as several duplicate rows.
---

# HOLODEX-218 · Claimed provider keys (F49)

A canonical field can **claim** a provider key: the key contributes its value as a candidate of that
field and stops auto-registering as a separate display-only row. **Done** = GH #178's three-identical-rows
case renders one Overview, and the owner can claim a key in-app on video, person and studio.

**Design package:** [spec](../specs/claimed-provider-keys.md) · ADR-074 *(reserved, unwritten)* · design handoff *(unwritten)* · testing-strategy §F49 *(unwritten)*

## Gates — definition of done

- [x] spec `write-spec` → [claimed-provider-keys.md](../specs/claimed-provider-keys.md) — RD1 both YAML + in-app · RD2 mechanism now, detection deferred · RD3 promote/claim mutually exclusive
- [ ] architecture `architecture` → ADR-074 — the DB claims store; extends ADR-056/062. Needed for slice B only
- [ ] design `design-handoff` → `docs/design/**` — the claim action on an auto-registered row (slice B)
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [~] security `security-review` — until: the ADR introduces anything beyond a keyed lookup table. Spec §9 records why not: no new egress/fs/credential surface, image perimeter untouched, same owner gate and validation shape as F44

## Up next — ordered (position = priority)

1. [ ] [backend] Slice A — `claimedKeys(effective []mapping.Field)` + second suppression input to `AutoRegisterFields` — `internal/resolver/auto_register.go`, `internal/api/auto_register.go`
2. [ ] [testing] Slice A unit coverage — bare-file-tag case, same-key-different-provider case, unconditional suppression — `internal/resolver/auto_register_test.go`
3. [ ] [architecture] ADR-074 — claims table grain, precedence position (spec Q1), dangling-claim policy (spec Q2) → gates slice B
4. [ ] [design] Claim action on an auto-registered row + canonical picker (spec Q3) → gates slice B frontend
5. [ ] [backend] Slice B — migration 0029 `field_claims`, repo CRUD, `mergeClaims` after `mergePromotions`, owner-gated `/admin/field-claims/...` incl. the RD3 promotion clear
6. [ ] [frontend] Slice B — claim affordance beside the F44 promote affordance; three-skin QA
7. [ ] [—] Proactive duplicate detection with library-wide counts → own issue (spec P1.3)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-27 · spec written; three decisions resolved
- skills: write-spec
- handoff: F49 spec landed at `docs/specs/claimed-provider-keys.md`. Three `needs-spec` decisions resolved with Kevin: claims live in **both** YAML `sources:` and a DB table (person/studio have no YAML at all, so in-app is the only path there); v1 is mechanism + explicit claim action with proactive detection deferred; promote and claim are mutually exclusive, claiming clears the promotion after naming it. Two things the ticket had slightly off, corrected in the spec: derive the claimed set from the **effective `[]mapping.Field`** handed to `ResolveFields` (covers video YAML + synthesized person/studio + F44 promotions in one pass) rather than from `Mappings.Fields()`, and a DB claim must **append a candidate source**, not merely suppress — otherwise the value is hidden with nowhere to go. Also: `promotionTarget`'s 422-on-canonical is not the blocker the ticket described; it applies to the key being promoted and is still correct for claims. What's missing is a target-canonical parameter, hence a separate endpoint. Next: slice A is independently shippable and closes GH #178 — no ADR or design gate needed for it.
