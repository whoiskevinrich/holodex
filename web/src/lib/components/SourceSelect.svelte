<script lang="ts">
	// Per-field source-of-truth control (F36, ADR-051). Owner-only, replace-field only.
	// Three stacked rows in the Metadata <dl> cell:
	//   row 1 — the resolved value as a read-only CurationChip (·source) + an out-of-sync pill
	//           when the decided value differs from the file's embedded tag (the lone text-warn);
	//   row 2 — the segmented `SourceSelect` radiogroup: Keep file · Adopt {provider}… · Custom,
	//           with a roving tabindex (one tab stop) and native radio arrow semantics;
	//   row 3 — the muted candidates line (file + each provider value), with an informational
	//           "providers differ" hint when ≥2 providers disagree (never warn — Open-Q3).
	// Changing a source is a DB-only decision (RD5): it calls `decide`, never a file write. The
	// file is touched only by the header "Write decisions to file" action. Tokens only; QA 3 skins.
	import type { DecisionSource, ResolvedField, ResolvedValue } from '$lib/types';
	import {
		decidedSource,
		fileCandidateValue,
		outOfSync,
		providerCandidates,
		providerOf,
		providersDiffer
	} from '$lib/f36';
	import { toMessage } from '$lib/format';
	import CurationChip from './CurationChip.svelte';

	let {
		field,
		decide
	}: {
		field: ResolvedField;
		// Persist a decision and refetch the detail (file ⇒ clear to default; manual ⇒ literal).
		// Owned by the page so this control stays free of api/transport concerns (cf. EnrichPicker).
		decide: (source: DecisionSource, manualValue?: string) => Promise<void>;
	} = $props();

	interface Segment {
		key: string; // 'file' | 'provider:<name>' | 'custom' — also the data-seg / DOM id key
		label: string;
		source: DecisionSource;
		value: string;
		aria: string;
	}

	const fileVal = $derived(fileCandidateValue(field));
	const provCands = $derived(providerCandidates(field));
	const current = $derived(decidedSource(field)); // 'file' | 'provider:<name>' | 'manual'
	// committedKey is the server's stored decision as a segment key (a manual decision maps to
	// the Custom segment). pendingKey is an optimistic override while a selection is settling, so
	// aria-checked tracks the arrow/click immediately (QA 3.14) before the debounced commit +
	// refetch land. The effect below clears it once the server has caught up.
	const committedKey = $derived(current === 'manual' ? 'custom' : current);
	let pendingKey = $state<string | null>(null);
	const selectedKey = $derived(pendingKey ?? committedKey);

	const segments = $derived<Segment[]>([
		{
			key: 'file',
			label: 'Keep file',
			source: 'file',
			value: fileVal,
			aria: fileVal ? `Keep file value "${fileVal}"` : 'Keep file'
		},
		...provCands.map((c) => {
			const name = c.provider || providerOf(c.source);
			return {
				key: c.source,
				label: `Adopt ${name}`,
				source: c.source as DecisionSource,
				value: c.value,
				aria: `Adopt ${name} value "${c.value}"`
			};
		}),
		{
			key: 'custom',
			label: 'Custom',
			source: 'manual' as DecisionSource,
			value: field.decision?.manual_value ?? '',
			aria: 'Custom value'
		}
	]);

	// The resolved value rendered as a read-only chip: file/manual read muted, a provider reads
	// accent — CurationChip derives that from sources/manual, so we hand it the right provenance.
	const chipItem = $derived<ResolvedValue>({
		value: field.values[0] ?? '',
		sources:
			current === 'manual' ? ['manual'] : current === 'file' ? ['file'] : [providerOf(current)],
		manual: current === 'manual'
	});

	let busy = $state(false);
	let error = $state('');
	let editing = $state(false); // Custom inline input open
	let draft = $state('');
	let groupEl = $state<HTMLElement | null>(null);
	let commitTimer: ReturnType<typeof setTimeout> | undefined;

	// Drop the optimistic override once the server's committed decision matches it (after the
	// commit + refetch). Idempotent, so a fresh selection mid-flight is never clobbered.
	$effect(() => {
		if (pendingKey !== null && pendingKey === committedKey) pendingKey = null;
	});

	function commitDecision(source: DecisionSource, manualValue?: string) {
		clearTimeout(commitTimer);
		busy = true;
		error = '';
		decide(source, manualValue)
			.catch((e) => {
				error = toMessage(e);
				pendingKey = null; // revert the optimistic selection on failure
			})
			.finally(() => {
				busy = false;
			});
	}

	function focusSeg(key: string) {
		groupEl?.querySelector<HTMLElement>(`[data-seg="${key}"]`)?.focus();
	}

	// Activate a segment now (click / Space / Enter): Custom opens the inline input (its commit
	// decides); a file/provider segment commits immediately. Re-selecting the current source is a
	// no-op. Focus already sits on the activated control, so this never moves focus.
	function activate(seg: Segment) {
		clearTimeout(commitTimer);
		if (seg.key === 'custom') {
			pendingKey = 'custom';
			startCustom();
			return;
		}
		if (seg.key === committedKey) {
			pendingKey = null;
			return;
		}
		pendingKey = seg.key;
		commitDecision(seg.source);
	}

	// Roving radiogroup arrow keys move focus + selection optimistically (native radio semantics,
	// QA 3.14), but the network commit is debounced — arrowing across segments issues ONE decision
	// for the settled segment, not one per keypress (cf. EnrichPicker's debounce). Space/Enter
	// commit the focused segment immediately.
	function onSegKey(e: KeyboardEvent, i: number) {
		const n = segments.length;
		const delta =
			e.key === 'ArrowRight' || e.key === 'ArrowDown'
				? 1
				: e.key === 'ArrowLeft' || e.key === 'ArrowUp'
					? -1
					: 0;
		if (delta) {
			e.preventDefault();
			const target = segments[(i + delta + n) % n];
			pendingKey = target.key; // selection follows focus immediately
			focusSeg(target.key);
			clearTimeout(commitTimer);
			if (target.key === 'custom') {
				startCustom();
			} else {
				commitTimer = setTimeout(() => {
					if (target.key !== committedKey) commitDecision(target.source);
					else pendingKey = null;
				}, 250);
			}
		} else if (e.key === ' ' || e.key === 'Enter') {
			e.preventDefault();
			activate(segments[i]);
		}
	}

	function startCustom() {
		draft = field.decision?.manual_value ?? field.values[0] ?? '';
		editing = true;
	}
	function commitCustom() {
		const v = draft.trim();
		editing = false;
		if (v) commitDecision('manual', v);
		else cancelCustom();
	}
	function cancelCustom() {
		editing = false;
		pendingKey = null; // drop the optimistic 'custom' selection — no decision was made
		focusSeg('custom'); // Escape/empty returns focus to the Custom segment (handoff a11y)
	}
	function onCustomKey(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			commitCustom();
		} else if (e.key === 'Escape') {
			e.preventDefault();
			cancelCustom();
		}
	}

	const showCandidates = $derived(provCands.length >= 1);
</script>

<div class="mt-1 space-y-1" class:opacity-60={busy} aria-busy={busy}>
	<!-- Row 1 — resolved value chip + out-of-sync pill (the single text-warn signal). -->
	<div class="flex flex-wrap items-center gap-2">
		<!-- Read-only resolved chip: isOwner={false} hides the F30 edit/remove/no-write
		     controls (the Custom segment is the edit affordance here). -->
		<CurationChip item={chipItem} isOwner={false} />
		{#if outOfSync(field)}
			<span
				class="inline-block rounded-full border border-warn px-2 py-0.5 text-[0.65rem] text-warn"
				aria-label={`${field.label} is out of sync with the file`}
			>
				file out of sync
			</span>
		{/if}
	</div>

	<!-- Row 2 — the segmented SourceSelect radiogroup. -->
	<div
		bind:this={groupEl}
		role="radiogroup"
		aria-label={`Source of truth for ${field.label}`}
		class="inline-flex flex-wrap items-stretch overflow-hidden rounded-theme border border-rule bg-surface-2 text-xs"
	>
		{#each segments as seg, i (seg.key)}
			{#if seg.key === 'custom' && editing}
				<!-- svelte-ignore a11y_autofocus -->
				<input
					bind:value={draft}
					onkeydown={onCustomKey}
					onblur={commitCustom}
					autofocus
					aria-label={`Custom value for ${field.label}`}
					placeholder="Custom value…"
					class="w-32 border-l border-rule bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent"
				/>
			{:else}
				<button
					type="button"
					role="radio"
					data-seg={seg.key}
					aria-checked={seg.key === selectedKey}
					aria-label={seg.aria}
					tabindex={seg.key === selectedKey ? 0 : -1}
					onclick={() => activate(seg)}
					onkeydown={(e) => onSegKey(e, i)}
					class="border-l border-rule px-2 py-0.5 first:border-l-0 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent {seg.key ===
					selectedKey
						? 'bg-accent text-accent-ink'
						: 'text-muted hover:text-accent'}"
				>
					{seg.label}
				</button>
			{/if}
		{/each}
	</div>

	<!-- Row 3 — muted candidates line (only when ≥1 provider candidate exists). -->
	{#if showCandidates}
		<p class="text-[0.7rem] text-muted">
			<span class="uppercase tracking-wide">candidates</span>
			{#if fileVal}
				· <span>file</span> <span class="text-ink">"{fileVal}"</span>
			{/if}
			{#each provCands as c (c.source)}
				· <span>{c.provider || providerOf(c.source)}</span>
				<span class="text-ink">"{c.value}"</span>
			{/each}
			{#if providersDiffer(field)}
				· <span class="italic">providers differ</span>
			{/if}
		</p>
	{/if}

	{#if error}
		<p class="text-xs text-warn" aria-live="polite">{error}</p>
	{/if}
</div>
