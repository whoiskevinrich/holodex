# ADR-061: Unified entity name-identity — normalized key + aliases + merge across People, Studios, Tags

**Status:** Proposed
**Date:** 2026-07-05
**Deciders:** Project owner

**Realizes / picks up:** [ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md) (studio entity — **realizes its deferred RD4 identity ops**: rename/aliases/merge, which ADR-053 explicitly parked as "a P2 worth building the first time two real spellings both carry libraries." The probe below is that moment) · [ADR-036](ADR-036-person-alias-search-indexing.md) / F23 (person aliases — **generalizes** its name-routing + FTS-mirror pattern from person-only to all three entities). **Complements (does not conflict):** [ADR-055](ADR-055-enrichment-unique-key-invariant.md) — ADR-055 governs **provider-supplied** identity (namespaced `external_id`, no name fallback) and repeatedly carves out owner-curated **name** identity as "a separate system, untouched." **This ADR is that separate system.** The two compose into one resolve order (§D5). **Extends:** [ADR-054](ADR-054-studio-external-id-dedup.md) (external-id-first resolve — this ADR defines the *next* step after an id miss) · [ADR-052](ADR-052-baseline-source-contract.md)/[ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (entity-generic seams — name-identity becomes the next entity-generic mechanism, the way resolution did) · [ADR-017](ADR-017-search-architecture.md) (mixed-entity FTS — alias search mirror) · [ADR-028](ADR-028-activity-surface-and-job-history.md) (one-time backfill as an observable job) · [ADR-030](ADR-030-access-control-gating-seam.md) (merge/alias mutations are owner-gated) · [ADR-018](ADR-018-scanner-change-detection.md) (scan-time resolve) · [ADR-059](ADR-059-person-link-resolved-derivation.md) (person link resolved-derivation — makes `video_people` **derived** via a generic `RelinkVideoEntity` and routes person names through the alias table at derivation time, so **person** merge now survives re-derivation like studio; convergence in D6). **Supersedes (narrowly):** ADR-053's "exact-name, no case-folding beyond binary collation" studio identity and its RD4 deferral; the tag "bare string" identity (repo.go `getOrCreateByName`). **Spec:** [entity-identity (F43)](../specs/entity-identity.md). **Issue:** HOLODEX-TBD (epic) + per-entity conformance sub-tasks.

---

## Context

Three browse-grade entities carry an authored/derived **name** as their identity: **person** (name from
file credits, F23 aliases + merge), **studio** (name derived from the resolved `studio` field, ADR-053,
**no** alias/merge), **tag** (bare string, `getOrCreateByName`, no alias/merge, not in the resolver). Their
identity handling is a **patchwork of three different collations and three different capabilities**, and the
seam that decides "is this incoming name the same entity or a new one" is inconsistent enough to be wrong.

**The load-bearing defect — the identity layer disagrees with itself about case.** Canonical `name` columns
are `UNIQUE` **case-sensitive** (binary: `people.name`, `studios.name`, `tags.name`), while
`person_aliases.alias` is `COLLATE NOCASE`. So the scan-time router
(`resolveOrCreatePerson`, `internal/repo/aliases.go`) — *exact canonical (binary) → alias (NOCASE) →
create* — can route the **same real-world name to two different entities purely by capitalization**:

- Person A has alias `"Fox"` (NOCASE); person B is canonically named `"fox"` (binary).
- File credit `"fox"` → exact-hits B. File credit `"Fox"` → misses B (binary), alias-hits A.
- Same name, two entities, decided by a capital letter. Nothing prevents the state; there is **no uniqueness
  between the canonical and alias namespaces**, and the two namespaces use **different collations**.

There are **three** reachable collision modes, not one: canonical↔canonical case split, canonical↔alias
collision, alias↔alias collision. A `normKey()` (`strings.ToLower(strings.TrimSpace(s))`,
`internal/repo/curation.go`) already exists — but it is used for **curation**, not **identity**.

### Evidence — production collision probe (2026-07-05, read-only, anonymized)

Corpus **1166 people / 143 studios / 748 tags**. [`detect_entity_collisions.sql`](../../scripts/detect_entity_collisions.sql) classifies every
normalized-key collision:

| Tier | person | studio | tag | Character |
|---|---|---|---|---|
| **A** — hard case/whitespace collisions (index-blocking) | 6 | 3 | 5 | **100% pure-case, all 2-way, all canonical** (zero alias tangles, zero whitespace) |
| **B** — punctuation/spacing near-misses (review candidates) | 8 | 7 | **41** | ~38 internal-whitespace, ~18 punctuation |

Two facts drive the whole design:

1. **Migration risk is small and safe.** The 14 hard collisions are *all* pure-case 2-way canonical pairs
   (`"fox"`/`"Fox"`). Case-folding the identity key resolves **100%** of them with no homonym ambiguity —
   nobody authored `"fox"` and `"Fox"` as deliberately-distinct entities.
2. **The near-miss cleanup is a tags problem.** Tags are **41 of 56** near-misses (labels drift:
   `sci fi`/`sci-fi`/`scifi`), vs 8 person + 7 studio. A review queue's customer is tags. All 3 studio
   collisions have **zero external ids** (file-born) — the case ADR-054's provider-id dedup structurally
   cannot cover.

### Current state

| Entity | Canonical collation | Aliases | Merge | Name is… |
|---|---|---|---|---|
| **Person** | `UNIQUE` binary | `person_aliases` NOCASE (+ FTS, ADR-036) | `MergePersons` (F23) | authored (file credit) |
| **Studio** | `UNIQUE` binary | none (ADR-053 RD4 deferred) | none | **derived** from resolved `studio` field |
| **Tag** | `UNIQUE` binary | none | none | authored (file tag) |

### Forces

- **Case and stray whitespace are never semantic identity.** The alias table already believes this (`NOCASE`);
  the canonical columns must agree, or the split above persists.
- **But near-misses are not all safe to auto-collapse.** Spelling/punctuation/word-order differences can be
  real distinctions (`"rom-com"` vs `"rom com"` = same; two studios spelled differently may be two companies).
  The owner's rule stands: **never silently merge across an ambiguous boundary** (the homonym rule, ADR-055).
- **Entities are born on two paths with different affordances.** The **scanner** is non-interactive (no
  prompt possible); the **editor** is interactive. One collision policy cannot serve both.
- **Studio identity is *derived*, so merge must survive re-derivation** (ADR-053): repointing links is
  undone on the next reconcile unless the merged name routes to the survivor — i.e. merge *needs* an alias.
- **One mechanism, every entity** (the ADR-052 lesson). Identity is the last per-entity special case; it
  should become entity-generic like resolution, not a fourth bespoke design when video-credits/tags grow.
- **Name-identity ≠ provider-identity** (ADR-055). This governs the human/file-authored side only; the
  provider→entity link stays id-first.

---

## Decision

Introduce a **shared entity name-identity spine** — one normalized-key + alias + merge + keep-separate
mechanism, instantiated per entity — and make it the identity/de-dup key for **name-derived** entities.
Seven sub-decisions.

### D1 — A normalized name key is the identity, spanning canonical ∪ aliases

Every entity type has a **`nameKey = normalize_<entity>(name)`** that is **unique per entity type across
both canonical names and aliases** (one uniqueness domain, not two). Replaces the binary `UNIQUE(name)` and
the separate NOCASE alias uniqueness with a single per-entity key. This closes all three collision modes by
construction: a canonical name, an alias, or a second canonical can never occupy the same `nameKey` twice.

### D2 — Normalization scope is per-entity (evidence-driven)

`normalize` is a **registry keyed by entity type**, not one global function:

| Entity | Hard key folds | Rationale (from probe) |
|---|---|---|
| **Person** | case + **edge** whitespace | Names are curated; internal spacing can be meaningful (`De La Cruz`). Only 8 near-misses. |
| **Studio** | case + edge whitespace | Same conservatism; external-id already handles provider dedup (ADR-054). |
| **Tag** | case + **all** whitespace (edge *and* internal) | Labels, high drift (41 near-misses, ~⅔ internal-whitespace). Folding internal space auto-resolves ~27 tag near-misses safely. |

Diacritic/punctuation folding stays **out** of the hard key for all three (FTS already folds diacritics for
*search*; folding them into *identity* risks false merges) — those differences flow to the review queue (D3).

### D3 — Two-path collision handling

- **Scan (non-interactive):** resolve by `nameKey`; a hard-key match routes (this is where case/whitespace
  variants collapse — they can never *create* a second entity). A genuinely-fuzzy near-miss (different
  `nameKey`, but a loose-key match) **creates the entity** and **flags the pair to a review queue** — never
  auto-merges. Scanning stays deterministic and prompt-free.
- **Editor (interactive):** creating/renaming/aliasing runs the same near-miss detection and **prompts**
  ("this looks like *X* — merge instead of create?"). The human decides in the moment.

Detection compares the loose key (defined in [`detect_entity_collisions.sql`](../../scripts/detect_entity_collisions.sql): lowercase + strip
whitespace/punctuation) — a cheap, pure-SQL proxy; edit-distance is a possible later refinement, not required.

### D4 — A `keep-separate` assertion (the negative of an alias)

A persisted **"these two entity ids are deliberately distinct"** marker. Dismissing a review-queue suggestion
records it, so the detector never re-proposes that pair on the next scan. Without it the queue re-nags forever
(the homonym pairs it surfaces are, by definition, ones the owner wants kept apart). It is the durable
counterpart to an alias: an alias says "these are the same," keep-separate says "these are not."

### D5 — Resolve order: provider id first (ADR-055), then name key (this ADR)

The two identity systems compose into one deterministic order at resolve-or-create:

1. **External-id match** — `<namespace>:<id>` (ADR-054/055), when a provider supplied one. *Unchanged.*
2. **Name-key match** — `nameKey` over canonical ∪ aliases (this ADR). Replaces today's binary-exact→NOCASE-alias steps.
3. **Create** — new entity (scan) or **prompt** (editor), per D3.

This is exactly the "separate system" ADR-055 D1 carves out: provider data is id-keyed; **name is
display-only for provider data** but **is identity for file-authored/owner-curated data**. No conflict —
different inputs, one ordered pipeline.

### D6 — Studio merge is durable *because* it registers an alias

Merging studio B→A: repoint `video_studios`, move decisions/curation/enrichment, **register B's name as an
alias of A**, delete B. The alias is load-bearing: ADR-053 derives `video_studios` from the resolved `studio`
field, so without the alias the next `RelinkVideoStudios` would re-run `resolveOrCreateStudio(B.name)` and
**resurrect B**. The alias makes step 2 of D5 route B's name to A, so the merge survives derivation and
prune-on-empty. (This is the precise reason ADR-053 could *defer* merge — it had no alias table — and why
this ADR must *add* one.) Person merge already does this (`MergePersons` registers the merged name as an
alias); studio and tag inherit the pattern.

**Convergence with ADR-059 (person links become derived too).** ADR-059 (F40) migrates `video_people` from
raw scan-time extraction to **resolved-value derivation** via a generic `RelinkVideoEntity` (folding
`RelinkVideoStudios` in), and routes person names through `resolveOrCreatePerson`'s alias table **at
derivation time**. So the alias-survives-re-derivation property this decision establishes for studio becomes
the property person **also depends on** — a person merge without the registered alias would be undone by
`RelinkVideoEntity` exactly as a studio merge would. This is a convergence, not a conflict: `RelinkVideoEntity`
must resolve person names through **this ADR's `resolveOrCreateByName`** (the id→name→create order, D5), and
ADR-059's authored-identity prune guard is satisfied by the registered alias (an alias *is* authored identity).
The two ADRs are complementary; whichever lands second wires its derivation reconcile to call the shared
resolver rather than a second name-routing path.

### D7 — Tags get the identity spine, **not** the field-resolution decision model

Tags gain `nameKey` + aliases + merge + keep-separate + an FTS alias mirror. Tags do **not** gain a
`BaselineSource`, per-field source decisions, or curation (ADR-051/052). A tag is a **single-field** entity —
its name *is* the entity — so the multi-field decision machinery would be ceremony with no payoff. This is
the load-bearing scoping call: **"same entity model" means the same *identity* spine, not the same
*field-resolution* model.** (ADR-055's conformance table already anticipates a future `tag_external_ids` if
tags ever become enrichable; that is out of scope here and orthogonal to name-identity.)

### Shape (the shared spine)

Consistent with the **already-polymorphic** entity-typed stores (`entity_enrichment`, decision, curation —
all keyed by `entity_type`, migration 0016), the spine is **polymorphic, not three copies**:

- `entity_aliases(entity_type, entity_id, alias, alias_key, …)` with `UNIQUE(entity_type, alias_key)`, plus a
  single `entity_aliases_fts` mirror filtered by `entity_type` in search (refines ADR-036's dedicated
  `person_aliases_fts` to a shared mirror; F23's table is migrated in).
- `entity_keep_separate(entity_type, id_lo, id_hi)` (D4).
- The canonical `nameKey` uniqueness is a per-entity **unique expression index** on `normalize_<entity>(name)`
  (no stored column; SQLite expression indexes encode the per-entity normalize of D2).
- A Go **name-identity package** (`resolveOrCreateByName`, `Merge`, `Alias`, `Rename`, near-miss `Detect`)
  parameterized by entity type + its normalize function — the identity twin of `ResolveFields` over
  `BaselineSource`.

### Migration + backfill (one-time, observable)

Migration `NNNN` builds the unique name-key indexes and the two tables and consolidates `person_aliases`. A
startup pass (a System Activity job, ADR-028, idempotent — the ADR-053 backfill pattern):

- **Auto-folds the 14 hard pure-case pairs** — safe, since all are 2-way canonical case-only (survivor = the
  lower `id`; the other's name becomes an alias; decisions/curation/enrichment move, not drop, where they
  don't conflict). The index build cannot fail on residual dupes because the pass resolves them first.
- **Seeds the review queue** with the ~56 near-misses — it does **not** auto-merge them.

---

## Options Considered

### Identity key — how "same name" is decided

#### A — Per-entity `normalize()` function + unique expression index (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — a normalize registry + expression indexes + one migration |
| Correctness | Closes all three collision modes; per-entity scope matches the data |
| Familiarity | High — extends the existing `normKey` + entity-typed-store patterns |

**Pros:** One uniqueness domain over canonical ∪ aliases; per-entity scope (tags fold internal whitespace,
people don't) is expressible; folds *more than case* where the data warrants. **Cons:** Expression indexes are
less obvious than a plain column; a `normalize` change is an index rebuild.

#### B — `COLLATE NOCASE` on the canonical column

**Pros:** One-line schema change; fixes the pure-case collisions (100% of Tier A). **Cons:** Case is *all* it
can do — no per-entity whitespace scope (tags need internal-whitespace folding), no shared canonical∪alias
domain, no hook for near-miss detection. Rejected: solves Tier A only and re-freezes the collation split at a
different setting.

#### C — A single `entity_names` table holding canonical + alias rows uniformly

**Pros:** The purest "one identity domain" — canonical is just a `kind='canonical'` row. **Cons:** Moves every
entity's canonical name out of its own table, a large refactor touching every read path, for a uniqueness
property expression indexes already give. Rejected: cost ≫ benefit; revisit only if a fourth entity makes it pay.

### Spine shape — shared vs replicated

**Polymorphic `entity_aliases`/`entity_keep_separate` (chosen)** matches the entity-typed enrichment/decision/
curation stores and gives one code path; cost is migrating F23's `person_aliases` + re-pointing its FTS mirror
(ADR-036). **Per-entity tables mirroring `person_aliases` (rejected)** — three near-identical tables + three
FTS mirrors to keep in sync; the "replicate per entity" option the owner declined.

### Scan-time collision policy

**Hard-normalize meaningless axes + queue the fuzzy (chosen)** — case/whitespace can never create a dupe;
spelling/punctuation defers to a human. **Never auto-collapse, queue everything (rejected)** — buries the
owner in `Action`/`action` confirmations forever. **Hard-normalize only, no queue (rejected)** — fastest, but
the 41 tag near-misses just accumulate silently as dupes.

---

## Trade-off Analysis

**One name-identity mechanism vs. three special cases.** The codebase already made resolution entity-generic
(ADR-052) and the enrichment/decision/curation stores entity-typed. Identity is the last divergence: person
has aliases+merge, studio has neither, tag has nothing. Collapsing them onto one `normalize`-parameterized
package + polymorphic alias/keep-separate store is the same move, and it means video-credits (F32) or any
future entity inherit identity for free. The cost — migrating F23's proven `person_aliases` — is real but
one-time and mechanical; the alternative is maintaining three copies of subtle routing logic forever.

**Auto-fold vs. confirm — resolved *by the data*.** The general worry with an auto-merge backfill is
destroying a real distinction you can't tell from a typo. The probe removes the worry for the migration
specifically: every hard collision is a pure-case 2-way canonical pair, which is unambiguous. So the backfill
auto-folds the 14 and — crucially — **does not** auto-touch the 56 near-misses, which are exactly the
ambiguous class. The dangerous operation is scoped to the provably-safe input; the ambiguous input goes to a
human via the queue. This is why the probe was worth running before the ADR.

**Per-entity normalize vs. one global key.** A single normalize would be wrong for two of three entities:
fold internal whitespace globally and you risk merging distinct person names; fold it nowhere and 27 tag
near-misses stay dupes. The per-entity registry is slightly more machinery for a strictly better fit, and the
data — not taste — sets each entity's dial.

**Review queue: real but modest, and tag-shaped.** 56 one-time pairs (73% tags) is a cleanup surface, not a
high-volume safety system. It ships *after* the identity fix and its justification is tag hygiene; the
keep-separate assertion is the only part that must be durable (so it stops nagging). Right-sizing it P2 avoids
gold-plating a homonym-safety machine the collision data says isn't needed.

**Composition with ADR-055, not contradiction.** ADR-055 forbids name as a *provider* identity key; this ADR
makes name *the* key for file-authored entities. They meet at the D5 resolve order (id → name → create) with
no overlap: provider input is id-keyed, file/owner input is name-keyed. Documenting the boundary here is what
keeps a future contributor from "fixing" one by breaking the other.

---

## Consequences

**What becomes easier**
- `"fox"`/`"Fox"` (and the other 13 pairs) stop being two entities; the canonical∪alias domain makes the
  split unrepresentable, not just cleaned-up-once.
- Studios and tags gain rename/alias/merge with the same UX and code as people; studio merge survives
  ADR-053 link re-derivation via the registered alias.
- Any future named entity (video credits, an enrichable tag) inherits identity with a normalize function and
  an `entity_type`, no new design.
- Tag hygiene becomes a first-class, owner-driven surface instead of an accreting pile of near-dupes.

**What becomes harder**
- A per-entity `normalize` is now load-bearing: changing it is an index rebuild + a re-scan of near-misses. It
  must be treated like a migration (documented, versioned).
- Two identity systems (id-first, name-first) coexist; a contributor must know which input they're resolving.
  Mitigated by the single D5 resolve order and this ADR as the reference.
- The scanner now has a side effect (queue-flagging) beyond create/link; kept idempotent and behind the same
  scan transaction discipline.

**What we'll need to revisit**
- **Review-queue UX + near-miss algorithm** — loose-key is the v1 detector; edit-distance/phonetic is a later
  refinement if tag drift outpaces it (`/design-handoff` owns the surface).
- **`person_aliases` migration** — the F23 FTS mirror (ADR-036) re-points to the shared `entity_aliases_fts`;
  verify search parity before dropping the old table.
- **Studio derived-name interaction** — confirm merge-alias + prune-on-empty + keep-separate compose cleanly
  across a re-enrich/decision-change (the ADR-053 derivation matrix gains identity rows).
- **ADR-059 resolver convergence** — whichever of F43 (this) / F40 (ADR-059) lands second must wire
  `RelinkVideoEntity`'s person name-routing to this ADR's `resolveOrCreateByName` (one resolver, not two), and
  confirm the authored-identity prune guard treats a merge-registered alias as authored identity (it does — an
  alias blocks the orphan prune). The "person merge survives re-derivation" case joins the testing matrix.
- **`tag_external_ids`** (ADR-055 future row) — only if tags become enrichable; independent of this ADR.

---

## Action Items

1. [ ] `/write-spec` — entity-identity (F43): resolve order (D5), per-entity normalize (D2), the two-path
   policy (D3), review-queue lifecycle + keep-separate (D4), tag scope (D7). Assign the feature number.
2. [ ] Migration `NNNN`: unique expression indexes on `normalize_<entity>(name)` (person/studio/tag);
   `entity_aliases` (+ shared `entity_aliases_fts`, migrate `person_aliases` in); `entity_keep_separate`;
   matching `.down.sql`.
3. [ ] `internal/repo` name-identity package: `resolveOrCreateByName`, `Merge`, `Alias`, `Rename`,
   near-miss `Detect`, parameterized by entity type + normalize; re-point `resolveOrCreatePerson`,
   `resolveOrCreateStudio`, and tag `getOrCreateByName` through it (D1/D5/D6).
4. [ ] Scanner near-miss flagging → review queue (D3); ensure idempotent + inside the scan transaction discipline.
5. [ ] One-time backfill job (ADR-028): auto-fold the 14 hard pairs (survivor = lower id, loser name→alias,
   move-not-drop decisions/curation/enrichment); seed the queue with the ~56 near-misses; idempotent.
6. [ ] Owner-gated (ADR-030) merge/alias/rename + review-queue resolve/dismiss endpoints for studio + tag
   (mirror the person endpoints); editor-side detect+prompt.
7. [ ] Add this ADR to `docs/architecture/README.md`; cross-reference from the F43 spec, ADR-053 (RD4 now
   realized), and ADR-055 (the name-identity companion).
8. [ ] `/testing-strategy`: collision matrix (all three modes × scan/editor), per-entity normalize scope,
   backfill auto-fold-vs-queue split, studio merge-survives-rederivation, keep-separate non-nag, FTS parity
   after `person_aliases` migration.
9. [ ] `/security-review` before merge: owner-gating on all new mutations; untrusted names through the
   existing sanitize perimeter; confirm no media-file writes (identity is DB-only, F37/ADR-053 precedent).
10. [ ] Create the HOLODEX epic + per-entity conformance sub-tasks; record the key in the branch name.
