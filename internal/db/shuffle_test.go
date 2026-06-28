package db

import "testing"

// holoShuffle must be deterministic, seed-sensitive, and well-distributed enough
// that ordering by it is a real shuffle (ADR-045).
func TestHoloShuffle(t *testing.T) {
	// Deterministic: same (id, seed) → same output.
	if holoShuffle(42, 7) != holoShuffle(42, 7) {
		t.Fatal("holoShuffle is not deterministic")
	}
	// Seed-sensitive: changing the seed changes the hash for a fixed id.
	if holoShuffle(42, 7) == holoShuffle(42, 8) {
		t.Error("holoShuffle ignores the seed")
	}
	// Id-sensitive: adjacent ids under one seed map to distinct, non-adjacent
	// values (a weak hash that returned id*k would cluster).
	if holoShuffle(1, 99) == holoShuffle(2, 99) {
		t.Error("holoShuffle collides on adjacent ids")
	}

	// Distribution sanity: bucket many ids by sign of the hash under one seed; a
	// healthy mix is roughly balanced, not all-positive or all-negative.
	const n = 1000
	var neg int
	for id := int64(0); id < n; id++ {
		if holoShuffle(id, 1) < 0 {
			neg++
		}
	}
	if neg < n/4 || neg > 3*n/4 {
		t.Errorf("hash sign distribution skewed: %d/%d negative", neg, n)
	}
}
