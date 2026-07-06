<script lang="ts">
	// "Keep which name?" — step two of a multi-select merge (F43, ADR-061): given 2+ selected
	// entities, choose which one survives; the rest fold into it (their videos move, their names
	// become aliases). Shared by /people and /tags (HOLODEX-163 — was two verbatim copies).
	// Entity-generic via `kind`; api.mergeEntities does the fold. The parent owns selection: it
	// passes the chosen `items` and reacts to `onmerged` (reload + clear) / `onclose` (dismiss).
	// Modal a11y — focus trap, Esc, backdrop, focus-return — matches its sibling EntityPicker.
	// Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage, videoCount } from '$lib/format';
	import type { EntityKind, EntityRef } from '$lib/types';

	let {
		kind,
		items,
		onclose,
		onmerged
	}: {
		kind: EntityKind;
		items: EntityRef[];
		onclose: () => void;
		onmerged: () => void;
	} = $props();

	// Per-entity noun for the confirm copy — the only textual delta across the three.
	const NOUNS: Record<EntityKind, string> = { person: 'person', studio: 'studio', tag: 'tag' };
	const noun = $derived(NOUNS[kind]);

	// The survivor defaults to the first selected until the owner picks another via the radios.
	let canonicalId = $state<number | null>(null);
	const canonical = $derived(canonicalId ?? items[0]?.id ?? null);
	let merging = $state(false);
	let mergeError = $state('');

	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null;
		dialogEl?.querySelector<HTMLElement>('input[type="radio"]:checked')?.focus();
		return () => trigger?.focus?.();
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const f = [...dialogEl.querySelectorAll<HTMLElement>('input, button, [tabindex="0"]')].filter(
			(el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null
		);
		if (f.length === 0) return;
		const first = f[0];
		const last = f[f.length - 1];
		if (e.shiftKey && document.activeElement === first) {
			e.preventDefault();
			last.focus();
		} else if (!e.shiftKey && document.activeElement === last) {
			e.preventDefault();
			first.focus();
		}
	}

	function cancel() {
		if (!merging) onclose();
	}

	async function confirmMerge() {
		if (canonical == null || merging) return;
		merging = true;
		mergeError = '';
		try {
			// Fold every other selected entity into the chosen survivor.
			for (const from of items.filter((e) => e.id !== canonical)) {
				await api.mergeEntities(kind, canonical, from.id);
			}
			onmerged();
			onclose();
		} catch (e) {
			mergeError = toMessage(e);
		} finally {
			merging = false;
		}
	}
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[10vh]"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) cancel();
	}}
>
	<div
		bind:this={dialogEl}
		onkeydown={trapTab}
		tabindex="-1"
		class="merge-pop flex w-full max-w-lg flex-col gap-3 rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="merge-canonical-title"
	>
		<h2 id="merge-canonical-title" class="skin-title text-lg font-semibold text-ink">
			Keep which name?
		</h2>
		<p class="text-xs text-muted">
			The chosen name stays; the others become its aliases and their videos move under it. Confirm
			these are the same {noun} — this can’t be auto-undone.
		</p>
		<fieldset class="space-y-1">
			{#each items as e (e.id)}
				<label class="flex cursor-pointer items-center gap-3 rounded-theme px-2 py-1.5 text-ink hover:bg-surface-2">
					<input type="radio" name="merge-canonical" class="accent-accent" value={e.id} checked={canonical === e.id} onchange={() => (canonicalId = e.id)} />
					<span class="flex-1 truncate">{e.name}</span>
					<span class="text-xs text-muted">{videoCount(e.video_count ?? 0)}</span>
				</label>
			{/each}
		</fieldset>
		{#if mergeError}
			<p class="text-sm text-warn">{mergeError}</p>
		{/if}
		<div class="flex flex-wrap items-center justify-end gap-2">
			<button
				onclick={cancel}
				disabled={merging}
				class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
			>
				Back
			</button>
			<button
				onclick={confirmMerge}
				disabled={merging || canonical == null}
				class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
			>
				{merging ? 'Merging…' : 'Merge'}
			</button>
		</div>
	</div>
</div>

<svelte:window onkeydown={(e) => e.key === 'Escape' && cancel()} />

<style>
	@media (prefers-reduced-motion: no-preference) {
		.merge-pop {
			animation: merge-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes merge-rise {
		from {
			opacity: 0;
			transform: scale(0.98);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
</style>
