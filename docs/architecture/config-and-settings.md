# Configuration and settings

This doc owns how Holodex is configured: the startup config layers (CLI flags, environment, a
local `.env`, `holodex.yaml`, built-in defaults), the parallel YAML files beside `holodex.yaml`,
the on-disk data layout under `DATA_PATH`, the caching posture, and the DB-backed `settings` store
for values the owner sets at runtime. What the provider and mapping YAMLs *mean* belongs to
[metadata-providers.md](metadata-providers.md) and [field-resolution.md](field-resolution.md). The
container volumes and image are in [deployment.md](deployment.md). The admin token and the
provider allowlist as a perimeter are in [security-perimeter.md](security-perimeter.md).

## One immutable `Config`, resolved once at startup by layered precedence

`config.Load` (`internal/config`) resolves every source into one `Config` struct that is passed
through the app and never mutated after boot:

```
CLI flags  >  real environment variables  >  .env  >  holodex.yaml  >  built-in defaults
```

- **Environment variables** are the main channel for deployment knobs (paths, ports, worker
  counts, feature switches), because Docker and compose inject them. Every `Config` field has an
  env twin.
- **`holodex.yaml`** is optional and is read only when the `-config` flag names it. It carries the
  same keys as the env vars plus structured values that don't fit an env string.
- **CLI flags** (`config.Overrides`) cover only the few knobs needed to run the binary by hand:
  `-host`, `-port`, `-media-path`, `-data-path` and `-log-level`. `ApplyOverrides` re-derives the
  computed paths afterwards.
- `Defaults()` is the bottom layer. A zero-config start works, and the only value it can't
  supply usefully is `MEDIA_PATH`.
- `holodex.yaml` is **boot-only**. Changing it takes a restart. Nothing writes it back.

**Rejected:** a hot-reloaded or app-written `holodex.yaml` — it would race the env layer, which
wins and can't be rewritten. The file may also be a read-only bind mount. And it would blur
operator provisioning with runtime preference.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## A local `.env` feeds the environment layer for development

`Load` first calls `loadDotenv(".env")` on the working directory. Each `KEY=VALUE` line is set
into the process environment **only if that key is unset**. So a `.env` sits below real env vars
and flags and doesn't change the precedence above.

- Format: one `KEY=VALUE` per line. `#` comments and blank lines are ignored, and an `export `
  prefix and one layer of quotes are stripped. A missing or unreadable file is a silent no-op.
- Keys are the binary's own names (`MEDIA_PATH`, `DATA_PATH`, `HOST`, …). The compose-only
  `HOLODEX_*` host-mount substitution variables can live in the same file and the binary ignores
  them.
- The parser is a short stdlib function, with no dotenv dependency. No `.env` ships in the image,
  and compose always sets real env vars, so this never affects production. `.env` is gitignored,
  and `.env.example` is the committed template.

Decided in [`96f469dc`](https://github.com/whoiskevinrich/holodex/commit/96f469dc).

## Parallel YAML files for structured, per-deployment config

Structured config that is owned by the deployment lives in its own YAML files beside
`holodex.yaml`, not inside it. Each file's path is a config key with an env twin, and the default
is relative to the working directory:

- `metadata-mappings.yaml`: `METADATA_MAPPINGS_PATH`, canonical field mapping.
- `metadata-sources.yaml`: `METADATA_SOURCES_PATH`, the provider registry and its SSRF/asset
  allowlist.
- `metadata-patterns.yaml`: `FILENAME_PATTERNS_PATH`, filename token-grammar patterns.

A missing file means "none configured", not an error. Unlike `holodex.yaml`, these three files
**reload at runtime**: owner-gated `POST /admin/reload-config` re-reads all three. Each real file
is gitignored, and only its `*.example` is committed. Secrets come from env or these gitignored
files and are never stored in the DB.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## A single `DATA_PATH` root holds all persistent state

Everything Holodex writes lives under one directory, `DATA_PATH` (`./data` by default, `/data`
in the image). That makes a deployment two mounts: `MEDIA_PATH` for the library and `DATA_PATH`
read-write. `Config.derive()` computes each subpath from the root unless it was set explicitly:

```
$DATA_PATH/
  holodex.db          # SQLite (+ -wal, -shm); DATABASE_PATH may override
  thumbnails/
  person-images/
  studio-images/
  film-images/
  provider-icons/
```

A new kind of generated data gets a derived subdirectory of `DATA_PATH`. It never hangs off the
database file's path, and it never gets its own volume.

Decided in [`3561addf`](https://github.com/whoiskevinrich/holodex/commit/3561addf).

## No application cache: SQLite serves reads behind a `Cache` interface with only a Noop backend

`internal/cache` defines a `Cache` interface (`Get` / `Set` / `Invalidate` / `InvalidatePrefix`),
and `cache.New` builds one at boot from `CACHE_BACKEND`. Only the **`Noop`** backend exists.
`memory` (the default) and `none` both resolve to it, and no other cache setting exists. The read paths go straight to SQLite, where WAL, covering indexes
and FTS5 meet the search latency target on personal-library hardware. The `Cache` interface is
injected into services so that a real backend can drop in without touching the service layer.

An in-process backend (ristretto) gets built only when profiling shows a real endpoint missing the
latency target. Its invalidation is then designed against the measured hot paths, and
flush-on-scan is the likely first cut. Redis follows only if there is ever more than one replica.

**Rejected:** shipping the ristretto backend up front — every scanner write would need correct
invalidation, and ristretto has no key enumeration, so prefix invalidation needs a side index or a
full flush. That machinery would buy a benefit nobody has measured.

Decided in [`36e8bd8f`](https://github.com/whoiskevinrich/holodex/commit/36e8bd8f).

## A generic `settings` key/value table for library-owned values

The `settings` table (migration `0050_settings`) has the columns `key TEXT PRIMARY KEY`,
`value TEXT NOT NULL` and `updated_at`. `repo.GetSetting` and `repo.PutSetting` give access to it.
Writes go through the single writer under `writeMu`, and a missing row reads as "no value" (the
caller falls back to its default). The table doesn't know about types. The domain of each value is
validated in Go by the code that writes it, never in SQL, so adding a key needs no migration.

The boundary for every value is **who owns it**:

- **`settings`** holds values owned by the *library*. Such a value should survive a restore of
  `/data` onto a new host, and it would be identical on a staging copy of the same archive.
  Internal bookkeeping that belongs with the DB also lives here (today
  `completeness.inputs_fingerprint`). A backup that leaves out the DB doesn't bring these values
  back.
- **YAML/env** holds values owned by the *deployment*: paths, ports, the owner token, provider
  perimeters and field mappings. These differ per environment and are rebuilt from compose plus
  the YAML files, never from a DB restore.
- **Secrets and security perimeters are never `settings`.** Nothing the UI can write may hold a
  token or widen an allowlist.
- A library-owned value that is *authored* (a structured block written in an editor) rather than
  *chosen* may stay in YAML until a UI exists to author it.

**Rejected:** writing UI-set values back into `holodex.yaml` — that would make the app rewrite its
own config file (losing comments and ordering, and failing on a read-only mount), where a KV row
under the single writer is the existing repo idiom.

Decided in [`4fec2bab`](https://github.com/whoiskevinrich/holodex/commit/4fec2bab).

## Promoting a config value to an owner-editable runtime setting

> **Decided, not yet built.** No value has been promoted, and the allowlist and owner settings API
> below don't exist yet. This is the pattern the first promotion must follow.

When a library-owned config value (for example the gallery cap, `card_layout` or the delete grace
period) gains an owner UI, it moves onto `settings` as a new **top** precedence layer, applied one
key at a time:

```
effective = settings row  ??  config seed (yaml/env)  ??  built-in default
```

- The config value becomes the provisioning default and the reset baseline, not a ceiling.
  "Reset" deletes the row. When a row exists it wins, even over an explicitly set env var.
- **Reads never touch the DB.** The existing in-memory field (for example the value behind
  `Repo.GalleryCapValue()`) stays the hot read path. A write validates the value, upserts the row
  and refreshes that field, all under `writeMu`. At boot, after the config seed is applied, stored
  overrides are re-applied to their fields. Every promoted key needs a round-trip test
  (write → restart → effective).
- Only allowlisted keys can be written. A typed, in-code allowlist declares each settable key's
  type, bounds and default. It is the security boundary, because a key the allowlist doesn't
  declare can never be written. The write API sits behind `requireOwner`.
- A per-key **pin** is reserved but not built: a YAML/env twin that, when set, beats the row and
  makes the control read-only. It is for infrastructure-as-code operators. Add it per key when it's
  needed. Don't flip env to win globally.

**Rejected:** a bespoke endpoint per setting — it still needs durable storage, so it saves almost
nothing, and every later setting would have to work out persistence and precedence again.
**Rejected:** env as a hard override over the row — with a single owner, the same person sets
both, and env-wins would make the UI control silently do nothing whenever the env var is set.

Decided in [`c7a60c87`](https://github.com/whoiskevinrich/holodex/commit/c7a60c87).
