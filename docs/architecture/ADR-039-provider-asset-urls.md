# ADR-039: Provider asset URLs — contract clarification + operator-configured asset-host allowlist

**Status**: Proposed
**Date**: 2026-06-16
**Deciders**: Project owner
**Relates to**: spec [Metadata Provider Contract](../specs/metadata-provider-contract.md) (the external-facing hand-off spec); extends [ADR-033](ADR-033-metadata-source-plugins.md) (metadata source plugins — provider protocol & SSRF perimeter) and [ADR-038](ADR-038-person-images.md) (person images — the asset-fetch that consumes provider URLs). Responds to an external provider team's RFC, "Specify how photo (and other asset) URLs are returned".

---

## Context

A provider sidecar's `POST /enrich` response carries field values in a `fields` map and linkable artifacts (a person portrait) in an `assets[]` array ([ADR-033](ADR-033-metadata-source-plugins.md)). The external provider team filed an RFC observing — correctly — that the **shape, advertisement, and URL constraints** of assets are underspecified in the hand-off contract, and that at least one wart exists (`photo` is advertised inside the `/describe` `fields` list even though it is never delivered as a field).

Two forces make this more than a doc tidy-up:

- **The contract spec is stale.** It still says *"Holodex's v1 client parses `assets` but does not download them."* That is no longer true: [ADR-038 / F25](ADR-038-person-images.md) shipped asset **download** — [`internal/enrich/assets.go`](../../internal/enrich/assets.go) fetches a provider asset URL, runs it through the person-image normalizer (decode → bound → re-encode → metadata strip), and stores it as a person image. The external contract and the shipped behaviour have diverged.

- **The RFC's URL model conflicts with our shipped SSRF perimeter.** The RFC proposes that the asset `url` point at "a host the provider controls or trusts" — explicitly an upstream **CDN on a different host** (`https://cdn.example.org/…`), with the provider **not** re-hosting the bytes. But our asset fetcher ([`assets.go:73`](../../internal/enrich/assets.go)) refuses **any host other than the provider's own `base_url` host**, and [ADR-038 §2](ADR-038-person-images.md) mandates reusing "the existing F22 network guards verbatim." The two cannot both be true.

  The conflict is not academic. A real **TMDB** provider serves portraits from `image.tmdb.org` — a different host from its sidecar (`http://holodex-tmdb:9100`). Under today's strict same-host rule, **Holodex would refuse to fetch any real TMDB photo.** The RFC also mandates `https`-only, but internal sidecars are reached over `http`; [`assets.go:70`](../../internal/enrich/assets.go) deliberately allows both.

The SSRF perimeter exists for a reason: an asset URL is a **provider-supplied value**, and provider responses are untrusted (ADR-033 §6, S2/S3). If Holodex fetched whatever host a provider named, a compromised or buggy provider could point the fetch at `169.254.169.254` (cloud metadata), an internal admin service, or any intranet host — classic SSRF amplification. The same-host rule closed that vector by making the fetch host equal to the one value the operator already vetted in config (`base_url`). The question this ADR settles is **how to let real CDN-backed providers work without reopening that vector.**

### Constraints

- **Trust must come from operator config, never from a provider response.** This is the load-bearing invariant of ADR-033's perimeter; any widening must preserve it.
- **No protocol-version bump.** The RFC is framed as a v1 clarification; the wire stays protocol version `1`. Changes must be additive and backward-tolerant.
- **Small footprint.** Single Go binary on modest hardware (ADR-008/023); no new dependency, no new service.

## Decision

Adopt the RFC as a **contract clarification**, with one substantive amendment to its URL model. Six parts:

### 1. Pin the asset object schema; ignore unknown kinds and keys (ratify shipped behaviour)

An asset is `{ "kind": <string>, "url": <absolute URL> }`. Holodex **ignores unknown `kind`s** (skips them rather than guessing a role) and **ignores unknown keys** — both already true: [`assetRoleFor`](../../internal/enrich/assets.go) returns `ok=false` for an unrecognized kind and the asset is dropped, and the JSON decoder discards unknown fields. This forward-compat is ratified as a contract guarantee so providers can emit future kinds/keys without a protocol bump.

**Recognized kinds (v1).** The contract advertises three person-image kinds — `photo`, `banner`, `poster` — and Holodex additionally accepts the synonyms it already maps (`portrait`/`headshot` → headshot, `backdrop` → banner). This documents the shipped [`assetRoleFor`](../../internal/enrich/assets.go) table rather than the RFC's narrower "exactly one kind: photo."

### 2. Advertise asset kinds separately in `/describe` (`asset_kinds`)

Add an optional `asset_kinds: ["photo", …]` array to the `/describe` manifest, parallel to `fields`, removing the wart where `photo` is advertised as a field. `Manifest` gains an `AssetKinds []string` (parsed; advisory/display only — Holodex does not gate fetching on it). **Backward tolerance:** Holodex continues to accept `photo` appearing in a provider's `fields` list during a deprecation window, treating it as equivalent to `asset_kinds: ["photo"]`. New providers SHOULD use `asset_kinds`.

### 3. URL host trust — operator-configured `asset_hosts` allowlist (the amendment)

The asset `url` MUST be an **absolute `http`/`https` URL whose host is on a per-source allowlist** composed of:

- the provider's own `base_url` host (the implicit, always-allowed entry — preserves the same-host path for providers that re-host their own bytes), **plus**
- any host the operator explicitly lists in a new **`asset_hosts`** field on that source in `metadata-sources.yaml`.

```yaml
sources:
  - name: tmdb
    base_url: http://holodex-tmdb:9100
    entity_types: [person]
    asset_hosts: [image.tmdb.org]   # operator-vetted CDN host(s) for this provider
    enabled: true
```

Trust still derives **entirely from operator config**, exactly like `base_url` — never from the provider response. A compromised provider can only name a host the operator already allowlisted; it cannot add one. Matching is **exact host** (no wildcard/subdotmatch in v1), so the operator names precisely the CDN hosts that provider uses. The other guards are unchanged and retained verbatim: **cross-host redirects refused** (a 30x off the initial asset host is not followed), **response size-capped** (16 MiB), **timeout-bounded**. We **reject** the RFC's blanket `https`-only rule (internal same-host sidecars are `http`) but **require `https` for any cross-host (public-internet) asset host** — `http` is permitted only for the provider's own `base_url` host on the trusted internal network. We likewise **reject** the RFC's "provider may name any host it trusts" rule (that is the SSRF vector); the operator allowlist is the controlled substitute.

**Discovery is a deferred convenience, not the trust root.** A provider MAY advertise the hosts it needs (e.g. an `asset_hosts` array in `/describe`) so an operator doesn't have to know a provider's CDN hostname by hand. If added, that advertisement is **only a suggestion surfaced to the operator for an explicit approval click** (a trust-on-first-use consent, OAuth-scopes-shaped) — never auto-trusted, since `/describe` is a provider response. The *grant* always remains an operator action writing the host into config. This consent UX is out of scope for this ADR (a follow-up); v1 is operator-typed `asset_hosts`.

This **supersedes the mechanism** of [`assets.go`](../../internal/enrich/assets.go)'s `host == base_url host` check (which becomes `host ∈ {base_url host} ∪ asset_hosts`), without changing ADR-033's invariant.

### 4. Fetch-soon, no long-lived URL storage (ratify) — `expires_at` deferred

Holodex already treats asset URLs as **fetch-soon**: it downloads the bytes at enrich time and stores the *image*, never persisting the URL as a long-lived reference. Signed/short-lived upstream URLs therefore work as long as they are valid at enrich time. The RFC's optional `expires_at` hint is **deferred to a future version** (unknown keys are ignored per §1, so it can be added without a protocol bump).

### 5. Multiple assets — preference-ordered, first success per role wins

`assets` MAY contain more than one entry; within a kind the array is **preference-ordered, most-preferred first**. Holodex uses the **first asset of a given role it can successfully fetch+store**, then stops for that role. Today [`downloadAssets`](../../internal/enrich/service.go) fetches *every* asset; this ADR tightens it to first-success-per-role so a provider can list fallbacks without Holodex storing duplicates into a single-slot core role. The global caps (1 MiB JSON body; 16 MiB per asset) still bound the array.

### 6. Empty = omit; shed assets before fields (bless as provider-side rules)

`assets` is **omitted when empty** (never `[]`), matching the `fields` rule; and when trimming to the 1 MiB body cap a provider **sheds `assets` before dropping any `fields`** (fields are canonical text; an asset URL is recoverable on a later enrich). Both are provider-side obligations Holodex already tolerates (decode treats absent and `[]` identically); they are documented as contract rules with **no core code change**.

## Options considered

### Asset-host trust model (the core decision)

| Option | Complexity | Security | Provider ergonomics | Notes |
|---|---|---|---|---|
| **A. Operator-configured `asset_hosts` allowlist (chosen)** | Low | Strong — trust from operator config, not provider response; perimeter widened to named hosts only | Good — real CDN providers (TMDB) work without re-hosting | Small additive config field; reuses existing redirect/size/timeout guards |
| B. Strict same-host (status quo) | None | Strongest — smallest surface | Poor — every provider must proxy/re-host image bytes through its own host | Zero core change, but contradicts the RFC and pushes real work onto every provider; TMDB needs a byte-proxy |
| C. Trust the provider-named host | Low | **Weak — reopens the SSRF vector** the perimeter was built to close | Best | A compromised provider could point Holodex at metadata/intranet endpoints. Rejected on security grounds |

**Pros (A):** preserves the "trust comes only from operator config" invariant; enables real providers; minimal, additive surface; degrades safely (no `asset_hosts` ⇒ same-host-only, identical to today).
**Cons (A):** widens the SSRF perimeter (mitigated: operator-controlled, exact-host, no-redirect, capped — and gated through `/security-review`); one more config knob operators must set correctly.

### Who downloads the bytes (the network-egress boundary)

This is a distinct axis from *host trust*: given a trusted host, **who makes the fetch, and when**. It is as much a product-posture call as a technical one (see the trade-off analysis).

| Option | Core egress | Plugin-author effort | Fit with posture | Notes |
|---|---|---|---|---|
| **A. Core fetches the URL synchronously at the owner's enrich click (chosen)** | Core gains an outbound-internet path (to allowlisted hosts only) | Low — a provider returns a link | Best — the fetch is an explicit owner action; no scheduler; URL is freshest (no expiry race) | The bytes pass the ADR-038 normalizer before disk |
| B. Provider re-hosts/proxies the bytes through its own `base_url` host | **None** — the sidecar stays the *sole* internet egress boundary; core can run fully network-internal | High — every provider must also serve image bytes | Strong isolation story for the privacy/air-gap segment | Rejected for v1: contradicts the RFC's "no re-host" and taxes every plugin author; revisit if isolation demand appears |
| C. Defer: store the URL, download via a background job / lazy-on-view | Core egress, deferred | Low | **Poor** — a download queue fights the explicit "no crawler/scheduler/background sync" posture (ADR-033); lazy-on-view moves egress to a public, viewer-triggered GET; either loses the signed-URL freshness | Rejected |

Chosen **A**: it is where the plugin-ecosystem value is (lowest author bar), the "approved hosts only" guardrail keeps the new egress small, and synchronous-at-enrich is the cleanest fit for the on-demand, no-scheduler posture. **B remains the escape hatch** if a future need to keep core fully internet-isolated outweighs plugin-author convenience.

### Advertising assets in `/describe`

| Option | Notes |
|---|---|
| **A. Add `asset_kinds`, tolerate `photo`-in-`fields` (chosen)** | Removes the wart, additive, backward-tolerant; no protocol bump |
| B. Keep `photo` in `fields` | Perpetuates the "advertised as a field, delivered as an asset" inconsistency the RFC flagged |

## Trade-off analysis

The decision trades a **deliberately widened SSRF perimeter** for **real-world provider viability**. The widening is bounded on every axis that matters: the additional hosts are named by the **operator** (the same trust basis as `base_url`, which a provider also cannot influence), matched **exactly**, fetched with **no cross-host redirect**, **https on the public internet**, **size-capped**, and **timeout-bounded**, with the downloaded bytes still passing through the ADR-038 normalizer (decode/strip) before they touch disk. The residual risk over the status quo is that an operator mis-lists a host they don't actually trust — a configuration error, the same class as mis-setting `base_url` to a hostile target, and out of scope for the code to second-guess. On the host-trust axis, Option C would have been strictly easier but reopens exactly the vector ADR-033 closed; strict same-host is safest but makes the contract's own worked example (TMDB) unimplementable without a byte-proxy.

On the **egress axis**, the chosen "core fetches" is a genuine product-posture trade, not just an engineering one: it gives core its first outbound-internet path (kept small by the operator allowlist) in exchange for the **lowest possible plugin-author bar** — a provider returns a link rather than standing up an image server. The alternative (provider re-hosts) would keep core fully network-isolatable — a real selling point for the local-first/air-gap segment — at the cost of taxing every plugin author and contradicting the RFC. We optimize for ecosystem breadth now and keep provider-re-host as a documented escape hatch (Option B) if the isolation demand materializes.

## Consequences

- **`internal/enrich`**: `Source` gains `AssetHosts []string` (parsed from YAML); `AssetClient` carries an **allowed-host set** ({base host} ∪ asset_hosts) instead of a single host; [`downloadAssets`](../../internal/enrich/service.go) dedups to **first-success-per-role**; `Manifest` gains `AssetKinds []string` (advisory). The scheme check stays `http`/`https`; the redirect/size/timeout guards are untouched.
- **Config**: `metadata-sources.yaml(.example)` documents the optional `asset_hosts` per source. Absent ⇒ same-host-only (today's behaviour exactly) — a safe default, no migration.
- **Contract spec** ([`docs/specs/metadata-provider-contract.md`](../specs/metadata-provider-contract.md)): remove the stale "does not download assets in v1" note; pin the asset schema and recognized kinds (§2.4, §4.3); document `asset_kinds` (§2.2) and the `asset_hosts` operator allowlist (§6 S7 + §10 wiring); add the RFC's conformance items. The worked **TMDB** spec ([`tmdb-provider.md`](../specs/tmdb-provider.md)) gains an `asset_hosts: [image.tmdb.org]` example.
- **What gets easier**: real CDN-backed providers (TMDB) become implementable without a byte-proxy; future asset kinds (`banner`, `thumbnail`) and asset metadata (`expires_at`, `width`/`height`, `mime`) drop in without a protocol bump (unknown kinds/keys already ignored).
- **What gets harder**: operators of a CDN-backed provider must list its image host(s); a provider that re-hosts its own bytes needs no change.
- **Security**: this widens an SSRF perimeter and therefore requires a `/security-review` sign-off before merge (per the project working agreements).
- **What we'll revisit (deferred)**: provider-advertised `asset_hosts` in `/describe` + an operator **consent UX** (trust-on-first-use approval, never auto-trust); `expires_at` asset expiry hints; richer asset metadata (`width`/`height`/`mime`/`alt`/`source`); wildcard/subdomain matching in `asset_hosts` if exact-host listing proves too rigid; additional asset kinds beyond the person-image roles; **provider-re-host (Option B)** as an isolation escape hatch if a fully internet-isolated core becomes a requirement.
- **Supersession**: immutable per repo convention — superseded, not edited, if the asset-host trust model changes again (e.g. to a global allowlist or a signed-handoff scheme).

## Action items

1. [ ] Update [`docs/specs/metadata-provider-contract.md`](../specs/metadata-provider-contract.md): drop the stale "not downloaded" note; pin asset schema + recognized kinds; add `asset_kinds`; replace the §6/S7 URL rule with the `asset_hosts` allowlist; add §10 operator wiring + the conformance items.
2. [ ] `internal/enrich`: add `Source.AssetHosts`; widen `AssetClient` to an allowed-host set; first-success-per-role in `downloadAssets`; add `Manifest.AssetKinds`. Unit-test host allow/deny (base host, listed host, unlisted host, cross-host redirect) and per-role dedup.
3. [ ] `metadata-sources.yaml.example`: document `asset_hosts` on the `tmdb` entry; update the example field-mapping in [`tmdb-provider.md`](../specs/tmdb-provider.md).
4. [ ] `/testing-strategy`: align tests to the new host-allowlist and dedup behaviour.
5. [ ] `/security-review` sign-off on the perimeter widening before merge.
6. [ ] Update the ADR index (this row) and the contract spec's references block.
