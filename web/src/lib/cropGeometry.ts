// Crop geometry for the promote-with-crop editor (F25.15). Pure math, extracted from
// the component so it can be unit-tested without a DOM/canvas.

import type { CoreRole } from './types';

// SINGLE SOURCE OF TRUTH for core-role aspect ratios. These MUST stay in sync with the
// `.portrait-frame--*` (display) and `.crop-frame--*` (preview) `aspect-ratio` rules in
// app.css — the crop preview frame, the rendered display frame, and the stored output all
// share one ratio so the crop is WYSIWYG (no anamorphic stretch, no extra cover-crop).
// Changing a ratio here means updating the matching CSS frame (and vice-versa); the dev
// assertion in CropEditor flags drift between this map and the live frame.
export const CORE_ROLE_ASPECT: Record<CoreRole, readonly [number, number]> = {
	headshot: [1, 1],
	banner: [8, 3],
	poster: [2, 3]
};

// Shorter edge (px) of a rendered crop. The longer edge is derived from the role's aspect,
// so output dimensions can never drift from the frame ratio.
const CROP_SHORT_EDGE = 600;

// cropTargetSize returns the output canvas [width, height] for a role, derived from its
// aspect ratio with the shorter edge fixed at CROP_SHORT_EDGE (headshot 600×600,
// banner 1600×600, poster 600×900).
export function cropTargetSize(role: CoreRole): [number, number] {
	const [aw, ah] = CORE_ROLE_ASPECT[role];
	return aw >= ah
		? [Math.round((CROP_SHORT_EDGE * aw) / ah), CROP_SHORT_EDGE]
		: [CROP_SHORT_EDGE, Math.round((CROP_SHORT_EDGE * ah) / aw)];
}

export interface CropInput {
	fw: number; // frame content width (the crop viewport)
	fh: number; // frame content height
	nw: number; // source image natural width
	nh: number; // source image natural height
	zoom: number; // editor zoom (≥1)
	offsetX: number; // editor pan, px in frame space
	offsetY: number;
	cw: number; // output canvas width (target ratio)
	ch: number; // output canvas height
}

// 2D affine (no rotation/skew): canvas = (ax·x + ex, ay·y + fy) for a source pixel.
export interface CropAffine {
	ax: number;
	ay: number;
	ex: number;
	fy: number;
}

// cropAffine maps source-image natural pixels → output-canvas pixels, replicating the
// editor preview's `object-fit: contain` base placement followed by a translate/scale
// transform about the frame centre. Drawing the source with this transform reproduces
// exactly what the frame shows (WYSIWYG), so the stored crop matches the preview.
export function cropAffine(i: CropInput): CropAffine {
	const scaleC = Math.min(i.fw / i.nw, i.fh / i.nh); // object-fit: contain base scale
	const bw = scaleC * i.nw; // contained image size, centred in the frame
	const bh = scaleC * i.nh;
	const kx = i.cw / i.fw; // frame → canvas scale
	const ky = i.ch / i.fh;
	return {
		ax: kx * i.zoom * scaleC,
		ay: ky * i.zoom * scaleC,
		ex: kx * ((i.zoom * (i.fw - bw)) / 2 + (i.fw / 2) * (1 - i.zoom) + i.offsetX),
		fy: ky * ((i.zoom * (i.fh - bh)) / 2 + (i.fh / 2) * (1 - i.zoom) + i.offsetY)
	};
}
