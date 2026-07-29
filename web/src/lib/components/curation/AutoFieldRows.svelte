<script lang="ts">
	// F39 (ADR-056): renders display-only auto-registered non-canonical fields as
	// read-only rows under an "Additional details" divider, shared by the video /
	// person / studio detail pages. No source chips, no curation — for owner and
	// visitor alike. Emits grid children directly (a divider spanning both columns,
	// then one row per field), so it drops straight into each page's Details `<dl>`.
	//
	// The backend already degrades a non-allowlisted image_url to text (ADR-039), so
	// the `display` here is trusted. Values are provider-sourced (auto-registered
	// fields never come from the file/record baseline), so a ProvenanceBadge always
	// shows the supplying provider.
	//
	// F44 (ADR-062): for the owner, each row also gains a trailing "Promote" pill that
	// opens the shared inline editor (PromoteFieldEditor) to make the field first-class
	// curatable. Visitors (isOwner=false) see exactly the F39 read-only rows — no pill,
	// no editor, no shape change.
	//
	// F49 (ADR-074): and a peer "Attach to…" pill, for the other answer — the key is the
	// same thing as a field that already exists, so it should feed that field rather than
	// stand as its own row. The two are a fork, not a sequence, so both pills render
	// (handoff §1). Two consequences shape this file:
	//
	//   DD5 — a successful attach DELETES the row the owner was looking at and moves its
	//         value elsewhere on the page, possibly below the fold. A row that just
	//         vanishes reads as a bug, so a confirmation strip with Undo takes its place,
	//         anchored at the row's index and held for the rest of the page session.
	//   DD7 — on rows that already span both columns (long_text / chips), the badge and
	//         pills move to their own right-aligned trailing line. Inline after a whole
	//         paragraph they drift far from the label they act on, and a second pill
	//         doubles the drift. This AMENDS F44's shipped layout — the promote pill
	//         moves too, so re-QA it here, not just the new one.
	import type { PromotionEntityType, ResolvedField } from '$lib/types';
	import { api } from '$lib/api';
	import { providerFromWinningSource, toMessage } from '$lib/format';
	import ProvenanceBadge from '../enrichment/ProvenanceBadge.svelte';
	import UrlValueList from './UrlValueList.svelte';
	import ChipValueList from './ChipValueList.svelte';
	import PromoteFieldEditor from './PromoteFieldEditor.svelte';
	import ClaimFieldEditor from './ClaimFieldEditor.svelte';

	let {
		fields,
		isOwner = false,
		entityType,
		entityNoun = '',
		onchanged
	}: {
		fields: ResolvedField[];
		isOwner?: boolean;
		entityType?: PromotionEntityType;
		entityNoun?: string;
		onchanged?: () => Promise<void> | void;
	} = $props();

	const provider = (f: ResolvedField) => providerFromWinningSource(f.winning_source);

	// A row is per key but a claim is per (provider, key) — one row can carry several
	// providers, which is what DD3's checklist bridges.
	const providersOf = (f: ResolvedField) => [
		...new Set((f.items ?? []).flatMap((v) => v.sources ?? []))
	];
	const providerValues = (f: ResolvedField) => {
		const out: Record<string, string> = {};
		for (const item of f.items ?? []) {
			for (const s of item.sources ?? []) out[s] ??= item.value;
		}
		return out;
	};

	// At most one editor is open at a time across both gestures (keyed by canonical) —
	// opening either closes the other.
	let promotingKey = $state<string | null>(null);
	let claimingKey = $state<string | null>(null);
	const canEdit = $derived(isOwner && !!entityType && !!onchanged);

	function openPromote(key: string) {
		claimingKey = null;
		promotingKey = key;
	}
	function openClaim(key: string) {
		promotingKey = null;
		claimingKey = key;
	}

	// DD5 — one confirmation strip per attached row, standing in the row's place. Anchored
	// by the index the row held, so a strip stays where its row was after the refetch
	// removes it; two rows attached in sequence keep two strips, in order (never coalesced).
	type Strip = {
		id: number;
		index: number;
		fieldKey: string;
		targetLabel: string;
		providers: string[];
		partial: boolean; // some of the row's providers stayed behind — say which attached
		undoing: boolean;
		error: string;
	};
	let strips = $state<Strip[]>([]);
	let stripSeq = 0;

	const entries = $derived.by(() => {
		const out: Array<{ field?: ResolvedField; strip?: Strip }> = fields.map((field) => ({
			field
		}));
		for (const strip of [...strips].sort((a, b) => a.index - b.index)) {
			out.splice(Math.min(strip.index, out.length), 0, { strip });
		}
		return out;
	});

	function recordStrip(f: ResolvedField, info: { targetLabel: string; providers: string[] }) {
		strips = [
			...strips,
			{
				id: ++stripSeq,
				index: fields.indexOf(f),
				fieldKey: f.canonical,
				targetLabel: info.targetLabel,
				providers: info.providers,
				partial: info.providers.length < providersOf(f).length,
				undoing: false,
				error: ''
			}
		];
	}

	// Matched by id, never by reference: every update replaces the object, so the strip a
	// handler closed over stops being the one in the array after the first patch.
	function update(id: number, patch: Partial<Strip>) {
		strips = strips.map((s) => (s.id === id ? { ...s, ...patch } : s));
	}

	// Undo issues the DELETE for every claim the action wrote, then refetches — the row
	// comes back the same way it went away.
	async function undo(strip: Strip) {
		if (strip.undoing) return;
		update(strip.id, { undoing: true, error: '' });
		try {
			for (const p of strip.providers) {
				await api.unclaimField(entityType!, p, strip.fieldKey);
			}
		} catch (e) {
			update(strip.id, { undoing: false, error: toMessage(e) });
			return;
		}
		strips = strips.filter((s) => s.id !== strip.id);
		await onchanged!();
	}

	const pillClass =
		'inline-flex items-center gap-1 rounded-full border border-rule px-2 py-0.5 text-xs text-muted hover:border-accent hover:text-accent focus-visible:text-accent';
</script>

<!-- The provenance badge and the two owner pills. `inline` is the F44 placement (a leading
     margin separates them from the value); the DD7 trailing line supplies its own gap. -->
{#snippet controls(f: ResolvedField, inline: boolean)}
	{#if provider(f) && f.display !== 'url'}
		<ProvenanceBadge provider={provider(f)} label={provider(f)} />
	{/if}
	{#if canEdit && promotingKey !== f.canonical && claimingKey !== f.canonical}
		<button
			type="button"
			onclick={() => openClaim(f.canonical)}
			aria-label={`Attach ${f.label} to another field`}
			class={inline ? `ml-1 ${pillClass}` : pillClass}
		>
			<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M13.8 10.2a4 4 0 010 5.7l-2.8 2.8a4 4 0 01-5.7-5.7l1.4-1.4m3.5-.1a4 4 0 010-5.7l2.8-2.8a4 4 0 015.7 5.7l-1.4 1.4"/></svg>
			Attach to…
		</button>
		<button
			type="button"
			onclick={() => openPromote(f.canonical)}
			aria-label={`Promote ${f.label}`}
			class={inline ? `ml-1 ${pillClass}` : pillClass}
		>
			<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 19V5m0 0l-6 6m6-6l6 6"/></svg>
			Promote
		</button>
	{/if}
{/snippet}

{#if fields.length || strips.length}
	<div class="mt-1 border-t border-rule pt-3 sm:col-span-2">
		<p class="text-xs text-muted">Additional details</p>
	</div>
	{#each entries as e (e.strip ? `s${e.strip.id}` : `f${e.field!.canonical}`)}
		{#if e.strip}
			{@const s = e.strip}
			<div
				class="mt-1 flex flex-wrap items-center gap-2 rounded-theme border border-dashed border-rule bg-surface-2 px-3 py-2 text-xs text-muted sm:col-span-2"
				class:opacity-60={s.undoing}
				aria-live="polite"
			>
				<svg
					class="h-3 w-3 shrink-0 text-accent"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					aria-hidden="true"
				>
					<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
				</svg>
				<span>
					“{s.fieldKey}”{#if s.partial}&nbsp;from {s.providers.join(', ')}{/if} attached to {s.targetLabel}.
				</span>
				{#if s.error}
					<span class="text-warn">{s.error}</span>
				{/if}
				<button
					type="button"
					onclick={() => undo(s)}
					disabled={s.undoing}
					class="btn-accent ml-auto px-2 py-0.5"
				>
					Undo
				</button>
			</div>
		{:else}
			{@const f = e.field!}
			{@const wide = f.display === 'long_text' || f.display === 'chips'}
			<div class={wide ? 'sm:col-span-2' : ''}>
				<dt class="inline text-muted">{f.label}:</dt>
				{#if f.display === 'long_text'}
					<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
				{:else if f.display === 'image_url'}
					<dd class="mt-1 block">
						<img
							src={f.values[0]}
							alt={f.label}
							class="max-h-64 rounded-theme border border-rule"
						/>
					</dd>
				{:else if f.display === 'chips'}
					<dd class="mt-1 block"><ChipValueList values={f.values} /></dd>
				{:else if f.display === 'url'}
					<!-- HOLODEX-137: provider icon + host in the link folds in provenance. -->
					<dd class="inline"><UrlValueList values={f.values} provider={provider(f)} /></dd>
				{:else}
					<dd class="inline text-ink">{f.values.join(', ')}</dd>
				{/if}
				<!-- DD7: on a both-column row the badge and pills get their own trailing line,
				     right-aligned so they read as chrome rather than as more prose; wrapping is
				     deliberate, so a narrow viewport stacks them instead of compressing them.
				     Every other display keeps F44's inline placement byte-for-byte. -->
				{#if wide}
					<div class="mt-1 flex flex-wrap items-center justify-end gap-2">
						{@render controls(f, false)}
					</div>
				{:else}
					{@render controls(f, true)}
				{/if}
			</div>
			{#if canEdit && promotingKey === f.canonical}
				<PromoteFieldEditor
					entityType={entityType!}
					fieldKey={f.canonical}
					mode="promote"
					inheritedLabel={f.label}
					render={f.display ?? ''}
					{entityNoun}
					onchanged={onchanged!}
					onclose={() => (promotingKey = null)}
				/>
			{/if}
			{#if canEdit && claimingKey === f.canonical}
				<ClaimFieldEditor
					entityType={entityType!}
					fieldKey={f.canonical}
					label={f.label}
					providers={providersOf(f)}
					providerValues={providerValues(f)}
					{entityNoun}
					onclaimed={(info) => recordStrip(f, info)}
					onchanged={onchanged!}
					onclose={() => (claimingKey = null)}
				/>
			{/if}
		{/if}
	{/each}
{/if}
