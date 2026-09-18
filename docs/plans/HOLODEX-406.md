---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-406
status: in-progress
release_note: The Enrich picker now shows each candidate's picture — a headshot, poster, or studio logo — beside its name, so same-named people and same-title films can be told apart at a glance. Providers that don't send one get a monogram in the same spot.
---

# HOLODEX-406 · F63 — `candidates[].image_url`: candidate thumbnail in the resolve picker

Brainstormed 2026-09-16 from "make the People, Media, and Film pickers support an image". The
facts reshaped it: there is **one** shared `EnrichPicker.svelte` (person, media, film **and**
studio pages mount it), no image field exists on `Candidate` anywhere (core, TS, sidecar), and
the perimeter question was already answered by ADR-056 — `render: image_url` field values are
hot-linked from an `asset_hosts`-allowlisted host through `Service.ImageURLAllowed`. So: one
optional contract key, one more caller of an existing gate, one fixed 40 × 60 column on the row.
No ADR. Supersedes F61's P2-c non-goal, whose stated cost ("the first candidate-level field
Holodex has to *fetch*") does not apply to a rendered, never-fetched thumb.

**Decisions locked from a three-option mockup** (2:3 list · 32 px circle · poster grid): the 2:3
list, because a circle crops posters and a grid drops match strength, `detail`, and the
roving-tabindex list; hot-link over proxy; studios ride along in the same box via `object-contain`
on the logo plate — zero entity-kind branching; the sidecar picks the rendition (TMDB `w185`);
the thumb is layer-1 identity evidence (ADR-090), never an image adoption.

## Gates — definition of done

- [x] spec `write-spec` — `docs/specs/candidates-image.md` (F63: FR1–FR4, P2-a/b, AC 1–11, test
  notes per surface, Resolved Decisions table) **and** the contract amendment
  (`metadata-provider-contract.md` §2.3 example + `candidates[].image_url` row, §5 caps row, §6
  S7 note, §8 example)
- [~] architecture `architecture` — n/a: additive optional response key + an existing gate
  (ADR-056 `ImageURLAllowed`) gaining one caller; no seam, no migration, no new config key
- [ ] design `design-handoff` — `candidates-image-handoff.md` + committed SVG: the row with an
  image / monogram / F61-expanded / studio logo letterboxed, all three skins; settle the one open
  design question (which monogram-plate idiom — `ProviderIcon` or `FilmsRow`)
- [ ] backend — `Candidate.ImageURL`, `image_url` step in `sanitizeCandidates` calling
  `Service.ImageURLAllowed`; `Fake` gains `ImageURL`; sanitizer table (the riskiest-assumption
  test: foreign host / suffix-spoof / scheme / malformed / `""` all cleared) + one resolve-handler
  round-trip each for person and film
- [ ] sidecar — `providers/tmdb` maps `profile_path` / `poster_path` / `logo_path` → `image_url`
  at `w185`; omit on null; unit test on the builders; operator docs note the picker renders from
  `image.tmdb.org`
- [ ] frontend — `EnrichCandidate.image_url?`; 40 × 60 slot on every `<li>`, `<img alt=""
  loading="lazy" referrerpolicy="no-referrer">` `object-contain` on `bg-logo-plate`, monogram on
  absent **and** on `error`, top-aligned; pure helper + vitest; stub personas covering the four
  slot states; three-skin QA by computed style (screenshots time out on this picker)
- [ ] testing `testing-strategy` — header entry, §4 backend row, §5 picker row, invariants
  (rendered-not-fetched; column always present; collapsed-row parity with and without image), a
  §12 assertion on text-block x-offset parity
- [ ] security `security-review` — the allowlist now gates a second browser-rendered surface;
  confirm `sanitizeCandidates` is upstream of every resolve handler (person, video, film, studio)
  and that nothing is fetched server-side

## Up next — ordered (position = priority)

1. [ ] [S] Open the Draft PR with the spec gate; gate-status checkboxes mirror Jira (`needs-spec`
   cleared on push).
2. [ ] [M] `/design-handoff` — SVG committed, monogram-plate idiom settled, `needs-design` cleared.
3. [ ] [M] Backend + sidecar gates (FR1/FR2/FR4) with the sanitizer table.
4. [ ] [M] Frontend FR3 + stub personas + three-skin QA.
5. [ ] [S] `/testing-strategy`, then `/security-review`, clear `needs-security-review`.
6. [ ] [S] Mark ready → CI fires In Review. Post-merge: contract-sync note lands downstream in the
   sidecar repo via its contract-watch skill (never from this branch).

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 → 17 · brainstorm, story filed, spec + contract amendment written
- skills: product-brainstorming (Explore subagent for the picker/contract/perimeter facts, three-option
  show_widget mockup with a stressed-state strip, decisions via cards), write-spec
- Brainstorm reframed "three pickers" as one shared component and "fetch through the perimeter"
  (F61's reason to defer) as "render through ADR-056's gate". Kevin chose: 2:3 thumb list,
  hot-link via allowlist, studios ride along. Set aside: hover/zoom, current-image-in-header,
  proxy/cache, size negotiation.
- Filed HOLODEX-406 with the gate-status checklist and `needs-spec` / `needs-design` /
  `needs-security-review`; renamed the branch to `HOLODEX-406-picker-candidate-thumb`; fired
  In Progress.
- Wrote `docs/specs/candidates-image.md` (F63) and the four contract edits (§2.3 row + example,
  §5 caps row, §6 S7 note, §8 example).
- Handoff: spec gate green, nothing else built. Next session opens the Draft PR (if this one
  didn't) and runs `/design-handoff` — the monogram-plate idiom is the one design question open.
