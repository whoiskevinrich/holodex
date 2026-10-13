// How the current page was reached (HOLODEX-459): SvelteKit's navigation type, recorded by
// the root layout's afterNavigate. A component that mounts only after its page's own fetch
// lands (EntityVideos, behind AsyncState) is too late for its own afterNavigate to fire, so
// it reads this instead.
export const lastNavigation = { type: 'enter' as string };
