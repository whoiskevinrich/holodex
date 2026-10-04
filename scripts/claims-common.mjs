// claims-common.mjs — shared helpers for scripts/feature-claims.mjs (and its hook,
// scripts/hooks/feature-claims-guard.mjs).
//
// Feature (F##) numbers are claimed on a branch long before they reach main, so the
// claim set is derived from every local and remote branch. These are the ref-ranking,
// reservation and git helpers that derivation needs. They used to live in
// adr-claims.mjs, which allocated numbered ADRs; numbered ADRs are retired (living topic
// docs need no numbers, see docs/reference/doc-types.md), so only the shared half remains.
//
// Dependency-free (node:child_process). Pure helpers are tested in
// scripts/claims-common.test.mjs.

import { execFileSync } from "node:child_process";
import { dirname } from "node:path";

export const STALE_DAYS = 14;

// Lowest positive integer above the high-water mark. Claims may be sparse; a gap below
// it is far more likely to be a deleted or renamed claim than a genuinely free slot.
export function nextFree(claims) {
  const nums = claims.map((c) => c.num);
  return (nums.length ? Math.max(...nums) : 0) + 1;
}

// refs/remotes/origin/HOLODEX-194-x reads better as origin/HOLODEX-194-x, and
// refs/heads/x as x.
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

// "instance-skin @ origin/HOLODEX-425" or, when many refs carry the rival,
// "… @ origin/HOLODEX-425 (+3 more refs)".
export function describeRivals(claim) {
  return claim.alsoOn
    .map((o) => `${o.slug} @ ${o.ref}${o.count > 1 ? ` (+${o.count - 1} more refs)` : ""}`)
    .join("; ");
}

export function parseReservations(text) {
  const out = [];
  for (const line of text.split(/\r?\n/)) {
    // Optional F prefix: feature-claims.mjs renders its holds as "F65  RESERVED  …".
    const m = /^F?(\d+)\s+RESERVED\s+(\S+)\s+(\S+)/.exec(line.trim());
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

// ---------------------------------------------------------------- git (impure)

export function git(args, cwd) {
  return execFileSync("git", args, { cwd, encoding: "utf8", maxBuffer: 32 * 1024 * 1024 });
}

// The main worktree's root. Worktrees share one .git, so <common-dir>/.. is the main
// checkout no matter which worktree we are invoked from — that is what makes one
// claims file visible to every session.
export function mainWorktreeRoot(cwd) {
  return dirname(git(["rev-parse", "--path-format=absolute", "--git-common-dir"], cwd).trim());
}

// Every local + remote branch, keyed by commit and reduced to the best-ranked ref name
// for that commit (main > origin/* > local). Many branches point at the same commit;
// scanning a tree once per SHA keeps this fast on a repo with dozens of stale branches.
export function refsBySha(cwd) {
  const refs = git(["for-each-ref", "--format=%(objectname) %(refname)", "refs/heads", "refs/remotes"], cwd)
    .split("\n")
    .filter(Boolean)
    .map((l) => {
      const i = l.indexOf(" ");
      return { sha: l.slice(0, i), ref: shortRef(l.slice(i + 1)) };
    })
    .filter((r) => !r.ref.endsWith("/HEAD"));

  const bySha = new Map();
  for (const r of refs) {
    if (!bySha.has(r.sha)) bySha.set(r.sha, []);
    bySha.get(r.sha).push(r.ref);
  }
  for (const [sha, names] of bySha) {
    bySha.set(sha, names.sort((a, b) => rankRef(a) - rankRef(b) || a.localeCompare(b))[0]);
  }
  return bySha;
}
