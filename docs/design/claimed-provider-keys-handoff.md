# Design handoff: Claimed provider keys — the claim action (F49)

**Spec**: [claimed-provider-keys.md](../specs/claimed-provider-keys.md) (F49, FR5 + FR8) ·
**ADR**: [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) ·
**Ticket**: [HOLODEX-218](https://whoiskevinrich.atlassian.net/browse/HOLODEX-218)

This is an **addendum** to the [F44 promote/override handoff](promote-override-fields-handoff.md) and the
[F39 provider render-hints handoff](provider-render-hints-handoff.md). The *Additional details* divider, the
auto-registered row shape, `ProvenanceBadge`, the inline accent-bordered expander, and the `dt`/`dd` grid are
**inherited unchanged**. This document specifies only what is new: a **second affordance** on an
auto-registered row, the editor it opens, and the **owner-tooling list** where attachments can be reviewed and
removed later (§3). One F44 layout detail is **amended** rather than inherited — see DD7.

Everything is **tokens-only** — no literal palette, radius, or font (see [theming.md](theming.md) and
`.claude/rules/frontend-theming.md`). All three skins are a required gate.

Scope is **slice B** ([spec §12](../specs/claimed-provider-keys.md#12-phasing)). Slice A (the resolver
mechanism) has shipped and needs nothing from this document.

---

## 1. What the owner is actually deciding

An auto-registered row is a provider key nobody has classified yet. There are exactly three answers, and the
[operator docs](../reference/canonical-fields.md#claiming-a-provider-key) already state them as one table:

| The key is… | Gesture | Result |
|---|---|---|
| the same thing as a field you already have | **Attach** (F49) | one row; the key becomes a candidate of that field |
| its own thing, deserving a row and curation | **Promote** (F44) | a new first-class curatable field |
| its own thing, fine as read-only | nothing | it stays a display-only row |

The two gestures are peers, so **both pills render on the row**. A single control that hides one behind a menu
would add a click to a shipped flow and would misrepresent the choice as sequential when it is a fork.

---

## 2. Design decisions

### DD1 — The verb is "Attach to…", not "Claim"

*Claim* is the word in the spec, the ADR, the table name, and the API path. It is the wrong word in the UI:
it describes what the **canonical field** does to the key, and the owner is looking at the key. "Attach to…"
describes the owner's own gesture, and the ellipsis promises the picker that follows.

**"Merge" is forbidden here.** Holodex already uses merge for entity identity (people, studios, tags), where
it means *two records become one record, permanently*. A claim is reversible config about field identity.
Reusing the word would collide with the most destructive operation in the product.

### DD2 — The picker lists **every** canonical field for the entity type, not the ones currently on screen

This settles [spec Q3](../specs/claimed-provider-keys.md#10-open-questions), and it is not the cosmetic choice
the question implies. **Undecided empty fields are dropped from `resolved[]`**
([`resolver.go:286`](../../internal/resolver/resolver.go)), so a picker built from what the page renders is
missing exactly the fields the owner needs most.

The failing case is the common one. A person page shows an auto-registered **Biography text** row from provB.
Canonical `bio` is empty for that person — which is *why* the provider's own key is the only biography on the
page. `bio` is therefore absent from `resolved[]`, so a screen-derived picker cannot offer it. The picker would
look correct on entities that don't need it and silently omit the target on entities that do.

**This requires a small endpoint** — the client has no other way to know the entity type's field set:

```
GET /admin/field-targets/{entity_type}   →  [{ canonical, label, merge }]
```

Owner-gated, alongside the claims API. It must return the **effective** set (post-`mergePromotions`), because
a claim may target a promoted field ([spec §6.2](../specs/claimed-provider-keys.md#62-a-db-claim-adds-a-source-it-does-not-merely-suppress)).
`merge` feeds DD4. `label` is what the picker shows; `canonical` is what the `PUT` sends.

**A `<select>` is sufficient — no typeahead.** Person is 7 fields, studio 5, video is whatever the operator
mapped (the shipped example maps 17 video-scoped entries). At that size a native select is faster, is already
the idiom in `PromoteFieldEditor`, and inherits three-skin styling from `inputClass` for free.

This is also the guard that makes FR4's *422 — target is not a field of this entity type* unreachable from the
UI. That validation stays server-side, but the client should never be able to provoke it.

### DD3 — A row with two providers needs a provider checklist

One auto-registered row can carry several providers: `AutoRegisterFields` accumulates per `(provider, key)` but
renders **per key**, unioning values. Meanwhile a claim is stored per `(entity_type, provider, field_key)`
(ADR-074 §D1). So the row is one thing and the claim is N things, and the UI has to bridge that.

Attaching all providers on the row is right for the duplicate-paragraph case and **wrong for
[spec §6.5 S2](../specs/claimed-provider-keys.md#65-scenario-cookbook)**, where `provA:rating` is an age
certificate and `provB:rating` is a 1–10 score. Those two are already merged into one misleading row today —
attaching both would bury the mistake instead of surfacing it.

So: **one provider → no checklist**, straight to the picker. **Two or more → a checklist, all checked by
default**, each option showing that provider's value, with a `--warn` note that a shared key is not proof of a
shared meaning. Unchecking one leaves it auto-registering on its own, which is exactly the S2 outcome.

### DD4 — The editor states the outcome before the owner commits

Claims append at **lowest** precedence (ADR-074 §D3), so on a replace field the attached value usually
**disappears from view**. An owner who just clicked *Attach* and watched their text vanish has been misled by
their own gesture. The editor must say which of two things will happen:

- **Replace field** — "*Overview* keeps showing tmdb's value. This text moves behind the source chip — pick it
  there to make it win." Names the existing F36 control, so the sentence is also the instruction.
- **Merge field** (`multi`/`merge`) — "*Genres* merges its sources, so these values join the list right away."

The `merge` flag from DD2's endpoint decides which sentence renders. This is the difference
[spec §6.5 S4](../specs/claimed-provider-keys.md#65-scenario-cookbook) calls out as *worth knowing before you
claim*; the editor is where knowing it matters.

### DD5 — Success leaves a confirmation in the row's place, with Undo

A successful claim **deletes the row the owner was looking at** and moves its value somewhere else on the page,
possibly below the fold. A vanishing row with no acknowledgement reads as a bug.

So the row is replaced in-flow by a dashed-border confirmation strip: *"synopsis" and "comments" attached to
Overview.* with an **Undo** button, held for the rest of the page session (cleared on reload). Undo issues the
`DELETE` for every claim the action wrote.

This covers *"I just did that and it was wrong"*, which is nearly all undo demand. It does **not** cover
*"I did that last month"* — see DD8.

### DD6 — The RD3 promotion clear is named before it happens, in `--warn`

Attaching a key that currently holds an F44 promotion destroys that promotion (spec RD3, ADR-074 §D5). The
editor states it as a consequence of the button the owner is about to press — *"synopsis" is promoted as
**Synopsis**. Attaching removes that promotion.* — in `text-warn`, above the action row.

The **Attach** button is **not** disabled and the copy is not a confirm dialog. The owner is told what will be
destroyed while they can still decline, which is the standard this repo already applies to destructive-adjacent
owner actions.

### DD7 — On `long_text` and `chips` rows, the controls get their own trailing line — **accepted**

Inherited from F44 and worth fixing while a second pill lands. Today the owner pills sit **inline after the
value**, which for a `long_text` row puts them after an entire paragraph — far from the label they act on. With
two pills the drift doubles.

**Accepted 2026-07-28.** This **amends F44's shipped layout** (a cross-reference note is in the
[promote handoff](promote-override-fields-handoff.md#5-states) itself), so it is no longer separable — it ships
with slice B.

For rows that already span both columns (`long_text`, `chips` — `sm:col-span-2`):

- `ProvenanceBadge` and both pills move to a **right-aligned line under the value**:
  `mt-1 flex flex-wrap items-center justify-end gap-2` on a `sm:col-span-2` row. Wrapping is deliberate — on a
  narrow viewport the badge and pills stack rather than compress.
- The **inline** branch (`text`, `chips` rendered short, `image_url`, `url`) is **unchanged**: badge and pills
  stay inline after the value, exactly as F44 ships them. One layout branch, keyed off the same
  `display === 'long_text' || display === 'chips'` condition `AutoFieldRows` already tests for `sm:col-span-2`
  — no new predicate.
- The editor still unfolds **below** that line, so the visual order is value → controls → editor: the control
  is adjacent to what it opens.
- Right alignment, not left: at the end of a paragraph a left-aligned control reads as a continuation of the
  prose. Pushed to the trailing edge it reads as chrome.

QA note: this changes a **shipped** F44 surface, so the promote pill must be re-verified on `long_text` rows in
all three skins, not just the new Attach pill.

### DD8 — Claims are listed in owner tooling — **accepted into slice B**

Spec P1.1 put *"claims listed in owner tooling"* at P1, and
[feature acceptance](../specs/claimed-provider-keys.md#11-acceptance-feature-level) requires *"the owner can
claim a key in-app on all three entity types, **and undo it**."* Those two did not reconcile, because a claim is
**invisible by construction** — it succeeds by removing the row that was its only evidence.

After DD5's confirmation is gone, a claim made last month has no surface at all: not on the page (the row is
suppressed), not in the F44 promotions list (different table), and not in YAML for person or studio (no such
file). P1.2's unclaim-from-the-source-chip gesture is the elegant answer but is also deferred.

**Accepted 2026-07-28: P1.1 is promoted to P0 and ships in slice B** as
[spec FR8](../specs/claimed-provider-keys.md#must-have-p0). The surface is specified in **§3** below. It reuses
the `GET` the API already has and adds no backend work beyond mounting a page.

The reason it is worth the page: DD5's Undo covers *"I just did that and it was wrong"*, which is nearly all
undo demand but not all of it. Without §3, a type-global config edit ships with no durable way to see or reverse
it — the kind of gap that surfaces months later to the person least able to explain it.

### DD9 — The list is the only place a **dangling** claim can be seen, so it must show them as inert

ADR-074 §D4 makes a claim whose target canonical no longer exists **inert and never pruned**: it suppresses
nothing and the key auto-registers again, exactly as pre-F49. That is the right engine behaviour — the black
hole is structurally impossible rather than policed — but it means a stale row sits in the store doing nothing,
and §3 is the **only** surface where it exists at all.

So the list marks it. A claim whose target is absent from
`GET /admin/field-targets/{entity_type}` renders with an **Inactive** marker in `text-warn` and the copy *"target
field no longer exists — this claim does nothing."* The cross-check is client-side against the DD2 endpoint the
page already loads, so this costs **no backend change** and no extra request.

Do not hide, sort down, or auto-remove them. An inert claim is not an error — it is a claim waiting for a
mapping that may come back — and the owner is the only one who can tell which.

---

## 3. The *Attached keys* list — `/owner/fields` (DD8)

A seventh tab in the [owner tooling hub](owner-tooling-hub-handoff.md), after *Metadata keys*, which it sits
next to for a reason: that tab lists the raw container tags an operator maps in YAML, and this one lists the
provider keys they attached in-app. Same subject, two halves.

**Label: "Attached keys"**, not "Field claims". *Claim* is the word in the spec, the ADR, the table and the API
path, and DD1 already ruled it out of owner-facing copy. The verb on the pill and the noun in the nav must be
the same word or the owner has to learn two.

### Layout

The hub `+layout.svelte` renders the `Owner` heading and tab row; the page contributes an intro paragraph and
the list, following the newer `/owner/duplicates` shape (**not** `/owner/keys`, which predates the hub and
carries its own redundant `<h1>`).

Intro copy:

> Provider keys you've attached to a Holodex field. An attached key stops showing as its own row — its value
> becomes a candidate of the field instead. Removing an attachment brings the row back.

One `<section class="rounded-theme border border-rule bg-surface">` per entity type in the order **Video ·
People · Studios**, each with the `/owner/duplicates` group heading idiom
(`px-3 pb-2 pt-3 text-xs uppercase tracking-wide text-muted`) reading `Video · 3`. **Entity types with no
claims are omitted entirely** — an empty section is noise, and the section heading is not a place to teach that
video claims exist.

### The row

A claim is `(provider, field_key) → canonical`, so the row is one sentence read left to right, with the action
at the trailing edge:

| Cell | Content | Classes |
|---|---|---|
| Key | `provider:field_key` | `font-mono text-ink` — matches the `/owner/keys` key column, which is the same kind of object |
| Arrow | `→` | `text-muted`, `aria-hidden="true"` |
| Target | the field's **label**, with `canonical` in mono after it | `text-ink` + `font-mono text-muted text-xs` |
| Status | *Inactive* marker, DD9 only | `text-warn` |
| Remove | trailing button | `.btn-ghost` |

Sorted `(provider, field_key)` — the same order claims append in (ADR-074 §D3), so the list reads in the order
the engine applies them.

### Remove

**One click, no modal confirm** — matching F44's Remove-promotion precedent, and honest: removing an attachment
restores a row, it does not destroy data.

Two things the copy must not overstate. Removing an attachment **does not** restore an F44 promotion that
attaching cleared (ADR-074 §D5 — the clear is a real delete, and the owner was warned by DD6 before it
happened), and the row returns on the **next load** of an affected entity page, not instantly on some other tab.
The row's confirmation is simply its own disappearance from the list; there is no strip here, because unlike
DD5 nothing the owner was reading has moved.

### States

| State | Behaviour |
|---|---|
| Loading | `<p class="py-16 text-center text-sm text-muted">Loading…</p>` — the hub's shared idiom |
| Error | `text-warn` with `role="alert"`, as `/owner/duplicates` does |
| Empty (no claims at all) | *"No attached keys yet. Attach one from a provider row on any video, person or studio page."* — the empty state names where the gesture lives, since it cannot be started here |
| Remove in flight | That row at `opacity-60`, its button disabled. **Never dim the `text-muted` cells on their own** (theming rule) |
| Remove failed | Row stays, message in `text-warn` above the list, `aria-live="polite"` |
| Not owner | Unreachable — the hub layout already gates and redirects; the page adds no gate of its own (ADR-030: the server is the sole authority) |

### Loading it

Three parallel `GET /admin/field-claims/{entity_type}` (video, person, studio) plus the three
`GET /admin/field-targets/{entity_type}` the DD9 cross-check needs — six requests, all cached-cheap keyed
lookups, issued together with `Promise.all`. Partial failure fails the page rather than rendering a list that
silently omits an entity type; a list that lies about what is attached is worse than no list.

## 4. Components

| Component | Status | Notes |
|---|---|---|
| `curation/AutoFieldRows.svelte` | modify | Add the second pill; add `claimingKey` state next to `promotingKey` (**at most one editor open across both** — opening either closes the other); DD7 layout branch |
| `routes/owner/fields/+page.svelte` | **new** | The *Attached keys* list (§3). Follows the `/owner/duplicates` page shape — intro `<p>`, one bordered section per entity type, hub-standard loading/error/empty |
| `routes/owner/+layout.svelte` | modify | One `tabs` entry: `{ href: '/owner/fields', label: 'Attached keys' }`, placed after *Metadata keys* |
| `curation/ClaimFieldEditor.svelte` | **new** | The editor. Same shell as `PromoteFieldEditor`: `mt-2 rounded-theme border border-accent bg-surface-2 p-3 sm:col-span-2`. Reuses its `inputClass` string verbatim |
| `curation/CLAUDE.md` | modify | Add the new file to the table (required by the folder rule) |
| `lib/api.ts` | modify | `listFieldClaims`, `claimField`, `unclaimField`, `listFieldTargets` — mirroring the F44 promotion calls, including the `redirect: 'manual'` session pattern every authed call needs. All four are used by both surfaces; §3 adds no new call |
| `lib/types.ts` | modify | `FieldClaim`, `FieldTarget`; reuse `PromotionEntityType` |

`ProvenanceBadge`, `ChipValueList`, `UrlValueList`, `SourceSelect` are untouched.

## 5. Tokens

No new tokens. The editor is the F44 editor's palette.

| Token | Usage |
|---|---|
| `border-accent` / `bg-surface-2` | Editor shell — marks it as the same class of object as the promote editor |
| `border-rule` → `hover:border-accent` | Attach pill, at rest and on hover. Identical to the Promote pill |
| `text-muted` | Scope caption, field labels, outcome sentence, Cancel |
| `text-ink` | Values, and the field name inside the outcome sentence |
| `text-warn` | RD3 promotion warning, the multi-provider caution, API errors, the §3 *Inactive* marker (DD9) |
| `.btn-ghost` | §3's per-row **Remove** — bordered neutral, the repo's "immediate resolve" treatment. Not `.btn-accent`: removing is not the affirmative action here |
| `bg-accent` / `text-accent-ink` | The **Attach** button — the editor's one primary action |
| `rounded-theme` | Editor, inputs, buttons. `rounded-full` on pills only (the intentional shape exception) |

## 6. States

| Element | State | Behaviour |
|---|---|---|
| Attach pill | default | `border-rule`, `text-muted`, `text-xs`, matching Promote |
| Attach pill | hover / focus-visible | `border-accent`, `text-accent` |
| Attach pill | editor open on this row | Both pills hidden (matches F44) |
| Editor | opening | Focus moves to the first control — the provider checklist if present, else the target `<select>` |
| Editor | busy | `opacity-60` + `aria-busy="true"`; buttons disabled; stays open |
| Editor | error | Message in `text-warn` with `aria-live="polite"`; editor stays open; nothing has moved |
| Editor | no target selected | **Attach stays enabled.** The select defaults to the first target, so there is no empty state to guard |
| Editor | zero targets | Cannot occur — every entity type has canonical fields. If the endpoint fails, show the error and keep the editor open |
| Confirmation strip | after success | Dashed `border-rule` on `bg-surface-2`, `text-muted` copy, accent check icon, outlined Undo |
| Confirmation strip | undo in flight | `opacity-60`; on success the strip is removed and the page refetches |
| Visitor (`isOwner=false`) | always | **No pill, no editor, no strip, no DOM difference** — byte-identical to the F39 read-only row |
| *Attached keys* page | all | See **§3 States**. Loading / error / empty follow the hub's shared idioms rather than anything invented here |

## 7. Accessibility

- The pill is a real `<button>` with `aria-label="Attach {label} to another field"` — the visible text is
  truncated by design, so the accessible name must carry the field.
- The editor is `role="form"` with the same `Escape`-to-close handler as `PromoteFieldEditor`. It is an
  **inline expander, not a dialog** (F44 DD1) — no focus trap. Focus returns to the pill that opened it.
- Focus order: pill → \[provider checkboxes] → target select → RD3 warning (not focusable) → Cancel → Attach.
- The outcome sentence (DD4) is plain text inside the form, so it is read on entry rather than announced as a
  live region — it is a precondition, not a status.
- The provider checklist is a plain `<label>`-wrapped checkbox group, matching the repo's existing form idiom.
- Success is announced via the confirmation strip carrying `aria-live="polite"`; the vanished row is otherwise
  a silent DOM removal.
- Contrast: never dim `text-muted` with `disabled:opacity` (theming rule — it lands at 2.4–2.9:1). The busy
  state dims the **container**, not muted labels on their own.

On the §3 list:

- Each entity section is a `<section>` whose heading is a real `<h2>`, so the page is navigable by heading —
  three groups is enough for that to matter.
- The `→` between key and target is decorative: `aria-hidden="true"`, and the Remove button's
  `aria-label="Remove attachment of {provider}:{key} from {target label}"` carries the whole relationship, since
  the visible button says only *Remove*.
- The DD9 *Inactive* marker is text, not colour alone — `text-warn` is the emphasis, the word is the signal.
- Removing a row is a silent DOM removal; the count in the section heading updates, and failures announce
  through the `aria-live="polite"` error line above the list.

## 8. Edge cases

- **Long provider values in the checklist** — truncate to one line with `text-ellipsis`; the full value is
  still on the row behind the editor.
- **Long target labels** — operator-set, capped at 64 by the F44 ingest sanitizer. The `<select>` handles them.
- **The last auto-registered row is claimed** — *Additional details* and its divider disappear entirely; the
  confirmation strip renders in their place so the section does not collapse to nothing mid-gesture.
- **Two rows claimed in sequence** — one strip per claimed row, in place. Do not coalesce.
- **Target field is empty for this entity** — normal and expected (DD2). The outcome sentence still applies:
  the attached value becomes a candidate; on a replace field it does not automatically win.
- **Claim fails after a partial multi-provider write** — the write is one request per provider; on any failure,
  report the error and refetch rather than assuming. Do not present a partial success as a success.
- **Slow connection** — the busy state holds the editor open. There is no optimistic removal of the row; the
  row disappears only after the refetch confirms it.
- **A claim made in YAML** — never appears in §3. The list is the DB claims store only; a `sources:` entry is
  the operator's own file and is edited there. The intro copy says *"attached to a Holodex field"* rather than
  *"all attachments"* for exactly this reason — do not tighten it into a claim of completeness.
- **A claim whose target was promoted, then unpromoted** — the DD9 inactive case, reached by a different route.
  Same treatment, no special copy.
- **Many claims** — the list is flat and ungrouped beyond entity type. If it ever needs filtering, that is a
  signal the claim is being used as a mapping editor, which is a different feature.

## 9. QA gate

Three-skin QA is required (Cinémathèque, Broadcast, Brutalist) per
`.claude/rules/frontend-theming.md`. The checklist is a separate document following the house convention —
items numbered `section.item`, tagged `[smoke]` / `[agent]` / `[human]`, sections grouped by tag, `[human]`
steps written so a stranger can run them; see
[promote-override-fields-qa-checklist.md](promote-override-fields-qa-checklist.md) as the nearest precedent.
Written alongside the implementation as
[claimed-provider-keys-qa-checklist.md](claimed-provider-keys-qa-checklist.md). The `[human]` items that need
eyes:

- The DD7 trailing control line — the two pills plus the provider badge, right-aligned under a paragraph, in
  each skin. **Includes re-verifying F44's promote pill**, whose placement this changes on a shipped surface.
- The accent editor box on each skin's `--surface-2`.
- The `--warn` RD3 line against each skin's accent — Brutalist's lime accent and hot red-orange warn are the
  pair most likely to fight.
- The §3 list in each skin: the *Attached keys* tab in the hub's active/inactive treatment, the mono key column
  against `text-ink`, and the DD9 *Inactive* marker (`--warn` again, this time on `--surface` rather than
  beside accent).

## 10. Decisions taken back from the owner

Both open items were **accepted on 2026-07-28**:

1. **DD8 — the claims list ships in slice B.** Spec P1.1 is promoted to P0 as FR8; the surface is §3.
2. **DD7 — the owner controls move to their own line** on `long_text` / `chips` rows, amending F44's shipped
   layout. No longer separable.

Nothing in this handoff is open. What remains deferred is deferred **by the spec**, not by this document:
P1.2 (unclaim from the target's source chip — the inverse gesture, where the value now lives) and P1.3
(proactive duplicate detection, slice C).
