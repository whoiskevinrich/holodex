---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-373
status: in-progress
release_note: Every person, studio, tag, film and video now carries a copyable reference like `film:42`; films can be aliased and renamed like everything else; a film's files can carry an edition (Theatrical, Final Cut) read from the file or set by you; and a name can be shown one way while the file keeps another.
---

# HOLODEX-373 · F60 — Entity identity card

Brainstorm 2026-09-12: "should every entity have a slug, canonical name, display name, aliases
and enrichment ids?" Inventory said three of the four already exist, unevenly — films are outside
the ADR-061 spine on every axis, provider ids live in two tables with two meanings, nothing has a
display name, URLs are numeric. The load-bearing case Kevin added — **editions/cuts** — was not
covered by the proposal at all and reshaped it.

**Locked (see the epic for rationale):** film = the work as the provider defines it, edition =
a property of the file; edition is an ordinary resolved video field (container tag > filename,
curation, writeback via the tag — never rename files); `kind:id` reference handle, slugs cut;
one `entity_external_ids` table; display name = curation on `name` for Person/Studio/Film only.
**Tags are always lowercase by Kevin's policy** (0034 is deliberate) — HOLODEX-379 closed Won't Do.

**Stories, in shipping order:** 374 handle (High) → 375 external-id unification (High) → 376
films into the spine → 377 edition → 378 display-as (Low; kill criterion in the handoff).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/entity-identity-card.md` (F60), RD1–RD12. Settled in the
  pass: edition writeback key is `Edition` on **both** backends (Matroska `EDITION`; MP4/MOV
  `XMP-prism:Edition` — **`QuickTime:Edition` is NOT writable**, the 09-12 "verified" claim was a
  bad `-listw` check; corrected 09-13 with a real write+read on generated samples, both read back
  as `Edition`); `Subtitle` rejected because it's already the
  tagline's key (`tags.go:101`); filename edition follows the F48 auto-apply rule, no special case;
  film alias routing needs a year match or a unique nameKey, else queue
- [x] architecture `architecture` — ADR for external-id unification + edition-as-field +
  curation-on-name; amendment notes on ADR-051 (name was the excluded field) and ADR-061
  (films, composite nameKey). Number via `node scripts/adr-claims.mjs`, never by eye
- [x] design `design-handoff` — `docs/design/entity-identity-card-handoff.md` +
  `entity-identity-card-mockup.svg` (4 panels). Two grounded changes vs the brainstorm: edition
  edits ride `SourceBadge` (badge-click), not a pencil; "Display as" is the name field's
  `SourceBadge`, "Rename in files" is the existing pencil — no second button. Found a real gap:
  `SourceBadge` only renders the badge when multi-source, so a filename-only edition would have no
  curation affordance (§2b). **OQ1** deep-link vs inline Set edition, **OQ2** the 378 go/no-go —
  both need Kevin
- [x] backend — 374 done: `internal/model/ref.go` (kinds + `Ref()` + `MarshalJSON` on the five
  entities, so `ref` rides every list/detail/nested payload from one place), `internal/api/ref.go`
  (`ParseRef` + `RefKindError`; `urlParamID` reads the route's kind off the chi pattern, nested ids
  by param name), MCP `get_video` accepts a ref + every result carries `ref`. 375 done: migration
  0046 `entity_external_ids` (fold + 4 cleanup triggers), `resolveOrCreateByName` id-first for
  every kind, `Enrich` records the adopted id as identity (person/studio/tag/film), merge repoints
  the polymorphic row, `GetFilmByExternalID`; memo column kept → HOLODEX-382. 376 done:
  migration 0047 (`ux_films_namekey` + film cleanup triggers), film in every spine registry
  (`canonicalTable`, alias-key map, merge config w/ scene-number-safe `moveAssocSQL`, review
  junction + seed), `CreateFilm` = film resolve-or-create per RD4 + `queueFilmSameTitle` (also on
  rename), year-aware `RenameEntity` collision, film identity routes under the films gate,
  `EntityRef.Year`, TMDB sidecar emits film `aliases`. 377 done: F48 lifts `{edition-X}` out of
  the stem before pattern matching (marker alone = a match), `edition` TierHigh, registry +
  `.example` row, `formatMap` `Edition` / `XMP-prism:Edition`, `videoEdition` stamps full-film
  rows through the pure resolver, `FacetScore.Curatable`. 378 done: the three `name` guards
  lifted (decision = display spelling, column untouched), studio/film `name` gains provider
  candidates (film under the sidecar's `title` key), `model.*.DisplayName`
  (`display_name,omitempty`, search rows only — pickers send `name` back), `repo.DisplayNames`
  (narrow SQL mirror of the decided-replace rule) + a display-spelling leg in `Search` for all
  three kinds
- [x] frontend — 374 done: `RefChip.svelte` + `--font-mono` token, mounted on all five pages
  (people/studios via `EntityVideoMeta`'s `ref` prop). 3-skin QA by computed style: text ≈17:1,
  glyph ≥4.9:1 on Broadcast. 376 done: `EntityKind` + `'film'` (api base, pickers, duplicates
  page/banner), film page title `NameEditControl` + `MergeOfferCard` verdict + near-miss advisory
  (studio wiring verbatim) + `AliasPanel` in the rail; `refLabel()` shows a film's year on every
  identity card. 3-skin QA'd live (verdict card + panel on all three). 377 done: film-page
  edition pill + dashed `+ Set edition` deep link, `#field-<canonical>` landing expands the
  badge (fresh load + same-page hash), a deep-linked *missing* curatable field renders as an
  empty SourceBadge row, `SourceBadge` badge always renders (RD12, `curation/CLAUDE.md`).
  3-skin QA'd live (pill 4.7–6.0:1, link 8.7–16.8:1; visitor sees pills, no link). 378 done:
  `DisplayNameLine.svelte` under the three headings ("In files as" / film "On record as" +
  `SourceBadge showValue={false}`; quiet `Display as…` link when nothing stands; focus hands
  across the link↔badge swap), `NameEditControl editValue` prefills canonical, search rows
  label `display_name ?? name`; film page now resets `expandedField` on nav. 3-skin QA'd live
  (muted 4.9–6.3:1, mono 16–19:1; visitor sees the line, no control; no overflow at 375px)
- [x] testing `testing-strategy` — 374/375/376/377/378 rows landed (`docs/testing-strategy.md`)
- [x] security `security-review` — run 2026-09-14 over the whole branch through 377: no findings
  (argv shape unchanged for the new keys, refs kind-checked off the route pattern, film mutations
  under `requireOwner`, `edition` to visitors is a resolved value not file metadata). Re-run
  2026-09-15 over the 378 diff: no findings (guards were 400 shape checks inside `requireOwner`,
  a `name` decision writes only `field_source_decisions`, search leg is bound params + Go-side
  matching, `display_name` exposes only what the resolved `name` row already shows visitors)

## Up next — ordered (position = priority)

1. [x] [—] OQ1 = deep link, OQ2 = keep 378 — ratified 2026-09-12
2. [x] [S] `/write-spec` — landed, `needs-spec` cleared
3. [x] [S] `/architecture` — ADR-096 landed, `needs-adr` cleared
4. [x] [—] Run the read-only prod probe for edition-bearing full-film titles before sizing 377 —
   a filename crawler, not SQL (the prod library isn't in a Holodex DB yet):
   `node scripts/probe-edition-filenames.mjs <media dir>` (anonymized counts; `--show N` prints
   names, keep out of pastes). Paste the counts into the next session log
5. [x] [M] 374 handle — shipped 2026-09-13 (tests: parser table, 5 entity routes + nested,
   kind mismatch 400, list/nested `ref`, MCP)
6. [x] [M] 375 external-id unification — shipped 2026-09-13 (precedence test written first)
7. [x] [M] 376 films into the spine — shipped 2026-09-14 (tests: composite-key collision,
   alias-with-year routing, ambiguous no-year → queue, rename keeps alias + queues same-title,
   cleanup trigger, provider aliases, film merge; API: routes gated, 409 conflict carries year)
7c. [x] [M] 377 edition — shipped 2026-09-14 (tests: marker lift table, tier, tag > filename,
   `Edition-und`, MKV+MP4 round trip `-tags integration`, film payload, `Curatable`)
7d. [x] [—] Kevin ruled 2026-09-14: §2c helper line stays unbuilt (A — "I plan to rework that
   system soonish"; no follow-up filed for the conditional variant). Prod mapping: editions
   live mostly on media, most have none → `edition` became a `CriticalityOptional` facet (listed,
   never scored/queued), so adding the block to prod has no score/queue side effect; the only
   remaining consideration is `EXTRACTION_AUTO_APPLY_ENABLED` (≈141 file writes if on). Kevin
   adds the block when ready
7e. [x] [L] 378 display-as — shipped 2026-09-15 (headers + search scope, Kevin's ruling from a
   side-by-side mockup). **Kill criterion QA 4.3 is still Kevin's to run** (handoff §8 4.3 /
   as-built §4): if the pencil-vs-badge split doesn't read, revert the 378 commit — the other
   four stories don't depend on it
7f. [x] [—] QA 4.3 run 2026-09-15: half-fail on the first pass (rename read; "change how it
   looks" → tmdb Refresh), pass on the re-run once the two verbs were stated in lay terms; the
   link now names the offered spelling. 378 stays; #332 marked ready
7g. [ ] [S] File the follow-up story: `display_name` on list cards / cast tiles / link cards
   (option B of the 2026-09-15 scope ruling) — only if 378 survives 4.3
7b. [ ] [S] Seed `same-title` pairs for films that pre-date 0047 (only create/rename queue them
   today) — file as a HOLODEX follow-up if Kevin wants the backfill
8. [ ] [—] On PR ready: sweep 374–378 to In Review by hand with the epic; on merge, sweep to Done
   (CI moves only the branch's key — an epic-keyed branch moves nothing)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-15 · 378 display-as — coded, tested, live-QA'd, security-reviewed
- skills: code-review (1 finding, fixed), security-review (clean)
Explore-agent trace first: `name` was already a synthesised resolved field on all three kinds and
the resolver honours a `name` decision generically — the only blocker was the 400 guard in each
handler, and every writeback / alias / MCP reader takes the canonical column. The one fork that
changed the backend shape was *where the resolved name renders*: Kevin asked for context and
options, ruled **headers + search** off a side-by-side mockup (cards/tiles/pickers stay
canonical; a `display_name`-on-cards story is a follow-up if 378 survives). Two hazards found on
the way and designed around: the pickers read `/search` and send `name` back for linking, so
search rows carry `display_name` *beside* a canonical `name` rather than replacing it; and
film's provider spelling is stored under the sidecar's `title` key, so film `name` candidates
are `<provider>:title` — the ADR-089 D3 guard test (name baseline-only) was retargeted to
assert that shape, with ADR-096 D5 recording why D3 now binds the column, not the candidate
list. Search matches the display spelling through `repo.DisplayNames`, a narrow SQL mirror of
the resolver's decided-replace rule (manual literal, or the decided provider's stored spelling;
`file` and unmatched-provider rows omitted — the resolver drops those, so the page falls back
to canonical), pinned to the resolved payload by the API test. Frontend: `DisplayNameLine`
(entity/, shared by the three pages) renders "In files as `<canonical>`" only when resolved ≠
canonical — a content line visitors see — with the name field's `SourceBadge` (`showValue`
off, the heading already shows the value) for the owner, and a quiet `Display as…` link when
nothing stands; `NameEditControl editValue` keeps the pencil prefilled with canonical. Helper
copy rewritten from the handoff draft because search *does* match the display spelling. Live
QA on 7810/5174 with seeded `tmdb` spellings: provider pick → h1 + line; pencil prefill
canonical + unchanged submit is a no-op; search `keßl`/`kessler` both return the row labelled
`Ana Keßler`; record-chip confirm → line gone, link back with focus; film "On record as";
studio custom; visitor sees line only; 3 skins + 375px. No component harness in `web/`, so
those are the evidence (handoff QA 3.6). Handoff: **378 shipped, Draft PR #332 updated; next is
Kevin's QA 4.3 go/no-go — pass → mark ready + sweep 374–378 to In Review; fail → revert the 378
commit and close it Won't Do.** Later the same day: 4.3 run. First pass reached for the tmdb
Refresh button for "change how it looks" (the adoption layer, not precedence) — at rest nothing
said another spelling existed. Re-run with the verbs in plain terms passed. One-line fix: the
quiet link names the offered spelling. **PR #332 marked ready; 373–378 swept to In Review.**

### 2026-09-14 · 377 edition — coded, tested, live-QA'd, security-reviewed
- skills: code-review (2 findings, both fixed), security-review (clean)
Explore-agent change map, then backend test-first: the F48 patterns are `^…$` full-stem matches,
so RD7's "anywhere in the basename" meant **lifting** `{edition-X}` out of the stem before
matching — otherwise `{title}` swallows the marker (the test showed it). `edition` = TierHigh so
a lone marker (0.30 + 0.50) clears the flag-gated auto-apply and a tag conflict never does; no
special rule. Extractor needed nothing (unclassified keys already land in the file layer).
Integration test round-trips a written edition on generated MKV + MP4 through the real
extractor. Live QA found the real gap: a file with **no tag, no marker, no decision has no
`edition` row at all** (resolver drops it), so RD11's deep link had nowhere to land — the media
page now renders a deep-linked missing field as an empty curatable row from its completeness
facet, gated by a new `curatable` flag (code-review caught that ungated it would hand a text
editor to `poster_url`). Two design deviations recorded in the handoff: the `— · file` chip stays
(it is the F37 blank-pin, the handoff's "never renders empty sources" was wrong), and the §2c
helper line was not built. Testbed: Dune (1984) has two real editions; local films mapping got
the `edition` block. Later in the session Kevin reviewed the as-built figure and moved the edition next to the
title on both pages: the film row's pill now sits directly after the file title (resolution +
Write button stay right-aligned) and the media header shows the same read-only pill in
`NameEditControl`'s `trailing` slot before the pencil; the Metadata row remains the curation
mount. Then "wrap vs. truncate on narrow screens": the header pill is uncapped — it drops
beneath the title at phone width and a too-wide value wraps inside the pill (`shrink-0` +
`max-w-full wrap-anywhere` in a media-page flex-wrap row; `NameEditControl`'s `trailing` slot
was tried first and squeezed the title to one letter per line — its row is non-wrapping by
design). Same rule then applied to the film row (title group `flex-wrap`, pill uncapped).
Figure + handoff §2–§3 as-built table updated. Follow-up in the same session: Kevin ruled on the two open calls — A for the helper line;
and since editions sit on media and most have none, `edition` became a `CriticalityOptional`
facet (listed for the deep link, never scored or queued) so a library's score doesn't fall
when the field is declared. Handoff: **377 shipped, Draft PR #332 updated; next is 378 (Low,
kill criterion) — or mark the PR ready and sweep 374–377 to In Review if 378 is cut.**

### 2026-09-14 · prod probe (filenames) — 377 sized
- skills: none (`scripts/probe-edition-filenames.mjs`, counts only), code-review, security-review
1253 video files (930 mkv · 311 mp4 · 10 avi · 2 m4v). **166 (13.2%) carry an edition marker: 141
strict Plex `{edition-X}`, 2 dash, 23 bare, 0 parenthesised** — RD7's strict grammar already
covers 85% of marked files, so the loose-pattern near-miss question stays deferred (≤25 files;
renaming them is cheaper than a parser). Keywords: remaster 41 · extended 29 · cut 27 · director
17 · unrated 15 · edition 11 · theatrical 9 · imax 7. Only **3 title groups hold >1 edition** (5
hold >1 file) — edition is overwhelmingly a descriptor on a single file, not a sibling
disambiguator. Kevin eyeballed the `bare` row: a few real, all filename problems to rename, not
a parser gap — **spec OQ "loose patterns in this epic?" resolved: no; RD7 stands.** Sizing consequences for 377: (a) first F48 extraction run yields ~141 exact
`filename:edition` candidates — the auto-apply flag decides whether that is 141 review rows at
once; (b) writeback of `Edition` to the 311 MP4s is a full-file rewrite each (XMP via exiftool),
MKVs are in-place via mkvpropedit; (c) the multi-edition film page state matters for 3 films.

### 2026-09-14 · 376 films into the spine — coded, tested, live-QA'd
- skills: code-review
Explore-agent change map first (no CHECK constraints to widen — 0047 is index + triggers only),
then the repo registries, `CreateFilm` as the film resolve path (films are never scanner-created,
so RD4's alias-year rule has exactly one home), and the studio page's rename/near-miss/alias
wiring copied onto the film page. Two calls made, not asked: **film merge is real** (a
`MergeOfferCard` verdict with no merge behind it would be a dead end — scene-number collisions
fall to NULL, never a dropped link) and **`EntityRef` gained `year`** so a "Dune ↔ Dune"
review pair is tellable apart. Aliases stay year-less (ADR-096's "alias APIs carry a year" did
not materialise; noted in the ADR checklist). Code-review found rename-onto-same-title wasn't
queueing → `queueFilmSameTitle` shared by create and rename. Live: renamed film:1, collision
verdict, alias add + conflict, Duplicates row with years, three skins. Handoff: **next is 377
(edition) — run the prod probe (up-next 4) first; 376 Jira stays In Progress until the epic
sweep; Draft PR #332 stays Draft.**

### 2026-09-13 · 375 external-id unification — coded, tested
- skills: code-review
Precedence test first (`TestResolvePrecedence_ExternalIDBeatsNameKey`: an exact nameKey match for
entity B loses to entity A's id — stronger than the existing convergence test), then migration
0046 with an up+down fold test. One scope call, made not asked: **the re-enrich memo column
stays.** Reader diff was larger than ADR-096 assumed — it also serves *video* refresh (video is
not in D2's type set) and is keyed by provider, not namespace. Filed HOLODEX-382 with the finding.
`Enrich` now records the adopted id for the four kinds, so the ADR-083 badge newly appears for
picker-enriched people/studios (noted in the spec). `identityQueryByType` deliberately still
builds three kinds — film resolve is 376's. Handoff: **next is 376 (films into the spine); 375
Jira stays In Progress until the epic sweep; Draft PR #332 stays Draft.**

### 2026-09-13 · 374 reference handle — coded, tested, live-QA'd
- skills: code-review
Zero schema. Design choice worth knowing: `ref` is emitted by `MarshalJSON` on the model structs
(derived from `id`), not a repo-populated column — nothing anonymously embeds an entity, so the
alias-type pattern is safe and nested payloads (Video.People, Film cast, search) get it free.
Route kind is read off `chi.RouteContext(r).RoutePattern()` — first entity collection segment —
rather than threading a kind through ~70 `pathID` call sites; non-entity routes (categories,
writeback jobs) stay bare-only. `/code-review high` clean. Screenshots time out (known) — QA is
computed-style. 374 Jira: In Progress; tests gate ticked. Handoff: **next is 375 — write the
F23 precedence test first (`entity_external_ids` consulted before nameKey/alias), then the
migration. Draft PR #332 stays Draft.**

### 2026-09-13 · tag casing is policy
Kevin: "I'd prefer tags always be lower case." Reframed 0034 from regret to policy across ADR-096
D5, spec, handoff; HOLODEX-379 → Won't Do. Strengthens D5's tag exclusion.

### 2026-09-13 · ADR-096
`/architecture` → ADR-096 (D1 reference, D2 external ids, D3 Film composite key, D4 edition, D5
display name — **Tag excluded**, upholding ADR-061). All three pre-implementation gates green.
Handoff: **next is code — 374 first (zero schema), then 375 with the precedence test first. Draft PR
#332 stays Draft until testing + security gates close on the implementation.**

### 2026-09-13 · open-question check — no spikes needed
- skills: architecture
Kevin asked whether the spec's open questions need spikes. Ran the one factual check instead:
generated MKV + MP4 samples, wrote an edition tag, read back with exiftool. MKV `EDITION` →
`Matroska:Edition` ✓. **`QuickTime:Edition` is not writable** (RD8 corrected to
`XMP-prism:Edition`, which round-trips). The other two questions are judgment calls made at
implementation time. Handoff: **next is `/architecture`.**

### 2026-09-12 · brainstorm → epic → design handoff → spec
- skills: product-brainstorming, design-handoff, write-spec

Skills: `/product-management:product-brainstorming`, `/design:design-handoff`. Created the Jira
tree (373 + 374–378, sibling 379). Branch renamed `HOLODEX-373-entity-identity-card`, epic In
Progress. Design gate landed; Kevin ratified OQ1 (deep link) and OQ2 (keep 378); spec gate landed
(F60, RD1–RD12). Handoff: **nothing is coded; next is `/architecture` on the same Draft PR, then
374.**
