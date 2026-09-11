---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-364
status: in-progress
release_note: A film's description now sits at the top of the right-hand column with its tags, reads the same for the owner and for visitors, and the owner edits its source from a pencil beside the heading instead of a second copy lower down.
---

# HOLODEX-364 · Film detail page — description onto the media page's overview rule

The follow-on HOLODEX-363 named: the two detail pages share `stage-grid` and had answered "where
does the synopsis go, and for whom?" differently. The film page rendered the description in the
header for everyone and — for the owner — a second time in the owner-only Details section as
`SourceBadge`'s ink, unclamped value span. That was a deliberate split that became drift once the
media page decided the opposite ([ui-vocabulary](../reference/ui-vocabulary.md), "deliberate
split vs. drift"), and the owner's double rendering was exactly the split HOLODEX-365 fixed on the
media Overview.

**What changed.** `#field-description` is the rail's first block, above Tags, unconditional (no
viewport branch, no role branch — the deep link must have one target). One `ExpandableText`
carries the value for both roles; the owner's pencil in the heading opens `SourceEditModal`
(`baselineKey="record"`); `SourceBadge` left the description entirely. `Details` kept only
`release_date`, still owner-only, its `hasDetails` gate and Enrich chip row untouched.

**Decisions the ticket left open, taken here:**

- **Tags moved to the rail with it.** The ticket said "top of the rail, above the film's Tags
  block", but the film's Tags were in the header. The column contract puts tags in the rail and
  the media page has them there under Overview, so the description and Tags moved together — the
  smallest change that makes both the ticket and the contract true. Cheap to revert if the owner
  wants Tags back in the header.
- **Visitor `ProvenanceBadge`: yes, and converged.** The rule says visitors get values plus their
  badge; the Person bio did, the media Overview did not. The film description shows it, and the
  media Overview gained the same three lines in this PR, so the three `long_text` blocks agree.
- **The "Enriched from X" visitor note stays retired.** It was dropped as a side effect of gating
  Details; on its merits, the per-value badge already says which provider each visible value came
  from, and a section-level note would be a second answer to the same question.
- **No film-specific mockup.** `-mb-14` is a fixed 56px overlap at 375 / 768 / 1440, and the
  header's height is set by the 240px poster, not by the prose, so the header/banner band did not
  visibly change. `needs-design` cleared on that basis.

## Gates — definition of done

- [~] spec `write-spec` — n/a: same field, same decision model, same values; only where it renders
  and which control opens the decision changed (same reasoning as HOLODEX-363 / HOLODEX-365)
- [~] architecture `architecture` — n/a: frontend-only, no seam touched
- [x] design `design-handoff` — the rail block is the HOLODEX-303 / HOLODEX-365 pattern
  ([person-detail-bio-header-handoff.md](../design/person-detail-bio-header-handoff.md)); the
  HOLODEX-363 handoff §3c now records the resolution and the measured overlap
- [~] backend — n/a
- [x] frontend — `films/[id]/+page.svelte`: description + Tags into the rail, pencil →
  `SourceEditModal`, visitor badge, `detailFields` excludes the description; `media/[id]/+page.svelte`:
  visitor `ProvenanceBadge` under the Overview. Verified live on `backend-films` film 1 (Dune,
  TMDB-enriched): owner and visitor both 14px / 22.75px muted `line-clamp-5` with chevron; owner
  pencil opens the modal with record / tmdb (selected) / Custom; visitor sees the tmdb badge and no
  Details; all three skins take tokens (prose == eyebrow colour, badge on the logo plate); no
  horizontal overflow at 375 / 768 / 1440. Media 208 (Dune 1984): visitor badge, owner pencil,
  same typography
- [~] testing `testing-strategy` — n/a: no logic added; `npm run check` 0 errors. Parity is
  satisfied by construction (one unconditional `ExpandableText`); HOLODEX-366's rung, when it
  lands, gains `field-description` as its second acceptance case
- [~] security `security-review` — n/a: no auth, access or infrastructure change; the pencil is
  gated on the same `isOwner && isReplaceField` term the badge row was

## Up next — ordered (position = priority)

1. [ ] [—] **Owner eyeballs film 1 on `backend-films`** — especially the Tags-to-rail call — then
   mark the PR ready; CI moves this ticket to In Review.
2. [ ] [S] [HOLODEX-366](https://whoiskevinrich.atlassian.net/browse/HOLODEX-366) — the harness
   parity assertion; add `field-description` to its acceptance note ("reverting HOLODEX-364 fails
   on `field-description`").
3. [ ] [S] **Extract the heading-pencil + `ExpandableText` + badge block?** HOLODEX-365 declined a
   `LongTextField` at two call sites and named this ticket as the trigger for a third. The media
   and film blocks are now byte-identical apart from the field and label; the Person bio differs
   (h3, `lines={4}`, hero column, snippet pencil). Owner's call — file a ticket if yes.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-11 · built and verified in one pass
- skills: code-review (high --fix), code-review
- copied the media page's `#field-overview` block rather than extracting it (ticket item 2 says
  "copy that block"); the extraction question is queued above, not decided here.
- one miss caught live: the modal's baseline chip read "File" until `baselineKey="record"` was
  passed — films are record-baselined like Person/Studio, and the media page (file-baselined)
  did not need the prop, so copying its block dropped it.
- testbed note: `backend-films` needs `FILMS_ENABLED=true` in the local `launch.json` env or every
  `/films` route 404s; and the TMDB sidecar must take its token from the user environment
  (ADR-094) — the stale `env` block in this worktree's gitignored `launch.json` returned 401 until
  removed.
- handoff: implementation complete and live-verified; Draft PR open. Next is the owner's look at
  film 1 (Tags-to-rail is the one call to confirm), then mark ready.
