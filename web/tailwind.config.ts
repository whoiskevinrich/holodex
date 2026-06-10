import type { Config } from 'tailwindcss';

// All colors/fonts/radius are semantic tokens backed by CSS variables. The
// actual values live per-skin in app.css under [data-theme="…"], so components
// never hardcode a palette — they use bg-bg / text-ink / text-accent / etc.
// (see ADR-021 and docs/design/theming.md). Adding a hex/zinc utility in a
// component is a theming bug: it won't change when the skin does.
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	theme: {
		extend: {
			colors: {
				bg: 'var(--bg)',
				surface: 'var(--surface)',
				'surface-2': 'var(--surface-2)',
				ink: 'var(--ink)',
				muted: 'var(--muted)',
				rule: 'var(--rule)',
				accent: 'var(--accent)',
				'accent-ink': 'var(--accent-ink)'
			},
			fontFamily: {
				display: 'var(--font-display)',
				ui: 'var(--font-ui)'
			},
			borderRadius: {
				theme: 'var(--radius)'
			}
		}
	},
	plugins: []
} satisfies Config;
