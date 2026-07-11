// Flightplan · hook stdin helpers (ADR-064). Claude Code pipes the hook payload as JSON on stdin;
// these read and parse it defensively (a missing/garbled payload yields "" / null, never a throw).

export async function readStdin() {
  if (process.stdin.isTTY) return "";
  const chunks = [];
  try {
    for await (const c of process.stdin) chunks.push(c);
  } catch {
    /* ignore */
  }
  return Buffer.concat(chunks).toString("utf8");
}

export function safeParse(s) {
  try {
    return JSON.parse(s);
  } catch {
    return null;
  }
}
