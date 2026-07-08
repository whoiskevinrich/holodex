# QA Checklist: In-app promote / override affordance for auto-registered fields (F44)

**Spec**: [promote-override-fields.md](../specs/promote-override-fields.md) ·
**Handoff**: [promote-override-fields-handoff.md](promote-override-fields-handoff.md) ·
**ADR**: [ADR-062](../architecture/ADR-062-in-app-field-promotion.md) ·
**Jira**: HOLODEX-171

Conventions: every item is numbered `section.item` and tagged by verifier —
`[smoke]` automated tests, `[agent]` agent-driven live QA, `[human]` needs human eyes.

---

## §1 Setup

- **1.1** `[agent]` Start the `backend-films` preview stack + `provider-tmdb` sidecar (see
  [[reference-holodex-preview-testbeds]]). Configure the provider (or the in-process fake for smoke) to return
  at least one **non-canonical** field with a stored value on **each** entity type — e.g. a scalar
  (`measurements`, `render:text`), a multi-value (`credited_as`, `render:chips`), and one `image_url` field
  with a value on an **allowlisted** host and one on a **non-allowlisted** host.
- **1.2** `[agent]` Enrich one person, one studio, and one video so each carries ≥1 auto-registered value;
  leave one entity with **no** non-canonical values (the golden no-op case).
- **1.3** `[agent]` Confirm Admin mode is **on** (owner + `effectiveOwner`) for the owner passes, and prepare a
  **visitor** session (Admin mode off / no owner cookie) for the visitor passes.

## §2 Smoke (run in `make test` / `npm run test`)

- **2.1** `[smoke]` Store/repo: `SetPromotion` upserts a `(entity_type, field_key)` row under `writeMu`
  (stamping `created_at`/`updated_at` in `timeLayout`); a second `SetPromotion` on the same key updates in
  place (no duplicate). `ClearPromotion` deletes; deleting a missing row is a no-op. `PromotionsForEntityType`
  returns only that type's rows.
- **2.2** `[smoke]` Ladder tier-0 (pure): for a promoted key, `(label,render,group,order)` resolves
  **promotion ▸ operator YAML ▸ registry ▸ provider hint ▸ title-case** — first tier wins. A promotion with an
  **empty** presentation column falls through to the lower tiers for that column only (e.g. empty `label`
  inherits the provider hint / title-case label while a non-empty `render` still wins).
- **2.3** `[smoke]` Tier-0 outranks YAML (D3): for a **video** key that has **both** a `metadata-mappings.yaml`
  mapping and a promotion, the promotion's label/render/order **and** curatable status win; the field renders
  **once** (promotion replaces the YAML `mapping.Field` on the `canonical` collision).
- **2.4** `[smoke]` Non-canonical guard: `SetPromotion` / the `PUT` handler **reject** a canonical key
  (`registry.IsKnown ⇒ 422`) and a `_`-prefixed key (`⇒ 422`); no row is created.
- **2.5** `[smoke]` Materialization: a promoted key produces a synthetic `mapping.Field{Canonical: key,
  Filterable: false, Multi: render=="chips"}` whose `ParsedSources` = one `provider:<ns>` per namespace present
  for `(entity_type, entity_id, field_key)` in that entity's shadow rows (union across providers); baseline is
  always a candidate; `manual` is always available.
- **2.6** `[smoke]` Decision/curation attach (F36/F30): after materialization, `ResolveFields` attaches
  `Decision`/`Candidates`/`InSync` to a scalar promoted field, and per-value union + curation items to a
  `chips` promoted field — via the existing code paths, `ResolvedField.AutoRegistered == false`.
- **2.7** `[smoke]` Auto-registration yields (FR3): a promoted key is **excluded** from `AutoRegisterFields`
  (it renders once, via the curatable path, not doubled) — with no new predicate.
- **2.8** `[smoke]` Per-entity curation independence: a `field_source_decisions` / `metadata_curation` row on a
  promoted key for entity A does **not** affect entity B; the presentation (from the global promotion) is
  shared.
- **2.9** `[smoke]` De-/re-promote survives curation: `ClearPromotion` reverts the key to an auto-registered
  row (`AutoRegistered == true`, no `Decision`); prior decision/curation rows persist (keyed by `field_key`)
  and re-apply on a subsequent `SetPromotion`.
- **2.10** `[smoke]` Sanitize/coerce on ingest (defense in depth): an over-long `label` is capped at 64 and
  control chars stripped; an unknown `render`/`group` coerces to `text`/its default; the stored/resolved values
  reflect the cleaned input.
- **2.11** `[smoke]` `image_url` gate unchanged: a promoted `image_url` value on a non-allowlisted host resolves
  as `text` (not an image); promotion does not widen ADR-039.
- **2.12** `[smoke]` API shape: `PUT` → 204 upsert; `DELETE` → 204 (idempotent); `GET
  /admin/field-promotions/{entity_type}` lists rows; all three require owner (**401** unauth, before the
  handler); `entity_type ∉ {video,person,studio}` → 4xx.
- **2.13** `[smoke]` Golden no-op: with **no** promotions, resolved output + rendering is **byte-identical** to
  pre-F44 (F39 baseline), on all three entities.
- **2.14** `[smoke]` Frontend (Vitest): `AutoFieldRows` renders the **Promote** control only when
  `isOwner`; the editor emits the correct `PUT` body (`label?/render?/group?/order?`) and `DELETE` on Remove;
  the render `<select>` offers exactly `text|long_text|chips|url|image_url`.

## §3 Agent live QA (preview tools against §1 stack)

- **3.1** `[agent]` Owner sees Promote: on the person page, each auto-registered row under "Additional details"
  shows a trailing **Promote** pill; the visitor session shows **none**.
- **3.2** `[agent]` Promote a scalar: click Promote on `measurements`, set label "Measurements" + `render:text`
  + group `attributes` + order, submit → the field **leaves** "Additional details" and appears in the curatable
  list above as a **`SourceSelect`** row (baseline `·record` chip + `·tmdb` chip + Custom); provenance intact.
- **3.3** `[agent]` Promote a chips field: promote `credited_as` with `render:chips` → it becomes a
  **`CurationFieldRow`** merge row (per-value ✕ + ＋ Add), not a scalar picker.
- **3.4** `[agent]` Renders once: the promoted key appears **once** — never both as an auto row and a mapped
  row.
- **3.5** `[agent]` Global presentation, per-entity value: the promoted label appears on **every** person with
  the key; suppressing/adding a value on person A does **not** change person B; the label/order is shared.
- **3.6** `[agent]` Edit: the promoted row shows an owner-only **Edit** pill on its label line; it opens the
  **same** editor pre-filled; changing the label + Save → the new label renders after reload.
- **3.7** `[agent]` Remove promotion: **Remove promotion** in the editor → the field returns to an
  auto-registered display-only row in "Additional details"; re-promoting restores the prior curation
  (decisions/adds re-apply).
- **3.8** `[agent]` Tier-0 over YAML: for a video key with a `metadata-mappings.yaml` mapping, add a promotion
  for the same key → the promotion's label/render/order + curatable status win; no doubled row.
- **3.9** `[agent]` Studio + media parity: promote / edit / de-promote works on `/studios/{id}` and
  `/media/{id}`, not just person.
- **3.10** `[agent]` Cannot promote canonical: no Promote control appears on canonical/mapped rows (e.g. `bio`);
  a direct `PUT` for `bio` returns 422.
- **3.11** `[agent]` `image_url` promoted: a promoted `image_url` on a non-allowlisted host renders as **text**
  (no `<img>`, no error), same as F39.
- **3.12** `[agent]` Error path: force a `PUT` failure (e.g. offline) → the editor stays open with a
  `text-warn` message; nothing moves; a retry succeeds and reloads.
- **3.13** `[agent]` Golden no-op: the entity with no promotions (1.2) is visually identical to pre-F44.

## §4 Human eyes — 3-skin QA (Cinémathèque · Broadcast · Brutalist)

Switch skins via the header picker (top-right). Confirm tokens react — no hardcoded color/radius/font (the
`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` guard stays empty). Navigate:
open any **person** page (People → a person) with Admin mode **on**; the fields sit under the person's photo in
the "Details" list, with provider extras under an **"Additional details"** subheading.

- **4.1** `[human]` The **Promote** pill (a small outlined button with an up-arrow, at the end of a provider
  extra's row) looks and behaves like the "＋ Add" button used elsewhere on the page — thin outline at rest,
  turning the accent color (gold in Cinémathèque, cyan in Broadcast, lime in Brutalist) on hover — in all three
  skins. It should not crowd or overlap the small grey provider badge (e.g. "tmdb") on the same row.
- **4.2** `[human]` Clicking Promote opens a small **bordered editor box** directly under that row, with an
  **accent-colored outline** that clearly reads as "you're editing this" (distinct from the plain rows). The
  rows below politely move down; nothing floats or overlaps. Confirm this in all three skins.
- **4.3** `[human]` Inside the editor: the **Label** text box, the **Render** and **Group** dropdowns, and the
  **Order** number box are all legible and correctly themed (corners are subtly rounded in Cinémathèque, square
  in Broadcast/Brutalist — this is expected, not a bug). The primary **Promote / Save** button is a solid
  accent-filled button with readable text on it in every skin.
- **4.4** `[human]` In **edit** mode, the **Remove promotion** action is styled in the **warning** color (a
  red/orange, deliberately different from the accent) — it must never look like the primary button.
- **4.5** `[human]` After promoting, the field **moves up** out of "Additional details" into the main details
  and gains value controls (either a row of selectable pills, or removable chips with a "＋ Add"). After
  **Remove promotion**, it moves back down to a plain read-only row. Confirm the move reads cleanly (no leftover
  duplicate, no flicker of the old row) in all three skins.
- **4.6** `[human]` As a **visitor** (Admin mode off): there are **no** Promote or Edit buttons and **no**
  editor anywhere — promoted fields just show their curated label and value like any other field.
- **4.7** `[human]` Empty/edge states read correctly: an entity with no provider extras shows no "Additional
  details" section at all; promoting the only extra makes that whole subheading disappear; a promoted
  `image_url` from an untrusted host shows the link as plain text (no broken-image box) — in all three skins.
- **4.8** `[human]` Keyboard: Tab reaches the Promote button; opening the editor lands focus in the Label box;
  Tab walks Label → Render → Group → Order → (Remove) → Cancel → Promote; **Esc** closes the editor and returns
  focus to the button you opened it from. No keyboard trap.
