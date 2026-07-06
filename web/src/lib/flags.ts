// Flag SVG assets, keyed by ISO 3166-1 alpha-2 (lowercase). Sourced from flag-icons
// (MIT) and bundled by Vite at build time, so they are served locally — no CDN, works
// offline, and ships inside the Go binary via the web/dist go:embed (ADR-007). The
// eager glob turns each SVG into a hashed asset URL; flagUrl looks one up by code.
const flagUrls = import.meta.glob('/node_modules/flag-icons/flags/4x3/*.svg', {
	eager: true,
	query: '?url',
	import: 'default'
}) as Record<string, string>;

// flagUrl returns the bundled URL for a country's flag, or undefined for an unknown code
// (the caller renders nothing rather than a broken image).
export function flagUrl(code: string): string | undefined {
	return flagUrls[`/node_modules/flag-icons/flags/4x3/${code}.svg`];
}
