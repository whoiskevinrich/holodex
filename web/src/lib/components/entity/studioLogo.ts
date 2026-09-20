// Studio logo caption rule (HOLODEX-411, docs/design/studio-logo-link-card-handoff.md §1;
// reused by the /studios list rows, HOLODEX-432). A logo drawn bare on the page is a
// *wordmark* when its natural width is at least twice its height — it already says the
// name, so the caption beside it is dropped and the name moves to the image alt. A squarer
// logo (a symbol mark), an icon on the plate, and the monogram keep the caption.
//
// `wordmark` is tri-state: null = the logo has not loaded yet, so nothing renders beside it
// (no caption flash on a wordmark); false = a failed load or a symbol mark (name shows).

export type Wordmark = boolean | null;

export function isWordmark(naturalWidth: number, naturalHeight: number): boolean {
	return naturalHeight > 0 && naturalWidth >= 2 * naturalHeight;
}

// showName: whether the studio name is rendered as text beside the image. `bare` is
// "this is a logo" (vs. icon/monogram on the plate) — only a bare logo can be a wordmark.
export function showName(bare: boolean, wordmark: Wordmark): boolean {
	return !bare || wordmark === false;
}

// imageAlt: the name is announced exactly once — as adjacent text when the caption shows
// (alt is empty, the image is decorative), or as the image alt when it doesn't.
export function imageAlt(name: string, bare: boolean, wordmark: Wordmark): string {
	return showName(bare, wordmark) ? '' : name;
}
