# ADR-095: Structured resolve hints — `hint.fields`, `hint.filename`, `hint.query_source`, and `searched[]`

**Status:** Proposed
**Date:** 2026-09-11
**Deciders:** Project owner (converged with the partner video provider over three rounds, 2026-09-11)

**Extends:** [ADR-080](ADR-080-configurable-provider-search-patterns.md) (revisits **D1 only** — the
decision that the `/resolve` hint is one flattened string; D2–D5 stand, and §4.9 gains a render rule) ·
[ADR-033](ADR-033-metadata-source-plugins.md) (provider sidecar contract — `/describe`/`/resolve`) ·
[ADR-056](ADR-056-provider-field-render-hints.md) (the additive, no-protocol-bump `/describe` extension
idiom this reuses for the opt-in list).
**Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) / [ADR-052](ADR-052-baseline-source-contract.md)
(`hint.fields` carries *resolved* values — post-decision, post-curation — which is also why it can be
poisoned by a wrong match and `filename` cannot) · [ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md)
(the batch path this ADR now aims better) · [ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md)
(the snapshot revert HOLODEX-370 made discoverable, which had to land first) ·
[ADR-028](ADR-028-activity-surface-and-job-history.md) (the enrich-run activity
entry `searched[]` is recorded into on the batch path).
**Contract:** [metadata provider contract](../specs/metadata-provider-contract.md) §2.2 (`/describe`
gains `resolve_hints`) / §2.3 (`/resolve` request gains three optional `hint` keys, response gains
`searched[]`) / §4.9 (residue rule) / §4.10 (new deep-dive: the opt-in, the miss definition, the
provider's obligations) / §5 (`searched[]` caps).
**Spec:** [Configurable provider search patterns (F54)](../specs/configurable-provider-search-patterns.md)
(the P1-a caption slot this replaces).
**Issue:** [HOLODEX-367](https://whoiskevinrich.atlassian.net/browse/HOLODEX-367) (epic) ·
[HOLODEX-368](https://whoiskevinrich.atlassian.net/browse/HOLODEX-368) (request side) ·
[HOLODEX-369](https://whoiskevinrich.atlassian.net/browse/HOLODEX-369) (picker caption) ·
[HOLODEX-370](https://whoiskevinrich.atlassian.net/browse/HOLODEX-370) (landed first, Done).

---

## Context

ADR-080 D1 chose to render every `/resolve` search into **one flattened string** — `hint.query` — and
rejected structured `hint.fields` (its Option B) for v1 "pending evidence": *revisit if/when a second
provider wants query control finer than a pre-built string.* That evidence now exists, from two probes
a partner video provider ran against its upstream on 2026-09-11.

**Probe 1 — the blob cannot be decomposed.** The provider replayed 32 interactive resolves. 18 matched
correctly at rank 1 with the §4.9 blob. 9 returned zero hits — and **6 of those 9 are reachable with a
performers + studio query with the title dropped.** That fallback strategy requires knowing which part
of the blob *is* the title, and the §4.9 grammar is space-joined and undelimited by design
(ADR-080 D3): `MyStudio My Title Some Actor` gives a provider no way to tell where the studio ends.
The provider can only ever search the whole string or nothing.

**Probe 2 — the raw filename is the highest-recall input, not merely the non-circular one.** Three real
bracketed release filenames (`[Studio] Title (Person, date) 1080p.ext`), each with a known-correct
catalog id, sent verbatim to the upstream's **filename matcher**: correct record, rank 1, sole hit,
3 of 3. The same three strings sent to the upstream's **free-text search**: zero hits, 3 of 3 — the
bracket/paren grammar is unsearchable as literal text. The §4.9 blob for the same files also matched,
but on bracket-less input the filename matcher degrades to the free-text engine, so the blob can only
ever *approximate* what the raw basename reaches directly. Two smaller findings from the same probe:
the upstream folds case (a lowercase person name in the filename still matched), and free-text search
ranks a bare title differently from the filename matcher — so `filename` and a composed query are not
interchangeable inputs; a provider routes them to different endpoints.

**A duplication report from the same review.** With `{studio} {title} {performers} {year}` and a
freshly scanned file whose `title` *is* its filename stem, the rendered query repeats the studio and
every performer twice — once from the structured tokens and once inside the title. The ADR-080 D4
sanitizer strips punctuation, not redundancy.

**The batch path never used §4.9 at all.** ADR-080 Action Item 5 disclosed a scope trim: refresh-all's
`enrichQueryHint` (`internal/api/enrich_review.go`) sends only the sanitized title, because it builds
one hint shared across every provider in a concurrent fan-out. That is the path with **no owner present
to retype the query** — the one where a better-aimed hint matters most, and the one ADR-066 auto-apply
rides on. It is also why [HOLODEX-370](https://whoiskevinrich.atlassian.net/browse/HOLODEX-370) had to
land first: a better-aimed batch search for a possibly-wrong record needed the owner to be able to see
and undo a written-back provider before this ADR made that search better.

### Forces

- **§2.3 states no unknown-key rule for the `/resolve` request body.** §2.2's "unknown keys ignored"
  budget covers the *manifest*; nothing in the contract promises a provider will tolerate new keys in a
  request it receives. Sending new hint keys unconditionally would be a wire-contract break for any
  strictly-decoding provider. New request keys therefore need an explicit opt-in.
- **Writeback closes a loop that `hint.fields` can be caught in.** `hint.fields` carries resolved values.
  After an owner writes provider X's values back to the file, re-extract makes them the file-layer
  baseline ([ADR-073](ADR-073-post-write-baseline-resync.md)/[ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md): `file:title` *is* `videos.title`) — so a wrong match, once written, is
  what every later resolve searches for. Holodex never renames media, so the basename is the one input
  that cannot be poisoned this way.
- **Default-allow is load-bearing.** A fleet default of "off" for `filename` would quietly cut recall for
  every provider whose upstream has a filename matcher, with nothing in the response to show why. The
  operator escape hatch must exist (some operators will not want basenames leaving the box), but the
  default must be the recall-preserving one.
- **Transparency has to ride the same change.** Today the owner sees the seeded query and nothing
  else. Once a provider may issue *several* upstream queries (a filename lookup, then a fields
  fallback), "no results" with no record of what was actually tried is a debugging dead end — and on
  the batch path there is no picker at all.
- **No richer pattern DSL.** ADR-080 D3's grammar stays the low-effort option; the answer to "the blob
  is undelimited" is to send the structure, not to invent delimiters.
- **Simplicity-first, as ADR-080.** Each new key must be derivable from data the handler already has in
  scope at the existing choke points (`videoHint`, `buildVideoQueries`, `enrichQueryHint`); no new
  resolution plumbing, no new store, no new endpoint.

---

## Decision

`/resolve`'s `hint` gains three **optional, opt-in** keys — `fields`, `filename`, `query_source` — sent
only to a provider that advertises wanting them; §4.9 rendering gains a **residue rule** that stops the
title from repeating the structured tokens; and the `/resolve` **response** gains `searched[]`, the
upstream queries the provider actually issued. `hint.query` keeps exactly its ADR-080 shape and
precedence. Eight sub-decisions:

### D1 — Opt-in via the manifest: `/describe.resolve_hints`

`enrich.Manifest` gains one optional key, `resolve_hints: string[]`, valued from `"fields"` and
`"filename"`. Holodex sends `hint.fields` only to a provider that lists `"fields"`, and `hint.filename`
only to one that lists `"filename"`; `hint.query_source` rides with either. A provider that omits the
key receives exactly today's request. Unknown list entries are ignored, known ones honored — the same
additive idiom as ADR-056's `field_hints` and ADR-080's `preferred_search_pattern`, and no
`protocol_version` bump.

**Why opt-in rather than "unknown keys ignored":** §2.3 makes no such promise for the request body
(see Forces). Opt-in is the only way to add request keys without a protocol bump that stays honest with
a strictly-decoding provider.

**Operator deny, `filename` only, default allow.** `enrich.Source` gains a per-source boolean in
`metadata-sources.yaml` (e.g. `send_filename: false`) that withholds `hint.filename` from that provider
even when it opts in. Default **allow**, per the Forces: the deny exists for operators who don't want
basenames leaving the box, not as a fleet posture. There is no operator deny for `fields` — those
values are already what the §4.9 blob sends today, just delimited.

### D2 — `hint.fields`: resolved canonical values, as-is, intersected with the provider's vocabulary

Shape mirrors the `/enrich` response's `fields` object — `{ "<canonical key>": [values…] }` — so the
vocabulary is one a provider already speaks:

- **Keys:** canonical keys (§4.2a) **∩** the provider's own advertised `/describe.fields`. A provider
  is never sent a key it did not say it understands.
- **Values:** the field's **resolved** values — post-decision, post-curation (ADR-051/052), the same
  `resolver.ResolvedField` slice `buildVideoQueries` already reads. Multi-valued fields carry every
  surviving value; nothing is capped to §4.9's `performersCap` here, because the point of structure is
  that the provider decides what to use.
- **Sent as-is.** No redundancy filtering against `title`, no sanitizer pass. The residue rule (D5) is a
  §4.9 *render* rule and does not touch `fields`.
- **Porting note (in the contract):** §4.9's `{performers}` token is `actors` + `director` merged; in
  `fields` they arrive as two separate keys. A provider composing its own fallback query merges them
  itself.

### D3 — `hint.filename`: the basename, verbatim

`filepath.Base(v.FilePath)` — extension included, brackets and parens intact, **no sanitizer pass.**
The grammar ADR-080 D4 strips (`[`, `]`, `(`, `)`, commas, `1080p`) is precisely what an upstream
filename matcher consumes (Probe 2); sanitizing it would hand the provider the same input the blob
already approximates. Never the directory, never the full path.

Documented in the contract as **the one input writeback cannot poison**: Holodex never renames media,
so the basename is the file's original release identity regardless of what has since been written
into its tags. It is also documented as *not interchangeable* with `fields` — a provider should route
the basename to a filename matcher and composed terms to free-text search, since the two rank
differently. Which upstream endpoint that is remains provider-internal; the contract explains *why*
both keys exist, not how to route them.

### D4 — `hint.query_source`: `"pattern"` | `"user"`, derived server-side

Tells the provider whether `hint.query` is the string Holodex rendered (`"pattern"`) or something the
owner typed (`"user"`), so the provider knows whether the query already encodes structure it can trust
or is a free-form override it must issue first (D7). Derived in the handler, never trusted from the
client: `enrichVideoResolve` re-renders through `Source.BuildQuery` from the same resolved slice in the
same call and compares the result to the submitted string — equal → `"pattern"`, else `"user"`. The
sanitized-title floor counts as `"pattern"` (it is a render, just of a one-token pattern). The batch
path is always `"pattern"`. A concurrent-edit race (the owner changes a field between page load and
submit, so the re-render differs) classifies as `"user"` — benign: the provider treats a
`"user"` query as authoritative and issues it first, which is the conservative reading either way.

### D5 — Residue rule: §4.9 drops `{title}` when it is only the other tokens

At §4.9 render time — and **only** there — tokenize the sanitized title (case-insensitive, Unicode
word tokens) and strip every token that matches the resolved studio, any resolved performer, or any
date token (`YYYY`, `YYYY-MM-DD`, `YY.MM.DD` and the like). If **no Unicode alphanumeric residue**
remains, the `{title}` / `{title?}` token renders empty for this pass. Lossless by construction: every
word dropped is already present in the query from the token that matched it. Provenance-independent:
it keys on content, not on whether `file:title` came from a tag or a stem, because there is no such
marker (ADR-093).

**A residue-dropped `{title}` is *rendered-empty*, not *missing*.** It does not trip ADR-080 D3's
required-token failure and fall the tier through — if it did, the tier would collapse to the
sanitized-title floor and send exactly the duplication the rule exists to prevent. The rule
therefore lives inside the render of the token, below the required/optional check.

Case-insensitive because Probe 2 confirmed the upstream folds case; that is the right floor for a
lossless comparison as well. The residue rule does **not** apply to the floor tier (a lone title has
nothing to be redundant with) and does **not** apply to `hint.fields` (D2).

### D6 — `searched[]`: the provider reports what it actually asked upstream

The `/resolve` response gains an optional `searched: string[]` — the upstream queries the provider
issued, in issue order. Caps live in §5 alongside the existing ones: ≤ 10 entries, ≤ 4096 chars each,
no newlines; sanitized on ingest exactly like `candidates[].label` (control chars stripped, capped).
Omitted when empty; video-only. **Not** manifest-gated — Holodex's decoder already ignores unknown
response keys, so a provider may start emitting it today.

**Two consumers, one per path.** Interactive: the picker shows a "Searched: …" caption — the
transparency caption ADR-080's design handoff specified as optional P1-a and never built; that slot
is replaced by this, since "what did it search" is the more useful question once a provider issues
more than one query. Batch: `searched[]` is recorded into the enrich-run activity entry
(`job_runs.detail`, F22.6b), the only place a no-owner-present run can leave a trace. The detail
invariant ("provider name + entity id + counts, never a filesystem path") is honored: a basename a
provider echoes back is not a path, and the same basename already renders on the owner's media page.

**Decided now, shipped second** — the request side (D1–D5) lands without it, so the provider can
emit `searched[]` before Holodex reads it and nothing waits on the caption's design gate.

### D7 — Miss definition and query order (contract text, provider-side obligations)

A **miss** is *no candidates surfaced* — upstream-empty **or** every upstream hit gated out by the
provider's own scoring. That definition matters because it decides when a provider may fall back:

- A `"user"` `hint.query` (D4) **MUST** be issued first, verbatim. The owner typed it; it outranks
  anything Holodex composed.
- After a miss, the provider **MAY** compose a fallback from `hint.fields` (D2) — the
  performers + studio strategy Probe 1 recovered six records with.
- Every query issued, in order, appears in `searched[]` (D6), so the owner can see both the first
  attempt and the fallback.

Holodex enforces none of this — it is what a conformant provider owes, written into §2.3/§5.

### D8 — Both paths, one hint builder

The interactive path (`enrichVideoResolve` → `videoHint`) and the batch path (`enrichQueryHint`) both
gain `fields`/`filename`/`query_source`, and the batch path gains §4.9 pattern rendering it never had.
That means the batch fan-out (`enrichRefreshAll` → `refreshOneProvider`) builds its hint **per
provider** inside the goroutine rather than once outside it — the restructuring ADR-080 Action Item 5
deferred. `filename` on the batch path is where D3 matters most: it is the path with no owner to
retype the query, and the one ADR-066 auto-apply acts on.

---

## Options Considered

### D1 — how new request keys reach a provider

#### A — manifest opt-in list `resolve_hints` (chosen)
**Pros:** honest against §2.3, which promises nothing about unknown request keys; a provider that has
not opted in is byte-for-byte unaffected; one list can grow (`"query_source"` is implied, future
`"person"`/`"studio"` shapes fit) without a second mechanism. **Cons:** a provider must ship a manifest
change to receive anything — acceptable, it must ship decoding code anyway.

#### B — send unconditionally, rely on "unknown keys ignored"
**Pros:** nothing to advertise. **Cons:** the rule does not exist for the request body; a
strictly-decoding provider would start failing every resolve on upgrade. Rejected.

#### C — bump `protocol_version` to 2
**Pros:** the formal answer. **Cons:** a major bump makes Holodex *refuse* every v1 provider (§2.2);
for three optional keys that is disproportionate, and every prior manifest extension has been
additive. Rejected.

### D2 — what `hint.fields` carries

#### A — resolved values, as-is, ∩ provider vocabulary (chosen)
**Pros:** one vocabulary the provider already speaks (`/enrich`'s); nothing Holodex would have to
second-guess; ADR-051's decisions and curation are honored because the resolver already applied them.
**Cons:** a wrong match written back to the file poisons these values (Forces) — mitigated by
`filename` (D3) existing precisely as the unpoisonable input, and by HOLODEX-370 making the revert
discoverable.

#### B — redundancy-filtered (drop values already inside `title`)
**Pros:** smaller payload. **Cons:** Holodex would be deciding what the provider's search can use,
which is the ADR-080 D1 mistake again in miniature; the provider asked for structure precisely so it
can decide. Rejected.

#### C — raw file-layer values only (bypass decisions/curation)
**Pros:** immune to a wrong *decision*. **Cons:** not immune to a wrong *writeback* (the file layer is
what gets poisoned), and it would silently ignore the owner's standing corrections. Rejected.

### D3 — what `hint.filename` carries

#### A — basename verbatim (chosen)
**Pros:** Probe 2 — 3/3 rank-1 through a filename matcher, 0/3 through free-text; the
unpoisonable input. **Cons:** a basename leaves the box. Mitigated by opt-in (a provider must ask) and
the operator deny; nothing else in the path (`videos.title` for an un-tagged file *is* the stem) is
meaningfully more private than the stem already sent today.

#### B — sanitized basename (D4 sanitizer applied)
**Pros:** consistent with the blob. **Cons:** destroys exactly the grammar the matcher consumes; the
provider would get a second copy of what `hint.query` already approximates. Rejected.

#### C — full path
Never. Directory structure is operator-private and no matcher wants it. Rejected.

### D5 — the duplication fix

#### A — residue rule at §4.9 render (chosen)
**Pros:** lossless (every dropped word is still in the query), provenance-independent, no new config,
and it composes with the provider-side copy (the provider committed to the same rule in its composer).
**Cons:** one more small piece of string handling in `query.go`; the tokenizer needs a date-token
definition. Both bounded.

#### B — a stem-vs-tag provenance marker on `title`
**Pros:** would let the rule fire only for filename-derived titles. **Cons:** no such marker exists
(`file:title` is `videos.title`), and adding one is a schema change for a distinction the content-based
rule does not need. Rejected.

#### C — a richer pattern DSL with delimiters / literals
**Pros:** would let an operator express `{studio} — "{title}"`. **Cons:** ADR-080 D3 already declined
literal decoration; the actual problem (the provider cannot decompose the blob) is solved by D2, not by
prettier blobs. Rejected; §4.9 stays the low-effort option.

### D6 — where the transparency lands

#### A — `searched[]` on the response, caption + activity detail (chosen)
**Pros:** the provider is the only party that knows what it asked upstream; the two consumers cover
the two paths. **Cons:** a second story with a design gate. Accepted — "decided now, shipped second."

#### B — Holodex logs what it *sent* and calls that transparency
**Pros:** no contract change. **Cons:** what Holodex sent and what the provider searched now diverge
by design (D7's fallback); logging the request answers the wrong question. Rejected.

---

## Trade-off Analysis

**Structure vs. provider burden.** ADR-080 D1 kept the wire flat so a provider needed nothing beyond
a free-text search. This ADR hands a provider structure it *may* use — but only after it asks (D1),
and `hint.query` is still there unchanged for a provider that never does. The burden is moved to
exactly the providers that wanted it.

**Recall vs. what leaves the box.** `filename` is the highest-recall input and it is also the most
"raw" thing Holodex has ever sent a provider. The resolution is layered: a provider must opt in, an
operator can deny per source, and the default is allow because the alternative silently loses the
recall the whole ADR exists for. The security review (Action Item 7) is scoped to this exact tension.

**Poisonable vs. unpoisonable inputs.** `fields` follows the owner's decisions and can be poisoned by
a written-back wrong match; `filename` ignores the owner's decisions and cannot be. Sending both is
deliberate — the provider gets one input that reflects curation and one that reflects the original
release — and it is why HOLODEX-370 (see and undo the writeback) had to precede this ADR rather than
follow it.

**A render rule vs. a provenance marker.** The residue rule fires on content, so it also fires when a
title *legitimately* equals the studio + performers (a scene named after its cast). That is accepted:
the query loses nothing in that case either, because every word is still present from its own token.

**Two stories, one decision.** `searched[]` is decided here so the provider can build against a
settled contract, but shipped after the request side so the caption's design gate does not hold up the
recall gains. The cost is a window where providers emit a key Holodex ignores — harmless by the
decoder's existing rule.

---

## Consequences

**What becomes easier**
- A provider can decompose the search (title vs. studio vs. performers) and run a fallback strategy
  after a miss — the six records Probe 1 could reach and the blob could not.
- A provider with a filename matcher gets the raw basename and can match the release directly — 3/3
  in Probe 2 where free-text was 0/3.
- The batch path stops sending a bare sanitized title and gets the same per-provider hint as the
  picker — the path with no owner present finally searches as well as the one with.
- The owner can see what a provider actually searched, on both paths, instead of inferring it from
  "no results".
- The "title = studio performer date" duplication stops, with no operator config.

**What becomes harder**
- One more manifest key and one more `metadata-sources.yaml` knob to reason about when a provider's
  match quality changes — mitigated by `searched[]`, which shows the operator what the provider did
  with what it was sent.
- `refreshOneProvider` builds its hint per goroutine instead of once — a small restructuring of the
  fan-out, and one more resolve of the video's fields on that path.
- A basename now leaves the box for opted-in providers unless an operator says otherwise — the
  security review must confirm the opt-in + deny layering is enough and that nothing beyond the
  basename can ride along.

**What we'll need to revisit**
- **`person` / `studio` structured hints.** The `resolve_hints` list and the `fields` shape permit
  them; nothing is built, because those entities still carry only `Name`/`Aliases` (ADR-080 D3).
- **A provider-side platform/hosting-site deny-list** for filenames that are obviously not releases —
  a provider concern, tracked as
  [HOLODEX-371](https://whoiskevinrich.atlassian.net/browse/HOLODEX-371) on the Holodex side only for
  the bracket-extracted-studio symptom.
- **Whether `query_source` needs a third value** (e.g. `"floor"`) once providers can show, via
  `searched[]`, whether they treat the sanitized-title floor differently from a real pattern.

---

## Action Items

1. [x] ADR-095 recorded; add to `docs/architecture/README.md`.
2. [x] Provider-contract spec (`docs/specs/metadata-provider-contract.md`): §2.2 `resolve_hints` row +
   example; §2.3 request table rows for `hint.fields` / `hint.filename` / `hint.query_source`, the
   porting note (`{performers}` vs. `actors`+`director`), a video request example, and `searched[]`
   on the response; §4.9 residue rule row; new §4.10 deep-dive in the §4.7–4.9 style (opt-in table,
   the miss definition, "user query first", route-separately, report-in-`searched[]`, why `filename`
   is the trustworthy input); §5 `searched[]` caps row. Same Draft PR as this ADR (ADR-069).
3. [x] `/write-spec` — [F54 spec](../specs/configurable-provider-search-patterns.md) amended: FR6
   (residue rule, incl. rendered-empty ≠ missing and the all-or-nothing example), FR7
   (`query_source` derivation), FR8 (structured hints, manifest gate, operator deny, both paths),
   FR9 ("Searched: …" caption from `searched[]`, replacing the never-built P1-a); AC-12–18; AC-10
   narrowed to non-opted-in providers; P2-a promoted; test notes per FR.
4. [x] `/design-handoff` — [the caption](../design/structured-resolve-hints-searched-caption-handoff.md)
   ([HOLODEX-369](https://whoiskevinrich.atlassian.net/browse/HOLODEX-369)): first query inline +
   `+N more` disclosure (chosen over collapsed-only from a two-option mockup), own `<p>` under the
   aria-live status line in every state, `<ol>` capped and scrolling for the 10-entry stressed
   state, tab-order inside the existing trap, batch-path detail-line format;
   [SVG mockup](../design/structured-resolve-hints-searched-caption-mockup.svg) +
   [QA checklist](../design/structured-resolve-hints-searched-caption-qa-checklist.md) committed.
5. [x] `/testing-strategy` — [`docs/testing-strategy.md`](../testing-strategy.md): §4 backend row
   (manifest-gate golden against the ADR-080 golden, operator deny incl. the default-allow assertion,
   `fields` ∩ advertised with a field the video has and the provider lacks, `Base()` verbatim,
   `query_source` derivation incl. client-supplied ignored, residue table incl. the residue-present
   *duplicated* render, per-provider batch hints inside the fan-out, `searched[]` ingest caps and
   the no-path detail); §5 frontend row for the caption (supersedes the F54 row's P1 clause; stressed
   state is a §12 geometry assertion); six Critical invariants; a §9 adversarial block; a §11 gap
   entry naming the three traps (golden edited to pass, the two rejected residue variants, the
   missing `enrich-picker-open` harness preparation).
6. [ ] **Implementation — request side** ([HOLODEX-368](https://whoiskevinrich.atlassian.net/browse/HOLODEX-368)):
   `Manifest.ResolveHints`; `Source` deny flag; `Hint{Fields, Filename, QuerySource}`; residue rule
   in `query.go`; `enrichVideoResolve` derives `query_source`; `enrichQueryHint`/`refreshOneProvider`
   build per-provider hints; `searched[]` decoded and recorded into `job_runs.detail`.
7. [ ] `/security-review` before merge — new outbound data: the raw basename to opted-in providers.
   Confirm: opt-in + operator deny layering; `filepath.Base` only, never a directory component;
   `fields` carries nothing the §4.9 blob did not already send; `searched[]` ingest is sanitized and
   capped like `candidates[].label`; the activity-detail no-path invariant holds.
8. [ ] **Implementation — caption** ([HOLODEX-369](https://whoiskevinrich.atlassian.net/browse/HOLODEX-369)),
   after the design gate.
