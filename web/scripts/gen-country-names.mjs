// Regenerates src/lib/countryNames.ts from flag-icons' country.json (MIT). Run after
// bumping the flag-icons dependency: `node scripts/gen-country-names.mjs`. Emits an ISO
// 3166-1 alpha-2 (lowercase) → English display-name map for the nationality flag tooltip
// (see nationality.ts). A couple of names are shortened for display.
import { createRequire } from 'node:module';
import { writeFileSync } from 'node:fs';

const require = createRequire(import.meta.url);
const countries = require('flag-icons/country.json').filter((c) => c.iso);

const OVERRIDE = { us: 'United States', gb: 'United Kingdom' };
const entries = countries
	.map((c) => [c.code, OVERRIDE[c.code] || c.name])
	.sort((a, b) => (a[0] < b[0] ? -1 : 1));

let out = '// AUTO-GENERATED from flag-icons country.json (MIT). Do not edit by hand.\n';
out += '// Regenerate: node scripts/gen-country-names.mjs  (see nationality.ts).\n';
out += '// Maps ISO 3166-1 alpha-2 (lowercase) → English display name for the flag tooltip.\n\n';
out += 'export const COUNTRY_NAMES: Record<string, string> = {\n';
for (const [code, name] of entries) out += `\t${code}: ${JSON.stringify(name)},\n`;
out += '};\n';

writeFileSync(new URL('../src/lib/countryNames.ts', import.meta.url), out);
console.log(`wrote src/lib/countryNames.ts with ${entries.length} entries`);
