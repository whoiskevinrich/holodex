// Package cache defines the cache abstraction (ADR-008). Phase 1 ships the
// in-process and noop backends; a Redis backend can be added without touching
// the service layer by implementing this interface.
package cache

import (
	"context"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Invalidate(ctx context.Context, key string) error
	InvalidatePrefix(ctx context.Context, prefix string) error
}

// Noop is a cache that stores nothing; useful for tests and CACHE_BACKEND=none.
type Noop struct{}

func (Noop) Get(context.Context, string) ([]byte, bool)            { return nil, false }
func (Noop) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (Noop) Invalidate(context.Context, string) error             { return nil }
func (Noop) InvalidatePrefix(context.Context, string) error       { return nil }

// New returns a cache for the given backend. The in-process ristretto backend
// (CACHE_BACKEND=memory) lands with the service layer; until then "memory"
// falls back to Noop so the app runs.
func New(backend string, maxMemoryMB int) Cache {
	switch backend {
	case "none":
		return Noop{}
	default:
		// TODO(phase1): ristretto-backed InProcess cache (ADR-008).
		return Noop{}
	}
}
