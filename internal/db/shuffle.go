package db

import (
	"database/sql/driver"
	"sync"

	sqlite "modernc.org/sqlite"
)

// holoShuffle is a deterministic, well-distributed integer hash of a row id mixed
// with a caller-supplied seed (ADR-045). It backs the holo_shuffle() SQL function
// that orders the paginated Media list for the "Random" sort: because the order is
// a pure function of (id, seed), LIMIT/OFFSET windows tile exactly under a fixed
// seed — no duplicate or skipped rows across pages, which ORDER BY RANDOM() cannot
// guarantee. The mix is splitmix64; the output is non-cryptographic by design (a
// cosmetic shuffle, not a security primitive).
func holoShuffle(id, seed int64) int64 {
	x := uint64(id) ^ (uint64(seed) * 0x9E3779B97F4A7C15)
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	x ^= x >> 31
	return int64(x)
}

var (
	shuffleOnce sync.Once
	shuffleErr  error
)

// registerShuffle registers holo_shuffle(id, seed) as a deterministic scalar
// function, available to every connection opened afterwards. Idempotent
// (sync.Once) so repeated Open calls — e.g. across tests — don't re-register the
// same name and error.
func registerShuffle() error {
	shuffleOnce.Do(func() {
		shuffleErr = sqlite.RegisterDeterministicScalarFunction(
			"holo_shuffle", 2,
			func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
				id, _ := args[0].(int64)
				seed, _ := args[1].(int64)
				return holoShuffle(id, seed), nil
			},
		)
	})
	return shuffleErr
}
