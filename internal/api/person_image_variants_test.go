package api_test

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"strings"
	"testing"
)

// getImage fetches url and decodes the body, returning the response headers too.
func getImage(t *testing.T, url string) (image.Config, http.Header) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get %s = %d", url, resp.StatusCode)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return cfg, resp.Header
}

// TestServePersonImageWidthVariants (HOLODEX-332): both serve routes honour ?w= —
// a tier serves that width with the original's immutable headers, a non-tier snaps
// up, and an out-of-range or junk w serves the original.
func TestServePersonImageWidthVariants(t *testing.T) {
	srv, _, pid := personImageServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid)

	code, out := uploadImage(t, base+"/image", "", "headshot", pngUpload(t, 400, 600))
	if code != http.StatusCreated {
		t.Fatalf("upload code = %d body=%v", code, out)
	}
	imgID := itoa(int64(out["id"].(float64)))

	for _, route := range []string{base + "/image/headshot", base + "/images/" + imgID} {
		cases := []struct {
			query string
			w, h  int
		}{
			{"?w=160", 160, 240},
			{"?w=200", 320, 480}, // snaps up
			{"?w=641", 400, 600}, // above the largest tier → original
			{"?w=0", 400, 600},
			{"?w=-5", 400, 600},
			{"?w=wide", 400, 600},
			{"", 400, 600},
			{"?w=640", 400, 600}, // the original is narrower: no upscale
		}
		for _, c := range cases {
			cfg, hdr := getImage(t, route+c.query)
			if cfg.Width != c.w || cfg.Height != c.h {
				t.Errorf("%s%s = %dx%d, want %dx%d", route, c.query, cfg.Width, cfg.Height, c.w, c.h)
			}
			if cc := hdr.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
				t.Errorf("%s%s cache-control = %q", route, c.query, cc)
			}
			if ct := hdr.Get("Content-Type"); ct != "image/jpeg" {
				t.Errorf("%s%s content-type = %q", route, c.query, ct)
			}
			if hdr.Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("%s%s missing nosniff", route, c.query)
			}
		}
	}
}

// TestServePersonImagePlaceholderIgnoresWidth: an empty role's placeholder SVG is the
// same with or without w.
func TestServePersonImagePlaceholderIgnoresWidth(t *testing.T) {
	srv, _, pid := personImageServer(t, "")
	base := srv.URL + "/api/v1/people/" + itoa(pid) + "/image/headshot"
	fetch := func(url string) (string, []byte) {
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("get %s: %v", url, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get %s = %d", url, resp.StatusCode)
		}
		return resp.Header.Get("Content-Type"), body
	}
	_, plain := fetch(base)
	ct, sized := fetch(base + "?w=160")
	if !strings.HasPrefix(ct, "image/svg") || !bytes.Equal(plain, sized) {
		t.Errorf("placeholder ?w=160: content-type %q, same bytes %v; want the plain SVG", ct, bytes.Equal(plain, sized))
	}
}
