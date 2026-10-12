---
key: HOLODEX-566
status: in-progress
profile: chore
depends-on: []
release_note: ""
---

# HOLODEX-566 · Migrate web imports from $lib to #lib subpath imports

SvelteKit 3 removed the built-in `$lib` alias. #484 kept it alive through the deprecated
`alias: { $lib: 'src/lib' }` option, which logs `config_option_deprecated_alias` on every build and
will be removed in a later SvelteKit release. This epic moves `web/src` to the `#lib` subpath import
declared in `web/package.json`. It is a refactor, so behaviour is unchanged.

## Gates — definition of done

## Up next — ordered (position = priority)

## Session log — newest first (cap: last 8 sessions; older → archive/)
