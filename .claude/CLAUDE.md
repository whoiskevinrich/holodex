# Holodex — Project Working Agreements

These rules govern how changes are made in this repo. They keep **specs, architecture,
design, tests, and security in lockstep**. Follow them for every change.

## Commands

Backend (Go) via `Makefile`; frontend (`web/`) via npm:

| Task | Command |
|---|---|
| Run the server | `make run` (`go run ./cmd/holodex`) |
| Production build | `make build` |
| Go tests | `make test` · integration: `make test-integration` |
| Frontend dev server | `make web-dev` (or `cd web && npm run dev`) |
| Frontend type-check / tests | `cd web && npm run check` · `npm run test` |
| Full stack in Docker | `make docker` (`docker compose up --build`) |

## Change-routing rules

While making a change, route it through the right skill based on what it touches:

| If the change touches… | Run… | Artifact produced/updated |
|---|---|---|
| **Functionality / behavior** (new feature, changed requirement, scope) | `/write-spec` (new) or edit the relevant `docs/specs/phase-*.md` | a spec |
| **Infrastructure / technical architecture** (stack, data model, deployment, cross-cutting decisions) | `/architecture` | a new or updated ADR in `docs/architecture/` |
| **UX / user-facing surface** (screens, flows, components, interactions) | `/design-handoff` | a design handoff spec |
| **Anything significant** (any of the above, or a multi-file behavior change) | `/testing-strategy` | updated `docs/testing-strategy.md` + tests aligned to the spec/architecture/design |
| **Authentication, access, or infrastructure** | `/security-review` | a security sign-off before merge |

A functional change with no spec update — or an infra change with no ADR — is **incomplete**.
On the Jira side, the same gates surface as the `needs-spec` / `needs-adr` / `needs-design` /
`needs-security-review` labels (see "Task tracking") — apply one when the change enters the
matching row, clear it when the artifact lands.

## Pre-commit checklist (every commit)

1. Run **`/simplify`** on the changed code (reuse, simplification, efficiency) — always, before committing.
2. If the change touched **auth, access, or infrastructure** → run **`/security-review`**.
3. Confirm the matching **spec / ADR / design / testing** artifacts (table above) were created or updated.
4. Confirm **no secrets, credentials, or PII** in the diff (see "Secrets & publishing").
5. If the change touched the **frontend** → honor the "Frontend theming" rules below: no hardcoded styling, and **QA all three skins**.

## Frontend theming (component discipline)

The UI is built on semantic design tokens with three switchable skins (see
[ADR-021](../docs/architecture/ADR-021-frontend-theming-and-skins.md) and
[`docs/design/theming.md`](../docs/design/theming.md)). Two rules are load-bearing:

- **Tokens only — never hardcode styling.** Components must use the semantic Tailwind
  utilities backed by CSS variables (`bg-bg`, `bg-surface`, `text-ink`, `text-muted`,
  `border-rule`, `bg-accent`/`text-accent`, `text-accent-ink`, `font-display`/`font-ui`,
  `rounded-theme`, `text-warn`/`border-warn`). **Never** a literal palette or value in a
  component: no `zinc-*`, `sky-*`, hex colors, named font families, or fixed `rounded-lg`/`px`
  radii. A hardcoded value is a theming bug — it won't react to the skin. Use `--warn`
  (`text-warn`/`border-warn`) for error/attention states — deliberately distinct from
  `--accent`, which doubles as the active/primary color. Skin-specific flourishes belong in
  `app.css` gated by `[data-theme]`, attached to the shared hook classes
  (`.app-atmosphere`, `.video-frame`, `.video-grid`, `.skin-title`) — not as per-component
  markup. Layout-mode rules attach to `.video-grid[data-layout='...']` (operator-set
  via `holodex.yaml: card_layout`; not a skin — do not gate with `[data-theme]`).
  Quick check over components: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` should be empty (raw hex values live only in `app.css` token blocks; `rounded-full` pills are an intentional shape).
- **QA all three skins.** When verifying any UI change, render and eyeball **Cinémathèque,
  Broadcast, and Brutalist** (switch via the header picker), not just the default —
  regressions routinely appear in only one skin (e.g. a badge/counter collision, an accent
  that doesn't read on its background). Confirm fonts load offline and the
  loading/empty/error/grid states are all themed.

## Before pushing or opening a PR

1. Sync task tracking in **Jira (project `HOLODEX`)** — triage/refresh the affected issues (status, links) — before every `git push` and `gh pr create`.
2. Re-confirm the pre-commit checklist above is satisfied for everything in the push.
3. Scan the working tree for secrets / PII (see "Secrets & publishing").

## Task tracking (Jira)

Tasks live in **Jira project HOLODEX** (`whoiskevinrich.atlassian.net`, team-managed) — scope
every interaction to that project. Migrated from the former `TASKS.md` on 2026-06-30; that
file is archived (frozen, read-only — see the banner at its top), so do not write tasks there.

- **Hierarchy:** Epic (an F## feature / phase) → Story (`feat`) · Bug (`fix`) · Task
  (`refactor`/`perf`/`docs`/`test`/`ci`/`build`/`chore`) → Sub-task. Team-managed: link a
  child to its parent via the **`parent`** field (there is no "Epic Link").
- **Priority / labels:** map `[P·effort]` → Priority (P1→High, P2→Medium, P3→Low) plus area
  labels.
- **Statuses:** To Do → In Progress → In Review → Done → Released. *Done* = merged to main;
  *Released* = shipped in a tagged GHCR image (set by the `ghcr` deployment, see below).
- **Artifact labels** mirror the change-routing table: tag an issue `needs-spec`,
  `needs-adr`, `needs-design`, or `needs-security-review` so the lockstep gate is visible on
  the board; clear the label when the artifact lands.
- When you note a **TODO** or **defer** an item (a stub, a "later", a "Phase 2", a known
  gap), capture it as a HOLODEX issue so it isn't lost in a code comment.

### Branch ↔ Jira linkage (load-bearing)

The GitHub-for-Jira app links branches, PRs, builds, and the `ghcr` deployment to an issue
**only when the issue key is present**. So:

- **Name every branch/worktree with its key:** `HOLODEX-123-short-slug`. This drives the
  Jira development panel and all automation; without it nothing links.
- **Keep commit subjects and PR titles clean Conventional Commits** — `release-please` and
  `git-cliff` parse them into the changelog. Do **not** put the key in the subject/PR title
  (it would pollute every CHANGELOG/Release line); the branch name carries it.
- Transitions run off Jira automation on dev events (branch → In Progress, PR open → In
  Review, merge → Done, `ghcr` deploy → Released), not Smart Commits — so commits stay clean.

## Secrets & publishing

- Never commit secrets, tokens, private keys, or PII. Configuration comes from environment
  variables or `holodex.yaml` (gitignored); only `holodex.yaml.example` (placeholder values) is committed.
- Generated/runtime data (`/data`, `*.db`, thumbnails, `web/node_modules`, build output, media fixtures)
  is gitignored — never commit it.
- Before pushing to a public remote, scan the working tree for sensitive values.

## Conventions

- ADRs are immutable decisions — **supersede** rather than rewrite. Index: `docs/architecture/README.md`.
- Specs live in `docs/specs/`; the testing strategy in `docs/testing-strategy.md`.
- Keep the ADR index and spec cross-references up to date when adding either.
