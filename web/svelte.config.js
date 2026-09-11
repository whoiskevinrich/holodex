import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	// Svelte Inspector (dev server only, never in the build): Alt+X, hover an element,
	// click to open its source in the editor, right-click for the component stack.
	// Svelte 5 has no DevTools panel — the browser extension is Svelte 4 only.
	// Safe to leave on: the plugin is serve-only, so `vite build` output (and thus the
	// Docker image) contains no inspector code — nothing to disable in production.
	// Per-machine overrides via web/.env: SVELTE_INSPECTOR_OPTIONS=false or
	// SVELTE_INSPECTOR_TOGGLE=<combo>.
	vitePlugin: { inspector: true },
	kit: {
		// SPA mode: a single index.html fallback for client-side routing (ADR-002).
		// Output to web/dist to match the Dockerfile go:embed source (ADR-007).
		adapter: adapter({ pages: 'dist', assets: 'dist', fallback: 'index.html' })
	}
};

export default config;
