import './styles/app.css';

import {Lock, Status} from '../wailsjs/go/app/App';
import {applyTranslations, t} from './i18n';
import {hydrateIcons} from './lib/icons';
import {initPlatform} from './lib/platform';
import {getSettings, loadSettings} from './lib/settings';
import {$, $$, notify} from './lib/ui';
import {clearHosts, initHosts, loadHosts} from './views/hosts';
import {initSessions} from './views/sessions';
import {initSettings, renderSettings} from './views/settings';
import {initUnlock, showUnlock} from './views/unlock';

// ---- Routing ----------------------------------------------------------------

async function route() {
    const status = await Status();
    renderSettings(status);
    if (status.unlocked) {
        await showApp();
    } else {
        stopIdleTimer();
        clearHosts();
        showUnlock(status, route);
    }
}

async function showApp() {
    $('#unlock-screen').hidden = true;
    $('#app-screen').hidden = false;
    showView('hosts');
    resetIdleTimer();
    await loadHosts();
}

function showView(name) {
    $$('.view').forEach((view) => (view.hidden = view.id !== `view-${name}`));
    $$('.nav-item[data-view]').forEach((item) => item.classList.toggle('active', item.dataset.view === name));
}

async function lock() {
    await Lock();
    await route();
    notify(t('toast.locked'));
}

// ---- Idle auto-lock -----------------------------------------------------------

let idleTimer = null;
let unlocked = false;

function resetIdleTimer() {
    unlocked = true;
    clearTimeout(idleTimer);
    const minutes = getSettings().autoLockMinutes;
    idleTimer = minutes ? setTimeout(lock, minutes * 60 * 1000) : null;
}

function stopIdleTimer() {
    unlocked = false;
    clearTimeout(idleTimer);
    idleTimer = null;
}

for (const event of ['mousemove', 'keydown', 'mousedown', 'wheel']) {
    window.addEventListener(event, () => unlocked && resetIdleTimer(), {passive: true});
}
document.addEventListener('settings:change', () => unlocked && resetIdleTimer());

// ---- Boot ---------------------------------------------------------------------

async function boot() {
    hydrateIcons();
    await initPlatform();

    initUnlock();
    initHosts();
    initSettings({onChanged: loadHosts});
    initSessions();
    $$('.nav-item[data-view]').forEach((item) => item.addEventListener('click', () => showView(item.dataset.view)));
    $('#lock-button').addEventListener('click', lock);

    await loadSettings();
    applyTranslations();
    await route();
}

boot();
