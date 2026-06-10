// Skin selection (ADR-021). The chosen skin is applied as `data-theme` on
// <html>; all visual tokens resolve from app.css. Persisted to localStorage so
// the preference survives reloads (satisfies F8.2's persistence requirement,
// generalized from dark/light to named skins).
export const THEMES = ['cinematheque', 'broadcast', 'brutalist'] as const;
export type Theme = (typeof THEMES)[number];

export const THEME_LABELS: Record<Theme, string> = {
	cinematheque: 'Cinémathèque',
	broadcast: 'Broadcast',
	brutalist: 'Brutalist'
};

const KEY = 'holodex-theme';
const DEFAULT: Theme = 'cinematheque';

function isTheme(v: unknown): v is Theme {
	return typeof v === 'string' && (THEMES as readonly string[]).includes(v);
}

class ThemeState {
	current = $state<Theme>(DEFAULT);

	// init reads the saved skin and applies it; call once on mount.
	init() {
		if (typeof localStorage !== 'undefined') {
			const saved = localStorage.getItem(KEY);
			if (isTheme(saved)) this.current = saved;
		}
		this.apply();
	}

	set(t: Theme) {
		this.current = t;
		if (typeof localStorage !== 'undefined') localStorage.setItem(KEY, t);
		this.apply();
	}

	private apply() {
		if (typeof document !== 'undefined') {
			document.documentElement.dataset.theme = this.current;
		}
	}
}

export const theme = new ThemeState();
