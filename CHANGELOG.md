# Changelog

## [1.13.2](https://github.com/whoiskevinrich/holodex/compare/v1.13.1...v1.13.2) (2026-07-26)


### 🐛 Bug Fixes

* **writeback:** clarify current vs incoming values in the writeback modal ([#172](https://github.com/whoiskevinrich/holodex/issues/172)) ([02cffc5](https://github.com/whoiskevinrich/holodex/commit/02cffc5fa49d6d497b505ff0b5f5ccc2e4f759a4))

## [1.13.1](https://github.com/whoiskevinrich/holodex/compare/v1.13.0...v1.13.1) (2026-07-26)


### 🐛 Bug Fixes

* **thumbnail:** scale embedded cover art to THUMBNAIL_WIDTH ([#170](https://github.com/whoiskevinrich/holodex/issues/170)) ([ee1f331](https://github.com/whoiskevinrich/holodex/commit/ee1f33179c9bf1225753ed275de52783dfdc903b))

## [1.13.0](https://github.com/whoiskevinrich/holodex/compare/v1.12.1...v1.13.0) (2026-07-26)


### 🚀 Features

* **activity:** attribute job runs to their entity and revert by batch column ([97ee991](https://github.com/whoiskevinrich/holodex/commit/97ee9917fb1e1bab393712759e49940d36be9319))
* **activity:** per-kind job-history digest as the default status view ([#166](https://github.com/whoiskevinrich/holodex/issues/166)) ([57daadc](https://github.com/whoiskevinrich/holodex/commit/57daadc6b1e419019877c189b01512d674f09b73))


### 🐛 Bug Fixes

* **ci:** measure release-candidate freshness per image, not against main ([#164](https://github.com/whoiskevinrich/holodex/issues/164)) ([f7fe7ac](https://github.com/whoiskevinrich/holodex/commit/f7fe7ac7dd333b15021f29dffb7270bd41dd9c91))
* **enrich:** render raw-enrichment poster images and log dropped non-person assets ([#169](https://github.com/whoiskevinrich/holodex/issues/169)) ([9dfcfe3](https://github.com/whoiskevinrich/holodex/commit/9dfcfe3a80b3a4fd046076e2b0be27ee4615b311))
* **web:** paint job history without waiting on the activity read-model ([d6a0c28](https://github.com/whoiskevinrich/holodex/commit/d6a0c2845c9fad820174488a91496509299886df))


### ⚙️ CI / Build

* guard against duplicate ADR numbers ([#165](https://github.com/whoiskevinrich/holodex/issues/165)) ([9ccbdc2](https://github.com/whoiskevinrich/holodex/commit/9ccbdc2f0fbdaf975949330bcf0ab6c941cca3c5))
* **release:** canary the release candidate, promote by retagging the same digest ([e8120b9](https://github.com/whoiskevinrich/holodex/commit/e8120b9d1bdb87a3041ec7acef4b4ebf528c6be2))

## [1.12.1](https://github.com/whoiskevinrich/holodex/compare/v1.12.0...v1.12.1) (2026-07-22)


### 🐛 Bug Fixes

* **extract:** drop numeric/date/resolution values from people extraction ([#153](https://github.com/whoiskevinrich/holodex/issues/153)) ([40cf762](https://github.com/whoiskevinrich/holodex/commit/40cf762fcd0c7361c24c2fc60bedc059566e3afc))
* **extraction:** co-locate row staging control and pin the commit bar ([#156](https://github.com/whoiskevinrich/holodex/issues/156)) ([5770363](https://github.com/whoiskevinrich/holodex/commit/5770363f014ffd6d5fb8520348557d50c26987eb))
* **web:** shared button treatments and text-muted disabled contrast ([#158](https://github.com/whoiskevinrich/holodex/issues/158)) ([ca6185b](https://github.com/whoiskevinrich/holodex/commit/ca6185bd5c0523b3e209f8f505f0769fa0c230c6))
* **writeback:** stop erasing metadata the batch didn't touch (MKV cover art + tags) ([#157](https://github.com/whoiskevinrich/holodex/issues/157)) ([0db2a18](https://github.com/whoiskevinrich/holodex/commit/0db2a18681edf72c200525e2a193c1ef0ccf3134))


### 📚 Documentation

* **readme:** clarify latest vs edge image tags ([#155](https://github.com/whoiskevinrich/holodex/issues/155)) ([b43b8d9](https://github.com/whoiskevinrich/holodex/commit/b43b8d99c1832208a4a783ba5ce5d8827aa6f4e0))

## [1.12.0](https://github.com/whoiskevinrich/holodex/compare/v1.11.0...v1.12.0) (2026-07-21)


### 🚀 Features

* **extract:** F48 on-demand metadata extraction — docs + all 6 phases ([#148](https://github.com/whoiskevinrich/holodex/issues/148)) ([078626a](https://github.com/whoiskevinrich/holodex/commit/078626a211784f7d8a9409982651d3be9a243d17))


### 🐛 Bug Fixes

* **extract:** multi-person review chips, entity creation on resolve, year misparse ([#152](https://github.com/whoiskevinrich/holodex/issues/152)) ([3cd9345](https://github.com/whoiskevinrich/holodex/commit/3cd93451b7d076c9e5608c30840e18b2642b5115))
* **web:** return to referring page on delete instead of browse root ([#145](https://github.com/whoiskevinrich/holodex/issues/145)) ([fdaebca](https://github.com/whoiskevinrich/holodex/commit/fdaebcaadfc7c9e00e82318d3553263e97045afb))
* **web:** return to referring page on delete instead of browse root ([#146](https://github.com/whoiskevinrich/holodex/issues/146)) ([9149de9](https://github.com/whoiskevinrich/holodex/commit/9149de9ee64abefc666565792b3971966a22c3a4))


### 🚜 Refactor

* **enrichment:** simplify F47 review-workflow follow-up cleanups ([#143](https://github.com/whoiskevinrich/holodex/issues/143)) ([1d35c3b](https://github.com/whoiskevinrich/holodex/commit/1d35c3b15d597664929a370b2a6bc861b8f1d67d))

## [1.11.0](https://github.com/whoiskevinrich/holodex/compare/v1.10.0...v1.11.0) (2026-07-14)


### 🚀 Features

* **enrichment:** review workflow — auto-apply, dismissals, provider refresh (ADR-066) ([#130](https://github.com/whoiskevinrich/holodex/issues/130)) ([f022df1](https://github.com/whoiskevinrich/holodex/commit/f022df1a18936369c306255dcbdb0f51b3f07ca6))


### 📚 Documentation

* **specs:** add age-in-media spec (HOLODEX-173) ([#131](https://github.com/whoiskevinrich/holodex/issues/131)) ([c7d3a81](https://github.com/whoiskevinrich/holodex/commit/c7d3a8154516df259b0b252a1282f4712816c10b))

## [1.10.0](https://github.com/whoiskevinrich/holodex/compare/v1.9.1...v1.10.0) (2026-07-13)


### 🚀 Features

* **enrichment:** emit provider-supplied profile_url on candidates (P1-1) ([#140](https://github.com/whoiskevinrich/holodex/issues/140)) ([f5fbfc9](https://github.com/whoiskevinrich/holodex/commit/f5fbfc99e484583f3e6d0028fbc0c80d61e449c4))


### 🐛 Bug Fixes

* **people:** correct banner ratio to 1600x600, poster-led hero hierarchy ([#136](https://github.com/whoiskevinrich/holodex/issues/136)) ([1a31732](https://github.com/whoiskevinrich/holodex/commit/1a3173269b779ec85c82f5037757aad3cf7c52cf))
* **video-detail:** honor card_layout in related-video shelves ([#132](https://github.com/whoiskevinrich/holodex/issues/132)) ([6157212](https://github.com/whoiskevinrich/holodex/commit/6157212e7cb25de770279c9966259facb91c01ac))


### 📚 Documentation

* **design:** age-in-media cast poster handoff (HOLODEX-173) ([#137](https://github.com/whoiskevinrich/holodex/issues/137)) ([45e9c13](https://github.com/whoiskevinrich/holodex/commit/45e9c13ffd90d7c03768a486edd2abc70758fae2))

## [1.9.1](https://github.com/whoiskevinrich/holodex/compare/v1.9.0...v1.9.1) (2026-07-12)


### 🐛 Bug Fixes

* **jira-sync:** never auto-transition Epic issues ([#128](https://github.com/whoiskevinrich/holodex/issues/128)) ([474bbd7](https://github.com/whoiskevinrich/holodex/commit/474bbd74424beabfd9ad5a6099db6ceaddc8c9a5))
* **metadata-mappings:** fully wire tmdb example config across video/person/studio ([#127](https://github.com/whoiskevinrich/holodex/issues/127)) ([f15f548](https://github.com/whoiskevinrich/holodex/commit/f15f548889f57ff2cb8a1e2d7719fbc9d2e64b63))

## [1.8.0](https://github.com/whoiskevinrich/holodex/compare/v1.7.0...v1.8.0) (2026-07-12)


### 🚀 Features

* **fields:** derived person Age / Age-at-death computed-field engine (F45) ([#115](https://github.com/whoiskevinrich/holodex/issues/115)) ([cf8a42c](https://github.com/whoiskevinrich/holodex/commit/cf8a42ce7ef7b797e46d9ecb8d13c401aa3db04d))
* **flightplan:** ADR-064 + worklog schema + batch-1 hooks (SessionStart, PostToolUse, Stop) ([#119](https://github.com/whoiskevinrich/holodex/issues/119)) ([6947856](https://github.com/whoiskevinrich/holodex/commit/694785649287412327f7babce104fd49ca140d92))


### 🐛 Bug Fixes

* **auth:** restore the way back into owner login ([#118](https://github.com/whoiskevinrich/holodex/issues/118)) ([34728c2](https://github.com/whoiskevinrich/holodex/commit/34728c283da8c5a1e49ad8f6253bad987401bc78))


### ⚡ Performance

* **promotions:** cache field_promotions read path like FieldHints ([#122](https://github.com/whoiskevinrich/holodex/issues/122)) ([dd7c1be](https://github.com/whoiskevinrich/holodex/commit/dd7c1be0e138f9f1f08373065ce0f3baecadcbaf))


### 📚 Documentation

* **conventions:** hide flightplan/agent-tooling commits from changelog via chore type ([#123](https://github.com/whoiskevinrich/holodex/issues/123)) ([efe17d7](https://github.com/whoiskevinrich/holodex/commit/efe17d7c8dde9a1a619ca501566698f455520a61))

## [1.7.0](https://github.com/whoiskevinrich/holodex/compare/v1.6.1...v1.7.0) (2026-07-08)


### 🚀 Features

* **fields:** in-app promote/override for auto-registered enrichment fields (F44) ([#112](https://github.com/whoiskevinrich/holodex/issues/112)) ([7cdd9a7](https://github.com/whoiskevinrich/holodex/commit/7cdd9a7f49ca15d05cac58dfa12ea27f52787021))
* **people:** owner/admin gallery cap bypass, grid + image viewer modals ([#114](https://github.com/whoiskevinrich/holodex/issues/114)) ([6057109](https://github.com/whoiskevinrich/holodex/commit/6057109c34ffb8dbb925c8313a0c610db7572791))

## [1.6.1](https://github.com/whoiskevinrich/holodex/compare/v1.6.0...v1.6.1) (2026-07-07)


### 📚 Documentation

* session-state workflow (idea-to-merge manual + whats-left probe) ([#110](https://github.com/whoiskevinrich/holodex/issues/110)) ([caaae7a](https://github.com/whoiskevinrich/holodex/commit/caaae7aac305b68136af86d1c9514e0d2c5bb134))

## [1.6.0](https://github.com/whoiskevinrich/holodex/compare/v1.5.0...v1.6.0) (2026-07-07)


### 🚀 Features

* **identity:** one-time near-miss review-queue seed (F43 S4) ([#103](https://github.com/whoiskevinrich/holodex/issues/103)) ([6d129b6](https://github.com/whoiskevinrich/holodex/commit/6d129b6c74c66c60856c1a036b20925027282d26))
* **identity:** unified entity name-identity spine (F43) ([#101](https://github.com/whoiskevinrich/holodex/issues/101)) ([1e6b1a8](https://github.com/whoiskevinrich/holodex/commit/1e6b1a8dfb53f330c1568c9f1d8d500a0189e23b))
* **providers/tmdb:** self-host the real TMDB brand icon (HOLODEX-161) ([#100](https://github.com/whoiskevinrich/holodex/issues/100)) ([91b10ad](https://github.com/whoiskevinrich/holodex/commit/91b10adcf862b453174637bd3c6c46360038183f))
* **web:** collapse enrich/clear buttons into provider chips (HOLODEX-136) ([#97](https://github.com/whoiskevinrich/holodex/issues/97)) ([5b35c35](https://github.com/whoiskevinrich/holodex/commit/5b35c35e3724d3ce17dcc271ad9f7cbf688173cd))
* **web:** nationality flag beside the person name ([#102](https://github.com/whoiskevinrich/holodex/issues/102)) ([1121811](https://github.com/whoiskevinrich/holodex/commit/1121811f7f4f73d9e6df5e129f6937a0a490cdf4))
* **web:** website field as provider icon + hostname (HOLODEX-137) ([#98](https://github.com/whoiskevinrich/holodex/issues/98)) ([708d05d](https://github.com/whoiskevinrich/holodex/commit/708d05d856c84f1f5cf27bc22cd4487d6a646fb7))

## [1.5.0](https://github.com/whoiskevinrich/holodex/compare/v1.4.0...v1.5.0) (2026-07-05)


### 🚀 Features

* **enrich:** accept provider-sourced WebP images (F42) ([#94](https://github.com/whoiskevinrich/holodex/issues/94)) ([c7a60c8](https://github.com/whoiskevinrich/holodex/commit/c7a60c878a70ee1146edc47ea2bfddae7b3b2641))
* **enrich:** provider field render hints + non-canonical auto-registration (F39) ([e3d6382](https://github.com/whoiskevinrich/holodex/commit/e3d63825d27713ff9b510eacd87ec790c4778f94))
* **enrich:** self-hosted provider brand icon (ADR-059) ([#95](https://github.com/whoiskevinrich/holodex/issues/95)) ([435da9c](https://github.com/whoiskevinrich/holodex/commit/435da9cf07d41fc3701472b826015c1fb27bcd80))
* **studios:** self-host studio logo instead of hotlinking the provider CDN ([#91](https://github.com/whoiskevinrich/holodex/issues/91)) ([ea48319](https://github.com/whoiskevinrich/holodex/commit/ea48319ed7732ab4f5ee80b6babb8dd4b4e23d22))


### 🐛 Bug Fixes

* **web:** auto-recover authed polls from ForwardAuth session expiry ([#84](https://github.com/whoiskevinrich/holodex/issues/84)) ([e88d075](https://github.com/whoiskevinrich/holodex/commit/e88d075584c7039b34a7839e9e5fe4538a6fb093))


### 📚 Documentation

* **architecture:** ADR-056 — Jira transitions via REST API ([#92](https://github.com/whoiskevinrich/holodex/issues/92)) ([6b434fc](https://github.com/whoiskevinrich/holodex/commit/6b434fcd5deb197e4b393f9e2cfdfce4caf51e37))
* correct enrichment-provider docs in CLAUDE.md ([#88](https://github.com/whoiskevinrich/holodex/issues/88)) ([1ce0759](https://github.com/whoiskevinrich/holodex/commit/1ce07590bdb4b23b35abf6153c2ef2da8311591c))
* F39 owner person + studio media linking (spec + ADR-056 + design + tests) ([#93](https://github.com/whoiskevinrich/holodex/issues/93)) ([8f71c09](https://github.com/whoiskevinrich/holodex/commit/8f71c09f2ff6dd09ac91622c27facadc8368a098))

## [1.4.0](https://github.com/whoiskevinrich/holodex/compare/v1.3.1...v1.4.0) (2026-07-04)


### 🚀 Features

* **browse:** entity-backed studio filter via ?studio_id (F38) ([#81](https://github.com/whoiskevinrich/holodex/issues/81)) ([3bb486f](https://github.com/whoiskevinrich/holodex/commit/3bb486f4800ae0c2d550c047ca518345dd5a9e74))
* **enrich:** per-provider match/enrich/clear UI on media + people detail ([#79](https://github.com/whoiskevinrich/holodex/issues/79)) ([726721c](https://github.com/whoiskevinrich/holodex/commit/726721c52e3f6d6e5afa006ede8cf093a3541ae3))
* **enrich:** TMDB studio company enrichment (F38 S3) ([#82](https://github.com/whoiskevinrich/holodex/issues/82)) ([b900b9f](https://github.com/whoiskevinrich/holodex/commit/b900b9fc39d135c785e9e968009f44de117f75d5))
* **metadata:** per-field source-of-truth SourceSelect control (F36) ([#71](https://github.com/whoiskevinrich/holodex/issues/71)) ([f0a8545](https://github.com/whoiskevinrich/holodex/commit/f0a8545b194761186201173f7eddeb0260929922))
* **metadata:** unify source-of-truth control on selectable source chips (F36) ([#73](https://github.com/whoiskevinrich/holodex/issues/73)) ([a993022](https://github.com/whoiskevinrich/holodex/commit/a993022169e9775b86a9a90327232c064a4ff38b))
* **people:** person detail on unified source-of-truth model (F37) ([#74](https://github.com/whoiskevinrich/holodex/issues/74)) ([0f5517b](https://github.com/whoiskevinrich/holodex/commit/0f5517b078376b220ed665f6e858a6bba69b2b87))
* **resolver:** inter-provider trust order via provider_trust_order (F36 P1-2) ([#78](https://github.com/whoiskevinrich/holodex/issues/78)) ([f0813fc](https://github.com/whoiskevinrich/holodex/commit/f0813fc90173d7a3c4013850f660634712341b16))
* **resolver:** per-field source-of-truth decisions backend (F36) ([#72](https://github.com/whoiskevinrich/holodex/issues/72)) ([2dc81f0](https://github.com/whoiskevinrich/holodex/commit/2dc81f0994a51b61e496ddfeb87ccddae10894d5))
* **studios:** external-id de-dup via provider company id (ADR-054) ([#76](https://github.com/whoiskevinrich/holodex/issues/76)) ([746a5ac](https://github.com/whoiskevinrich/holodex/commit/746a5ac0e1d71be2e2af88da5ce1ddc7a44336b0))
* **studios:** logo well + monogram in /studios list (F38) ([#83](https://github.com/whoiskevinrich/holodex/issues/83)) ([f1dd03c](https://github.com/whoiskevinrich/holodex/commit/f1dd03cb41d26b02b0ec127e5fbce0c76b069110))


### 🚜 Refactor

* **resolver:** pin BaselineSource contract for entity-agnostic resolution ([#69](https://github.com/whoiskevinrich/holodex/issues/69)) ([8e8884d](https://github.com/whoiskevinrich/holodex/commit/8e8884dc508c965159cd41b767bb95925a38e68d))


### 📚 Documentation

* **architecture:** ADR-055 universal enrichment unique-key invariant ([#80](https://github.com/whoiskevinrich/holodex/issues/80)) ([e779e27](https://github.com/whoiskevinrich/holodex/commit/e779e271792a51c1a1adfe85c9e41567b62fcc93))
* **claude-md:** auto-rename worktree branch to its Jira key on start ([#70](https://github.com/whoiskevinrich/holodex/issues/70)) ([fa84b6a](https://github.com/whoiskevinrich/holodex/commit/fa84b6a07a99fac0ebf74c5163f356c6a0b140bf))
* correct TMDB video field mapping + add CLAUDE.md codebase map ([#77](https://github.com/whoiskevinrich/holodex/issues/77)) ([ab2de60](https://github.com/whoiskevinrich/holodex/commit/ab2de60b92ab84bc4b14c1ed21020ed939dfa966))
* **F36:** per-field source-of-truth decisions (ADR-051) design package ([#66](https://github.com/whoiskevinrich/holodex/issues/66)) ([12a984b](https://github.com/whoiskevinrich/holodex/commit/12a984b649f475a3bee1155dcbfafb21591d5c96))
* Jira ↔ GitHub pipeline integration & Jira-native task tracking ([#67](https://github.com/whoiskevinrich/holodex/issues/67)) ([867aad0](https://github.com/whoiskevinrich/holodex/commit/867aad0c6a9712c253cb4c735ec9724cecc5bedd))
* **specs:** studio entity promotion spec (F38) ([#75](https://github.com/whoiskevinrich/holodex/issues/75)) ([16a621a](https://github.com/whoiskevinrich/holodex/commit/16a621afbc95c12f436812a8bba5772999a7788b))

## [1.3.1](https://github.com/whoiskevinrich/holodex/compare/v1.3.0...v1.3.1) (2026-06-30)


### 🐛 Bug Fixes

* **metadata:** strip Matroska language suffix from tag keys ([#63](https://github.com/whoiskevinrich/holodex/issues/63)) ([#64](https://github.com/whoiskevinrich/holodex/issues/64)) ([30e570d](https://github.com/whoiskevinrich/holodex/commit/30e570da7bc68da59a79b054e14ea449e3c1051c))

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
