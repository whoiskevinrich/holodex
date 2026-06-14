import type { Resolution } from './types';

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
