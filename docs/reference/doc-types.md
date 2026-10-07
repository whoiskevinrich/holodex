# Doc types: spec, architecture, design

Holodex keeps three kinds of design documentation. Each answers one question, and none of them
repeats another's content. All three are **living documents**: each one states what is true now,
is edited in place when that changes, and leaves the history to git.

| | Spec | Architecture | Design |
|---|---|---|---|
| Answers | What does the feature do? | Which technology or structure, and why not the alternative? | What does the owner see and touch? |
| Lives in | `docs/specs/` | `docs/architecture/<topic>.md` | `docs/design/` |
| Written when | Behaviour changes | A real technology fork (see below) | There is a visible surface |
| Never contains | Tables, endpoints, component or function names | User-visible rules, UI | Rules the backend enforces, schema |
| History | git | git, plus a "Decided in" link per section | git |

## Rules

- **Each fact lives in one document.** If another document needs a fact, it links to that
  document and doesn't restate it. Some decisions really do span two layers. In that case, write
  each half in its own document and link the two.
- **A changed product rule is a spec edit.** Never record a behaviour change as a new architecture
  doc or an amendment elsewhere. The PR that edits the spec carries the reasoning.
- **Architecture means a technology fork:** a choice of stack, data model, storage, service
  boundary, security perimeter, deployment or a cross-cutting code pattern, where a credible
  alternative was rejected. A feature with no such fork writes no architecture doc. Its worklog
  records the gate as a deliberate skip:

  ```
  - [~] architecture — no technology fork
  ```

- **Architecture sections cite their deciding commit.** End each decision section with a line
  linking the squash commit on `main` that made or last changed the decision:

  ```
  Decided in [`3f2a9c1`](https://github.com/whoiskevinrich/holodex/commit/3f2a9c1) (HOLODEX-123).
  ```

  When the decision changes, rewrite the section and update the link. The previous version is one
  hop away in git. Keep a rejected alternative in the text only while it still explains the
  current choice.
- **Process docs are not architecture.** CI, Jira, branching and agent tooling belong in
  `docs/reference/`.

## Enforcement

- **`.claude/rules/doc-types.md`** carries the boundaries above. It loads whenever a file under
  `docs/specs/`, `docs/architecture/` or `docs/design/` is opened.
- **`scripts/hooks/doc-type-guard.mjs`** is a PreToolUse hook. It injects the matching boundary when
  `/architecture`, `/write-spec` or `/design-handoff` is invoked. Those skills are plugin-owned and
  don't know these rules; the `/architecture` template scaffolds a numbered ADR. The hook also
  blocks creating a new `docs/architecture/ADR-NNN-*.md`.

## The archived ADRs

The numbered `ADR-NNN` files were folded into topic docs (architecture), specs and design docs
(product and UI rules) and `docs/reference/` (process), then moved to
`docs/architecture/archive/` (HOLODEX-523). Each archived file opens with a banner naming where its
content now lives, and `archive/README.md` maps every number.

- They are **history, never current truth**: don't cite, edit or extend them.
- An `ADR-NNN` mention in code, a spec or a worklog still resolves through the archive. Replace it
  with a topic-doc reference when you next touch that file; there is no sweep.
