# People attach/detach + relationship picker (F56.5, HOLODEX-272)

Parent epic: [HOLODEX-267](https://whoiskevinrich.atlassian.net/browse/HOLODEX-267) — Entity
decision editing overhaul, last open child story. Depends on
[HOLODEX-270](https://whoiskevinrich.atlassian.net/browse/HOLODEX-270) (composite-key collision
check, shipped) — People is one of the four composite-key fields {Title, People, Date, Studio},
and `internal/repo/video_collision.go`'s own comment already earmarks this story as the second of
two remaining trigger points (Studio, HOLODEX-271, shipped first).

## Problem Statement

The video page's People grid (`web/src/routes/media/[id]/+page.svelte:902-923`) has no add,
remove, or role-change affordance today — an owner who wants to correct a wrong person, add one a
provider missed, or remove one who shouldn't be linked has no path in the UI. The only way to
change who's linked is to overwrite the entire `actors` or `director` field's manual value as a
single free-text/JSON literal through the generic field-decision editor — there's no picker, no
search, and no way to change one person without re-typing the whole list. The cost of not solving
this is the same workaround pattern the other three composite-key fields had before their own
stories shipped: owners either leave a wrong or incomplete People list standing, or edit file
metadata out-of-band and force a re-scan just to get one name changed.

## Existing State (grounded in code, this session)

- **People grid today**: outer guard is bare `{#if video.people?.length}` (`+page.svelte:902`) —
  unlike Tags (`{#if isOwner || video.tags?.length}`, `:812`) and Studio, there's no `isOwner ||`
  branch, so an owner with zero people sees no section at all. Inside the `{#each}` (`:907`): a
  link + `PersonPoster` + caption, no `isOwner` check, no remove control.
- **`video_people` is fully DERIVED, not an independently mutable relation.**
  `internal/db/migrations/0037_person_link_derivation.up.sql` (F40/ADR-072) made
  `video_people(video_id, person_id, role)` — `role` ∈ `{'actor', 'director', ''}`, PK is the
  triple — a table rebuilt on every relink by `Repo.ReconcileVideoPeople`
  (`internal/repo/person_links.go:44-134`), called from `RelinkVideoPeople`
  (`internal/api/person_links.go:105-163`), which derives its rows from `resolver.Resolve(...)`
  over the *resolved* `actors`/`director` field decisions — ordinary `field_source_decisions`
  rows, same `SetDecision`/`SetDecisionChecked` mechanism Title and Studio already use. There is
  no other writer of `video_people` anywhere in the codebase today.
- **Commit path precedent — corrected during implementation.** `actors`/`director` are configured
  `multi: true` (Merge-mode) fields in `metadata-mappings.yaml`, and `internal/api/decisions.go`'s
  `replaceField` gate explicitly rejects any field where `f.Multi || f.Merge` with a 400 —
  `SetDecision`/`setFieldDecision` structurally cannot be used for these fields at all, so the
  "falls through to `SetDecision`" framing below (written before implementation) was wrong. The
  actual, already-shipped mechanism is `metadata_curation` (F30/ADR-048) via `SetCuration`/
  `ClearCuration` (`action=add`/`suppress`/`nowrite`) — confirmed by `internal/api/curation.go`'s
  own comment ("Also how the owner-view link picker attaches a person/studio to a video — a link
  IS a curation add (ADR-072 RD1)") and by ADR-072 Action Item 9 ("Owner-view link picker (curation
  add) — reuses the F30 curation endpoint; no new API"). `CurationFieldRow.svelte` already
  implements this attach/detach mechanism live in production for `actors`/`director` today via the
  generic Metadata-fields curation row — this story's actual gap is narrower than originally
  scoped: `POST /media/{id}/curation` had no composite-key collision check, unlike Title/Studio's
  `setFieldDecision`.
- **No `FindPeopleCollision` exists.** `FindTitleCollision`/`FindStudioCollision`
  (`internal/repo/video_collision.go`) share `compositeKeyCandidates`/`hydrateCollision`/
  `recordedAtOf`/`linkedIDKey`/`linkedNameKey`/`normalizedNameKey` helpers and both currently treat
  People as a *fixed* input (read from the video's current `video_people` rows) when checking
  whether a Title or Studio change collides. A People-triggered check inverts this: hold
  Title/Studio/Date fixed, vary the *proposed person-id-set* — the genuinely new piece, not a
  copy-paste of an existing one.
- **No reusable multi-select picker exists.** `StudioPicker.svelte` (HOLODEX-271, 299 lines) is
  the closest precedent — docked-pencil affordance, `PickerShell` popover, known-candidate chips,
  300ms-debounced search against `api.search(q)`, create-fallback, caller-injected `decide()` with
  a conflict/verdict slot — but it's single-select, built for one Studio value.
  `EntityPickerDialog.svelte` (247 lines, used by the Extraction tab's suggestion picker) has the
  same debounce/search/create-fallback pattern but is also single-select
  (`onselect: (name, existing) => void`, fires once then closes). Neither models a set of
  already-attached items with independent per-item removal. `EntityPicker.svelte` is a *merge*
  picker (`api.mergeEntities`, destroys the source entity) and is the wrong tool entirely for a
  non-destructive link. `PickerShell.svelte` (100 lines) is generic, entity-agnostic dialog chrome
  and is reusable as-is regardless of how the People-specific body is built.
- **`PersonPoster.svelte`** (21 lines) has no hover-overlay markup or slot for one — same as Tags,
  a remove-`×` has to live in the call site's markup, not inside the component.

## Resolved Decisions

Two architecturally load-bearing questions were resolved via AskUserQuestion before writing
requirements, since they determine both the backend shape and whether this story needs its own
ADR (every prior HOLODEX-267 story marked `architecture` `[~]` not needed — this is the first that
might not have):

1. **Attach/detach writes through the curation model, not `video_people` directly — corrected
   during implementation.** `actors`/`director` are `multi`/`merge` fields, which
   `setFieldDecision`'s `replaceField` gate structurally refuses (400) — so "the same
   `SetDecision`/`relinkIfEntity` path Title and Studio already use," as originally written here,
   was never actually available for People. The real mechanism, already shipped and in production
   use via `CurationFieldRow.svelte`, is `metadata_curation` (F30/ADR-048): attaching a person is a
   curation `action=add` on the `actors`/`director` field, detaching is `action=suppress`, both via
   `POST /media/{id}/curation` → `relinkIfEntity`. `video_people` stays fully derived — the
   ADR-072 invariant is preserved, not bypassed — so **no new ADR is needed**; ADR-072 Action Item
   9 pre-decided this exact reuse ("reuses the F30 curation endpoint; no new API"). What *is* new
   for this story is narrower than originally scoped: `POST /media/{id}/curation` had no
   composite-key collision check on a person-typed add/suppress, unlike Title/Studio's
   `setFieldDecision` — closing that gap (`FindPeopleCollision` + a `SetCurationChecked` atomic
   check-then-write, gated by `registry.Lookup(field).EntityKind == EntityKindPerson`) is the
   entirety of the backend work this story adds. The alternative (a new endpoint writing
   `video_people` directly) remains rejected for the same reason as originally written: it would
   silently break under the next unrelated field edit, since `ReconcileVideoPeople`
   unconditionally rebuilds the table from the resolved `actors`/`director` values on every relink.
2. **The picker requires a role choice (Actor or Director) at attach time.** `video_people`'s PK
   is `(video_id, person_id, role)`, and the resolver already treats `actors` and `director` as
   distinct typed fields with distinct provider sourcing — defaulting every add to a generic
   unlabeled role would let this list quietly disagree with what the rest of the resolver
   considers meaningful. The attach flow costs one extra click (pick a role) in exchange for
   staying honest about which of the two underlying fields is actually being edited.

## Goals

- An owner can add a person to a video's People list — via a known candidate, a full-library
  search, or an inline create-fallback — without leaving the video detail page, mirroring the
  Tags/Studio editing experience already shipped for the other three composite-key fields.
- An owner can remove a person from the People list with the same one-click hover-reveal
  interaction already established for Tags.
- Every attach/detach that changes the video's composite key runs through the same collision gate
  Title and Studio already use, so two videos can't silently collapse to the same {Title, People,
  Date, Studio} identity via a People edit specifically.
- The People section is visible (with an add affordance) to an owner even when the video currently
  has zero people linked — closing the empty-state gap Tags/Studio don't have.

## Non-Goals

- Redesigning `EntityPicker`/`EntityPickerDialog` — both stay as they are for their existing call
  sites (merge flow, Extraction-tab suggestion picker); this story adds a new component rather
  than generalizing either into multi-select.
- Changing how `actors`/`director` are resolved from providers — the existing per-field
  file/provider/manual resolution model is unaffected; this story only adds a friendlier
  owner-facing way to set the manual layer.
- A bulk/multi-video People editor — this is scoped to one video's detail page, matching how
  Title/Studio renames were scoped.
- Reordering or deduplicating an existing People list beyond what attach/detach naturally does.

## User Stories

- As a video owner, I want to add a person to a video's People list by picking from known
  candidates (already suggested by the file or a provider) so that correcting or completing the
  list doesn't require typing a full name from scratch.
- As a video owner, I want to search the full person library and pick an existing person when
  they're not already a candidate, so that I'm not limited to what a provider or the file metadata
  already surfaced.
- As a video owner, I want to create a new person inline when the one I want doesn't exist yet, so
  that attaching them doesn't require a separate trip to create the person first.
- As a video owner, I want to remove a person from the list with one click, so that fixing a wrong
  attribution is as fast as removing a wrong tag.
- As a video owner, I want to be told when my attach/detach would make this video's composite key
  ({Title, People, Date, Studio}) match another active video, so that I can either view the
  existing video or confirm I really do want two records (e.g. two distinct encodes of the same
  session).
- As a video owner with zero people currently linked, I want to still see an "+ Add person"
  affordance, so that a video with no attribution isn't a dead end.

## Requirements

### Must-Have (P0)

- **Fix the People section's empty-state guard.** Change `{#if video.people?.length}` to
  `{#if isOwner || video.people?.length}` (`+page.svelte:902`), matching the Tags/Studio pattern,
  so an owner sees the section (with just the add affordance) even at zero people.
  - Given an owner viewing a video with zero linked people, when the page loads, then the People
    section renders with a visible "+ Add person" control and no other content.
  - Given a non-owner viewing the same video, when the page loads, then the People section renders
    nothing, same as today.
- **Hover-reveal remove control on each `PersonPoster` card**, mirroring Tags' `curation-chip`/
  `curation-actions` idiom at the call site (`PersonPoster` itself stays unchanged — no
  hover-overlay markup added to the component).
  - Given an owner hovers or focuses a linked person's card, when they activate the remove
    control, then that person is detached (subject to the collision gate below) without a page
    reload.
- **A new People picker component** (working name `PersonPicker.svelte`), composing `PickerShell`
  for dialog chrome, reusing as much of `StudioPicker`'s debounce/create-fallback/conflict-slot
  pattern as the multi-select shape allows:
  - Known-candidate chips for people the file or a provider already suggested but aren't yet
    attached.
  - Debounced full-library search (matching `StudioPicker`'s 300ms `api.search(q)` pattern).
  - Inline create-fallback ("Use "…" as a new person") when no match exists.
  - A role choice (Actor / Director) presented as part of picking or confirming a person, per the
    Resolved Decisions above — not defaulted silently.
- **Backend: attach/detach commits through the existing curation endpoint.** No new endpoint —
  attach is `POST /media/{id}/curation` with `{field: "actors"|"director", value: <name>,
  action: "add"}`; detach is the same route with `action: "suppress"`, given a person (existing
  name or a new inline-create) and a role. This is already how `CurationFieldRow.svelte` attaches
  people today; the picker is a friendlier front end over the same mechanism, not a new write path.
- **`FindPeopleCollision`**, structurally a sibling of `FindTitleCollision`/`FindStudioCollision`
  reusing `compositeKeyCandidates`/`hydrateCollision`, inverted to hold Title/Studio/Date fixed and
  vary the proposed person-*name*-set (name, not id — mirroring `FindStudioCollision`, since a
  newly-created person has no id yet at check time); wired into `setCuration`'s add/suppress path
  via a new `SetCurationChecked` (atomic check-then-write, mirroring `SetDecisionChecked`), gated
  on `registry.Lookup(field).EntityKind == EntityKindPerson` so it's field-generic rather than a
  hardcoded `actors`/`director` string list. Same 409 + `override` contract Title/Studio use.
  - Given an attach/detach would produce an exact composite-key match against another active
    video, when the owner commits, then they see the same `CollisionOfferCard` verdict panel
    (View existing video / Save anyway) already shared by Title and Studio.

### Nice-to-Have (P1)

- Visual indication in the People grid of each person's role (Actor vs. Director) at rest, not
  just during attach — a caption or badge, styled with existing tokens.
- Keyboard-only attach/detach flow parity with the existing Tags add/remove keyboard support.

### Future Considerations (P2)

- Bulk People editing across multiple videos at once (explicitly out of scope for this story; not
  designed against here so it isn't foreclosed).
- Reordering people within a role (the schema and UI here don't guarantee or expose order).

## Success Metrics

This is an internal owner-tooling feature on a personal media server with a single owner, not a
multi-user product — there's no adoption funnel to measure. Success is binary and verified by
manual QA, matching how HOLODEX-268/269/270/271/273 were each closed out: the People section
renders correctly (including the zero-people empty state) across all three skins, attach/detach
round-trips correctly through `video_people` on reload, and the collision gate fires on a real
composite-key collision reproduced against a live backend instance (the same live-QA pattern
HOLODEX-271 used for Studio).

## Open Questions

None blocking. The implementation-detail question originally left open here (new dedicated
endpoint vs. an addition to `setFieldDecision`) turned out to be moot: `actors`/`director` are
`multi`/`merge` fields that `setFieldDecision` structurally can't handle, so there was never a
choice to make — attach/detach reuses the existing `POST /media/{id}/curation` endpoint (see
Resolved Decision #1, corrected during implementation), matching `internal/api/curation.go`'s
existing title/studio-style special-casing pattern rather than `decisions.go`'s.

## Timeline Considerations

No hard deadline. This is the last open story under HOLODEX-267 — closing it out closes the epic
(pending a check of whether HOLODEX-267's own stale `needs-design`/`needs-spec` labels should be
cleared once this story's own spec/design gates land, per the epic-status review done this
session). No dependency on work outside this epic.
