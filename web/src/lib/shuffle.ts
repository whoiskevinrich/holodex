// Deterministic seeded shuffle for the client-side "Random" sort on the unpaged
// People/Tags lists (ADR-045 §3). A mulberry32 PRNG drives a Fisher–Yates shuffle,
// so the same (input order, seed) always yields the same order — letting the order
// stay stable across re-renders and reshuffle only when the session seed changes.
// Returns a new array; the input is not mutated. Non-cryptographic by design.
export function seededShuffle<T>(items: readonly T[], seed: number): T[] {
	const out = items.slice();
	let a = seed >>> 0;
	const rand = () => {
		a = (a + 0x6d2b79f5) | 0;
		let t = Math.imul(a ^ (a >>> 15), 1 | a);
		t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};
	for (let i = out.length - 1; i > 0; i--) {
		const j = Math.floor(rand() * (i + 1));
		[out[i], out[j]] = [out[j], out[i]];
	}
	return out;
}
