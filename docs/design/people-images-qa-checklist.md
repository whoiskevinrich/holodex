# Manual QA Checklist: People Images (F24)

**Spec**: [People Images (F24)](../specs/people-images.md) · **ADR**: [ADR-037](../architecture/ADR-037-person-images.md) · **Design**: [handoff](people-images-handoff.md) + [system pattern](people-images-design-system.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. Items are grouped into sections **by verifier**, so each actor runs only their own:
> - **§2 Smoke** — covered by an automated test or build gate (`go test`, `svelte-check`, the token-guard `rg`). Green build = pass; pre-checked `[x]` with the test named.
> - **§3 Agent** — an AI agent drives the running app (DOM/ARIA/network/computed-style). Deterministic, no human judgment.
> - **§4 Human** — needs a human's eye (legibility, contrast, aesthetics, per-skin "look").
>
> §1 is one-time **setup** that §3 and §4 depend on. Every item is numbered `section.item` — cite the number when filing a miss.
> A **`/security-review` sign-off is required before merge** in addition to these (binary file ingest + serving; threat model in spec §9).

---

## 1. Setup / preconditions

> **Who does this:** whoever sets up the session (a developer or the agent) — *not* the §4 human. **Quick "is it ready?" check:** open the app, go to **People → any person**; you should see a wide banner area and a round/square headshot by the name, even if they're just placeholder silhouettes. If `ADMIN_TOKEN` is unset you'll also see upload/edit affordances; that means you're set up as owner.

- [ ] 1.1 App running with a library that has at least one **person with videos** (note their `/people/[id]` URL) and that person credited on at least one **video** (note that `/media/[id]`). Dev: `--media-path E:/AMVTestCopy --host 127.0.0.1`.
- [ ] 1.2 Exercise **both token states**: `ADMIN_TOKEN` unset (open/owner) vs set (locked → unlock via `/status`).
- [ ] 1.3 Have test images ready: a normal **JPEG** and **PNG**; a **wrong-ratio** image (very wide and very tall); an image with **planted EXIF/GPS**; a **renamed non-image** (e.g. `evil.txt` → `evil.jpg`); an **oversized** image (> the configured byte/dimension bound).
- [ ] 1.4 One person with an **enriched `gender`** value (female and male if possible) and one **without** any gender, to check placeholder buckets.
- [ ] 1.5 Devtools open (Network + Console); skin picker reachable (header); a `prefers-reduced-motion: reduce` profile ready.

---

## 2. Smoke — automated (green in CI)

- [ ] 2.1 **Ingest normalization strips metadata + re-encodes** — a JPEG/PNG/GIF/WebP decodes and re-encodes to the safe format; an image with planted EXIF/GPS comes out with **none**; output dimensions/bytes bounded (F24.9, spec T1/T8). *(personimage `TestNormalizeStripsMetadata`, `TestNormalizeReencodes`.)*
- [ ] 2.2 **Ingest rejects hostile bytes, writes nothing** — renamed-non-image / truncated / polyglot → error, no file on disk; a decompression-bomb (huge declared dimensions) rejected **before** full decode (F24.9, T1/T4). *(personimage `TestNormalizeRejectsNonImage`, `TestNormalizeRejectsBomb`.)*
- [ ] 2.3 **Placeholder resolution is deterministic + bucketed** — `(skin,role,gender)` → asset; `nonbinary`+unknown+absent → `neutral`; never persisted/counted (F24.5). *(personimage `TestPlaceholderResolution`, golden SVG per cell.)*
- [ ] 2.4 **Repo CRUD + core-slot uniqueness** — insert/get/list/delete; a 2nd `headshot` **replaces** (one row, new id), never two; reorder updates `sort_order` (F24.1/F24.7/F24.15). *(repo `TestPersonImagesCRUD`, `TestCoreSlotReplace`.)*
- [ ] 2.5 **20-extra gallery cap transactional; core unaffected** — 21st `extra` rejected; filling a core role is never blocked by the cap (F24.8). *(repo `TestGalleryCap`.)*
- [ ] 2.6 **Cascade on person delete** — deleting the person removes its `person_images` rows (ADR-037). *(repo `TestPersonImagesCascade`.)*
- [ ] 2.7 **Serving returns real-or-placeholder, version-stamped** — filled role → real image + `?v=` + `Cache-Control: …immutable`; empty role → resolved placeholder (not 404); unknown role → 400; unknown person → 404; replace emits a **new `?v=`** (F24.6, T7). *(api `TestServePersonImage`, `TestServeReplaceBustsVersion`.)*
- [ ] 2.8 **Upload validation + owner-gating** — multipart `POST …/image`: missing field/bad role/oversized/bad image → 400; `MaxBytesReader` enforced; **401 without token when gated**, 201 with it; `DELETE`/reorder/promote likewise gated (F24.7/F24.11, T2/T6). *(api `TestUploadValidated`, `TestPersonImageEndpointsGated`.)*
- [ ] 2.9 **Enrichment asset download through SSRF guards + normalization** — a provider asset URL is fetched via the allowlist + no-cross-host-redirect + size-cap, normalized, stored with provenance (`source=enrichment`); a hostile URL (internal/redirect-to-internal/oversized/non-image) is refused, nothing written (F24.10, T3). *(enrich `TestEnrichDownloadsAsset`, `TestEnrichAssetSSRFRefused`.)*
- [ ] 2.10 **Path safety** — no request value (role/person/filename) is concatenated into a filesystem path; a traversal attempt still resolves to a server-assigned path (T2). *(personimage `TestImagePathServerAssigned`.)*
- [ ] 2.11 **`personImageURL` cache-bust** — builds `…/image/{role}?v=n`; omits `?v=` when version absent (F24.6). *(web `api.test.ts`.)*
- [ ] 2.12 **`svelte-check`** passes with the new image types + page/component changes.
- [ ] 2.13 **Token-discipline guard** empty over the new components: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` (a `rounded-full` avatar, if used, is the only intentional fixed radius).

---

## 3. Agent — drive the running app

**Display & placeholders**

- [ ] 3.1 `/people` cards show a **1:1 headshot** well above the name; with no image it's the themed placeholder, not a broken-image box; the grid does **not** reflow as images load (box reserved).
- [ ] 3.2 `/people/[id]` shows a **16:9 banner** hero and a **1:1 avatar** by the name; empty → placeholders; the Aliases (F23) and Enrichment (F22) panels still render below, unchanged.
- [ ] 3.3 `/media/[id]` renders each credited person as a **2:3 poster card** linking to `/people/{id}`; missing poster → placeholder; name reads beneath.
- [ ] 3.4 **Placeholder bucket** follows enriched gender: the female-gender person shows the female silhouette, the male one the male, and the **no-gender** person the **neutral** one (F24.5). Switching skins swaps the placeholder art.
- [ ] 3.5 **Error fallback**: force a real image URL to 404 (e.g. bad `?v=`) → the frame shows the **placeholder**, never a broken-image glyph or a 404 box.

**Owner gating & controls**

- [ ] 3.6 **Owner**: upload affordances appear on the banner/avatar (hover or Edit) and an **"Add image"** tile in the gallery; each gallery item shows delete + set-as-{role} + reorder affordances.
- [ ] 3.7 **Non-owner** (token set, none entered): **every** upload/delete/promote/reorder control is **absent from the DOM** (not merely hidden); images + gallery still display read-only.

**Upload / replace / delete**

- [ ] 3.8 Upload a valid JPEG to the **headshot** slot → `POST …/image` (multipart) fires; on success the avatar swaps to the real image and the URL gains a **`?v=`**; the list card reflects it after reload.
- [ ] 3.9 Upload a **second** headshot → the slot **replaces** (still one headshot); the served URL shows a **new `?v=`** and the old image is no longer served.
- [ ] 3.10 Upload a **renamed non-image** / **oversized** file → **400**; an inline `text-warn` message in **words** (not color-only); nothing added; control re-enabled.
- [ ] 3.11 Add gallery extras up to **20**; the **21st** add is blocked — the "Add" tile is disabled with a `text-warn`/`border-warn` "Gallery is full (20 max)." message; the server also returns 400 if forced.
- [ ] 3.12 Delete a core image → it reverts to the **placeholder**; delete a gallery extra → it leaves the gallery; on a forced failure the item is **restored** with an inline error.

**Promote (P1)**

- [ ] 3.13 Owner picks a gallery extra → **"Set as poster"** opens a **2:3 crop editor**; saving creates/replaces the `poster` from a **cropped copy**; the **original extra still appears** in the gallery (it's a copy, F24.15); no orphaned files.

**A11y & network**

- [ ] 3.14 Every image is an `<img>` with a meaningful `alt` (person name; placeholders read "No photo of {name}"), never an empty/`alt=""` broken box.
- [ ] 3.15 Owner action controls are real `<button>`s with `aria-label` (e.g. "Set as headshot", "Delete image", "Reorder"); the gallery offers a **keyboard** reorder (move up/down), not drag-only; the crop modal is `role="dialog" aria-modal="true"`, focus-trapped, Esc-closable, **returns focus** to its trigger.
- [ ] 3.16 Real-image responses carry `Cache-Control: public, max-age=…, immutable`; placeholder responses are cacheable; no request sends the admin token on a **public** image GET.

---

## 4. Human — needs your eyes (all three skins)

> **How to run this:** open the app in a browser. In the header there's a **skin picker** — run every item below **three times**, once in each skin: **Cinémathèque**, **Broadcast**, **Brutalist**. Visit **People** (list), then **a person's page**, then **a video that credits people**. You're checking it *looks right* and reads well — not that buttons work (the agent already checked that).

- [ ] 4.1 **Headshots read as portraits** on the people list — the square sits cleanly in the card, the name/count below are uncrowded, and nothing reflows as you scroll. *(token ref: `bg-surface-2` well, `text-ink` name.)*
- [ ] 4.2 **The person page banner** looks intentional in each skin — the wide 16:9 image (or placeholder) spans the content, the headshot tucks against it by the name without colliding with the Aliases panel; spacing feels even. *(token ref: `.portrait-frame--16x9` / `--1x1`.)*
- [ ] 4.3 **Placeholders look designed, not broken** — the silhouette + skin treatment (warm letterbox on Cinémathèque, scanline-tinted on Broadcast, hairline/square on Brutalist) reads as a deliberate "no photo yet", and the **neutral** one isn't jarringly different from the gendered ones. The placeholder glyph is clearly visible but quiet (a muted tone), never the bright accent. *(token ref: `text-muted` glyph on `bg-surface-2`.)*
- [ ] 4.4 **Frame corners match the skin** — softly rounded on Cinémathèque, **square** on Broadcast and Brutalist. (A round `rounded-full` headshot, if used, stays round everywhere by design.) *(token ref: `rounded-theme`.)*
- [ ] 4.5 **Poster cards on the video page** look like a cast strip — the 2:3 cards line up, names read beneath, and they clearly invite a click; placeholders among real posters don't look out of place.
- [ ] 4.6 **Owner edit affordances** are legible and use the skin's highlight color (lime/cyan/gold), and the little camera/upload/✕ marks aren't lost on the image — but they stay out of the way until you hover/focus. *(token ref: `text-accent` controls; never `--warn` for normal actions.)*
- [ ] 4.7 **Error wording** (upload a bad/oversized file; fill the gallery to 21) — the message is **words you can read** in the **error color (red/orange), clearly different from the highlight** — this separation matters most on Brutalist (bright lime highlight). *(token ref: `text-warn`/`border-warn`, distinct from `--accent`.)*
- [ ] 4.8 **The crop editor** (promote a gallery image to poster) is usable and readable — you can see the 2:3 frame you're cropping to, the zoom control is obvious, and Save/Cancel read clearly with the skin's button styles.
- [ ] 4.9 **Names render fully** under avatars/posters, including **accented and non-Latin** names ("Beyoncé", "宮崎駿") — actual characters, **no boxes/▯/garbled glyphs** — in all three skins (the Broadcast/Brutalist fonts are blocky).
- [ ] 4.10 **Narrow window** — drag the window narrow: the gallery reflows to fewer columns, the banner keeps its shape, the avatar/name stack gracefully, and poster cards scroll rather than squish.
- [ ] 4.11 **Reduced motion** — with "reduce motion" on, images appear without a distracting shimmer/fade, and the gallery reorder/crop modal don't animate jarringly.
- [ ] 4.12 **Overall it looks intentional** — across list, person page, and video page, the images make the app feel like a media library, nothing collides with existing badges/counters, and the three skins each feel coherent rather than an afterthought.
