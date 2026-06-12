// Curated demo library for the Holodex showcase (docs/specs/showcase-demo-corpus.md).
//
// Every title, person, and studio here is fictional — invented for the demo so the
// corpus carries no third-party IP. The set is chosen to exercise the real UI:
//   - a spread of resolutions (SD / HD / FHD / 4K) so every badge bucket appears,
//   - realistic, varied runtimes (the duration badge),
//   - people and tags that repeat across titles so the People/Tags pages and the
//     facet filters look populated and connected,
//   - a diacritic title ("Amélie en Hiver") to show FTS folding (ADR-017),
//   - years from 2008–2024 for the year filter.
//
// `res` drives the encoded pixel dimensions (and therefore the resolution badge,
// width-based per ADR-012). `durSec` drives the duration badge. `art` selects the
// generated key-art treatment (see poster.mjs).

export const RES_DIMS = {
	SD: { w: 854, h: 480 },
	HD: { w: 1280, h: 720 },
	FHD: { w: 1920, h: 1080 },
	'4K': { w: 3840, h: 2160 }
};

// hh:mm:ss -> seconds, so the table below stays readable.
const dur = (h, m, s = 0) => h * 3600 + m * 60 + s;

export const items = [
	{
		slug: 'nightshade',
		title: 'Nightshade',
		people: ['Lana Reyes', 'Marcus Vane'],
		tags: ['Thriller', 'Noir'],
		year: 2021,
		res: '4K',
		durSec: dur(1, 58, 12),
		art: 'sun'
	},
	{
		slug: 'the-cartographer',
		title: 'The Cartographer',
		people: ['Eli Brandt'],
		tags: ['Drama'],
		year: 2019,
		res: 'FHD',
		durSec: dur(1, 42, 30),
		art: 'rings'
	},
	{
		slug: 'solar-drift',
		title: 'Solar Drift',
		people: ['Nadia Okonkwo', 'Marcus Vane'],
		tags: ['Sci-Fi', 'Adventure'],
		year: 2023,
		res: '4K',
		durSec: dur(2, 14, 5),
		art: 'horizon'
	},
	{
		slug: 'amelie-en-hiver',
		title: 'Amélie en Hiver',
		people: ['Camille Beaulieu'],
		tags: ['Romance', 'Drama'],
		year: 2008,
		res: 'HD',
		durSec: dur(2, 2, 0),
		art: 'bars'
	},
	{
		slug: 'concrete-garden',
		title: 'Concrete Garden',
		people: ['Yuki Tanaka'],
		tags: ['Documentary'],
		year: 2017,
		res: 'FHD',
		durSec: dur(1, 31, 48),
		art: 'grid'
	},
	{
		slug: 'static-bloom',
		title: 'Static Bloom',
		people: ['Priya Nair'],
		tags: ['Short', 'Experimental'],
		year: 2022,
		res: 'HD',
		durSec: dur(0, 48, 19),
		art: 'halftone'
	},
	{
		slug: 'harbor-lights',
		title: 'Harbor Lights',
		people: ['Tom Halloran', 'Grace Liu'],
		tags: ['Drama', 'Romance'],
		year: 2015,
		res: 'FHD',
		durSec: dur(1, 49, 2),
		art: 'horizon'
	},
	{
		slug: 'vantablack',
		title: 'Vantablack',
		people: ['Sofia Marchetti'],
		tags: ['Horror', 'Thriller'],
		year: 2020,
		res: '4K',
		durSec: dur(1, 36, 40),
		art: 'rings'
	},
	{
		slug: 'the-long-saturday',
		title: 'The Long Saturday',
		people: ['Dev Anand', 'Rosa Pike'],
		tags: ['Comedy'],
		year: 2013,
		res: 'HD',
		durSec: dur(1, 27, 33),
		art: 'sun'
	},
	{
		slug: 'ferrous',
		title: 'Ferrous',
		people: ['Anton Volkov', 'Nadia Okonkwo'],
		tags: ['Sci-Fi', 'Thriller'],
		year: 2024,
		res: '4K',
		durSec: dur(2, 8, 55),
		art: 'bars'
	},
	{
		slug: 'paper-moons',
		title: 'Paper Moons',
		people: ['Studio Lumen'],
		tags: ['Animation', 'Family'],
		year: 2018,
		res: 'FHD',
		durSec: dur(1, 35, 0),
		art: 'sun'
	},
	{
		slug: 'dust-and-echoes',
		title: 'Dust & Echoes',
		people: ['Cormac Reilly', 'Grace Liu'],
		tags: ['Western', 'Drama'],
		year: 2011,
		res: 'FHD',
		durSec: dur(2, 1, 27),
		art: 'horizon'
	},
	{
		slug: 'neon-tide',
		title: 'Neon Tide',
		people: ['Mei Chen', 'Rafael Cruz'],
		tags: ['Crime', 'Noir'],
		year: 2019,
		res: '4K',
		durSec: dur(1, 52, 9),
		art: 'halftone'
	},
	{
		slug: 'the-quiet-coast',
		title: 'The Quiet Coast',
		people: ['Ingrid Sø'],
		tags: ['Drama'],
		year: 2016,
		res: 'HD',
		durSec: dur(1, 18, 44),
		art: 'horizon'
	},
	{
		slug: 'overgrowth',
		title: 'Overgrowth',
		people: ['Amara Diallo'],
		tags: ['Documentary', 'Nature'],
		year: 2021,
		res: '4K',
		durSec: dur(0, 52, 30),
		art: 'grid'
	},
	{
		slug: 'tin-soldier',
		title: 'Tin Soldier',
		people: ['Henrik Møller', 'Rosa Pike'],
		tags: ['War', 'Drama'],
		year: 2009,
		res: 'SD',
		durSec: dur(1, 44, 12),
		art: 'bars'
	},
	{
		slug: 'glasshouse',
		title: 'Glasshouse',
		people: ['Olivia Frost', 'Mei Chen'],
		tags: ['Mystery', 'Thriller'],
		year: 2022,
		res: 'FHD',
		durSec: dur(1, 39, 51),
		art: 'rings'
	},
	{
		slug: 'migration',
		title: 'Migration',
		people: ['Kwame Mensah'],
		tags: ['Documentary', 'Nature'],
		year: 2020,
		res: '4K',
		durSec: dur(1, 14, 6),
		art: 'horizon'
	}
];
