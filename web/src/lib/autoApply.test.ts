import { describe, expect, it } from 'vitest';
import { autoApplyPick } from './autoApply';

const strong = { external_id: '812', auto_apply: true };
const weak = { external_id: '9001', auto_apply: false };

// F47 RD1: exactly one strong candidate applies unattended; anything else waits for
// the owner's pick. HOLODEX-418: a re-match never applies unattended, however strong.
describe('autoApplyPick', () => {
	it('picks the lone strong candidate on an allowed (first-match) search', () => {
		expect(autoApplyPick([strong], true)).toBe(strong);
	});
	it('ignores weaker candidates beside the lone strong one', () => {
		expect(autoApplyPick([weak, strong, { ...weak, external_id: '9002' }], true)).toBe(strong);
	});
	it('falls through to the list for zero or two-plus strong candidates', () => {
		expect(autoApplyPick([], true)).toBeUndefined();
		expect(autoApplyPick([weak], true)).toBeUndefined();
		expect(autoApplyPick([strong, { ...strong, external_id: '813' }], true)).toBeUndefined();
	});
	it('never picks when the caller disallows it — a re-match or an owner-typed search', () => {
		expect(autoApplyPick([strong], false)).toBeUndefined();
	});
});
