import { describe, expect, it } from 'vitest';
import { ApiError } from './api';
import { providerErrorLine } from './providerError';

const path = '/media/1/enrich/resolve';

// HOLODEX-87: an unreachable provider reads as the handoff's sentence, never the raw
// status line; a rate-limit 503 keeps the server's own line, which carries the wait.
describe('providerErrorLine', () => {
	it('names the provider for a 502, with or without a server body', () => {
		expect(providerErrorLine(new ApiError(502, path, 'provider lookup failed'), 'tmdb')).toBe(
			'tmdb is unavailable right now.'
		);
		expect(providerErrorLine(new ApiError(502, path), 'tmdb')).toBe('tmdb is unavailable right now.');
	});
	it('treats a gateway timeout as unreachable too', () => {
		expect(providerErrorLine(new ApiError(504, path), 'tmdb')).toBe('tmdb is unavailable right now.');
	});
	it("keeps a paused provider's rate-limit line", () => {
		const line = 'tmdb is rate-limiting — try again in 42 s';
		expect(providerErrorLine(new ApiError(503, path, line), 'tmdb')).toBe(line);
	});
	it('passes any other failure through unchanged', () => {
		expect(providerErrorLine(new ApiError(500, path, 'boom'), 'tmdb')).toBe('boom');
		expect(providerErrorLine(new TypeError('Failed to fetch'), 'tmdb')).toBe('Failed to fetch');
	});
});
