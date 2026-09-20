#!/usr/bin/env node
// feature-claims.mjs — "which feature number (F##) is actually free?"
//
// The F-number twin of adr-claims.mjs. Feature numbers are claimed on a branch long
// before they reach main, so "highest spec heading on main + 1" collides with whatever
// is in flight: on 2026-09-17 HOLODEX-390 and HOLODEX-406 both took F63, and the one
// that merged second had to be renumbered across 53 references (HOLODEX-407).
//
// ADRs claim a number via filename; features claim via the spec's H1 —
// `# Spec: Candidate thumbnail in the resolve picker (F64)` in docs/specs/*.md — so
// this scans the first `# ` line of every spec on every local and remote branch. A
// `# QA: …` companion doc restates its feature's number and is not a claim.
//
// Two things differ from the ADR rule, because the spec corpus already breaks it:
//   * Sub-features (`F56.2`, `F21.3`) are children, not rivals, of their parent number.
//     They are tracked by their full id and only the integer part feeds "next free".
//   * Two specs on main can share a bare number (F55, F56 both do). That is a merged
//     fact, not something a script can fix, so a collision is flagged only when a spec
//     that is NOT on main claims a number some other spec also claims — the in-flight
//     case that renumbering can still catch before merge.
//
// Same mechanics otherwise: derived from git (read-only, never fetches), cached in a
// gitignored file at the MAIN worktree root, --reserve for a number not yet in git.
// Shared helpers are imported from adr-claims.mjs; pure helpers here are exported for
// scripts/feature-claims.test.mjs; main() runs only when executed directly.
//
// Usage:
//   node scripts/feature-claims.mjs                 # refresh the file, print the next free number
//   node scripts/feature-claims.mjs --print         # print only, do not write the file
//   node scripts/feature-claims.mjs --reserve slug  # hold the next free number for un-pushed work
//   node scripts/feature-claims.mjs --release 65    # drop a reservation you did not use

import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import {
  STALE_DAYS,
  daysSince,
  describeRivals,
  git,
  mainWorktreeRoot,
  nextFree,
  parseReservations,
  pruneReservations,
  rankRef,
  refsBySha,
} from "./adr-claims.mjs";

export const CLAIMS_FILENAME = ".feature-claims";
const SPEC_GLOB = "docs/specs/*.md";

// "# Spec: Candidate thumbnail in the resolve picker (F64)" -> { num: 64, id: "64" }
// "# Unified name-edit mechanism (F56.2, HOLODEX-269)"     -> { num: 56, id: "56.2" }
// "# Spec: Job History — Digest, … (F21.3b)"                -> { num: 21, id: "21.3b" }
// "# Spec: Queryable fields substrate … (F46 Phase 1)"     -> { num: 46, id: "46-phase1" }
// "# QA: Metadata Writeback (F28)"                          -> null  (companion doc)
// "# Spec: Sticky sort preferences + Random sort"           -> null  (unnumbered)
export function parseHeading(line) {
  if (!/^#\s/.test(line) || /^#\s*QA\b/i.test(line)) return null;
  const m = /\bF(\d{1,3})(\.\d+[a-z]?)?(?:\s+Phase\s+(\d+))?\b/i.exec(line);
  if (!m) return null;
  return { num: Number(m[1]), id: m[1] + (m[2] ?? "") + (m[3] ? `-phase${m[3]}` : "") };
}

// docs/specs/candidates-image.md -> candidates-image
export function specSlug(path) {
  return path.replace(/^.*\//, "").replace(/\.md$/, "");
}

// One row per feature id, keeping the best-ranked ref as canonical. `alsoOn` lists the
// other specs wearing the same id (one entry per rival slug with a representative ref and
// a count), and `collision` is set only when at least one of those specs is not on main.
export function collapseClaims(rows) {
  const byId = new Map();
  for (const row of rows) {
    if (!byId.has(row.id)) byId.set(row.id, new Map());
    const bySlug = byId.get(row.id);
    const seen = bySlug.get(row.slug);
    if (!seen) bySlug.set(row.slug, { slug: row.slug, ref: row.ref, count: 1, num: row.num });
    else {
      seen.count += 1;
      if (rankRef(row.ref) < rankRef(seen.ref)) seen.ref = row.ref;
    }
  }
  return [...byId]
    .map(([id, bySlug]) => {
      const [winner, ...rivals] = [...bySlug.values()].sort(
        (a, b) => rankRef(a.ref) - rankRef(b.ref) || a.slug.localeCompare(b.slug),
      );
      return {
        id,
        num: winner.num,
        slug: winner.slug,
        ref: winner.ref,
        alsoOn: rivals.map(({ slug, ref, count }) => ({ slug, ref, count })),
        collision: rivals.length > 0 && [winner, ...rivals].some((e) => rankRef(e.ref) !== 0),
      };
    })
    .sort((a, b) => a.num - b.num || a.id.localeCompare(b.id, undefined, { numeric: true }));
}

export function collisions(claims) {
  return claims.filter((c) => c.collision);
}

export function renderFile(claims, reservations, next, now = new Date()) {
  const lines = [
    `# ${CLAIMS_FILENAME} — claimed feature numbers, derived from git. NOT COMMITTED.`,
    "#",
    "# Generated by scripts/feature-claims.mjs. Do not hand-edit the derived rows; they are",
    "# rebuilt from the H1 of every docs/specs/*.md on every local and remote branch on each",
    "# run. RESERVED rows are local holds for work you have not pushed yet, and are dropped",
    "# automatically once the number appears in git.",
    "#",
    "#   node scripts/feature-claims.mjs                 refresh + print the next free number",
    "#   node scripts/feature-claims.mjs --reserve slug  hold the next free number",
    "#   node scripts/feature-claims.mjs --release NN    drop an unused hold",
    "#",
    `# Generated: ${now.toISOString()}`,
    `# Next free: F${next}`,
    "",
  ];

  const collided = collisions(claims);
  if (collided.length) {
    lines.push("# !! COLLISIONS — one number, two different specs, at least one not on main. Renumber before merging.");
    for (const c of collided) {
      lines.push(`#    F${c.id}  ${c.slug} @ ${c.ref}  VS  ${describeRivals(c)}`);
    }
    lines.push("");
  }

  for (const c of claims) {
    lines.push(`F${c.id.padEnd(9)}  ${c.slug.padEnd(46)}  ${c.ref}`);
    // Shared numbers already on main are a fact worth seeing, not a failure.
    if (!c.collision && c.alsoOn.length) {
      lines.push(`${"".padEnd(10)}  shared with ${c.alsoOn.map((o) => o.slug).join(", ")}`);
    }
  }
  for (const r of reservations) {
    const age = daysSince(r.at, now);
    const stale = age >= STALE_DAYS ? `  (stale, ${age}d — release it if abandoned)` : "";
    lines.push(`F${String(r.num).padEnd(9)}  RESERVED  ${r.slug}  ${r.at}${stale}`);
  }
  lines.push("");
  return lines.join("\n");
}

// ---------------------------------------------------------------- git + IO (impure)

// `git grep` output is "<sha>:<path>:<line>"; the first "# " line per file is its H1.
export function parseGrep(text, ref) {
  const firstByPath = new Map();
  for (const line of text.split("\n")) {
    const m = /^[0-9a-f]+:([^:]+):(.*)$/.exec(line);
    if (!m || firstByPath.has(m[1])) continue;
    firstByPath.set(m[1], m[2]);
  }
  const rows = [];
  for (const [path, h1] of firstByPath) {
    const parsed = parseHeading(h1);
    if (parsed) rows.push({ ...parsed, slug: specSlug(path), ref });
  }
  return rows;
}

// Exported for scripts/hooks/feature-claims-guard.mjs, which needs the same claim set
// at /write-spec time without shelling out and re-parsing this script's output.
export function scanRefs(cwd) {
  const rows = [];
  for (const [sha, ref] of refsBySha(cwd)) {
    let out;
    try {
      out = git(["grep", "-E", "^# ", sha, "--", SPEC_GLOB], cwd);
    } catch {
      // Exit 1 = no match (ref predates docs/specs); anything else = unreadable ref
      // (partial clone, pruned object). Neither should fail the run.
      continue;
    }
    rows.push(...parseGrep(out, ref));
  }
  return rows;
}

export function readReservations(file) {
  try {
    return parseReservations(readFileSync(file, "utf8"));
  } catch {
    return [];
  }
}

function main(argv) {
  const cwd = process.cwd();
  const root = mainWorktreeRoot(cwd);
  const file = join(root, CLAIMS_FILENAME);

  const claims = collapseClaims(scanRefs(cwd));
  let reservations = pruneReservations(readReservations(file), claims);

  const releaseIdx = argv.indexOf("--release");
  if (releaseIdx !== -1) {
    const num = Number(String(argv[releaseIdx + 1]).replace(/^F/i, ""));
    reservations = reservations.filter((r) => r.num !== num);
  }

  let next = nextFree([...claims, ...reservations]);

  const reserveIdx = argv.indexOf("--reserve");
  if (reserveIdx !== -1) {
    const slug = argv[reserveIdx + 1];
    if (!slug || slug.startsWith("--")) {
      console.error("--reserve needs a slug, e.g. --reserve candidates-image");
      process.exitCode = 2;
      return;
    }
    reservations.push({ num: next, slug, at: new Date().toISOString().slice(0, 10) });
    console.log(`Reserved F${next} for "${slug}".`);
    next = nextFree([...claims, ...reservations]);
  }

  const text = renderFile(claims, reservations, next);
  if (!argv.includes("--print")) writeFileSync(file, text, "utf8");

  const collided = collisions(claims);
  for (const c of collided) {
    console.error(`COLLISION F${c.id}: ${c.slug} @ ${c.ref}  VS  ${describeRivals(c)}`);
  }

  console.log(`Next free feature number: F${next}`);
  if (!argv.includes("--print")) console.log(`Claims file: ${file}`);
  if (collided.length) process.exitCode = 1;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) main(process.argv.slice(2));
