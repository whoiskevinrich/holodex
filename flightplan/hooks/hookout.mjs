// Flightplan · hook output helpers (ADR-064) — the counterpart to stdin.mjs: how a hook writes its
// result back to Claude Code. Centralized because the flush-before-exit is a load-bearing, easy-to-
// get-wrong detail: a hook's stdout is a PIPE, and process.exit() before the write drains truncates
// the payload. Every emitting hook goes through here so that fix lives in one place. Dependency-free.

// Repo-relative, forward-slashed form of an absolute path — for banners and messages.
export function relPath(root, p) {
  return p.startsWith(root) ? p.slice(root.length + 1).replaceAll("\\", "/") : p;
}

// Write a JSON hook result, then exit only once stdout has drained (the write callback fires
// post-flush). The caller owns the payload shape; this owns the flush-and-exit.
export function emitJson(obj) {
  process.stdout.write(JSON.stringify(obj) + "\n", () => process.exit(0));
}
