import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import ModalStack from './__fixtures__/ModalStack.svelte';

// The shared Modal's behaviour contract (HOLODEX-215, docs/design/shared-modal-handoff.md):
// focus capture/return, the Tab trap, busy-suppressed dismissal, and the stack — Escape and
// the backdrop close only the topmost modal, and a stacked modal sits one layer up.

function dialog(name: string) {
	return document.querySelector<HTMLElement>(`[role="dialog"][aria-labelledby="${name}-title"]`);
}

describe('Modal', () => {
	it('focuses the first focusable control and returns focus to the opener on close', async () => {
		const screen = await render(ModalStack);
		const opener = screen.getByRole('button', { name: 'Open A' });
		await opener.click();

		await expect.element(screen.getByRole('dialog', { name: 'Modal A' })).toBeInTheDocument();
		expect(document.activeElement).toBe(screen.getByLabelText('A field').element());

		await userEvent.keyboard('{Escape}');
		await expect.element(screen.getByRole('dialog', { name: 'Modal A' })).not.toBeInTheDocument();
		expect(document.activeElement).toBe(opener.element());
	});

	it('traps Tab, skipping disabled and tabindex="-1" controls', async () => {
		const screen = await render(ModalStack);
		await screen.getByRole('button', { name: 'Open A' }).click();
		const field = screen.getByLabelText('A field').element();
		const openB = screen.getByRole('button', { name: 'Open B' }).element();

		await userEvent.keyboard('{Tab}');
		expect(document.activeElement).toBe(openB);
		await userEvent.keyboard('{Tab}');
		expect(document.activeElement).toBe(field);
		await userEvent.keyboard('{Shift>}{Tab}{/Shift}');
		expect(document.activeElement).toBe(openB);
	});

	it('ignores Escape and backdrop clicks while busy', async () => {
		const screen = await render(ModalStack, { busy: true });
		await screen.getByRole('button', { name: 'Open A' }).click();

		await userEvent.keyboard('{Escape}');
		dialog('a')!.parentElement!.click();
		await expect.element(screen.getByRole('dialog', { name: 'Modal A' })).toBeInTheDocument();
	});

	it('leaves an Escape a control already claimed (preventDefault) alone', async () => {
		const screen = await render(ModalStack);
		await screen.getByRole('button', { name: 'Open A' }).click();
		(screen.getByLabelText('Claiming field').element() as HTMLElement).focus();

		await userEvent.keyboard('{Escape}');
		await expect.element(screen.getByRole('dialog', { name: 'Modal A' })).toBeInTheDocument();
	});

	it('closes on a backdrop click but not on a click inside the panel', async () => {
		const screen = await render(ModalStack);
		await screen.getByRole('button', { name: 'Open A' }).click();

		dialog('a')!.click();
		await expect.element(screen.getByRole('dialog', { name: 'Modal A' })).toBeInTheDocument();
		dialog('a')!.parentElement!.click();
		await expect.element(screen.getByRole('dialog', { name: 'Modal A' })).not.toBeInTheDocument();
	});

	it('stacks: one Escape closes only the topmost, and the page listener never sees it', async () => {
		const onpageescape = vi.fn();
		const screen = await render(ModalStack, { onpageescape });
		await screen.getByRole('button', { name: 'Open A' }).click();
		await screen.getByRole('button', { name: 'Open B' }).click();

		const backdropA = dialog('a')!.parentElement!;
		const backdropB = dialog('b')!.parentElement!;
		expect(backdropA.className).toContain('z-50');
		expect(backdropB.className).toContain('z-[60]');
		expect(backdropB.className).toContain('bg-bg/85');
		expect(backdropB.className).toContain('items-center');

		await userEvent.keyboard('{Escape}');
		await expect.element(screen.getByRole('dialog', { name: 'Modal B' })).not.toBeInTheDocument();
		await expect.element(screen.getByRole('dialog', { name: 'Modal A' })).toBeInTheDocument();
		expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Open B' }).element());

		await userEvent.keyboard('{Escape}');
		await expect.element(screen.getByRole('dialog', { name: 'Modal A' })).not.toBeInTheDocument();
		expect(onpageescape).not.toHaveBeenCalled();

		await userEvent.keyboard('{Escape}');
		expect(onpageescape).toHaveBeenCalledOnce();
	});
});
