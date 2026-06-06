# ADR-002: Frontend Framework — SvelteKit

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex needs a frontend that:
- Renders a virtual-scroll or paginated grid of potentially thousands of video cards
- Updates filter/search results without full page reloads
- Runs entirely client-side (SPA mode) — no SSR required for a personal local tool
- Ships as static files served by the Go binary or a CDN
- Supports a polished dark-mode UI with a reasonable component ecosystem

Candidates evaluated: SvelteKit, Solid.js, Next.js (React), Vue 3 + Vite.

## Decision

**SvelteKit** in SPA/static-adapter mode is the chosen frontend framework.

## Rationale

- **Compiled output**: Svelte compiles components to vanilla JS at build time; no virtual DOM, no framework runtime in the bundle. This directly supports the ≤ 150ms navigation target.
- **Lean bundle**: Static output is well-suited to being embedded in the Go binary via `embed.FS` and served from a single executable.
- **Built-in router**: SvelteKit's client-side router handles URL-reflected filter state (requirement F4.7) with zero additional libraries.
- **Component ecosystem**: `shadcn-svelte` and `Melt UI` provide accessible, unstyled primitives that compose well with a custom dark-mode design system.
- **Reactivity model**: Svelte's fine-grained reactive stores are a natural fit for the filter state object that drives both the URL and the API query.

## Rejected Alternatives

| Option | Reason rejected |
|--------|-----------------|
| Solid.js | Faster in synthetic benchmarks but smaller ecosystem; fewer UI component libraries; higher contributor friction |
| Next.js (React) | Larger bundle; React reconciler overhead on grid-heavy renders; RSC complexity unnecessary for a local SPA |
| Vue 3 + Vite | Solid middle ground but no meaningful advantage over SvelteKit for this use case |

## Consequences

- Build output is a static asset directory (`/dist`) embedded into the Go binary via `go:embed` — no separate frontend container needed.
- Tailwind CSS is the styling layer (pairs naturally with shadcn-svelte).
- TypeScript is used in the frontend; the Go API contract is the shared interface boundary.
- SvelteKit is configured with `@sveltejs/adapter-static` (SPA mode with `fallback: 'index.html'`).
- The Go server serves the embedded static files and catches all non-API routes for client-side routing.
