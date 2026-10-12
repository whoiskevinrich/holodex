---
key: HOLODEX-566
status: in-progress
profile: chore
depends-on: []
release_note: "No visible change. The web app now builds without a deprecated SvelteKit option, so a future SvelteKit upgrade won't break it."
---

# HOLODEX-566 · Migrate web imports from $lib to #lib subpath imports

SvelteKit 3 removed the built-in `$lib` alias. #484 kept it alive through the deprecated
`alias: { $lib: 'src/lib' }` option, which logs `config_option_deprecated_alias` on every build and
will be removed in a later SvelteKit release. This epic moves `web/src` to the `#lib` subpath import
declared in `web/package.json`. It is a refactor, so behaviour is unchanged.

## Gates — definition of done

Posture `chore` has no gates. This is a mechanical refactor with no behaviour, spec, design or
security surface. It was verified by a clean `npm run check` (0 errors, no
`config_option_deprecated_alias`), `npm test` (617/617) and `npm run build`.

## Up next — ordered (position = priority)

1. [ ] [—] Squash-merge the PR on green CI with subject `refactor(web): import from #lib subpath imports instead of the $lib alias`, then confirm CI moved HOLODEX-566 to Done

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-12 · migrate $lib → #lib
- skills: code-review high --fix, handoff
- handoff: `web/src` imports now go through `#lib/*` in `web/package.json` and the deprecated `alias` option is gone. Extensionless `.ts` modules gained an explicit `.ts`, because subpath imports don't probe extensions. Check, tests and build pass from a clean `.svelte-kit`. Older `$lib/...` mentions in design handoffs, worklogs and archived ADRs were left as history. Next: merge the PR on green CI.
