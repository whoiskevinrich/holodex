# Holodex — Project Working Agreements

These rules govern how changes are made in this repo. They keep **specs, architecture,
design, tests, and security in lockstep**. Follow them for every change.

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

## Pre-commit checklist (every commit)

1. Run **`/simplify`** on the changed code (reuse, simplification, efficiency) — always, before committing.
2. If the change touched **auth, access, or infrastructure** → run **`/security-review`**.
3. Confirm the matching **spec / ADR / design / testing** artifacts (table above) were created or updated.
4. Confirm **no secrets, credentials, or PII** in the diff (see "Secrets & publishing").

## Before pushing or opening a PR

1. Run **`/productivity:update`** to sync task tracking (pull/triage tasks, refresh memory) — before every `git push` and `gh pr create`.
2. Re-confirm the pre-commit checklist above is satisfied for everything in the push.
3. Scan the working tree for secrets / PII (see "Secrets & publishing").

## Task tracking

- When you note a **TODO**, or **defer** an item (a stub, a "later", a "Phase 2", a known gap), run **`/productivity:update`** to capture it so it isn't lost in a code comment.
- Keep deferred work visible in the task list, not only inline in the source.

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
