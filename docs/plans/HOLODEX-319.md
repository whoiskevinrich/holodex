---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-319                 # the tracker key; must match the branch key regex
status: in-review                   # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The provider-facing metadata contract now documents Film as a first-class entity type, including the billed-cast field, the banner asset kind, and the release-date-to-year rule.
---

# HOLODEX-319 · Metadata provider contract: catch up to the shipped film entity (post-F59)

HOLODEX-313 corrected the provider docs **mid**-F59, when the film `banner` role
([ADR-089](../architecture/ADR-089-film-enrichment-field-vocabulary.md) D4) and the billed-cast
read (D1/D2) were still decided-but-unbuilt. Both shipped in
[#293](https://github.com/whoiskevinrich/holodex/pull/293), so the same sections went stale again in
the opposite direction — they now *understate* what a film provider can send. Done means an external
provider author reading `metadata-provider-contract.md` cold can implement `entity_type: "film"`
correctly with no access to the Holodex tree, and its worked example (`tmdb-provider.md`) agrees with
it.

**Design package:** no new spec or ADR — this documents decisions already made in
[ADR-085](../architecture/ADR-085-films-entity.md) /
[ADR-086](../architecture/ADR-086-film-provider-enrichment.md) /
[ADR-089](../architecture/ADR-089-film-enrichment-field-vocabulary.md) and shipped under
[HOLODEX-308](https://whoiskevinrich.atlassian.net/browse/HOLODEX-308); no design handoff (no
user-facing surface); no testing-strategy row (no behaviour change).

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → `docs/specs/metadata-provider-contract.md` + `docs/specs/tmdb-provider.md` — this issue's whole deliverable
- [x] architecture `architecture` → not needed; records ADR-085/086/089 decisions, makes none
- [x] design `design-handoff` → not needed, no user-facing surface
- [x] testing `testing-strategy` → not needed, no behaviour change (every statement added was read off shipped code, see session log)
- [~] security `security-review` → not touched (no auth/access/infra change) — deferred, not applicable

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [spec] Contract §2.2/§2.3/§2.4, §3, §4.2c, §4.3, §4.5, §4.6, Open items — `docs/specs/metadata-provider-contract.md`
2. [x] [spec] Worked example: film `banner` asset, post-F59 landing set, `asset_kinds` — `docs/specs/tmdb-provider.md`
3. [ ] [—] Refresh each provider repo's `docs/upstream/` snapshot + `contract-sync.json` by running its `contract-watch` skill — **⛔ blocked on this PR merging to `main`** (the watcher diffs against `main` and records a GitHub blob SHA, so a pre-merge refresh would pin a SHA that does not exist). Not a holodex-repo change; owner decision recorded below.

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-05 · contract caught up to the shipped film entity; downstream sync deferred by design

- skills: (none — docs)
- **Every added statement was read off shipped code, not off the ADRs.** The ADRs were amended twice
  during F59's implementation, so building the doc from them alone would have re-documented decisions
  that changed. Sources of truth used: `internal/api/film_fields.go` (`filmScalarFields` is still
  `{description, release_date}` — the vocabulary did *not* widen), `internal/api/film_cast.go`
  (`actors` read at display time, no writes at all), `internal/api/film_year.go`
  (`filmReleaseYear` = leading 4 chars; `syncFilmYear` fill-only), `internal/enrich/assets.go`
  (`assetRoleFor` film → `poster`/`banner`/`backdrop`, `thumb` gone),
  `internal/enrich/service.go` (`people[]` gated on `EnrichEntityVideo`; `aliasEntityType` excludes
  film), and `providers/tmdb/handler.go`+`tmdb.go` (what the sidecar advertises and emits).
- **The most useful thing added is not the new `actors` row — it is the "accepted, stored, and then
  ignored" list.** The old text told a provider author what lands; it never told them what silently
  does not, which is the failure mode that actually costs them a day. `title`, `studio` +
  `_studio_external_ids`, `director`, `aliases` and `people[]` are now each named with the reason.
- Also documented for the first time: `release_date` is not purely cosmetic on a film — it fills
  `films.year`, which is half the `(name, year)` identity key. A provider sending `31/03/1999`
  produces no year and will never know why, so the parse rule is stated explicitly.
- **Two adjacent claims were wrong and are corrected rather than left to contradict the new text.**
  §3's *"a video's poster and a studio's logo are `fields` entries … v1 has no non-person image
  sink"* had been false for studio since F51/ADR-079 and became false for film at ADR-086; it sat
  three paragraphs from the new film section. `tmdb-provider.md`'s `asset_kinds` said
  `["headshot", "gallery", "banner"]` where the handler advertises
  `["photo", "gallery", "banner", "logo", "poster"]` — correcting only film's half would have left a
  list that is still wrong, so the whole list was fixed (this sweeps in the person `headshot`→`photo`
  and studio `logo` drift; called out in the PR rather than done quietly).
- **`contract-sync.json` is deliberately untouched, and it is not in this repo.** It lives at
  `docs/upstream/contract-sync.json` in each provider repo, next to a byte-exact snapshot of this
  contract, and is refreshed by their `contract-watch` skill — which diffs against
  `whoiskevinrich/holodex@main` and stores the upstream **GitHub blob SHA**. Refreshing it from a
  branch would record a SHA that does not exist. Owner chose "defer until merge" from a question
  card. Neither provider implements `film`, so the entire delta is optional/additive for them; the
  post-merge run should report drift, classify it optional, and refresh cleanly.
- Verified: every relative doc link in the contract resolves on disk, and every internal `#anchor`
  I introduced matches a real heading. One pre-existing broken anchor was found and **left alone** —
  `#45-video-credits-people` at contract §4.1 (the heading's real slug is
  `#45-video-credits--per-person-castcrew-with-headshots`, which the other four references already
  use). Unrelated to films; flagged, not fixed.
- handoff: HOLODEX-319 is complete and ready for review — docs only, no code, no tests to run. The
  one open item is Up-next #3, which cannot start until this merges; after that, run
  `bash .claude/skills/contract-watch/scripts/check-drift.sh` in each provider repo and let the skill
  produce its impact report before refreshing either snapshot.
