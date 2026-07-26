import { fileCandidateValue } from './f36';
import type { ResolvedField, Resolution } from './types';

// formatDuration renders seconds as H:MM:SS (or M:SS under an hour) — F2.4.
export function formatDuration(totalSec: number): string {
	if (!totalSec || totalSec < 0) return '0:00';
	const h = Math.floor(totalSec / 3600);
	const m = Math.floor((totalSec % 3600) / 60);
	const s = Math.floor(totalSec % 60);
	const mm = String(m).padStart(2, '0');
	const ss = String(s).padStart(2, '0');
	return h > 0 ? `${h}:${mm}:${ss}` : `${m}:${ss}`;
}

// resolutionBucket classifies by width with a 10% tolerance — mirrors ADR-012
// (kept in sync with internal/metadata/resolution.go).
export function resolutionBucket(width: number): Resolution {
	if (width >= 3456) return '4K';
	if (width >= 1728) return 'FHD';
	if (width >= 1152) return 'HD';
	return 'SD';
}

export function formatBytes(bytes: number): string {
	if (!bytes) return '0 B';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	const i = Math.floor(Math.log(bytes) / Math.log(1024));
	return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

// formatBitrate renders kbps as "N.N Mbps" once it reaches 1000, else "N kbps".
export function formatBitrate(kbps: number): string {
	return kbps >= 1000 ? `${(kbps / 1000).toFixed(1)} Mbps` : `${kbps} kbps`;
}

export function formatYear(iso?: string | null): string {
	if (!iso) return '';
	const d = new Date(iso);
	// recorded_at is a UTC instant (e.g. "2021-01-01T00:00:00Z"); read the year in
	// UTC so a midnight-UTC date doesn't roll back a year for viewers west of UTC.
	return Number.isNaN(d.getTime()) ? '' : String(d.getUTCFullYear());
}

// toMessage normalizes a caught value into a display string (replaces the
// `e instanceof Error ? e.message : …` repeated across route pages).
export function toMessage(e: unknown): string {
	return e instanceof Error ? e.message : 'Failed to load';
}

// videoCount renders the pluralized "N video(s)" label used on detail pages.
export function videoCount(n: number): string {
	return `${n} video${n === 1 ? '' : 's'}`;
}

// monogram is the fallback glyph for an entity with no logo/icon: the real first
// glyph of the name (upper-cased), not an A–Z jump bucket — so "24 Frames" shows "2"
// and "東宝" shows "東". Shared by the studios list well and the provider brand icon
// (ADR-059). Empty/whitespace name → "?".
export function monogram(name: string): string {
	return name.trim().charAt(0).toUpperCase() || '?';
}

// isHttpUrl gates a provider-supplied value before it becomes a link `href`.
// Enrichment text fields are bounded server-side (F22.9b) but NOT scheme-checked,
// and Svelte does not sanitize `href` — so a value like "javascript:…" would be a
// live XSS sink. Only http(s) values render as links; anything else falls back to
// plain text. http(s)-only also rules out data:/blob:/file:/mailto:/etc.
export function isHttpUrl(s: string): boolean {
	return /^https?:\/\//i.test(s.trim());
}

// providerFromWinningSource extracts the enrichment provider namespace from a
// ResolvedField's `winning_source` ("tmdb:bio" → "tmdb"), or "" when the winner is a
// baseline source (file/record/manual) that gets no provenance badge. Shared by the
// detail pages and AutoFieldRows so the baseline-namespace set lives in one place.
export function providerFromWinningSource(winningSource?: string): string {
	const ns = (winningSource ?? '').split(':')[0];
	// `computed` (F45, ADR-063) is a derived-field provenance namespace, not a provider:
	// guard it here too so a "computed:age" winner can never resolve to a phantom "computed"
	// provider bubble anywhere it slips past the caller's own f.computed branch.
	return ns === 'record' || ns === 'file' || ns === 'manual' || ns === 'computed' ? '' : ns;
}

// valueMatchesFile is true when an arbitrary candidate value (e.g. a form's live draft) is
// textually identical to the field's file baseline. The live counterpart to f36.ts's
// outOfSync(): that reads a frozen resolver snapshot (field.in_sync) and only applies once a
// decision exists, while this takes the value to compare as a parameter so a caller (the F28
// writeback form) can re-check it against edits made after the page loaded.
export function valueMatchesFile(field: ResolvedField, value: string): boolean {
	return field.candidates !== undefined && value.trim() === fileCandidateValue(field).trim();
}

// calculatedFrom builds the transitive provenance phrase for a computed field (F45,
// ADR-063) from its input LABELS — e.g. ["Born"] → "calculated from Born",
// ["Born","Died"] → "calculated from Born and Died" (serial "and" for the last of 3+).
// Shown as a hover tooltip on the derived value; "" when there are no inputs.
export function calculatedFrom(labels: string[]): string {
	if (labels.length === 0) return '';
	let joined: string;
	if (labels.length === 1) joined = labels[0];
	else if (labels.length === 2) joined = `${labels[0]} and ${labels[1]}`;
	else joined = `${labels.slice(0, -1).join(', ')}, and ${labels[labels.length - 1]}`;
	return `calculated from ${joined}`;
}

// formatAgo renders a past ISO timestamp as a compact relative time ("3m ago").
export function formatAgo(iso?: string | null): string {
	if (!iso) return '';
	const then = new Date(iso).getTime();
	if (Number.isNaN(then)) return '';
	const s = Math.max(0, Math.round((Date.now() - then) / 1000));
	if (s < 60) return `${s}s ago`;
	const m = Math.floor(s / 60);
	if (m < 60) return `${m}m ago`;
	const h = Math.floor(m / 60);
	if (h < 24) return `${h}h ago`;
	return `${Math.floor(h / 24)}d ago`;
}

// formatUntil renders a future ISO timestamp as "in 4m" / "in 6h" / "in 5d"
// (or "due now"). Scales past minutes so a multi-day purge window reads sanely.
export function formatUntil(iso?: string | null): string {
	if (!iso) return '';
	const t = new Date(iso).getTime();
	if (Number.isNaN(t)) return '';
	const s = Math.round((t - Date.now()) / 1000);
	if (s <= 0) return 'due now';
	if (s < 60) return `in ${s}s`;
	const m = Math.floor(s / 60);
	if (m < 60) return `in ${m}m`;
	const h = Math.floor(m / 60);
	if (h < 24) return `in ${h}h`;
	return `in ${Math.floor(h / 24)}d`;
}

// formatDurMs renders a millisecond duration compactly ("120ms", "8.4s", "1m 12s").
export function formatDurMs(ms: number): string {
	if (ms < 1000) return `${ms}ms`;
	const s = ms / 1000;
	if (s < 60) return `${s.toFixed(1)}s`;
	const m = Math.floor(s / 60);
	return `${m}m ${Math.round(s % 60)}s`;
}

// formatUptime renders a seconds uptime as "3d 4h" / "10h 2m" / "5m".
export function formatUptime(sec?: number): string {
	if (!sec || sec <= 0) return '—';
	const d = Math.floor(sec / 86400);
	const h = Math.floor((sec % 86400) / 3600);
	const m = Math.floor((sec % 3600) / 60);
	if (d > 0) return `${d}d ${h}h`;
	if (h > 0) return `${h}h ${m}m`;
	return `${m}m`;
}
