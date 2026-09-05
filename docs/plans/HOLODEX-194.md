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
- [~] architecture `architecture` — not required: no new seam. Reuses the existing extraction
      pipeline, the existing resolve endpoint, and the existing write queue; the one backend change
      is an optional query parameter on an existing owner-gated route. ADR-067/068 unchanged.
      (The dry-run variant *would* have needed an ADR — it was deferred partly for that reason.)
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

### 2026-09-04 · gap analysis → option chosen → gate artifacts
- skills: design-handoff, write-spec
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
