# ADR-112: Re-score completeness at boot only when its inputs changed, and never on the request path

**Status:** Proposed
**Date:** 2026-09-27
**Deciders:** Project owner

**Supersedes:** [ADR-099](ADR-099-completeness-score-required-band.md) **D4's boot hook only**: the
"denominator changes dirty everything … at boot" clause. The trigger-fed dirty set, the lazy owner-read
drain, and the mapping-reload and promotion/claim hooks all stand. **Relates to:**
[ADR-102](ADR-102-instance-skin-and-settings-store.md) D2 (the `settings` table). HOLODEX-469.

---

## Context

ADR-099 D4 marks every video, person and studio dirty on **every** boot. It reasons that registry
criticality is compiled in, so a build that re-tags a facet is only ever seen at boot. The first
owner read of any store-backed surface (`/people`, `/media`, `/studios`, `/completeness/facets`,
playlists, the person hover card) then drains that whole set synchronously under `writeMu`. It
re-scores all three entity types, not just the page's.

On production that meant a restart with no change at all still cost the owner roughly **8 s** of
blank page on the first list. The library is a few thousand videos and about a thousand people. The
timestamps showed the scores written in one continuous run, about half of it on videos. D4's own
promise, "Cost is O(entities mutated since the last owner read)", only held between restarts.

The boot hook guards against one real risk: a new build scoring differently from the build that
filled the store. It does that by paying the full cost on every restart, although almost no
restart is a new build.

## Decision

### D1: Fingerprint the scoring inputs; re-dirty everything at boot only when the fingerprint moves

At boot, compute a SHA-256 over:

- **the running executable's bytes** (`os.Executable()`). This stands in for everything compiled in:
  registry criticality, the resolver, the scoring formula, `appendAutoRegistered`. It needs no
  version constant that someone has to remember to bump, and the image carries no `-ldflags`
  version stamp (`main.version` is `"dev"` in production), so a version string could not do it.
- **`metadata-mappings.yaml` and `metadata-sources.yaml`**, the per-deployment config the resolver
  reads at boot. Each is length-prefixed. An empty path or a missing file hashes as "absent", so
  creating or deleting the file still moves the fingerprint.

Compare the result with `settings['completeness.inputs_fingerprint']`:

- **equal**: leave the store alone.
- **different or absent**: `MarkAllCompletenessDirty`, *then* record the new fingerprint. A failed
  mark records nothing, so the next boot retries.
- **fingerprint fails** (no executable path, unreadable file): mark dirty and record nothing. This is
  the old always-dirty behaviour, never a silently stale store.

The fingerprint lives in the database next to the scores it describes, not in a file beside the
binary. A `/data` restored from another build carries a fingerprint that does not match, and is
re-scored. `settings` (ADR-102 D2) is a plain key/value table that travels with `/data`, which is
the property wanted here. The key is internal: no handler reads or writes it.

### D2: The boot drain runs in the background, and requests skip their drain while it runs

After the handlers are wired and **before the server listens**, `main` calls
`Handlers.DrainCompletenessInBackground`. It always runs, whether or not D1 re-dirtied anything,
because writes since the last owner read (a scan while nobody browsed, a mapping reload with no
read after it) also leave dirty rows that the first page would otherwise pay for.

While it runs, a flag is up, and `drainCompleteness` on the request path **returns immediately**.
The page serves the store as it stands: the previous build's rings, or none on a fresh store. The
alternative, queueing on `writeMu`, would hand the whole cost straight back to the page. D4 already
made the badge best-effort ("the badge is not the page"), so a briefly stale ring is inside the
contract. The ids stay dirty, and the first read after the flag drops drains whatever is left.

The flag is raised synchronously, before the goroutine starts and before `ListenAndServe`, so no
request can see it down while the boot drain has not yet taken `writeMu`.

## Consequences

- An ordinary restart does **no** completeness work. The first owner page costs what the second does.
- A deploy (new image) or a config edit re-scores everything once, in the background. The scan
  starting at boot waits on `writeMu` behind it, as it did behind the request-path drain before.
- A page opened during that background re-score shows the previous scores. Nothing is lost: the
  badge updates on the next read after the drain.
- The executable hash costs one sequential read of the binary at boot, about 100 ms for a
  tens-of-MB image. `go run` in development builds a fresh binary per code change, so the dev loop
  still re-scores after a code change and skips it on a plain restart.
- **Not covered, and not newly uncovered:** a provider sidecar that changes its field hints without
  a config change. The old boot hook only caught that by accident on a restart, and it was never
  caught at runtime. It stays an open gap.
- A mapping reload (`POST /admin/reload-config`) still marks everything dirty and still drains on
  the next owner read, synchronously. That is an owner-initiated action, rare, and out of scope here.

## Alternatives considered

- **Hash registry criticality only.** Rejected. A resolver or formula change moves scores without
  touching criticality, and the executable hash catches both.
- **A hand-bumped `completenessScoreVersion` constant.** Rejected. It relies on someone remembering
  to bump it, and forgetting fails silently with stale rings.
- **Keep the always-dirty boot and only move the drain to the background (D2 alone).** Rejected as
  the whole fix. Every restart would still spend the re-score, holding `writeMu` for seconds
  against the boot scan.
- **Skip the drain whenever `writeMu` is busy (`TryLock`).** Rejected. `writeMu` is held by every
  write, so ordinary scanner or queue writes would make owner reads serve stale rings in normal
  operation. D2 skips only while the boot drain itself runs.
- **Drain only the requested entity type on the request path.** Deferred. It helps the reload
  case, but the boot case is gone under D1 + D2.
- **Batch the per-video genre-writeback lookups in `completenessForVideos`.** A separate follow-up.
  It would make every drain cheaper, but it doesn't change when drains happen.
