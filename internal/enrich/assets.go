package enrich

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"holodex/internal/model"
)

// Asset download (F25, ADR-038 / ADR-039). A person enrich run can return asset
// URLs (e.g. a portrait); these are fetched through the SAME SSRF perimeter as the
// provider's API calls and never an unguarded http.Get:
//
//   - The asset URL host must be on the source's allowlist — its own base_url host,
//     or a host the operator listed in asset_hosts (ADR-039). Trust comes only from
//     that config, never from the provider response, so a provider cannot point the
//     fetch at an arbitrary internal address.
//   - Plain http is allowed only for the base_url host (the trusted internal
//     network); a cross-host (CDN) asset must travel over https.
//   - Cross-host redirects are refused (same CheckRedirect as the API client).
//   - The body is size-capped and the request timeout-bounded.
//
// The bytes are returned raw; the caller runs them through personimage.Normalize
// (the metadata strip) before anything is written to disk.

// maxAssetBytes caps a downloaded asset (a portrait is small; this stops a hostile
// provider streaming an unbounded body). Larger than the JSON response cap because
// images legitimately exceed 1 MiB.
const maxAssetBytes = 16 << 20 // 16 MiB

// AssetClient fetches a provider asset URL under the SSRF guard. Constructed per
// Source so it carries that source's host allowlist.
type AssetClient struct {
	baseHost string          // the source's base_url host — http or https allowed
	allowed  map[string]bool // every host the fetch may target (base host + asset_hosts)
	hc       *http.Client
}

// newAssetClient builds an asset fetcher whose allowlist is the source's base_url
// host plus any operator-configured asset_hosts (ADR-039).
func newAssetClient(src Source) *AssetClient {
	baseHost := ""
	if u, err := url.Parse(src.base()); err == nil {
		baseHost = normalizeHost(u.Host)
	}
	allowed := make(map[string]bool, 1+len(src.AssetHosts))
	if baseHost != "" {
		allowed[baseHost] = true
	}
	for _, h := range src.AssetHosts {
		if h = normalizeHost(h); h != "" {
			allowed[h] = true
		}
	}
	return &AssetClient{
		baseHost: baseHost,
		allowed:  allowed,
		hc: &http.Client{
			Timeout:       15 * time.Second,
			CheckRedirect: refuseCrossHostRedirect,
		},
	}
}

// normalizeHost lowercases and trims a host for case-insensitive allowlist matching
// (host names are case-insensitive; url.Parse does not normalize their case).
func normalizeHost(h string) string { return strings.ToLower(strings.TrimSpace(h)) }

// normalizeHosts cleans a configured asset_hosts list: trim/lowercase each entry and
// drop empties (deduplication is implicit — the allowlist is a set).
func normalizeHosts(in []string) []string {
	out := make([]string, 0, len(in))
	for _, h := range in {
		if h = normalizeHost(h); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// Fetch downloads an asset URL, refusing any host other than the provider's own
// (the SSRF allowlist) and capping the body. Returns the raw bytes for the caller
// to normalize.
func (c *AssetClient) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse asset url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("asset url scheme %q not allowed", u.Scheme)
	}
	host := normalizeHost(u.Host)
	if !c.allowed[host] {
		// The fetch may only target a host the operator trusts — base_url's own host
		// or an asset_hosts entry — never an address derived from the provider response.
		return nil, fmt.Errorf("asset host %q not on the provider allowlist", u.Host)
	}
	if host != c.baseHost && u.Scheme != "https" {
		// Plain http is allowed only for the base_url host on the trusted internal
		// network; a cross-host (CDN) asset must travel over https (ADR-039 §3).
		return nil, fmt.Errorf("cross-host asset %q must use https", u.Host)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build asset request: %w", err)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("asset request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("asset returned %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read asset body: %w", err)
	}
	if int64(len(data)) > maxAssetBytes {
		return nil, fmt.Errorf("asset exceeds %d bytes", maxAssetBytes)
	}
	return data, nil
}

// assetRoleFor maps a provider asset kind to a person-image core role (F25). photo
// → headshot is the default; banner/poster map through when a provider supplies
// them. An unknown kind returns ok=false so the asset is skipped (never stored under
// a guessed role).
func assetRoleFor(kind string) (string, bool) {
	switch kind {
	case "photo", "portrait", "headshot", "":
		return model.PersonImageHeadshot, true
	case "banner", "backdrop":
		return model.PersonImageBanner, true
	case "poster":
		return model.PersonImagePoster, true
	default:
		return "", false
	}
}
