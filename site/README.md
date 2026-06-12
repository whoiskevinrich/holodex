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

The page is static and host-agnostic (relative asset paths), so any static host works:

- **GitHub Pages** — publish the `site/` directory (a Pages Action that uploads `site/` as
  the artifact, or push it to a `gh-pages` branch). Then optionally point a custom subdomain
  at it with a `CNAME`.
- **Cloudflare Pages / any static host** — upload `site/` as the site root.

> Wiring up a deploy touches CI/infra — run `/security-review` and keep the distribution ADRs
> in lockstep before merging a workflow.
