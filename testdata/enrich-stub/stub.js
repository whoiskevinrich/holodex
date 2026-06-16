// F22 metadata-source provider STUB for manual QA / preview (ADR-033).
//
// This is NOT used in production or `go test` (those use the in-process
// `enrich.Fake`). It's a tiny runnable HTTP provider so a human or agent can
// exercise the live People-enrichment flow with no network, no API keys, and no
// real sidecar container.
//
// Start it (recommended — lets the preview launcher manage it, avoiding orphaned
// Bash processes that squat the port):
//     preview_start enrich-stub
//   or directly:
//     node testdata/enrich-stub/stub.js          # honours PORT/HOST, defaults 127.0.0.1:9100
//
// Then point Holodex at it (metadata-sources.yaml — see metadata-sources.yaml.example):
//     sources: [{ name: fake, base_url: http://127.0.0.1:9100, entity_types: [person], enabled: true }]
// and load it without a restart:
//     POST /api/v1/admin/reload-config   (send X-Admin-Token if ADMIN_TOKEN is set)
//
// Contract (ADR-033 F22.1): GET /healthz, GET /describe, POST /resolve, POST /enrich.
// /resolve matches any query that is a substring of "hayao miyazaki" (so "miyaz"
// hits, "zzzz" returns no candidates — exercises the no-results path); /enrich
// returns canned People fields including a CJK alias (font / tofu checks).
const http = require('http');

const PORT = Number(process.env.PORT) || 9100;
const HOST = process.env.HOST || '127.0.0.1';
const NAME = 'fake';
// DELAY_MS slows the user-facing /resolve + /enrich (not /healthz or /describe) to
// simulate a slow network — exercises the picker's loading + slow-connection states
// (QA 3.18 / 3.21). 0 = instant. Set via the `enrich-stub-slow` launch config.
const DELAY_MS = Number(process.env.DELAY_MS) || 0;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const fields = {
  bio: ['Japanese filmmaker and co-founder of Studio Ghibli.'],
  birthdate: ['1941-01-05'],
  nationality: ['Japanese'],
  website: ['https://example.com/miyazaki'],
  aliases: ['宮崎駿', 'Miyazaki Hayao'] // 宮崎駿 — CJK, for the tofu check
};

function readBody(req) {
  return new Promise((resolve) => {
    let b = '';
    req.on('data', (c) => (b += c));
    req.on('end', () => {
      try { resolve(JSON.parse(b || '{}')); } catch { resolve({}); }
    });
  });
}

http
  .createServer(async (req, res) => {
    res.setHeader('content-type', 'application/json');
    const url = req.url.split('?')[0];
    if (DELAY_MS && (url === '/resolve' || url === '/enrich')) await sleep(DELAY_MS);
    if (url === '/healthz') {
      return res.end(JSON.stringify({ status: 'ok', provider: NAME, version: 'stub-1' }));
    }
    if (url === '/describe') {
      return res.end(
        JSON.stringify({
          provider: NAME,
          version: 'stub-1',
          protocol_version: 1,
          entity_types: ['person'],
          id_namespaces: ['tmdb', 'imdb'],
          fields: Object.keys(fields)
        })
      );
    }
    if (url === '/resolve') {
      const body = await readBody(req);
      const q = ((body.hint && body.hint.query) || '').trim().toLowerCase();
      const hit = q.length >= 2 && 'hayao miyazaki'.includes(q);
      const candidates = hit
        ? [
            {
              external_id: 'tmdb:608',
              namespace: 'tmdb',
              label: 'Hayao Miyazaki',
              confidence: 0.9,
              disambiguation: 'Director · 1941 · Studio Ghibli'
            }
          ]
        : [];
      return res.end(JSON.stringify({ candidates }));
    }
    if (url === '/enrich') {
      return res.end(JSON.stringify({ fields }));
    }
    res.statusCode = 404;
    res.end('{}');
  })
  .listen(PORT, HOST, () => console.log(`F22 enrich stub on http://${HOST}:${PORT}`));
