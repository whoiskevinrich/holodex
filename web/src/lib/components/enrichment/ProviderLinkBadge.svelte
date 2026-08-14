<script lang="ts">
	// Provider-attested outbound link badge (HOLODEX-266, ADR-083 D2/D3): one pill per
	// stored external id on a person/studio (video later) — a clickable provider-name
	// link when the provider declared a link_templates entry for this (namespace,
	// entity kind), else a non-interactive "known to" pill (the ADR-083 D2 degraded
	// state: a missing template isn't an error, so the identity signal still renders).
	// `link.provider` is the id's NAMESPACE (e.g. "imdb"), which can differ from the
	// enrichment provider that emitted it (e.g. TMDB emitting "imdb:"-namespaced ids) —
	// so the brand icon lookup below may fall back to a monogram even when a real
	// provider enriched this entity, which is expected, not a bug. See
	// docs/design/provider-link-badge-handoff.md for the full state/a11y spec.
	import type { ExternalLink } from '$lib/types';
	import { isHttpUrl } from '$lib/format';
	import { providers } from '$lib/providers.svelte';
	import ProviderIcon from './ProviderIcon.svelte';

	let { link, entityName }: { link: ExternalLink; entityName: string } = $props();

	// Defense in depth: the backend only ever builds this URL from a validated,
	// http(s)-only template (enrich.ValidateLinkTemplate), but Svelte doesn't sanitize
	// `href` — so gate on scheme here too, the same rule EnrichPicker's profile_url
	// link applies.
	const linked = $derived(!!link.url && isHttpUrl(link.url));

	// Shared provider directory (ADR-059) for the badge's brand icon; monogram fallback
	// until it resolves or when this namespace has no self-hosted icon of its own.
	$effect(() => void providers.load());

	const baseClass =
		'inline-flex items-center gap-1 rounded-full border border-rule px-2 py-0.5 text-xs text-muted';
	const interactiveClass =
		'hover:border-accent hover:text-ink focus-visible:border-accent focus-visible:text-ink focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent';
</script>

<svelte:element
	this={linked ? 'a' : 'span'}
	href={linked ? link.url : undefined}
	target={linked ? '_blank' : undefined}
	rel={linked ? 'noopener noreferrer' : undefined}
	aria-label={linked
		? `View ${entityName} on ${link.label}'s site (opens in a new tab)`
		: `Known to ${link.label}`}
	class="{baseClass} {linked ? interactiveClass : ''}"
>
	<ProviderIcon name={link.provider} iconUrl={providers.iconUrl(link.provider)} size={16} decorative />
	<span>{link.label}</span>
</svelte:element>
