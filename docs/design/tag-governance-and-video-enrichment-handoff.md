# Design Handoff: Tag Governance & Video Enrichment (F50)

**Spec**: [tag-governance-and-video-enrichment.md](../specs/tag-governance-and-video-enrichment.md)
**ADR**: [ADR-075](../architecture/ADR-075-tag-governance-and-video-enrichment.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — tokens only, QA all three skins.
**Issue**: [HOLODEX-224](https://whoiskevinrich.atlassian.net/browse/HOLODEX-224) · **Surfaces**:
`media/[id]/+page.svelte` (tag chips), `tags/+page.svelte` (hierarchy), `tags/[id]/+page.svelte`
+ `EntityVideos.svelte` (ancestor breadcrumb), a new `owner/tags/+page.svelte` (deny-list).

---

## Overview

F50 gives owners three new touch points on top of F43's existing tag identity spine
(`resolveOrCreateByName`, merge/alias, near-miss review): manual add/remove on a video, a global
deny-list, and a strict one-parent hierarchy. This doc resolves the three placements the spec
left open for design (§Open Questions Q1–Q2, §P1-1/P1-2/P1-3) and specifies exact states/tokens
for the one genuinely new interactive surface (P0-8, the media-page chips).

### Design-system fit (the `/design-system` check)

No new components, no new tokens. Every surface below is an **extension of an existing,
shipped pattern**, chosen specifically to avoid a fourth bespoke tag-editing UI in this
codebase:

| New surface | Reuses the shape of |
|---|---|
| Media-page tag chips (add/remove) | `CurationChip.svelte`'s pill + hover-reveal remove icon + `·provenance` suffix idiom |
| Add-tag input + near-miss nudge | `/tags` page's own inline actionInput + `actionNearMiss` card (lines 366–421, 332–362) |
| Deny-list management | `/owner/duplicates`'s grouped-section shell + the Owner tab shell |
| Hierarchy set/clear parent | `/tags` page's existing per-pill ⋯ menu (Rename / Add alias / Merge into…) — one more entry |
| Ancestor breadcrumb | `EntityVideos.svelte`'s existing `hero`-snippet seam |

---

## 1. Media-page tag chips — add/remove (P0-8)

**Today** (`media/[id]/+page.svelte:406-417`): tags render only when `video.tags?.length`, as
read-only `<a href="/tags/{id}">` links — `rounded-theme bg-surface-2 px-2.5 py-1 text-sm
text-ink hover:text-accent`.

**Change**: for owners, replace with a removable-chip row + an add-tag affordance. Visitors keep
the exact current read-only markup unchanged — only the owner-gated branch is new.

### States

| State | Markup / classes |
|---|---|
| Section visibility (owner) | Section **always renders** for an owner, even with zero tags — the guard becomes `{#if isOwner \|\| video.tags?.length}`. Visitors keep today's "hide when empty" behavior. |
| Chip, default | `group relative inline-flex items-center gap-1 rounded-full border border-rule bg-surface-2 px-2.5 py-1 text-sm text-ink` — pill shape (not `rounded-theme`) to match `/tags`' own pill idiom and `CurationChip`, since this is now an editable chip, not a plain link tile. |
| Chip name | Still a link to `/tags/{id}` (`hover:text-accent`), nested inside the chip so click-to-browse survives the edit affordance. |
| Provenance suffix | Reuse `CurationChip`'s `·{label}` suffix exactly, **shown only when `source !== 'manual'`**: `·file` in `text-muted text-[0.65rem]`, `·{provider}` (e.g. `·tmdb`) in `text-accent text-[0.65rem]`. Manual tags (the owner's own input) get no suffix — see "Provenance decision" below. |
| Remove control (owner only) | `×` icon button, `rounded p-0.5 -m-0.5 text-muted hover:text-accent focus-visible:text-accent`, revealed via the existing `.curation-actions` hover/focus-within class from `app.css` (same reveal-on-hover mechanism `CurationChip` already uses — do not invent a second one). `aria-label="Remove tag {name}"`. |
| Add trigger | `+ Add tag` button, `.btn-quiet` (borderless neutral — revealing an input is a UI-only toggle, not a mutation, per `frontend-theming.md`'s button-treatment rule). |
| Add input (revealed) | `rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none` — identical classes to `/tags`' own `actionInput` (line 373), for visual consistency between the two places an owner types a tag name. Typeahead against existing tag names (reuse whatever `EntityPicker`/`api.listTags` search already backs `/tags`'s own lookups — no new search endpoint). |
| Denied-term submission | Inline rejection message, `text-sm text-warn`, e.g. `"'{term}' is on the deny-list."` — **not** a silent no-op (spec requirement, §UI). Input stays populated so the owner can see what was rejected. |
| Near-miss nudge | Reuse the `/tags` page's `actionNearMiss` card verbatim (copy: "Looks a lot like **{name}** ({count}) — use that instead?" / "Use existing" / "Add as new anyway"), same two-button layout and `bg-accent`/`border-rule` treatment (lines 332–362). Do not build a second near-miss component. |
| Busy/error | Same `disabled:opacity-60` on the submit button + `text-sm text-warn` error line pattern already used everywhere else on this page (writeback button, rename form). |

### Provenance decision (resolves a spec gap)

The spec adds a `source` column (`file` / `manual` / `provider:<name>`) but never specifies
whether the media-page chip should *show* it (flagged by exploration as an open gap). Decision:
**show it, using the existing `·{label}` suffix from `CurationChip`, suppressed for `manual`.**
Rationale: an owner looking at a video's tag row needs to know "did TMDB add this, or did I" —
that's exactly what F22's `ProvenanceBadge` already exists to answer for other fields, and
`CurationChip`'s suffix is the density-appropriate version of the same idiom for a chip *list*
(vs. `ProvenanceBadge`'s one-per-field icon). Manual is unmarked because it is the "I did this"
baseline an owner doesn't need flagged back to themselves — mirrors `ProvenanceBadge`'s own
stated principle that the unmarked baseline stays quiet ("never `--warn`," here: never louder
than plain text).

---

## 2. Deny-list management (P1-1, resolves spec Open Question Q2)

**Decision: a new Owner tab, not folded into Duplicates.** Duplicates is about identity
resolution (merge/alias — "these are the same thing"); the deny-list is about exclusion
("this string is never a tag") — different owner intent, and Duplicates' grouped-by-kind layout
has no natural row shape for "add a new blocked term." Add one entry to the tab array in
`owner/+layout.svelte:13-21`:

```js
{ href: '/owner/tags', label: 'Deny-list' }
```

Placed after `Duplicates` (adjacent tag-governance concerns stay adjacent in the tab order).

### Page shape (`owner/tags/+page.svelte`, new)

Mirror `owner/duplicates/+page.svelte`'s shell exactly: one `rounded-theme border border-rule
bg-surface` section, `text-xs uppercase tracking-wide text-muted` heading ("Denied terms · N"),
list rows below.

| Element | Spec |
|---|---|
| Intro copy | `text-sm text-muted`, e.g. "Blocks a term from becoming a tag, from any source — exact match, case-insensitive. Denying **Gnome** does not block **Garden Gnome**." (states the exact-match behavior up front, per `frontend-theming.md`'s "don't assume" spirit — this is the one non-obvious rule an owner needs before typing.) |
| Add form | Text input (same classes as the media-page add-input above) + submit button styled `border-warn text-warn hover:bg-warn/10` (matches this page's existing destructive/blocking-action treatment, e.g. the media page's Trash button) — labeled **"Deny"**. Denying is a blocking action, not an affirmative one, so it does **not** get `.btn-accent`. |
| Row | `term` text (`text-sm text-ink`) + `Remove` button (`.btn-ghost` — an immediate, reversible resolve) right-aligned. |
| Existing-tag caveat | If the owner denies a term that is already a live tag name, show a one-line inline note under that row on next add attempt: "Existing tags with this name aren't removed — this only blocks new/re-materialized tags." (Denying is forward-only per the spec; don't let the UI imply a retroactive purge.) |
| Empty state | `py-16 text-center text-sm text-muted`, "No denied terms yet." — matches Duplicates' and `/tags`' own empty-state copy pattern exactly. |

---

## 3. Hierarchy curation (P1-2, P1-3)

### Set/clear parent — one more `/tags` pill-menu entry (P1-2)

`tags/+page.svelte`'s per-pill ⋯ menu (lines 303–331) already has Rename / Add alias / Merge
into… as `role="menuitem"` buttons. Add a fourth: **"Set parent…"** (or **"Change parent…"** /
**"Clear parent"** when one is already set — swap label, don't add a second menu item).

- Opens the same inline-editor slot the rename/alias flow already uses (`menuAction` gains a
  `'parent'` variant) — a text input with typeahead against existing tag names, identical
  classes to the existing `actionInput` (line 373).
- If a parent is currently set, show it above the input: `Parent: {name}` in `text-sm text-ink`
  with an inline `Clear` button (`.btn-quiet`).
- Cycle rejection: reuse the exact `actionError`/`text-sm text-warn` slot already wired to this
  menu (line 419) — e.g. "Can't set **German Shepherd** as its own ancestor." No new error
  surface needed; this is a straight passthrough of the D1(b) server-side guard from ADR-075.
- No new component. This is additive to the existing menu's `menuAction` union type and its
  existing form.

### Ancestor breadcrumb (P1-3 — optional per spec; scoping it in for `/tags/{id}` only)

**Decision: show it on the `/tags/{id}` video-list page, skip it on media-page chips and the
`/tags` list pills.** The spec marks this optional; the media-page chip row is already the
densest surface in this handoff (name + provenance suffix + remove control), and a breadcrumb
prefix per chip would crowd it without adding information the owner can't already get by
clicking through. `/tags/{id}` has room and is where "where does this sit in the tree" is
actually the question being asked.

Implementation seam: `EntityVideos.svelte` (`web/src/lib/components/entity/EntityVideos.svelte`)
already has a `hero` snippet slot that "REPLACES the default title+count block" (its own header
comment, line 12-15) — exactly the seam the person page uses for its banner. Give
`tags/[id]/+page.svelte` a `hero` snippet that renders:

```
{#if ancestors.length}
  <p class="text-sm text-muted">{ancestors.join(' › ')} ›</p>
{/if}
<h1 class="skin-title text-2xl font-semibold text-ink">{tag.name}</h1>
<p class="text-sm text-muted">{videoCount(videos.length)}</p>
```

`ancestors` comes from `api.getTag(id)` — the response already needs to carry the ancestor
chain for the writeback ancestor-chain feature (RD-hierarchy in the spec), so this is read-only
reuse of data the API already returns, not a new fetch.

---

## Accessibility notes (all three surfaces)

- Remove/× buttons: `aria-label="Remove tag {name}"`, focus-visible reveal (not hover-only) —
  already guaranteed by reusing `.curation-actions`, which is `:hover, :focus-within` per its
  existing definition.
- Deny-list add/remove buttons: native `<button>` + `<form onsubmit>`, keyboard-operable by
  default — no custom handling needed, matching every other form on `/tags` and `/owner`.
- "Set parent…" menu item: inherits the existing menu's `role="menuitem"` + roving behavior
  already implemented for Rename/Add alias — no new keyboard pattern to design.
- Ancestor breadcrumb: plain text, no interactive elements — no additional a11y surface.

## QA

Companion checklist: [tag-governance-and-video-enrichment-qa-checklist.md](tag-governance-and-video-enrichment-qa-checklist.md).
Tokens only, per `.claude/rules/frontend-theming.md`; QA Cinémathèque, Broadcast, and Brutalist
for every new state above — the `·provenance` accent suffix and the `border-warn` Deny button
are the two spots most likely to break contrast in an unfamiliar skin.
