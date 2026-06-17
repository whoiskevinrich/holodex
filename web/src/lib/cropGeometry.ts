// Crop geometry for the promote-with-crop editor (F25.15). Pure math, extracted from
// the component so it can be unit-tested without a DOM/canvas.

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
