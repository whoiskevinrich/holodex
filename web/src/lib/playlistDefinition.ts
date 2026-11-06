// The playlist editor's pure rules (F89 handoff smart-playlist-rules-handoff.md): the cap
// readout's wording, and which chip and copy a refused draft turns into.
import { ApiError } from './api';
import { filtersToParams } from './filters';
import { toMessage } from './format';
import type { MediaQuery } from './listState';
import type { ChipCond, ChipModel } from './query';
import type { PlaylistRefusal, PlaylistStaleRef } from './types';

/** The controls row's readout (P0-2). `saved` is the stored count, once it differs. */
export function capReadout(matches: number, cap: number | null, random: boolean, saved?: number): string {
	const n = matches.toLocaleString();
	let s = `${n} match`;
	if (cap != null && cap < matches) {
		s = random ? `${n} match, showing a fresh ${cap.toLocaleString()} each play` : `${n} match, showing ${cap.toLocaleString()}`;
	}
	const shown = cap != null ? Math.min(cap, matches) : matches;
	return saved != null && saved !== shown ? `${s} (was ${saved.toLocaleString()})` : s;
}

/** A draft's stored query string. missing_facet isn't list state (listState.ts), so it rides apart. */
export function draftQueryString(query: MediaQuery, missingFacets: string[]): string {
	const p = filtersToParams({ ...query, sort: undefined }, false);
	for (const m of missingFacets) p.append('missing_facet', m);
	return p.toString();
}

/** Whether leaving would lose anything: the applied draft, or text typed and not applied. */
export function draftDirty(
	draft: { qs: string; sort: string; cap: number | null },
	saved: { qs: string; sort: string; cap: number | null },
	text: string,
	appliedText: string
): boolean {
	return draft.qs !== saved.qs || draft.sort !== saved.sort || draft.cap !== saved.cap || text.trim() !== appliedText.trim();
}

/** Why a draft can't save, one sentence and its fix. */
export interface DefinitionProblem {
	sentence: string;
	fix: string;
	/** The refused reference, whose chip is marked. */
	via?: number;
	/** The definition's own owner-only conditions are at fault. */
	ownerOnly?: boolean;
}

/** Every chip in a model, group members included. */
export function allChips(model: ChipModel | null): ChipCond[] {
	if (model?.kind !== 'chips') return [];
	return model.items.flatMap((i) => (i.kind === 'chip' ? [i.chip] : i.members.flat()));
}

/** A problem the parse alone shows: a stored reference whose target is gone. The server
 * can't word it, since its canonicaliser refuses the id first. */
export function missingProblem(model: ChipModel | null): DefinitionProblem | null {
	const gone = allChips(model).find((c) => c.missing);
	if (!gone) return null;
	if (gone.id == null) {
		return { sentence: 'This definition filters on a field that no longer exists.', fix: `Remove “${gone.label}”.` };
	}
	const noun = gone.field === 'playlist' ? 'a playlist' : 'something';
	return { sentence: `This definition refers to ${noun} that was deleted.`, fix: `Remove “${gone.label}”.` };
}

/** A refused save or check as the editor's problem. */
export function toProblem(e: unknown, model: ChipModel | null): DefinitionProblem {
	const body = e instanceof ApiError ? (e.body as Partial<PlaylistRefusal> | undefined) : undefined;
	const ref: PlaylistStaleRef | undefined = body?.stale_refs?.[0];
	if (body?.owner_only) {
		const chip = allChips(model).find((c) => c.ownerOnly);
		return {
			sentence: body.error ?? toMessage(e),
			fix: chip ? `Remove “${chip.label}”, or make the playlist private.` : (body.fix ?? ''),
			ownerOnly: true
		};
	}
	return { sentence: body?.error ?? toMessage(e), fix: body?.fix ?? '', via: ref?.via };
}

/** Whether a problem marks this chip. */
export function chipAtFault(chip: ChipCond, problem: DefinitionProblem | null): boolean {
	if (chip.missing) return true;
	if (!problem) return false;
	if (problem.ownerOnly) return chip.ownerOnly === true;
	return problem.via != null && chip.field === 'playlist' && chip.id === problem.via;
}

/** The grid-replacing notice for a stale playlist, owner's copy (handoff "New notices"). */
export function staleNotice(r: PlaylistStaleRef): { sentence: string; detail: string; makePrivate?: boolean } {
	const fix = 'Nothing is shown until the definition is fixed.';
	const noun: Record<string, string> = { person: 'person', tag: 'tag', studio_id: 'studio', category_id: 'category', playlist: 'playlist' };
	switch (r.kind) {
		case 'missing': {
			const n = noun[r.key ?? ''] ?? 'entry';
			return { sentence: `This playlist refers to a ${n} that was deleted`, detail: `(${n} #${r.value}). ${fix}` };
		}
		case 'unknown_key':
			return { sentence: 'This playlist filters on a field that no longer exists', detail: `(${r.key}). ${fix}` };
		case 'owner_only':
			if (r.key === 'playlist') {
				return {
					sentence: 'This public playlist includes a private playlist',
					detail: `(“${r.via_name ?? r.name}”). Make it private or edit the definition.`,
					makePrivate: true
				};
			}
			return {
				sentence: r.via_name ? `This public playlist includes one with an owner-only condition` : 'This public playlist uses an owner-only condition',
				detail: `${r.via_name ? `(“${r.via_name}”). ` : ''}Make it private or edit the definition.`,
				makePrivate: true
			};
		case 'loop':
			return { sentence: 'This playlist includes itself through another playlist', detail: `(${r.name}). ${fix}` };
		case 'limit':
			return {
				sentence: 'This playlist nests too many playlists to show',
				detail: '(another playlist’s edit took it past a limit). Nothing is shown until the definition is simplified.'
			};
		case 'unstable':
			return {
				sentence: 'This playlist includes one that is shuffled and capped',
				detail: `(“${r.name}”), so its contents change every time. ${fix}`
			};
		default:
			return { sentence: 'This playlist’s definition can’t be read', detail: fix };
	}
}
