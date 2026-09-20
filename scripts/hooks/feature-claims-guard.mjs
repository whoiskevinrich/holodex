#!/usr/bin/env node
// feature-claims-guard.mjs — the F## claim rule, enforced where the number is picked.
//
// CLAUDE.md says to run scripts/feature-claims.mjs before writing a spec's H1, but a
// rule nobody runs is a rule that fails at merge time: HOLODEX-425 wrote `(F66)` while
// HOLODEX-421 had already claimed F66 on another branch, and the collision surfaced only
// when main was merged in (36 references renumbered). This hook moves the check to the
// two moments it can still help, as a Claude Code PreToolUse hook (.claude/settings.json):
//
//   * Skill  — when /write-spec (any plugin prefix) is invoked, the next free number and
//              any in-flight collisions are injected as context, so the scaffold step
//              has the answer in hand.
//   * Write / Edit / MultiEdit — when the content being written to docs/specs/*.md has an
//              H1 that claims an F## already owned by a *different* spec on any ref, or
//              reserved for another slug, the write is BLOCKED (exit 2) with the rival and
//              the next free number. A spec re-saving its own number, a `# QA:` companion,
//              an unnumbered H1, or any other file passes untouched.
//
// Never re-enters Claude, and a tooling failure (git missing, unreadable repo) never
// blocks — the guard is a seatbelt, not a gate on the tooling itself. Two known gaps,
// accepted rather than engineered around: an Edit whose new_string is only the `(F66)`
// fragment (not the whole H1 line) carries no H1 and passes; and a spec renamed to a new
// path keeps reading as a rival of its old slug until the old path is gone from every ref
// (the block message names it, so the cause is obvious).
//
// Input is the hook JSON on stdin ({ tool_name, tool_input, … }); the pure parts —
// isWriteSpecSkill, headingOf, specSlugOf, decide, contextFor — are exported for
// scripts/hooks/feature-claims-guard.test.mjs; main() runs only when executed directly.

import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { describeRivals, mainWorktreeRoot, nextFree, pruneReservations } from "../adr-claims.mjs";
// `collisions` must be feature-claims' own: it honours the merged-on-main tolerance (F55/F56
// legitimately share numbers on main); adr-claims' version flags any shared number.
import {
  CLAIMS_FILENAME,
  collapseClaims,
  collisions,
  parseHeading,
  readReservations,
  scanRefs,
  specSlug,
} from "../feature-claims.mjs";

const SPEC_PATH = /(^|[\\/])docs[\\/]specs[\\/][^\\/]+\.md$/;

// "product-management:write-spec", "write-spec", "docs:write-spec" all count.
export function isWriteSpecSkill(toolInput) {
  const skill = String(toolInput?.skill ?? "");
  return /(^|:)write-spec$/.test(skill);
}

// The H1 a Write/Edit/MultiEdit would put in the file, or null when the change carries
// none. Edits are judged by their new_string alone: a spec whose H1 is being changed
// shows the new H1 there; an edit elsewhere in the file carries no H1 and passes.
export function headingOf(toolName, toolInput) {
  const bodies = [];
  if (toolName === "Write") bodies.push(toolInput?.content);
  else if (toolName === "Edit") bodies.push(toolInput?.new_string);
  else if (toolName === "MultiEdit") for (const e of toolInput?.edits ?? []) bodies.push(e?.new_string);
  for (const body of bodies) {
    if (typeof body !== "string") continue;
    const h1 = body.split(/\r?\n/).find((l) => /^#\s/.test(l));
    if (h1) return h1;
  }
  return null;
}

// docs/specs/instance-skin.md (any separators, any prefix) -> instance-skin; null otherwise.
export function specSlugOf(filePath) {
  const p = String(filePath ?? "");
  return SPEC_PATH.test(p) ? specSlug(p.replace(/\\/g, "/")) : null;
}

// The judgement, kept pure: given the spec's slug, its H1, the collapsed claim set and the
// live reservations, either {block:false} or {block:true, message}.
export function decide({ slug, h1, claims, reservations }) {
  const parsed = parseHeading(h1 ?? "");
  if (!parsed) return { block: false };
  const rivals = [];
  for (const c of claims) {
    if (c.id !== parsed.id) continue;
    const holders = [{ slug: c.slug, ref: c.ref }, ...(c.alsoOn ?? [])];
    for (const h of holders) if (h.slug !== slug) rivals.push(`${h.slug} @ ${h.ref}`);
  }
  for (const r of reservations) {
    if (r.num === parsed.num && r.slug !== slug) rivals.push(`${r.slug} (reserved ${r.at})`);
  }
  if (!rivals.length) return { block: false };
  const next = nextFree([...claims, ...reservations]);
  return {
    block: true,
    message:
      `F${parsed.id} is already claimed by ${[...new Set(rivals)].join("; ")}. ` +
      `Next free feature number is F${next} — use it in the H1 \`# Spec: … (F${next})\`, or run ` +
      `\`node scripts/feature-claims.mjs\` to see every claim (\`--reserve ${slug}\` holds a number for un-pushed work).`,
  };
}

// What /write-spec gets told before it scaffolds.
export function contextFor(claims, reservations) {
  const next = nextFree([...claims, ...reservations]);
  const lines = [
    `Feature numbers (scripts/feature-claims.mjs): the next free number is F${next}. ` +
      `Use it in the spec's H1 — \`# Spec: … (F${next})\` — and hold it with ` +
      `\`node scripts/feature-claims.mjs --reserve <slug>\` if the spec will not be pushed right away.`,
  ];
  const collided = collisions(claims);
  if (collided.length) {
    lines.push(
      "In-flight collisions already on other branches (do not add to them): " +
        collided.map((c) => `F${c.id}: ${c.slug} @ ${c.ref} vs ${describeRivals(c)}`).join("; "),
    );
  }
  return lines.join("\n");
}

function loadClaims(cwd) {
  const claims = collapseClaims(scanRefs(cwd));
  const reservations = pruneReservations(readReservations(join(mainWorktreeRoot(cwd), CLAIMS_FILENAME)), claims);
  return { claims, reservations };
}

async function readStdin() {
  let raw = "";
  for await (const chunk of process.stdin) raw += chunk;
  return raw ? JSON.parse(raw) : {};
}

async function main() {
  let input;
  try {
    input = await readStdin();
  } catch {
    return; // not hook JSON — nothing to judge
  }
  const toolName = input.tool_name ?? "";
  const toolInput = input.tool_input ?? {};

  if (toolName === "Skill") {
    if (!isWriteSpecSkill(toolInput)) return;
    let ctx;
    try {
      const { claims, reservations } = loadClaims(process.cwd());
      ctx = contextFor(claims, reservations);
    } catch (err) {
      ctx = `feature-claims scan failed (${err?.message ?? err}); run \`node scripts/feature-claims.mjs\` by hand before picking the F## for the H1.`;
    }
    process.stdout.write(
      JSON.stringify({ hookSpecificOutput: { hookEventName: "PreToolUse", additionalContext: ctx } }),
    );
    return;
  }

  if (toolName === "Write" || toolName === "Edit" || toolName === "MultiEdit") {
    const slug = specSlugOf(toolInput.file_path);
    if (!slug) return;
    const h1 = headingOf(toolName, toolInput);
    if (!h1) return;
    let verdict;
    try {
      const { claims, reservations } = loadClaims(process.cwd());
      verdict = decide({ slug, h1, claims, reservations });
    } catch (err) {
      process.stderr.write(`feature-claims-guard: scan failed, not blocking (${err?.message ?? err})\n`);
      return;
    }
    if (verdict.block) {
      process.stderr.write(`feature-claims-guard: ${verdict.message}\n`);
      process.exitCode = 2;
    }
  }
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) main();
