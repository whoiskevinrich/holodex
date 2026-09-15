#!/usr/bin/env node
// ============================================================================
// Edition filename probe — READ-ONLY, ANONYMIZED OUTPUT (F60 / HOLODEX-377 sizing).
//
//   node scripts/probe-edition-filenames.mjs <dir> [<dir>...] [--show N]
//
// Walks each directory recursively, looks at every video file's basename, and
// reports how edition/cut markers actually appear in the library — so the 377
// parser is sized against real names rather than the spec's assumption:
//
//   plex   {edition-Director's Cut}      the strict grammar RD7 parses today
//   paren  (Director's Cut) / [Unrated]  loose, bracketed
//   dash   ... - Final Cut               loose, dash-suffixed
//   bare   ...Extended.Remastered...     keyword loose in the name
//
// Output is counts only: per class, per keyword, per extension, plus how many
// title groups hold MORE than one file once the marker is stripped (the
// multi-edition films 377 exists for). Nothing is written. Names are printed
// ONLY with --show N (N sample basenames per class) — keep that out of pastes.
// ============================================================================
import { readdir, stat } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

export const VIDEO_EXT = new Set([
  ".mkv", ".mp4", ".m4v", ".mov", ".avi", ".webm", ".wmv", ".ts", ".mpg", ".mpeg",
]);

// Keywords a cut/edition marker is made of. Order matters only for reporting.
export const KEYWORDS = [
  "director", "extended", "unrated", "uncut", "theatrical", "final", "ultimate",
  "special", "anniversary", "criterion", "redux", "remaster", "restored", "imax",
  "cut", "edition",
];
const KEYWORD_RE = new RegExp(`\\b(${KEYWORDS.join("|")})(?:'?s|ed)?\\b`, "gi");

// RD7: `{edition-<text>}` anywhere in the basename.
const PLEX_RE = /\{edition-([^}]*)\}/i;
// A bracketed group that contains a keyword: (Director's Cut), [Extended Edition].
const PAREN_RE = /[([]([^)\]]*)[)\]]/g;
// A " - <text>" tail whose text contains a keyword: "Blade Runner - Final Cut".
const DASH_RE = /\s[-–—]\s([^-–—]+)$/;

function hasKeyword(s) {
  KEYWORD_RE.lastIndex = 0;
  return KEYWORD_RE.test(s);
}

function keywordsIn(s) {
  KEYWORD_RE.lastIndex = 0;
  const out = new Set();
  for (const m of s.matchAll(KEYWORD_RE)) out.add(m[1].toLowerCase());
  return [...out];
}

// classify returns the marker class for a basename (extension already stripped),
// the marker text, and the title left once the marker is removed — the grouping
// key for "how many films have more than one edition on disk".
export function classify(stem) {
  const plex = stem.match(PLEX_RE);
  if (plex) {
    return { cls: "plex", marker: plex[1].trim(), title: titleKey(stem.replace(PLEX_RE, "")) };
  }
  PAREN_RE.lastIndex = 0;
  for (const m of stem.matchAll(PAREN_RE)) {
    if (hasKeyword(m[1])) {
      return { cls: "paren", marker: m[1].trim(), title: titleKey(stem.replace(m[0], "")) };
    }
  }
  const dash = stem.match(DASH_RE);
  if (dash && hasKeyword(dash[1])) {
    return { cls: "dash", marker: dash[1].trim(), title: titleKey(stem.slice(0, dash.index)) };
  }
  if (hasKeyword(stem)) {
    return { cls: "bare", marker: keywordsIn(stem).join(" "), title: titleKey(stem.replace(KEYWORD_RE, "")) };
  }
  return { cls: "none", marker: "", title: titleKey(stem) };
}

// titleKey folds a stem to a grouping key: lowercase, dots/underscores as spaces,
// a trailing "(1982)"/"1982" year kept (it IS identity, ADR-096 D3), whitespace
// collapsed, dangling separators trimmed.
export function titleKey(s) {
  return s
    .toLowerCase()
    .replace(/[._]+/g, " ")
    .replace(/\s+/g, " ")
    .replace(/^[\s\-–—]+|[\s\-–—]+$/g, "")
    .trim();
}

async function* walk(dir) {
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch (e) {
    process.stderr.write(`skip ${dir}: ${e.message}\n`);
    return;
  }
  for (const ent of entries) {
    const full = path.join(dir, ent.name);
    if (ent.isDirectory()) yield* walk(full);
    else if (ent.isFile() && VIDEO_EXT.has(path.extname(ent.name).toLowerCase())) yield full;
  }
}

export function summarize(files) {
  const byClass = { plex: 0, paren: 0, dash: 0, bare: 0, none: 0 };
  const byKeyword = {};
  const byExt = {};
  const samples = { plex: [], paren: [], dash: [], bare: [] };
  const groups = new Map(); // titleKey → { editions: Set<marker>, files: n }
  for (const file of files) {
    const base = path.basename(file);
    const ext = path.extname(base).toLowerCase();
    const stem = base.slice(0, -ext.length);
    const { cls, marker, title } = classify(stem);
    byClass[cls]++;
    byExt[ext] = (byExt[ext] ?? 0) + 1;
    if (cls !== "none") {
      for (const k of keywordsIn(marker || stem)) byKeyword[k] = (byKeyword[k] ?? 0) + 1;
      samples[cls].push(base);
    }
    const g = groups.get(title) ?? { editions: new Set(), files: 0 };
    g.editions.add(marker);
    g.files++;
    groups.set(title, g);
  }
  let multiEditionTitles = 0;
  let multiFileTitles = 0;
  for (const g of groups.values()) {
    if (g.files > 1) multiFileTitles++;
    if (g.editions.size > 1) multiEditionTitles++;
  }
  return { total: files.length, byClass, byKeyword, byExt, multiFileTitles, multiEditionTitles, samples };
}

function printReport(r, show) {
  const pct = (n) => (r.total ? `${((100 * n) / r.total).toFixed(1)}%` : "-");
  console.log(`video files: ${r.total}`);
  console.log("\nmarker class            files    share");
  for (const [cls, n] of Object.entries(r.byClass)) {
    console.log(`  ${cls.padEnd(20)} ${String(n).padStart(6)}   ${pct(n)}`);
  }
  const marked = r.total - r.byClass.none;
  console.log(`  ${"any marker".padEnd(20)} ${String(marked).padStart(6)}   ${pct(marked)}`);
  console.log("\nkeyword (in marked names)   files");
  for (const [k, n] of Object.entries(r.byKeyword).sort((a, b) => b[1] - a[1])) {
    console.log(`  ${k.padEnd(26)} ${String(n).padStart(5)}`);
  }
  console.log("\nextension   files");
  for (const [e, n] of Object.entries(r.byExt).sort((a, b) => b[1] - a[1])) {
    console.log(`  ${e.padEnd(10)} ${String(n).padStart(5)}`);
  }
  console.log(`\ntitle groups with >1 file:     ${r.multiFileTitles}`);
  console.log(`title groups with >1 edition:  ${r.multiEditionTitles}   (the multi-edition films 377 exists for)`);
  if (show > 0) {
    console.log(`\n--- samples (--show ${show}; contains real names, do not paste) ---`);
    for (const [cls, list] of Object.entries(r.samples)) {
      if (!list.length) continue;
      console.log(`${cls}:`);
      for (const b of list.slice(0, show)) console.log(`  ${b}`);
    }
  }
}

async function main(argv) {
  const dirs = [];
  let show = 0;
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === "--show") show = Number(argv[++i] ?? 0);
    else dirs.push(argv[i]);
  }
  if (!dirs.length) {
    console.error("usage: node scripts/probe-edition-filenames.mjs <dir> [<dir>...] [--show N]");
    process.exit(2);
  }
  const files = [];
  for (const d of dirs) {
    if (!(await stat(d)).isDirectory()) {
      console.error(`not a directory: ${d}`);
      process.exit(2);
    }
    for await (const f of walk(d)) files.push(f);
  }
  printReport(summarize(files), show);
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  await main(process.argv.slice(2));
}
