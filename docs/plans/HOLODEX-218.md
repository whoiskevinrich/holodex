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

**Design package:** [spec](../specs/claimed-provider-keys.md) · [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) · [design handoff](../design/claimed-provider-keys-handoff.md) · [testing-strategy §F49](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → [claimed-provider-keys.md](../specs/claimed-provider-keys.md) — RD1 both YAML + in-app · RD2 mechanism now, detection deferred · RD3 promote/claim mutually exclusive
- [x] architecture `architecture` → [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) — `field_claims` (migration 0029, PK carries `provider`) · D2 suppression derives from the **merged field set**, not the claims table · D3 no precedence column, append last · D4 dangling claims inert, never pruned · D5 promotion clear at write time. Amends ADR-056 §D4
- [x] design `design-handoff` → [claimed-provider-keys-handoff.md](../design/claimed-provider-keys-handoff.md) — DD1 verb is "Attach to…" (never "merge") · **DD2 settles Q3**: picker lists the whole entity-type field set, needs `GET /admin/field-targets/{entity_type}` · DD3 provider checklist when a row carries 2+ providers · DD4 outcome preview (replace vs merge) · DD5 in-place confirmation + session Undo · DD6 RD3 warning in `--warn` · **DD7 accepted** — controls on their own line for `long_text`/`chips`, amending F44's shipped layout · **DD8 accepted** — P1.1 claims list promoted to P0 as spec FR8, specified as handoff §3 (`/owner/fields`, "Attached keys") · DD9 dangling claims render **Inactive** there (the only surface they have)
- [x] backend — **slice A** (`ClaimedKeys` + second suppression input + operator docs; GH #178 closed for YAML users) **and slice B** (migration 0029 `field_claims`, `repo/claims.go`, `mergeClaims` at all three call sites, owner-gated `/admin/field-claims/...` + `/admin/field-targets/{entity_type}`)
- [~] frontend — slice B built: Attach pill + `ClaimFieldEditor.svelte` + DD5 strip + DD7 layout amendment + FR8 `/owner/fields`. QA checklist written ([claimed-provider-keys-qa-checklist.md](../design/claimed-provider-keys-qa-checklist.md)); §2/§3 run green across all three skins by contrast/geometry sweep, §4's human-eye items pending. Slice A ships no UI change
- [x] testing `testing-strategy` → §9 F49 block + `internal/resolver/auto_register_test.go` — five cardinal invariants: provider-scoped, unconditional, no black hole, rendered/claimed both retained, golden no-op
- [~] security `security-review` — until: the ADR introduces anything beyond a keyed lookup table. Spec §9 records why not: no new egress/fs/credential surface, image perimeter untouched, same owner gate and validation shape as F44

## Up next — ordered (position = priority)

1. [ ] [qa] §4 of the QA checklist — the human-eye items, in all three skins, **now running against a PR already in review**. The DD7 trailing line is the one that matters: it moved a **shipped** F44 control, so the promote pill needs eyes on it too
2. [x] [—] Proactive duplicate detection with library-wide counts → filed as **HOLODEX-222** (slice C, spec
   P1.3), carrying RD2's two constraints in the description so they can't be lost: counts stay library-wide
   (a claim is global, so a per-entity sample is the wrong evidence for it) and value equality only ever
   prompts, never auto-folds

**PR #182 marked ready for review on 2026-07-28** (fires `In Review`, ADR-069) — ahead of §4 rather than
after it, at Kevin's call. Its title was corrected from `docs(specs):` to `feat(fields):` at the same time:
this repo squash-merges, so the PR title is what release-please and git-cliff read, and the docs-first
subject it was opened with would have filed a feature under docs.

**Done:** slice A — `ClaimedKeys` + suppression + unit coverage + FR7 operator docs. Slice B backend —
migration 0029, `repo/claims.go`, `mergeClaims`, the claims API + `GET /admin/field-targets/{entity_type}`.
Slice B frontend — Attach pill + `ClaimFieldEditor.svelte`, DD5 confirmation strip with Undo, DD7 layout
amendment, FR8 `/owner/fields` "Attached keys" list + hub tab, FR7's last docs box.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-28 · gate bookkeeping resynced; slice C filed
- skills: —
- handoff: Two pieces of drift closed, both found by asking what was actually left rather than by
  reading the checkboxes. **HOLODEX-218's Jira description was stale in a way that read backwards**: it
  still called the design handoff "the only outstanding gate" (it landed days ago) and marked three-skin
  QA `[x]` on the grounds that slice A ships no UI — true when written, but slice B does ship UI, so the
  one genuinely open gate was showing as green. Rewritten: handoff `[x]`, three-skin QA `[~]` and named
  explicitly as **the** open gate with 4.1 and 4.7 called out, both slice-B acceptance boxes checked
  (§3.3/§3.6 ran green live; the promotion clear is pinned at both the repo and API layer, including that
  un-claiming does not resurrect it), and a Slice B section plus the code-review outcome added. **Slice C
  had never been filed** — RD2 deferred it "to its own issue" and no such issue existed, so it was one
  session away from being lost to a spec footnote; now **HOLODEX-222** under the Enrichment fields epic,
  carrying RD2's constraints and the four open questions the spec will have to answer. PR #182's gate
  block got the same treatment (slice C row now links the issue; a `/code-review` line records the three
  bugs and the single-provider-fixture lesson). §4 remains the only thing between this and merge.

### 2026-07-28 · code review of the branch — three frontend bugs, all fixed
- skills: code-review
- handoff: A `/code-review high --fix` pass over the branch diff found three real frontend bugs, all in
  slice B, all now fixed and each proven live by reverting the fix and re-measuring. **(1) The Attach
  editor lost focus entirely on single-provider rows.** The focus `$effect` fires before the target list
  loads, so it called `focus()` on a still-`disabled` `<select>` — a no-op. With no DD3 checklist to focus
  instead, and the pill removed from the DOM in the same update, `activeElement` fell to `<body>`; measured
  `{disabled:true, opts:0, active:"BODY"}` at tick 0 and still `"BODY"` after settling, with **Escape not
  closing the editor** because the handler sits on the editor div. The effect now reads `targets.length` so
  it re-runs when the options land, latched by a `focused` flag so it can't steal focus back. This is the
  single-provider case — the *common* one — which is exactly what the two-provider QA fixture doesn't cover;
  worth remembering when writing the next fixture. **(2) `/owner/fields` left a phantom row on concurrent
  Remove.** `remove()` spliced by object identity, but each successful removal rebuilds every group object,
  so a second removal already in flight matched nothing: server reported 1 claim left while the UI listed 2.
  Matched by `rowKey` now — the same stale-reference class the DD5 strip already fixed by `id`, which is a
  sign the pattern wants to be the default here, not the exception. **(3) DD9 could mark a live claim
  Inactive.** `targetOf()` compared canonicals with `===`, but the API lower-cases a claim's canonical on
  write while a target's comes verbatim from the mapping (`mapping.go` only trims; `ByCanonical` is
  case-insensitive for exactly that reason) — a video YAML declaring `canonical: Overview` would render a
  working attachment as "target field no longer exists". Now compared case-insensitively, matching the
  server's `EqualFold`. Not reachable on person/studio (synthesized canonicals are all lower-case, verified
  live), so only an operator's video YAML triggers it. §4's human-eye pass is still the one open gate.

### 2026-07-28 · slice B frontend — the Attach gesture, the strip, the Attached keys list
- skills: —
- handoff: F49's UI is built and F49 is now feature-complete. `AutoFieldRows` gained the peer **Attach to…**
  pill, `ClaimFieldEditor.svelte` is the editor behind it, and `/owner/fields` is the FR8 list. Three things
  worth carrying. **(1) Two bugs the live QA caught that no type-check would.** The DD5 strip's Undo silently
  did nothing after the first click, because `update()` replaces the strip object and the handler's captured
  reference was no longer the one in the array — everything is matched by `id` now. And the DD3 checklist's
  value caption widened the whole Details column and scrolled the page sideways: `<fieldset>` carries a UA
  `min-inline-size: min-content`, so `min-w-0` on both the fieldset and the editor root is load-bearing, not
  tidying. **(2) The QA fixture is part of the feature.** A one-provider row can't exercise the checklist or
  the partial-attach outcome, so the checklist's §1 specifies a two-provider row with a `long_text` hint; I
  seeded it directly into `entity_enrichment` + `provider_field_hints` and tore it down after. **(3) DD6 is
  nearly unreachable and I kept it anyway** — a promoted key normally renders as its own first-class field,
  not an auto-registered row, so the "attaching removes that promotion" warning rarely fires. The server
  clears the promotion either way, so showing it is the honest call; §5.2 of the checklist records why, so
  nobody deletes it as dead UI without knowing. Agent QA (contrast + geometry) is green in all three skins;
  §4's human items are what's left. Also noted, **not** caused by this change: the active owner-hub tab
  renders the same background in every skin.

### 2026-07-28 · slice B backend built — the claim store, the merge, the API
- skills: —
- handoff: Slice B's backend is in: migration `0029_field_claims`, `internal/repo/claims.go`,
  `mergeClaims` at all three call sites, and the owner-gated `/admin/field-claims/...` trio plus
  `GET /admin/field-targets/{entity_type}`. Three things worth carrying forward. **(1) `SetClaim` is a
  real transaction, not two writes** — the RD3 promotion clear runs inside it, because a claim landing
  beside a surviving promotion of the same key would render that key twice, which is the exact bug F49
  exists to fix. **(2) `mergeClaims` clones before appending.** Base fields are shared across requests
  (the parsed mappings store, the synthesized person/studio sets), so `append` on `ParsedSources` would
  write into a backing array another request is reading — `slices.Clone` per touched field. It also skips
  a source the field already lists, so an operator whose YAML already says `provA:synopsis` doesn't get a
  duplicated candidate when the owner claims the same key in-app. **(3) The test that changed my mind
  about the docs:** D3's lexicographic append order is observable — claiming `tmdb:biography` *first* and
  `provb:life_story` second resolves to **provb**, because claims sort by `(provider, field_key)`, not by
  insertion. That's the "reproducible from table contents, not edit history" property, and it's now pinned
  rather than implied. `entityTypeFields` serves both the DD2 picker endpoint and FR4's
  target-must-be-declared 422 from one derivation, which is what keeps the ADR-074 security deferral
  honest — a claim can only add a candidate to a surface the entity type already declares. Full Go suite
  green. Next: the two frontend items; nothing else blocks them.

### 2026-07-28 · both open design questions answered yes; scope folded in
- skills: design-handoff
- handoff: Kevin accepted **both** items §9 put back to him, so the handoff has nothing open. **DD8 → spec FR8**: P1.1 is promoted to P0 and the surface is now specified as handoff §3 — `/owner/fields`, a seventh hub tab labelled **Attached keys** (not "claims"; DD1's verb has to be the same word in the nav as on the pill), one bordered section per entity type in the `/owner/duplicates` shape, rows reading `provider:key → target label` with a one-click Remove. Specifying it surfaced **DD9**, which nobody had asked for: ADR-074 §D4 keeps a dangling claim forever and inert, and §3 is the *only* place it exists at all, so the list marks it **Inactive** in `--warn` — a client-side cross-check against the DD2 targets endpoint the page already loads, costing no backend work. Two copy traps written down so the build doesn't fall into them: Remove does **not** resurrect a promotion that attaching cleared (§D5's clear is a real delete), and YAML `sources:` claims never appear in the list, so the intro says "attached to a Holodex field" rather than anything that sounds like completeness. **DD7 → accepted**, which makes it an amendment to a *shipped* F44 surface rather than a new one; a cross-reference blockquote now sits in `promote-override-fields-handoff.md` §5, and the QA gate calls for re-verifying the promote pill on `long_text` rows, not just the new Attach pill. Acceptance bullet tightened to say undo is immediate *and* later — it was previously true only for the seconds after the gesture. Next: slice B backend, then the two frontend items; no gate is outstanding.

### 2026-07-28 · design handoff written; Q3 settled, two scope questions raised
- skills: design-handoff
- handoff: [Design handoff](../design/claimed-provider-keys-handoff.md) landed — the last gate on slice B. Written as an addendum to the F44 promote handoff; the row shape, the accent inline-expander shell and `inputClass` are all inherited, so the new surface is one pill plus `ClaimFieldEditor.svelte`. **DD2 is the finding that changes backend scope**: Q3 read as a cosmetic pick between "list everything" and "list what's on screen", but `resolver.go:286` drops undecided **empty** fields from `resolved[]`, so a screen-derived picker omits precisely the target the owner needs — a person's empty `bio` is missing exactly when provB's biography key is the only one on the page. That forces a small owner-gated `GET /admin/field-targets/{entity_type}` returning the **effective** (post-promotion) set with a `merge` flag; the flag then feeds DD4's outcome preview, which is load-bearing because claims append at lowest precedence and an owner who clicks Attach on a replace field watches their text vanish unless told first. **DD3** falls out of a mismatch nobody had written down: a row is per-key but a claim is per-(provider, key), so a 2-provider row needs a checklist — attaching both is right for the duplicate-paragraph case and wrong for S2's age-certificate-vs-score. **DD8 is a recommendation, not a decision**: a claim is invisible by construction (it succeeds by deleting its own evidence), so with P1.1 deferred the feature ships a type-global config edit with no durable way to see or reverse it, while feature acceptance already promises undo. Either pull P1.1 into slice B or amend the acceptance bullet to say undo is session-scoped. DD7 (owner controls on their own line for `long_text`/`chips`) touches F44's shipped layout and is explicitly separable. Next: both open items are Kevin's call; the rest of slice B can start on the backend either way.

### 2026-07-28 · slice A built — GH #178 closed for YAML users
- skills: testing-strategy, design-handoff
- handoff: Slice A shipped: `resolver.ClaimedKeys(effective []mapping.Field)` derives `"provider:key"` from the merged field set, `AutoRegisterFields` gained `claimed` as a second suppression input beside `rendered`, and `appendAutoRegistered` gained the effective `[]mapping.Field` (all three callers already held it — `mfields` in `handlers.go`, `fields` in `person_fields.go`/`studios.go`). Ten lines of behaviour, and the doc comment now carries the reason `rendered` and `claimed` both have to stay: `rendered` catches `tmdb:overview` when `overview` renders from the `file:` baseline alone with no provider source at all, `claimed` catches `provA:synopsis` feeding `overview`. Neither subsumes the other and a test fails if either is deleted. The provider-scoped case fell out of the existing per-`(provider, key)` accumulator with no special case, exactly as ADR-074 predicted — `provA:rating` claimed with `provB:rating` not leaves a `rating` row carrying provB's value, provenance and `WinningSource` only. Unconditional suppression is structural, not a rule: `AutoRegisterFields` is handed no resolution outcome, so it cannot depend on one; the test pins the observable half by asserting identical suppression with the claimed source listed first (wins) and last (loses). FR7 shipped **with** slice A rather than after it, because the behaviour change makes an operator's existing mental model wrong: `canonical-fields.md` gained a *Claiming a provider key* section (S1–S6 + the claim/promote/do-nothing table), the F39 blockquote and the bare-key table row are amended, `configuration.md`'s promotion section links the table from the other direction, and `metadata-mappings.yaml.example` carries the commented S1 example. Full Go suite + `go vet` green. Note: `gofmt -l` flags ~10 files repo-wide including ones untouched here — pre-existing comment-alignment drift, not line endings (`.gitattributes` eol=lf is active), and CI gates on `go vet` only, so it was left alone. Next: the **design gate** is now the only thing standing between here and slice B.

### 2026-07-28 · ADR-074 written; both blocking questions settled
- skills: architecture
- handoff: [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) landed, amending ADR-056 §D4 (a cross-reference blockquote is now in ADR-056 itself). The decision worth carrying forward is **D2**: auto-registration's second suppression input is derived from the **merged `[]mapping.Field`**, not from the `field_claims` table. That single choice collapses YAML `sources:` and DB claims to one code path, covers synthesized person/studio and F44 promotions for free, and — because suppression reads the *materialized* field set — makes "value suppressed with nowhere to go" unrepresentable rather than policed, which is what settles spec Q2 (D4: a dangling claim is inert and is never pruned; the key simply auto-registers again, exactly as pre-F49). Q1 settled as D3: the row carries the target canonical only, claims append last in `(provider, field_key)` order — identity is not precedence, and ADR-051 decisions stay the winner-picking instrument. Note the PK grain differs from `field_promotions` on purpose: `provider` is in the key, because `provA:synopsis` and `provB:synopsis` are different assertions. Security stayed deferred and the ADR records the test it was deferred against, including one honest asymmetry: a claim *can* feed a filterable canonical (a promotion cannot create one), bounded by FR4's 422 that the target must be a declared field of that entity type. Next: slice A still needs nothing from anyone — ADR/design gate slice B only.

### 2026-07-27 · spec written; three decisions resolved
- skills: write-spec
- handoff: F49 spec landed at `docs/specs/claimed-provider-keys.md`. Three `needs-spec` decisions resolved with Kevin: claims live in **both** YAML `sources:` and a DB table (person/studio have no YAML at all, so in-app is the only path there); v1 is mechanism + explicit claim action with proactive detection deferred; promote and claim are mutually exclusive, claiming clears the promotion after naming it. Two things the ticket had slightly off, corrected in the spec: derive the claimed set from the **effective `[]mapping.Field`** handed to `ResolveFields` (covers video YAML + synthesized person/studio + F44 promotions in one pass) rather than from `Mappings.Fields()`, and a DB claim must **append a candidate source**, not merely suppress — otherwise the value is hidden with nowhere to go. Also: `promotionTarget`'s 422-on-canonical is not the blocker the ticket described; it applies to the key being promoted and is still correct for claims. What's missing is a target-canonical parameter, hence a separate endpoint. Next: slice A is independently shippable and closes GH #178 — no ADR or design gate needed for it.
