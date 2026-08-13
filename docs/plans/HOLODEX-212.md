---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-212                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fixed a security gap where provider-sourced cover-art downloads and image_url fields could bypass the metadata-provider host allowlist.
---

# HOLODEX-212 · Close the SSRF allowlist gap on image writeback + resolved image_url fields

Done means every path that fetches or renders a provider-sourced image — writeback's cover-art
download and resolver/API `image_url` display — is gated by the same ADR-039 asset-host allowlist
the F39 auto-registration path already enforced, with fail-closed defaults and no divergence
between the resolver-side and API-side gates.

**Design package:** ADR-039 (asset-host allowlist) · ADR-056 (image perimeter) — no new spec/ADR/design; this is a conformance fix to an existing decision, not a new one.

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [~] spec `write-spec` — not applicable: no requirement/scope change, closes a gap in an existing decision (ADR-039)
- [~] architecture `architecture` — not applicable: enforces ADR-039/056 as already decided, no new architectural decision
- [~] design `design-handoff` — not applicable: no operator-facing screen/flow change (image_url degrades to existing text display, no new UI)
- [x] backend — `internal/writeback/writeback.go`, `internal/enrich/service.go`, `internal/resolver/resolver.go`, `internal/api/field_promotions.go`, `internal/api/handlers.go`, `cmd/holodex/main.go`
- [~] frontend — not applicable: no `web/**` changes; SPA already renders `display: "text"` vs `image_url` correctly
- [x] testing `testing-strategy` — `internal/enrich/fetch_allowed_image_test.go`, `internal/resolver/image_gate_test.go`, `internal/writeback/image_fetch_test.go`
- [x] security `security-review` — clean run, 0 findings ≥8/10 confidence; fail-closed behavior verified end-to-end

## Up next — ordered (position = priority)

1. [ ] [—] open PR, get it merged — no code work remains

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-13 · Implemented the SSRF perimeter fix end to end

Closed two gaps: (1) `writeback.downloadImageToTemp` fetched cover-art images with no
host-allowlist check at all — fixed via a fail-closed `ImageFetcher` seam
(`writeback.SetImageFetcher`) wired to a new `enrich.Service.FetchAllowedImage`, which checks the
URL's host against every *enabled* provider's allowlist (base host ∪ `asset_hosts`) before
fetching. (2) Canonical `image_url` fields (`poster_url`, studio `logo`) resolved by
`internal/resolver` had no gate at all — unlike the existing F39/F44 API-side gate — so a
provider-sourced image URL could render as an `<img>` regardless of allowlist; fixed via
`resolver.Options.ImageURLAllowed` (a plain predicate, since the pure resolver can't import
`internal/enrich`) and a new `gateImageDisplay` step in `ResolveFields` that degrades a
disallowed image_url to text display. Both gates exempt `file`/`manual` winning sources — those
are operator/file-controlled, not the untrusted provider vector this perimeter protects.

While comparing the new `resolver.gateImageDisplay` against the pre-existing API-side
`api.gateImageURL` (F39/F44 auto-registration/promotion), `/simplify`'s reuse pass found the two
had silently diverged: `gateImageURL` lacked the file/manual exemption, so a promoted field
winning from `file:...`/`manual:...` would call `ImageURLAllowed("file"/"manual", url)`, which
never matches a configured provider and returns false — silently degrading a legitimate
file-embedded or owner-typed image to plain text. Fixed by aligning `gateImageURL` with the same
exemption. `/security-review` ran clean (0 findings ≥8/10): fail-closed on nil fetcher, nil
callback, and unlisted host all verified; no bypass via userinfo/case/redirect tricks; no way for
an attacker-controlled provider to spoof the file/manual exemption.

- skills: testing-strategy, simplify, security-review
- handoff: code + tests + gates are done and green; next session should scan for secrets, commit, push `HOLODEX-212-ssrf-image-gate`, and open the PR (Jira already `In Progress`, issue gate-status checkboxes and PR body still need the same checklist).
