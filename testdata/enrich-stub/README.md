# Enrich stub — fake metadata-source providers for manual QA

A tiny runnable HTTP provider implementing the [ADR-033](../../docs/architecture/ADR-033-metadata-source-plugins.md)
contract, so you can drive the live **enrichment** flow without a real sidecar, network,
or API keys.

> Not used by `go test` (that uses the in-process `enrich.Fake`) or in production —
> this is dev/QA only. Node, no dependencies.

## Ten providers, one process

Core builds a request URL by concatenating the configured `base_url` with `/describe`,
`/resolve` or `/enrich` — it never re-parses the base or replaces its path
(`internal/enrich/client.go`). So a `base_url` that already carries a path just works, and
N config entries pointing at N path prefixes are N independent providers:

```
base_url: http://127.0.0.1:9100/p/alpha   ->  /p/alpha/describe, /p/alpha/resolve, ...
```

That is what makes "five providers disagreeing about one field" cost no extra processes.

The persona table lives in **[`personas.json`](personas.json)**, not in the code, because
three things have to agree about it: this stub serves them, `testdata/stressseed` seeds
their values into `entity_enrichment`, and
[`testdata/stressseed/sources.yaml`](../stressseed/sources.yaml) registers them with the
server. `TestShippedSourcesRegisterEveryPersona` keeps the last two in step.

| group | personas | what it exercises |
|---|---|---|
| `precedence` | `alpha` `bravo` `charlie` `delta` `stress-provider-with-a-very-long-name-40` | ADR-090 **layer 2** — five namespaces with *different* values for the same field, so the ADR-051 `SourceBadge` chip row has something to choose between |
| `adoption` | `flood` `twins` | ADR-090 **layer 1** — `EnrichPicker` candidate lists |
| `fault` | `slow` `boom` `garbage` | loading and error states, on demand |

Two personas carry a deliberate deformity: **`charlie` advertises no `brand_icon`**, so the
`ProviderIcon` monogram fallback is reachable; and the **40-character name** is there
because two places render a provider name with no truncation and no max-width — the
expanded chip's `·{provenance}` suffix (`CurationChip.svelte`) and the owner's
per-provider action chips (`EnrichProviderChips.svelte`).

Values inside a group are **all distinct on purpose**: the chip row folds by *value*
(`web/src/lib/f36.ts`), so five providers that agreed would render as one chip and teach
nothing. Two tests assert it.

### The bare routes still serve the original persona

`/healthz`, `/describe`, `/resolve` and `/enrich` (no prefix) are the single `fake`
provider they always were — substring-matching `hayao miyazaki`, `person` only, CJK alias
for the tofu check — so the F22 QA checklist and any `metadata-sources.yaml` already
pointing at `http://127.0.0.1:9100` keep working unchanged.

## Start it

```
preview_start enrich-stub
```

(`.claude/launch.json` has an `enrich-stub` entry → `node testdata/enrich-stub/stub.js`,
port 9100.) Or directly:

```powershell
node testdata/enrich-stub/stub.js          # 127.0.0.1:9100 (override with PORT / HOST)
```

It binds the same port as `provider-tmdb` — run one or the other.

`DELAY_MS` slows `/resolve` + `/enrich` on **every** persona (0 = instant), for a
fleet-wide slow network. `SLOW_MS` (default 1500) is how long the `slow` persona alone
waits, so the loading state stays reachable without making every other provider unusable.
Neither delays `/healthz` or `/describe`: a provider that cannot be discovered is a
different bug from one that cannot answer, and core calls `/describe` on the hot path of
every resolve and enrich.

## Wire it to a running backend

The **`backend-stress`** profile already points `METADATA_SOURCES_PATH` at
`testdata/stressseed/sources.yaml`, so with the stress fixture there is nothing to do —
seed, start both, done:

```powershell
go run ./testdata/stressseed
preview_start backend-stress
preview_start enrich-stub
```

For any other profile, add the entries by hand to your `metadata-sources.yaml` (copy them
from `sources.yaml`) and reload without a restart:

```powershell
curl -X POST -H "X-Admin-Token: $TOKEN" http://127.0.0.1:7800/api/v1/admin/reload-config
curl -H "X-Admin-Token: $TOKEN" http://127.0.0.1:7800/api/v1/enrich/sources
```

## Reaching each layer

**Precedence (layer 2)** is already there after a seed — `go run ./testdata/stressseed`
writes the namespaces itself, so `/media/902` boots with five competing chips on `Tagline`
and `Language` and `/media/901` with one. The stub does not need to be running for that;
it is needed to *re-enrich*. See the stress fixture's
[README](../stressseed/README.md#the-enrichment-dimension).

**Adoption (layer 1)** is only reachable live, because a candidate list exists only during
a resolve. Open any entity's enrich picker against `flood` or `twins`:

- `flood` returns **30** candidates. Core caps a response at `maxCandidates = 25`
  (`internal/enrich/service.go`), so the picker renders 25 — the persona stresses the list
  *and* proves the cap holds.
- `twins` returns 8 candidates with the **same label**, differing only in
  `disambiguation` (one pair is a genuine exact duplicate, so even the tiebreaker ties).

Both stay below the `0.85` auto-apply threshold on purpose: a *lone* candidate at or above
it is applied without the picker ever opening, which would skip the list they exist to
stress.

**Faults** — `slow` (delayed), `boom` (503), `garbage` (valid HTTP, invalid JSON, so core
reports a decode error rather than a status error). Each faults `/resolve` + `/enrich`
only; `/describe` stays healthy so a failure is attributable to the call you made.
