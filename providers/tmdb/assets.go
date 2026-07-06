package main

import _ "embed"

// brandIconPNG is the TMDB brand mark (HOLODEX-161), served at GET /brand-icon.png and
// advertised as the provider's `brand_icon` (Holodex contract §4.8, ADR-059). It is a
// raster (PNG) because Holodex's image ingest rejects SVG; TMDB publishes the logo as
// SVG, so this is a rasterized copy of the official "primary" mark from
// https://www.themoviedb.org/about/logos-attribution, on a white background. Bundling +
// self-serving it (rather than hotlinking) keeps the icon on the sidecar's own host —
// always Holodex's allowlisted base host — so no operator asset_hosts change is needed.
//
//go:embed assets/tmdb-brand.png
var brandIconPNG []byte
