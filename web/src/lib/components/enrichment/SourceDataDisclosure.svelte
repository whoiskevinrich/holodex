<script lang="ts">
	// The owner's raw source payloads at the foot of the media page, as one disclosure
	// (HOLODEX-97, handoff Part C): a source switcher and a single key/value table, instead
	// of an "Enrichment data: …" toggle per source in two layouts. An audit view of what
	// each source sent, so keys stay raw (the file's source_key, the provider's canonical)
	// and an image field shows its URL, never a second render of the poster. Tokens only.
	import type { EnrichedField, ExtraMetadata } from '#lib/types.ts';

	let {
		file,
		providers
	}: {
		file: ExtraMetadata[];
		providers: Map<string, EnrichedField[]>;
	} = $props();

	interface Row {
		key: string;
		value: string;
		mono: boolean;
	}
	interface Source {
		name: string;
		rows: Row[];
	}

	const LONG_TEXT_MAX = 120;

	function providerValue(f: EnrichedField): string {
		if (f.display === 'long_text') {
			const v = f.values[0] ?? '';
			return v.length > LONG_TEXT_MAX ? v.slice(0, LONG_TEXT_MAX) + '…' : v;
		}
		return f.values.join(', ');
	}

	const sources = $derived.by(() => {
		const out: Source[] = [];
		if (file.length) {
			out.push({ name: 'File', rows: file.map((m) => ({ key: m.source_key, value: m.value, mono: false })) });
		}
		for (const [p, fields] of providers) {
			out.push({
				name: p,
				rows: fields.map((f) => ({ key: f.canonical, value: providerValue(f), mono: f.display === 'image_url' }))
			});
		}
		return out;
	});
	const keyCount = $derived(sources.reduce((n, s) => n + s.rows.length, 0));

	const panelId = $props.id();
	let open = $state(false);
	let selected = $state('');
	// Falls back to the first source when nothing is picked yet or the pick was cleared.
	const current = $derived(sources.find((s) => s.name === selected) ?? sources[0]);

	function onGroupKey(e: KeyboardEvent) {
		const delta =
			e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
		if (!delta || !current) return;
		e.preventDefault();
		const i = sources.indexOf(current);
		selected = sources[(i + delta + sources.length) % sources.length].name;
		(e.currentTarget as HTMLElement).querySelector<HTMLElement>(`[data-source="${CSS.escape(selected)}"]`)?.focus();
	}
</script>

{#if sources.length}
	<section>
		<button
			type="button"
			onclick={() => (open = !open)}
			aria-expanded={open}
			aria-controls={panelId}
			class="text-sm text-muted hover:text-ink"
		>
			{open ? '▾' : '▸'} Source data · {sources.length} source{sources.length === 1 ? '' : 's'}, {keyCount}
			key{keyCount === 1 ? '' : 's'}
		</button>
		<div id={panelId} hidden={!open} class="mt-2 space-y-2">
			{#if sources.length > 1}
				<div role="radiogroup" aria-label="Source" tabindex="-1" class="flex flex-wrap gap-2" onkeydown={onGroupKey}>
					{#each sources as s (s.name)}
						{@const on = s.name === current?.name}
						<button
							type="button"
							role="radio"
							aria-checked={on}
							tabindex={on ? 0 : -1}
							data-source={s.name}
							onclick={() => (selected = s.name)}
							class="rounded-theme px-2.5 py-1 text-xs {on ? 'btn-accent' : 'btn-quiet border border-rule'}"
						>
							{s.name}
						</button>
					{/each}
				</div>
			{/if}
			{#if current}
				<table class="w-full text-left text-xs">
					<caption class="sr-only">{current.name} source data</caption>
					<tbody>
						{#each current.rows as r, i (r.key + i)}
							<tr class="border-b border-rule">
								<td class="py-1 pr-4 align-top font-mono text-muted">{r.key}</td>
								<td class="py-1 text-ink {r.mono ? 'break-all font-mono' : ''}" title={r.mono ? r.value : undefined}
									>{r.value}</td
								>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	</section>
{/if}
