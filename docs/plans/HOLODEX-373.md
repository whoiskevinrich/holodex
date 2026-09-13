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
one `entity_external_ids` table; display name = curation on `name`, no column. Tag lowercasing
(0034) is a storage regret, not display — HOLODEX-379, outside the epic.

**Stories, in shipping order:** 374 handle (High) → 375 external-id unification (High) → 376
films into the spine → 377 edition → 378 display-as (Low; kill criterion in the handoff).

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/entity-identity-card.md` (F60), RD1–RD12. Settled in the
  pass: edition writeback key is `Edition` on **both** backends (Matroska `EDITION`, exiftool
  `QuickTime:Edition` — native, verified on 13.59); `Subtitle` rejected because it's already the
  tagline's key (`tags.go:101`); filename edition follows the F48 auto-apply rule, no special case;
  film alias routing needs a year match or a unique nameKey, else queue
- [ ] architecture `architecture` — ADR for external-id unification + edition-as-field +
  curation-on-name; amendment notes on ADR-051 (name was the excluded field) and ADR-061
  (films, composite nameKey). Number via `node scripts/adr-claims.mjs`, never by eye
- [x] design `design-handoff` — `docs/design/entity-identity-card-handoff.md` +
  `entity-identity-card-mockup.svg` (4 panels). Two grounded changes vs the brainstorm: edition
  edits ride `SourceBadge` (badge-click), not a pencil; "Display as" is the name field's
  `SourceBadge`, "Rename in files" is the existing pencil — no second button. Found a real gap:
  `SourceBadge` only renders the badge when multi-source, so a filename-only edition would have no
  curation affordance (§2b). **OQ1** deep-link vs inline Set edition, **OQ2** the 378 go/no-go —
  both need Kevin
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review` — new writeback tag key, new mutation surface on `name`

## Up next — ordered (position = priority)

1. [x] [—] OQ1 = deep link, OQ2 = keep 378 — ratified 2026-09-12
2. [x] [S] `/write-spec` — landed, `needs-spec` cleared
3. [ ] [S] `/architecture` (clears `needs-adr`) — external-id unification + edition-as-field +
   curation-on-name; amendment notes on ADR-051 and ADR-061; `node scripts/adr-claims.mjs` first
4. [ ] [—] Run the read-only prod probe for edition-bearing full-film titles
   (`cut|edition|extended|unrated|remaster`) before sizing 377
5. [ ] [M] 374 handle — start here; zero schema
6. [ ] [M] 375 external-id unification — the F23 precedence test is the first thing to write
7. [ ] [—] On PR ready: sweep 374–378 to In Review by hand with the epic; on merge, sweep to Done
   (CI moves only the branch's key — an epic-keyed branch moves nothing)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-12 · brainstorm → epic → design handoff → spec
- skills: product-brainstorming, design-handoff, write-spec

Skills: `/product-management:product-brainstorming`, `/design:design-handoff`. Created the Jira
tree (373 + 374–378, sibling 379). Branch renamed `HOLODEX-373-entity-identity-card`, epic In
Progress. Design gate landed; Kevin ratified OQ1 (deep link) and OQ2 (keep 378); spec gate landed
(F60, RD1–RD12). Handoff: **nothing is coded; next is `/architecture` on the same Draft PR, then
374.**
