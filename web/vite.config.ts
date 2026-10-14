import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { playwright } from '@vitest/browser-playwright';
import { configDefaults, defineConfig } from 'vitest/config';

// Dev proxy targets default to the standard backend ports (7800/7801). Override
// with HOLODEX_API_PORT / HOLODEX_MCP_PORT when running multiple sessions side by
// side so each dev server proxies to its own backend.
const apiPort = process.env.HOLODEX_API_PORT || '7800';
const mcpPort = process.env.HOLODEX_MCP_PORT || '7801';

export default defineConfig({
	// Tailwind v4 via its Vite plugin (replaces the PostCSS pipeline + autoprefixer;
	// vendor prefixing is now handled by Lightning CSS). Config is CSS-first in
	// app.css — see ADR-025. The plugin must precede sveltekit().
	// SvelteKit 3 takes its config here; svelte.config.js is no longer read.
	plugins: [
		tailwindcss(),
		sveltekit({
			preprocess: vitePreprocess(),
			// Svelte Inspector (dev server only, never in the build): Alt+X, hover an element,
			// click to open its source in the editor, right-click for the component stack.
			// Svelte 5 has no DevTools panel — the browser extension is Svelte 4 only.
			// Safe to leave on: the plugin is serve-only, so `vite build` output (and thus the
			// Docker image) contains no inspector code — nothing to disable in production.
			// Per-machine overrides via web/.env: SVELTE_INSPECTOR_OPTIONS=false or
			// SVELTE_INSPECTOR_TOGGLE=<combo>.
			inspector: true,
			// SPA mode: a single index.html fallback for client-side routing (ADR-002).
			// Output to web/dist to match the Dockerfile go:embed source (ADR-007).
			adapter: adapter({ pages: 'dist', assets: 'dist', fallback: 'index.html' })
		})
	],
	build: {
		// Never inline the bundled flag SVGs (HOLODEX-139). Vite inlines assets under
		// ~4 KB as data URIs; left unchecked, ~200 flags balloon the person-page chunk by
		// hundreds of KB even though a page shows one flag. Emitting them as files keeps
		// the chunk small and fetches only the flags actually rendered (cached, offline).
		assetsInlineLimit: (filePath) =>
			filePath.includes('flag-icons/flags/') ? false : undefined
	},
	server: {
		// Proxy API + MCP to the Go backend during development (ADR-007).
		proxy: {
			'/api': `http://localhost:${apiPort}`,
			'/mcp': `http://localhost:${mcpPort}`
		}
	},
	// Two Vitest projects (HOLODEX-223, stack.md "Frontend component tests"): a test that
	// needs the component alive — mount effects, awaited fetches, clicks, focus — is a
	// `*.svelte.test.ts` and runs in headless Chromium; everything else (pure logic, SSR
	// `render()` markup checks, the geometry .mjs tests) stays in Node.
	test: {
		projects: [
			{
				extends: true,
				test: {
					name: 'client',
					include: ['src/**/*.svelte.test.ts'],
					browser: {
						enabled: true,
						provider: playwright(),
						headless: true,
						instances: [{ browser: 'chromium' }]
					}
				}
			},
			{
				extends: true,
				test: {
					name: 'server',
					environment: 'node',
					exclude: [...configDefaults.exclude, 'src/**/*.svelte.test.ts']
				}
			}
		]
	}
});
