// Keep Tab / Shift+Tab inside a modal container — the idiom ConfirmDialog and the picker
// modals carry inline, extracted for the Filters sheet (F73). Skips disabled and hidden
// controls so a responsive (`sm:hidden`) element is never a dead stop.
const FOCUSABLE = 'button, input, select, textarea, a[href], [tabindex="0"]';

export function trapTab(e: KeyboardEvent, container: HTMLElement | null): void {
	if (e.key !== 'Tab' || !container) return;
	const f = [...container.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(
		(el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null
	);
	if (f.length === 0) return;
	const first = f[0];
	const last = f[f.length - 1];
	if (e.shiftKey && document.activeElement === first) {
		e.preventDefault();
		last.focus();
	} else if (!e.shiftKey && document.activeElement === last) {
		e.preventDefault();
		first.focus();
	}
}

export function firstFocusable(container: HTMLElement | null): HTMLElement | undefined {
	return [...(container?.querySelectorAll<HTMLElement>(FOCUSABLE) ?? [])].find(
		(el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null
	);
}
