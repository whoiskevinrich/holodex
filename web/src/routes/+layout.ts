// SPA mode (ADR-002): no SSR, no prerender — the Go binary serves a single
// index.html fallback and the client router takes over.
export const ssr = false;
export const prerender = false;
