# QA: TMDB Provider Sidecar + ADR-039 Core Changes

**Feature**: Real TMDB metadata provider (`providers/tmdb/`) + ADR-039 `asset_hosts` allowlist  
**Related**: [ADR-033](../architecture/ADR-033-metadata-source-plugins.md), [ADR-039](../architecture/ADR-039-provider-asset-urls.md), [ADR-040](../architecture/ADR-040-tmdb-provider-repo-placement.md), [tmdb-provider spec](tmdb-provider.md)

---

## 0. Setup

**0.1** [smoke] Ensure Go builds cleanly: `go build ./...`  
**0.2** [smoke] Ensure tests pass: `go test ./...`  
**0.3** [smoke] Build the provider binary: `go build -o /tmp/holodex-provider-tmdb ./providers/tmdb`  
**0.4** [smoke] `Dockerfile.provider-tmdb` builds without error: `docker build -f Dockerfile.provider-tmdb -t holodex-provider-tmdb:qa .`  
**0.5** [human] Copy `.env.example` → `.env` in the worktree; confirm `TMDB_API_TOKEN` is set (obtain from TMDB dashboard → Settings → API → Read Access Token)  
**0.6** [human] Copy `metadata-mappings.yaml.example` → `metadata-mappings.yaml` (next to `holodex.yaml`) and reload config. This is what wires tmdb's raw enrichment fields (`release_date`, `bio`, `birthdate`, studio `description`/`country`/`logo`, …) into the resolved API — §3, §7b, and §8 below all assume it's in place. Without it, enrichment can succeed yet none of those fields render.

---

## 1. Provider contract — smoke (no real TMDB, no network)

Start the provider with a real token but point at it locally:

```powershell
$env:TMDB_API_TOKEN = "<your token>"
$env:PORT = "9200"
./tmp/holodex-provider-tmdb   # or docker run -e TMDB_API_TOKEN=... -p 9200:9100 holodex-provider-tmdb:qa
```

**1.1** [smoke] `GET http://localhost:9200/healthz` → `200`, body contains `"provider":"tmdb"` and `"status":"ok"`  
**1.2** [smoke] `GET http://localhost:9200/describe` → `200`, body contains `"protocol_version":1`, `entity_types` contains both `"person"` **and** `"video"`, `"asset_kinds":["photo"]`; `"fields"` list includes person fields (`bio`, `birthdate`, …) and video fields (`overview`, `release_date`, `genres`, `poster_url`, …); `"photo"` is **not** in `fields`  
**1.3** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"person","hint":{"query":"Hayao Miyazaki"}}` → `200`, at least one candidate with `external_id` prefixed `tmdb:` and `namespace:"tmdb"`  
**1.4** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"person","hint":{"query":"xyzzy_no_match_999"}}` → `200`, `{"candidates":[]}`  
**1.5** [smoke] `POST http://localhost:9200/enrich` with body `{"entity_type":"person","external_id":"tmdb:608"}` → `200`, fields object contains `bio`, `birthdate`, `aliases`; `assets` array contains `{"kind":"photo","url":"https://image.tmdb.org/..."}` (when Miyazaki has a photo on TMDB)  
**1.6** [smoke] `POST http://localhost:9200/enrich` with body `{"entity_type":"person","external_id":"tmdb:999999999"}` → non-2xx (404 or 502), not a 200 with empty fields  
**1.7** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"person","hint":{"external_ids":["tmdb:608"]}}` → `200`, exactly one candidate `external_id:"tmdb:608"`  
**1.8** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"person","hint":{"external_ids":["imdb:nm0594503"]}}` → `200`, candidate(s) for Hayao Miyazaki (TMDB find-by-IMDb-ID path)  
**1.9** [smoke] No TMDB token/key appears in any response body or server log output

---

## 2. ADR-039 core changes — `asset_hosts` allowlist

**2.1** [smoke] `go test ./internal/enrich/...` passes; specifically the host-allowlist tests:
  - Base host (`holodex-tmdb:9100`) allowed
  - Allowlisted extra host (`image.tmdb.org`) allowed
  - Unknown host (`evil.example.com`) refused with error
  - Cross-host redirect refused (redirect from `image.tmdb.org` to `evil.example.com` stops at 30x)
  - `http://image.tmdb.org` refused when `image.tmdb.org` is in `asset_hosts` (requires `https` for non-base hosts)
  - `http://holodex-tmdb:9100` (base host) allowed even with `http` scheme  

**2.2** [smoke] First-success-per-role: when provider returns `[{kind:"photo",url:A},{kind:"photo",url:B}]`, only the first is fetched+stored; the second is skipped

**2.3** [smoke] `Manifest.AssetKinds` field parsed correctly when a provider's `/describe` returns `asset_kinds:["photo"]`

---

## 3. End-to-end via Holodex + real TMDB provider

Set up local Holodex pointing at the provider sidecar:

**3.1** [human] Add to `metadata-sources.yaml`:
  ```yaml
  sources:
    - name: tmdb
      base_url: http://127.0.0.1:9200
      entity_types: [person, video]
      asset_hosts: [image.tmdb.org]
      enabled: true
  ```
  Then reload: `POST http://localhost:7800/api/v1/admin/reload-config` (with `X-Admin-Token`)

**3.2** [human] Navigate to any Person page (e.g. a person from your library) → the "Enrich" button shows "tmdb" as an option (because the provider is enabled for `person`)

**3.3** [human] Click "Enrich from tmdb" → a name-search resolve picker appears; type a few letters of the person's name → candidates appear with `disambiguation` strings (role · known work)

**3.4** [human] Select a candidate → enrichment runs → the Person page shows:
  - `Bio` field with "from tmdb" provenance badge
  - `Born` (birthdate) with "from tmdb" badge
  - `Nationality` (place of birth string) with "from tmdb" badge
  - `Aliases` list (including native script if TMDB has it) with "from tmdb" badge
  - Person image updated to TMDB portrait (headshot slot) if TMDB had a `profile_path`

**3.5** [human] Confirm three skins (Cinémathèque, Broadcast, Brutalist): the ProvenanceBadge and enriched fields render correctly in all three; no hardcoded colors visible

**3.6** [human] Navigate to System Activity → the enrich run appears in the job history with `status: ok` and a detail line like `tmdb → person #N (5 fields)`

**3.7** [human] Re-enrich the same person (click Enrich again on the same person) → the picker does not re-appear (existing match is used); a fresh fetch updates the data

**3.8** [human] Clear enrichment (`DELETE /api/v1/admin/people/{id}/enrich/tmdb`) → fields disappear; re-enrich works again

---

## 4. Provider image (Docker)

**4.1** [smoke] `docker build -f Dockerfile.provider-tmdb -t holodex-provider-tmdb:qa .` succeeds (multi-stage build)  
**4.2** [smoke] `docker run --rm -e TMDB_API_TOKEN=<token> -p 9200:9100 holodex-provider-tmdb:qa` starts; `GET /healthz` returns 200  
**4.3** [smoke] HEALTHCHECK in the image passes: `docker inspect` shows `(healthy)` after 30s  
**4.4** [smoke] `docker run --rm holodex-provider-tmdb:qa` (no token set) exits with a non-zero status and a useful error message

---

## 5. Security checks

**5.1** [smoke] `grep -r "TMDB_API" providers/tmdb/` shows the token is only read via `os.Getenv`, never hardcoded  
**5.2** [smoke] `/healthz`, `/describe` responses contain no token, API key, or credential  
**5.3** [smoke] `POST /resolve` and `POST /enrich` responses contain no token/key  
**5.4** [smoke] Provider log output (at INFO level) contains no token/key  
**5.5** [smoke] Provider builds with `CGO_ENABLED=0`; the binary is statically linked (no C deps)  
**5.6** [smoke] Trivy scan (`trivy image holodex-provider-tmdb:qa`) reports no CRITICAL or HIGH CVEs with a fix available (same bar as the main image)  
**5.7** [agent] No wildcard/subdomain in `asset_hosts` — only exact host `image.tmdb.org`; confirm `AssetClient` does exact-host match, not suffix match

---

## 6. Non-functional

**6.1** [smoke] `POST /resolve` with a real query returns within 3s under normal TMDB conditions  
**6.2** [smoke] `POST /enrich` returns within 3s under normal TMDB conditions  
**6.3** [human] Simulate slow TMDB by adding a 7s delay in the TMDB client timeout test → provider returns 502 within 8s (Holodex's hard limit); Holodex shows a generic error, does not hang  
**6.4** [smoke] A response where `biography` > 4000 chars is trimmed to a sentence boundary before being returned (test with a mock response)  
**6.5** [smoke] A candidate list > 10 items from TMDB is capped at 10 in the response  
**6.6** [smoke] `/describe` response body is well under 1 MiB; `/resolve` and `/enrich` are under 1 MiB even for rich results

---

## 7. Film / Video enrichment (F26)

### 7a. Provider contract — video entity type

**7a.1** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"video","hint":{"query":"Fight Club"}}` → `200`, at least one candidate with `external_id:"tmdb:550"`, `label:"Fight Club"`, `disambiguation:"1999"`  
**7a.2** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"video","hint":{"query":"xyzzy_no_match_999"}}` → `200`, `{"candidates":[]}`  
**7a.3** [smoke] `POST http://localhost:9200/enrich` with body `{"entity_type":"video","external_id":"tmdb:550"}` → `200`, fields include `overview`, `release_date:"1999-10-15"`, `runtime:"139"`, `genres:["Drama","Thriller"]`, `tagline`, `imdb_id:"tt0137523"`, `poster_url` starts with `"https://image.tmdb.org/t/p/original/"`; **no** `assets[]` in the response  
**7a.4** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"video","hint":{"external_ids":["imdb:tt0137523"]}}` → `200`, candidate for Fight Club  
**7a.5** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"video","hint":{"external_ids":["tmdb:550"]}}` → `200`, single candidate for Fight Club with `confidence:1.0`  
**7a.6** [smoke] `POST http://localhost:9200/enrich` with body `{"entity_type":"video","external_id":"tmdb:999999999"}` → non-2xx  
**7a.7** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"series","hint":{"query":"anything"}}` → `200`, `{"candidates":[]}` (unknown entity type returns empty, not an error)  

### 7b. End-to-end via Holodex UI

**7b.1** [human] Navigate to a Media detail page (`/media/{id}`) while logged in as owner with the TMDB provider configured for `entity_types: [person, video]` → a **Film Details** section appears with an "Enrich from tmdb" button  
**7b.2** [human] Click "Enrich from tmdb" on the Media page → the resolver picker opens pre-filled with the video's title; type a film name → movie candidates appear with year disambiguation (e.g. `"1999"`)  
**7b.3** [human] Select "Fight Club (1999)" from the candidates → enrichment applies → Film Details section shows:
  - Overview text (multi-sentence synopsis) with "from tmdb" provenance badge
  - Poster rendered as an `<img>` element (not raw URL text)
  - Release date, runtime (minutes), genres (comma-joined list), tagline, IMDb ID, language — each with "from tmdb" badge
  - "Clear tmdb data" button now visible

**7b.4** [human] After enrichment, reload the page → Film Details persist (enrichment is stored, not session-state)  
**7b.5** [human] Click "Clear tmdb data" → all Film Details fields removed; the "Enrich from tmdb" button is the only control remaining; page no longer shows enrichment rows  
**7b.6** [human] Confirm three skins (Cinémathèque, Broadcast, Brutalist): Film Details section and poster `<img>` render correctly in all three; no hardcoded colors  
**7b.7** [human] Navigate to System Activity → the video enrich run appears in job history with `status: ok` and a detail line referencing the provider and video id  
**7b.8** [human] On a Media page with no video-capable provider configured (provider `entity_types` contains only `person`): Film Details section does **not** appear

---

## 8. Studio enrichment (F38 S3)

### 8a. Provider contract — studio entity type

**8a.1** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"studio","hint":{"query":"Pixar"}}` → `200`, at least one candidate with `external_id` prefixed `tmdb:`, `namespace:"tmdb"`  
**8a.2** [smoke] `POST http://localhost:9200/resolve` with body `{"entity_type":"studio","hint":{"query":"xyzzy_no_match_999"}}` → `200`, `{"candidates":[]}`  
**8a.3** [smoke] `POST http://localhost:9200/enrich` with body `{"entity_type":"studio","external_id":"tmdb:3"}` (Pixar) → `200`, fields include `description`, `country`, `website`, `logo` starting with `"https://image.tmdb.org/t/p/original/"`; **no** `assets[]` in the response  
**8a.4** [smoke] A studio with no `homepage` upstream still returns a non-empty `website` (falls back to the studio's TMDB page — see `tmdbEntityURL` in `providers/tmdb/tmdb.go`)

### 8b. End-to-end via Holodex UI

**8b.1** [human] Navigate to a Studio page (`/studios/{id}`) while logged in as owner with the TMDB provider configured for `entity_types` including `studio` → the "Details" section header shows an Enrich chip for `tmdb`  
**8b.2** [human] Click the tmdb Enrich chip → the resolver picker opens pre-filled with the studio's name; type a studio name → candidates appear  
**8b.3** [human] Select a candidate → enrichment applies → the Details section shows:
  - `Description` (long-text) with "from tmdb" provenance
  - `Country` with "from tmdb" provenance
  - `Website` with "from tmdb" provenance, rendered as a link
  - `Logo` rendered as an `<img>` from the self-hosted, normalized copy (`studio.logo_url`), not the raw TMDB URL

**8b.4** [human] Reload the page → Details persist (enrichment is stored, not session-state)  
**8b.5** [human] Clear the tmdb chip (overflow menu) → studio `description`/`country`/`website`/`logo` fields removed; the Enrich chip is available again  
**8b.6** [human] Confirm three skins (Cinémathèque, Broadcast, Brutalist): the Details section, logo `<img>`, and provenance badges render correctly in all three; no hardcoded colors  
**8b.7** [human] Navigate to System Activity → the studio enrich run appears in job history with `status: ok` and a detail line referencing the provider and studio id  
**8b.8** [human] On a Studio page with no studio-capable provider configured (provider `entity_types` omits `studio`): no Enrich chip appears, and the Details section is hidden unless there's something else to curate  
