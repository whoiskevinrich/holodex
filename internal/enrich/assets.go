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
// URLs (e.g. a portrait); these are fetched through the SSRF perimeter:
//
//   - The asset URL host must be on the allowlist: the provider's own base_url host
//     (implicit, always allowed) OR a host the operator listed in asset_hosts for
//     this source (ADR-039 §3). A provider cannot reach an unlisted address.
//   - Any host other than the provider's base_url host MUST use https (public CDN).
//   - Cross-host redirects are refused (the redirect target must be re-checked).
//   - The body is size-capped and the request timeout-bounded.
//
// The bytes are returned raw; the caller runs them through personimage.Normalize
// (the metadata strip) before anything is written to disk.

// maxAssetBytes caps a downloaded asset. Larger than the JSON response cap because
// images legitimately exceed 1 MiB.
const maxAssetBytes = 16 << 20 // 16 MiB

// AssetClient fetches a provider asset URL under the SSRF guard. Constructed per
// Source so it carries that source's per-source allowlist (ADR-039).
type AssetClient struct {
	baseHost     string              // provider's own base_url host; http is allowed here
	allowedHosts map[string]struct{} // {baseHost} ∪ asset_hosts; non-base hosts require https
	hc           *http.Client
}

// newAssetClient builds an asset fetcher whose host allowlist is the provider's
// base_url host (always included) plus any operator-listed asset_hosts (ADR-039 §3).
func newAssetClient(src Source) *AssetClient {
	hosts := make(map[string]struct{})
	baseHost := ""
	if u, err := url.Parse(src.base()); err == nil && u.Host != "" {
		baseHost = u.Host
		hosts[u.Host] = struct{}{}
	}
	for _, h := range src.AssetHosts {
		if h = strings.TrimSpace(h); h != "" {
			hosts[h] = struct{}{}
		}
	}
	return &AssetClient{
		baseHost:     baseHost,
		allowedHosts: hosts,
		hc: &http.Client{
			Timeout: 15 * time.Second,
			// Cross-host redirects stop at the 30x — the redirect target is not on the
			// allowlist even if the origin was. Same-host redirects (≤5) are followed.
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

// checkHost enforces the asset URL host policy (ADR-039 §3): scheme http/https, host
// on {base_url host} ∪ asset_hosts, and https for any non-base (public-CDN) host. It
// is the single source of that policy — Fetch and assetHostAllowed both call it, so
// the download gate and the render gate can never drift.
func (c *AssetClient) checkHost(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("asset url scheme %q not allowed", u.Scheme)
	}
	if _, ok := c.allowedHosts[u.Host]; !ok {
		return fmt.Errorf("asset host %q not on the provider allowlist", u.Host)
	}
	if u.Host != c.baseHost && u.Scheme != "https" {
		return fmt.Errorf("asset host %q requires https", u.Host)
	}
	return nil
}

// assetHostAllowed reports whether rawURL passes checkHost WITHOUT fetching. It is the
// render-time gate for a provider-hinted image_url value (F39, ADR-056/ADR-039): a
// value that would be refused as a download must not be emitted as an <img> src either.
func assetHostAllowed(src Source, rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return false
	}
	return newAssetClient(src).checkHost(u) == nil
}

// Fetch downloads an asset URL through the per-source SSRF guard and returns the
// raw bytes for the caller to normalize. The host must be on the allowlist; any
// host other than the provider's own base host additionally requires https.
func (c *AssetClient) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse asset url: %w", err)
	}
	if err := c.checkHost(u); err != nil {
		return nil, err
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
	case "gallery":
		return model.PersonImageExtra, true
	default:
		return "", false
	}
}
