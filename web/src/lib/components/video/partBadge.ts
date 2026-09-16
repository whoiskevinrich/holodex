// partBadgeLabel is the part badge's vocabulary (HOLODEX-389, design handoff §5),
// shared by VideoCard, the media page header, the film full-film row and the owner
// queue rows — one fact, one reading (cf. film/sceneNumber.ts's sceneBadgeLabel).
// Long form everywhere (OQ3): it fits beside the duration badge even at the
// 16-column tier. No pluralisation, no "of N" (RD1: ordinal only), and no
// normalisation — the lifter and extractor already produced the ordinal, and a
// hand-curated value renders verbatim (RD8: the custom chip has no validation).
export function partBadgeLabel(part: string | number): string {
	return `Part ${String(part).trim()}`;
}
