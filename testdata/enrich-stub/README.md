# F22 enrich stub — fake metadata-source provider for manual QA

A tiny runnable HTTP provider implementing the [ADR-033](../../docs/architecture/ADR-033-metadata-source-plugins.md)
contract, so you can drive the live **People enrichment** flow (the
[QA checklist](../../docs/design/metadata-enrichment-qa-checklist.md) §3/§4) without
a real sidecar, network, or API keys.

> Not used by `go test` (that uses the in-process `enrich.Fake`) or in production —
> this is dev/QA only. Node, no dependencies.

## Start it

**Via the preview launcher** (recommended — it manages the process so it can't orphan and squat the port):

```
preview_start enrich-stub
```

(`.claude/launch.json` has an `enrich-stub` entry → `node testdata/enrich-stub/stub.js`, port 9100.)

**Or directly:**

```powershell
node testdata/enrich-stub/stub.js          # 127.0.0.1:9100 (override with PORT / HOST)
```

## Slow-network mode (QA 3.18 / 3.21)

`DELAY_MS` slows the user-facing `/resolve` + `/enrich` (not `/healthz` / `/describe`)
to simulate a slow provider — so the picker's **loading** ("Searching…") and
**slow-connection** states are actually visible. Run the **`enrich-stub-slow`** launch
config (same port 9100, `DELAY_MS=1500`):

```
preview_start enrich-stub-slow        # or: $env:DELAY_MS='1500'; node testdata/enrich-stub/stub.js
```

It's the same port as `enrich-stub`, so stop the fast one first (run one or the other).

## Wire Holodex to it

1. `metadata-sources.yaml` (repo root; gitignored — copy from `metadata-sources.yaml.example`,
   which has this exact stub block commented):

   ```yaml
   sources:
     - name: fake
       base_url: http://127.0.0.1:9100
       entity_types: [person]
       enabled: true
   ```

2. Load it without restarting the backend (the F22.2d reload path):

   ```powershell
   curl -X POST -H "X-Admin-Token: $TOKEN" http://127.0.0.1:7800/api/v1/admin/reload-config
   ```

   (omit the header if `ADMIN_TOKEN` is unset / open mode). Then verify:

   ```powershell
   curl http://127.0.0.1:9100/healthz
   curl -H "X-Admin-Token: $TOKEN" http://127.0.0.1:7800/api/v1/enrich/sources   # -> lists "fake"
   ```

## Behaviour

- `GET /healthz`, `GET /describe`, `POST /resolve`, `POST /enrich`.
- `/resolve` returns a candidate (`tmdb:608`, "Hayao Miyazaki", confidence 0.9) for any
  query that is a substring of `hayao miyazaki` (e.g. `miyaz`); anything else → no candidates
  (the no-results path).
- `/enrich` returns canned fields: bio, birthdate, nationality, website, and aliases
  including a CJK alias (`宮崎駿`) for the tofu check.
