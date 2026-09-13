---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-380
status: in-progress
release_note: When a provider returns several records for the same release, the Enrich picker can now show each record's summary — studio chain, how fully it's catalogued — behind an info toggle that opens automatically when two candidates share a name; an auto-applied match's summary is kept in the activity log so it can be checked later.
---

# HOLODEX-380 · F61 — `candidates[].detail`: revealable per-candidate record summary

The partner video provider brought a proposal on 2026-09-13: its upstream catalogues one release
once per distribution outlet — identical label, date, cast; different outlet chain, tag count,
image set — and the picker's `label · disambiguation · confidence · view source ↗` gives the owner
nothing to choose *which record to bind to*, a choice that sets `_studio_external_ids` and `genres`.
Reviewed as maintainer, accepted as proposed with four open questions answered from a three-state
mockup. No ADR: additive optional response key under §2.3's unknown-key promise, same posture as
`profile_url` and `searched[]`.

**Decisions that were mine, not the proposal's** (flag on review): the resolve activity entry
now fires when *either* `searched[]` or an auto-applied candidate's `detail` is present — today
`RecordSearched` is a no-op on empty `searched[]`, which would silently drop the audit line for a
provider that emits `detail` but not `searched[]` (FR5). `detail` is **entity-agnostic**, unlike
`searched[]`'s `video`-only posture, because that posture came from `hint.*` being video-shaped and
nothing here is. Label-collision normalization is case-fold + whitespace-collapse + trim (FR4).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/candidates-detail.md` (F61: FR1–FR6, P1-a, P2-a–c, AC 1–11,
  test notes per surface, Resolved Decisions table) **and** the contract amendment
  (`metadata-provider-contract.md` §2.3 example + `candidates[].detail` row, §5 caps row)
- [~] architecture `architecture` — n/a: additive optional response key, no seam touched, no
  migration; `profile_url` (F47/RD6) precedent. Revisit only if P2-c (thumbnail) ever returns
- [ ] design `design-handoff` — three-state mockup (collapsed / one toggled / auto-expanded on
  collision) rendered in-session 2026-09-13; must be committed as SVG in `docs/design/` with the
  handoff doc; settles glyph, expanded-line typography, P1-a copy
- [ ] backend — `Candidate.Detail []string` (`internal/enrich/enrich.go`), `sanitizeDetail` sibling
  of `sanitizeSearched` with its own 8/256 caps, wired into the resolve sanitizer loop;
  `RecordSearched` learns the applied candidate (FR5); `Fake` gains per-candidate `Detail`
- [ ] frontend — `EnrichPicker.svelte`: toggle-button glyph trailing `view source ↗`, inline
  expansion, per-row state reset on new response, label-collision auto-expand; `EnrichCandidate`
  type; three-skin QA via computed styles
- [ ] testing `testing-strategy` — per the spec's Test Notes: sanitizer table test, refresh-all
  audit entry (applied / needs_review / no-entry), picker toggle + keyboard + collision, geometry
  (collapsed height parity, no clipping at 25), stub fixture with four same-label records
- [~] security `security-review` — n/a unless the implementation touches the SSRF perimeter (it
  must not — `detail` is rendered text, never fetched). Confirm at the backend gate

## Up next — ordered (position = priority)

1. [ ] [S] Open the **Draft PR** with this spec + contract amendment (spec gate lands first per
   ADR-069); gate-status checkboxes in the PR body mirror Jira.
2. [ ] [M] `/design-handoff` — commit the three-state mockup as SVG next to
   `docs/design/candidates-detail-handoff.md`; clear `needs-design` in Jira when it lands.
3. [ ] [M] Backend FR1/FR2/FR5 + tests, then frontend FR3/FR4 + tests + three-skin QA.
4. [ ] [S] `/testing-strategy`, then mark the PR ready → CI fires In Review.
5. [ ] [—] Tell the provider side the merged contract text matches the proposal so they can emit
   `detail` on every candidate (the audit path needs it on lone candidates too).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-13 · proposal reviewed, decisions locked, spec + contract amendment written
- skills: write-spec; maintainer review of the provider proposal with a three-state mockup
  (show_widget) to settle Q2/Q4
- Reviewed the proposal against the decoder, sanitizer, picker row, and refresh-all path; answered
  the four open questions (verbatim / toggle + inline / audit-log in v1 / auto-expand on
  collision); created HOLODEX-380, renamed the branch, fired In Progress; wrote the F61 spec and
  the §2.3 + §5 contract rows.
- Handoff: spec gate green and uncommitted on `HOLODEX-380-candidates-detail`; next is commit +
  Draft PR, then `/design-handoff` with the SVG.
