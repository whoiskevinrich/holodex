// Dev-only: no <video>/<audio> on the page may ever make a sound (HOLODEX-499). Agents
// drive playback in the background — a scripted `ended` hand-off, an auto-advance —
// and every one of them used to come out of the owner's speakers.
//
// Volume 0, deliberately NOT `muted`: a muted play() is exempt from the browser's
// autoplay policy, so muting would hide exactly the class of bug (an unmuted play()
// refused) that playback work needs to see. Media events don't bubble, hence capture.

function silence(e: Event) {
	const el = e.target;
	if (el instanceof HTMLMediaElement && el.volume !== 0) el.volume = 0;
}

// Covers elements in the document only; a detached `new Audio()` would escape it.
const EVENTS = ['loadstart', 'play', 'volumechange'];

export function silenceMediaInDev() {
	if (!import.meta.env.DEV) return;
	for (const type of EVENTS) document.addEventListener(type, silence, true);
	return () => {
		for (const type of EVENTS) document.removeEventListener(type, silence, true);
	};
}
