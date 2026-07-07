---
paths:
  - "providers/**"
---

# Provider sidecars

- **`providers/tmdb` is a standalone sidecar in the same Go module.** It talks to the core only
  over HTTP (protocol v1: `/healthz` · `/describe` · `/resolve` · `/enrich`) and **must not
  import `internal/*`**. The authoritative, source-neutral protocol — endpoints, caps, security
  rules — is [`docs/specs/metadata-provider-contract.md`](../../docs/specs/metadata-provider-contract.md);
  change it and both sides together.
- **`_`-prefixed enrichment field keys are internal provider→core sidecars, not display fields**
  (`model.InternalFieldPrefix`, ADR-054). They're persisted in the shadow store but **never
  resolved or rendered** (`enrich.FieldsFromRows` skips them). They're cross-boundary contracts
  shared as string literals (core + every provider) — never invent new ones ad hoc. v1 defines
  `_studio_external_ids` (studio de-dup by id).