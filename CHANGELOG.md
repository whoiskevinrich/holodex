# Changelog

## [1.3.0](https://github.com/whoiskevinrich/holodex/compare/v1.2.0...v1.3.0) (2026-06-29)


### 🚀 Features

* **enrich:** deduplicate enrichment photos by content hash (F34, ADR-050) ([#60](https://github.com/whoiskevinrich/holodex/issues/60)) ([5f6c3ea](https://github.com/whoiskevinrich/holodex/commit/5f6c3eac0fb86d23009290eb6c61448b33e8e2ad))
* **owner:** consolidate owner tooling into /owner hub + nav split (F35) ([#62](https://github.com/whoiskevinrich/holodex/issues/62)) ([d7faf61](https://github.com/whoiskevinrich/holodex/commit/d7faf6103a3ffdb6d8db7fb2119e49c1f9989314))

## [1.2.0](https://github.com/whoiskevinrich/holodex/compare/v1.1.0...v1.2.0) (2026-06-29)


### 🚀 Features

* **enrich:** don't overwrite owner-set person images on enrichment (F33, ADR-049) ([#57](https://github.com/whoiskevinrich/holodex/issues/57)) ([35b03bd](https://github.com/whoiskevinrich/holodex/commit/35b03bd3658c05130e53291d0cb6f9cffbd7ffcb))


### 📚 Documentation

* ADR-049 (+ index) and people-images spec F25.31 addendum. ([35b03bd](https://github.com/whoiskevinrich/holodex/commit/35b03bd3658c05130e53291d0cb6f9cffbd7ffcb))

## [1.1.0](https://github.com/whoiskevinrich/holodex/compare/v1.0.0...v1.1.0) (2026-06-29)


### 🚀 Features

* **admin-mode:** Admin Mode toggle to hide owner UI for visitor view (F29) ([#54](https://github.com/whoiskevinrich/holodex/issues/54)) ([e18ddd1](https://github.com/whoiskevinrich/holodex/commit/e18ddd1799fd8a94ca4556297831baba82798809))
* **metadata:** granular curation & merge + durable write queue (F30, ADR-048) ([#55](https://github.com/whoiskevinrich/holodex/issues/55)) ([bae434c](https://github.com/whoiskevinrich/holodex/commit/bae434c3d378aecc142c62a9efbc30ee4176bd15))
* **refresh:** per-item Refresh Metadata — forced re-extract + re-enrich (F31) ([#56](https://github.com/whoiskevinrich/holodex/issues/56)) ([7a5c03c](https://github.com/whoiskevinrich/holodex/commit/7a5c03c0648a986bfd2a4ba2f320eeaba232be86))


### ⚙️ CI / Build

* stop caching binfmt image to silence cache-reserve warning ([#52](https://github.com/whoiskevinrich/holodex/issues/52)) ([541ed4e](https://github.com/whoiskevinrich/holodex/commit/541ed4e0c52c0934c339b31f284805ff20ccabfd))

## 1.0.0 (2026-06-28)


### 🚀 Features

* **activity:** /status page + header indicator UI (F21.4-6) ([e1a9581](https://github.com/whoiskevinrich/holodex/commit/e1a95819f9ea2756bfa83b4b633de57ff0db604d))
* **activity:** backend read-model + 30-day job history (F21.1-3, ADR-028) ([1882a89](https://github.com/whoiskevinrich/holodex/commit/1882a8953b81640532d2daeedc26407c2854ea1d))
* **activity:** owner gating seam — ADMIN_TOKEN + requireOwner (F21.7, ADR-030) ([e7495f8](https://github.com/whoiskevinrich/holodex/commit/e7495f8e65c0d297999a04e12f8d74d89164d2fc))
* **api:** admin rescan endpoint + Prometheus /metrics (F13) ([6a40aea](https://github.com/whoiskevinrich/holodex/commit/6a40aeae52642e23b64c25478d7d52163832281d))
* **auth:** persist owner session across reloads via HttpOnly cookie (ADR-045) ([#51](https://github.com/whoiskevinrich/holodex/issues/51)) ([8efa124](https://github.com/whoiskevinrich/holodex/commit/8efa124e95b918b46fc086cad7f6fa7136b76122))
* **browse:** Phase 2 browse polish — sort, codecs, shelf, keyboard, responsive (F12) ([164e4b2](https://github.com/whoiskevinrich/holodex/commit/164e4b24e13cdc0a36cf5455becfe576e175a511))
* **config:** load a local .env for development config (ADR-027) ([0352e4c](https://github.com/whoiskevinrich/holodex/commit/0352e4cd67d273e0636977c6e57c640e727cdd77))
* **config:** load a local .env for development config (ADR-027) ([96f469d](https://github.com/whoiskevinrich/holodex/commit/96f469dc5cdb409f104e6bd870e170bb9f7e15be))
* **demo:** seeded showcase corpus generator ([dbda0d5](https://github.com/whoiskevinrich/holodex/commit/dbda0d5fd43499e3d17c5b88524fda6768d84809))
* **dist:** publish GHCR image + pull-based compose for self-hosting ([5d4554f](https://github.com/whoiskevinrich/holodex/commit/5d4554fcf819520dbd106b04e2d157e52d3ba8de))
* **dist:** publish GHCR image + pull-based compose for self-hosting ([8b83d2f](https://github.com/whoiskevinrich/holodex/commit/8b83d2f75795fcaa03c77540f1180dc5a9f5749e))
* **enrich:** metadata source plugins — People enrichment (F22, ADR-033) ([ddb6c1c](https://github.com/whoiskevinrich/holodex/commit/ddb6c1c86fd006b91290f5919d19a06798deb18c))
* **enrich:** metadata source plugins — People enrichment slice (F22, ADR-033) ([90b3b4a](https://github.com/whoiskevinrich/holodex/commit/90b3b4a0cbea2eb4eccfd71b91423d52d4519239))
* **enrich:** record enrich runs in activity history (F22.6b) ([2b568fc](https://github.com/whoiskevinrich/holodex/commit/2b568fc07c699ca4672e5a6cc2c94053210a9a53))
* **enrich:** render Website URLs as clickable new-tab links ([#41](https://github.com/whoiskevinrich/holodex/issues/41)) ([34a5bb7](https://github.com/whoiskevinrich/holodex/commit/34a5bb7bd51e9bb2e4439f2889374c7eec81a196))
* **enrich:** video enrichment (F26) + unified resolver (F27) + metadata writeback (F28) ([#35](https://github.com/whoiskevinrich/holodex/issues/35)) ([66d2a04](https://github.com/whoiskevinrich/holodex/commit/66d2a04ac3b2a21850e2893a6dab35acb5fa8e86))
* **mcp:** MCP server with 4 tools over stdio + HTTP/SSE (F10) ([77ae090](https://github.com/whoiskevinrich/holodex/commit/77ae090167bb937463fba2fb1a520321bc0d835a))
* **media:** soft-delete media with grace-period purge + Trash (F24) ([#34](https://github.com/whoiskevinrich/holodex/issues/34)) ([6d3cb98](https://github.com/whoiskevinrich/holodex/commit/6d3cb986f738f4548cbcb7b0dbec0cc5863bea27))
* **metadata:** configurable metadata field mapping (F20) ([4267654](https://github.com/whoiskevinrich/holodex/commit/4267654ef924d869f26b3d422f4df3903feb5fc0))
* **people:** A–Z jump-nav on people list + Tags-above-People on video page ([9d014e1](https://github.com/whoiskevinrich/holodex/commit/9d014e16603ba41ed439c9bf9803af5673e9c11f))
* **people:** crop editor mouse-wheel zoom + WYSIWYG crop fidelity (F25.15) ([aede191](https://github.com/whoiskevinrich/holodex/commit/aede191d428a196dfa7b24c1169a6ef9a51bd6ab))
* **people:** person aliases & merge (F23) ([88fba41](https://github.com/whoiskevinrich/holodex/commit/88fba4114ce176228eadcdbf6f6da41f1b423b08))
* **people:** person aliases & merge (F23) ([07df521](https://github.com/whoiskevinrich/holodex/commit/07df5212faaa51c42022e270b62e3eed532b772b))
* **people:** person images with themed placeholders (F24) ([c7f1f45](https://github.com/whoiskevinrich/holodex/commit/c7f1f45db31d209824e3477cde61725b94902fc1))
* **people:** person images with themed placeholders (F25) ([8d29713](https://github.com/whoiskevinrich/holodex/commit/8d29713ec1ec862c0b0d9f1a9ced3801b645b5f1))
* **people:** rule-of-thirds guide overlay in the crop editor ([c6f28c8](https://github.com/whoiskevinrich/holodex/commit/c6f28c8b41308400c1a199cc3909c21795b6b22e))
* **person-images:** person-page polish + gallery-cap hardening (F25.23-28) ([#44](https://github.com/whoiskevinrich/holodex/issues/44)) ([5e6a72a](https://github.com/whoiskevinrich/holodex/commit/5e6a72a76585a6275e4d7d36e7d3713f2cd821ae))
* **person-images:** show banner only when set (F25.30) ([#47](https://github.com/whoiskevinrich/holodex/issues/47)) ([e8818c8](https://github.com/whoiskevinrich/holodex/commit/e8818c8c341f599f9167bdd836678ea2c34cb055))
* Phase 1 MVP — indexer, browse/search UI, and 3-skin theming ([36e8bd8](https://github.com/whoiskevinrich/holodex/commit/36e8bd8f24a56a7d5544d20819158db190bd46b0))
* **quick-wins:** overlay fix, search history, fluid Back, "More with…" shelves ([5b1a83d](https://github.com/whoiskevinrich/holodex/commit/5b1a83dc50de5b80c867312f39ad00be2ac4dfcb))
* **quick-wins:** overlay fix, search history, fluid Back, "More with…" shelves ([296dbe1](https://github.com/whoiskevinrich/holodex/commit/296dbe1b1d830a01c1ed1b9ce69a554482c92392))
* **site:** interactive landing page with live skin-switcher ([4f0151b](https://github.com/whoiskevinrich/holodex/commit/4f0151be567384aba06008a6e6a4ea48b32331a8))
* **sort:** sticky per-page sort + seeded Random (ADR-045) ([#48](https://github.com/whoiskevinrich/holodex/issues/48)) ([f483a0b](https://github.com/whoiskevinrich/holodex/commit/f483a0b94c49dad08f1b5a198372af174366b844))
* System Activity — under-the-hood view (F21) ([6475b5a](https://github.com/whoiskevinrich/holodex/commit/6475b5a937452c63af486816d9fdf02be1818f8e))
* **thumbnails:** tiered cover-art pipeline (ADR-009) ([eab49dc](https://github.com/whoiskevinrich/holodex/commit/eab49dcd206683d36b86ccdea43808f9b0ed5f5d))
* **thumbnails:** tiered cover-art pipeline (ADR-009) ([59798d1](https://github.com/whoiskevinrich/holodex/commit/59798d1bfb5257db7da188fa36799b19aca5629e))


### 🐛 Bug Fixes

* **deps:** bump go-chi/chi to v5.3.0 (GHSA-vrw8-fxc6-2r93) ([3a9b387](https://github.com/whoiskevinrich/holodex/commit/3a9b387d5d97b6da7d13a67852146cc3bb2d9cf5))
* **deps:** bump go-chi/chi to v5.3.0 (GHSA-vrw8-fxc6-2r93) ([9c34727](https://github.com/whoiskevinrich/holodex/commit/9c3472790585e178b870d697c2999db04bd27023))
* **enrich:** auto-search picker on open; landing count + Recently-Added toggle ([728da0a](https://github.com/whoiskevinrich/holodex/commit/728da0a2f0b5f9e7bbbb3f1f47e8fa2bbfd431cd))
* **enrich:** picker focus trap + clearer highlight; slow-stub for QA 3.18/3.21 ([0ff7fde](https://github.com/whoiskevinrich/holodex/commit/0ff7fde608dce96bf04d63e8239ce4453d82a756))
* **enrich:** Tab reaches picker results (roving tabindex) ([d4a2d72](https://github.com/whoiskevinrich/holodex/commit/d4a2d728058d352da49f07608170e43eff1f75fe))
* **env:** restore HOLODEX_MEDIA_PATH in .env.example ([dffb63a](https://github.com/whoiskevinrich/holodex/commit/dffb63aa7a7bb14d36dbb2ef0ff122c934564cd9))
* **people:** F25 post-merge QA + review fixes ([35198ce](https://github.com/whoiskevinrich/holodex/commit/35198cecd1731fe8b1801ed167c886548f132ee0))
* **people:** global search returns a matched person's media (F23) ([fa8e352](https://github.com/whoiskevinrich/holodex/commit/fa8e352844eb9dca20751ad06c0fce7de09b5ce0))
* **people:** person-page image QA — hero band, gallery row, crop preview ([8ec68ca](https://github.com/whoiskevinrich/holodex/commit/8ec68ca89620dea91c86b79229b02e92592966ed))
* **people:** PR-review fixes — banner crop aspect, gallery loading, error handling ([a941c53](https://github.com/whoiskevinrich/holodex/commit/a941c5303c4e288d018a4c5f5584487f8e487d62))
* **person-images:** scale cropped banner correctly (5:2 not 5:1) ([#49](https://github.com/whoiskevinrich/holodex/issues/49)) ([5fbb6c2](https://github.com/whoiskevinrich/holodex/commit/5fbb6c205c2bbc937d546dd08feaab7f7a26dadd))
* **quick-wins:** guard async effects + reset playback/highlight on item change ([4868499](https://github.com/whoiskevinrich/holodex/commit/48684995e853ae8676097a96bd7c7b43fe237502))
* **scanner:** reactivate unchanged rows + guard zero-file walks ([#26](https://github.com/whoiskevinrich/holodex/issues/26)) ([fa51970](https://github.com/whoiskevinrich/holodex/commit/fa519707ab161fe375689419ecbf44c6e152af33))
* **scanner:** reactivate unchanged rows + guard zero-file walks ([#26](https://github.com/whoiskevinrich/holodex/issues/26)) ([48a6df9](https://github.com/whoiskevinrich/holodex/commit/48a6df900b8711207c0614b4bc101fef1a85cf4b))
* **security:** pass absolute paths to exiftool/ffprobe (argv flag-smuggling hardening) ([1c8b51b](https://github.com/whoiskevinrich/holodex/commit/1c8b51bdfa666ebe5aff0513b3ef8744ed312cb3))
* **thumbnails+enrich:** cover-art detection, thumbnail cache-busting, F26-F28 follow-ups ([#40](https://github.com/whoiskevinrich/holodex/issues/40)) ([ff22255](https://github.com/whoiskevinrich/holodex/commit/ff222554594c6fb1683ffc83889f52d9bff0fa90))
* **web:** read recorded year in UTC to fix off-by-one ([25ae8b5](https://github.com/whoiskevinrich/holodex/commit/25ae8b567ab47217115c42cefb71122e07cb7c4a))


### 🚜 Refactor

* **activity:** simplify the gating seam (/simplify) ([0129a34](https://github.com/whoiskevinrich/holodex/commit/0129a34564d35083f04a5b6238cdab686078786f))
* **enrich:** /simplify follow-ups ([5e55eb2](https://github.com/whoiskevinrich/holodex/commit/5e55eb2c576cc25d508291080c59d2b4b35c5e03))


### 📚 Documentation

* add .showcase.md self-report for portfolio sync ([92b344d](https://github.com/whoiskevinrich/holodex/commit/92b344dd2687cb6d767cb20314b5f60091af4f35))
* add .showcase.md self-report for portfolio sync ([a3febfd](https://github.com/whoiskevinrich/holodex/commit/a3febfdad3e28a3d15fc28e4668706e076663138))
* add provider hand-off specs (generic contract + TMDB worked example) ([b245bb2](https://github.com/whoiskevinrich/holodex/commit/b245bb2c888f48c9023b1dee7aef0e7ca7d8e400))
* ADR-020 (frontend embed/build, supersedes ADR-007 mechanics), ADR-021 (theming tokens), ADR-022 (defer in-process cache); phase-1 spec + testing-strategy marked implemented; CLAUDE.md gains token-only styling discipline + 3-skin QA rules. ([36e8bd8](https://github.com/whoiskevinrich/holodex/commit/36e8bd8f24a56a7d5544d20819158db190bd46b0))
* **adr-020:** note MEDIA_PATH unification (amends the HOLODEX_MEDIA_PATH reference) ([71edff1](https://github.com/whoiskevinrich/holodex/commit/71edff1f5f70714c6997b4a0ed3d02f6cd0a6bb0))
* **adr-025:** note the v4 space-y/x selector change ([8f5173a](https://github.com/whoiskevinrich/holodex/commit/8f5173a6b1dd0a657ccf16771b31655ebc59e79e))
* bump ADR count to 34 after ADR-035 merge ([5188044](https://github.com/whoiskevinrich/holodex/commit/5188044901cc3a3c184af9d301354824628dd232))
* **enrich:** ADR-039 provider asset URLs + asset_hosts allowlist ([#32](https://github.com/whoiskevinrich/holodex/issues/32)) ([f6c5880](https://github.com/whoiskevinrich/holodex/commit/f6c588054669d0d76d6eb2fcb49d166b8e089eaa))
* **enrich:** explicit file placement in F22 QA checklist ([9747fda](https://github.com/whoiskevinrich/holodex/commit/9747fda82005e5f93954db787762f48772ad5120))
* **enrich:** group QA checklist by verifier (Setup/Smoke/Agent/Human) ([e700db7](https://github.com/whoiskevinrich/holodex/commit/e700db7526625f6aecae37bfe715862ff3b95f07))
* **enrich:** mark QA 3.12 done (enrich job history) ([07cfcd4](https://github.com/whoiskevinrich/holodex/commit/07cfcd4c3390cc165475d43da7557acc3e11c9db))
* **enrich:** number QA-checklist items (section.item) for quick feedback ([d203970](https://github.com/whoiskevinrich/holodex/commit/d2039708b8a9a725eff3aa739be15b714087809d))
* **enrich:** tag QA items by verifier (smoke/agent/human) ([196281f](https://github.com/whoiskevinrich/holodex/commit/196281f5b83bd5858c0fb8d4096e9f6bd32a8e1d))
* **F21:** design handoff for /status page + header indicator ([7b113ff](https://github.com/whoiskevinrich/holodex/commit/7b113ff19337f22576e41e52684c1780eb080a54))
* **F21:** spec System Activity "Under the Hood" + ADR-028/030 + tests ([afe8bce](https://github.com/whoiskevinrich/holodex/commit/afe8bcefdbf39cb24d813b1edea2c2dd2645baca))
* **governance:** task-tracking rules + ignore personal productivity files ([112cfb3](https://github.com/whoiskevinrich/holodex/commit/112cfb3b6de6b8e4769d07ae13942de057d73112))
* **governance:** track tasks in the main worktree TASKS.md ([5dcc539](https://github.com/whoiskevinrich/holodex/commit/5dcc5394efb8c90e15c34a68764f8ad156d6c55d))
* provider hand-off specs (generic contract + TMDB worked example) ([271911d](https://github.com/whoiskevinrich/holodex/commit/271911d2c5b7779a2c33441c512689b75bf8a9c2))
* **quick-wins:** add QA checklist ([6695f1e](https://github.com/whoiskevinrich/holodex/commit/6695f1e7ef3a1463bb545c03ef28a81ccceddfbb))
* **readme:** product-first rewrite with three-skin gallery ([f6238ca](https://github.com/whoiskevinrich/holodex/commit/f6238ca0aafbfc244569f8c2a118d86b01b4e7c2))
* refresh feature highlights for shipped Phase 2 + F22 enrichment ([cc38f11](https://github.com/whoiskevinrich/holodex/commit/cc38f11012ef19f1e4fc845e94e78c6610f43a8a))
* refresh feature highlights for shipped Phase 2 + F22 enrichment ([e7dbc36](https://github.com/whoiskevinrich/holodex/commit/e7dbc360fba47e71abc64f51a15e5bff3ceda24f))
* refresh README + flesh out metadata-sources.yaml.example ([21f8652](https://github.com/whoiskevinrich/holodex/commit/21f8652f1d4f4793c62b6f1d1f160d5a2c451392))


### 🧪 Testing

* **enrich:** bundle a runnable fake provider stub for manual QA ([77e2212](https://github.com/whoiskevinrich/holodex/commit/77e2212137eb6b55f18f5477ffd38db8c97b1d8c))
* **people:** cover the asset sink (normalize + rollback) and upload size-cap ([86f40cc](https://github.com/whoiskevinrich/holodex/commit/86f40cc6732edde77d67594cf98bb10a9d9e9cb8))


### ⚙️ CI / Build

* **codeql:** fix init input name (language -&gt; languages) ([c638eac](https://github.com/whoiskevinrich/holodex/commit/c638eaccc4287843a180a145324e1bcbbbc77788))
* **codeql:** use manual go build instead of autobuild ([80948af](https://github.com/whoiskevinrich/holodex/commit/80948af59e7eb081c14c52e7262e9726f24519e1))
* **image:** fix Trivy action tag (0.36.0 -&gt; v0.36.0) ([8c30d73](https://github.com/whoiskevinrich/holodex/commit/8c30d73c362520833faa52320f67c5a0e671c5d5))
* **image:** fix Trivy action tag (0.36.0 → v0.36.0) ([8887798](https://github.com/whoiskevinrich/holodex/commit/88877989cc56d79fe377a04a025395b28f547e6d))
* **pages:** deploy landing page to GitHub Pages at holodex.whoiskevinrich.com ([ae378f8](https://github.com/whoiskevinrich/holodex/commit/ae378f8e5c5d814122369e9f2a4d5715f516a723))
* PR quality gate, supply-chain scanning, and tag-driven releases ([c7fe52a](https://github.com/whoiskevinrich/holodex/commit/c7fe52abc48936c24f5ccc12aaff7388553ffb3a))
* PR quality gate, supply-chain scanning, and tag-driven releases ([8775e9c](https://github.com/whoiskevinrich/holodex/commit/8775e9cfc1ae39c3967c81e9bb5070f5b4751f2e))
* **release:** automate version computation with Release Please (ADR-043) ([#45](https://github.com/whoiskevinrich/holodex/issues/45)) ([1d82c59](https://github.com/whoiskevinrich/holodex/commit/1d82c5973a72ab7e15d91c53e7563b7edacd725f))
* **release:** bump release-please-action to v5 (node24) ([#50](https://github.com/whoiskevinrich/holodex/issues/50)) ([113258a](https://github.com/whoiskevinrich/holodex/commit/113258a113e3bc371a4317169346ccd3fa2813c9))
* **release:** git-cliff changelog + GHCR deployment linkage (ADR-034) ([4be79ab](https://github.com/whoiskevinrich/holodex/commit/4be79ab7b4bd8530c88e19269611d0ebbe22220f))
* **release:** git-cliff changelog + GHCR deployment linkage (ADR-034) ([61559e1](https://github.com/whoiskevinrich/holodex/commit/61559e1bda8a4e9f37189518ab84477095b59044))
* scope image build to source paths, gate release on CI, add CodeQL concurrency (ADR-035) ([f1ab4dd](https://github.com/whoiskevinrich/holodex/commit/f1ab4dd76a5e4ccbfd657939db886ff6f2394511))
* scope image build, gate release on CI, add CodeQL concurrency (ADR-035) ([0703e1a](https://github.com/whoiskevinrich/holodex/commit/0703e1a31b148c37b8897e27613689f4591ceac1))
* **web:** migrate Tailwind CSS v3 → v4 (CSS-first config) ([d3c80c8](https://github.com/whoiskevinrich/holodex/commit/d3c80c8a6f7f17486f419971ac756aa80bbe7529))
* **web:** migrate Tailwind CSS v3 → v4 (CSS-first config) ([5acb50f](https://github.com/whoiskevinrich/holodex/commit/5acb50fcb5c52a555a8c27b3c678b4548ce955d6))
* **windows:** embed asInvoker manifest to suppress UAC prompt on launch ([#39](https://github.com/whoiskevinrich/holodex/issues/39)) ([8599a80](https://github.com/whoiskevinrich/holodex/commit/8599a8011fe93dd2bd1eeb12729f6400b28441f2))
