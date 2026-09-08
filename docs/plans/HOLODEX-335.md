---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-335                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix — a title or release date written back to a file no longer reports "out of sync" forever. Fields whose written tag cannot be read back now report no sync state at all instead of a false mismatch, and the shipped example mapping declares the file tags the writeback writes.
---

# HOLODEX-335 · Writeback read-back: pair the write target with a baseline source, make `in_sync` tri-state

Reported as "writeback succeeds but `title` / `release_date` stay permanently out of sync". The
report was accurate and unusually well-evidenced: the tags **were** written (`ffprobe` read both
back), `studio` and `overview` cleared in the same batch, and the symptom reproduced across seven
sampled entities and both container formats — so not a cache, not a failed write, not the queue.

**Root cause: two tables decide this and nothing connects them.** Writeback picks its destination
tag from `internal/writeback`'s `formatMap` (`release_date` → `Year`). The resolver computes
`in_sync` in `replaceMarkers` as `decided == fileVal`, where `fileVal` comes from `baselineValue` —
a walk of the field's **`sources:` list in `metadata-mappings.yaml`**, looking for a `file:`
namespace entry. The shipped example declares `release_date` with `filename:` and `tmdb:` sources
only, so `fileVal` is permanently `""` and every standing decision compares unequal. `studio`
cleared only because it happens to list a bare `Publisher` — the same tag writeback writes.

ADR-073 already fixed the adjacent failure (the stored baseline was never refreshed after a write)
and it holds; this is one step earlier — the baseline is fresh, but nothing declares where to read
it from.

**Two of the reporter's inferences were wrong and are corrected in the ADR:** `poster_url` is not
exempt via a per-field path (it simply carries no standing decision — `dec == nil` leaves `inSync`
true), and the extractor is not missing a mapping for `title`/`release_date`. Verified with exiftool
on both containers that `Title` and `Year` round-trip fine; `Title` is folded into `videos.title`,
which `file:title` addresses.

**The round-trip is not uniform, which shaped the fix.** Three write targets verifiably do not read
back on MP4 — `tagline` (`QuickTime:Keywords` → consumed into content tags), `original_title` (no
MP4 target at all) and `original_language` (`QuickTime:MediaLanguage` is not a defined exiftool tag,
so the write is silently dropped). Declaring a `file:` source for those would trade a permanent
false on every container for a permanent false on MP4 — so they stay provider-only and report
unknown instead.

**Overlaps:** the `in_sync` marker is ADR-051's; the post-write refresh it depends on is ADR-073's;
the write-target table is ADR-041's.

## Gates — definition of done

- [x] spec `write-spec` — **judged `[~]` first, and that was wrong.** A doc audit found
  `docs/specs/field-source-of-truth.md` documents the `in_sync` contract in terms this change
  breaks: "A field is *out of sync* when its decided value differs from the value currently embedded
  in the file's tag" is the exact two-way comparison ADR-093 splits, and the API section promised
  `in_sync` unconditionally. Updated there plus `fire-and-forget-writeback.md`,
  `metadata-plugins.md`, `metadata-provider-contract.md`, `claimed-provider-keys.md` and
  `qa-writeback.md`
- [x] architecture `architecture` —
  `docs/architecture/ADR-093-writeback-readback-and-tristate-in-sync.md`, indexed in
  `docs/architecture/README.md`
- [~] design `design-handoff` — not applicable; no new surface. The visible change is the removal
  of a false signal, and the frontend needed no code change (`outOfSync` is already `=== false`)
- [x] testing `testing-strategy` — four resolver cases pinning the true/false/unknown split
  (mutation-checked: reverting the fix fails `TestResolve_NoBaselineSource_InSyncUnknown` with the
  exact reported `got false`), one cross-table guard test, one frontend case
- [~] security `security-review` — not applicable; no auth, access, or infrastructure surface. No
  change to the SSRF/asset perimeter, the owner gate, or any write path
- [ ] three-skin QA — not yet run. The detail page's per-field pill and the header's "N out of
  sync" count are the affected surfaces

## Up next — ordered (position = priority)

1. [x] [investigate] Trace the read side (`replaceMarkers` → `baselineValue` → `videoBaseline`)
   against the write side (`formatMap`), and prove the round-trip empirically with exiftool on real
   MKV/MP4 fixtures rather than reasoning about tag names — `internal/resolver`, `internal/writeback`
2. [x] [architecture] ADR-093, with the measured round-trip table so D3's three exemptions are
   evidence-backed rather than asserted — `docs/architecture/`
3. [x] [backend] `hasBaselineSource` + tri-state `in_sync` in `replaceMarkers` —
   `internal/resolver/resolver.go`
4. [x] [config] File baseline sources in the shipped example (`release_date` → `Year`,
   `overview` → `Comment`), plus the round-trip warning in the header comment —
   `metadata-mappings.yaml.example`
5. [x] [testing] `TestExampleMappingCoversWriteTargets` locking `formatMap` to the example's
   `file:` sources, with `unreadableWriteTargets` carrying a written reason per exemption —
   `internal/writeback/example_mapping_test.go`
6. [x] [docs] "Writeback round-trip" reference section + the upgrade note for an existing
   gitignored `metadata-mappings.yaml` — `docs/reference/canonical-fields.md`
7. [x] [tracking] Fire In Progress; file the spun-off issues — **HOLODEX-336** (MP4
   `original_language`/`tagline` write targets), **HOLODEX-337** (`overview` mismatch against a
   *populated* candidate), **HOLODEX-338** (`file_writebacks.value` join fidelity) — all linked
   Relates to this one
8. [x] [backend] **ADR-093 D5 — the live-config warning.** `writeback.ReadbackGaps` is now the one
   implementation, warned at process start and `reload-config`, with the guard test holding the
   example to the same function — `internal/writeback/readback.go`, `cmd/holodex/main.go`,
   `internal/api/handlers.go`
9. [x] [docs] Provider-contract + configuration documentation swept for the tri-state contract, the
   write/read-back pairing and the new WARN — `docs/reference/configuration.md`,
   `docs/specs/metadata-provider-contract.md`, `metadata-plugins.md`, `claimed-provider-keys.md`,
   `field-source-of-truth.md`, `fire-and-forget-writeback.md`, `qa-writeback.md`
10. [ ] [—] Live three-skin QA on a real writeback: confirm the pill clears for `release_date` once
    the file source is declared, and that an undeclared field shows no sync state rather than a lit
    pill. Also confirm the new WARN fires on Kevin's real mapping and names the right keys (QA 0.2b)
11. [ ] [—] `/simplify` then `/code-review` on the diff, mark the PR ready

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-07 (later still) · Documentation sweep — and a gate I had wrongly closed
- skills: architecture
- **The audit overturned my own judgment.** I had marked the spec gate `[~] not applicable` on the
  grounds that this was "a defect in an existing contract, no new requirement". But
  `field-source-of-truth.md` *is* where that contract is written down, and it defined sync state as
  a two-way comparison and promised `in_sync` unconditionally in the API payload — both now wrong.
  The gate is `[x]`, not `[~]`. Worth remembering as a pattern: "no new behaviour" does not mean
  "no spec change" when the change is to the meaning of an existing field.
- **Provider contract is genuinely almost untouched, and I said so rather than padding.**
  `metadata-provider-contract.md` never mentions `in_sync` — it is a wire contract, and this is a
  Holodex-side mapping concern. One edit: its single mapping-authoring instruction (§4.7) now says a
  writeback-capable replace field must also list the written file tag, with an explicit note that
  this places **no** requirement on the provider's wire contract. `.claude/rules/provider-sidecar.md`
  needed nothing at all.
- **Two shipped examples carried the bug.** `metadata-plugins.md`'s F27 operator example mapped
  `overview` to `tmdb:overview` alone — the same shape as the defect. Fixed, alongside the F28
  "Format mapping" section, which described `TagForField` as the whole story.
- **Corrected a claim that is now actively misleading.** `claimed-provider-keys.md` said a bare
  source "has no effect on any provider's `comment` key" — true about *claiming*, but it reads as
  "inert", and an operator pruning inert-looking bare sources would silently delete their own sync
  reporting. Now states the second, load-bearing job.
- **Caught a dead anchor before it shipped.** The new `## Writeback round-trip` heading carried an
  inline ADR link, so GitHub would slug it `#writeback-round-trip-adr-093` and all three inbound
  references — including the path in the runtime WARN — would have 404'd. Heading made plain, ADR
  moved to the body; verified every relative link added in this sweep resolves on disk.
- **QA gained the step that would have caught this class:** 0.2b starts with a gapped mapping,
  asserts exactly one WARN naming the right key, then asserts silence after the fix.
- **Next session:** items 10–11 — live QA, then `/code-review` before marking ready.

### 2026-09-07 (later) · Live-config warning (D5) + Jira sync
- skills: architecture
- **Took the review's verdict and built the piece it said was missing.** D2/D4 only ever reached a
  *new* install; `metadata-mappings.yaml` is gitignored, so the one person who could close the gap
  — the operator running the affected deployment — was the one the fix could not reach.
  `writeback.LogReadbackGaps` now warns at process start and at `reload-config`, naming the field
  and the key that closes it.
- **One implementation, three consumers.** `ReadbackGaps` moved out of the test into
  `internal/writeback/readback.go`; the guard test now holds the shipped example to the same
  function rather than re-deriving the comparison, and `UnreadableWriteTargets` moved with it so
  the exemptions cannot diverge between the test and the runtime warning. That deletes the
  duplication the original test carried — the very disease this ADR is about.
- **Made silence load-bearing and tested it.** The warning is suppressed for D3's exemptions (no
  `file:` source could close those, so a line every start would only train Kevin to ignore the
  channel) and `TestLogReadbackGaps_SilentWhenClean` pins that. `..._MessageIsActionable` pins that
  the line names the field and the key — a warning that only said "something is wrong" would leave
  the reader where the bug report started.
- **Layering call, stated:** `internal/writeback` now imports `internal/mapping` (a leaf config
  package, no cycle). The alternative — validating inside `mapping.Load` — would have taught the
  config loader about file writing, which is backwards.
- **Jira caught up now the connector is back:** HOLODEX-335 → In Progress; HOLODEX-336/337/338
  filed with self-contained repros and linked Relates. 336 carries the explicit note that the guard
  test does *not* fire when its underlying reason is fixed, so the exemption entries must be deleted
  by hand as part of that work.
- **Known behavior change:** anyone upgrading with a provider-only `release_date` gets a new WARN
  line per affected field until they edit. Intended, and recorded in the ADR's consequences rather
  than left as a surprise.
- **Next session:** items 9–10 — live QA (including that the new warning fires against the real
  mapping), then `/code-review` before marking the PR ready.

### 2026-09-07 · Root cause found, fix + ADR + guard test landed
- skills: architecture, simplify
- **Confirmed the report's conclusion but not its reasoning.** The reporter proposed "the extractor
  has no mapping for `title`/`release_date`". It does — the gap is the *mapping config*, one layer
  up. Corrected two other inferences: `poster_url` has no per-field exemption (it just carries no
  standing decision), and the `file` candidate is emitted for every replace field regardless.
- **Measured instead of trusting tag tables.** Built 64x64 MKV/MP4 fixtures, wrote each canonical's
  write target the way `writeback` does, and read back with exiftool. That is what produced the
  round-trip table in the ADR — and it caught two things reasoning would have missed: exiftool
  refuses `QuickTime:MediaLanguage` outright ("Tag not defined", write dropped), and MP4's
  `QuickTime:Keywords` reads back as `Keywords`, which the extractor's `tagKeys` swallows into
  `Extracted.Tags`. Both would have become new permanent-false fields had I bulk-added `file:`
  sources for everything in `formatMap`.
- **The fix is one predicate, deliberately kept distinct from an existing one.** `baselineValue`'s
  `ok` means "a baseline source carried a value"; the new `hasBaselineSource` means "a baseline
  source is declared". Collapsing those two questions is precisely the bug, so they stay separate
  functions rather than one function with a second return.
- **Mutation-checked the regression test** rather than trusting that it covers the bug: reverting
  `resolver.go` alone leaves the new test failing with `want nil, got false` — the reported symptom
  verbatim — and reverting only the example mapping fails the cross-table guard, naming `overview`
  and `release_date`. Both halves are pinned independently.
- **Scoped the guard test to replace fields on purpose.** Merge fields carry no `in_sync` (ADR-051
  RD1), which also sidesteps `genres`/`actors` — their write targets (`Genre`/`Artist`) are consumed
  into `Extracted.Tags`/`People` and could never satisfy a `file:` source anyway. Asserting over
  them would have forced a false exemption.
- **Named the residual honestly rather than fixing it quietly.** The deeper fix — compute `in_sync`
  against the *write target* for the container, which `markWriteTargets` already resolves — would
  fix every existing deployment with no config edit and delete the two-table hazard. It needs a
  tag → read-location map and splits the file-candidate chip from the sync comparison, so it is
  recorded in ADR-093's Consequences as the thing to revisit if the config migration proves a
  support burden.
- **`/simplify` (4 agents) changed the shape of the fix in three places.** All four converged on
  the same duplication: `hasBaselineSource` re-walked `f.ParsedSources` calling `baseline.Baseline`
  — the third such walk in `replaceMarkers` — to answer a question the existing `baselineValue`
  walk already had in hand. Folded into `baselineValue` as a fourth return (`declared`), deleting
  the helper. The two questions stay distinct as distinct *return values*, which is the point;
  what was wrong was walking twice to ask them. Also merged a redundant test pair (the
  declared-empty and declared-matching cases are one field's two directions) and cut the inline
  rationale that restated the `InSync` doc four lines up — 73% of the added non-test source lines
  were comment for a three-line behavior change.
- **The altitude review landed the most valuable correction, and it was against my own test.** The
  guard asserted a `file:` source was *present*, not that it *matched the tag writeback writes* — so
  `overview: [Description, tmdb:overview]` would have passed while reproducing this exact bug. Now
  compares normalized read keys (group prefix stripped, case folded) and names the expected key in
  the failure. Verified by mutation: swapping `Comment` → `Description` fails with "Add one of
  [comment]". The reviewer also caught that `title` — one of the two fields in the ADR's title — is
  invisible to the test because the example ships that block commented out. Recorded as a stated
  blind spot in D4 rather than papered over by uncommenting a field the example deliberately leaves
  off.
- **Took the "this is a ratchet, not a fix" verdict on the chin and rewrote D4 rather than
  defending it.** The review's proposed middle-altitude move — run the same check against the
  *live* mapping at load/reload and warn per writable-but-unreadable field — is the piece that
  would actually reach the deployment that reported this, since `metadata-mappings.yaml` is
  gitignored and D2 only touches the example. Not built: it is a new startup surface rather than a
  narrowing of this defect, and the scope was explicitly chosen. It is now the first line of
  ADR-093's next-step consequences, and is the recommendation carried to Kevin.
- **Left untouched:** `internal/writeback/{tags,snapshot}.go` and
  `internal/resolver/auto_register_test.go` fail `gofmt -l` / `prettier --check` on a clean tree
  (`web/src/lib/{types,f36,f36.test}.ts` too, and not a line-ending artifact — `git ls-files --eol`
  reports `lf/lf`). Pre-existing and repo-wide; reporting it rather than bundling a formatting sweep
  into a one-field regression fix.
- **Next session:** items 7–9 — file the spun-off issues, live QA the pill against a real writeback,
  then `/simplify` + `/code-review` before marking the PR ready.
