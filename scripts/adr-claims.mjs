#!/usr/bin/env node
// adr-claims.mjs — "which ADR number is actually free?"
//
// ADR numbers are claimed in a branch long before they reach main, so picking
// "highest on main + 1" collides with whatever is in flight. PR #257 hit this three
// times running: it claimed 086, main shipped 086, it moved to 087, main shipped 087,
// it moved to 088, main shipped 088.
//
// This derives the claim set from git itself — every local and remote branch, not just
// main — so an in-flight ADR on someone else's branch is visible before you take its
// number. The result is cached in a gitignored file at the MAIN worktree root, shared
// by every worktree (they share one .git). Gitignored on purpose: a committed registry
// would itself be a merge-conflict magnet, since every branch would edit the same lines.
//
// Because a number you are about to use is not in git yet, --reserve records a local
// hold. Reservations are dropped automatically once the same number shows up in git
// (the real claim supersedes the hold), and are flagged stale after STALE_DAYS so
// abandoned holds do not silently squat a number forever.
//
// Dependency-free (node:child_process, node:fs). Read-only against git — it never
// fetches, checks out, or writes to the repo. Pure helpers are exported for
// scripts/adr-claims.test.mjs; main() runs only when executed directly.
//
// Usage:
//   node scripts/adr-claims.mjs                 # refresh the file, print the next free number
//   node scripts/adr-claims.mjs --print         # print only, do not write the file
//   node scripts/adr-claims.mjs --reserve slug  # hold the next free number for un-pushed work
//   node scripts/adr-claims.mjs --release 091   # drop a reservation you did not use

import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

export const CLAIMS_FILENAME = ".adr-claims";
export const STALE_DAYS = 14;
const ADR_DIR = "docs/architecture";

// ADR-004-metadata-extraction.md -> { num: 4, slug: "metadata-extraction" }
export function parseAdrPath(path) {
  const m = /(?:^|\/)ADR-(\d+)-(.+)\.md$/.exec(path);
  if (!m) return null;
  return { num: Number(m[1]), slug: m[2] };
}

// Lowest positive integer not present. Claims may be sparse; we never reuse a gap
// below the high-water mark, because a gap is far more likely to be a deleted or
// renamed ADR than a genuinely free slot.
export function nextFree(claims) {
  const nums = claims.map((c) => c.num);
  return (nums.length ? Math.max(...nums) : 0) + 1;
}

// A ref name like refs/remotes/origin/HOLODEX-194-x reads better as origin/HOLODEX-194-x,
// and refs/heads/x as x. Purely cosmetic — the file is meant to be eyeballed.
export function shortRef(ref) {
  return ref.replace(/^refs\/(heads|remotes)\//, "");
}

// main first, then origin/*, then local branches — so the "real" claim for a number
// sorts above a branch that is merely carrying it.
export function rankRef(ref) {
  if (ref === "origin/main" || ref === "main") return 0;
  if (ref.startsWith("origin/")) return 1;
  return 2;
}

// One row per ADR number, keeping the best-ranked ref as the canonical one.
//
// `alsoOn` records only *rival slugs* — a different ADR wearing the same number. The
// same slug on forty branches is just forty branches carrying one ADR, which is normal
// and would otherwise bury the signal (this repo has ~80 refs; the first cut of this
// function emitted a 90-line collision line for ADR-059). Rivals are grouped by slug
// with a representative ref and a count, not one entry per ref.
export function collapseClaims(rows) {
  const byNum = new Map();
  for (const row of rows) {
    const existing = byNum.get(row.num);
    if (!existing) {
      byNum.set(row.num, { ...row, rivals: new Map() });
      continue;
    }
    const winner = rankRef(row.ref) < rankRef(existing.ref) ? { ...row, rivals: existing.rivals } : existing;
    const loser = winner === existing ? row : existing;
    // Re-file the displaced slug as a rival only if it is genuinely a different ADR.
    for (const [slug, info] of loser.rivals ?? []) {
      if (!winner.rivals.has(slug)) winner.rivals.set(slug, { ...info });
    }
    if (loser.slug !== winner.slug) {
      const seen = winner.rivals.get(loser.slug);
      if (seen) seen.count += 1;
      else winner.rivals.set(loser.slug, { ref: loser.ref, count: 1 });
    }
    byNum.set(row.num, winner);
  }
  return [...byNum.values()]
    .map((c) => ({
      num: c.num,
      slug: c.slug,
      ref: c.ref,
      // Drop any rival that is actually the winner's own slug (possible after re-filing).
      alsoOn: [...c.rivals].filter(([slug]) => slug !== c.slug).map(([slug, i]) => ({ slug, ...i })),
    }))
    .sort((a, b) => a.num - b.num);
}

// A number carrying two different slugs is a real collision — the thing this script exists
// to catch. Same slug on several refs is just a branch carrying its own ADR, which is fine.
export function collisions(claims) {
  return claims.filter((c) => c.alsoOn.length > 0);
}

// "frontend-component-reuse-discipline @ origin/HOLODEX-287" or, when many refs carry the
// rival, "… @ origin/HOLODEX-287 (+3 more refs)".
export function describeRivals(claim) {
  return claim.alsoOn
    .map((o) => `${o.slug} @ ${o.ref}${o.count > 1 ? ` (+${o.count - 1} more refs)` : ""}`)
    .join("; ");
}

export function parseReservations(text) {
  const out = [];
  for (const line of text.split(/\r?\n/)) {
    const m = /^(\d+)\s+RESERVED\s+(\S+)\s+(\S+)/.exec(line.trim());
    if (m) out.push({ num: Number(m[1]), slug: m[2], at: m[3] });
  }
  return out;
}

export function daysSince(iso, now = new Date()) {
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return 0;
  return Math.floor((now.getTime() - then) / 86_400_000);
}

// A reservation is fulfilled once git carries that number — the branch got pushed, so
// the hold has done its job and should stop cluttering the file.
export function pruneReservations(reservations, claims) {
  const claimed = new Set(claims.map((c) => c.num));
  return reservations.filter((r) => !claimed.has(r.num));
}

export function renderFile(claims, reservations, next, now = new Date()) {
  const lines = [
    `# ${CLAIMS_FILENAME} — claimed ADR numbers, derived from git. NOT COMMITTED.`,
    "#",
    "# Generated by scripts/adr-claims.mjs. Do not hand-edit the derived rows; they are",
    "# rebuilt from every local and remote branch on each run. RESERVED rows are local",
    "# holds for work you have not pushed yet, and are dropped automatically once the",
    "# number appears in git.",
    "#",
    "#   node scripts/adr-claims.mjs                 refresh + print the next free number",
    "#   node scripts/adr-claims.mjs --reserve slug  hold the next free number",
    "#   node scripts/adr-claims.mjs --release NNN   drop an unused hold",
    "#",
    `# Generated: ${now.toISOString()}`,
    `# Next free: ${String(next).padStart(3, "0")}`,
    "",
  ];

  const collided = collisions(claims);
  if (collided.length) {
    lines.push("# !! COLLISIONS — one number, two different ADRs. Renumber before merging.");
    for (const c of collided) {
      lines.push(`#    ${String(c.num).padStart(3, "0")}  ${c.slug} @ ${c.ref}  VS  ${describeRivals(c)}`);
    }
    lines.push("");
  }

  for (const c of claims) {
    lines.push(`${String(c.num).padStart(3, "0")}  ${c.slug.padEnd(46)}  ${c.ref}`);
  }
  for (const r of reservations) {
    const age = daysSince(r.at, now);
    const stale = age >= STALE_DAYS ? `  (stale, ${age}d — release it if abandoned)` : "";
    lines.push(`${String(r.num).padStart(3, "0")}  RESERVED  ${r.slug}  ${r.at}${stale}`);
  }
  lines.push("");
  return lines.join("\n");
}

// ---------------------------------------------------------------- git + IO (impure)

function git(args, cwd) {
  return execFileSync("git", args, { cwd, encoding: "utf8", maxBuffer: 32 * 1024 * 1024 });
}

// The main worktree's root. Worktrees share one .git, so <common-dir>/.. is the main
// checkout no matter which worktree we are invoked from — that is what makes one
// claims file visible to every session.
export function mainWorktreeRoot(cwd) {
  return dirname(git(["rev-parse", "--path-format=absolute", "--git-common-dir"], cwd).trim());
}

function scanRefs(cwd) {
  const refs = git(["for-each-ref", "--format=%(objectname) %(refname)", "refs/heads", "refs/remotes"], cwd)
    .split("\n")
    .filter(Boolean)
    .map((l) => {
      const i = l.indexOf(" ");
      return { sha: l.slice(0, i), ref: shortRef(l.slice(i + 1)) };
    })
    .filter((r) => !r.ref.endsWith("/HEAD"));

  // Many branches point at the same commit; scanning a tree once per SHA keeps this
  // fast on a repo with dozens of stale branches.
  const bySha = new Map();
  for (const r of refs) {
    if (!bySha.has(r.sha)) bySha.set(r.sha, []);
    bySha.get(r.sha).push(r.ref);
  }

  const rows = [];
  for (const [sha, refNames] of bySha) {
    let listing;
    try {
      listing = git(["ls-tree", "-r", "--name-only", sha, "--", `${ADR_DIR}/`], cwd);
    } catch {
      continue; // ref we cannot read (partial clone, pruned object) — skip, don't fail the run
    }
    const parsed = listing.split("\n").map(parseAdrPath).filter(Boolean);
    if (!parsed.length) continue;
    const ref = [...refNames].sort((a, b) => rankRef(a) - rankRef(b) || a.localeCompare(b))[0];
    for (const p of parsed) rows.push({ ...p, ref });
  }
  return rows;
}

function readReservations(file) {
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
    const num = Number(argv[releaseIdx + 1]);
    reservations = reservations.filter((r) => r.num !== num);
  }

  let next = nextFree([...claims, ...reservations.map((r) => ({ num: r.num }))]);

  const reserveIdx = argv.indexOf("--reserve");
  if (reserveIdx !== -1) {
    const slug = argv[reserveIdx + 1];
    if (!slug || slug.startsWith("--")) {
      console.error("--reserve needs a slug, e.g. --reserve two-layer-metadata");
      process.exitCode = 2;
      return;
    }
    reservations.push({ num: next, slug, at: new Date().toISOString().slice(0, 10) });
    console.log(`Reserved ADR-${String(next).padStart(3, "0")} for "${slug}".`);
    next = nextFree([...claims, ...reservations.map((r) => ({ num: r.num }))]);
  }

  const text = renderFile(claims, reservations, next);
  if (!argv.includes("--print")) writeFileSync(file, text, "utf8");

  const collided = collisions(claims);
  for (const c of collided) {
    console.error(`COLLISION ADR-${String(c.num).padStart(3, "0")}: ${c.slug} @ ${c.ref}  VS  ${describeRivals(c)}`);
  }

  console.log(`Next free ADR number: ${String(next).padStart(3, "0")}`);
  if (!argv.includes("--print")) console.log(`Claims file: ${file}`);
  if (collided.length) process.exitCode = 1;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) main(process.argv.slice(2));
