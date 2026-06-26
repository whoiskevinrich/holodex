# QA Checklist: Person-page polish (parallax banner · inline poster · list scroll-restore)

Design handoff [`person-page-polish-handoff.md`](person-page-polish-handoff.md) · spec
[`people-images.md`](../specs/people-images.md) (F25.26–28 follow-ups) · [ADR-038](../architecture/ADR-038-person-images.md) /
[ADR-032](../architecture/ADR-032-browse-state-preservation.md).

**Legend** — verifier tags: **[smoke]** trivially confirmable the app runs · **[agent]** verified
programmatically this session (DOM inspection / svelte-check) · **[human]** needs a person's eyes (live
motion and per-skin look can't be captured in the headless preview — it produces **no animation frames**,
so scroll-driven motion and screenshots don't render there).

---

## Setup

- **0.1** Backend on `:7800` (`MEDIA_PATH=<dir> go run ./cmd/holodex --host 127.0.0.1`), frontend on
  `:5173` (`npm --prefix web run dev`), open **http://localhost:5173/**. Use a media set that produces
  people (e.g. the AMV test copy) so `/people` is populated.
- **0.2** To see the **owner** affordances (Replace overlays), run with an admin session; to test the
  **visitor** path, use a non-owner session.

## Smoke

- [x] **1.1 [smoke]** `npm run check` (svelte-check) passes — **0 errors** (2 pre-existing warnings in an
  unrelated file, `WritebackFormDialog.svelte`).
- [x] **1.2 [smoke]** Open a person detail page (`/people/{id}`) — it renders the hero (banner + headshot +
  poster) with no console errors.

## Agent (verified this session via DOM inspection)

- [x] **2.1 [agent]** Banner is **5:2** (`aspect-ratio: 5 / 2`), **487px** tall on a 1217px-wide column,
  `max-height: 540px` — doubled from the old 5:1 / 270px cap.
- [x] **2.2 [agent]** Banner `<img>` is rendered at **140%** of frame height (overhang for parallax travel)
  under `prefers-reduced-motion: no-preference`.
- [x] **2.3 [agent]** Setting `--banner-shift` drives the image transform (manual `-24%` →
  `translateY(-162px)`); PersonBanner **seeds** the correct initial shift on mount (`≈ -13.5%` at the
  banner's load position).
- [x] **2.4 [agent]** `.crop-frame--banner` was updated to **5:2** in lockstep with the rendered banner.
- [x] **2.5 [agent]** The **poster (2:3)** renders inline in the hero row, to the right of the 1:1 headshot,
  bottom-aligned and overlapping the banner's lower edge (96×144 at `sm:w-24`). The old standalone "Replace
  poster" button is gone (hero row has exactly the avatar + poster).
- [x] **2.6 [agent]** **Scroll restore:** scroll `/people` to 260px → open a person → `← All people` →
  back at **260px** (exact). 
- [x] **2.7 [agent]** No hardcoded styling introduced (token scan over the changed components is clean —
  no `zinc-*`/`sky-*`/hex/`rounded-lg`).

## Human (needs eyes — not capturable in the headless preview)

- [ ] **3.1 [human] Parallax drifts on scroll.** Open a person page. Slowly scroll the page down and back
  up while watching the **banner image**. The picture should glide **vertically a little slower than the
  page** (a depth/parallax feel) — and the band should **stay fully covered** the whole time (no blank
  edge creeping in at the top or bottom of the banner). It should feel subtle, not jumpy.
- [ ] **3.2 [human] Reduced motion = no parallax.** Turn on your OS "reduce motion" setting (Windows:
  Settings → Accessibility → Visual effects → Animation effects **off**), reload the person page, and
  scroll. The banner should now be **completely still** (a normal cropped image) — no drift.
- [ ] **3.3 [human] All three skins.** Switch skin via the header picker and re-check a person page in
  **Cinémathèque, Broadcast, and Brutalist**. In each: the taller banner reads as a backdrop (not a wall),
  the headshot + poster sit cleanly over its lower edge without colliding, and corners match the skin
  (rounded in Cinémathèque; square in Broadcast/Brutalist). In **Broadcast**, the faint scanline wash
  should sit over the banner and poster (they share `.portrait-frame`).
- [ ] **3.4 [human] Poster visibility — visitor vs owner.** As an **owner**, a person with **no** uploaded
  poster still shows the poster slot (a themed placeholder) with an **Edit** overlay; clicking **Edit**
  opens the file picker and an upload replaces it. As a **visitor** (non-owner), a person with **no**
  poster shows **no** poster card at all (no placeholder clutter); a person **with** a poster shows it.
- [ ] **3.5 [human] Scroll restore feels right.** On a long `/people` list, scroll well down, open a
  person, then use the in-app **← All people** link (and separately the browser **Back** button) — both
  should drop you back roughly where you were, not at the top. Switching the **sort** (name ⇄ count) and
  then returning should **not** restore an old position (the list reordered) — top is correct there.
- [ ] **3.6 [human] Banner replace still works.** As owner, **Replace banner** uploads a new image; after
  it lands, the new banner shows at the taller 5:2 size and the crop you chose matches what's displayed.
