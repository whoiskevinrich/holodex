<script lang="ts">
	// PersonLinkChip (F68, HOLODEX-431): the one person-link component, and the only
	// mount point of PersonHoverCard. A TRANSPARENT wrapper: the consumer keeps its
	// own <a> (classes, title, handlers, roving tabindex — the search rows need all
	// of it, and a surface's pill shape can carry meaning, like the film page's
	// dashed "billed, in no scene you own" chip) and renders it as this component's
	// children; the chip finds that link in the DOM and adds the hover card beside it:
	//   · opens on hover-intent (250 ms) or immediately on keyboard focus;
	//   · stays open across the gap and the card (150 ms leave grace);
	//   · Escape / outside click close it (use:dismissable), one card app-wide;
	//   · one measure on open flips it above / clamps it inside the viewport (RD10);
	//   · never mounts under a coarse pointer — the link is the complete fallback.
	// The card's data is fetched on intent (aborted on leave) and cached per session.
	import type { Snippet } from 'svelte';
	import { onDestroy, tick } from 'svelte';
	import type { PersonCard } from '$lib/types';
	import { dismissable } from '$lib/actions/dismissable';
	import PersonHoverCard from './PersonHoverCard.svelte';
	import {
		cannotHover,
		CLOSE_GRACE_MS,
		hoverCard,
		loadPersonCard,
		OPEN_DELAY_MS,
		placeCard,
		type CardLease,
		type Placement
	} from './personCard.svelte';

	let {
		id,
		name,
		wrapClass = 'relative inline-block',
		children
	}: {
		id: number;
		/** Drawn on the card while its payload loads. */
		name: string;
		/** `relative inline-block` for an inline chip; a block consumer passes `relative block`. */
		wrapClass?: string;
		/** The consumer's own `<a href="/people/{id}">` — the trigger. */
		children: Snippet;
	} = $props();

	// A per-mount token keys the app-wide "one open card" latch; the DOM id ties
	// the link's aria-describedby to its card.
	const token = Symbol('person-chip');
	const mountId = nextMountId();
	const cardId = $derived(`person-card-${id}-${mountId}`);
	const noHover = cannotHover();

	const open = $derived(hoverCard.openToken === token);
	let card = $state<PersonCard | null>(null);
	let placement = $state<Placement>({ above: false, shiftX: 0 });
	let wrapEl = $state<HTMLSpanElement>();
	let cardEl = $state<HTMLDivElement>();
	let openTimer: ReturnType<typeof setTimeout> | undefined;
	let closeTimer: ReturnType<typeof setTimeout> | undefined;
	let inflight: CardLease | null = null;

	// A chip that unmounts mid-hover (navigation) must not fire a timer or hold a
	// flight after it is gone.
	onDestroy(() => {
		clearTimers();
		inflight?.release();
		inflight = null;
	});

	const linkEl = () => wrapEl?.querySelector('a') ?? null;

	// aria-describedby lives on the consumer's link, so it is set from here rather
	// than in markup; removed again on close.
	$effect(() => {
		const a = linkEl();
		if (!a) return;
		if (open) a.setAttribute('aria-describedby', cardId);
		else a.removeAttribute('aria-describedby');
	});

	function clearTimers() {
		clearTimeout(openTimer);
		clearTimeout(closeTimer);
		openTimer = closeTimer = undefined;
	}

	async function openNow() {
		clearTimers();
		if (open) return;
		hoverCard.openToken = token;
		fetchCard();
		await tick();
		measure();
	}

	// `replace` re-fetches over a card already shown (after the ring refreshed the
	// person): the stale card stays mounted until the fresh one lands, so nothing —
	// including the ring the owner just pressed, and its focus — unmounts meanwhile.
	function fetchCard(replace = false) {
		if ((card && !replace) || inflight) return;
		const mine = loadPersonCard(id);
		inflight = mine;
		mine.promise
			.then(async (c) => {
				if (inflight !== mine) return;
				card = c;
				await tick();
				measure();
			})
			.catch(() => {
				// Quiet: a failed or released fetch closes nothing loudly. With no card to
				// show the chip stays a plain link and the next hover retries (the cache
				// dropped the entry); with a stale card it simply keeps it.
				if (inflight === mine && !card && hoverCard.openToken === token) hoverCard.openToken = null;
			})
			.finally(() => {
				if (inflight === mine) inflight = null;
			});
	}

	function close(returnFocus = false) {
		clearTimers();
		inflight?.release();
		inflight = null;
		if (hoverCard.openToken === token) hoverCard.openToken = null;
		if (returnFocus) linkEl()?.focus();
	}

	function scheduleOpen() {
		if (noHover) return;
		clearTimeout(closeTimer);
		closeTimer = undefined;
		if (open || openTimer) return;
		openTimer = setTimeout(() => void openNow(), OPEN_DELAY_MS);
	}

	function scheduleClose() {
		clearTimeout(openTimer);
		openTimer = undefined;
		if (!open) {
			inflight?.release();
			inflight = null;
			return;
		}
		closeTimer = setTimeout(() => close(), CLOSE_GRACE_MS);
	}

	function onFocusIn(e: FocusEvent) {
		if (noHover) return;
		clearTimeout(closeTimer);
		closeTimer = undefined;
		// The link itself takes focus → open at once, no intent delay (R2). Focus
		// arriving inside the card (Tab into its links) just keeps it open.
		if (e.target === linkEl()) void openNow();
	}

	function onFocusOut(e: FocusEvent) {
		const next = e.relatedTarget;
		if (next instanceof Node && wrapEl?.contains(next)) return;
		close();
	}

	function measure() {
		const a = linkEl();
		if (!open || !a || !cardEl) return;
		placement = placeCard(
			a.getBoundingClientRect(),
			cardEl.offsetWidth,
			cardEl.offsetHeight,
			window.innerWidth,
			window.innerHeight
		);
	}

	// The ring refreshed the person (F65.8): re-fetch over the card in place.
	function onRefreshed() {
		fetchCard(true);
	}
</script>

<script module lang="ts">
	let mountSeq = 0;
	function nextMountId(): number {
		return ++mountSeq;
	}
</script>

<svelte:window onresize={measure} onscroll={measure} />

<!-- The wrapper is presentational: the consumer's link inside is the interactive
     element; the pointer handlers here only run the hover-intent timers. -->
<span
	bind:this={wrapEl}
	role="presentation"
	class={wrapClass}
	data-person-chip={cardId}
	onpointerenter={scheduleOpen}
	onpointerleave={scheduleClose}
	onfocusin={onFocusIn}
	onfocusout={onFocusOut}
	use:dismissable={{ enabled: open, inside: `[data-person-chip="${cardId}"]`, onclose: (esc) => close(esc) }}
>
	{@render children()}
	{#if open}
		<div
			bind:this={cardEl}
			class="person-hover-card-motion absolute left-0 z-50 {placement.above ? 'bottom-full mb-1.5' : 'top-full mt-1.5'}"
			style:transform={placement.shiftX ? `translateX(${placement.shiftX}px)` : undefined}
		>
			<PersonHoverCard {id} {name} {card} {cardId} onrefreshed={onRefreshed} />
		</div>
	{/if}
</span>
