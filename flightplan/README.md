# Flightplan

A portable Claude Code plugin that turns per-epic session state into a **git-native worklog**
maintained by hooks, not by memory. See **[ADR-064](../docs/architecture/ADR-064-flightplan-plugin.md)**
for the full design and rationale; this README is the operator's-eye view.

> **Load-bearing principle:** never let durable state depend on agent discipline. Derive it from an
> artifact (commit, branch, PR, file-on-disk) or fire it from a hook. The agent may *think*, never
> *remember*.

## What's here

| Path | Role | Status |
|---|---|---|
| `templates/worklog.md` | The per-epic worklog **schema** — copy per epic to `<worklog.dir>/<KEY>.md` | ✅ this batch |
| `examples/HOLODEX-6-field-source-of-truth.md` | A real rollout (F36) expressed in the schema — the migration example | ✅ this batch |
| `flightplan.example.yaml` | The portability seam — each repo copies to `.claude/flightplan.yaml` | ✅ this batch |
| `hooks/session-start.mjs` | `SessionStart` — branch-key → `In Progress` + scaffold + orientation banner | ✅ this batch |
| `hooks/post-tool-use.mjs` | `PostToolUse(Skill)` — log the skill run + flip its gate `[ ] → [/]` | ✅ this batch |
| `lib/worklog.mjs` | **The canonical worklog parser** — read + write (`logSkillRun`/`flipGate`) helpers, pure and fs-free. Shared with `scripts/whats-left.mjs`, the repo's other consumer of the schema | ✅ this batch |
| `lib/worklog.test.mjs` | Unit tests for the above; run by `make test-scripts` | ✅ this batch |
| `hooks/worklog.mjs` | Thin fs adapter over `lib/worklog.mjs` (reads the file, re-exports the parser) | ✅ this batch |
| `hooks/config.mjs`, `hooks/stdin.mjs`, `hooks/hookout.mjs` | Shared config + key resolution + hook stdin/stdout helpers | ✅ this batch |
| `hooks/stop.mjs` | `Stop` — nag when the worklog falls behind the code (mechanical only) | ✅ this batch |
| `skills/handoff`, `skills/triage` | Judgment surfaces (`[x]`-done, `release_note`, inbox drain) | ⏳ batch 2 |

All three hooks are wired in `.claude/settings.json` and read `.claude/flightplan.yaml` (via the
shared `config.mjs`). `SessionStart` fires `In Progress` through `scripts/jira-transition.mjs`
(ADR-058's shared lib) — with no local `JIRA_BASE_URL`/`JIRA_USER_EMAIL`/`JIRA_API_TOKEN` it
soft-fails and the banner says so, while the offline scaffold + orientation always run.
`PostToolUse(Skill)` maintains today's session-log entry and moves a gate to `[/]` when its producing
skill runs — it never sets `[x]` (that judgment is `/handoff`'s, batch 2). `Stop` is the freshness
watchdog: at the end of a turn, if a changed file is newer than the worklog, it nags (via a
non-blocking `systemMessage`) to leave a handoff note before the thread is lost. It **cannot** write
the note itself — a `Stop` hook must never invoke Claude (fork-bomb rule) — so it only makes the
omission loud; the next `SessionStart` banner independently flags "last session left no handoff." The
signal is self-clearing: the moment the worklog is touched it moves ahead of the code and the nag
goes quiet, with no stored state.

Packaged as a self-contained `flightplan/` dir so extraction to another Jira-tracked repo is a
copy-out + a `.claude/flightplan.yaml` swap, not a rewrite.

## The worklog schema (four sections)

One epic → one worklog → one definition of done. Every section is designed so a **mechanical hook**
can read or append to it without judgment:

1. **Frontmatter** — `key`, coarse `status`, `depends-on`, `release_note`. The `SessionStart` banner
   and the coarse Jira-status sync read this; `/handoff` authors `release_note`.
2. **Gates — definition of done** — a checklist keyed to `flightplan.yaml`'s `gates`. States:
   `[ ]` not started · `[/]` in progress · `[~]` deferred (`until: <event>`) · `[x]` done.
   `PostToolUse(Skill)` flips a gate to `[/]` when its producing skill runs; **only `/handoff` sets
   `[x]`** (the one "done" judgment a hook must never make).
3. **Up next — ordered** — a numbered queue where **position *is* the priority** (no P1/P2 tags to
   drift). Each item carries a `[gate]` tag and an optional file path; `→ KEY` promotes a separable
   item to its own issue; `⛔` marks a blocked item. The top item is surfaced verbatim at session start.
4. **Session log — append-only** — one dated entry per session: which skills ran + a one-line handoff
   sentence (the last of which the next `SessionStart` banner echoes). Capped to the last N; older
   entries are archived, not deleted.

## Orientation (what `SessionStart` will push — batch 1)

A compact (~150-token) banner distilled from the worklog, **not** the whole file:

```
▸ HOLODEX-6 · F36 per-field source-of-truth · in-progress · gates 4/6
  next: [frontend] SourceSelect radiogroup — web/src/lib/fields/SourceSelect.svelte
  last: S1 backend merged (#72); frontend builds against frozen types.
  ⛔ none
```

The full worklog stays on disk behind that pointer; it's read on demand only. Heavier
re-orientation (reconstructing state from diffs) is delegated to a disposable subagent that returns
only the distillate — the main thread never eats the raw reads.
