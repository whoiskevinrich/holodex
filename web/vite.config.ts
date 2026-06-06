import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		// Proxy API + MCP to the Go backend during development (ADR-007).
		proxy: {
			'/api': 'http://localhost:7800',
			'/mcp': 'http://localhost:7801'
		}
	}
});
