<script lang="ts">
	// One plan row, the same shape for every reason (handoff calls 1–5): title line with the 16 px
	// checkbox slot, the name line(s), then the explanation. Names are never truncated.
	import {
		badNameLine,
		changedFields,
		changedSince,
		failureLine,
		lockLine,
		plainName,
		readBack,
		reasonChip,
		staleFailure,
		tokenLabel,
		type RenamePlanRow
	} from '#lib/rename.ts';
	import NameLockButton from './NameLockButton.svelte';

	export type RowJob = { state: 'renaming' | 'renamed' | 'failed'; error?: string };

	let {
		row,
		templates,
		selectable = false,
		selected = false,
		job,
		ontoggle,
		onlock,
		onretry,
		ondismiss,
		onrefresh
	}: {
		row: RenamePlanRow;
		// The template list the plan used, for a won't-read-back row's explanation.
		templates: string[];
		// Whether the row takes a checkbox (a reason that renames, in Manual and above, not previewing).
		selectable?: boolean;
		selected?: boolean;
		job?: RowJob;
		ontoggle?: () => void;
		onlock?: (locked: boolean) => void;
		onretry?: () => void;
		ondismiss?: () => void;
		onrefresh?: () => void;
	} = $props();

	const chip = $derived(reasonChip(row));
	const showsNew = $derived(!!row.name && row.name !== row.current && (row.reason === 'will-rename' || row.reason === 'out-of-date'));
	const rb = $derived(row.reason === 'wont-read-back' ? readBack(row, templates) : null);
	const meta = $derived(
		[row.template != null && !chip ? `template ${row.template + 1}` : '', row.reason === 'out-of-date' ? 'out of date' : '']
			.filter(Boolean)
			.join(' · ')
	);
	const locked = $derived(row.reason === 'locked');
</script>

<li class="flex gap-3 border-t border-rule px-3 py-2.5 first:border-t-0" data-video={row.video_id}>
	<span class="flex w-4 shrink-0 justify-center pt-0.5">
		{#if job?.state === 'renaming'}
			<svg class="h-4 w-4 animate-spin text-accent" viewBox="0 0 24 24" fill="none" role="img" aria-label="Renaming">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
			</svg>
		{:else if job?.state === 'renamed'}
			<span class="text-sm leading-none text-accent" role="img" aria-label="Renamed">✓</span>
		{:else if job?.state === 'failed'}
			<span class="text-sm leading-none text-warn" role="img" aria-label="Failed">⚠</span>
		{:else if selectable}
			<input
				type="checkbox"
				class="h-4 w-4 accent-accent"
				checked={selected}
				aria-label="Rename {row.title || row.current}"
				onchange={() => ontoggle?.()}
			/>
		{/if}
	</span>
	<div class="min-w-0 flex-1 space-y-1">
		<div class="flex items-start justify-between gap-2">
			<p class="min-w-0 text-sm text-ink">
				<a href="/media/{row.video_id}" class="hover:underline" data-plan-video>{row.title || row.current}</a>
				{#if job}
					<span class="text-xs {job.state === 'failed' ? 'text-warn' : 'text-muted'}">{job.state}</span>
				{:else if chip}
					<span
						class="ml-1 inline-block rounded-full border px-1.5 py-px align-middle text-[10px] {chip.refusal
							? 'border-warn text-warn'
							: 'border-rule text-muted'}">{chip.label}</span
					>
				{/if}
				{#if meta && !job}<span class="text-xs text-muted">{meta}</span>{/if}
			</p>
			{#if !job}
				<span class="shrink-0">
					{#if locked}
						<NameLockButton videoId={row.video_id} title={row.title || row.current} locked labelled onchange={onlock} />
					{:else}
						<NameLockButton videoId={row.video_id} title={row.title || row.current} locked={false} onchange={onlock} />
					{/if}
				</span>
			{/if}
		</div>

		{#if job?.state === 'renamed'}
			<p class="font-mono text-xs text-ink wrap-anywhere">{row.name}</p>
		{:else if showsNew}
			<p class="flex flex-wrap gap-x-2 font-mono text-xs wrap-anywhere">
				<span class="min-w-0 text-muted line-through decoration-warn">{row.current}</span>
				<span class="min-w-0 text-ink max-sm:basis-full"><span class="text-muted" aria-label="becomes">→</span> {row.name}</span>
			</p>
		{:else}
			<p class="font-mono text-xs text-muted wrap-anywhere">{row.current}</p>
		{/if}

		{#if job?.state === 'failed'}
			<p class="text-xs text-warn">
				{failureLine(job.error)}
				{#if staleFailure(job.error)}
					<button type="button" class="text-accent hover:underline" onclick={onrefresh}>Refresh plan</button>
				{:else}
					<button type="button" class="text-accent hover:underline" onclick={onretry}>Retry</button>
				{/if}
				<button type="button" class="text-muted hover:text-ink hover:underline" onclick={ondismiss}>Dismiss</button>
			</p>
		{:else if !job}
			{#if row.reason === 'out-of-date' && row.changes?.length}
				<p class="text-xs text-muted max-sm:hidden">
					Since Holodex named it, {changedFields(row.changes)} changed:
					{#each row.changes as c, i (c.token)}{#if i > 0}, {/if}<span class="font-mono text-ink">{c.recorded.join(', ')}</span>
						→ <span class="font-mono text-ink">{c.current.join(', ')}</span>{/each}
				</p>
				<p class="text-xs text-muted sm:hidden">{changedSince(row.changes)}</p>
			{:else if row.holder_id && row.name}
				<p class="text-xs text-muted">
					Video {row.holder_id} already has <span class="font-mono text-ink">{plainName(row.name)}</span>, so this one carries its id.
				</p>
			{:else if row.reason === 'wont-read-back'}
				<p class="text-xs text-muted">
					{#if rb}
						{tokenLabel(rb.token).replace(/^./, (s) => s.toUpperCase())}
						<span class="font-mono text-ink">{rb.value}</span>
						{#if rb.breaker}
							contains <span class="font-mono text-ink">{rb.breaker}</span>, which ends {`{${rb.token}}`} in template {rb.template}.
							It would read back as <span class="font-mono text-ink">{rb.readsAs || '(nothing)'}</span>.
						{:else}
							would read back as <span class="font-mono text-ink">{rb.readsAs || '(nothing)'}</span>.
						{/if}
					{:else}
						The name wouldn't read back to the same values.
					{/if}
				</p>
			{:else if row.reason === 'clash' && row.name}
				<p class="text-xs text-muted">
					<span class="font-mono text-ink">{row.name}</span> is already in this folder and isn't in your library.
				</p>
			{:else if row.reason === 'bad-name'}
				<p class="text-xs text-muted">{badNameLine(row)}</p>
			{:else if row.reason === 'locked'}
				<p class="text-xs text-muted">{lockLine(row.lock_source, row.locked_at)}</p>
			{:else if row.reason === 'held'}
				<p class="text-xs text-muted">
					Its original is held. Keep or restore it on its video page to rename it.
				</p>
			{:else if row.reason === 'no-template'}
				<p class="text-xs text-muted">No template fits: each one needs a value this video doesn't have.</p>
			{/if}
		{/if}
	</div>
</li>
