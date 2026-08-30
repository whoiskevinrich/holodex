---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-293                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: [HOLODEX-292]               # [KEY-…] cross-epic deps that must land first
release_note:                # not user-facing — pure internal markup consolidation, no behavior change
---

# HOLODEX-293 · Migrate categories/[id] tag chips to shared TagLinkChip

Replace `categories/[id]`'s pre-HOLODEX-292 inline owner/visitor tag-chip markup with the shared
`TagLinkChip.svelte` component (already wired into Media and Film). Done means: the category page's
chip markup byte-matches the Media/Film call-site pattern, `npm run check` is clean, and both
owner/visitor rendering is verified across all three skins.

**Design package:** no spec/ADR/design-handoff (pure like-for-like markup swap onto an
already-designed and already-reviewed component — same precedent as HOLODEX-292) · [testing-strategy.md §11](../testing-strategy.md)

## Gates — definition of done

- [~] spec `write-spec` — not applicable, no new capability, identical precedent to HOLODEX-292
- [~] architecture `architecture` — not applicable, no schema/cross-cutting decision
- [~] design `design-handoff` — not applicable, reuses HOLODEX-292's already-committed design as-is
- [~] backend — not applicable, `category.tags` already fully populated, no query changes
- [x] frontend → `web/src/routes/categories/[id]/+page.svelte`
- [x] testing `testing-strategy` — manual driven-browser QA (owner/visitor × 3 skins), no backend to Go-test
- [~] security `security-review` — not applicable, no auth/access/infrastructure touched

## Up next — ordered (position = priority)

1. [ ] [—] Jira sync: confirm HOLODEX-293 reflects gates above, push, open PR

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-29 · session
- skills: simplify
- handoff: Migrated `categories/[id]`'s inline owner/visitor tag-chip pair (the last unmigrated
  caller from HOLODEX-292) to `<TagLinkChip tag={t} busy={tagBusy} onremove={isOwner ? removeTag : undefined} />`,
  deleting the duplicated markup and its now-stale comment pointing at code that no longer exists
  on the Media page. Confirmed `removeTag(tagId: number)` already matches the prop signature
  exactly (no closure needed). `npm run check`: 0 errors. Verified live in-browser: created a test
  category, added/removed a tag, confirmed the owner chip (remove button, full hit-target, static
  `×` glyph) and visitor read-only pill (full 72×29.6px padded `<a>`, matches WCAG-safe hit target)
  render correctly across Cinémathèque, Broadcast, and Brutalist. Branch stacked on
  `HOLODEX-292-tag-link-chip` (not `main`) since PR #270 hasn't merged yet and `TagLinkChip.svelte`
  doesn't exist on main. Next session: sync Jira, push, open PR (base branch is HOLODEX-292's, not
  main — merge order matters).
