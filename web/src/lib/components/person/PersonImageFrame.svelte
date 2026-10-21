<script lang="ts">
	// Shared person-image frame (F25, ADR-038) backing PersonAvatar/Banner/Poster.
	// Builds the role-serving URL with the `?v=` cache-buster (there is one look, so no
	// theme parameter — the empty-slot placeholder never varies). The backend
	// always returns a usable image (real or themed placeholder), so an `img` error
	// just keeps the framed well — never a broken-image glyph. The caller supplies the
	// aspect/size via `frameClass` (e.g. "portrait-frame--1x1 w-20"), and the rendered
	// width via `sizes`, which opts the frame into the `?w=` tiers (HOLODEX-332).
	import { api } from '#lib/api.ts';
	import type { PersonImageRole, PersonImageWidth } from '#lib/types.ts';
	import { personImageSrcset } from './personImageSrcset.ts';

	let {
		personId,
		role,
		imageId,
		name,
		frameClass,
		version,
		eager = false,
		alt,
		sizes
	}: {
		personId: number;
		role: PersonImageRole;
		// One specific gallery image instead of the role's current pick (F70's compare
		// strip). `role` still types the slot for the caller; the by-id URL replaces the
		// by-role one. Everything else — cache-bust, placeholder, error — is shared.
		imageId?: number;
		name: string;
		frameClass: string;
		version?: number;
		eager?: boolean;
		// Override the alt text. Defaults to `name`; pass "" to mark the image decorative
		// (e.g. a poster shown alongside a headshot that already announces the person).
		alt?: string;
		// The frame's rendered width as an `<img sizes>` value. Omit it on a full-resolution
		// surface (the banner): no `sizes`, no `srcset`, the original only.
		sizes?: string;
	} = $props();

	const url = (width?: PersonImageWidth) =>
		imageId === undefined
			? api.personImageURL(personId, role, { version, width })
			: api.personGalleryImageURL(personId, imageId, { version, width });

	// The URLs re-derive when the person, role, image or version changes. An empty slot
	// (no version) serves the placeholder SVG, which has no tiers, so it gets no srcset.
	const src = $derived(url());
	const srcset = $derived(sizes && version ? personImageSrcset(url) : undefined);
	const altText = $derived(alt ?? name);

	let loaded = $state(false);
</script>

<div class="portrait-frame {frameClass}">
	<img
		{srcset}
		sizes={srcset ? sizes : undefined}
		{src}
		alt={altText}
		loading={eager ? 'eager' : 'lazy'}
		decoding="async"
		class={loaded ? 'is-loaded' : ''}
		onload={() => (loaded = true)}
		onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')}
	/>
</div>
