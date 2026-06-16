package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTP transport limits (ADR-033 F22.9b — provider responses are untrusted).
const (
	maxResponseBytes = 1 << 20         // 1 MiB cap on any provider response body
	requestTimeout   = 8 * time.Second // per-call ceiling
)

// httpClient calls one provider sidecar over its configured base_url. It is the
// SSRF perimeter (ADR-033 F22.9c): the base_url comes from trusted config, and
// the client refuses to follow a redirect to a different host — a provider
// response cannot bounce core onto an internal address it didn't configure.
type httpClient struct {
	base string
	hc   *http.Client
}

func newHTTPClient(src Source) *httpClient {
	return &httpClient{
		base: src.base(),
		hc: &http.Client{
			Timeout: requestTimeout,
			// Disallow cross-host redirects: a 30x to another host is treated as the
			// final response (not followed), closing the SSRF redirect vector. Same-host
			// redirects (rare for an API) are still followed.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 0 && req.URL.Host != via[0].URL.Host {
					return http.ErrUseLastResponse
				}
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// base normalizes the configured URL to have no trailing slash.
func (s Source) base() string {
	b := s.BaseURL
	for len(b) > 0 && b[len(b)-1] == '/' {
		b = b[:len(b)-1]
	}
	return b
}

func (c *httpClient) Describe(ctx context.Context) (Manifest, error) {
	var m Manifest
	err := c.do(ctx, http.MethodGet, "/describe", nil, &m)
	return m, err
}

func (c *httpClient) Resolve(ctx context.Context, entityType string, hint Hint) ([]Candidate, error) {
	body := map[string]any{"entity_type": entityType, "hint": hint}
	var out struct {
		Candidates []Candidate `json:"candidates"`
	}
	if err := c.do(ctx, http.MethodPost, "/resolve", body, &out); err != nil {
		return nil, err
	}
	return out.Candidates, nil
}

func (c *httpClient) Enrich(ctx context.Context, entityType, externalID string) (EnrichResult, error) {
	body := map[string]any{"entity_type": entityType, "external_id": externalID}
	var out EnrichResult
	err := c.do(ctx, http.MethodPost, "/enrich", body, &out)
	return out, err
}

// do issues one request to the provider and decodes a size-capped JSON response.
// A non-2xx status, transport error, or malformed body fails just this call —
// the caller turns that into a single failed fetch, never a crash (F22.9b).
func (c *httpClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("provider request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned %d", resp.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode provider response: %w", err)
	}
	return nil
}
