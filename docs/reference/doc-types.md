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

## Transition (HOLODEX-523)

The numbered `ADR-NNN` files are being folded into topic docs (HOLODEX-526, HOLODEX-527) and then
removed (HOLODEX-528). Until then:

- **No new numbered ADRs.**
- A technology decision either edits the ADR that already covers it, in place, or starts a topic
  doc at `docs/architecture/<topic>.md`.
- Existing `ADR-NNN` links stay valid until HOLODEX-528 rewrites them.
