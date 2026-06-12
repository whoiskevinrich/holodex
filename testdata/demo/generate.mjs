// Generates the Holodex demo showcase library (docs/specs/showcase-demo-corpus.md).
//
// For each curated item it renders generated key-art (poster.mjs -> JPEG via sharp)
// then muxes a tiny MP4 with ffmpeg:
//   - video stream  : the poster, encoded at the item's target resolution (sets the
//                     width-based resolution badge, ADR-012) for the item's runtime
//                     at 1 fps (a near-zero-bitrate static clip — small + fast),
//   - attached_pic  : the same poster as the container's cover art, which Holodex
//                     extracts at index time (ADR-009 Tier 1) and shows on the card.
// The result: pointing MEDIA_PATH at the output directory yields a full, good-looking,
// fully metadata-driven library — no real video files required.
//
// Usage:
//   npm install            # once (pulls sharp)
//   npm run generate       # full corpus -> ./library
//   node generate.mjs --only nightshade          # one item
//   node generate.mjs --max-seconds 8            # cap runtimes (fast smoke build)
//   node generate.mjs --out /path/to/media       # custom output dir
//
// Requires ffmpeg on PATH (same dependency as the app, ADR-004/007).

import { execFile } from 'node:child_process';
import { mkdir, rm, stat } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import sharp from 'sharp';
import { items, RES_DIMS } from './items.mjs';
import { buildSVG } from './poster.mjs';

const run = promisify(execFile);
const here = dirname(fileURLToPath(import.meta.url));

// Cover-art size embedded as the container's attached_pic (the card thumbnail).
const THUMB = { w: 640, h: 360 };

function parseArgs(argv) {
	const a = { out: join(here, 'library'), only: null, maxSeconds: Infinity };
	for (let i = 0; i < argv.length; i++) {
		const v = argv[i];
		if (v === '--out') a.out = argv[++i];
		else if (v === '--only') a.only = argv[++i];
		else if (v === '--max-seconds') a.maxSeconds = Number(argv[++i]);
	}
	return a;
}

const hms = (sec) => {
	const h = Math.floor(sec / 3600);
	const m = Math.floor((sec % 3600) / 60);
	const s = Math.floor(sec % 60);
	const mm = String(m).padStart(2, '0');
	const ss = String(s).padStart(2, '0');
	return h > 0 ? `${h}:${mm}:${ss}` : `${m}:${ss}`;
};

async function ensureFfmpeg() {
	try {
		await run('ffmpeg', ['-version']);
	} catch {
		console.error('ERROR: ffmpeg not found on PATH. Install ffmpeg and retry.');
		process.exit(1);
	}
}

// buildItem renders the key-art and muxes one MP4. Returns the output file size so
// the caller can total only what this run produced. `index` is the item's position
// in the full list, so the poster palette is stable regardless of --only filtering.
async function buildItem(item, index, args, postersDir, outDir) {
	const dims = RES_DIMS[item.res];
	const durSec = Math.min(item.durSec, args.maxSeconds);

	const full = join(postersDir, `${item.slug}.full.jpg`);
	const thumb = join(postersDir, `${item.slug}.thumb.jpg`);
	// One SVG decode shared by both encodes via clone() (sharp's multi-output pattern).
	const base = sharp(Buffer.from(buildSVG(item, index)));
	await Promise.all([
		base.clone().resize(dims.w, dims.h).jpeg({ quality: 88 }).toFile(full),
		base.clone().resize(THUMB.w, THUMB.h).jpeg({ quality: 85 }).toFile(thumb)
	]);

	const out = join(outDir, `${item.slug}.mp4`);
	const ffArgs = [
		'-y',
		'-loglevel', 'error',
		'-loop', '1', '-r', '1', '-t', String(durSec), '-i', full, // video source: poster
		'-i', thumb, // attached cover art
		'-map', '0:v:0', '-map', '1:v:0',
		'-c:v:0', 'libx264', '-preset', 'ultrafast', '-tune', 'stillimage', '-pix_fmt', 'yuv420p',
		'-c:v:1', 'copy', '-disposition:v:1', 'attached_pic',
		'-metadata', `title=${item.title}`,
		'-metadata', `artist=${item.people.join(', ')}`,
		'-metadata', `genre=${item.tags.join(', ')}`,
		'-metadata', `date=${item.year}-01-01`,
		'-movflags', '+faststart',
		out
	];
	const t0 = Date.now();
	await run('ffmpeg', ffArgs, { maxBuffer: 1 << 24 });
	const secs = ((Date.now() - t0) / 1000).toFixed(1);
	const { size } = await stat(out);
	console.log(
		`  ✓ ${item.slug.padEnd(20)} ${item.res.padEnd(3)} ${hms(durSec).padStart(8)}  ${(size / 1024).toFixed(0)} KB  (${secs}s)`
	);
	return size;
}

async function main() {
	const args = parseArgs(process.argv.slice(2));
	await ensureFfmpeg();

	// Pair each item with its stable index up front, then filter, so the palette
	// index survives --only without an indexOf lookup in the loop.
	let entries = items.map((item, index) => ({ item, index }));
	if (args.only) entries = entries.filter((e) => e.item.slug === args.only);
	if (entries.length === 0) {
		console.error(`No items matched (--only ${args.only}).`);
		process.exit(1);
	}

	const postersDir = join(here, '.posters');
	await rm(postersDir, { recursive: true, force: true });
	await mkdir(postersDir, { recursive: true });
	await mkdir(args.out, { recursive: true });

	console.log(`Generating ${entries.length} demo title(s) -> ${args.out}\n`);
	const start = Date.now();
	let total = 0;
	try {
		for (const { item, index } of entries) {
			total += await buildItem(item, index, args, postersDir, args.out);
		}
	} finally {
		await rm(postersDir, { recursive: true, force: true });
	}

	console.log(
		`\nDone in ${((Date.now() - start) / 1000).toFixed(1)}s — ${entries.length} files, ${(total / 1024 / 1024).toFixed(1)} MB total.`
	);
	console.log(`\nPoint Holodex at it:\n  MEDIA_PATH=${args.out} go run ./cmd/holodex`);
}

main().catch((err) => {
	console.error(err);
	process.exit(1);
});
