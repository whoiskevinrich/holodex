import { describe, it, expect, beforeEach, vi } from 'vitest';
import { hotkeys, guardKeydown, fire, SHEET_KEY, type KeyLike } from './hotkey.svelte';

// Tests run in node (no DOM): a "node" is any object with focus/click, and the guard
// reads only the KeyLike shape.
function fakeNode(label = 'Refresh all') {
	return {
		textContent: label,
		title: '',
		focus: vi.fn(),
		click: vi.fn()
	} as unknown as HTMLElement;
}

function ev(over: Partial<KeyLike> = {}): KeyLike {
	return {
		key: 'e',
		defaultPrevented: false,
		ctrlKey: false,
		altKey: false,
		metaKey: false,
		target: { tagName: 'BODY' },
		...over
	};
}

describe('hotkey registry (F62 P0-1)', () => {
	beforeEach(() => {
		hotkeys.entries = [];
	});

	it('registers and unregisters by node', () => {
		const n = fakeNode();
		hotkeys.register({ key: 'e', label: 'Refresh all', node: n });
		expect(hotkeys.find('e')?.label).toBe('Refresh all');
		hotkeys.unregister(n);
		expect(hotkeys.find('e')).toBeUndefined();
	});

	it('duplicate key: first wins, one row listed, dev warning', () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
		const a = fakeNode('A');
		const b = fakeNode('B');
		hotkeys.register({ key: 'e', label: 'A', node: a });
		hotkeys.register({ key: 'e', label: 'B', node: b });
		expect(hotkeys.find('e')?.node).toBe(a);
		expect(hotkeys.rows.map((r) => r.label)).toEqual(['A']);
		hotkeys.register({ key: 'a', label: 'Z', node: fakeNode('Z') });
		expect(hotkeys.rows.map((r) => r.key)).toEqual(['a', 'e']);
		expect(warn).toHaveBeenCalledTimes(1);
		// Unmounting the first promotes the second — no stale binding.
		hotkeys.unregister(a);
		expect(hotkeys.find('e')?.node).toBe(b);
		warn.mockRestore();
	});
});

describe('guardKeydown (F62 RD6)', () => {
	beforeEach(() => {
		hotkeys.entries = [];
		hotkeys.register({ key: 'e', label: 'Refresh all', node: fakeNode() });
	});

	it('fires a registered key from body', () => {
		expect(guardKeydown(ev(), false)).toBe('e');
	});

	it('ignores an unregistered key', () => {
		expect(guardKeydown(ev({ key: 'z' }), false)).toBeNull();
	});

	it('respects defaultPrevented (HOLODEX-249)', () => {
		expect(guardKeydown(ev({ defaultPrevented: true }), false)).toBeNull();
	});

	it('bails on Ctrl/Alt/Meta, allows Shift (for ?)', () => {
		expect(guardKeydown(ev({ ctrlKey: true }), false)).toBeNull();
		expect(guardKeydown(ev({ altKey: true }), false)).toBeNull();
		expect(guardKeydown(ev({ metaKey: true }), false)).toBeNull();
		expect(guardKeydown(ev({ key: SHEET_KEY }), false)).toBe(SHEET_KEY);
	});

	it.each(['INPUT', 'TEXTAREA', 'SELECT', 'VIDEO'])('never fires while %s is focused', (tag) => {
		expect(guardKeydown(ev({ target: { tagName: tag } }), false)).toBeNull();
		expect(guardKeydown(ev({ key: SHEET_KEY, target: { tagName: tag } }), false)).toBeNull();
	});

	it('never fires inside contenteditable', () => {
		expect(guardKeydown(ev({ target: { tagName: 'DIV', isContentEditable: true } }), false)).toBeNull();
	});

	it('another open dialog silences page keys AND the sheet toggle (never stacks)', () => {
		expect(guardKeydown(ev(), true)).toBeNull();
		expect(guardKeydown(ev({ key: SHEET_KEY }), true)).toBeNull();
	});

	it('the open sheet silences page keys but ? still closes it', () => {
		expect(guardKeydown(ev(), false, true)).toBeNull();
		expect(guardKeydown(ev({ key: SHEET_KEY }), false, true)).toBe(SHEET_KEY);
	});
});

describe('fire (F62 RD2)', () => {
	it('focuses then clicks the bound node', () => {
		const n = fakeNode();
		const order: string[] = [];
		(n.focus as ReturnType<typeof vi.fn>).mockImplementation(() => order.push('focus'));
		(n.click as ReturnType<typeof vi.fn>).mockImplementation(() => order.push('click'));
		fire({ key: 'e', label: 'Refresh all', node: n });
		expect(order).toEqual(['focus', 'click']);
	});
});
