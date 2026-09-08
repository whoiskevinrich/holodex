# ADR-094: Local development credentials come from the environment, never a repo file

**Status**: Proposed
**Date**: 2026-09-08
**Deciders**: Project owner
**Relates to**: ADR-027 (config precedence — CLI > env > yaml > `.env`), ADR-033/ADR-040
(provider sidecar — HTTP-only, no `internal/*` import), ADR-046 (owner session cookie),
ADR-070 (periodic image CVE scan — the other CI security gate). Issue: HOLODEX-341.

---

## Context

`.claude/launch.json` is the per-worktree dev-server config. It is the only place that
sets environment variables for the preview launcher, so over time it accreted three
plaintext credentials: `ADMIN_TOKEN` on both backend profiles and `TMDB_API_TOKEN` on the
provider sidecar profile.

The file is gitignored — `.gitignore` carries `.claude/launch.json*` with a glob wide
enough to cover `.bak` variants, and that rule did its job: **the token was never
committed**. Verified two independent ways: every object in the database (4,585 blobs,
1,352 commits, all local and remote-tracking refs) contains no fragment of the token, and
a full-history `gitleaks` scan reports no leaks. `graphify-out/`, which *is* tracked and
would otherwise be a plausible index-shaped leak path, is also clean.

But gitignored is not the same as private, and that gap is what prompted this ADR. A file
in the working tree is routinely read into agent context, and from there into transcripts,
screenshots, and screen shares. A credential sitting in `launch.json` leaks through the
*development workflow* even though it never touches a commit. That is a different threat
model from "don't commit secrets", and the existing gitignore rule does not address it.

Two structural facts made the file the path of least resistance:

1. **The sidecar has no `.env`.** The backend loads a local `.env` via `loadDotenv` in
   `internal/config` (ADR-027). `providers/tmdb` is a separate module that must not import
   `internal/*` (ADR-033), so it cannot reuse that loader — it reads plain `os.Getenv`.
   There was no sanctioned file-based home for its token, so it went inline.

2. **`launch.json` is per-worktree.** Each worktree needs its own copy, so every credential
   is duplicated once per worktree and drifts independently. A rotation has to be chased
   across all of them, and a stale copy (`launch.json.bak`, six weeks old) silently
   retained superseded values.

## Decision

**D1 — `launch.json` holds paths and profile config only; never credentials.** A committed
`.claude/launch.json.example` documents the shape *and* the rule, so a new worktree copies
a correct template rather than reinventing one. `.gitignore` re-includes that single file
(`!.claude/launch.json.example`) against the existing glob.

**D2 — Local-dev secrets come from the user environment.** On Windows, `setx ADMIN_TOKEN`
and `setx TMDB_API_TOKEN` once per machine. The value then lives in exactly one place, is
inherited by every worktree and every future clone, survives worktree churn, and never
enters a file that gets read into agent context. Rotation is one command, not a sweep.

**D3 — The sidecar deliberately gains no `.env` loader.** It stays plain `os.Getenv`. This
preserves the ADR-033 boundary (no shared config code across the sidecar seam, no
duplicated loader) and keeps the dev contract identical to the container contract, where
the token already arrives as an environment variable from compose. The backend keeps its
`.env` path for non-secret local config; real environment wins over `.env`, so D2 composes
cleanly with it.

**D4 — CI enforces the rule over full history.** A `secrets` job in `ci.yml` runs
`gitleaks` against the branch's whole history, not the diff, checked out at
`fetch-depth: 0`. A secret added in an earlier commit and "removed" in a later one still
fails, because it is still in the objects — which is the only check that matches how git
actually works. It runs on every PR and push to `main`, and `release.yml` reuses `ci.yml`
via `workflow_call`, so the image build is gated by it too.

## Consequences

- **`setx` only affects processes started afterwards.** The editor/agent must be restarted
  once after setting the variables, or the dev servers will start without them. The
  template file states this, because it is the one non-obvious step.
- **A fresh clone needs two commands before the sidecar runs.** The failure is loud and
  already handled: `providers/tmdb/main.go` exits with
  `TMDB_API_TOKEN or TMDB_API_KEY must be set`.
- **A committed secret becomes a CI failure that cannot be fixed forward.** Once the value
  is in a pushed object, the only real remedy is rotation; a follow-up commit does not
  remove it. Making the gate hard is the point.
- **Full-history scanning costs ~40 s per run** at the current repo size, and grows with
  history. If that becomes material, the job can scan `--log-opts` limited to the PR range
  *provided* a separate scheduled full-history scan exists — but not before, since the
  diff-only check is the weaker guarantee.
- **`docs/specs/qa-tmdb-provider.md` step 0.5 was wrong** and is corrected here: it told
  the reader to set `TMDB_API_TOKEN` in `.env`, which `.env.example` never contained and
  which the sidecar could not have read.

## Alternatives considered

**Teach the sidecar to load its own `.env`.** Rejected: it duplicates config-loading logic
across the ADR-033 boundary for one variable, and diverges the dev path from the container
path where the token already comes from the environment. It also keeps a secret in a file,
which is the thing being fixed.

**`${env:VAR}` interpolation inside `launch.json`.** Rejected: launcher support is
unverified, and it retains a secret-shaped slot in the file — an invitation to paste a
literal value in when interpolation misbehaves.

**Encrypted secrets in-repo (SOPS/age).** Rejected as disproportionate: this is a
single-developer repo whose only local secrets are a dev gate token and one third-party
read-only API token. Key management would exceed the problem.

**Do nothing — the gitignore rule already worked.** Rejected: it prevented the commit, but
not the exposure surface that actually prompted this (working-tree files entering
transcripts), and it left the sidecar with no sanctioned mechanism at all.
