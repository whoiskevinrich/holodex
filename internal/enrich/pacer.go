package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Provider traffic contract (F66, ADR-103). Core paces every /resolve and /enrich
// call it makes to a sidecar through one token bucket per provider; a 429 from the
// sidecar pauses that bucket. The numbers are the contract's (§4.13), not tunables.
const (
	defaultRequestsPerSecond = 2.0
	defaultBurst             = 4
	minRequestsPerSecond     = 0.1
	maxRequestsPerSecond     = 50.0
	minBurst                 = 1
	maxBurst                 = 100
	// defaultRetryAfter applies when a 429 carries no usable Retry-After;
	// maxRetryAfter caps whatever the sidecar asked for.
	defaultRetryAfter = 30 * time.Second
	maxRetryAfter     = 300 * time.Second
)

// RateLimit is the pace Holodex holds its calls to one provider at. It is carried
// three ways with one precedence (ADR-103 D3, mirroring ADR-080 D2): the operator's
// metadata-sources.yaml `rate_limit:` → the provider's /describe `rate_limit` →
// DefaultRateLimit. A zero value for either key means "not given" and takes that
// key's default, so `{burst: 10}` is a valid partial declaration.
type RateLimit struct {
	RequestsPerSecond float64 `yaml:"requests_per_second" json:"requests_per_second"`
	Burst             int     `yaml:"burst" json:"burst"`
}

// UnmarshalJSON tolerates a malformed /describe declaration instead of failing the
// whole manifest (the contract promises a bad rate_limit is ignored with a warning,
// never an error to the provider): a non-object, or a key of the wrong type, marks
// the value malformed (negative) so Normalize reports !ok and the caller drops it.
func (rl *RateLimit) UnmarshalJSON(b []byte) error {
	var raw struct {
		RPS   json.RawMessage `json:"requests_per_second"`
		Burst json.RawMessage `json:"burst"`
	}
	malformed := RateLimit{RequestsPerSecond: -1}
	if err := json.Unmarshal(b, &raw); err != nil {
		*rl = malformed
		return nil
	}
	var out RateLimit
	if len(raw.RPS) > 0 && json.Unmarshal(raw.RPS, &out.RequestsPerSecond) != nil {
		*rl = malformed
		return nil
	}
	if len(raw.Burst) > 0 && json.Unmarshal(raw.Burst, &out.Burst) != nil {
		*rl = malformed
		return nil
	}
	*rl = out
	return nil
}

// DefaultRateLimit is the pace every provider gets when nobody declares one.
var DefaultRateLimit = RateLimit{RequestsPerSecond: defaultRequestsPerSecond, Burst: defaultBurst}

// Normalize fills missing keys from the default and clamps the rest into the
// contract's range. ok is false when the declaration is malformed (negative, NaN or
// infinite) — the caller drops the whole object and the default applies. clamped is
// true when an in-range-but-out-of-bounds value was pulled to the edge, so the
// caller can log it (the contract promises out-of-range is clamped *and* logged).
func (rl RateLimit) Normalize() (out RateLimit, clamped, ok bool) {
	rps, burst := rl.RequestsPerSecond, rl.Burst
	if math.IsNaN(rps) || math.IsInf(rps, 0) || rps < 0 || burst < 0 {
		return RateLimit{}, false, false
	}
	if rps == 0 {
		rps = defaultRequestsPerSecond
	}
	if burst == 0 {
		burst = defaultBurst
	}
	if rps < minRequestsPerSecond {
		rps, clamped = minRequestsPerSecond, true
	} else if rps > maxRequestsPerSecond {
		rps, clamped = maxRequestsPerSecond, true
	}
	if burst < minBurst {
		burst, clamped = minBurst, true
	} else if burst > maxBurst {
		burst, clamped = maxBurst, true
	}
	return RateLimit{RequestsPerSecond: rps, Burst: burst}, clamped, true
}

// ErrProviderPaused is the typed back-pressure signal (ADR-103 D2): the provider's
// bucket is paused for RetryAfter more seconds because the sidecar answered 429. The
// pacer returns it immediately rather than sleeping through the pause — whether to
// wait is the caller's policy (D4): the sweep runner waits and retries once, an
// interactive handler maps it to 503 + Retry-After.
type ErrProviderPaused struct {
	Provider   string
	RetryAfter time.Duration
}

func (e *ErrProviderPaused) Error() string {
	return fmt.Sprintf("provider %q is rate-limiting; retry after %s", e.Provider, e.RetryAfter.Round(time.Second))
}

// RetryAfterSeconds is the value for an outbound Retry-After header — whole
// seconds, rounded up, never less than 1.
func (e *ErrProviderPaused) RetryAfterSeconds() int {
	s := int(math.Ceil(e.RetryAfter.Seconds()))
	if s < 1 {
		return 1
	}
	return s
}

// errRateLimited is what the HTTP transport returns for a 429: the sidecar's
// Retry-After, already defaulted and capped. The pacer decorator turns it into a
// pause plus an *ErrProviderPaused; nothing above the decorator sees it.
type errRateLimited struct {
	RetryAfter time.Duration
}

func (e *errRateLimited) Error() string {
	return fmt.Sprintf("provider returned 429 (retry after %s)", e.RetryAfter)
}

// parseRetryAfter reads a Retry-After header as delta-seconds only (the contract's
// form; the HTTP-date form counts as unparseable). Absent, unparseable or
// non-positive → defaultRetryAfter; anything larger than maxRetryAfter is capped.
func parseRetryAfter(header string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil || n <= 0 {
		return defaultRetryAfter
	}
	d := time.Duration(n) * time.Second
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	return d
}

// pacer is one provider's token bucket plus its 429 pause. It is created once per
// provider name and never re-created (ADR-103 D1): setLimit adjusts the live bucket
// so in-flight tokens and an active pause survive a reload-config or a changed
// /describe declaration. The clock and the sleep are injected (D5) so tests advance
// a fake now and assert computed delays instead of sleeping.
type pacer struct {
	provider string
	now      func() time.Time
	sleep    func(context.Context, time.Duration) error

	mu          sync.Mutex
	lim         *rate.Limiter
	cur         RateLimit
	pausedUntil time.Time
}

func newPacer(provider string, rl RateLimit) *pacer {
	return &pacer{
		provider: provider,
		now:      time.Now,
		sleep:    sleepCtx,
		lim:      rate.NewLimiter(rate.Limit(rl.RequestsPerSecond), rl.Burst),
		cur:      rl,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// setLimit applies a (normalized) limit to the live bucket, only if it changed.
func (p *pacer) setLimit(rl RateLimit) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if rl == p.cur {
		return
	}
	now := p.now()
	p.lim.SetLimitAt(now, rate.Limit(rl.RequestsPerSecond))
	p.lim.SetBurstAt(now, rl.Burst)
	p.cur = rl
}

// pause holds the bucket closed for d from now (a 429's Retry-After). A later, longer
// pause extends it; a shorter one never shortens an existing pause.
func (p *pacer) pause(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	until := p.now().Add(d)
	if until.After(p.pausedUntil) {
		p.pausedUntil = until
	}
}

// acquire takes one token, sleeping at most one bucket delay (burst / rps). A paused
// bucket returns *ErrProviderPaused at once — the pacer never sleeps through a pause.
// A cancelled context during the delay returns the context's error and gives the
// reservation back.
func (p *pacer) acquire(ctx context.Context) error {
	p.mu.Lock()
	now := p.now()
	if now.Before(p.pausedUntil) {
		remaining := p.pausedUntil.Sub(now)
		p.mu.Unlock()
		return &ErrProviderPaused{Provider: p.provider, RetryAfter: remaining}
	}
	res := p.lim.ReserveN(now, 1)
	p.mu.Unlock()
	if !res.OK() {
		// Only possible when burst < 1, which Normalize forbids.
		return fmt.Errorf("provider %q: rate limiter cannot admit a request", p.provider)
	}
	delay := res.DelayFrom(now)
	if delay <= 0 {
		return nil
	}
	if err := p.sleep(ctx, delay); err != nil {
		res.CancelAt(p.now())
		return err
	}
	return nil
}

// pacedClient wraps a ProviderClient with its provider's pacer (ADR-103 D1). Only
// /resolve and /enrich are paced: /describe is Holodex's own probe, piggybacked on
// every call by verifiedClient, never upstream traffic. The limit is re-resolved
// on every admission so a reload-config or a changed /describe declaration is live
// on the next call. Wrapping happens above newClient, so injected test fakes are
// paced exactly like the HTTP transport.
type pacedClient struct {
	inner ProviderClient
	p     *pacer
	limit func() RateLimit
}

func (c *pacedClient) Describe(ctx context.Context) (Manifest, error) {
	return c.inner.Describe(ctx)
}

func (c *pacedClient) Resolve(ctx context.Context, entityType string, hint Hint) (ResolveResult, error) {
	if err := c.admit(ctx); err != nil {
		return ResolveResult{}, err
	}
	out, err := c.inner.Resolve(ctx, entityType, hint)
	return out, c.observe(err)
}

func (c *pacedClient) Enrich(ctx context.Context, entityType, externalID string) (EnrichResult, error) {
	if err := c.admit(ctx); err != nil {
		return EnrichResult{}, err
	}
	out, err := c.inner.Enrich(ctx, entityType, externalID)
	return out, c.observe(err)
}

func (c *pacedClient) admit(ctx context.Context) error {
	c.p.setLimit(c.limit())
	return c.p.acquire(ctx)
}

// observe turns the transport's 429 into a pause on this provider's bucket and the
// typed error the callers branch on; every other error passes through untouched.
func (c *pacedClient) observe(err error) error {
	var rl *errRateLimited
	if errors.As(err, &rl) {
		c.p.pause(rl.RetryAfter)
		return &ErrProviderPaused{Provider: c.p.provider, RetryAfter: rl.RetryAfter}
	}
	return err
}
