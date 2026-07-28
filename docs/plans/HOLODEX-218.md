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

**Design package:** [spec](../specs/claimed-provider-keys.md) · [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) · design handoff *(unwritten)* · testing-strategy §F49 *(unwritten)*

## Gates — definition of done

- [x] spec `write-spec` → [claimed-provider-keys.md](../specs/claimed-provider-keys.md) — RD1 both YAML + in-app · RD2 mechanism now, detection deferred · RD3 promote/claim mutually exclusive
- [x] architecture `architecture` → [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) — `field_claims` (migration 0029, PK carries `provider`) · D2 suppression derives from the **merged field set**, not the claims table · D3 no precedence column, append last · D4 dangling claims inert, never pruned · D5 promotion clear at write time. Amends ADR-056 §D4
- [ ] design `design-handoff` → `docs/design/**` — the claim action on an auto-registered row (slice B)
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [~] security `security-review` — until: the ADR introduces anything beyond a keyed lookup table. Spec §9 records why not: no new egress/fs/credential surface, image perimeter untouched, same owner gate and validation shape as F44

## Up next — ordered (position = priority)

1. [ ] [backend] Slice A — `resolver.ClaimedKeys(effective []mapping.Field)` + second suppression input to `AutoRegisterFields`; `appendAutoRegistered` takes the effective fields (all three callers already hold them) — `internal/resolver/auto_register.go`, `internal/api/auto_register.go`
2. [ ] [testing] Slice A unit coverage — bare-file-tag + `file:` namespace claim nothing, same-key-different-provider isolation, unconditional suppression (winning *and* losing), partially-claimed key still auto-registers with the remaining provider only — `internal/resolver/auto_register_test.go`
3. [ ] [design] Claim action on an auto-registered row + canonical picker (spec Q3) → gates slice B frontend
4. [ ] [backend] Slice B — migration 0029 `field_claims` (PK `(entity_type, provider, field_key)`), repo CRUD mirroring `promotions.go`, `mergeClaims` after `mergePromotions` at all three call sites, owner-gated `/admin/field-claims/...` incl. the RD3 promotion clear in one transaction
5. [ ] [frontend] Slice B — claim affordance beside the F44 promote affordance; three-skin QA
7. [ ] [—] Proactive duplicate detection with library-wide counts → own issue (spec P1.3)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-28 · ADR-074 written; both blocking questions settled
- skills: architecture
- handoff: [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) landed, amending ADR-056 §D4 (a cross-reference blockquote is now in ADR-056 itself). The decision worth carrying forward is **D2**: auto-registration's second suppression input is derived from the **merged `[]mapping.Field`**, not from the `field_claims` table. That single choice collapses YAML `sources:` and DB claims to one code path, covers synthesized person/studio and F44 promotions for free, and — because suppression reads the *materialized* field set — makes "value suppressed with nowhere to go" unrepresentable rather than policed, which is what settles spec Q2 (D4: a dangling claim is inert and is never pruned; the key simply auto-registers again, exactly as pre-F49). Q1 settled as D3: the row carries the target canonical only, claims append last in `(provider, field_key)` order — identity is not precedence, and ADR-051 decisions stay the winner-picking instrument. Note the PK grain differs from `field_promotions` on purpose: `provider` is in the key, because `provA:synopsis` and `provB:synopsis` are different assertions. Security stayed deferred and the ADR records the test it was deferred against, including one honest asymmetry: a claim *can* feed a filterable canonical (a promotion cannot create one), bounded by FR4's 422 that the target must be a declared field of that entity type. Next: slice A still needs nothing from anyone — ADR/design gate slice B only.

### 2026-07-27 · spec written; three decisions resolved
- skills: write-spec
- handoff: F49 spec landed at `docs/specs/claimed-provider-keys.md`. Three `needs-spec` decisions resolved with Kevin: claims live in **both** YAML `sources:` and a DB table (person/studio have no YAML at all, so in-app is the only path there); v1 is mechanism + explicit claim action with proactive detection deferred; promote and claim are mutually exclusive, claiming clears the promotion after naming it. Two things the ticket had slightly off, corrected in the spec: derive the claimed set from the **effective `[]mapping.Field`** handed to `ResolveFields` (covers video YAML + synthesized person/studio + F44 promotions in one pass) rather than from `Mappings.Fields()`, and a DB claim must **append a candidate source**, not merely suppress — otherwise the value is hidden with nowhere to go. Also: `promotionTarget`'s 422-on-canonical is not the blocker the ticket described; it applies to the key being promoted and is still correct for claims. What's missing is a target-canonical parameter, hence a separate endpoint. Next: slice A is independently shippable and closes GH #178 — no ADR or design gate needed for it.
