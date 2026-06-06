import type { Config } from 'tailwindcss';

export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	darkMode: 'class', // dark mode is the default (toggled by a class on <html>) — F8
	theme: {
		extend: {}
	},
	plugins: []
} satisfies Config;
