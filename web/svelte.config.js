import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		// SPA mode: a single index.html fallback for client-side routing (ADR-002).
		// Output to web/dist to match the Dockerfile go:embed source (ADR-007).
		adapter: adapter({ pages: 'dist', assets: 'dist', fallback: 'index.html' })
	}
};

export default config;
