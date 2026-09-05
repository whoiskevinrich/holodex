---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-194                 # the tracker key; must match the branch key regex
status: in-progress               # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Added an Extract from filename action to the media detail page, with an inline panel for reviewing and applying what it finds — no trip to the owner tools required.
---

# HOLODEX-194 · Extract from filename on the media detail page (F48.5a)

F48's filename extraction shipped complete on the backend, including a per-video endpoint
(`POST /media/{id}/extract`, `internal/api/extract.go:13`) and its client wrapper
(`api.extractVideo`, `web/src/lib/api.ts:851`) — but the wrapper has **no callers**. The F48 design
handoff scoped the frontend to the `/owner` hub only, so extraction can only be triggered, and its
results can only be resolved, from a page other than the one showing the video whose metadata is
wrong.

The trigger alone would not close this. Extraction auto-apply defaults **off**
(`cmd/holodex/main.go:337`), so in the default configuration nearly every candidate lands as
`logged_only` or `queued` — a lone button would appear to do nothing, because its entire outcome
would be on another page. "Done" here means the media page can both **run** extraction and
**resolve** what it produced, reusing the owner tab's row and preview components unchanged.

Option B of four considered (trigger-only / trigger+inline review / dry-run preview / ambient
panel). Dry-run preview and the ambient always-on panel were deliberately deferred — see the
handoff's Non-goals.

**Design package:** [docs/specs/metadata-extraction.md](../specs/metadata-extraction.md) §F48.6b
(F48.6i–F48.6l) · [docs/design/media-page-extraction-handoff.md](../design/media-page-extraction-handoff.md)
\+ [mockup SVG](../design/media-page-extraction-mockup.svg) ·
[docs/design/media-page-extraction-qa-checklist.md](../design/media-page-extraction-qa-checklist.md)

## Gates — definition of done

- [x] spec `write-spec` → amended `docs/specs/metadata-extraction.md`: F48.5a's acceptance criteria
      now names the media-page surface, and a new §F48.6b adds F48.6i–F48.6l (panel, shared resolve
      path, `?video_id=` filter, explicit empty states)
- [x] architecture `architecture` — **not required for the implementation** (no new seam: reuses the
      existing extraction pipeline, resolve endpoint and write queue; the one backend change is an
      optional query parameter on an existing owner-gated route; ADR-067/068 unchanged). But the
      *design pattern* this work arrived at was generalized on the owner's instruction into
      [ADR-090](../architecture/ADR-090-two-layer-entity-metadata-management.md) — two-layer entity
      metadata management (adoption vs precedence), with this ticket as its first instance
- [x] design `design-handoff` → `docs/design/media-page-extraction-handoff.md` with a committed
      SVG mockup (4 states + implementer notes) and a numbered, verifier-tagged QA checklist
- [ ] backend → optional `?video_id=` filter on `GET /owner/extraction-queue`
      (`internal/api/extract_review.go:27`), owner-gated as today
- [ ] frontend → "Extract from filename" button in the Metadata actions row
      (`web/src/routes/media/[id]/+page.svelte:1090`) + inline panel reusing
      `ExtractionQueueRow` / `ExtractionPreviewDialog` unchanged; staging helper lifted out of
      `routes/owner/extraction/+page.svelte` and shared
- [ ] testing `testing-strategy` → handler test for the `video_id` filter (including the
      non-owner rejection), and a regression guard that media-page and owner-tab resolve are one
      code path
- [ ] security `security-review` → new query parameter on an owner-gated route + a new owner-only
      surface; needs a pass before ready-for-review

## Up next — ordered (position = priority)

1. [ ] [—] review the design package (handoff + mockup + spec amendment) on the Draft PR before
      implementation starts
2. [ ] [—] backend: `?video_id=` filter + handler test
3. [ ] [—] frontend: extract the shared staging helper, then the button + panel
4. [ ] [—] QA all three skins against the checklist, `/security-review`, mark PR ready for review

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-04 · ADR-090 — generalize the two-layer model
- skills: architecture
- handoff: Owner opted into this design's two-layer model as the standard for entity metadata
  management across all entities, explicitly choosing documentation over a tracking ticket ("the
  ticket would get lost"). Wrote
  [ADR-090](../architecture/ADR-090-two-layer-entity-metadata-management.md): layer 1 = adoption
  (transient, judged against the entity's own baseline), layer 2 = precedence (standing, ADR-051's
  chip row), with D1 forbidding a provider value inside an adoption row, D2 putting layer 1 on the
  entity page, D3 requiring the adopted value to visibly land with a `ProvenanceBadge`, D4 "a new
  source is a namespace not a subsystem", and D5 keeping layer-1 tables scoped where the question
  is. **A survey corrected the draft's central claim**: enrichment and duplicates already have
  inline entity-page entry points (`EnrichProviderChips`/`EnrichPicker`, `MergeOfferCard`) —
  extraction is the only queue without one, so D2 completes an existing pattern rather than
  introducing one. Scoped honestly: does not cover tags/categories (no layer 2 at all), merge/multi
  fields (no winner), entity-link fields, or identity fields; film is a half-instance. Indexed in
  `docs/architecture/README.md` and pointed at from the auto-loaded `.claude/CLAUDE.md` core-model
  section so it is found at the seam. Also noted in passing: PR #257 carries `ADR-088` which now
  collides with main's shipped ADR-088 (third collision on that branch) — needs renumbering to 091+.
  **Next session:** unchanged — design review on the Draft PR, then the backend `?video_id=` filter.

### 2026-09-04 · design critique → mockup rev 2
- skills: design-critique
- handoff: Critiqued the mockup against the question "how does this cohere with provider
  enrichment?" and found one critical gap: the design showed extraction dead-ending in its own
  panel, with nothing tying it to the resolved-fields list where `SourceBadge`/`ProvenanceBadge`
  already record provenance. Verified against `resolvePrecedence`
  (`internal/resolver/resolver.go:612`) and `internal/extract/store.go:12` that `filename` is a
  resolver namespace peer of `file`/`tmdb`, so the cross-source conflict is *already* handled by
  the shipped F36/ADR-051 chip row — nothing new needed, it just was not drawn. Mockup rev 2 adds
  **state 3** (provenance carrying through to the Metadata list + the expanded chip row with
  `filename` as a third source) and a **two-layer diagram** separating layer 1 (filename vs file
  tag, transient, this panel) from layer 2 (file vs filename vs tmdb, standing, SourceBadge). Also
  fixed the toolbar button, which the rev-1 mockup drew as a bordered pill — the real markup at
  `+page.svelte:1090` is borderless ghost text, and the boxed version would have shipped a control
  heavier than its neighbours. Handoff gained a "How this fits with provider enrichment" section
  with the two implementer rules that fall out of it; QA checklist gained 5 cases (2.5a, 3.6a,
  3.6b, 4.7a, 4.7b). **Next session:** unchanged — design review on the Draft PR, then the backend
  `?video_id=` filter.

### 2026-09-04 · gap analysis → option chosen → gate artifacts
- skills: design-handoff, write-spec, design-critique
- handoff: Investigated the reported gap ("no way to extract metadata from the filename on the
  media page"). Root cause is narrower than it looked: the per-video endpoint and its `api.ts`
  wrapper already exist and are owner-gated; only the surface is missing, and HOLODEX-194 was
  already filed for exactly this in July. Presented four options as side-by-side mockups; owner
  chose **B** (trigger + inline review) and "gates first, then implement". Wrote the design
  handoff, the committed SVG mockup, and the QA checklist; amended the F48 spec with §F48.6b
  (F48.6i–F48.6l) and an amendment note in the header. Renamed the branch
  `claude/media-metadata-extraction-d945e9` → `HOLODEX-194-media-page-extraction` and fired
  In Progress per ADR-058. **Next session:** wait for design review on the Draft PR, then
  implement in the order under "Up next" — backend filter first, since the frontend panel depends
  on it.
