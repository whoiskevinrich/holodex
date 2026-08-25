# Holodex — Project Working Agreements

These rules govern how changes are made in this repo. They keep **specs, architecture,
design, tests, and security in lockstep**. Follow them for every change.

## Context discipline (token budget)

The 5-hour usage window is spent on context that is **re-sent every turn**, so one wide read
early taxes every later step. Keep per-turn context flat:

- **Delegate exploration to subagents.** "Find / trace / where-is / list all callers" sweeps go
  to a Task or Explore subagent so file dumps stay out of the main session — only the conclusion
  returns. Don't grep or fan-read the tree inline when a subagent can.
- **Read narrowly.** Prefer a targeted `rg` with a head limit and `Read` with offset/limit over
  whole files "to be safe." **Never read `TASKS.md`** (80K, archived and frozen) — use Jira.
- **Point at indexes, pull one doc.** Start from `docs/architecture/README.md` and open a single
  ADR deliberately; don't read the ADR/spec tree. Same for `docs/specs/`.
- **Clear between unrelated asks.** Prefer `/clear` or a fresh session when switching tasks so one
  ask isn't billed for the previous one's accumulated context.

## Commands

Backend (Go) via `Makefile`; frontend (`web/`) via npm:

| Task | Command |
|---|---|
| Run the server | `make run` (`go run ./cmd/holodex`) |
| Production build | `make build` |
| Go tests | `make test` · integration: `make test-integration` |
| Flightplan/scripts unit tests only | `make test-scripts` |
| Frontend dev server | `make web-dev` (or `cd web && npm run dev`) |
| Frontend type-check / tests | `cd web && npm run check` · `npm run test` |
| Full stack in Docker | `make docker` (`docker compose up --build`) |

## Codebase map

Go backend + SvelteKit SPA. Where things live, and the one model that ties them together:

| Path | Responsibility |
|---|---|
| `cmd/holodex` | Entrypoint + bootstrap — runs migrations, one-time backfills, wires services |
| `internal/api` | HTTP handlers (chi); owner-gated mutations via `requireOwner` |
| `internal/repo` | SQLite data access; a single writer serialized under `writeMu` (WAL reads lock-free) |
| `internal/resolver` | **Pure** unified field resolution over `BaselineSource` + enrichment + curation + decisions; entity-generic (video/person/studio) |
| `internal/enrich` | Provider HTTP client + `entity_enrichment` shadow store; the SSRF/asset perimeter. Providers are declared (not compiled in) in `metadata-sources.yaml` — `base_url` **is** the SSRF allowlist, `asset_hosts` the image-download allowlist |
| `internal/mapping`, `internal/registry` | Canonical field mapping (`metadata-mappings.yaml`, ADR-013) + per-field metadata (labels/display) |
| `internal/db/migrations` | golang-migrate, numbered `NNNN_name.{up,down}.sql` (see `.claude/rules/migrations.md`) |
| `providers/tmdb` | Standalone metadata-provider **sidecar** (see `.claude/rules/provider-sidecar.md`) |
| `web/` | SvelteKit SPA (see `.claude/rules/frontend-theming.md`) |

**The core model** (ADR-033/051/052): the file layer is the **baseline/default truth**; provider
enrichment is an **additive shadow** (never flattened into the file layer); the **pure resolver** is
the sole merge point; standing **per-field source decisions** + **curation** override precedence at
resolve time. Person and Studio are **entities** riding that same decision model over a
`BaselineSource`. Read the relevant ADR before changing any of these seams.

## Change-routing rules

While making a change, route it through the right skill based on what it touches:

| If the change touches… | Run… | Artifact produced/updated |
|---|---|---|
| **Functionality / behavior** (new feature, changed requirement, scope) | `/write-spec` (new) or edit the relevant `docs/specs/phase-*.md` | a spec |
| **Infrastructure / technical architecture** (stack, data model, deployment, cross-cutting decisions) | `/architecture` | a new or updated ADR in `docs/architecture/` |
| **UX / user-facing surface** (screens, flows, components, interactions) | `/design-handoff` | a design handoff spec |
| **Anything significant** (any of the above, or a multi-file behavior change) | `/testing-strategy` | updated `docs/testing-strategy.md` + tests aligned to the spec/architecture/design |
| **Authentication, access, or infrastructure** | `/security-review` | a security sign-off before merge |

**`/design-handoff` must persist any rendered mockup as a committed asset** (SVG preferred —
self-contained, renders inline on GitHub, no CDN/font dependency) in `docs/design/` next to the
handoff doc, referenced from it with an image embed. A mockup that only exists as a chat/session
artifact is lost the moment context is summarized or the session ends, which has caused approved
designs to not make it into implementation — the committed doc is the only copy that survives.

A functional change with no spec update — or an infra change with no ADR — is **incomplete**.
On the Jira side, the same gates surface as the `needs-spec` / `needs-adr` / `needs-design` /
`needs-security-review` labels (see "Task tracking") — apply one when the change enters the
matching row, clear it when the artifact lands.

**Pre-implementation gates ship as a Draft PR.** The spec / ADR / design rows above produce
artifacts that want review *before* the code exists. As soon as the first of them lands, push
and open a **Draft** PR (`gh pr create --draft`) — the epic's one PR, which then accumulates the
remaining gates. A Draft PR fires **no** Jira transition, so the ticket correctly stays
`In Progress`; **`In Review` fires when you mark it ready for review**, which you do only once
the gates are green. Don't split a gate artifact into its own PR merged ahead of the
implementation. See [ADR-069](../docs/architecture/ADR-069-draft-prs-for-pre-implementation-gates.md).

## Pre-commit checklist (every commit)

1. Run **`/simplify`** on the changed code (reuse, simplification, efficiency) — always, before committing.
2. If the change touched **auth, access, or infrastructure** → run **`/security-review`**.
3. Confirm the matching **spec / ADR / design / testing** artifacts (table above) were created or updated.
4. Confirm **no secrets, credentials, or PII** in the diff (see "Secrets & publishing").
5. If the change touched the **frontend** → honor the theming rules (auto-loaded from
   `.claude/rules/frontend-theming.md` when you open a `web/**/*.svelte` file): no hardcoded
   styling, and **QA all three skins**.

## Frontend theming

Tokens-only components + **QA all three skins**. The full, load-bearing rules live in
`.claude/rules/frontend-theming.md` and load automatically when you open a `web/**/*.svelte`
file (also ADR-021 and `docs/design/theming.md`).

## Before pushing or opening a PR

1. Sync task tracking in **Jira (project `HOLODEX`)** — triage/refresh the affected issues (status, links) — before every `git push` and `gh pr create`.
2. **Update the flightplan worklog in the same push, not after.** Before pushing, open
   `docs/plans/HOLODEX-<key>.md` and bring it current with what this push actually did: flip
   any gate this push closed to `[x]`, append a session-log entry (skills run + a one-line
   handoff sentence), and update `Up next`/`release_note` if they changed. Stage the worklog
   file alongside the code/PR changes so it ships in the same commit — don't leave it for a
   follow-up commit or wait for the user to notice it's stale. (`/handoff`, the skill meant to
   automate this judgment call, is not yet built — see `flightplan/README.md` — so this is a
   manual step until it lands.)
3. Re-confirm the pre-commit checklist above is satisfied for everything in the push.
4. Scan the working tree for secrets / PII (see "Secrets & publishing").
5. **Draft unless the gates are green.** Open with `gh pr create --draft` whenever work
   remains (see "Pre-implementation gates ship as a Draft PR" above); drop `--draft` — or mark
   an existing Draft ready — only when every gate in the routing table is satisfied. Marking
   ready is the act that moves the ticket to `In Review`.

## Task tracking (Jira)

Tasks live in **Jira project HOLODEX** (`whoiskevinrich.atlassian.net`, team-managed) — scope
every interaction to that project. Migrated from the former `TASKS.md` on 2026-06-30; that
file is archived (frozen, read-only) — **do not read or write it**; use Jira.

- **Hierarchy:** Epic (an F## feature / phase) → Story (`feat`) · Bug (`fix`) · Task
  (`refactor`/`perf`/`docs`/`test`/`ci`/`build`/`chore`) → Sub-task. Team-managed: link a
  child to its parent via the **`parent`** field (there is no "Epic Link").
- **Priority / labels:** map `[P·effort]` → Priority (P1→High, P2→Medium, P3→Low) plus area
  labels.
- **Statuses:** To Do → In Progress → In Review → Done → Released. *In Progress* covers the
  whole build **including while a Draft PR is open**; *In Review* = the PR was marked ready for
  review; *Done* = merged to main; *Released* = shipped in a tagged GHCR image (set by the
  `ghcr` deployment, see below).
- **Artifact labels** mirror the change-routing table: tag an issue `needs-spec`,
  `needs-adr`, `needs-design`, or `needs-security-review` so the lockstep gate is visible on
  the board; clear the label when the artifact lands.
- When you note a **TODO** or **defer** an item (a stub, a "later", a "Phase 2", a known
  gap), capture it as a HOLODEX issue so it isn't lost in a code comment.

### Branch ↔ Jira linkage (load-bearing)

The GitHub-for-Jira app links branches, PRs, builds, and the `ghcr` deployment to an issue
**only when the issue key is present**. So:

- **Name every branch/worktree with its key:** `HOLODEX-123-short-slug`. This drives the
  Jira development panel and the CI transitions; without it nothing links.
- **Auto-rename on start (agent default — don't wait to be asked).** When work begins on a
  HOLODEX issue inside a worktree whose branch does **not** already carry the key (e.g. an
  auto-generated `claude/<slug>`), rename it to the key **as the first action**, before any
  commit: `git branch -m HOLODEX-123-short-slug`. The worktree directory name can stay as-is;
  only the branch name must carry the key. The substring is enough — GitHub-for-Jira detects
  the key anywhere in the branch name, so a `worktree-`/other prefix still links.
- **Fire `In Progress` at that same start-of-work step (agent default).** Immediately after
  the rename, transition the issue to **In Progress** via the Jira MCP `transitionJiraIssue`.
  Per ADR-058, `In Progress` is the one transition with no server-side event, so the agent
  owns it (CI owns In Review/Done/Released) — it's a REST call, not a metered Automation run.
- **Keep commit subjects and PR titles clean Conventional Commits** — `release-please` and
  `git-cliff` parse them into the changelog. Do **not** put the key in the subject/PR title
  (it would pollute every CHANGELOG/Release line); the branch name carries it.
- **Internal agent-tooling work (e.g. `flightplan/`, `.claude/`) uses `chore(flightplan): ...`,
  not `feat`/`fix`.** Both `cliff.toml` and `release-please-config.json` already hide `chore`
  commits, so this keeps agent-tooling changes out of user-facing CHANGELOG/Release notes
  without any config change — neither tool supports filtering by scope, only by type.
- Transitions run via **direct Jira REST API calls** (ADR-058), not Jira Automation (which
  meters the shared Free-plan quota): **In Progress** is agent-fired at branch-rename (above);
  **In Review** (PR marked *ready for review* — a Draft PR fires nothing, ADR-069), **Done**
  (merge), and **Released** (`ghcr` deploy) are fired by CI
  (`.github/workflows/jira-sync.yml` + `release.yml`, scripts in `scripts/`). Not Smart
  Commits — commits stay clean. Full reference: `docs/reference/jira-pipeline.md`.

## Secrets & publishing

- Never commit secrets, tokens, private keys, or PII. Configuration comes from environment
  variables or three parallel gitignored YAML files — `holodex.yaml` (main config),
  `metadata-mappings.yaml` (field mapping, ADR-013), `metadata-sources.yaml` (provider registry /
  SSRF allowlist, ADR-033); only their `*.example` placeholders are committed.
- Generated/runtime data (`/data`, `*.db`, thumbnails, `web/node_modules`, build output, media fixtures)
  is gitignored — never commit it.
- Before pushing to a public remote, scan the working tree for sensitive values.

## Gotchas

- **Migrations** are append-only with a manual down — details in `.claude/rules/migrations.md`
  (loads when you touch `internal/db/migrations/`).
- **Provider sidecars** (`providers/tmdb`) talk to core over HTTP only and must not import
  `internal/*`; `_`-prefixed enrichment keys are internal contracts — details in
  `.claude/rules/provider-sidecar.md` (loads when you touch `providers/`).

## Conventions

- ADRs are immutable decisions — **supersede** rather than rewrite. Index: `docs/architecture/README.md`.
- Specs live in `docs/specs/`; the testing strategy in `docs/testing-strategy.md`.
- Keep the ADR index and spec cross-references up to date when adding either.