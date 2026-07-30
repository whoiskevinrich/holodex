# Spec: Installable PWA + instance-configurable branding

**Status**: Draft
**Phase**: Post–Phase 3 backlog (standalone user-facing feature; no phase dependency)
**Owner**: Project owner
**Date**: 2026-07-30
**Feature block**: **F51** — make the web SPA installable/pinnable as a standalone app on
iOS, Android, and desktop Chrome/Edge, and let a self-hosted instance's operator swap in
their own app icon + favicon via config. No offline caching, no push notifications, no
mobile-nav redesign — installability and branding only.

**Issue**: not yet created — no HOLODEX issue exists for this feature. Recommend creating
an Epic (F51) with two Stories (Track A install, Track B branding) before implementation
starts; see Timeline Considerations.

**New ADRs required**: none. Track B reuses the serving *pattern* established by
[ADR-059](../architecture/ADR-059-self-hosted-provider-brand-icons.md) (on-disk normalized
cache, served from the app's own origin, re-linked at boot/config-reload) rather than
introducing a new cross-cutting decision. One deliberate deviation from ADR-059 is called
out in FR6 below (no DB table, since cardinality is 1 — see Open Questions).

**Depends on**:
- `holodex.yaml` config load + reload path (main config file, [ADR-014](../architecture/ADR-014-configuration-and-data-layout.md))
- The existing `web/src/app.html` SvelteKit shell (currently references a `favicon.png`
  that does not exist anywhere in the repo — confirmed via search; there is no static icon
  asset in `web/` today)
- `docs/reference/configuration.md`'s existing per-domain `##` section format
- `internal/providericon` / `internal/personimage` (ADR-059's normalize/store pattern) as
  prior art — **not** a direct dependency, since that package normalizes-and-re-encodes a
  single image and does **not** resize to multiple target dimensions, which this feature
  needs (see FR6 and Open Questions)

---

## Problem Statement

Holodex's web SPA has no PWA manifest, no installability metadata, and no static icon
assets at all — the only reference to a favicon in `app.html` points at a file that isn't
in the repo. Anyone (owner or self-hosting operator) who wants Holodex pinned to a phone
home screen or installed as a desktop app today gets, at best, a browser-generated
screenshot-icon bookmark. Because Holodex is publicly distributed via GHCR for other
operators to self-host, a hardcoded single icon would also be wrong for anyone whose
instance isn't branded as "Holodex" — there's no way for an operator to make an installed
copy look like *their* deployment. Neither gap blocks core functionality, but both are
gaps in "this should just exist" table-stakes polish for a project that's otherwise a
complete, self-hostable product.

## Goals

1. **Installable on the platforms that matter** — a user can add Holodex to their home
   screen (iOS Safari, Android Chrome) or install it as a desktop app (Chrome/Edge) and get
   a real app icon and standalone window, not a URL bookmark.
2. **Every instance ships a usable default** — out of the box, with zero config, Holodex
   presents a complete, valid icon set (manifest icons, favicon, apple-touch-icon). No
   instance ever shows a broken or missing icon.
3. **An operator can rebrand their instance's icon without touching frontend code** — set
   one config value, restart or reload, and the icon set (browser tab favicon + installed
   app icon) reflects their own image.
4. **Documented like every other config domain** — an operator can find and use this
   without reading source, via `docs/reference/configuration.md`.
5. **No regression to existing behavior** — an instance that sets no branding config
   behaves identically to the shipped default; this is additive only.

## Non-Goals

- **Offline caching of the app shell or video/library data.** A service worker sized for
  "cache the UI shell" is a different, larger problem than "cache a multi-GB media
  library," and no offline requirement was raised. *(Ruled out during brainstorming —
  storage-quota problem, not this spec's problem.)*
- **Push notifications.** Needs a push server + VAPID keys and is unreliable on iOS
  Safari; no one asked for it. *(Ruled out during brainstorming.)*
- **A bespoke mobile navigation shell or kiosk-mode redesign.** Installability doesn't
  require changing the SPA's existing responsive layout or IA. *(Ruled out during
  brainstorming — separate problem from "is it installable.")*
- **A separate lightweight companion view** (e.g. `/m`) instead of manifest-wrapping the
  existing SPA. *(Ruled out during brainstorming — more build cost for no stated benefit.)*
- **Configurable app name/short_name, theme color, or other manifest fields beyond
  icon/favicon.** Only icon + favicon were requested as operator-configurable; other
  manifest fields ship as fixed Holodex defaults for v1. *(Could be a P2 — see Future
  Considerations.)*
- **Live-updating already-installed home-screen icons.** iOS/Android generally snapshot a
  PWA's icon at install time; changing the config after install typically requires the
  user to uninstall/reinstall to see the new icon. This spec documents that limitation
  (FR9) rather than attempting to solve it — it isn't solvable from the server side.

---

## Users & Value

- **Any user (owner or viewer) of an instance**: can pin/install the app and get a real
  icon instead of a bookmark screenshot, on whichever device they use.
- **Self-hosting operator**: can make an installed/pinned copy of their instance look like
  *their* deployment instead of generic Holodex branding, using the same
  config-file-and-restart workflow they already use for every other Holodex setting.
- **Operator provisioning a fresh instance with no branding config**: unaffected — gets the
  shipped default icon set and a fully installable app with zero setup.

---

## Functional Requirements

### Must-Have (P0) — Track A: installability

#### FR1 — Default icon asset set

Ship a default, complete icon source (a simple mark — not a commissioned design pass; see
Open Questions) rendered into the full required set: manifest icons (192×192, 512×512, and
a maskable 512×512 variant), `apple-touch-icon` (180×180), and a multi-resolution
`favicon.ico`/`favicon.png`. This replaces the currently-dangling `favicon.png` reference
in `app.html`.

- **Given** a fresh instance with no branding config, **when** the app loads, **then** the
  browser tab shows a real favicon (not a broken-image icon) and the manifest resolves a
  full icon set.

#### FR2 — Web app manifest

A `manifest.webmanifest` (or `manifest.json`) served at the SPA root with `name`,
`short_name`, `icons` (the FR1 set), `start_url`, `display: "standalone"`, `theme_color`,
and `background_color`. Linked from `app.html` via `<link rel="manifest">`.

- **Given** the manifest is served, **when** validated against the installability
  criteria for Chrome/Edge (name, icons, `start_url`, `display`), **then** it passes with
  no warnings.

#### FR3 — iOS installability meta tags

`app.html` gains `apple-touch-icon` link(s), `apple-mobile-web-app-capable`,
`apple-mobile-web-app-status-bar-style`, and `apple-mobile-web-app-title`, since iOS
Safari does not read the web manifest for these.

- **Given** an iOS Safari user taps Share → Add to Home Screen, **then** the resulting icon
  is the FR1 apple-touch-icon (not a page screenshot) and opening it launches in standalone
  mode (no browser chrome).

#### FR4 — Cross-platform install verification

Manual verification (see Test Notes) that install/pin works end to end on: Chrome desktop
(install prompt via omnibox), Android Chrome (Add to Home Screen → standalone launch), and
iOS Safari (Add to Home Screen → standalone launch).

- **Given** each of the three platforms above, **when** the install/pin flow is completed,
  **then** the app opens in standalone display (no URL bar) with the correct icon and name.

### Nice-to-Have (P1) — Track B: instance-configurable branding

#### FR5 — `holodex.yaml` branding config key

A new config key (exact name TBD during implementation — see Open Questions; e.g.
`branding.icon_path`) accepting a path to a local image file. Absent or unreadable → falls
back to the FR1 default (FR8), never an error at boot.

- **Given** `branding.icon_path` is unset, **then** the instance behaves exactly as an
  instance with no branding section at all (Goal 5).

#### FR6 — Normalize-on-boot/reload

At boot and on config-reload, if the branding config is set, generate the full icon set
(FR2's manifest sizes + FR3's apple-touch-icon + favicon) from the configured source image
and write it to an on-disk served directory — mirroring ADR-059's shape (a sole-writer
relink function triggered at the same lifecycle points as `RelinkProviderIcon`). **Unlike**
ADR-059, this does **not** add a DB table: there is exactly one branding icon per instance
(cardinality 1, not per-provider), so the served URL is cache-busted with a content hash of
the source file rather than a DB row id — a `provider_icons`-style table would be
unnecessary state for a single value that config-reload already re-derives from disk.

- **Given** `branding.icon_path` points at a valid image, **when** the instance boots or
  config reloads, **then** the full icon set is (re)generated and the manifest/favicon URLs
  include a hash that changes only when the source image's bytes change.
- **Given** `branding.icon_path` points at a missing or unreadable file, **when** the
  instance boots, **then** it logs a warning and falls back to the FR1 default rather than
  failing to start.

#### FR7 — Serve from own origin

Generated icons are served from the app's own origin (not the configured path directly),
matching ADR-059's non-hotlinking principle — consistent with how provider brand icons are
already served rather than referencing the provider's CDN.

#### FR8 — Default fallback

Codified as acceptance criteria on FR5/FR6 above: no code path exists where an instance
serves a broken or missing icon, whether branding config is absent, invalid, or points at
an unreadable file.

#### FR9 — Configuration documentation

A new `## App icon & branding` section in `docs/reference/configuration.md`, matching the
existing per-domain format (one `##` section per config area, each with a YAML example —
see `## Presentation`, `## Person images` for the house style). Includes: the config key
and its default, a worked YAML example, supported image formats/recommended source
dimensions, and an explicit callout of the "already-installed icons don't live-update"
caveat from the Non-Goals section, so operators don't file a bug expecting otherwise.

- **Given** an operator reads only this doc section, **then** they can set the config
  key correctly without reading source code.

### Future Considerations (P2)

- **P2-a — Configurable app name/short_name/theme_color.** Only icon/favicon were
  requested; extending the same config section to cover manifest text/color fields is a
  natural follow-up with the same mechanism.
- **P2-b — Manifest `shortcuts`.** Long-press/right-click quick actions on the installed
  icon (e.g. "Browse", "Search") — raised during brainstorming as a nice extra tied to
  "pin where I want," deferred since it wasn't asked for explicitly.
- **P2-c — Maskable-icon safe-zone validation.** Warn the operator (log or `/owner` surface)
  if their source image doesn't have enough padding for Android's maskable-icon safe zone,
  rather than silently shipping a badly-cropped icon.

---

## Acceptance Criteria

1. A fresh instance with no branding config is installable/pinnable on Chrome desktop,
   Android Chrome, and iOS Safari, showing the default icon and launching standalone.
2. `app.html` no longer references a nonexistent `favicon.png`; the browser tab shows a
   real icon on every page.
3. Setting the branding config key to a valid image and restarting (or triggering
   config-reload) changes the served favicon and manifest icons; unsetting it (or an
   invalid path) falls back to the default with only a warning log, never a startup
   failure.
4. The served icon URLs are cache-busted by source-file content hash, not by request-time
   parameters an operator has to manage manually.
5. `docs/reference/configuration.md` has an `## App icon & branding` section with a working
   YAML example, in the same format as its sibling sections.
6. No DB migration is introduced by this feature (FR6's cardinality-1 simplification).

---

## Test Notes (for `/testing-strategy`)

- **Manifest validity** — automated check that the served manifest is valid JSON and
  satisfies Chrome's installability criteria (name, icons array with required sizes,
  `start_url`, `display`).
- **Fallback behavior** — unit test: missing/unreadable configured icon path → boot
  succeeds, warning logged, served icons equal the FR1 default bytes.
- **Content-hash cache-busting** — unit test: changing the source file's bytes changes the
  served URL's hash suffix; re-running normalize on unchanged bytes produces the same hash
  (idempotent, no spurious cache invalidation).
- **Manual cross-platform install** (FR4) — Chrome desktop install prompt, Android Chrome
  Add to Home Screen, iOS Safari Add to Home Screen; verify icon, standalone display, and
  app name on each.
- **Docs** — the YAML example in `docs/reference/configuration.md` is copy-pasteable and
  matches the actual config key/shape shipped.

---

## Open Questions

- **[engineering, blocking for FR6]** No image-resizing capability exists in the codebase
  today — `internal/providericon`/`internal/personimage` normalize and re-encode a single
  image but do not resize to multiple target dimensions. FR6 needs real multi-size
  resizing (manifest icons at 192/512, maskable 512, apple-touch-icon 180, favicon). Needs
  a decision on approach: add a Go image-resize dependency (e.g. `golang.org/x/image` or
  `disintegration/imaging`) vs. some other mechanism. Affects implementation estimate for
  Track B.
- **[design, blocking for FR1]** What should the *default* Holodex icon actually look like?
  No source art exists in the repo. Needs at least a simple placeholder mark (wordmark or
  monogram) before FR1/FR2 can ship — worth timeboxing rather than blocking on a full brand
  exercise.
- **[engineering, non-blocking]** Exact `holodex.yaml` config key name/shape for FR5 (e.g.
  `branding.icon_path` as a nested key vs. a flat `app_icon_path`) — resolve during
  implementation against existing config-key conventions.
- **[engineering, non-blocking]** Confirm whether config-reload already covers `holodex.yaml`
  broadly or only specific sections (metadata-sources.yaml's reload is well-established per
  ADR-059/-033; verify the same reload path applies to the main config file for FR6).

---

## Timeline Considerations

No hard deadline. Two independently shippable tracks — Track A (P0, install/pin with a
default icon) has no dependency on Track B (P1, instance branding) and could ship alone if
the FR1 design open question takes longer to resolve than the mechanical manifest/meta-tag
work. Recommend filing as one Epic (F51) with two Stories in Jira before implementation
begins, per this repo's branch↔Jira linkage convention, and running `/architecture` only if
the FR6 resize-dependency decision turns out to be more architecturally significant than
expected (unlikely, but the open question above is why it isn't preemptively ruled out
here).
