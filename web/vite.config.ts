import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// Dev proxy targets default to the standard backend ports (7800/7801). Override
// with HOLODEX_API_PORT / HOLODEX_MCP_PORT when running multiple sessions side by
// side so each dev server proxies to its own backend.
const apiPort = process.env.HOLODEX_API_PORT || '7800';
const mcpPort = process.env.HOLODEX_MCP_PORT || '7801';

export default defineConfig({
	// Tailwind v4 via its Vite plugin (replaces the PostCSS pipeline + autoprefixer;
	// vendor prefixing is now handled by Lightning CSS). Config is CSS-first in
	// app.css — see ADR-025. The plugin must precede sveltekit().
	plugins: [tailwindcss(), sveltekit()],
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
	}
});
