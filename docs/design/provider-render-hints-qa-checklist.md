# QA Checklist: Provider render hints + non-canonical field auto-registration (F39)

**Spec**: [provider-render-hints.md](../specs/provider-render-hints.md) ·
**Handoff**: [provider-render-hints-handoff.md](provider-render-hints-handoff.md) ·
**ADR**: [ADR-056](../architecture/ADR-056-provider-field-render-hints.md) ·
**Jira**: HOLODEX-128

Conventions: every item is numbered `section.item` and tagged by verifier —
`[smoke]` automated tests, `[agent]` agent-driven live QA, `[human]` needs human eyes.

---

## §1 Setup

- **1.1** `[agent]` Start the `backend-films` preview stack + `provider-tmdb` sidecar (see
  [[reference-holodex-preview-testbeds]]). Configure the sidecar (or the in-process fake for smoke) to
  advertise a **non-canonical** field with a `field_hints` entry and to return a value for it — e.g.
  `gender` (`render:text`), `credited_as` (`render:chips`), `trivia` (`render:long_text`),
  `home_page` (`render:url`), and one `image_url` field with a value on an **allowlisted** host and one on a
  **non-allowlisted** host.
- **1.2** `[agent]` Enrich one person, one studio, and one video from that provider so each entity carries at
  least one non-canonical value; leave one entity with **no** non-canonical values (the empty/absent group
  case).

## §2 Smoke (run in `make test` / `npm run test`)

- **2.1** `[smoke]` Contract decode: `enrich.Manifest.FieldHints` decodes a `field_hints` map; a manifest
  with **no** `field_hints` decodes unchanged (byte-identical to pre-F39); partial/garbage hint objects
  coerce (unknown `render`/`group` → `text`/`extended`, over-long `label` capped, control chars stripped).
- **2.2** `[smoke]` Ladder precedence (pure): for a key, `(label,render,order)` resolves operator mapping ▸
  code registry ▸ provider hint ▸ title-case — first tier wins; a hint on a **canonical** key (`bio`) is
  **ignored** (registry stands); a hint on a `_`-prefixed key is ignored.
- **2.3** `[smoke]` Persistence: reading `/describe` in an owner action **replaces** that provider's
  `provider_field_hints` rows (delete-then-insert, one txn); the read path (`GET /people|studios|media/{id}`)
  resolves hints from the table with **no** provider call.
- **2.4** `[smoke]` Auto-registration predicate: a shadow key is auto-registered iff present **and** not
  `_`-prefixed **and** not in the code registry **and** not already mapped/synthesized; an unmapped
  **canonical** provider key is **not** auto-surfaced; a `_studio_external_ids` row never surfaces.
- **2.5** `[smoke]` Presence gate: an advertised, hinted non-canonical key with **no** stored value for the
  entity produces **no** resolved field.
- **2.6** `[smoke]` Ordering: canonical/mapped fields keep their order; auto-registered fields follow, sorted
  by (group `primary<attributes<extended`, then `order`, then key).
- **2.7** `[smoke]` Entity-generic: auto-registration rides `ResolveFields` for video, person, **and** studio
  (extend the ADR-052 non-video baseline unit); resolver core unchanged (build fails if the video path
  diverged).
- **2.8** `[smoke]` `image_url` gate: a hinted `image_url` value on an allowlisted host resolves as an image;
  a value on a **non-allowlisted** host resolves as `text` (the resolved field marks it non-image). `url`
  non-http value → text.
- **2.9** `[smoke]` `mapping.Field.Display` propagation: `ResolveFields` sets `Display = f.Display` when set,
  else `registry.Lookup(...).Display`; empty `Display` reproduces today's output (regression guard).
- **2.10** `[smoke]` Display-only: an auto-registered `ResolvedField` carries **no** `Decision`/`Candidates`
  and no curation `Items` state; the decision/curation endpoints reject its canonical key (not a mapped
  field).
- **2.11** `[smoke]` Backward-compat golden: a provider with no `field_hints` + an entity with no
  non-canonical values → resolved output **byte-identical** to pre-F39.
- **2.12** `[smoke]` Frontend (Vitest): the `ChipValueList` renders one pill per value (read-only, no
  ✕/＋); the auto-registered read-only branch switches on `display` for text/long_text/chips/url/image_url.

## §3 Agent live QA (preview tools against §1 stack)

- **3.1** `[agent]` Person detail: below the curatable fields, an **"Additional details"** divider + heading
  appears, with `Gender: Female` (text), `Also credited as: [chip][chip]` (chips), `Trivia:` paragraph
  (long_text), `Home page:` link (url) — each with a `from tmdb` provenance badge.
- **3.2** `[agent]` Zero controls: in **owner** mode the auto-registered rows show **no** `SourceSelect`, no
  ✕/＋, no Write button — identical to visitor mode.
- **3.3** `[agent]` `image_url`: the allowlisted-host field renders a thumbnail; the non-allowlisted-host
  field renders the URL as **text** (no `<img>`, no broken-image icon, no error state).
- **3.4** `[agent]` Absent group: the entity left without non-canonical values (1.2) shows **no** divider and
  **no** "Additional details" group — page identical to pre-F39.
- **3.5** `[agent]` Studio + media: the same group renders on `/studios/{id}` and `/media/{id}` with the
  hinted modes and provenance.
- **3.6** `[agent]` Operator override: add a `metadata-mappings.yaml` entry for the same key + reload-config →
  the field moves out of "Additional details" into the curatable set with source chips (promotion path);
  provider hint no longer governs its label/render.
- **3.7** `[agent]` No-hint fallback: a non-canonical field returned **without** a `field_hints` entry renders
  with a title-cased label as plain text in the group (the floor, reached automatically).

## §4 Human eyes — 3-skin QA (Cinémathèque · Broadcast · Brutalist)

Switch skins via the header picker. Confirm tokens react (no hardcoded color/radius/font — the
`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)'` guard over `web/src` stays empty).

- **4.1** `[human]` Each render mode reads correctly in all three skins: `text`/`long_text` against
  `bg-surface`; `chips` pills use `border-rule`/`text-ink`; `url` links use `text-accent`; `image_url`
  thumbnail uses `rounded-theme`/`border-rule`.
- **4.2** `[human]` No badge-vs-chip collision on a `chips` row (the F36/F38 regression class) in any skin.
- **4.3** `[human]` The "Additional details" divider (`border-rule`) + heading (`text-muted`, `text-xs`,
  sentence case) sit correctly below the curatable fields; the group is absent when empty.
- **4.4** `[human]` The non-allowlisted `image_url` text fallback is legible (no phantom image frame) in all
  three skins.
