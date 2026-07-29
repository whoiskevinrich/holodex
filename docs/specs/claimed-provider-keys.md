# Spec: Claimed provider keys — attaching a provider key to an existing field (F49)

**Status**: Draft
**Phase**: Phase 3 enrichment follow-up
**Owner**: Project owner
**Date**: 2026-07-27
**Feature block**: **F49** — let a canonical field **claim** a provider key, so the key contributes its
value as a candidate of that field and stops rendering as a separate display-only auto-registered row.

**Issue**: [HOLODEX-218](https://whoiskevinrich.atlassian.net/browse/HOLODEX-218) ·
tracks [GH #178](https://github.com/whoiskevinrich/holodex/issues/178)
**ADR**: [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) — the `field_claims` store, the
derivation seam, and the merge-order relationship to F44 promotions. Settles Q1/Q2 below.
**Design**: required — the claim action on an auto-registered row is user-facing. *(not yet written —
`needs-design`)*
**Testing**: `docs/testing-strategy.md` needs an F49 block. *(not yet written)*

**Depends on** (all shipped):
- presence-driven auto-registration ([F39](provider-render-hints.md) /
  [ADR-056](../architecture/ADR-056-provider-field-render-hints.md), `AutoRegisterFields`,
  `ResolvedField.AutoRegistered`)
- in-app field promotion ([F44](promote-override-fields.md) /
  [ADR-062](../architecture/ADR-062-in-app-field-promotion.md), `mergePromotions`, `field_promotions`)
- the entity-agnostic resolver + canonical registry
  ([ADR-052](../architecture/ADR-052-baseline-source-contract.md), `ResolveFields`, `mapping.Field`)
- per-field source decisions ([F36](field-source-of-truth.md) /
  [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md))
- the owner gate ([ADR-046](../architecture/ADR-046-owner-session-persistence.md), `requireOwner`)

**Touches** an owner-gated mutation that changes which fields render and which sources feed a canonical
field. It writes presentation/precedence config the resolver trusts — the same perimeter F44 crossed —
so it inherits F44's posture. No new network, filesystem, or auth surface. A `/security-review` is
**not** required (see §9).

---

## 1. Problem Statement

A production library renders **Overview, Synopsis and Comments as three separate rows carrying identical
text**. The owner's only lever — listing the offending keys under a mapping's `sources:` — changes
nothing.

Both symptoms have one cause. `appendAutoRegistered` ([internal/api/auto_register.go:21](../../internal/api/auto_register.go)) builds its
suppression set from **canonical names**:

```go
for _, f := range resolved {
    rendered[strings.ToLower(strings.TrimSpace(f.Canonical))] = true
}
```

and `AutoRegisterFields` ([internal/resolver/auto_register.go:74](../../internal/resolver/auto_register.go)) tests it against the **raw
provider key**:

```go
if key == "" || strings.HasPrefix(key, model.InternalFieldPrefix) || registry.IsKnown(key) || rendered[key] { continue }
```

Both sides are lowercased and trimmed, so the comparison *looks* sound. It is comparing two different
namespaces. **A provider key is suppressed only when it happens to be spelled the same as the canonical
it maps into.** `tmdb:overview` → canonical `overview` survives by coincidence; `provA:synopsis` →
canonical `overview` does not, because `synopsis` is neither a registry canonical nor a rendered
canonical name.

That also explains the failed workaround: **nothing on this path ever reads the mapping's `sources:`
list**. Suppression is decided before the mapping is consulted, so listing a key there cannot affect it.

This is not a TMDB defect — the sidecar emits only `overview` (`providers/tmdb/tmdb.go`; `handler.go`
advertises no synopsis or comments key). The duplicates come from other configured providers.

### 1.1 The three namespaces

| Namespace | Example | Owned by |
|---|---|---|
| Provider key | `provA:synopsis`, `provB:comments`, `tmdb:overview` | each provider, independently |
| Holodex canonical | `overview` | the registry |
| File tag | `Comment`, `QuickTime:Comment` | the container spec |

Claiming maps 1 → 2. Writeback (`writeback.formatMap`) already maps 2 → 3. Only the first has no
accessor. F49 supplies it.

### 1.2 Cost of not fixing it

Every provider that names a common field differently adds a permanent duplicate row to every entity it
touches. The media page is already the most crowded surface in the product
([HOLODEX-220](https://whoiskevinrich.atlassian.net/browse/HOLODEX-220)), and each duplicate is
indistinguishable from a real field — the owner cannot tell "two providers disagree" from "one value,
rendered three times". It gets monotonically worse as providers are added, and there is no
configuration that fixes it today.

## 2. Resolved Decisions

Three questions carried the `needs-spec` label. Resolved with the project owner on **2026-07-27**:

### RD1 — Claims persist in **both** YAML `sources:` and a DB claims table

The suppression set is the union. GH #178's literal request is satisfied by the YAML half alone; the
in-app half exists because **`metadata-mappings.yaml` governs video only**. Person and studio field sets
are synthesized in code ([person_fields.go:34](../../internal/api/person_fields.go),
[studios.go:45](../../internal/api/studios.go)) — there is no person or studio YAML, so a stray
`provA:synopsis` on a person page could never be claimed by config. YAML-only would leave two of three
entity types permanently unfixable.

This also mirrors the precedent F44 set: promotion is DB-stored and materializes into the same
`mapping.Field` shape (`field_promotions.go:120`). Splitting the two closest operations an operator has —
*promote this key to its own field* vs *attach this key to an existing field* — across a gitignored
config edit plus `reload-config` on one side and a click on the other would be arbitrary.

### RD2 — v1 ships the mechanism and an explicit claim action; proactive detection is deferred

v1 makes claims work and gives the owner a way to make one. Proactive duplicate detection — comparing an
auto-registered key's values against a canonical field's resolved values and prompting — is a separate
issue: it needs its own design handoff, a library-wide aggregation query, and prompt-dismissal state.
Deferring it keeps GH #178 closed quickly and keeps the design gate on v1 small.

**Constraint carried to the follow-up:** counts shown must be **library-wide, never per-entity**. "Are
these two keys the same field?" is a config-level question whose answer must not depend on which video
happens to be open. For the same reason, value equality is only ever a signal to *prompt* on — never an
auto-fold. Per-entity equality would make a key vanish on one video and reappear on the next.

### RD3 — Promotion and claim are mutually exclusive; claiming clears the promotion

They are contradictory statements about one key: *this is its own field* vs *this belongs to `overview`*.
Holding both would leave an inert promotion in the DB that the promotion UI still lists — exactly the
invisible state that makes config debugging hard. Claiming a promoted key clears the promotion, and the
UI names what it will remove before it does. One key, one home.

## 3. Goals

1. **A provider key listed as a source of a field never auto-registers** — regardless of whether it wins
   resolution for any given entity.
2. **A claimed key still contributes its value.** Claiming attaches, it does not discard: the key becomes
   a candidate source of the target field, visible through the F36 source-decision chip.
3. **The reported case collapses to one row.** `overview.sources: [tmdb:overview, provA:synopsis,
   provB:comments, Comment]` renders a single Overview with combined provenance.
4. **Claiming is available for all three entity types**, including the two with no YAML.
5. **Claiming is provider-scoped** — provider B's identically-named key is untouched by provider A's claim.

## 4. Non-Goals

| Non-goal | Why |
|---|---|
| Auto-folding a key on value equality | Per-entity equality is not a config-level truth (RD2). A key that vanishes on one video and returns on the next is worse than a duplicate. |
| Proactive duplicate detection + prompt | Deferred to a follow-up (RD2). Needs its own design gate. |
| Per-entity claims | A claim answers "are these the same field?", which is type-global. Per-entity scope would reintroduce exactly the instability RD2 rejects. |
| Removing the existing `rendered` canonical-name check | It is incomplete, not wrong. It still suppresses an unmapped provider key that collides with a rendered canonical name. Dropping it would create *new* duplicates. |
| One key claimed by two canonicals | A key has one home. Multi-claim has no use case and makes precedence ambiguous. |
| Changing writeback | Claiming affects which sources feed a canonical, not which file tag it writes to. That is [HOLODEX-217](https://whoiskevinrich.atlassian.net/browse/HOLODEX-217). |
| Reconciling the shadow store | Claims are a read/resolve-time concern. No `entity_enrichment` row is rewritten or deleted. |

## 5. User Stories

1. As the **owner**, I want a provider's differently-named key to fold into the field it actually is, so
   my media page shows one Overview instead of three rows of the same paragraph.
2. As the **owner**, I want to attach a stray key to an existing field **from the page I noticed it on**,
   so I do not have to edit gitignored YAML and reload the server to fix a display problem.
3. As the **owner viewing a person or studio**, I want the same affordance, because those pages have no
   config file I could edit instead.
4. As the **owner**, when I attach a key that I previously promoted to its own field, I want to be told
   the promotion will be removed before it happens, so I am never surprised by a field disappearing.
5. As an **operator deploying with config**, I want `sources:` in `metadata-mappings.yaml` to be
   authoritative and self-documenting, so a mapping file can ship its claims with it.
6. As the **owner**, I want a claimed value to remain reachable through the source chip even when another
   provider wins the field, so claiming never silently loses data.
7. As the **owner**, I want clear instructions on how to map keys across the scenarios I actually hit,
   complete with worked examples, so I can tell which mechanism a given duplicate needs and copy a
   working config instead of inferring one from field reference tables.

## 6. Mechanism

### 6.1 Derive claims from the effective field set, not from `Mappings.Fields()`

The ticket proposed deriving the claimed set from `Mappings.Fields()[].Sources`. Reading the code, the
better seam is the **effective `[]mapping.Field` already handed to `ResolveFields`**:

- **video** — parsed YAML mappings
- **person / studio** — synthesized in code, and already provider-scoped: `personFields` builds
  `{Namespace: provider, Key: canonical}` per provider ([person_fields.go:69](../../internal/api/person_fields.go))
- **promoted fields** — `mergePromotions` materializes the same `mapping.Field` shape with
  `ParsedSources` ([field_promotions.go:120](../../internal/api/field_promotions.go))

One derivation over the effective set covers all four cases uniformly, and any future field source gets
claim behaviour for free. Deriving from `Mappings.Fields()` would cover video only.

```
claimedKeys(effective []mapping.Field) map[string]bool
    // key: "<provider>:<key>", both lowercased and trimmed
```

**Only namespaced provider sources claim.** A bare source (`Comment`, `Artist`) and the `file:`
namespace are file tags, not provider keys — they must never claim, or one mapping's `Comment` source
would swallow every provider's `comment` key.

### 6.2 A DB claim adds a source; it does not merely suppress

This is the load-bearing detail. A YAML `sources:` entry does two things at once: it makes the key a
**candidate** of the field *and* (after F49) claims it. A DB claim must do both, or the value is
suppressed with nowhere to go and the owner has silently lost it from the UI.

So a claim materializes as an appended `mapping.Source{Namespace: provider, Key: key}` on the target
field, exactly as `mergePromotions` materializes a promotion — after which §6.1's single derivation sees
it with no special case.

**Precedence:** an in-app claim appends at the **end** of the candidate list (lowest precedence). Adding
a claim must never silently change which value currently wins; the owner picks a different winner with
the existing F36 source chip if they want one.

**Merge order:** claims merge **after** promotions, so a claim may target a promoted field (key `X`
promoted to its own field, later joined by `provB:slogan`). RD3 forbids the same key being both.

### 6.3 Suppression is unconditional

A claimed key does not auto-register **whether or not it wins resolution for the entity being viewed**.
The claim is a config-level statement about identity, not a per-entity outcome. A key that renders on
videos where its provider loses, and vanishes where it wins, would be the same instability RD2 rejects.

### 6.4 What the API needs that F44 does not have

The ticket says `promotionTarget` "rejects canonical keys with 422, so the existing promote flow cannot
express *attach to `overview`*". Precisely: `promotionTarget`'s 422 applies to the **key being promoted**,
and that rule is still correct for claims — you cannot claim `bio`, it is already canonical. What is
missing is a **target canonical** parameter, which the promote endpoint has no place for. Claims need
their own endpoint rather than an extension of the promote one.

### 6.5 Scenario cookbook

The normative content behind US7 / FR7. Every example is a complete, copyable config. Providers are
named `provA` / `provB` generically; substitute the real names from `metadata-sources.yaml`.

**First, the choice that decides everything else:**

| The key is… | Use | Result |
|---|---|---|
| the same thing as a field you already have | **claim** (F49) | one row; the key becomes a candidate of that field |
| its own thing, deserving a row and curation | **promote** (F44) | a new first-class field |
| its own thing, fine as read-only | do nothing | it auto-registers display-only (F39) |
| noise you never want to see | *not covered* — no suppress-without-a-home exists (§4) | — |

They are mutually exclusive per key (RD3).

---

**S1 — One value, several provider names** *(the GH #178 case)*

Three providers describe the plot; each names it differently. Result: three identical rows.

```yaml
fields:
  - canonical: overview
    sources:
      - tmdb:overview          # winner — first non-empty wins
      - provA:synopsis         # claimed → no longer its own row
      - provB:comments         # claimed → no longer its own row
      - Comment                # bare = file tag (file:Comment); claims nothing
```

One **Overview** row. `provA` and `provB` become candidates behind the source chip, so their text is
still reachable — it just stops being three paragraphs of the same thing.

Order is precedence: sources are walked left-to-right, first non-empty wins. Moving `provA:synopsis`
to the top makes it the winner without changing what is claimed.

---

**S2 — Two providers, same key name, different meanings**

`provA:rating` is an age certificate; `provB:rating` is a 1–10 score. Claiming is provider-scoped, so
naming one leaves the other alone:

```yaml
fields:
  - canonical: content_rating
    sources:
      - provA:rating           # claimed
                               # provB:rating is untouched → still auto-registers as its own row
```

This is why a bare `rating` may never claim: it would swallow both.

---

**S3 — The key is on a person or a studio**

`metadata-mappings.yaml` governs **video only**. There is no person or studio YAML, so config cannot
express this at all — use the in-app claim on the row itself (FR5):

> Person → the auto-registered **Biography Text** row → *attach to an existing field* → pick **Bio**.

Type-global: it applies to every person, not just the one on screen.

---

**S4 — Claim onto a merge field**

A merge field (`multi: true`) unions its sources rather than picking a winner, so a claimed key there
**contributes values** instead of sitting behind the chip as a runner-up:

```yaml
fields:
  - canonical: genres
    multi: true
    sources:
      - tmdb:genres
      - provA:categories       # claimed → its values join the set
```

Worth knowing before claiming: on a **replace** field the claimed source is usually invisible until you
pick it; on a **merge** field its values appear immediately.

---

**S5 — The key deserves its own field**

`provA:filming_locations` is real information no canonical field covers. Claiming it onto something
would bury it. Promote instead (F44) — or, for video, add a mapping:

```yaml
fields:
  - canonical: filming_locations
    label: Filming Locations
    multi: true
    sources:
      - provA:filming_locations
```

A canonical entry and a claim are the same YAML gesture; the difference is whether the `canonical:` is
an existing field (claim) or a new one (promote).

---

**S6 — Undoing a claim**

- **YAML** — remove the source line and `reload-config`. The key auto-registers again on the next render.
- **In-app** — `DELETE /admin/field-claims/{entity_type}/{provider}/{field_key}`, or the unclaim
  affordance (P1.2) once it exists.

Nothing in the shadow store is rewritten either way, so an unclaim is always a clean reversal (§4).

---

**Two things that will not work, and why**

- **A bare key never claims.** `sources: [Comment]` means `file:Comment` — a file tag. It has no effect
  on any provider's `comment` key.
- **Claiming a canonical name is rejected** (422). `bio` is already a field; there is nothing to attach.

## 7. Requirements

### Must-have (P0)

**FR1 — Claimed-key derivation.** A pure helper derives `map["provider:key"]bool` from an effective
`[]mapping.Field`.
- [x] Given a field with `sources: [tmdb:overview, provA:synopsis]`, both `tmdb:overview` and
      `provA:synopsis` are claimed
- [x] Given a bare source `Comment`, nothing is claimed
- [x] Given a `file:Title` source, nothing is claimed
- [x] Keys and providers are compared lowercased and trimmed
- [x] `provA:synopsis` being claimed leaves `provB:synopsis` unclaimed

**FR2 — Suppression.** `AutoRegisterFields` takes the claimed set as a second suppression input and
skips a field whose `provider:key` is claimed.
- [x] The existing `rendered` canonical check is retained unchanged (Non-goals)
- [x] Suppression does not depend on whether the claimed source won resolution for that entity

**FR3 — DB claims store.** Migration `0029_field_claims` (`.up.sql` + hand-written `.down.sql`) plus repo
CRUD, keyed by `(entity_type, provider, field_key)` with the target canonical as payload.
- [x] `entity_type` is one of `video`, `person`, `studio` (reuses `parseEntityType`)
- [x] A claim merges as an appended candidate source on the target field (§6.2)
- [x] Claim merge runs after promotion merge
- [x] A claim whose target canonical is absent from the effective field set is ignored at resolve time
      and logged — it must never suppress into a black hole

**FR4 — Owner-gated claim API**, mounted inside `requireOwner`, type-global path (not
`/{entity}/{id}/...`) for the same reason F44 chose `/admin/field-promotions`.
- [x] `GET /admin/field-claims/{entity_type}` lists claims
- [x] `PUT /admin/field-claims/{entity_type}/{provider}/{field_key}` with `{"canonical": "..."}` upserts
- [x] `DELETE /admin/field-claims/{entity_type}/{provider}/{field_key}` is idempotent (204 on missing)
- [x] 400 on unknown `entity_type`; 422 on a reserved (`_`-prefixed) or canonical `field_key`
- [x] 422 when the target canonical is not a field of that entity type
- [x] PUT on a key that currently holds an F44 promotion clears the promotion in the same transaction (RD3)
- [x] `GET /admin/field-targets/{entity_type}` serves the picker's target list (handoff DD2): the
      **effective** post-promotion field set with a `merge` flag, which the page cannot derive from
      `resolved[]` because empty undecided fields never render

**FR5 — In-app claim action.** An owner-only affordance on an auto-registered row: *attach this field to
an existing field*, with a picker over that entity type's canonical fields.
- [x] Reachable from the row itself, alongside the existing F44 promote affordance
- [x] Claiming a promoted key names the promotion it will remove, before the claim is applied
- [x] The row disappears on reload and its value appears under the target field's source chip
- [x] On `long_text` / `chips` rows the owner controls move to their own trailing line
      ([handoff DD7](../design/claimed-provider-keys-handoff.md)) — this **amends F44's shipped layout**, so
      the promote pill is re-QA'd there too
- [~] Three-skin QA (`.claude/rules/frontend-theming.md`) — checklist written
      ([claimed-provider-keys-qa-checklist.md](../design/claimed-provider-keys-qa-checklist.md)); §2/§3 run,
      §4's human-eye items pending

**FR6 — Coverage.** Unit tests on FR1 (including the bare-file-tag and same-key-different-provider cases),
FR2 suppression, and the FR4 validation matrix.

**FR7 — Operator documentation.** §6.5's cookbook lands in the operator-facing docs, not only in this
spec. The behaviour change in §8 means an operator's existing mental model is now wrong, so this ships
**with slice A**, not after it.
- [x] [`docs/reference/canonical-fields.md`](../reference/canonical-fields.md) gains a *Claiming a
      provider key* section under "How to reference fields", covering S1–S6 with the copyable YAML
- [x] Its F39 auto-registration note is amended — a key listed in `sources:` no longer auto-registers
- [x] The bare-key row in the source-namespace table states that a bare key claims nothing
- [x] `metadata-mappings.yaml.example` carries a commented multi-provider `overview` example (S1)
- [x] The claim/promote/do-nothing decision table is stated once and linked from both directions
- [x] Slice B adds the in-app path (S3) and the unclaim path (S6) to the same section

**FR8 — Attached-keys list in owner tooling.** *(Was P1.1; promoted to P0 on 2026-07-28 — see
[handoff DD8](../design/claimed-provider-keys-handoff.md).)* A claim is **invisible by construction**: it
succeeds by removing the row that was its only evidence. Without a list, a type-global config edit ships with
no durable way to see or reverse it, and the acceptance bullet *"and undo it"* is only true for the seconds
after the gesture.
- [x] A seventh owner-hub tab at `/owner/fields`, labelled **Attached keys** (never "claims" — handoff DD1)
- [x] Lists every DB claim across all three entity types, grouped by type, sorted `(provider, field_key)`
- [x] Each row shows `provider:field_key → target label` and a one-click **Remove**
- [x] A claim whose target canonical is absent from the effective field set is marked **Inactive** — this is
      the only surface where a dangling claim (RD-adjacent: ADR-074 §D4 keeps it, inert) is visible at all
- [x] Remove does **not** restore a promotion that claiming cleared (§RD3), and the copy does not imply it does
- [~] Three-skin QA — see FR5

### Should-have (P1)

- ~~**P1.1** Claims listed in owner tooling, so the owner can see and undo every claim in one place.~~
  **Promoted to P0 as FR8** (2026-07-28). Numbering below is unchanged so existing references still resolve.
- **P1.2** An "unclaim" path from the target field's source chip (the inverse gesture, where the value now lives).
- **P1.3** Proactive duplicate detection + prompt — the deferred half of RD2, with library-wide counts.

### Future considerations (P2)

- **P2.1** Suggesting a claim from a provider's own render hints, when a provider advertises a label that
  matches a canonical's label.
- **P2.2** Bulk claim across entity types in one action.

## 8. Behaviour change to accept

**This is not purely additive.** A key an operator already lists under `sources:` for other reasons stops
auto-registering the day this ships. In practice such a key was rendering twice (once inside the
canonical field, once as its own row), so the change removes a duplicate rather than data — but a row the
owner is used to seeing will disappear.

Two consequences worth stating plainly:

1. A **losing** claimed source no longer gets an accidental row of its own. Its value is reachable through
   the F36 source chip, which is where a non-winning candidate is supposed to live — but it is one click
   away instead of on the page.
2. Anyone who deliberately exploited the bug to surface a second provider's text as a separate row must
   now use F44 promotion, which is the affordance actually meant for that.

**Release note required.**

## 9. Security posture

No `/security-review` gate. Reasoning:

- No new network egress, filesystem access, or credential handling.
- The ADR-039/056 image perimeter is untouched: `gateImageURL` runs on the auto-registered and promoted
  paths and is unaffected by suppression. A claimed key routes through the **canonical** field path,
  which already applies the canonical image rules.
- The write surface is owner-gated by the same `requireOwner` group as F44, with the same validation
  shape and no free-text sink — `provider`, `field_key` and `canonical` are all validated against known
  vocabularies.

If the ADR introduces anything beyond a keyed lookup table, revisit this.

## 10. Open Questions

Blocking — **both settled by [ADR-074](../architecture/ADR-074-claimed-provider-keys.md)**:

- ~~**Q1 (architecture)** — does the claims table carry only `(entity_type, provider, field_key) →
  canonical`, or does it also carry the precedence position?~~ **Canonical only** (ADR-074 D3). Claims
  append last, ordered `(provider, field_key)`. Identity is not precedence: a claim must never move the
  winner, and per-entity precedence is already F36's job.
- ~~**Q2 (architecture)** — a claim whose target canonical later disappears becomes dangling. Prune on
  read, prune on config reload, or surface in owner tooling?~~ **None of the three** (ADR-074 D4). The row
  survives, inert. Because suppression derives from the *merged field set* rather than the claims table
  (D2), a claim that fails to materialize suppresses nothing — the key auto-registers again, exactly as
  before F49. The black hole is structurally impossible rather than policed.

Non-blocking:

- ~~**Q3 (design)** — does the claim picker list every canonical for that entity type, or only fields that
  currently render for the entity being viewed?~~ **Every canonical for the entity type**
  ([handoff DD2](../design/claimed-provider-keys-handoff.md#dd2--the-picker-lists-every-canonical-field-for-the-entity-type-not-the-ones-currently-on-screen)).
  Not a cosmetic choice: undecided **empty** fields are dropped from `resolved[]` (`resolver.go:286`), so a
  screen-derived picker omits exactly the targets the owner needs — a person's empty `bio` is missing precisely
  when the provider's own biography key is the only one on the page. Requires a small owner-gated
  `GET /admin/field-targets/{entity_type}` returning the **effective** (post-promotion) set with a `merge` flag.
- **Q4 (product)** — should a claimed key remain visible to MCP consumers under its original key? Nothing
  in the MCP surface reads auto-registered rows today, so this only matters if that changes.

## 11. Acceptance (feature-level)

- [ ] A provider key listed in any mapping's `sources:` does not auto-register, whether or not it wins
      resolution for a given entity
- [ ] Claiming is provider-scoped — provider B's identically-named key is unaffected
- [ ] Bare file-tag sources claim nothing
- [ ] The reported case collapses to one Overview row with combined provenance
- [ ] The owner can claim a key in-app on all three entity types, and undo it — immediately from the row
      (handoff DD5) **and later** from the Attached keys list (FR8)
- [ ] Claiming a promoted key clears the promotion, after saying so
- [ ] A claimed value stays reachable via the F36 source chip
- [ ] No auto-folding on value equality anywhere in v1
- [ ] An operator can find, in the reference docs, a worked example for each scenario in §6.5 and tell
      from it whether their case wants a claim, a promotion, or nothing

## 12. Phasing

| Slice | Contents | Gate |
|---|---|---|
| **A — mechanism** ✅ | FR1, FR2, FR6, FR7 (derivation + suppression + operator docs). Fixes GH #178 for YAML users. | spec ✓, testing ✓ |
| **B — in-app claims** | FR3, FR4, FR5, **FR8**. Unblocks person/studio. Adds `GET /admin/field-targets/{entity_type}` (handoff DD2) and the Attached keys list. | ADR-074 ✓, design handoff ✓ |
| **C — detection** | P1.3, own issue | own design handoff |

Slice A is independently shippable and closes the reported bug. B is what makes the feature complete for
entity types with no config file.
