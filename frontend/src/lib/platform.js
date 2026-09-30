import {Environment} from '../../wailsjs/runtime/runtime';

// "darwin" | "windows" | "linux". Guessed from the user agent until the Wails
// runtime answers, so it is usable synchronously from the first render.
let platform = /Mac/i.test(navigator.platform) ? 'darwin' : /Win/i.test(navigator.platform) ? 'windows' : 'linux';

export async function initPlatform() {
    try {
        ({platform} = await Environment());
    } catch {
        // Keep the guess (e.g. running in a plain browser).
    }
    document.body.classList.add(`platform-${platform}`);
    return platform;
}

export const getPlatform = () => platform;
export const isMac = () => platform === 'darwin';

/**
 * App shortcuts. On macOS they use Cmd, which never reaches the shell. On
 * Windows/Linux Ctrl+W, Ctrl+C... are shell keys (delete word, SIGINT), so
 * the app uses Ctrl+Shift like Windows Terminal and GNOME Terminal do.
 *
 * Returns the action name or null.
 */
export function shortcutFor(event) {
    if (event.type !== 'keydown') return null;
    const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;

    if (isMac()) {
        if (!event.metaKey || event.ctrlKey || event.altKey) return null;
        if (key === 'w' && !event.shiftKey) return 'close-tab';
        if (/^[1-9]$/.test(key) && !event.shiftKey) return `tab-${key}`;
        if (key === '}' || (event.shiftKey && key === ']')) return 'next-tab';
        if (key === '{' || (event.shiftKey && key === '[')) return 'prev-tab';
        return null;
    }

    if (event.ctrlKey && event.key === 'Tab') return event.shiftKey ? 'prev-tab' : 'next-tab';
    if (!event.ctrlKey || event.metaKey || event.altKey) return null;
    if (event.shiftKey) {
        if (key === 'w') return 'close-tab';
        if (key === 'c') return 'copy';
        if (key === 'v') return 'paste';
        return null;
    }
    // Alt+digit would clash with readline; Ctrl+digit is unused by shells.
    if (/^[1-9]$/.test(key)) return `tab-${key}`;
    if (key === 'Insert') return 'copy';
    return null;
}

/** crypto.randomUUID needs a secure context, which custom schemes may not be. */
export function newId() {
    if (crypto.randomUUID) {
        try {
            return crypto.randomUUID();
        } catch {
            // fall through
        }
    }
    const b = crypto.getRandomValues(new Uint8Array(16));
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    const h = [...b].map((x) => x.toString(16).padStart(2, '0')).join('');
    return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}
