# Local development credentials

Where secrets live when you run Holodex locally, and why never in a file in the repo — not even
a gitignored one.

## The rule

**Local-dev secrets come from your user environment. No file in the working tree holds one.**

A gitignored file never reaches a commit, but it is still private only in name: working-tree
files get read into agent context, and from there into transcripts, screenshots and screen
shares. The environment holds each value in one place per machine, every worktree and clone
inherits it, and rotating it is one command instead of a sweep across worktrees.

## Setting it up

On Windows, once per machine:

```powershell
setx ADMIN_TOKEN    "<your local dev token>"
setx TMDB_API_TOKEN "<TMDB Settings > API > Read Access Token>"
```

**`setx` only affects processes started afterwards.** Restart your editor or agent once, or the
dev servers start without the values.

Use TMDB's **Read Access Token** (the long `eyJ…` value), not the 32-character v3 API key shown
beside it. They are sent differently — the token as `Authorization: Bearer`, the key as an
`api_key` query parameter — and are not interchangeable. If you only have a v3 key, set
`TMDB_API_KEY` instead.

## Where each file stands

| File | Tracked? | Holds |
|---|---|---|
| `.claude/launch.json` | gitignored (`.claude/launch.json*`, which also catches `.bak` copies) | Per-worktree dev-server profiles: paths, ports, non-secret env. **Never credentials.** |
| `.claude/launch.json.example` | committed (re-included against the glob) | The template a new worktree copies, with the rule written into it |
| `.env` | gitignored | Non-secret local backend config. Read by the backend only |
| `.env.example` | committed | Placeholders, commented out, no values |

The **backend** reads `.env`, and a real environment variable wins over it, so the environment
rule composes with `.env` cleanly.

The **TMDB sidecar** (`providers/tmdb`) has no `.env` loader and won't get one: it can't import
`internal/*`, and it reads plain `os.Getenv` — the same contract it has in its container, where
compose passes the token in as an environment variable. Its token can only come from the
environment.

## How it fails

Both failure modes are loud:

- **Missing:** the sidecar exits with `TMDB_API_TOKEN or TMDB_API_KEY must be set`.
- **Swapped:** at startup the sidecar classifies each value by shape. A v3 key in
  `TMDB_API_TOKEN` (or a token in `TMDB_API_KEY`) logs which variable to use and exits non-zero.
  A value that matches neither shape only warns, so a future TMDB format degrades to a hint.
  Only the classification is logged, never the value.

The shape check can't tell a well-formed token that is expired, revoked or wrong; that still
shows up as a 401 at request time.

## What CI enforces

The `secrets` job in `ci.yml` runs gitleaks over the branch's **full history** on every PR and
push to `main`, and the release re-runs it. A committed secret can't be fixed with a follow-up
commit, because the value is still in the objects — **rotate it.** See
[`ci-and-releases.md`](ci-and-releases.md#the-merge-gate).
