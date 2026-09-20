package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeClock drives a pacer without sleeping: now advances only when the test says
// so, and sleep records each requested delay and advances the clock by it.
type fakeClock struct {
	t      time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.sleeps = append(c.sleeps, d)
	c.t = c.t.Add(d)
	return nil
}

func testPacer(rl RateLimit) (*pacer, *fakeClock) {
	clk := newFakeClock()
	p := newPacer("tmdb", rl)
	p.now, p.sleep = clk.now, clk.sleep
	return p, clk
}

func TestRateLimitNormalize(t *testing.T) {
	cases := []struct {
		name    string
		in      RateLimit
		want    RateLimit
		clamped bool
		ok      bool
	}{
		{"zero takes default", RateLimit{}, DefaultRateLimit, false, true},
		{"partial burst", RateLimit{Burst: 10}, RateLimit{2, 10}, false, true},
		{"partial rps", RateLimit{RequestsPerSecond: 10}, RateLimit{10, 4}, false, true},
		{"in range", RateLimit{10, 20}, RateLimit{10, 20}, false, true},
		{"rps below floor clamps", RateLimit{0.01, 4}, RateLimit{0.1, 4}, true, true},
		{"rps above cap clamps", RateLimit{500, 4}, RateLimit{50, 4}, true, true},
		{"burst above cap clamps", RateLimit{2, 1000}, RateLimit{2, 100}, true, true},
		{"negative rps malformed", RateLimit{-1, 4}, RateLimit{}, false, false},
		{"negative burst malformed", RateLimit{2, -1}, RateLimit{}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, clamped, ok := tc.in.Normalize()
			if got != tc.want || clamped != tc.clamped || ok != tc.ok {
				t.Fatalf("Normalize(%+v) = %+v, clamped=%v, ok=%v; want %+v, %v, %v",
					tc.in, got, clamped, ok, tc.want, tc.clamped, tc.ok)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	cases := map[string]time.Duration{
		"":                              defaultRetryAfter,
		"42":                            42 * time.Second,
		" 7 ":                           7 * time.Second,
		"0":                             defaultRetryAfter,
		"-5":                            defaultRetryAfter,
		"9000":                          maxRetryAfter,
		"Sat, 19 Sep 2026 12:00:00 GMT": defaultRetryAfter, // HTTP-date form is unparseable by contract
		"soon":                          defaultRetryAfter,
	}
	for in, want := range cases {
		if got := parseRetryAfter(in); got != want {
			t.Errorf("parseRetryAfter(%q) = %s; want %s", in, got, want)
		}
	}
}

func TestPacerBurstThenDelay(t *testing.T) {
	p, clk := testPacer(DefaultRateLimit) // 2 req/s, burst 4
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if err := p.acquire(ctx); err != nil {
			t.Fatalf("acquire %d within burst: %v", i, err)
		}
	}
	if len(clk.sleeps) != 0 {
		t.Fatalf("burst should not sleep, slept %v", clk.sleeps)
	}
	if err := p.acquire(ctx); err != nil {
		t.Fatalf("acquire 5: %v", err)
	}
	if len(clk.sleeps) != 1 || clk.sleeps[0] != 500*time.Millisecond {
		t.Fatalf("fifth acquire should wait one token interval (500ms), slept %v", clk.sleeps)
	}
}

func TestPacerSetLimitAdjustsLiveBucket(t *testing.T) {
	p, clk := testPacer(DefaultRateLimit)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_ = p.acquire(ctx)
	}
	p.setLimit(RateLimit{RequestsPerSecond: 10, Burst: 4}) // bucket drained; refill now 10/s
	if err := p.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if len(clk.sleeps) != 1 || clk.sleeps[0] != 100*time.Millisecond {
		t.Fatalf("after SetLimit(10/s) the next token is 100ms away, slept %v", clk.sleeps)
	}
}

func TestPacerPauseReturnsImmediately(t *testing.T) {
	p, clk := testPacer(DefaultRateLimit)
	ctx := context.Background()
	p.pause(30 * time.Second)
	clk.t = clk.t.Add(12 * time.Second)

	err := p.acquire(ctx)
	var paused *ErrProviderPaused
	if !errors.As(err, &paused) {
		t.Fatalf("acquire on a paused bucket = %v; want *ErrProviderPaused", err)
	}
	if paused.Provider != "tmdb" || paused.RetryAfter != 18*time.Second || paused.RetryAfterSeconds() != 18 {
		t.Fatalf("paused = %+v (secs %d); want tmdb / 18s", paused, paused.RetryAfterSeconds())
	}
	if len(clk.sleeps) != 0 {
		t.Fatalf("a pause must never be slept through, slept %v", clk.sleeps)
	}

	clk.t = clk.t.Add(18 * time.Second) // pause elapsed
	if err := p.acquire(ctx); err != nil {
		t.Fatalf("acquire after the pause elapsed: %v", err)
	}
}

func TestPacerPauseNeverShortens(t *testing.T) {
	p, clk := testPacer(DefaultRateLimit)
	p.pause(60 * time.Second)
	p.pause(5 * time.Second)
	clk.t = clk.t.Add(10 * time.Second)
	var paused *ErrProviderPaused
	if err := p.acquire(context.Background()); !errors.As(err, &paused) || paused.RetryAfter != 50*time.Second {
		t.Fatalf("shorter pause must not cut the longer one: %v", err)
	}
}

func TestPacerCancelledContextReturnsToken(t *testing.T) {
	p, clk := testPacer(DefaultRateLimit)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_ = p.acquire(ctx)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := p.acquire(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("acquire with cancelled ctx = %v; want context.Canceled", err)
	}
	// The cancelled reservation was given back: the next caller waits one interval,
	// not two.
	if err := p.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if got := clk.sleeps[len(clk.sleeps)-1]; got != 500*time.Millisecond {
		t.Fatalf("next acquire should wait 500ms, waited %s (all sleeps %v)", got, clk.sleeps)
	}
}

// stubClient is the ProviderClient behind a pacedClient in these tests.
type stubClient struct {
	calls   int
	resolve error
}

func (s *stubClient) Describe(context.Context) (Manifest, error) { return Manifest{}, nil }
func (s *stubClient) Resolve(context.Context, string, Hint) (ResolveResult, error) {
	s.calls++
	return ResolveResult{}, s.resolve
}
func (s *stubClient) Enrich(context.Context, string, string) (EnrichResult, error) {
	s.calls++
	return EnrichResult{}, nil
}

func TestPacedClient429PausesAndTypes(t *testing.T) {
	p, clk := testPacer(DefaultRateLimit)
	inner := &stubClient{resolve: &errRateLimited{RetryAfter: 45 * time.Second}}
	limit := DefaultRateLimit
	c := &pacedClient{inner: inner, p: p, limit: func() RateLimit { return limit }}
	ctx := context.Background()

	_, err := c.Resolve(ctx, "person", Hint{})
	var paused *ErrProviderPaused
	if !errors.As(err, &paused) || paused.RetryAfter != 45*time.Second || paused.Provider != "tmdb" {
		t.Fatalf("429 from the transport should surface as *ErrProviderPaused{tmdb,45s}: %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("inner called %d times; want 1", inner.calls)
	}

	// The bucket is now paused: the next call never reaches the transport.
	clk.t = clk.t.Add(time.Second)
	_, err = c.Enrich(ctx, "person", "x")
	if !errors.As(err, &paused) || paused.RetryAfter != 44*time.Second {
		t.Fatalf("second call during the pause = %v; want paused 44s", err)
	}
	if inner.calls != 1 {
		t.Fatalf("a paused bucket must not call the transport (calls=%d)", inner.calls)
	}

	// /describe is never paced.
	if _, err := c.Describe(ctx); err != nil {
		t.Fatalf("Describe during a pause: %v", err)
	}

	// A changed limit is picked up on the next admission, after the pause.
	clk.t = clk.t.Add(44 * time.Second)
	inner.resolve = nil
	limit = RateLimit{RequestsPerSecond: 10, Burst: 1}
	if _, err := c.Resolve(ctx, "person", Hint{}); err != nil {
		t.Fatalf("resolve after the pause: %v", err)
	}
	if p.cur != limit {
		t.Fatalf("pacer limit = %+v; want %+v applied on admission", p.cur, limit)
	}
}

func TestPacedClientOtherErrorsPassThrough(t *testing.T) {
	p, _ := testPacer(DefaultRateLimit)
	want := errors.New("provider returned 502")
	inner := &stubClient{resolve: want}
	c := &pacedClient{inner: inner, p: p, limit: func() RateLimit { return DefaultRateLimit }}
	if _, err := c.Resolve(context.Background(), "person", Hint{}); !errors.Is(err, want) {
		t.Fatalf("non-429 error should pass through unchanged: %v", err)
	}
	if err := p.acquire(context.Background()); err != nil {
		t.Fatalf("a non-429 error must not pause the bucket: %v", err)
	}
}

func TestRegistryLoadRateLimitValidation(t *testing.T) {
	path := writeSources(t, `
sources:
  - name: tuned
    base_url: http://tuned:9100
    entity_types: [person]
    enabled: true
    rate_limit: { requests_per_second: 10, burst: 20 }
  - name: partial
    base_url: http://partial:9100
    entity_types: [person]
    enabled: true
    rate_limit: { burst: 8 }
  - name: hot
    base_url: http://hot:9100
    entity_types: [person]
    enabled: true
    rate_limit: { requests_per_second: 500 }
  - name: bad
    base_url: http://bad:9100
    entity_types: [person]
    enabled: true
    rate_limit: { requests_per_second: -1 }
  - name: plain
    base_url: http://plain:9100
    entity_types: [person]
    enabled: true
`)
	var buf bytes.Buffer
	reg, err := Load(path, slog.New(slog.NewTextHandler(&buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]*RateLimit{
		"tuned":   {10, 20},
		"partial": {2, 8},
		"hot":     {50, 4},
		"bad":     nil,
		"plain":   nil,
	}
	for name, rl := range want {
		src, ok := reg.ByName(name)
		if !ok {
			t.Fatalf("%s must stay enabled", name)
		}
		switch {
		case rl == nil && src.RateLimit != nil:
			t.Errorf("%s: rate_limit should be dropped, got %+v", name, *src.RateLimit)
		case rl != nil && (src.RateLimit == nil || *src.RateLimit != *rl):
			t.Errorf("%s: rate_limit = %+v; want %+v", name, src.RateLimit, *rl)
		}
	}
	for _, msg := range []string{"hot.rate_limit", "bad.rate_limit"} {
		if !strings.Contains(buf.String(), msg) {
			t.Errorf("expected a warning naming %s, got log: %s", msg, buf.String())
		}
	}
}

func TestRateLimitPrecedence(t *testing.T) {
	path := writeSources(t, `
sources:
  - name: fixed
    base_url: http://fixed:9100
    entity_types: [person]
    enabled: true
    rate_limit: { requests_per_second: 1, burst: 1 }
  - name: free
    base_url: http://free:9100
    entity_types: [person]
    enabled: true
`)
	store, err := NewStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	s := &Service{store: store, log: slog.New(slog.NewTextHandler(&buf, nil))}

	if got := s.rateLimitFor("free"); got != DefaultRateLimit {
		t.Fatalf("undeclared provider = %+v; want default", got)
	}
	s.persistRateLimit("free", Manifest{RateLimit: &RateLimit{RequestsPerSecond: 10, Burst: 200}})
	if got := s.rateLimitFor("free"); got != (RateLimit{10, 100}) {
		t.Fatalf("/describe declaration (clamped) = %+v; want {10 100}", got)
	}
	if !strings.Contains(buf.String(), "free./describe.rate_limit") {
		t.Errorf("clamp should be logged naming the provider, got: %s", buf.String())
	}
	s.persistRateLimit("fixed", Manifest{RateLimit: &RateLimit{RequestsPerSecond: 10, Burst: 20}})
	if got := s.rateLimitFor("fixed"); got != (RateLimit{1, 1}) {
		t.Fatalf("operator yaml must outrank /describe, got %+v", got)
	}
	s.persistRateLimit("free", Manifest{}) // provider stops declaring
	if got := s.rateLimitFor("free"); got != DefaultRateLimit {
		t.Fatalf("absent /describe key should clear the cache, got %+v", got)
	}
	s.persistRateLimit("free", Manifest{RateLimit: &RateLimit{RequestsPerSecond: -3}})
	if got := s.rateLimitFor("free"); got != DefaultRateLimit {
		t.Fatalf("malformed /describe object should be ignored, got %+v", got)
	}
}

func TestServiceClientIsPacedPerProvider(t *testing.T) {
	path := writeSources(t, `
sources:
  - name: a
    base_url: http://a:9100
    entity_types: [person]
    enabled: true
  - name: b
    base_url: http://b:9100
    entity_types: [person]
    enabled: true
`)
	store, err := NewStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := NewServiceWithClient(store, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		func(Source) ProviderClient { return &stubClient{} })

	_, c1, err := s.client("a")
	if err != nil {
		t.Fatal(err)
	}
	_, c2, _ := s.client("a")
	_, c3, _ := s.client("b")
	pa, pb := c1.(*pacedClient).p, c3.(*pacedClient).p
	if pa != c2.(*pacedClient).p {
		t.Fatal("the same provider must share one pacer across clients")
	}
	if pa == pb {
		t.Fatal("different providers must not share a bucket")
	}
	if pa.provider != "a" || pb.provider != "b" {
		t.Fatalf("pacers keyed wrong: %q %q", pa.provider, pb.provider)
	}
}

func TestHTTPClient429IsTyped(t *testing.T) {
	retryAfter := "17"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := newHTTPClient(Source{BaseURL: srv.URL})

	_, err := c.Resolve(context.Background(), "person", Hint{})
	var rl *errRateLimited
	if !errors.As(err, &rl) || rl.RetryAfter != 17*time.Second {
		t.Fatalf("429 + Retry-After: 17 should be errRateLimited{17s}, got %v", err)
	}
	retryAfter = ""
	_, err = c.Enrich(context.Background(), "person", "x")
	if !errors.As(err, &rl) || rl.RetryAfter != defaultRetryAfter {
		t.Fatalf("429 without Retry-After should default to %s, got %v", defaultRetryAfter, err)
	}
}

func TestRateLimitUnmarshalJSONToleratesMalformed(t *testing.T) {
	cases := map[string]RateLimit{
		`{"requests_per_second": 10, "burst": 20}`: {10, 20},
		`{"burst": 8}`:                   {0, 8},
		`{}`:                             {},
		`"fast"`:                         {RequestsPerSecond: -1},
		`{"requests_per_second": "ten"}`: {RequestsPerSecond: -1},
		`{"burst": 4.5}`:                 {RequestsPerSecond: -1},
	}
	for in, want := range cases {
		var m Manifest
		if err := json.Unmarshal([]byte(`{"provider":"t","protocol_version":1,"rate_limit":`+in+`}`), &m); err != nil {
			t.Fatalf("a bad rate_limit must never fail the manifest: %q → %v", in, err)
		}
		if m.RateLimit == nil || *m.RateLimit != want {
			t.Errorf("rate_limit %s = %+v; want %+v", in, m.RateLimit, want)
		}
	}
	var m Manifest
	if err := json.Unmarshal([]byte(`{"provider":"t","protocol_version":1}`), &m); err != nil || m.RateLimit != nil {
		t.Fatalf("absent rate_limit should stay nil: %+v %v", m.RateLimit, err)
	}
}
