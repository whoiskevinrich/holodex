---
key: HOLODEX-530
status: in-review            # chore posture has no gates, so the merge guard reads this field
profile: chore               # dead-config removal; no behavior change
depends-on: []
release_note: "The unused `cache_max_memory_mb` / `CACHE_MAX_MEMORY_MB` and `redis_url` / `REDIS_URL` settings are gone. They never had an effect, and an existing `holodex.yaml` that still sets them loads unchanged."
---

# HOLODEX-530 · Remove dead cache config

Found while folding the caching ADRs into `config-and-settings.md` (HOLODEX-526). Only the `Noop`
cache backend exists, so `CACHE_MAX_MEMORY_MB` and `REDIS_URL` were parsed and never read. Both
keys are removed from `internal/config`, `holodex.yaml.example` and `docs/reference/configuration.md`.
`cache.New` drops its unused size parameter, and the `internal/cache` doc comments no longer claim an
in-process backend ships. The `Cache` interface and `CACHE_BACKEND` stay as the seam for a future
backend. YAML decoding is non-strict, so existing config files that set the old keys still load.

## Gates — definition of done

(chore posture: no gates)

## Up next — ordered (position = priority)

1. [ ] [—] Merge when CI passes; HOLODEX-530 moves to Done on merge

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-09 · removed dead cache keys
- skills: code-review high --fix
- handoff: Removed CACHE_MAX_MEMORY_MB and REDIS_URL from the code, example and docs, and corrected the cache.go comments; the Cache interface is unchanged. Ready to merge on green CI.
