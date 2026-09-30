import {GetSettings, SaveSettings} from '../../wailsjs/go/app/App';
import {detectLanguage, setLanguage} from '../i18n';

let current = {language: '', autoLockMinutes: 15};

export const getSettings = () => ({...current});

/** Loads settings; on first run picks the OS language and persists it. */
export async function loadSettings() {
    current = await GetSettings();
    if (!current.language) {
        current.language = detectLanguage();
        await SaveSettings(current);
    }
    setLanguage(current.language);
    return getSettings();
}

export async function updateSettings(patch) {
    const next = {...current, ...patch};
    await SaveSettings(next);
    current = next;
    if ('language' in patch) setLanguage(next.language);
    document.dispatchEvent(new CustomEvent('settings:change', {detail: getSettings()}));
    return getSettings();
}
