// Nationality → country resolution (HOLODEX-139). The `nationality` canonical field
// carries free text — TMDB supplies a person's place of birth (e.g. "London, England,
// United Kingdom"), and the contract (§4.2) also permits a plain nationality word
// (e.g. "British"). This module maps that text to an ISO 3166-1 alpha-2 country so the
// person hero can show a flag. It is deliberately pure (no Vite/asset imports) so it is
// unit-testable; the flag image lives in flags.ts, keyed by the `code` returned here.
import { COUNTRY_NAMES } from './countryNames';

export interface Country {
	/** ISO 3166-1 alpha-2, lowercase (e.g. "gb"). */
	code: string;
	/** English display name for the tooltip / alt text (e.g. "United Kingdom"). */
	name: string;
}

// fold normalizes a lookup token: strip diacritics, drop periods, lowercase, collapse
// whitespace. So "Türkiye", "U.S.A.", and "  France " all reduce to stable keys.
function fold(s: string): string {
	return s
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.toLowerCase()
		.replace(/\./g, '')
		.replace(/\s+/g, ' ')
		.trim();
}

// Alternate phrasings that differ from the canonical country name — the country slot in
// a place-of-birth string, historical names, and common abbreviations. Folded keys.
const SYNONYMS: Record<string, string> = {
	usa: 'us',
	us: 'us',
	'united states': 'us',
	'united states of america': 'us',
	america: 'us',
	uk: 'gb',
	'united kingdom': 'gb',
	'great britain': 'gb',
	britain: 'gb',
	england: 'gb',
	scotland: 'gb',
	wales: 'gb',
	'northern ireland': 'gb',
	'russian federation': 'ru',
	'soviet union': 'ru',
	ussr: 'ru',
	'south korea': 'kr',
	korea: 'kr',
	'republic of korea': 'kr',
	'north korea': 'kp',
	czechia: 'cz',
	czechoslovakia: 'cz',
	turkey: 'tr',
	'ivory coast': 'ci',
	burma: 'mm',
	'cape verde': 'cv',
	'east timor': 'tl',
	palestine: 'ps',
	'vatican city': 'va',
	vatican: 'va',
	'the netherlands': 'nl',
	holland: 'nl',
	uae: 'ae',
	'republic of ireland': 'ie',
	macao: 'mo',
	'west germany': 'de',
	'east germany': 'de',
	'taiwan province of china': 'tw',
	'democratic republic of congo': 'cd',
	'dr congo': 'cd',
	'republic of the congo': 'cg',
	congo: 'cg'
};

// Nationality adjectives (demonyms) for a provider that emits a nationality word rather
// than a place of birth (contract §4.2 example: "British"). A modest, common-cases set;
// an unlisted demonym simply yields no flag (a supported degrade). Folded keys.
const DEMONYMS: Record<string, string> = {
	american: 'us',
	british: 'gb',
	english: 'gb',
	scottish: 'gb',
	welsh: 'gb',
	irish: 'ie',
	french: 'fr',
	german: 'de',
	italian: 'it',
	spanish: 'es',
	portuguese: 'pt',
	dutch: 'nl',
	belgian: 'be',
	swiss: 'ch',
	austrian: 'at',
	swedish: 'se',
	norwegian: 'no',
	danish: 'dk',
	finnish: 'fi',
	icelandic: 'is',
	polish: 'pl',
	czech: 'cz',
	russian: 'ru',
	ukrainian: 'ua',
	greek: 'gr',
	turkish: 'tr',
	hungarian: 'hu',
	romanian: 'ro',
	japanese: 'jp',
	chinese: 'cn',
	korean: 'kr',
	indian: 'in',
	australian: 'au',
	'new zealander': 'nz',
	canadian: 'ca',
	mexican: 'mx',
	brazilian: 'br',
	argentine: 'ar',
	argentinian: 'ar',
	chilean: 'cl',
	colombian: 'co',
	cuban: 'cu',
	egyptian: 'eg',
	israeli: 'il',
	iranian: 'ir',
	'south african': 'za',
	nigerian: 'ng',
	filipino: 'ph',
	thai: 'th',
	vietnamese: 'vn',
	indonesian: 'id',
	malaysian: 'my',
	pakistani: 'pk'
};

// One folded-name → code index built once from the canonical names plus the overlays.
const nameToCode: Record<string, string> = (() => {
	const idx: Record<string, string> = {};
	for (const [code, name] of Object.entries(COUNTRY_NAMES)) idx[fold(name)] = code;
	for (const [name, code] of Object.entries(SYNONYMS)) idx[name] = code;
	for (const [name, code] of Object.entries(DEMONYMS)) idx[name] = code;
	return idx;
})();

function lookup(token: string): Country | null {
	const code = nameToCode[fold(token)];
	if (!code) return null;
	return { code, name: COUNTRY_NAMES[code] ?? token };
}

// countryFromNationality maps one nationality value to a country, or null when it can't
// be resolved (rendered as no flag). A comma-separated value follows the place-of-birth
// convention "City, Region, Country" — the country is the last segment; a single token
// is tried as a country name or a demonym.
export function countryFromNationality(value: string): Country | null {
	const v = (value ?? '').trim();
	if (!v) return null;
	const parts = v
		.split(',')
		.map((p) => p.trim())
		.filter(Boolean);
	if (parts.length >= 2) {
		const hit = lookup(parts[parts.length - 1]);
		if (hit) return hit;
	}
	return lookup(v);
}

// countriesFromNationality resolves every value, de-duplicates by code (order-preserving),
// and drops the unresolved ones — the hero shows the first as the primary flag and the
// rest as a "+N" count.
export function countriesFromNationality(values: string[]): Country[] {
	const seen = new Set<string>();
	const out: Country[] = [];
	for (const value of values ?? []) {
		const c = countryFromNationality(value);
		if (c && !seen.has(c.code)) {
			seen.add(c.code);
			out.push(c);
		}
	}
	return out;
}
