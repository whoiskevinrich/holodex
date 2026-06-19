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

---

## 1. Provider contract — smoke (no real TMDB, no network)

Start the provider with a real token but point at it locally:

```powershell
$env:TMDB_API_TOKEN = "<your token>"
$env:PORT = "9200"
./tmp/holodex-provider-tmdb   # or docker run -e TMDB_API_TOKEN=... -p 9200:9100 holodex-provider-tmdb:qa
```

**1.1** [smoke] `GET http://localhost:9200/healthz` → `200`, body contains `"provider":"tmdb"` and `"status":"ok"`  
**1.2** [smoke] `GET http://localhost:9200/describe` → `200`, body contains `"protocol_version":1`, `"entity_types":["person"]`, `"asset_kinds":["photo"]`; `"fields"` list does **not** contain `"photo"`  
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
      entity_types: [person]
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
