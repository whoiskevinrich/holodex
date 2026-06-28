import { describe, it, expect } from 'vitest';
import { cropAffine, cropTargetSize, CORE_ROLE_ASPECT } from './cropGeometry';
import { CORE_ROLES } from './types';

describe('cropTargetSize', () => {
	it('derives output dimensions from each role aspect (short edge fixed)', () => {
		expect(cropTargetSize('headshot')).toEqual([600, 600]);
		expect(cropTargetSize('banner')).toEqual([1500, 600]);
		expect(cropTargetSize('poster')).toEqual([600, 900]);
	});

	it('output ratio always equals the role aspect (no drift from the source of truth)', () => {
		for (const role of CORE_ROLES) {
			const [w, h] = cropTargetSize(role);
			const [aw, ah] = CORE_ROLE_ASPECT[role];
			expect(w / h).toBeCloseTo(aw / ah, 5);
		}
	});
});

describe('cropAffine', () => {
	it('maps a square image filling a square frame to fill the canvas (no crop)', () => {
		// 100px square source, 300px square frame, no zoom/pan, 600px canvas.
		// contain scale = 3 → image fills the frame; canvas is 2× the frame.
		const a = cropAffine({ fw: 300, fh: 300, nw: 100, nh: 100, zoom: 1, offsetX: 0, offsetY: 0, cw: 600, ch: 600 });
		expect(a).toEqual({ ax: 6, ay: 6, ex: 0, fy: 0 });
		// → drawImage(100×100) renders at 600×600, exactly filling the canvas.
		expect(a.ax * 100).toBe(600);
	});

	it('centres a square image letterboxed inside a wide (5:1) frame', () => {
		// 100px square in a 500×100 frame: contain scale = 1, image centred with side bars.
		// Canvas 1500×300 (3× the frame). The image should land centred horizontally.
		const a = cropAffine({ fw: 500, fh: 100, nw: 100, nh: 100, zoom: 1, offsetX: 0, offsetY: 0, cw: 1500, ch: 300 });
		expect(a.ax).toBe(3);
		expect(a.ay).toBe(3);
		// Image drawn from x=ex to ex+100*ax = 600..900, centred in the 1500-wide canvas.
		expect(a.ex).toBe(600);
		expect(a.ex + 100 * a.ax).toBe(900);
		expect(a.fy).toBe(0);
	});

	it('uses a uniform scale only when the canvas aspect matches the frame aspect', () => {
		// Regression for the banner crop bug: the crop preview frame is 5:2 but the output
		// canvas was 5:1 (1500×300), so x was scaled 2× more than y — the stored JPEG came
		// out anamorphically stretched and displayed at the wrong scale. A 5:2 frame must be
		// paired with a 5:2 canvas (1500×600) so ax === ay (no distortion).
		const frame = { fw: 500, fh: 200, nw: 100, nh: 100, zoom: 1, offsetX: 0, offsetY: 0 }; // 5:2 frame
		const good = cropAffine({ ...frame, cw: 1500, ch: 600 }); // 5:2 canvas — matches
		expect(good.ax).toBe(good.ay);
		const bad = cropAffine({ ...frame, cw: 1500, ch: 300 }); // 5:1 canvas — the old bug
		expect(bad.ax).not.toBe(bad.ay);
	});

	it('zoom scales about the frame centre (a centred point stays centred)', () => {
		const base = { fw: 300, fh: 300, nw: 100, nh: 100, offsetX: 0, offsetY: 0, cw: 600, ch: 600 };
		const z1 = cropAffine({ ...base, zoom: 1 });
		const z2 = cropAffine({ ...base, zoom: 2 });
		// The image centre (natural 50,50) maps to the same canvas point at any zoom.
		const centreAt = (a: ReturnType<typeof cropAffine>) => ({ x: a.ax * 50 + a.ex, y: a.ay * 50 + a.fy });
		expect(centreAt(z2)).toEqual(centreAt(z1));
		expect(centreAt(z1)).toEqual({ x: 300, y: 300 }); // centre of the 600×600 canvas
	});
});
