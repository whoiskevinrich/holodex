---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-266                 # the tracker key; must match the branch key regex
status: in-progress               # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Person and studio pages now show a clickable provider badge (e.g. "IMDb") linking out to the source, matching the video badge from F55.
---

# HOLODEX-266 · Provider link badge — extend namespace-qualified display to person/studio

Extends the F55/ADR-082 provider-badge display (clickable provider-name badge, inline in the
entity header's metadata row, linking out to the third-party source) from video to person and
studio detail pages — done when a person/studio with a stored external id shows the same badge
video already gets, without touching completeness scoring or the ADR-054/055 identity model.

**Design package:** [ADR-083](../architecture/ADR-083-provider-link-badge-person-studio.md)
(extends [ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md))

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] architecture `architecture` → `docs/architecture/ADR-083-provider-link-badge-person-studio.md`
- [ ] design `design-handoff`
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [architecture] ADR: read-only projection (D1), provider-declared `link_templates` resolved
   server-side (D2), one badge per stored id (D3) — `docs/architecture/ADR-083-provider-link-badge-person-studio.md`
2. [ ] [design] design-handoff note covering the multi-badge and no-link-degradation states (visual
   spec otherwise reuses the existing video badge mockups from this session)
3. [ ] [backend] `Manifest.LinkTemplates map[string]map[string]string` (`internal/enrich/enrich.go`)
   + sanitize/validate on `/describe` ingest + `BuildProviderLink(namespace, entityKind, id)` helper
4. [ ] [backend] person/studio detail handlers project `person_external_ids`/`studio_external_ids`
   into `external_links: [{provider, label, url}]` via the new helper
5. [ ] [frontend] extract the video badge into a shared component; person/studio headers render
   zero-to-many badges from `external_links`
6. [ ] [testing] template-mismatch degradation + multi-badge cases
7. [ ] [security] `LinkTemplates` validation (single `{id}` placeholder, `https://` scheme, no
   injection via a malicious provider) before interpolation into a served URL

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-09 · Architecture gate closed — ADR-083 written
- skills: architecture
- handoff: this session started from an observation that `imdb_id` was mislabeled for a
  provider-agnostic deployment (already fully resolved at the schema/registry/provider level by
  ADR-082 — the only real gap was frontend display), iterated through mockups to a clickable
  provider-name badge placed inline in the header metadata row (not id text, not a separate
  section), then the owner asked to extend that badge to person and studio. Research showed that's
  not a copy-paste: `person_external_ids`/`studio_external_ids` (ADR-054/055) are join-table
  identity keys outside the resolver, not resolved scalars like video's `external_provider_id`.
  Wrote ADR-083: D1 person/studio badge data is a read-only projection of those tables (no
  resolver/F55-scoring change); D2 the outbound link is built server-side from a new
  provider-declared `Manifest.LinkTemplates` (extends the `/describe` contract alongside
  `IDNamespaces`/`BrandIcon`) rather than a frontend-hardcoded map, keeping "providers declared,
  not compiled in" (ADR-033) intact; D3 render one badge per stored external-id row (0..N) rather
  than inventing a "primary" — an intentional asymmetry from video's single badge. Added the
  ADR-083 row to `docs/architecture/README.md`. HOLODEX-260 (the original F55 epic) is fully done
  and merged (PR #222), so this scope extension got its own epic, HOLODEX-266, rather than
  reopening a closed one; renamed the worktree branch and fired the Jira In Progress transition.
  Next: `/design-handoff` for the multi-badge/no-link states, then backend (`LinkTemplates` +
  the person/studio projection).
