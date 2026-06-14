# ADR-027: Local `.env` loading for development config

**Status**: Accepted
**Date**: 2026-06-13
**Deciders**: Project owner
**Extends**: ADR-014 (Configuration strategy & precedence)

---

## Context

ADR-014 fixed the config precedence as **CLI > environment > `holodex.yaml` > defaults**,
with environment variables as the primary deployment mechanism. In production that
works well — Docker/compose inject real env vars. In **local development** it is
awkward: the binary has no zero-config discovery, so `go run ./cmd/holodex` starts
with an empty `MEDIA_PATH` (the scanner logs *"MEDIA_PATH not set; skipping scan"*
and the library is empty) unless the developer exports vars or passes flags every
run. Two sharp edges compound this:

- The repo already carries a gitignored `.env`, but it was **docker-compose-only**
  (compose reads `.env` for `${HOLODEX_*}` host bind-mount substitution). Developers
  reasonably assume the binary reads it too — it did not.
- The preview launcher (`.claude/launch.json`) ignores its `env` field, so the
  common "set it in the launcher" path silently provides nothing.

## Decision

The config loader reads a local **`.env`** (in the working directory) at the very
start of `Load`, before any other layer. Each `KEY=VALUE` line is set into the
process environment **only if that key is not already set**, so it feeds the
existing environment layer without changing ADR-014's precedence:

```
CLI flags  >  real environment variables  >  .env  >  holodex.yaml  >  defaults
```

- Format: `KEY=VALUE` per line; `#` comments and blank lines ignored; an optional
  `export ` prefix and one layer of surrounding quotes are stripped.
- A missing/unreadable `.env` is a **silent no-op**.
- The keys are the binary's own variable names (`MEDIA_PATH`, `DATA_PATH`, `HOST`,
  …) — **not** the compose-only `HOLODEX_*` names, which remain compose
  substitution variables. A `.env` can hold both.

## Rationale

- **Zero new dependency** — a ~20-line stdlib parser, consistent with the project's
  lean go.mod (cf. ADR-026).
- **Reads from the working directory**, so it works regardless of how the binary is
  launched — sidestepping the launcher's `env` handling entirely.
- **Never affects production**: no `.env` ships in the container image, and real env
  vars (which compose always sets) take precedence over `.env` anyway.
- Pairs with the developer's worktree workflow: a global `WorktreeCreate` hook copies
  `.env` into new worktrees, so a checked-out tree is immediately runnable.

## Consequences

- `.env` is now a documented local-dev config source (README + `holodex.yaml.example`),
  while `holodex.yaml` remains the home for structured config (mappings, etc.).
- Because `.env` only fills **unset** keys, an explicitly-exported env var or a CLI
  flag still wins — no surprise overrides of ops config.
- The committed `metadata-mappings.yaml.example` / `holodex.yaml.example` pattern is
  unchanged; `.env` stays gitignored (only its purpose is now broader).
