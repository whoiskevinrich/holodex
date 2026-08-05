# QA Checklist: Configurable provider search patterns (HOLODEX-254)

Work through this against a running app. Testbed: any Holodex instance with a `metadata-sources.yaml`
enabling at least one provider (e.g. `backend-amv`), signed in as owner. Some items need a
`search_pattern`/`default_search_pattern` set in that file and `POST /admin/reload-config` (or a
restart) to pick it up.

Spec [`configurable-provider-search-patterns.md`](../specs/configurable-provider-search-patterns.md) ·
design handoff [`configurable-provider-search-patterns-handoff.md`](configurable-provider-search-patterns-handoff.md) ·
ADR [`ADR-080`](../architecture/ADR-080-configurable-provider-search-patterns.md).

Legend: **[smoke]** = quick programmatic check · **[agent]** = verified this session
(`javascript_tool` / unit tests) · **[human]** = needs a human look.

---

## 1. Setup / smoke

1.1 **[smoke]** `go test ./internal/enrich/...` passes, including new `query_test.go` cases for
`BuildQuery` (all precedence tiers, optional-token omission, required-token fallthrough,
unknown-token rejection) and `sanitizeTitle` (bracket/comma stripping, resolution regex, whitespace
collapse, non-resolution-digit false-positive guards, empty-result fallback to raw).
1.2 **[smoke]** `npm --prefix web run check` passes with 0 new errors.
1.3 **[smoke]** A golden test asserting `POST /resolve`'s request body shape is byte-identical to
pre-F53 for a video with no pattern configured (only the `hint.query` *content* changes per FR4, not
the JSON shape).

## 2. Agent-verified (this session)

2.1 **[agent]** With `search_pattern: "{studio?} {title?} {performers?} {year?}"` set for a provider
and reload-config'd, opening Enrich for a video with resolved studio/title/actors/year seeds the
search box with the rendered pattern (verified against `getMedia`'s `enrich_queries` field), not the
raw title.
2.2 **[agent]** With the same pattern but the video has **no** resolved `studio`, the whole tier falls
through (studio is inside a `{studio?}` optional token in this example, so it's simply omitted from
the output — not a fallthrough case; verify a *second* pattern using `{studio}` **without** `?` on a
video with no studio correctly falls through to the next-lower tier instead of rendering with a gap).
2.3 **[agent]** `preferred_search_pattern` returned by a stubbed provider `/describe` is used when no
operator `search_pattern` is set for that provider, and is ignored (operator wins) when one is set.
2.4 **[agent]** A video with zero resolved studio/performers/year and no pattern configured seeds the
box with the **sanitized** raw title (`[MyStudio] My Title (Some Actor, Other Actor) 720p` →
`MyStudio My Title Some Actor Other Actor`) — not the literal string.
2.5 **[agent]** A title containing `Agent 007` or `Suite 1080` is unaffected by the sanitizer (no
false-positive resolution-token strip).
2.6 **[agent]** A degenerate title that sanitizes to empty (e.g. `[720p]` alone) seeds the box with
the **raw**, unsanitized title — never blank.
2.7 **[agent]** An unknown token name in a configured `search_pattern` is rejected at config-load
time (check server logs for the warning) and does **not** disable the provider — Enrich still works
for it, falling to the next-lower tier.
2.8 **[agent]** `EnrichPicker.svelte`'s own file has zero diff from before this change (file-level
assertion, not just behavioral) — confirms the "no component change" claim in the handoff.
2.9 **[agent]** Person and Studio detail pages' Enrich pickers are unaffected — `entityName` still
equals the raw `person.name`/`studio.name`, no pattern logic invoked for those entity types.
2.10 **[agent]** F22.5b's auto-search-on-open and single-strong-match auto-apply still fire correctly
against a pattern-seeded or sanitized-seeded value (not just a raw-title-seeded one).
2.11 **[agent]** No console errors across pattern-configured, sanitized-floor, and unknown-token-
fallback scenarios.
2.12 *(only if the P1 caption is built)* **[agent]** The "Pre-filled from…" caption appears only when
the seed differs from the raw resolved title, shows the correct source ("search pattern" vs
"filename cleanup"), and disappears on the first keystroke in the search box.

## 3. Human look

3.1 **[human]** As the owner, open a video whose title looks like a raw scraped filename (brackets,
commas, a resolution tag) and has **no** search pattern configured for its provider. Click Enrich.
The search box should show a cleaned-up version of the title — no brackets, no commas, no `720p`/
similar — not the literal messy string.
3.2 **[human]** Add a `search_pattern` for that provider in `metadata-sources.yaml`
(`{studio?} {title?} {performers?} {year?}`) and reload config (`POST /admin/reload-config` or
restart). Reopen Enrich on a video that has a resolved studio, title, and year. The search box should
now show those fields space-separated, not the title alone.
3.3 **[human]** Retype the search box after either of the above. Editing should feel exactly like it
does today — the picker doesn't change based on how the box got its starting text, only what starts
in it.
3.4 **[human]** Open Enrich on a Person or Studio page. The search box should still be seeded with the
person's/studio's plain name, exactly as before this change — nothing here should look different.
3.5 *(only if the P1 caption is built)* **[human]** Repeat 3.1–3.2 and look for a small muted line
under the search box saying where the pre-filled text came from. It should read clearly (not
washed-out) and disappear the moment you start typing. Check all three skins (header picker:
Cinémathèque, Broadcast, Brutalist).
