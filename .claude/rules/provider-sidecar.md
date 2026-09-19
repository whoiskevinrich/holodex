---
paths:
  - "providers/**"
  - "internal/enrich/**"
---

# Provider sidecars

- **`providers/tmdb` is a standalone sidecar in the same Go module.** It talks to the core only
  over HTTP (protocol v1: `/healthz` · `/describe` · `/resolve` · `/enrich`) and **must not
  import `internal/*`**. The authoritative, source-neutral protocol — endpoints, caps, security
  rules — is [`docs/specs/metadata-provider-contract.md`](../../docs/specs/metadata-provider-contract.md);
  change it and both sides together.
- **Contract-doc gate (every commit).** Any new or altered provider ability — a `/describe` key,
  a request/response field, a `_`-prefixed sidecar, a canonical field's meaning, a cap, a
  status code, a behaviour the core client enforces — **must land in
  `docs/specs/metadata-provider-contract.md` in the same commit**, in every place an
  implementer would read it: the §2 endpoint table, the §4 subsection (new or existing), the
  canonical-field row itself if a field's semantics moved (not just a cross-reference from
  elsewhere), the §8 worked example, and `testdata/enrich-stub/` if the stub is meant to
  mirror it. A provider change with no contract-doc diff is incomplete — check
  `git diff --stat` for the spec before committing. (#344 stated a `website`/`homepage` rule
  only in §4.11 and left the §4.2 field rows stale; #345 was the follow-up.)
- **`_`-prefixed enrichment field keys are internal provider→core sidecars, not display fields**
  (`model.InternalFieldPrefix`, ADR-054). They're persisted in the shadow store but **never
  resolved or rendered** (`enrich.FieldsFromRows` skips them). They're cross-boundary contracts
  shared as string literals (core + every provider) — never invent new ones ad hoc. v1 defines
  `_studio_external_ids` (studio de-dup by id); `_source_url` (the provider's own page for the
  enriched entity — the badge's per-pill fallback behind `link_templates`, ADR-098, contract §4.12).