// Package cache defines the cache abstraction (config-and-settings.md). Only the
// Noop backend exists; an in-process or Redis backend can be added later without
// touching the service layer by implementing this interface.
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

func (Noop) Get(context.Context, string) ([]byte, bool)               { return nil, false }
func (Noop) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (Noop) Invalidate(context.Context, string) error                 { return nil }
func (Noop) InvalidatePrefix(context.Context, string) error           { return nil }

// New returns a cache for the given backend. The in-process ristretto backend is
// deferred until there is a measured need (config-and-settings.md): the read paths
// meet the NFR on SQLite alone, so "memory" resolves to Noop for now. The
// interface stays stable so a real backend drops in later without touching the
// service layer.
func New(backend string) Cache {
	switch backend {
	case "none":
		return Noop{}
	default:
		return Noop{} // in-process cache deferred
	}
}
