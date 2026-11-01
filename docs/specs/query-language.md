# Spec: Query language — AND, OR and NOT over Tags, people, studio, duration and provider match (F87)

**Status**: Draft
**Phase**: F46 (queryable fields), the boolean layer over its typed conditions. Touches the resolver,
enrichment and writeback seams **not at all**.
**Owner**: Project owner
**Date**: 2026-10-31
**Feature block**: **F87**. One way to ask for a set of videos that can say AND, OR and NOT, with
grouping, across Tags, people, studio, duration and the existing browse facets. It is typed into the
nav search box or built with chips on browse, by owners and visitors alike, and it is the form smart
playlists store.

**Epic**: [HOLODEX-178](https://whoiskevinrich.atlassian.net/browse/HOLODEX-178) (F46) · story
[HOLODEX-611](https://whoiskevinrich.atlassian.net/browse/HOLODEX-611). Sibling epic
[HOLODEX-15](https://whoiskevinrich.atlassian.net/browse/HOLODEX-15) (Search & discovery).

**Changes other specs** (edited in place in the same PR):
- F73 [list-toolbar.md](list-toolbar.md): the Non-Goal *Tag, studio or person pickers on Media* is
  lifted for the conditions this spec adds.
- F75 [smart-playlists.md](smart-playlists.md): a smart playlist stores **this spec's query**; F75's
  "AND/NOT and cross-entity rules" Non-Goal is resolved here.

**Depends on** (all shipped): browse filters in the URL and the list-state model (F73); smart
playlists (F75); the unified nav search box ([nav-search-live-filter.md](nav-search-live-filter.md));
tag aliases and the tag hierarchy (a Tag condition includes its descendant Tags, as browse does today).

**Architecture**: [browse-and-list-state.md, "Boolean queries"](../architecture/browse-and-list-state.md#boolean-queries-an-expr-key-parsed-once-in-go-compiled-through-the-builders-fragments)
(the query is text, parsed once on the server) and [security-perimeter.md, "Query
expressions"](../architecture/security-perimeter.md#query-expressions-are-untrusted-input-bounded-before-any-lookup). **Design handoff**: [query-language-handoff.md](../design/query-language-handoff.md) +
[mockup](../design/query-language-mockup.svg), approved 2026-10-31: OR groups as one inset box with
parentheses around AND sets, NOT as an outlined chip. **Security review**: required
(visitors submit queries).

---

## Problem Statement

Browse can only narrow. Every chosen Tag, person and studio must *all* match, so the owner can't ask for
"either of these", "anything but this", or a mix of the two. The owner's real questions look like:

1. Videos tagged (*science-fiction* and *fantasy*) or *action*, not *comedy*, with Harrison Ford.
2. Videos from Paramount that aren't matched to any provider.
3. Videos tagged *comedy* under 2 minutes.

Only the third is possible today, and only by clicking through two separate controls. The first can't
be expressed at all, the second needs a provider-match condition that doesn't exist, and none of
them can be typed. Smart playlists (F75) were built to inherit whatever browse can express, so the
gap also caps what a playlist can be. A future natural-language tool (the CLI project's MCP server)
needs a well-defined query to translate into, and today there isn't one.

## Goals

1. **All three motivating queries work**, each as one typed line, and each returns exactly the
   videos it describes.
2. **One query, many front ends.** Typed text, browse chips and a saved smart playlist are the
   same query. Any of them can be turned into the others with no loss, except where the chips can't
   draw it (Goal 4).
3. **Visitors can use it too**, with the same syntax and chips, and they can never reach a condition
   or a value that's hidden from visitors.
4. **The simple stays simple.** A query with no OR or NOT looks and behaves like today's browse
   filters. Grouping appears only when the owner writes it.
5. **Ready for natural language without building it.** The query has a documented, stable form a
   tool can produce and Holodex can validate, so the MCP tool later is a translation job, not a new
   query feature.

## Non-Goals

- **Natural-language search and the MCP tool.** *Why*: the CLI project owns it. This spec only
  guarantees a target for it (P2-1).
- **"Starring" and other role-aware person conditions.** A person condition here means any credit.
  *Why*: billing order isn't stored yet ([HOLODEX-180](https://whoiskevinrich.atlassian.net/browse/HOLODEX-180)).
- **Provenance conditions beyond "matched to a provider".** For example, "this field has no provider
  value" or "the file is the source of this field". *Why*: they depend on resolved values, which
  aren't stored; and the owner's real query only needs the match.
- **Querying people, studios or Tags as results.** Results are always videos. *Why*: every
  motivating query is about videos; entity lists have their own toolbar (F73).
- **A visual query builder.** Chips can't draw everything; anything they can't draw is edited as
  text. *Why*: a drag-and-drop group editor is a large UI for a rare need, and F75 already chose
  "browse is the only editor".
- **Ranking by relevance.** Results use the list's chosen sort, as today. *Why*: unrelated to
  expressiveness.
- **Named saved searches apart from smart playlists.** *Why*: a smart playlist *is* a saved query;
  a second kind would split them.

## Concepts

- **Condition**: one test on a video, written `field:value`. Fields:
  - **tag**: the video has this Tag or one of its descendants. A Tag's aliases are accepted.
  - **person**: anyone credited on the video is this person (any credit).
  - **studio**: one of the video's studios is this studio.
  - **category**, **resolution**, **year**, **duration**: today's browse facets. *year* and
    *duration* take a comparison or a range.
  - **matched**: the video is matched to a provider (`matched:any`, or a specific provider by its
    name). Owner-only.
  - **Bare words** (no `field:`): a video matches when its title contains the word **or** it has
    the Tag whose name or alias is exactly that word (with that Tag's descendants). A bare word
    never matches a person or a studio; those need `person:` and `studio:`. Quote a phrase to
    treat it as one word: `"science-fiction"`.
- **Combining**: conditions next to each other must all match (AND). `OR` between two parts means
  either may match. A leading `-` (or `NOT`) means the part must not match. Parentheses group.
- **The query**: the whole expression. It is stored by identity (Tag, person and studio by their
  identity, not their name), so renaming any of them never breaks a query.

The three motivating queries, as the owner types them:

```
((tag:science-fiction tag:fantasy) OR tag:action) -tag:comedy person:"Harrison Ford"
studio:Paramount -matched:any
tag:comedy duration:<2m
```

AND binds tighter than OR, so the outer parentheses in the first query matter: without them,
`-tag:comedy person:"Harrison Ford"` would apply only to the *action* side. The chip view (P0-6)
makes the grouping visible.

## User Stories

**Owner**
- As the owner, I want to type a query with AND, OR and NOT into the search box so that I can find
  exactly the set I'm thinking of without clicking through filters.
- As the owner, I want to find videos not matched to any provider within a studio so that I can
  clean up a part of the library at a time.
- As the owner, I want to save any query as a smart playlist so that it stays true as the library
  grows.
- As the owner, I want a mistyped Tag or person name to be pointed out, not to silently return
  nothing, so that I trust an empty result.

**Visitor**
- As a visitor, I want to combine Tags, people, studio and duration so that I can find what I'm in
  the mood for.
- As a visitor, I want suggestions as I type `tag:` or `person:` so that I don't have to know the
  exact names or the syntax.
- As a visitor, I want a link to a query to open the same results for anyone so that I can share it.

**Edge cases**
- As anyone, when my query matches nothing, I want to see the query I ran and a way to edit it,
  not a blank grid.
- As a visitor, if a shared link contains an owner-only condition, I want a clear message that part
  of it isn't available, not someone else's private result set.

## Requirements

### P0: must have

**P0-1 Conditions.** Every field in *Concepts* is supported, including the existing browse facets, so
everything browse can do today can be written as a query.
- [ ] Each field above is accepted; any other field name is an error naming the field (P0-7).
- [ ] A Tag condition includes descendant Tags, exactly as browse does today.
- [ ] A Tag can be named by any of its aliases; casing and runs of whitespace don't matter, as with
      Tags everywhere.
- [ ] `duration` accepts `<`, `>`, `<=`, `>=` and a range `a..b`, with units s, m and h
      (`duration:<2m`, `duration:20m..1h`). `year` accepts the same comparisons on whole years.
- [ ] A bare `comedy` returns videos with "comedy" in the title plus videos tagged *comedy*;
      `tag:comedy` returns only the tagged ones. A bare word that's only part of a Tag's name
      (`science` against *science-fiction*) matches titles only.

**P0-2 Logic.** AND (adjacency), OR, NOT (`-` or `NOT`) and parentheses, nested to any depth up to the
limits in P0-9.
- [ ] Given motivating query 1 with the outer parentheses, it returns exactly the videos credited to
      Harrison Ford, not tagged *comedy*, and tagged *action* or tagged both *science-fiction* and
      *fantasy*.
- [ ] `OR` binds looser than AND; `-` applies to the single condition or group right after it.
- [ ] A query that is only negations (`-tag:comedy`) means "every video except these".
- [ ] Two conditions on the same field with no operator between them both have to match, as today
      (`tag:a tag:b` = both Tags).

**P0-3 Provider match (owner-only).** `matched:any` is true when the video is matched to at least one
provider; `matched:<provider>` when it is matched to that one. `-matched:any` finds unmatched videos.
- [ ] Given motivating query 2, it returns Paramount's videos that have no provider match.
- [ ] For a visitor, a `matched` condition is refused by the server, not just hidden in the UI (P0-8).

**P0-4 Typing a query in the nav search box.** Plain words keep today's live, tabbed search unchanged.
As soon as the text contains a field condition, an operator or a parenthesis, the box treats it as a
video query: pressing Enter opens browse with that query applied.
- [ ] `harrison` still shows today's live grouped results.
- [ ] `tag:comedy duration:<2m` + Enter opens browse showing motivating query 3's results.
- [ ] The query text is in the page URL, so Back, reload and sharing behave as browse filters do
      today (F73's list-state rules).

**P0-5 Suggestions while typing.** After `field:` the box suggests matching values (Tags with their
aliases, people, studios, categories, providers for the owner). Before a colon it suggests field
names. Choosing a suggestion inserts it, quoted if it contains spaces.
- [ ] Typing `tag:sci` suggests *science-fiction*.
- [ ] A visitor is never suggested a value hidden from visitors, or the `matched` field.
- [ ] Suggestions are keyboard-navigable, like the rest of nav search.

**P0-6 The query as chips on browse.** Browse shows the active query as chips, one per condition,
with NOT shown on the chip and OR groups shown as one group. Removing a chip removes that condition.
Adding a Tag, person or studio from browse adds a chip (an AND). F73's toolbar facets (duration, year,
resolution, category) stay as they are, and setting one adds or updates its chip. A bare word's chip
says it matches title or Tag.
- [ ] A query the chips can draw round-trips: chips → text → chips gives the same query.
- [ ] A query the chips can't draw shows as a single *Advanced query* row with **Edit as text**,
      and never as a simplified or wrong chip set.
- [ ] Setting a duration range in the toolbar and typing `duration:` produce the same condition.

**P0-7 Errors you can act on.** A query that can't run says why, in the box, and points at the part
that's wrong: an unknown field, an unknown Tag/person/studio name, unbalanced parentheses, a bad
range.
- [ ] `tag:sciencefiction` (no such Tag or alias) reports the name and suggests the closest Tags;
      it does **not** run and show an empty grid.
- [ ] A name that matches two people asks which one is meant.
- [ ] A query that runs and matches nothing shows the query and **Edit**, not a bare empty grid.

**P0-8 Visitors.** Visitors have the same syntax, suggestions and chips. The server enforces what
they may ask.
- [ ] An unknown or ambiguous name reports the same way to a visitor as to the owner. (No Tag,
      person or studio is hidden from visitors today; if one ever is, it must behave as unknown.)
- [ ] A visitor query or shared link containing `matched` is refused with a message that part of
      the query is owner-only; nothing runs.
- [ ] Results never include a video a visitor couldn't see by browsing.

**P0-9 Limits.** A query has a maximum nesting depth, a maximum number of conditions and a maximum
length. Past a limit it is refused with a message saying which one (P0-7). The values are set in the
security review, high enough that no hand-written query reaches them.

**P0-10 Smart playlists store the query.** Save as smart playlist (F75) saves the query. Opening the
playlist re-runs it; *Edit filter* opens browse with it.
- [ ] Every smart playlist saved before this feature opens with the same results after it.
- [ ] Renaming a Tag, person or studio in a saved query changes how the query reads, not what it
      matches.

### P1: nice to have

- **P1-1 Syntax help.** A small *Search syntax* link in the search panel lists the fields, operators
  and the three motivating queries as examples.
- **P1-2 Editable OR groups in chips.** Add a value to an existing OR group, or turn a chip into an
  OR with a new value, without going to text.
- **P1-3 Recent queries.** Field queries go into search history like plain searches, shown as the
  query text.

### P2: future (design so these stay possible)

- **P2-1 Natural-language queries (CLI MCP tool).** A tool turns a sentence into the query and runs
  it. The query's machine form and a name-to-identity lookup must be documented and stable enough
  for a tool outside Holodex to produce, and every query a tool sends gets the same validation,
  visitor rules and limits as a typed one.
- **P2-2 Role-aware person conditions** (`starring:`), once billing order is stored (HOLODEX-180).
- **P2-3 More F46 conditions**: age in media, nationality, and other typed fields as F46 phases
  add them, with no change to the logic layer.

## Success Metrics

A personal library has no analytics, so success is checked, not measured:

- **Leading (at launch):** all three motivating queries return correct results when typed (P0-2,
  P0-3); every existing smart playlist returns the same set before and after (P0-10); no visitor test
  can reach an owner-only condition or hidden value (P0-8).
- **Lagging (a month on):** the owner has saved at least one smart playlist that uses OR or NOT, and
  reaches for the typed query instead of clicking through browse filters for multi-condition finds.
  If the owner still clicks through filters, the syntax or suggestions are too hard. Revisit P1-1 and
  P1-2.

## Open Questions

None open. The P0-9 limits were confirmed and a rate limit rejected in the security review
(2026-11-01, [security-perimeter.md](../architecture/security-perimeter.md#query-expressions-are-untrusted-input-bounded-before-any-lookup)).

## Decisions

- **RD1 Bare words match title or exact Tag** (2026-10-31). *Why*: forgiving for visitors; creating
  a Tag later only widens a query, never flips its meaning. Exact name or alias only, and never
  people or studios, so bare words stay predictable. *Rejected*: title only (strict but makes
  everyone learn `tag:`); "Tag if one exists" (meaning flips when a Tag is created); title, Tag,
  person or studio (least predictable).
- **RD2 Toolbar facets stay and write chips** (2026-10-31). *Why*: smallest change to F73, and
  visitors keep the controls they can see. *Rejected*: folding facets into the chip row.

## Timeline Considerations

- **No hard deadline.**
- **Gates before build:** architecture (the query's form and grammar), design handoff (search box
  query mode, chips, errors, at 375px), testing strategy, security review.
- **Suggested slicing within one epic:** (1) the query and its server evaluation with P0-1–3, P0-8,
  P0-9; (2) the search box, suggestions and errors (P0-4, P0-5, P0-7); (3) chips and smart-playlist
  storage (P0-6, P0-10). Ship them together: the query without its front ends can't be used, and
  the front ends need the server rules.
