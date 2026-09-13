import { describe, expect, it } from 'vitest';
import { collisionOpen, detailLabel, hasDetail, normalizeLabel } from './candidateDetail';

// The label-collision rule from docs/design/candidates-detail-handoff.md ("States and
// interactions") and the QA checklist's §2.3 fixtures. Each case is named for the
// checklist item it covers so a failure points at the design doc.
const lines = ['Studio: Outlet B › Network X', 'Record: 28 tags · synopsis'];
const c = (external_id: string, label: string, detail?: string[]) => ({ external_id, label, detail });

describe('collisionOpen — fixture (a): four same-label rows plus one distinct', () => {
	const cands = [
		c('t:1', 'Harbor Lights', lines),
		c('t:2', 'harbor  lights', lines),
		c('t:3', 'Harbor Lights', lines),
		c('t:4', ' Harbor Lights ', lines),
		c('t:5', 'Harbor Lights II', lines)
	];
	const open = collisionOpen(cands);

	it('2.3 · every member of the collision group opens on first render', () => {
		expect(open).toEqual({ 't:1': true, 't:2': true, 't:3': true, 't:4': true });
	});

	it('2.3 · a distinct label outside the group stays closed even with detail', () => {
		expect(open['t:5']).toBeUndefined();
	});

	it('2.3 · normalization: case, doubled and surrounding whitespace collide', () => {
		expect(normalizeLabel(' Harbor  LIGHTS ')).toBe('harbor lights');
		expect(normalizeLabel('harbor lights')).toBe(normalizeLabel('Harbor\tLights'));
	});
});

describe('collisionOpen — fixture (c): mixed group', () => {
	it('2.3 · only the member that carries detail opens; the other has no toggle', () => {
		const open = collisionOpen([c('t:1', 'Harbor Lights', lines), c('t:2', 'Harbor Lights')]);
		expect(open).toEqual({ 't:1': true });
	});
});

describe('collisionOpen — fixture (b) and (d)', () => {
	it('2.3 · all-distinct labels open nothing, however much detail they carry', () => {
		expect(
			collisionOpen([c('t:1', 'A', lines), c('t:2', 'B', lines), c('t:3', 'C', lines)])
		).toEqual({});
	});

	it('pre-F61 provider (no detail anywhere) opens nothing', () => {
		expect(collisionOpen([c('t:1', 'A'), c('t:2', 'A')])).toEqual({});
	});

	it('empty response ⇒ empty map', () => {
		expect(collisionOpen([])).toEqual({});
	});
});

describe('hasDetail — toggle presence', () => {
	it('2.4 · missing key and [] are the same: no toggle', () => {
		expect(hasDetail(c('t:1', 'A'))).toBe(false);
		expect(hasDetail(c('t:1', 'A', []))).toBe(false);
		expect(hasDetail(c('t:1', 'A', ['x']))).toBe(true);
	});
});

describe('detailLabel — toggle copy', () => {
	it('reads `details` closed and `hide details` open', () => {
		expect(detailLabel(false)).toBe('details');
		expect(detailLabel(true)).toBe('hide details');
	});
});
