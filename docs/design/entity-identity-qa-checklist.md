# Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)

**Spec**: [entity-identity.md](../specs/entity-identity.md) · **ADR**: [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md) · **Design**: [handoff](entity-identity-handoff.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. Items are grouped into sections **by verifier**, so each actor runs only their own:
> - **§2 Smoke** — covered by an automated test or build gate (`go test`, `svelte-check`, the token-guard `rg`). Green build = pass; the target test is named.
> - **§3 Agent** — an AI agent drives the running app (DOM/ARIA/network/computed-style). Deterministic, no human judgment.
> - **§4 Human** — needs a human's eye (legibility, contrast, aesthetics, per-skin "look", CJK tofu).
>
> §1 is one-time **setup** that §3 and §4 depend on. Every item is numbered `section.item` — cite the number when filing a miss.
> Test names are the **targets** the implementation creates (this checklist ships with the spec/handoff, ahead of code).

---

## 1. Setup / preconditions

> **Who does this:** whoever sets up the session (a developer or the agent) — *not* the §4 human. **Quick "is it ready?" check:** open **Owner → Duplicates**; if you see a grouped list of look-alike pairs (tags first), the backfill ran and you're set up.

- [ ] 1.1 App running on a library seeded so the collision surfaces exist: at least one **pure-case** pair per entity (`fox`/`Fox` studio, `Action`/`action` tag, a person cased two ways) and one **fuzzy** pair (`sci fi`/`scifi` tag, `Warner Bros.`/`Warner Bros` studio). Dev: `--media-path E:/TestCopy-Film --host 127.0.0.1`.
- [ ] 1.2 Run the **one-time identity backfill** once (fresh boot post-migration), so the 14 hard pairs are folded and the ~56 near-misses are queued.
- [ ] 1.3 Exercise **both token states**: `ADMIN_TOKEN` unset (open/owner) vs set (locked → unlock via `/owner`), and confirm `effectiveOwner` on/off.
- [ ] 1.4 Have ready: a studio pair to merge (to test re-derivation), a tag to rename into an existing one (near-miss), and names with **diacritics** ("Beyoncé") and **CJK** ("宮崎駿") for §4.
- [ ] 1.5 Devtools open (Network + Console); skin picker reachable; a `prefers-reduced-motion: reduce` profile ready.
- [ ] 1.6 Keep the read-only probe handy to re-verify counts: `sqlite3 -readonly <db> ".read scripts/detect_entity_collisions.sql"` — **Tier A should read 0** after the backfill.

---

## 2. Smoke — automated (green in CI)

**Identity core**
- [ ] 2.1 **Resolve order + per-entity normalize** — `resolveOrCreateByName` tries external-id → `nameKey` (canonical ∪ alias) → create; `"fox"≡"Fox"≡" fox "` converge for person/studio/tag; `"sci fi"≡"scifi"` converge for **tag** only; `"Mary Jane"≢"MaryJane"` stays two for person/studio (RD1–RD3). *(target `TestResolveOrCreateByName_Precedence`, `TestNormalizePerEntity`.)*
- [ ] 2.2 **`nameKey` uniqueness across canonical ∪ aliases** — a name owned by a canonical row and the same name as another entity's alias cannot coexist; the unique expression index rejects a second canonical case-variant (cardinal: case/whitespace never forks identity). *(target `TestNameKeyUniqueAcrossNamespaces`.)*

**Collision matrix (all three modes × scan/editor × entity)**
- [ ] 2.3 **canonical↔canonical (case)** — scanning `"Fox"` when studio `"fox"` exists routes to the one studio (no second); an editor rename onto another entity's `nameKey` → **409**, no mutation. *(target `TestCollision_CanonicalCase`.)*
- [ ] 2.4 **canonical↔alias** — adding an alias equal to another entity's canonical name → **409** with `{conflict}`; scanning that string routes to the alias's canonical. *(target `TestCollision_CanonicalAlias`.)*
- [ ] 2.5 **alias↔alias** — two entities cannot both own the same alias `nameKey`; the second add is rejected/deduped (no silent homonym merge). Parameterized person/studio/tag. *(target `TestCollision_AliasAlias`.)*

**Merge**
- [ ] 2.6 **Merge mechanics (per entity)** — de-duped association union; decisions/curation/enrichment **moved not dropped** where non-conflicting (survivor wins on conflict); loser name → alias; loser deleted; self-merge/unknown → 400/404. *(target `TestMergeEntity`, `TestMergeEntityValidation`, parameterized.)*
- [ ] 2.7 **Studio merge survives re-derivation (cardinal, RD6)** — merge `WB`→`Warner Bros.`, then run `RelinkVideoStudios` (scan/enrich/decision); the loser is **not** recreated and both libraries sit on the survivor. *(target `TestStudioMergeSurvivesRederivation`.)*
- [ ] 2.8 **Merge/alias survive re-scan (person + tag)** — a full upsert pass routes alias-named files to the canonical and never re-creates a merged-away duplicate (extends the F23 invariant to tags). *(target `TestIdentityMergeSurvivesRescan`.)*

**Backfill + queue + keep-separate**
- [ ] 2.9 **Backfill auto-folds only the safe (cardinal, RD10)** — over a seeded library the pass folds the pure-case hard pairs (survivor = lower id, move-not-drop) and **queues** the near-misses without merging any; **idempotent** (second pass folds/queues nothing); one job run, **no path/secret** in `detail`. *(target `TestIdentityBackfill`, `TestIdentityBackfillIdempotent`.)*
- [ ] 2.10 **Near-miss detection excludes exact + kept-separate** — the loose-key detector flags only fuzzy pairs (different `nameKey`), never an exact match and never a pair in `entity_keep_separate`. *(target `TestNearMissDetect`.)*
- [ ] 2.11 **Kept-separate never nags (cardinal, RD5)** — after `dismiss`, a subsequent scan-flag + detector pass does **not** re-propose that pair. *(target `TestKeepSeparateNoNag`.)*

**Search / migration**
- [ ] 2.12 **F23 search parity after migration (RD11)** — `person_aliases` → `entity_aliases`/`entity_aliases_fts`; the existing F23 search-by-alias tests pass **unmodified** before the old table drops. *(target: the F23 suite, unchanged + `TestPersonAliasMigrationParity`.)*
- [ ] 2.13 **Alias search for studio + tag** — a studio/tag alias surfaces its entity in global search (diacritic-folded, deduped with name matches → appears once, per-group limit). *(target `TestEntityAliasSearch`.)*

**Endpoints + boundaries**
- [ ] 2.14 **Endpoints owner-gated + validated** — `alias add/delete`, `merge`, `rename` for studio + tag mirror person: **401** without token when gated, **400** invalid/self-merge/empty, **404** unknown, **409** cross-entity name; `GET /owner/duplicates` + `POST …/dismiss` owner-gated. *(target `TestIdentityEndpointsGatedAndValidated`.)*
- [ ] 2.15 **Identity is DB-only (no media write)** — no alias/merge/rename/dismiss/backfill path issues a `WriteBatch` or creates `.holodex-tmp`/`.holodex-new`; zero `/writeback` calls (F37/ADR-053 precedent). *(target `TestIdentityNoFileWrite`.)*
- [ ] 2.16 **`svelte-check`** passes with the new alias/queue types + page changes.
- [ ] 2.17 **Token-discipline guard** empty against the changed markup: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` (alias chips + tag pills `rounded-full` are the only intentional fixed radius).

> `/security-review` sign-off is required before merge in addition to these (new owner-gated mutations across three entities — ADR-030).

---

## 3. Agent — drive the running app

**Owner-gating & control visibility**
- [ ] 3.1 **Owner**: person & studio pages show the "Also known as" panel with add field + per-chip ✕ + "Merge … in"; `/tags` shows a "Manage tags" toggle; `/owner/duplicates` is reachable.
- [ ] 3.2 **Non-owner** (token set, none entered): alias chips render **read-only**; add/✕/merge, the tag manage toggle, the banner, and the Duplicates tab are **absent from the DOM** (not hidden).

**Alias panels — person + studio (reuse F23)**
- [ ] 3.3 On a **studio** page, add/remove aliases exactly as on a person page: add fires `POST /studios/{id}/aliases`, chip appears, input clears, focus stays; ✕ optimistically removes (restore + `text-warn` on forced failure).
- [ ] 3.4 A **rename** on a studio updates its name and keeps the old name as an alias; renaming onto another studio's `nameKey` → the **exact-collision card** (§3.9), no silent merge.

**Merge — `EntityPicker`**
- [ ] 3.5 "Merge … in" opens `EntityPicker` (focus to input); the list **excludes the current entity** and shows each candidate's video count; roving tabindex (↑/↓/Enter).
- [ ] 3.6 Pick → informed confirm "Merge {name} ({n}) into {canonical}?" (Back/Merge) → modal closes, page reloads with the enlarged list and the merged name as a new alias; the loser's page 404s.
- [ ] 3.7 **Studio re-derivation (RD6)**: after a studio merge, trigger a rescan **and** a re-enrich → the merged-away studio does **not** reappear in `/studios` and both libraries stay on the survivor.

**Tag manage mode**
- [ ] 3.8 Toggle "Manage tags" → pills become **selectable** (selected = `border-accent bg-surface-2` + ✓); select 2 → the manage bar's **Merge…** opens the picker; each pill's `⋯` menu offers **rename / add alias / merge into…**; toggling off restores plain browse pills.

**Exact-collision card vs near-miss soft-warning (the load-bearing UX distinction)**
- [ ] 3.9 **Exact collision** — add an alias / rename to a string that is *exactly* another entity's `nameKey` → a **bordered card** ("**{X}** ({n}) is already a separate {entity}. Are they the same?") with **Yes, merge them in** / **No, keep separate**; the create does **not** happen until you choose (blocking). "Keep separate" records the pair.
- [ ] 3.10 **Near-miss** — create/rename a tag to a *fuzzy* match (`scifi` when `sci fi` exists) → a **quiet, non-blocking** line "Looks like **sci fi** (n) — merge instead?" with **Create anyway**; "Create anyway" proceeds **and** records keep-separate (it won't re-warn next time).

**Banner + Duplicates tab**
- [ ] 3.11 A list with a non-empty queue shows the **"N possible duplicates" banner** (owner-only); "Review" deep-links `/owner/duplicates?type=…` filtered to that entity; the banner is **absent** at zero and for visitors.
- [ ] 3.12 `/owner/duplicates`: pairs **grouped by entity, tags first**; each row shows both names + counts + variation. **Merge** opens the informed confirm; **Keep separate** fades the row out and it **does not re-surface** on reload (keep-separate honored).
- [ ] 3.13 After clearing the queue, the tab shows the themed **empty state** ("No possible duplicates.").

**Backfill outcome + search**
- [ ] 3.14 Re-run the probe (§1.6) → **Tier A = 0**; the pure-case pairs are gone from `/people`, `/studios`, `/tags` and every association survives on the survivor.
- [ ] 3.15 Add a studio/tag alias → global search for it surfaces the entity (once) + its media; deleting the alias stops surfacing it that way.

**A11y**
- [ ] 3.16 `EntityPicker` is `role=dialog aria-modal`, focus-trapped, Esc-closable, focus returns to trigger; the list is a `role=listbox` with roving tabindex.
- [ ] 3.17 The tag `⋯` opener is a real `<button aria-label="Tag actions: {name}">`; its popover is a keyboard-navigable menu (Esc closes, focus returns); selectable pills expose `aria-pressed`.
- [ ] 3.18 The banner is `role=status`/`aria-live=polite`; each Duplicates row has an accessible name ("Possible duplicate: sci fi and scifi"); after Keep-separate, focus moves to the next row (never `<body>`).

---

## 4. Human — needs your eyes (all three skins)

> **How to run this:** open the app in a browser. In the header there's a **skin picker** — run every item **three times**, once in each skin: **Cinémathèque · Broadcast · Brutalist**. You're checking things *look right* and read clearly — the agent already checked they work.

- [ ] 4.1 **Studio "Also known as" panel** looks like the person one you know — quiet alias pills on the card, ✕ turns the skin's highlight (never red/orange) on hover, the "Add" button is the skin's main highlight with legible text. Nothing looks like a different app.
- [ ] 4.2 **Tag manage mode** reads clearly: a selected pill picks up the skin's accent edge and a check; the `⋯` menu is legible; plain (unmanaged) tags look unchanged.
- [ ] 4.3 **The two prompts feel different on purpose** — the exact-collision card is a bordered box that clearly asks you to choose; the near-miss is a light, ignorable nudge with "Create anyway". On **Brutalist** especially, the error/warn color must stay clearly different from the bright-lime highlight (the merge action).
- [ ] 4.4 **The duplicates banner** reads as an advisory (its marker is the warn color, not the highlight), and "Review" clearly leads somewhere.
- [ ] 4.5 **The Duplicates tab reads like a tidy worklist**, not a wall: scanning the tag pairs and clearing a few feels quick; the two names + counts per row are easy to compare; merging shows you what you're folding in before it happens; the empty state feels like a healthy finish, not an error.
- [ ] 4.6 **Accented + CJK names render** in every skin — add "Beyoncé" and "宮崎駿" as aliases and confirm no boxes/▯/garbled glyphs on the blocky Broadcast/Brutalist faces.
- [ ] 4.7 **Card edges match the skin** — panels/rows softly rounded on Cinémathèque, square on Broadcast/Brutalist; only the alias pills and tag pills stay fully rounded (intentional).
- [ ] 4.8 **Narrow window** — drag the window narrow: alias chips, tag pills, and duplicate-row actions wrap gracefully; nothing overlaps or collides.
- [ ] 4.9 **Reduced motion** — with "reduce motion" on, the merge dialog and the Keep-separate row fade shouldn't animate distractingly.

> Verify with `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` returning empty for the changed markup.
