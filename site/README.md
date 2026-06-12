# Holodex landing page

A single self-contained marketing page (`index.html`) — no build step, no framework. Its
hero swaps the **real** product screenshots (grid + detail) between the three skins, tinted
with each skin's actual accent, mirroring the in-app token swap ([ADR-021](../docs/architecture/ADR-021-frontend-theming-and-skins.md)).

Screenshots in `screenshots/` are captured from the [demo corpus](../docs/specs/showcase-demo-corpus.md)
(copies of `docs/assets/screenshots/`).

## Preview locally

`file://` won't load the relative images, so serve the folder over HTTP:

```bash
npx --yes serve site          # or: python -m http.server -d site 4321
# -> http://localhost:4321
```

## Deploy

Deployed to **GitHub Pages** at the custom domain **`holodex.whoiskevinrich.com`** by
[`.github/workflows/pages.yml`](../.github/workflows/pages.yml), which uploads `site/` as-is on
every push to `main` that touches it. The `CNAME` file pins the custom domain.

Two one-time setup steps (outside this repo):

1. **Repo setting** — Settings → Pages → *Build and deployment* → **Source: GitHub Actions**.
2. **Cloudflare DNS** (matches the two-tier DNS strategy — proxy **off** so GitHub provisions TLS):

   | Type | Name | Target | Proxy |
   |------|------|--------|-------|
   | CNAME | `holodex` | `whoiskevinrich.github.io` | **Off** (grey cloud) |

   After the first deploy, enable **Enforce HTTPS** in Settings → Pages (once GitHub finishes
   issuing the certificate, usually a few minutes).

The page is host-agnostic otherwise (relative asset paths), so it also runs from any static
host or `npx serve site` locally.
