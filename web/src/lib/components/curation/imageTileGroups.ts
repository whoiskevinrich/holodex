// The Poster row's tile groups (F85, docs/design/writeback-poster-chooser-handoff.md §2): the
// tiles of SourceImageTiles, grouped by where each image came from. Pure; pinned by
// imageTileGroups.test.ts.
import { formatDuration } from '#lib/format.ts';
import { isLocalSource, isProviderSource, providerOf, type SourceChip } from '#lib/f36.ts';
import type { ResolvedField } from '#lib/types.ts';

export interface TileGroup {
	key: string;
	caption: string;
	// Always a list, even for one tile, so several posters per provider (HOLODEX-600) join a
	// group without a layout or keyboard change. Empty only for a provider with no poster.
	chips: SourceChip[];
}

// imageTileGroups orders the groups In the file → Holodex → Automatic → one per provider.
// Holodex and Automatic are left out when they have no tile. `providers` are the video's
// matched providers: one with no poster sends no candidate, so its group comes from this list
// and has no tiles. A provider folded into another's tile (the same image) has no group.
export function imageTileGroups(chips: SourceChip[], providers: string[] = [], baselineKey = 'file'): TileGroup[] {
	const pick = (keys: string[]) => keys.flatMap((k) => chips.filter((c) => c.key === k));
	const groups: TileGroup[] = [{ key: baselineKey, caption: 'In the file', chips: pick([baselineKey]) }];
	// A standing manual literal (API-made) is the owner's own choice, so it sits with theirs.
	const holodex = [...pick(['local:frame', 'local:upload']), ...chips.filter((c) => c.manual && c.value)];
	if (holodex.length) groups.push({ key: 'holodex', caption: 'Holodex', chips: holodex });
	const automatic = pick(['local:automatic']);
	if (automatic.length) groups.push({ key: 'automatic', caption: 'Automatic', chips: automatic });

	const provChips = chips.filter((c) => isProviderSource(c.key));
	const names = [...new Set([...providers, ...provChips.map((c) => providerOf(c.key))])];
	for (const name of names) {
		const own = provChips.filter((c) => providerOf(c.key) === name);
		if (!own.length && chips.some((c) => c.sources.includes(name))) continue;
		groups.push({ key: `provider:${name}`, caption: own[0]?.labels[0] ?? name, chips: own });
	}
	return groups;
}

// tileImage is what a tile shows: the stored image Holodex serves for `·file` and its own
// posters (never the page's presented poster, and never a `local:` token), else the
// candidate's URL. '' means the file has no cover art.
export function tileImage(field: ResolvedField, chip: SourceChip): string {
	const candidate = field.candidates?.find((c) => c.source === chip.key);
	if (candidate?.image_url) return candidate.image_url;
	return chip.key === 'file' || isLocalSource(chip.key) ? '' : chip.value;
}

// tileTitle: the picked time on the `·frame` tile (P1-1), `frame at m:ss` or `h:mm:ss`.
export function tileTitle(field: ResolvedField, chip: SourceChip): string | undefined {
	const at = field.candidates?.find((c) => c.source === chip.key)?.frame_at;
	return at === undefined ? undefined : `frame at ${formatDuration(at)}`;
}
