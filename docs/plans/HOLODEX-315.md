---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-315                 # the tracker key; must match the branch key regex
status: in-review                   # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: ~
---

# HOLODEX-315 · TMDB `/describe` under-declares `asset_kinds`

The contract calls `asset_kinds` *"the binary asset kinds you can supply"*, and the TMDB sidecar's
manifest has never listed all of them. Done means the declared list matches what the enrich builders
actually emit, **and** a test holds the two together so they cannot drift again — which is the half
of this issue that had no owner.

**Design package:** none — conformance fix plus its regression guard. No spec, ADR or design artifact
is implicated; the contract already defines the vocabulary this brings the sidecar into line with
([§2.2](../specs/metadata-provider-contract.md), [§4.3](../specs/metadata-provider-contract.md)).

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → not needed; `docs/specs/tmdb-provider.md` updated to match the manifest, no contract change
- [x] architecture `architecture` → not needed
- [x] design `design-handoff` → not needed
- [x] backend → `providers/tmdb/handler.go`
- [x] testing `testing-strategy` → `TestDescribeAssetKindsCoverEveryEmittedKind`; non-vacuity verified both ways (see session log)
- [~] security `security-review` → not touched (no auth/access/infra change) — deferred, not applicable

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [testing] The drift guard the issue asked for — `providers/tmdb/tmdb_test.go`
2. [x] [backend] Declare the kind the guard found undeclared — `providers/tmdb/handler.go`
3. [x] [spec] Bring the manifest's documented copy back in line — `docs/specs/tmdb-provider.md`
4. [ ] [—] Converge `photo`/`headshot` on the contract's canonical person kind → HOLODEX-322

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-05 · the guard landed and immediately caught a kind nobody had noticed

- skills: (none — implementation)
- **This issue was two-thirds done without being closed, and the missing third is what mattered.**
  The code half (`banner`/`gallery` added to `AssetKinds`) shipped inside
  [#293](https://github.com/whoiskevinrich/holodex/pull/293); the documented copy caught up in
  [#299](https://github.com/whoiskevinrich/holodex/pull/299). The regression guard the issue
  explicitly asked for — *"a test asserting that every distinct Kind produced by the enrich builders
  appears in the declared AssetKinds"* — was never written, so both of those were point-in-time fixes
  to a list that drifts by construction.
- **Writing it found a third undeclared kind on the first run: `headshot`.** `buildEnrichResponse`
  labels a person's primary portrait `headshot`, while `headshotFor` labels a `people[]` credit's
  portrait `photo` — the same role, two spellings, from two builders. The manifest declared only
  `photo`. Latent for exactly the reason the issue predicted: `assetRoleFor` dispatches on the kind
  in the response, not on the manifest, and it maps `photo`/`portrait`/`headshot`/`""` to one role.
- **Declared both rather than changing the wire.** That is this issue's own prescription ("add the
  kinds TMDB actually emits"), and the manifest's job is to describe what is sent, not what ought to
  be. The *better* end state — the reference implementation emitting the contract's canonical `photo`
  everywhere — is a wire change with its own test and doc fallout, so it is filed as HOLODEX-322 and
  pointed at from the code comment rather than smuggled in here.
- **The guard has two halves, and both were verified non-vacuous by breaking the code on purpose:**
  - Removing `"headshot"` from `AssetKinds` → *`buildEnrichResponse` emits asset kind "headshot" but
    /describe.asset_kinds = [...] does not declare it.* The failure names the builder, so the next
    person does not have to go find it.
  - Changing `headshotFor` to emit `"portrait"` → **both** assertions fire: the expected-set check
    catches that the builders changed at all, and the coverage check catches the undeclared kind.
- That second assertion exists because the first one alone is a trap: the test can only see kinds its
  fixtures trigger, so it would silently pass if a new kind were added down a branch the fixtures do
  not reach. Pinning the observed set makes the test state what it has actually seen, so a builder
  change has to be acknowledged in one place or the other. The fixtures deliberately populate every
  asset-producing field on every builder, and run the movie builder for **both** `video` and `film`
  rather than assuming the film arm is a superset.
- Full `providers/tmdb` package green; `gofmt` clean.
- handoff: HOLODEX-315 is complete — manifest, docs and guard all agree, and the guard bites in both
  directions. The one open thread is HOLODEX-322 (converge `photo`/`headshot`), which this change
  deliberately set up rather than performed: the guard's `want` set is exactly what that ticket has to
  edit, so it will fail loudly and correctly the moment someone starts it.
