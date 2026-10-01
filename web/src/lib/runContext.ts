import { goto } from '$app/navigation';
import { api } from '$lib/api';
import { forgetPlaylist, loadPlaylist, setPlayIntent } from '$lib/playlistContext';
import type { Video } from '$lib/types';
import { sortScenes } from '$lib/filmScenes';
import {
	loadRun,
	newRunId,
	newSeed,
	queryLabel,
	runHref,
	sameSource,
	saveRun,
	startRun,
	type Run,
	type RunMode,
	type RunParam,
	type RunSource
} from '$lib/run';

// The impure half of a run (ADR-121 D7): fetching a source's ids, starting a run from a
// grid or a playlist page, and rebuilding one from the URL when its stored copy is gone
// (a new tab, cleared storage). The pure model is run.ts.

function storage(): Storage | undefined {
	try {
		return sessionStorage;
	} catch {
		return undefined;
	}
}

export { queryLabel } from '$lib/run';

export interface SourceIds {
	ids: number[];
	seed?: number;
	/** The source's display name, when fetching it told us (a playlist or a film). */
	name?: string;
}

/** The source's ids in source order. `fresh` re-reads a playlist past its fetch cache. */
export async function fetchSourceIds(source: RunSource, fresh = false): Promise<SourceIds> {
	if (source.kind === 'playlist') {
		if (fresh) forgetPlaylist(source.id);
		const res = await loadPlaylist({ id: source.id, seed: source.seed });
		if (!res) throw new Error('playlist unavailable');
		return { ids: res.items.map((v) => v.id), seed: res.seed, name: res.playlist.name };
	}
	if (source.kind === 'film') {
		const res = await api.getFilm(source.id);
		return { ids: sortScenes(res.scenes ?? []).map((s) => s.video.id), name: res.film.name };
	}
	return api.mediaIds(source.query, source.sort, source.seed);
}

function withFetched(source: RunSource, got: SourceIds): RunSource {
	const out = got.seed != null && source.kind !== 'film' ? { ...source, seed: got.seed } : source;
	return got.name ? { ...out, label: { ...out.label, name: got.name } } : out;
}

/**
 * Start a run from a grid or a playlist page (Play all / Shuffle): fetch the ids once
 * (spec RD5), go to the first item with a play intent so it autoplays (an in-app press
 * is a gesture). Resolves false when there's nothing to play or the fetch fails, so the
 * caller can say "Couldn't start playback".
 */
export async function beginRun(source: RunSource, mode: RunMode): Promise<boolean> {
	try {
		const got = await fetchSourceIds(source);
		if (!got.ids.length) return false;
		const run = startRun({ id: newRunId(), source: withFetched(source, got), ids: got.ids, mode, seed: newSeed() });
		saveRun(storage(), run);
		setPlayIntent(run.order[0]);
		await goto(runHref(run.order[0], run));
		return true;
	} catch {
		return false;
	}
}

/**
 * The run a /media/[id] URL belongs to: the stored one when this tab has it, else a new
 * in-order run rebuilt from the URL's source (no play intent, so nothing autoplays).
 * Null when the source can't be read (a private playlist seen by a visitor, say): the
 * param is then inert and the video just plays.
 */
export async function resolveRun(param: RunParam): Promise<Run | null> {
	const held = param.run ? loadRun(storage(), param.run) : null;
	if (held && sameSource(held.source, param)) return held;
	const source: RunSource =
		param.playlist != null
			? { kind: 'playlist', id: param.playlist, seed: param.seed, label: { kind: '', name: '', href: `/playlists/${param.playlist}` } }
			: param.film != null
				? { kind: 'film', id: param.film, label: { kind: 'Film', name: '', href: `/films/${param.film}` } }
				: { kind: 'query', query: param.from ?? '', sort: param.sort, seed: param.seed, label: queryLabel(param.from ?? '') };
	try {
		const got = await fetchSourceIds(source);
		const run = startRun({
			id: param.run ?? newRunId(),
			source: withFetched(source, got),
			ids: got.ids,
			mode: 'in-order',
			seed: newSeed()
		});
		saveRun(storage(), run);
		return run;
	} catch {
		return null;
	}
}

/** Persist a run after a toggle or a new pass. */
export function storeRun(run: Run) {
	saveRun(storage(), run);
}

// The next-up tile: a playlist already carries its items; a grid run fetches the one
// video it needs, once.
const tiles = new Map<number, Promise<Video | null>>();

export function nextTile(run: Run, videoId: number): Promise<Video | null> {
	if (run.source.kind === 'playlist') {
		return loadPlaylist({ id: run.source.id, seed: run.source.seed }).then(
			(res) => res?.items.find((v) => v.id === videoId) ?? null
		);
	}
	let pending = tiles.get(videoId);
	if (!pending) {
		pending = api
			.getMedia(videoId)
			.then((res) => res.video)
			.catch(() => {
				tiles.delete(videoId);
				return null;
			});
		tiles.set(videoId, pending);
	}
	return pending;
}
