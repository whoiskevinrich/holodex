import { toMessage } from '$lib/format';

export interface PopoverMenuOptions {
	/** Extra reset to run alongside the built-in value/error reset when a menu opens. */
	onOpen?: () => void;
	/** Extra reset to run alongside the built-in action reset when a menu closes. */
	onClose?: () => void;
}

/**
 * "Which popover is open, by id" + its inline value/busy/error form slot — the state
 * machine shared by /tags' per-pill ⋯ menu (tag: rename/alias/parent/merge) and its
 * reduced category counterpart (rename/delete only, HOLODEX-240 §2). The menu
 * *contents* stay page-local (they genuinely differ); this only owns which id is
 * open, the current sub-view (`action`), and the inline rename/alias form's
 * value/busy/error plumbing, which are 100% identical in shape between the two.
 *
 * Methods are arrow fields (not `this`-bound at call time) because they're handed
 * out as bare callbacks — e.g. `use:dismissable`'s `onclose`.
 */
export class PopoverMenu<TAction extends string = 'menu'> {
	openId = $state<number | null>(null);
	action = $state<TAction>('menu' as TAction);
	triggers = $state<Record<number, HTMLButtonElement | null>>({});
	value = $state('');
	busy = $state(false);
	error = $state('');

	constructor(private opts: PopoverMenuOptions = {}) {}

	isOpen = (id: number) => this.openId === id;

	open = async (id: number, focus?: () => void) => {
		this.openId = id;
		this.action = 'menu' as TAction;
		this.value = '';
		this.error = '';
		this.opts.onOpen?.();
		if (focus) {
			await Promise.resolve();
			focus();
		}
	};

	close = (returnFocus = true) => {
		const id = this.openId;
		this.openId = null;
		this.action = 'menu' as TAction;
		this.opts.onClose?.();
		if (returnFocus && id != null) this.triggers[id]?.focus();
	};

	toggle = (id: number, focus?: () => void) => {
		if (this.openId === id) this.close();
		else this.open(id, focus);
	};

	/** Busy/error-guarded async action with no form value involved (merge/parent/…). */
	run = async (fn: () => Promise<void>, mapError?: (err: unknown) => string) => {
		if (this.busy) return;
		this.busy = true;
		this.error = '';
		try {
			await fn();
		} catch (err) {
			this.error = mapError ? mapError(err) : toMessage(err);
		} finally {
			this.busy = false;
		}
	};

	/** `run`, plus the trim-and-guard-on-empty step shared by every inline rename/alias form. */
	submit = (fn: (value: string) => Promise<void>, mapError?: (err: unknown, value: string) => string) => {
		const trimmed = this.value.trim();
		if (!trimmed) return;
		return this.run(() => fn(trimmed), mapError && ((err) => mapError(err, trimmed)));
	};
}
