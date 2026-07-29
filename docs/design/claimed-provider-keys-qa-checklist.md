# QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)

**Spec**: [claimed-provider-keys.md](../specs/claimed-provider-keys.md) ·
**Handoff**: [claimed-provider-keys-handoff.md](claimed-provider-keys-handoff.md) ·
**ADR**: [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) ·
**Jira**: HOLODEX-218

Conventions: every item is numbered `section.item` and tagged by verifier —
`[smoke]` automated tests, `[agent]` agent-driven live QA, `[human]` needs human eyes.

Slice B only. Slice A (the resolver mechanism) is covered by
`internal/resolver/auto_register_test.go` and needs nothing here.

---

## §1 Setup

- **1.1** `[agent]` Start `provider-tmdb`, `backend-films`, then `web` (see
  [[reference-holodex-preview-testbeds]]). Confirm Owner view is on — the whole surface is owner-only.
- **1.2** `[agent]` Give one person a **two-provider auto-registered row**: the same paragraph stored under
  the same non-canonical key for two different providers, hinted `render: long_text` so the row takes the DD7
  wide branch. Enriching from one provider is not enough — the checklist (DD3) and the partial-attach case
  (2.4 / 3.3) both need two.
- **1.3** `[agent]` Leave one **text**-display auto-registered row on the same entity (e.g. a scalar provider
  key). DD7 changes the wide branch only, and the inline branch has to be checked side by side with it.
- **1.4** `[agent]` Write one **dangling** claim straight to `field_claims` — a target canonical that does not
  exist. The API's 422 makes this state unreachable from the UI, and it is the only way to see DD9.
- **1.5** `[agent]` Tear the fixture down afterwards. Claims are global config, not per-entity state; a leftover
  claim silently suppresses a row on every entity of that type.

## §2 Smoke (`make test`)

- **2.1** `[smoke]` Store grain: `SetClaim` upserts on `(entity_type, provider, field_key)`; the same key on two
  providers is two rows; `ClearClaim` is per provider and idempotent; entity types are independent.
  (`internal/repo/claims_test.go`)
- **2.2** `[smoke]` Migration 0029's PK carries `provider` — a `field_promotions`-shaped PK would reject the
  second provider's row. Down drops the table; re-up leaves it empty. (`internal/db/field_claims_test.go`)
- **2.3** `[smoke]` RD3/D5: `SetClaim` clears that key's promotion **in the same transaction**, and only that
  key's; `ClearClaim` does **not** resurrect it. (`internal/repo/claims_test.go`)
- **2.4** `[smoke]` The cardinal GH #178 flow: a claimed key stops auto-registering **and** feeds the target
  field as a candidate — suppression and contribution are one act. (`internal/api/field_claims_test.go`)
- **2.5** `[smoke]` Provider-scoped: claiming `provA:key` leaves a row carrying only provB's value and
  provenance. (`internal/api/field_claims_test.go`)
- **2.6** `[smoke]` D3 append order is `(provider, field_key)`, not insertion order — claiming `tmdb` first and
  another provider second resolves to the other provider. (`internal/api/field_claims_test.go`)
- **2.7** `[smoke]` A dangling claim is inert: the key auto-registers again, exactly as pre-F49, and nothing is
  pruned. (`internal/api/field_claims_test.go`)
- **2.8** `[smoke]` Validation: canonical key 422, `_`-prefixed key 422, missing `canonical` 400, a target the
  entity type does not declare 422; every route owner-gated (401 without the token).
  (`internal/api/field_claims_test.go`)
- **2.9** `[smoke]` `GET /admin/field-targets/{entity_type}` returns the **effective** post-promotion field set
  with the `merge` flag — including fields that are empty for every entity.
  (`internal/api/field_claims_test.go`)

## §3 Agent (live, one skin)

- **3.1** `[agent]` The wide row shows **Attach to…** and **Promote** as peers on one line; opening either
  editor hides both pills, and opening the second closes the first.
- **3.2** `[agent]` The editor's target `<select>` lists the entity type's **whole** field set (DD2) —
  specifically a field that is **empty on this entity** and therefore absent from the page.
- **3.3** `[agent]` Attach both providers to a **replace** field: the row disappears, the confirmation strip
  takes its place at the row's old position with an Undo, and the value is now a candidate of the target's
  source chip. Undo restores the row and deletes both claims.
- **3.4** `[agent]` Attach **one** of two providers: the row **survives** carrying the other provider's value
  and provenance only (spec §6.5 S2), and the strip names the provider that was attached.
- **3.5** `[agent]` The DD4 outcome sentence switches with the target: merge copy on a `merge: true` field,
  replace copy on a scalar.
- **3.6** `[agent]` `/owner/fields` lists the claims grouped by entity type with the right counts, the dangling
  one marked **Inactive**, and Remove drops the row and decrements the heading.
- **3.7** `[agent]` No horizontal page scroll with the editor open — a long provider value in the checklist
  truncates to one line rather than widening the Details column.
- **3.8** `[agent]` Visitor view: no pill, no editor, no strip on the same entity.

## §4 Human (all three skins — Cinémathèque, Broadcast, Brutalist)

Switch skins with the picker in the header (the three small buttons next to *Owner view*). Do each item in
**all three** before moving on — regressions here routinely show up in only one skin.

- **4.1** `[human]` Go to a person page that has an *Additional details* section with a long paragraph row.
  The provider badge and the two pills should sit on their **own line under the paragraph, pushed to the right
  edge**, not trailing the last sentence. This moved a **shipped** control, so check the **Promote** pill here
  too, not just the new one. On a narrow window they should wrap onto two lines rather than squash together.
- **4.2** `[human]` On the same page, a short one-line row (e.g. *Known for*) should look **unchanged** — badge
  and pills still sitting right after the value on the same line. If those two rows look the same as each
  other, something is wrong.
- **4.3** `[human]` Click **Attach to…**. The panel that opens should read clearly as a box: a coloured border
  against a slightly lighter background than the page (`--accent` on `--surface-2`). Check it does not blend
  into the page in any skin.
- **4.4** `[human]` In that panel, the orange/red caution line about providers sharing a key name
  (`--warn`) sits right next to the panel's coloured border (`--accent`). Confirm the two colours don't fight
  or read as the same thing. Brutalist is the risky one — lime border, hot red-orange text.
- **4.5** `[human]` Attach something, then look at the confirmation strip that replaces the row: dashed border,
  a check mark, and an outlined **Undo** at the right edge. The text must stay readable against the strip's
  background in every skin.
- **4.6** `[human]` Go to **Owner → Attached keys**. The tab should look active in the same way the other owner
  tabs do. The `provider:key` column is monospaced and clearly darker/lighter than the surrounding text; the
  **Inactive** marker reads as a warning without being the loudest thing on the page.
- **4.7** `[human]` Shrink the window to phone width on both surfaces. Nothing should overflow sideways, and
  the Remove buttons should stay reachable.

## §5 Known gaps

- **5.1** The active owner-hub tab renders the same background in all three skins. This is the shared tab class
  from F35, not something F49 changed — noted here so a skin pass does not re-discover it as new.
- **5.2** DD6's "attaching removes that promotion" warning is implemented but hard to reach from the Attach
  pill: a promoted key normally renders as its own first-class field, not as an auto-registered row. The server
  clears the promotion either way, so the warning stays as the honest thing to show if the state does occur.
