# QA Checklist: People on the unified source-of-truth model (F37)

**Spec**: [people-source-of-truth.md](../specs/people-source-of-truth.md) ·
**Handoff**: [people-source-of-truth-handoff.md](people-source-of-truth-handoff.md) ·
**Jira**: HOLODEX-10

Conventions: every item is numbered `section.item` and tagged by verifier —
`[smoke]` automated tests, `[agent]` agent-driven live QA, `[human]` needs human eyes.

---

## §1 Setup

- **1.1** `[agent]` Start the `backend-films` preview stack + `provider-tmdb` sidecar (see
  [[reference-holodex-preview-testbeds]]); enrich at least one person from TMDB (e.g. via a film's
  cast) so bio/born/website carry provider values.
- **1.2** `[agent]` Ensure one person has **no** provider match (scan-only) and one person has an
  F23 alias, so the empty-candidates and two-alias-systems rows are exercisable.

## §2 Smoke (run in `make test` / `npm run test`)

- **2.1** `[smoke]` `personBaseline`: `name` resolves from the record; every other canonical person
  field resolves an empty baseline; person fields flow through `ResolveFields` with the resolver
  core unmodified (build fails if `Resolve`'s video path changed).
- **2.2** `[smoke]` RD6 additivity: a person with enrichment and zero decisions resolves every field
  to the same values the raw enrichment list showed (snapshot equality).
- **2.3** `[smoke]` Decision short-circuit: `record` (blank-pin), `provider:tmdb`, and `manual`
  decisions each override the default for a person field; re-enrich flows through a provider pin.
- **2.4** `[smoke]` `PUT /people/{id}/fields/name/decision` → 400; unknown field → 404; unmatched
  provider → 400; visitor (no owner session) → 401/403 on all new endpoints.
- **2.5** `[smoke]` Rename: transaction renames + inserts the old name as an alias + FTS matches the
  old name afterwards; rename to another person's name → 409 with that person's id/name/video count
  and **no** mutation; rename to one of the person's own aliases is idempotent-safe.
- **2.6** `[smoke]` Merge cleanup (RD5): merging drops the duplicate's `field_source_decisions` and
  `metadata_curation` rows in the same transaction; canonical rows untouched.
- **2.7** `[smoke]` Payload: person `resolved[]` fields carry **no** `in_sync` key; `enriched[]` is
  gone from `GET /people/{id}`.
- **2.8** `[smoke]` `sourceChips`/`selectedChipKey` with `baselineKey='record'`: record chip
  anchored first, provider-equal-to-record folds to `·record + tmdb`, blank-pin selects the record
  chip; media-page behavior (`baselineKey='file'` default) unchanged (existing 60 tests still green).

## §3 Agent live QA (preview tools against §1 stack)

- **3.1** `[agent]` Enriched person: bio renders as full prose with the chip radiogroup beneath;
  born/nationality/website rows show `[— ·record]` idle + provider chip selected (RD6).
- **3.2** `[agent]` Blank-pin: select `— ·record` on bio → `PUT …/decision {source:"record"}` 2xx,
  field shows `—` after refetch; clear (re-select provider…) restores.
- **3.3** `[agent]` Custom on a replace field → inline input → commit → `·manual` chip selected and
  frozen across a re-enrich.
- **3.4** `[agent]` Name: selecting the tmdb chip opens the confirm dialog and fires **no** decision
  call; Cancel/Escape returns focus to the chip; Confirm → `POST …/rename` → record chip carries the
  new name and the F23 section lists the old name.
- **3.5** `[agent]` Name collision: rename to an existing person's name → dialog swaps to the merge
  offer; "Keep separate" closes with no mutation; the merge path lands in the existing F23 confirm.
- **3.6** `[agent]` Also-known-as merge row: ✕ suppresses a provider alias (survives refetch),
  `+ Add` adds a manual one; the F23 aliases card below is unaffected by either.
- **3.7** `[agent]` No-match person: replace rows show only `[— ·record] [＋ Custom]`; no provider
  chips; page renders without the Enrichment-era empty states regressing.
- **3.8** `[agent]` Absences: no "Write decisions to file" button, no out-of-sync pill anywhere on
  the page; zero `/writeback` network calls during all of §3.
- **3.9** `[agent]` Visitor view (Admin Mode off): read-only resolved values; no radiogroups,
  dialogs, or curation controls.
- **3.10** `[agent]` Keyboard: Tab lands on the selected chip, arrows rove + debounce to one PUT,
  Space/Enter activates; dialog traps focus and Escape exits.

## §4 Human (3-skin eyeball — Cinémathèque, Broadcast, Brutalist)

Open a TMDB-enriched person page (e.g. People → any cast member of an enriched film), switch skins
via the header picker, and check:

- **4.1** `[human]` The chip rows read as one system with the media page: same pill shape, the
  selected chip's dot + border stand out in every skin (accent gold / cyan / lime — reference:
  `border-accent` + accent-filled dot), idle chips stay quiet (muted text on `bg-surface-2`).
- **4.2** `[human]` `·record` provenance reads muted (grey-ish), **not** accent-colored, on every
  chip it appears on — including a folded `·record + tmdb` chip (the provider name doesn't drag the
  whole suffix to accent).
- **4.3** `[human]` The bio prose is comfortably readable above its chip row in all skins (no chip
  row crowding the text; wraps cleanly at narrow widths).
- **4.4** `[human]` The rename dialog looks native to each skin (card surface, rule border, themed
  radius; accent Rename button readable — check Brutalist's lime-on-dark) and nothing in it reads as
  an error (no red/warn tones).
- **4.5** `[human]` "Also known as" (chips, inside Details) and the "Aliases" routing card (below,
  with its add input) read as clearly different things at a glance — if they look like duplicates,
  flag it.
- **4.6** `[human]` Fonts load offline and the loading/empty/error states of the person page are
  themed in all three skins (standard theming sweep).
