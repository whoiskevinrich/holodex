package enrich

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"holodex/internal/model"
)

// Asset download (F24, ADR-037 / ADR-033 F14.3). A person enrich run can return
// asset URLs (e.g. a portrait); these are fetched through the SAME SSRF perimeter
// as the provider's API calls and never an unguarded http.Get:
//
//   - The asset URL host must equal the provider's configured base_url host (the
//     allowlist already vetted base_url from trusted config). A provider cannot
//     point the fetch at an arbitrary internal address.
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
	allowedHost string
	hc          *http.Client
}

// newAssetClient builds an asset fetcher pinned to the source's base_url host.
func newAssetClient(src Source) *AssetClient {
	host := ""
	if u, err := url.Parse(src.base()); err == nil {
		host = u.Host
	}
	return &AssetClient{
		allowedHost: host,
		hc: &http.Client{
			Timeout: 15 * time.Second,
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
	if c.allowedHost == "" || u.Host != c.allowedHost {
		// The provider may only serve assets from the host we already trust via its
		// configured base_url — never bounce the fetch onto another address.
		return nil, fmt.Errorf("asset host %q not on the provider allowlist", u.Host)
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

// assetRoleFor maps a provider asset kind to a person-image core role (F24). photo
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
