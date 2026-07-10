# QA Checklist: Derived / calculated person fields — Age & Age at death (F45)

**Spec**: [derived-person-fields.md](../specs/derived-person-fields.md) ·
**Handoff**: [derived-person-fields-handoff.md](derived-person-fields-handoff.md) ·
**ADR**: [ADR-063](../architecture/ADR-063-derived-computed-fields.md) ·
**Jira**: HOLODEX-73

Conventions: every item is numbered `section.item` and tagged by verifier —
`[smoke]` automated tests, `[agent]` agent-driven live QA, `[human]` needs human eyes.

---

## §1 Setup

- **1.1** `[agent]` Start the `backend-films` preview stack + `provider-tmdb` sidecar (see
  [[reference-holodex-preview-testbeds]]) — the testbed where enriched people carry clean ISO `birthdate`
  (3/3 at spec time).
- **1.2** `[agent]` Prepare four people: **(a)** enriched with `birthdate`, **no** `deathdate` (living — expect
  Age); **(b)** enriched with **both** `birthdate` and `deathdate` (deceased — expect Age at death); **(c)** a
  person with **no** `birthdate` (expect neither row); **(d)** a person whose `birthdate` is present but
  **unparseable** (e.g. a free-text value — expect neither row, no error).
- **1.3** `[agent]` Confirm Admin mode is **on** (owner + `effectiveOwner`) for owner passes, and prepare a
  **visitor** session (Admin mode off / no owner cookie) for visitor passes.

## §2 Smoke (run in `make test` / `npm run test`)

- **2.1** `[smoke]` `deriveAge`: `floor(now − birthdate)` whole years for a present, parseable `birthdate`;
  returns `computable=false` (⇒ no row) when `birthdate` is absent **or** unparseable; when `deathdate` is
  also present, `deriveAge` yields **no** row (age-at-death takes over).
- **2.2** `[smoke]` `deriveAgeAtDeath`: `floor(deathdate − birthdate)` requiring **both** inputs;
  `computable=false` if either is missing/unparseable; `floor` correctness at exact-anniversary boundaries.
- **2.3** `[smoke]` Leap-day convention (AC-9): `birthdate = 2000-02-29`, computed on `2026-02-28` vs
  `2026-03-01`, crosses the birthday **exactly once** — the documented convention asserted with a fixed `now`.
- **2.4** `[smoke]` `Derive(resolved, now)` purity: fixed `now` in ⇒ deterministic rows out; no I/O, no
  package-level clock; a `grep` over `internal/resolver/` finds **no** `time.Now` (AC-8).
- **2.5** `[smoke]` Stamping: an emitted row has `Computed: true`, `WinningSource == "computed:<canonical>"`
  (via `fieldsource.ForComputed`), registry `Label`/`Display`, and **nil** `Decision` / `Candidates` /
  `InSync`.
- **2.6** `[smoke]` Mutual exclusion: a person with both inputs yields **exactly one** row (`age_at_death`, not
  `age`); a living person yields exactly `age`.
- **2.7** `[smoke]` Ordering: `Derive` positions the computed row **immediately after `birthdate`** in
  `resolved[]` (adjacency is a payload guarantee, FR5).
- **2.8** `[smoke]` Non-adoptable guard (ADR-063 §D3): a decision `POST` naming a `Computed` canonical (`age`)
  or any `computed:` source is **rejected 400** and never written to `field_source_decisions`;
  `fieldsource.Valid("computed:age") == false` and `computed` is **absent** from `ForNamespace`.
- **2.9** `[smoke]` Time-varying, never stored (AC-2): advancing the injected `now` past the next birthday
  increments the rendered Age with **no** DB write and **no** migration/column touched.
- **2.10** `[smoke]` API integration: `personResolved` emits the derived row(s) in `resolved[]` for a
  birthdate-bearing person and **omits** them otherwise; owner and visitor payloads are identical (D3); the row
  carries `derived_from` = dependency **labels** (e.g. `["Born"]`).
- **2.11** `[smoke]` Frontend (Vitest): `providerFromWinningSource("computed:age") === ""` (the §3 gotcha
  guard); `calculatedFrom(["Born"]) === "calculated from Born"`; the person page's compact loop renders a
  computed row with the phrase on the value's `title` + `aria-label`, **no** icon/badge, **no** `SourceSelect`,
  and **no** promote pill for the owner.
- **2.12** `[smoke]` Golden no-op: with no computed fields registered (or a person with no birthdate), resolved
  output + person render is unchanged from pre-F45.

## §3 Agent live QA (preview tools against §1 stack)

- **3.1** `[agent]` Living person (1.2a): an **Age** row shows a bare integer directly under **Born**; the value
  carries `title="calculated from Born"` (hover tooltip) and there is **no** icon/badge on the row.
- **3.2** `[agent]` Deceased person (1.2b): an **Age at death** row shows `floor(deathdate − birthdate)` and
  **no** running Age row; the value tooltip reads "calculated from Born and Died".
- **3.3** `[agent]` Missing input (1.2c): **neither** Age nor Age-at-death appears — as owner **and** as
  visitor; no placeholder, no "—", no nudge.
- **3.4** `[agent]` Unparseable input (1.2d): **no** Age row, no error, no partial value.
- **3.5** `[agent]` Owner == visitor: the Age row is byte-identical between the Admin-mode-on and visitor
  sessions — **no** `SourceSelect` chips, no promote pill, no Custom entry on it.
- **3.6** `[agent]` No phantom provider: inspect the row — there is **no** badge element at all (no computed
  icon, no monogram "C" provider bubble — the §3 `providerFromWinningSource` gotcha).
- **3.7** `[agent]` Payload check: the person detail response has the computed row with `computed: true`,
  `winning_source: "computed:age"`, `derived_from: ["Born"]`, and **no** `decision`/`candidates`/`in_sync`.
- **3.8** `[agent]` Decision endpoint refuses computed: a `POST` (or SourceSelect attempt) naming `age` /
  `computed:age` returns **400** and writes nothing.

## §4 Human eyes — 3-skin QA (Cinémathèque · Broadcast · Brutalist)

Switch skins via the header picker (top-right). Confirm tokens react — no hardcoded color/radius/font (the
`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` guard stays empty). Navigate:
open a **person** page (People → an enriched person, e.g. one with a birth year) with Admin mode **on**; the
facts sit under the person's photo in the **Details** list.

- **4.1** `[human]` A new **Age** line appears right below the **Born** line, showing just a number (like
  `36`). It reads as a normal fact in the list — same text size and color as the other values — in all three
  skins.
- **4.2** `[human]` There is **no** symbol, icon, or badge next to the age number — it looks like any other
  plain value. Resting the pointer on the number pops a tooltip reading **"calculated from Born."** Confirm the
  line looks identical (no stray mark) in Cinémathèque, Broadcast, and Brutalist.
- **4.3** `[human]` The Age line sits flush under Born with normal spacing — nothing crowds or overlaps the
  number or the next line down, in any skin (check the tightest one, Brutalist).
- **4.4** `[human]` Open a person who has **died** (has both a birth and a death date): the line reads **"Age at
  death"** with a number, and there is **no** separate running "Age" line. Hovering the number says "calculated
  from Born and Died".
- **4.5** `[human]` Open a person with **no birth date**: there is **no** Age line at all — not a dash, not a
  blank, nothing — whether you're in Admin mode or viewing as a visitor.
- **4.6** `[human]` In **Admin mode**, the Age line has **no** editing controls — no row of selectable source
  pills, no "Promote", no "Custom". It looks and behaves exactly like it does for a plain visitor. (Contrast:
  the Bio/Name lines above it *do* have those controls for the owner.)
- **4.7** `[human]` Keyboard/reader: tabbing through the Details list **skips** the Age line (it has nothing to
  click); a screen reader still reads the whole line including the "calculated from Born" note.
