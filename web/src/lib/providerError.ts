// The Enrich picker's error line for a failed provider call (HOLODEX-87), pulled out of
// EnrichPicker.svelte so the rule is unit-testable without a DOM — same split as
// autoApply.ts. The handoff's "Provider unreachable" state
// (docs/design/metadata-enrichment-handoff.md) names the provider instead of echoing the
// server's generic 502 ("provider lookup failed") or a proxy's bodiless status line.
import { ApiError } from './api.ts';
import { toMessage } from './format.ts';

/**
 * The owner-facing sentence for a resolve/apply failure. A gateway failure (502/504) is
 * the provider being unreachable; everything else keeps its own message — notably a
 * paused provider's 503, whose server-written "tmdb is rate-limiting — try again in 42 s"
 * is already the friendly line and carries the wait.
 */
export function providerErrorLine(e: unknown, provider: string): string {
	if (e instanceof ApiError && (e.status === 502 || e.status === 504)) {
		return `${provider} is unavailable right now.`;
	}
	return toMessage(e);
}
