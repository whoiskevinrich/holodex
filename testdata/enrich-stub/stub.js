// Metadata-source provider STUB for manual QA / preview (ADR-033).
//
// This is NOT used in production or `go test` (those use the in-process
// `enrich.Fake`). It's a tiny runnable HTTP provider so a human or agent can
// exercise the live enrichment flow with no network, no API keys, and no real
// sidecar container.
//
// ONE PROCESS, MANY PROVIDERS (HOLODEX-348)
//
// Core builds a request URL by string-concatenating the configured base_url with
// "/describe" / "/resolve" / "/enrich" (internal/enrich/client.go:94) — it never
// re-parses the base or replaces its path. So a base_url that already carries a
// path just works, and N config entries pointing at N path prefixes on this one
// process are N independent providers as far as core is concerned:
//
//     http://127.0.0.1:9100/p/alpha   ->  GET /p/alpha/describe, POST /p/alpha/resolve, ...
//
// That is what makes "five providers disagreeing about one field" cost no extra
// processes. The persona table below is the whole configuration; the fixture's
// committed registry (testdata/stressseed/sources.yaml) is its mirror image.
//
// The bare routes (/describe, /resolve, /enrich, /healthz) still serve the
// original single `fake` persona unchanged, so the F22 QA checklist and any
// metadata-sources.yaml already pointing at http://127.0.0.1:9100 keep working.
//
// Start it:
//     preview_start enrich-stub
//   or directly:
//     node testdata/enrich-stub/stub.js          # honours PORT/HOST, defaults 127.0.0.1:9100
//
// See README.md for wiring it to a running backend.
const http = require('http');
const zlib = require('zlib');

const PORT = Number(process.env.PORT) || 9100;
const HOST = process.env.HOST || '127.0.0.1';
// DELAY_MS slows the user-facing /resolve + /enrich (not /healthz or /describe) on
// EVERY persona, to simulate a slow network fleet-wide. 0 = instant. The `slow`
// persona below is delayed on its own regardless, so the loading state stays
// reachable without making every other provider unusable.
const DELAY_MS = Number(process.env.DELAY_MS) || 0;
const SLOW_MS = Number(process.env.SLOW_MS) || 1500;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---------------------------------------------------------------------------
// Brand icons
//
// Core downloads a provider's advertised brand icon and re-encodes it as JPEG
// (internal/api/provider_icon.go), so the icon has to be a real raster — an SVG
// would not decode. Rather than embed a blob, synthesize a solid-colour PNG:
// distinct colours are the entire point of an icon in a five-chip row, and a
// generator keeps this file dependency-free, which is the stub's one promise.
//
// ADR-039: the icon must be served from the provider's own base_url host (or a
// configured asset_host). Every persona here shares this process's host, so
// serving each icon from the persona's own path prefix satisfies that by
// construction.
const CRC_TABLE = (() => {
  const t = new Int32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c;
  }
  return t;
})();

function crc32(buf) {
  let c = -1;
  for (let i = 0; i < buf.length; i++) c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
  return (c ^ -1) >>> 0;
}

function pngChunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length, 0);
  const typed = Buffer.concat([Buffer.from(type, 'ascii'), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(typed), 0);
  return Buffer.concat([len, typed, crc]);
}

// solidPng renders a size×size truecolour PNG in one flat colour.
function solidPng(size, [r, g, b]) {
  const stride = size * 3 + 1; // one filter byte per scanline
  const raw = Buffer.alloc(size * stride);
  for (let y = 0; y < size; y++) {
    const o = y * stride;
    raw[o] = 0; // filter: none
    for (let x = 0; x < size; x++) {
      raw[o + 1 + x * 3] = r;
      raw[o + 2 + x * 3] = g;
      raw[o + 3 + x * 3] = b;
    }
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(size, 0);
  ihdr.writeUInt32BE(size, 4);
  ihdr[8] = 8; // bit depth
  ihdr[9] = 2; // colour type: truecolour
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    pngChunk('IHDR', ihdr),
    pngChunk('IDAT', zlib.deflateSync(raw)),
    pngChunk('IEND', Buffer.alloc(0))
  ]);
}

// ---------------------------------------------------------------------------
// The persona table
//
// Three groups, and the split is deliberate — a persona that both disagrees about
// a field AND returns a 5xx would make a failure unattributable, which is the
// same one-factor-at-a-time rule the seeder's ladder follows (spec D3). See
// personas.json for what each group means.
//
// `values` is keyed by canonical field. Person fields are canonical in Go
// (internal/api/person_fields.go), so a person's chip row needs no mapping at
// all; the video fields need `<name>:<field>` in the mapping's `sources:` list,
// which testdata/stressseed/mappings.yaml carries.
// Core checks entity-type support against the YAML `entity_types`, not against
// /describe, so this list is advisory — but an honest one keeps the two readable
// side by side.
const ALL_ENTITY_TYPES = ['person', 'video', 'studio', 'film'];

// The table lives in personas.json because the seeder reads it too — it writes
// these same values into entity_enrichment so the fixture boots with a populated
// chip row. Two copies would drift, and a drifted copy means the fixture states a
// value the page stops rendering the moment anyone hits Refresh.
const PERSONAS = require('./personas.json').personas;

// The original single persona, still served on the bare routes.
const LEGACY = {
  slug: '',
  name: 'fake',
  icon: null,
  entityTypes: ['person'],
  values: {
    bio: ['Japanese filmmaker and co-founder of Studio Ghibli.'],
    birthdate: ['1941-01-05'],
    nationality: ['Japanese'],
    website: ['https://example.com/miyazaki'],
    aliases: ['宮崎駿', 'Miyazaki Hayao'] // 宮崎駿 — CJK, for the tofu check
  }
};

const BY_SLUG = new Map(PERSONAS.map((p) => [p.slug, p]));

// ---------------------------------------------------------------------------
// Candidates
//
// Confidence is load-bearing. A LONE candidate at or above 0.85 is auto-applied
// without the picker ever opening (EnrichPicker.svelte), so:
//   - precedence personas return one strong candidate, because one click that
//     just stores a value is what populating a five-chip row wants;
//   - adoption personas stay below the threshold, or the list they exist to
//     stress would never be shown.
const STRONG = 0.9;
const WEAK = 0.6;

// The namespace IS the prefix before the first colon — not a label beside it. The
// contract says so twice (metadata-provider-contract.md §4.1 and its conformance
// list), core's own in-process provider derives it the same way
// (internal/enrich/fake.go), and ADR-082 stores the pair as one `<namespace>:<id>`
// string that the UI splits back apart. This file is the contract's designated
// reference implementation, so a hand-written namespace that disagrees with the id it
// sits next to is copied outward as well as being wrong here.
function namespaceOf(externalID) {
  const colon = externalID.indexOf(':');
  // Not `slice(0, indexOf(...))`: indexOf returns -1 when there is no colon, and
  // slice(0, -1) would quietly hand back the id minus its last character — a namespace
  // that is a near-miss of the real one, which is far harder to notice than nothing.
  if (colon < 0) throw new Error(`external_id ${JSON.stringify(externalID)} has no ":" — it must be <namespace>:<id>`);
  return externalID.slice(0, colon);
}

// candidate builds one candidate with its namespace derived from the id rather than
// written beside it, so the pair cannot be edited apart. That divergence is the whole
// defect this shape exists to prevent, and a hand-written second copy of the prefix
// re-creates it the moment somebody edits one of the two.
const candidate = (externalID, rest) => ({ external_id: externalID, namespace: namespaceOf(externalID), ...rest });

// Which namespace a persona issues ids in, read off the candidates it actually returns
// rather than restated. §4.1 — "You MUST emit ids only in advertised namespaces" — is a
// claim about /describe agreeing with /resolve, so deriving one from the other is the
// only version of it that cannot drift. Personas answer to their own name because that
// is the id the seeder records (testdata/stressseed/enrichment.go), so a Refresh against
// the running stub re-fetches the same record.
function idNamespaceFor(persona) {
  return namespaceOf(candidatesFor(persona, '')[0].external_id);
}

function candidatesFor(persona, query) {
  if (persona.candidates === 'flood') {
    return Array.from({ length: 30 }, (_, i) =>
      candidate(`flood:${100 + i}`, {
        label: `${query || 'Candidate'} ${String(i + 1).padStart(2, '0')}`,
        confidence: Number((WEAK - i * 0.01).toFixed(2)),
        disambiguation: `Result ${i + 1} of 30 · flooded list`
      })
    );
  }
  if (persona.candidates === 'twins') {
    const where = [
      'Director · 1941 · Studio Ghibli',
      'Animator · 1941 · Toei Animation',
      'Producer · 1963 · Nippon Animation',
      'Writer · 1978 · Telecom Animation',
      'Director · 1985 · Topcraft',
      'Editor · 1971 · A-Pro',
      'Composer · 1950 · unaffiliated',
      'Director · 1941 · Studio Ghibli' // a genuine duplicate: even the tiebreaker ties
    ];
    return where.map((d, i) =>
      candidate(`twins:${200 + i}`, {
        label: 'Hayao Miyazaki',
        confidence: WEAK,
        disambiguation: d
      })
    );
  }
  return [
    candidate(`${persona.name}:608`, {
      label: 'Hayao Miyazaki',
      confidence: STRONG,
      disambiguation: `Director · 1941 · via ${persona.name}`
    })
  ];
}

// ---------------------------------------------------------------------------

function describeFor(persona, origin) {
  const body = {
    provider: persona.name,
    version: 'stub-1',
    protocol_version: 1, // core hard-refuses anything else (internal/enrich/enrich.go)
    entity_types: persona.entityTypes || ALL_ENTITY_TYPES,
    // Only what this persona actually issues. §4.1: "You MUST emit ids only in
    // namespaces you advertise" — advertising `tmdb` while emitting `alpha:608` is
    // the same disagreement from the other end.
    id_namespaces: [idNamespaceFor(persona)],
    fields: Object.keys(persona.values || {})
  };
  if (persona.icon) {
    const path = persona.slug ? `/p/${persona.slug}` : '';
    body.brand_icon = { url: `${origin}${path}/icon.png` };
  }
  return body;
}

function readBody(req) {
  return new Promise((resolve) => {
    let b = '';
    req.on('data', (c) => (b += c));
    req.on('end', () => {
      try {
        resolve(JSON.parse(b || '{}'));
      } catch {
        resolve({});
      }
    });
  });
}

// route splits a request path into its persona and endpoint. `/p/<slug>/resolve`
// addresses a persona; a bare `/resolve` is the legacy one.
function route(path) {
  const m = /^\/p\/([^/]+)(\/.*)?$/.exec(path);
  if (!m) return { persona: LEGACY, endpoint: path };
  return { persona: BY_SLUG.get(m[1]), endpoint: m[2] || '/' };
}

// Exported so the conformance test can check the persona table without binding a port;
// the server only starts when this file is run directly, not when it is required.
module.exports = { PERSONAS, LEGACY, candidatesFor, describeFor, namespaceOf, idNamespaceFor };

if (require.main !== module) return;

http
  .createServer(async (req, res) => {
    const path = req.url.split('?')[0];
    const { persona, endpoint } = route(path);

    if (!persona) {
      res.statusCode = 404;
      res.setHeader('content-type', 'application/json');
      return res.end(JSON.stringify({ error: 'unknown persona', known: [...BY_SLUG.keys()] }));
    }

    if (endpoint === '/icon.png' && persona.icon) {
      const png = solidPng(64, persona.icon);
      res.setHeader('content-type', 'image/png');
      res.setHeader('content-length', png.length);
      return res.end(png);
    }

    res.setHeader('content-type', 'application/json');

    // /healthz and /describe are never delayed or faulted: a provider that cannot
    // be discovered is a different bug from one that cannot answer, and core calls
    // /describe on the hot path of every resolve and enrich (verifiedClient).
    if (endpoint === '/healthz') {
      return res.end(JSON.stringify({ status: 'ok', provider: persona.name, version: 'stub-1' }));
    }
    if (endpoint === '/describe') {
      return res.end(JSON.stringify(describeFor(persona, `http://${req.headers.host || `${HOST}:${PORT}`}`)));
    }

    const userFacing = endpoint === '/resolve' || endpoint === '/enrich';
    if (userFacing) {
      const delay = DELAY_MS + (persona.fault === 'slow' ? SLOW_MS : 0);
      if (delay) await sleep(delay);
      if (persona.fault === 'error') {
        res.statusCode = 503;
        return res.end(JSON.stringify({ error: 'stub is deliberately unavailable' }));
      }
      if (persona.fault === 'malformed') {
        // Valid HTTP, invalid JSON — core reports "decode provider response"
        // rather than a status error (internal/enrich/client.go).
        return res.end('{"fields": {"bio": ["unterminated');
      }
    }

    if (endpoint === '/resolve') {
      const body = await readBody(req);
      const q = ((body.hint && body.hint.query) || '').trim().toLowerCase();
      // The legacy persona keeps its substring gate so the no-results path stays
      // reachable; the stress personas answer anything, because the fixture's
      // entities are not named after a real person.
      if (persona === LEGACY) {
        const hit = q.length >= 2 && 'hayao miyazaki'.includes(q);
        return res.end(JSON.stringify({ candidates: hit ? candidatesFor(persona, q) : [] }));
      }
      return res.end(JSON.stringify({ candidates: candidatesFor(persona, body.hint && body.hint.query) }));
    }

    if (endpoint === '/enrich') {
      return res.end(JSON.stringify({ fields: persona.values || {} }));
    }

    res.statusCode = 404;
    res.end('{}');
  })
  .listen(PORT, HOST, () =>
    console.log(
      `enrich stub on http://${HOST}:${PORT} — legacy persona at /, ` +
        `${PERSONAS.length} stress personas at /p/{${[...BY_SLUG.keys()].join(',')}}`
    )
  );
