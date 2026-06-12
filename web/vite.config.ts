import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	// Tailwind v4 via its Vite plugin (replaces the PostCSS pipeline + autoprefixer;
	// vendor prefixing is now handled by Lightning CSS). Config is CSS-first in
	// app.css — see ADR-025. The plugin must precede sveltekit().
	plugins: [tailwindcss(), sveltekit()],
	server: {
		// Proxy API + MCP to the Go backend during development (ADR-007).
		proxy: {
			'/api': 'http://localhost:7800',
			'/mcp': 'http://localhost:7801'
		}
	}
});
