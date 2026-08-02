# Design Handoff: Writeback dialog — poster comparison + enrichment/decision legibility gap

**Spec**: [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) §Writeback ·
**ADRs**: [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md) ·
[ADR-041](../architecture/ADR-041-metadata-writeback.md)
**Builds on**: [writeback-selection-handoff.md](writeback-selection-handoff.md) (HOLODEX-213) — this
doc assumes that one's decided/undecided split as ground truth and does not change it.
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Surface**: `WritebackFormDialog.svelte` (issue 1) · `SourceSelect.svelte` / `f36.ts` /
`internal/resolver/resolver.go` (issue 2, traced below). **Issue**: [HOLODEX-245](https://whoiskevinrich.atlassian.net/browse/HOLODEX-245).

---

## Overview

Two reports against the writeback modal turned out to be independent — one is a missing display
(read-only, no data change), the other is a genuine cross-surface contract gap traced to a
specific resolver function. Neither is a regression: both surfaces do exactly what their own code
comments say. They just don't agree with each other.

---

## Issue 1 — the dialog never shows the file's current poster next to the enriched candidate

**Current behavior** (`WritebackFormDialog.svelte:354-364`): an `image_url` row renders only
`row.value` — the single winning value frozen at dialog-open — as a thumbnail + URL string, fully
read-only. Every other field type gets a `was: {fileVal}` comparison line
(`WritebackFormDialog.svelte:372-374`) so the owner can see what they're overwriting before
confirming; image rows skip that branch entirely. `fileCandidateValue(row.field)` is already
fetched at line 307 for that comparison — it's just never rendered for `image_url`.

**Fix (decided: Option A — read-only comparison, no new selection model)**: render the file's
current candidate thumbnail next to the enriched one, matching the existing `was:` idiom instead
of inventing one. This stays read-only — no `SourceSelect`/`CurationChip` radiogroup — because
picking a candidate *would* be a source decision, and per `SourceSelect.svelte:16` ("Changing a
source is a DB-only decision (RD5): it calls `decide`, never a file write") that path is
deliberately walled off from the writeback action. A rejected alternative (pickable candidates,
reusing the `SourceSelect` chip idiom) was mocked and set aside for exactly that reason — it would
need its own `decide()` wiring and product sign-off on blurring RD5, which is out of scope here.

### Layout

```
[ ] Poster                                    ·tmdb
    File (current)        →      Enriched (will write)
    ┌──────────┐                 ┌──────────┐
    │  (img)   │                 │  (img)   │
    └──────────┘                 └──────────┘
```

- Two `max-h-14`-scale thumbnails (matching the dialog's existing image row sizing, not the
  page's `max-h-64`— the dialog is denser), separated by a small arrow glyph.
- **File (current)** label uses `text-muted`; its thumbnail border is `border-rule`.
- **Enriched (will write)** label uses `text-accent` (this is the value the checkbox governs);
  its thumbnail border is `border-accent`.
- When the file candidate is empty (no embedded cover art), the "File (current)" slot renders the
  existing empty-state treatment already used elsewhere for a missing image (`—` or a muted
  placeholder icon) — never a broken-image icon, consistent with the `image_url` safety-fallback
  rule in [provider-render-hints-handoff.md §4](provider-render-hints-handoff.md).
- When `matchesFile` is true (the enriched value already matches the file), skip the comparison
  entirely and use the same "already in file, nothing to write" line the text fields get
  (`WritebackFormDialog.svelte:365-370`) — no need for two identical thumbnails side by side.
- URL text below the thumbnails stays as today (`break-all text-xs text-muted`), shown once for
  the enriched value only — the file value has no accessible URL to show (it's an embedded asset).

### States

| State | Render |
|---|---|
| Enriched value differs from file, file has a candidate | Two thumbnails + arrow, as above |
| Enriched value differs from file, file has **no** candidate | File slot shows the muted empty placeholder, not a broken image |
| Enriched value **matches** file (`matchesFile`) | Single "already in file, nothing to write" line (existing pattern), no comparison |
| Non-allowlisted image URL (ADR-039 asset-host check) | Plain text fallback per provider-render-hints-handoff.md §4 — no `<img>`, no error icon |

### Accessibility

- Each thumbnail gets `alt="{field.label} — file"` / `alt="{field.label} — enriched, from {provider}"` (not the generic `alt="cover"` currently used) so a screen reader announces which is which.
- No new interactive elements — the row's only control remains the existing checkbox.

---

## Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided"

This is real, not a misperception, and traced to a specific line. It is **not** a bug in
`needsWriteback()` or in `SourceSelect` individually — both do exactly what their own code
comments specify. The gap is that two surfaces disagree on what "decided" means for one specific
case: an empty file baseline whose winner is a provider value.

### The trace

1. **Enrichment never writes a decision.** `EnrichPicker.svelte`'s `apply(provider, externalId)`
   (`EnrichPicker.svelte:14`) returns `{ enriched: EnrichedField[] }` and the caller's `onapplied`
   only refreshes the resolved-field list — it never calls `decide()`. Running enrichment
   populates the `entity_enrichment` shadow store; it does not write a `field_source_decisions`
   row (ADR-051 §2). So after enriching, **no field has a standing decision** unless the owner
   separately used `SourceSelect` to pick one.
2. **But `SourceSelect` can visually show the provider value as selected anyway.**
   `selectedChipKey()` (`f36.ts:125-145`) has a deliberate exception for this exact case
   (comment at `f36.ts:120-124`, F37 RD6): *"with an EMPTY baseline the resolver's winner is a
   provider value, so the provider chip reads selected... display identical to the raw enrichment
   list."* This is intentional — RD6 was a considered decision, not an oversight — but its effect
   is that the media page's radiogroup shows a filled/accented radio dot on the enriched value
   with **no decision behind it**.
3. **The resolver only flips `in_sync` when a real decision exists.**
   `replaceMarkers()` (`internal/resolver/resolver.go:556-574`) defaults `inSync := true` (L559)
   and only recomputes it — `inSync = decided == fileVal` (L570) — inside the `if dec != nil`
   branch (L560). With no standing decision, `in_sync` stays `true` regardless of what the
   resolver's implicit winner is.
4. **`needsWriteback()` reads `in_sync`, not the display.** `f36.ts:163-165`:
   `isReplaceField(field) && outOfSync(field)`, i.e. `field.in_sync === false`. Since step 3 left
   `in_sync` at `true`, the field is — correctly, by its own contract — classified **undecided**
   and lands unchecked behind the writeback dialog's disclosure, per HOLODEX-213.

**Net effect**: the owner enriches a video, sees the new poster/title/etc. rendered as the
selected chip on the media page (step 2), opens "Write metadata to file," and finds that exact
field sitting unchecked in the collapsed "12 provider values you haven't decided on" group. From
the owner's chair this reads as "I decided this, why didn't it select" — it's a legibility gap
between RD6's display exception and the writeback predicate's stricter, correct-by-design
definition of "decided," not a broken checkbox.

### Fix options (engineering + product should pick one — none of these is free)

| Option | What | Tradeoff |
|---|---|---|
| **1 — Reconcile the display (recommended)** | `SourceSelect` stops rendering the RD6 empty-baseline provider chip as a filled/selected radio dot; give it a distinct "would apply, not yet decided" treatment (e.g. accent outline without the filled dot + `·pending` or similar) | No data-model change, cheapest to ship. But it partially walks back F37 RD6, which was a deliberate call ("display identical to the raw enrichment list") — needs a design nod before touching it, not just an engineering call. |
| **2 — Make enrichment decide** | `EnrichPicker`'s apply flow calls `decide(providerSource)` for every field it newly wins on an empty baseline, so enriching creates the standing decision the UI already implies | Matches what the owner intuitively expects. But it blurs `SourceSelect.svelte:16`'s RD5 boundary — "changing a source is a DB-only decision... never baked into a writeback action" — enrichment isn't the writeback action, but it would now silently create the same kind of standing decision SourceSelect's explicit click does. Needs product sign-off; also raises a question this doc doesn't answer: does this apply retroactively to already-enriched fields, or only to future `apply()` calls? |
| **3 — Copy only** | Leave both predicates as-is; add a line to the writeback dialog's undecided-group explaining why an enriched-but-undecided value isn't pre-checked | Cheapest, ships today, fixes nothing structurally — the owner still has to expand + manually check every field they just enriched. |

This doc does not pick between 1 and 2 — that's a product call on whether "I ran enrichment"
should count as a decision. Recommend routing that specific question back through `/architecture`
or the F36 spec owner before implementation, since either changes a documented RD. Option 3 can
ship independently and immediately regardless of which (if either) of 1/2 is chosen later.

### Verification checklist (for whichever option ships)

- [ ] A field with a **real** standing decision (via `SourceSelect`, any baseline state) still
      pre-checks and lands in "decided" — this path was already correct; don't regress it.
- [ ] A field winning purely by **mapping precedence** with a non-empty file baseline stays
      undecided, unchecked — HOLODEX-213's original behavior, unaffected by this fix.
- [ ] The specific empty-baseline-provider-winner case (F37 RD6) is the only one that changes.
- [ ] All three skins: the new "would apply, not yet decided" treatment (if Option 1) reads at
      AA contrast — check against the existing contrast table in
      [writeback-selection-handoff.md](writeback-selection-handoff.md).
