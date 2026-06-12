// Generated key-art for the demo corpus. Each item gets a deterministic 16:9
// "still" — flat cinematic background + an abstract motif + the title — rendered
// as SVG and rasterized to JPEG by sharp (see generate.mjs). Posters are
// skin-independent: they are the embedded cover art the app extracts (ADR-009
// Tier 1), so they look identical across all three UI skins.

const PALETTES = [
	{ bg: '#11132b', glow: '#222a5e', accent: '#ffd166', ink: '#f4f1ff' },
	{ bg: '#2a0f12', glow: '#5a1f24', accent: '#ff7a59', ink: '#ffeede' },
	{ bg: '#0c1f17', glow: '#15402f', accent: '#76e0a3', ink: '#eafff4' },
	{ bg: '#08222b', glow: '#0f4453', accent: '#45c8e0', ink: '#e6fbff' },
	{ bg: '#1d1030', glow: '#3a2060', accent: '#c792ff', ink: '#f5ecff' },
	{ bg: '#1c1a17', glow: '#3a352c', accent: '#e8a33d', ink: '#f6efe2' },
	{ bg: '#0a1530', glow: '#152a5e', accent: '#6ea8ff', ink: '#eaf1ff' },
	{ bg: '#2b1a0e', glow: '#56341c', accent: '#ffb46b', ink: '#fdeede' },
	{ bg: '#240b16', glow: '#4a162d', accent: '#ff5d8f', ink: '#ffe9f0' },
	{ bg: '#18200c', glow: '#313f18', accent: '#c6e65a', ink: '#f2ffd9' }
];

const W = 1600;
const H = 900;

const XML_ENTITIES = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };

function esc(s) {
	return String(s).replace(/[&<>"']/g, (c) => XML_ENTITIES[c]);
}

// Motifs are drawn into the upper/right field; the lower-left is reserved for the
// title block. All use only the palette's accent at low opacity so the type stays
// the focal point.
function motif(kind, a) {
	switch (kind) {
		case 'sun':
			return `<circle cx="1180" cy="300" r="210" fill="${a}" opacity="0.16"/>
				<circle cx="1180" cy="300" r="210" fill="none" stroke="${a}" stroke-width="3" opacity="0.5"/>`;
		case 'rings':
			return [120, 230, 340, 450]
				.map((r) => `<circle cx="1230" cy="250" r="${r}" fill="none" stroke="${a}" stroke-width="2.5" opacity="0.32"/>`)
				.join('');
		case 'bars':
			return Array.from({ length: 7 }, (_, i) => {
				const x = 760 + i * 130;
				return `<rect x="${x}" y="-50" width="46" height="1000" transform="rotate(18 ${x} 450)" fill="${a}" opacity="${0.05 + i * 0.02}"/>`;
			}).join('');
		case 'grid': {
			const lines = [];
			for (let x = 820; x <= 1560; x += 92) lines.push(`<line x1="${x}" y1="40" x2="${x}" y2="620" stroke="${a}" stroke-width="1.5" opacity="0.22"/>`);
			for (let y = 60; y <= 560; y += 92) lines.push(`<line x1="800" y1="${y}" x2="1560" y2="${y}" stroke="${a}" stroke-width="1.5" opacity="0.22"/>`);
			return lines.join('');
		}
		case 'halftone': {
			let d = '';
			for (let r = 0; r < 7; r++)
				for (let c = 0; c < 9; c++) {
					const cx = 880 + c * 80;
					const cy = 110 + r * 70;
					const rad = 4 + ((c + r) % 5) * 4;
					d += `<circle cx="${cx}" cy="${cy}" r="${rad}" fill="${a}" opacity="0.4"/>`;
				}
			return d;
		}
		case 'horizon':
		default:
			return `<circle cx="1230" cy="330" r="170" fill="${a}" opacity="0.18"/>
				<line x1="780" y1="560" x2="1560" y2="560" stroke="${a}" stroke-width="3" opacity="0.55"/>
				<line x1="780" y1="600" x2="1560" y2="600" stroke="${a}" stroke-width="1.5" opacity="0.3"/>`;
	}
}

// buildSVG returns a 1600x900 SVG string for one item. The palette is chosen by
// index so the grid reads as a varied set rather than a single hue.
export function buildSVG(item, index) {
	const p = PALETTES[index % PALETTES.length];
	const subtitle = (item.tags[0] || '').toUpperCase();
	return `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
	<defs>
		<radialGradient id="glow" cx="72%" cy="32%" r="62%">
			<stop offset="0%" stop-color="${p.glow}"/>
			<stop offset="100%" stop-color="${p.bg}"/>
		</radialGradient>
		<linearGradient id="scrim" x1="0" y1="0" x2="0" y2="1">
			<stop offset="55%" stop-color="${p.bg}" stop-opacity="0"/>
			<stop offset="100%" stop-color="${p.bg}" stop-opacity="0.85"/>
		</linearGradient>
	</defs>
	<rect width="${W}" height="${H}" fill="url(#glow)"/>
	${motif(item.art, p.accent)}
	<rect width="${W}" height="${H}" fill="url(#scrim)"/>
	<rect x="90" y="612" width="64" height="6" fill="${p.accent}"/>
	<text x="90" y="742" font-family="Georgia, 'Times New Roman', serif" font-size="120" font-weight="700" fill="${p.ink}">${esc(item.title)}</text>
	<text x="92" y="800" font-family="Arial, Helvetica, sans-serif" font-size="34" letter-spacing="7" fill="${p.accent}">${esc(subtitle)}</text>
	<text x="1510" y="800" text-anchor="end" font-family="Arial, Helvetica, sans-serif" font-size="34" letter-spacing="3" fill="${p.ink}" opacity="0.75">${item.year}</text>
</svg>`;
}
