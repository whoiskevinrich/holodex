<script lang="ts">
	// Banner hero (F25, ADR-038) — a thin wrapper over PersonImageFrame fixing the banner
	// role to a wide 5:2 band capped at 540px tall (cover-cropped), so the hero reads as a
	// proper backdrop without running off the page on wide viewports. The aspect ratio,
	// max height, and the image overhang live on `.portrait-frame--banner` in app.css; the
	// 5:2 ratio is shared with the crop editor's banner frame (.crop-frame--banner) so what
	// you crop matches what's shown.
	import PersonImageFrame from './PersonImageFrame.svelte';

	let {
		personId,
		name,
		version,
		eager = false
	}: {
		personId: number;
		name: string;
		version?: number;
		eager?: boolean;
	} = $props();

	// Parallax: translate the (overhanging) banner image opposite to page scroll as the
	// hero passes the viewport, for depth. Driven from a passive, rAF-throttled scroll
	// listener that sets `--banner-shift` on the wrapper (inherited by the image in CSS).
	// A pure-CSS scroll-driven timeline can't do this — the image's nearest scroll
	// container is its own overflow:hidden frame, so a `view()` timeline is inert. No-op
	// under reduced-motion (CSS also drops the transform there). The shift stays within
	// the image's overhang so the frame is always fully covered.
	let wrap = $state<HTMLElement | null>(null);

	$effect(() => {
		const el = wrap;
		if (!el || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

		let raf = 0;
		const update = () => {
			raf = 0;
			const r = el.getBoundingClientRect();
			// 0 as the band enters from the bottom → 1 as it leaves past the top.
			const progress = (window.innerHeight - r.top) / (window.innerHeight + r.height);
			const clamped = Math.min(1, Math.max(0, progress));
			el.style.setProperty('--banner-shift', `${(-4 - clamped * 20).toFixed(2)}%`);
		};
		const onScroll = () => {
			if (!raf) raf = requestAnimationFrame(update);
		};

		update(); // seed the initial position
		window.addEventListener('scroll', onScroll, { passive: true });
		window.addEventListener('resize', onScroll, { passive: true });
		return () => {
			window.removeEventListener('scroll', onScroll);
			window.removeEventListener('resize', onScroll);
			cancelAnimationFrame(raf);
		};
	});
</script>

<div bind:this={wrap}>
	<PersonImageFrame {personId} role="banner" {name} {version} {eager} frameClass="portrait-frame--banner w-full" />
</div>
