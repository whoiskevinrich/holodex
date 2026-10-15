// Keep Tab / Shift+Tab inside a modal container — used by the shared Modal (HOLODEX-215) and
// the Filters sheet (F73). Skips disabled, hidden and `tabindex="-1"` controls so a responsive
// (`sm:hidden`) element or a roving-tabindex row is never a dead stop.
const FOCUSABLE = 'button, input, select, textarea, summary, a[href], [tabindex]';

function focusables(container: HTMLElement | null): HTMLElement[] {
	return [...(container?.querySelectorAll<HTMLElement>(FOCUSABLE) ?? [])].filter(
		(el) => el.tabIndex >= 0 && !(el as HTMLButtonElement).disabled && el.offsetParent !== null
	);
}

export function trapTab(e: KeyboardEvent, container: HTMLElement | null): void {
	if (e.key !== 'Tab' || !container) return;
	const f = focusables(container);
	if (f.length === 0) {
		// Nothing to cycle through: keep focus on the container rather than leak out.
		e.preventDefault();
		return;
	}
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
	return focusables(container)[0];
}
