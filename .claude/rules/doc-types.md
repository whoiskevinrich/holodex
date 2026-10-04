---
paths:
  - "docs/specs/**"
  - "docs/architecture/**"
  - "docs/design/**"
---

# Doc types: spec, architecture, design

Full rules: `docs/reference/doc-types.md`. All three are **living docs**: each states what is true
now and is edited in place; git is the history. **One fact lives in one doc** — link, don't restate.

| You are writing… | It answers | It must never contain |
|---|---|---|
| a **spec** (`docs/specs/`) | what the feature does, as the owner experiences it | tables, endpoints, component or function names |
| an **architecture doc** (`docs/architecture/<topic>.md`) | which technology or structure, and why not the credible alternative | user-visible rules, thresholds the owner sees, UI |
| a **design doc** (`docs/design/`) | what the owner sees and touches: components, flows, states | rules the backend enforces, schema |

- **A changed product rule is a spec edit**, never a new architecture section or ADR.
- **Architecture is written only for a real technology fork.** No fork → the worklog records
  `[~] architecture — no technology fork` and no doc is written.
- **No new numbered ADRs.** Edit the topic doc that owns the decision (index:
  `docs/architecture/README.md`), or start a new topic doc. Each section ends with a "Decided in"
  line linking the deciding squash commit (format in `docs/reference/doc-types.md`).
- Material that belongs to another type goes there (or onto the ticket), not into this doc
  "for context".
