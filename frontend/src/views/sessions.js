import {Terminal} from '@xterm/xterm';
import {FitAddon} from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

import {AnswerHostKey, CloseSession, Connect, ResizeSession, SendInput} from '../../wailsjs/go/app/App';
import {ClipboardGetText, ClipboardSetText, EventsOn} from '../../wailsjs/runtime/runtime';
import {applyTranslations, t, translateError} from '../i18n';
import {hueFor} from '../lib/color';
import {hydrateIcons, icon} from '../lib/icons';
import {isMac, newId, shortcutFor} from '../lib/platform';
import {$, $$, openModal} from '../lib/ui';

const STEPS = ['connecting', 'verifying', 'authenticating', 'shell'];

const TERMINAL_THEME = {
    background: '#0d0e12',
    foreground: '#e7e8ee',
    cursor: '#c9b1ff',
    cursorAccent: '#0d0e12',
    selectionBackground: 'rgba(155, 108, 255, 0.35)',
    black: '#1a1c23', brightBlack: '#5d6173',
    red: '#f06273', brightRed: '#ff8a97',
    green: '#3fcf8e', brightGreen: '#6ee7b0',
    yellow: '#f2b45a', brightYellow: '#ffd08a',
    blue: '#6c9cff', brightBlue: '#94b8ff',
    magenta: '#b18bff', brightMagenta: '#cdb3ff',
    cyan: '#4fd1d9', brightCyan: '#7fe6ec',
    white: '#c9cbd6', brightWhite: '#ffffff',
};

/**
 * A tab. Reconnecting starts a new backend session (connId) in the same tab.
 * status: connecting | connected | closed | error
 */
const tabs = new Map();
let activeTab = 'home';

// ---- Public API ---------------------------------------------------------------

/** Opens a new tab and connects to host (a HostSummary). */
export async function connectTo(host) {
    let password = '';
    if (host.authMethod === 'password' && !host.hasPassword) {
        const values = await openModal({
            title: t('session.passwordTitle', {target: `${host.username}@${host.address}`}),
            message: t('session.passwordMessage'),
            fields: [{name: 'password', label: t('host.password')}],
        });
        if (!values) return;
        password = values.password;
    }

    const tab = createTab(host, password);
    activate(tab.id);
    start(tab);
}

export function initSessions() {
    $('#tab-home').addEventListener('click', () => activate('home'));
    document.addEventListener('i18n:change', () => tabs.forEach(renderOverlay));
    window.addEventListener('resize', () => fitActive());

    document.addEventListener('keydown', (event) => {
        // Keys typed in a terminal are handled by its own key handler.
        if (event.target.closest?.('.xterm')) return;
        const action = shortcutFor(event);
        if (action && runShortcut(action)) event.preventDefault();
    });
}

/** Runs an app shortcut (see shortcutFor). Returns false if it didn't apply. */
function runShortcut(action) {
    const ids = ['home', ...tabs.keys()];
    const tab = tabs.get(activeTab);
    switch (action) {
        case 'close-tab':
            if (!tab) return false;
            closeTab(tab.id);
            return true;
        case 'next-tab':
        case 'prev-tab': {
            const step = action === 'next-tab' ? 1 : -1;
            activate(ids[(ids.indexOf(activeTab) + step + ids.length) % ids.length]);
            return true;
        }
        case 'copy':
            if (!tab?.term.hasSelection()) return false;
            ClipboardSetText(tab.term.getSelection());
            return true;
        case 'paste':
            if (!tab) return false;
            ClipboardGetText().then((text) => text && tab.term.paste(text));
            return true;
        default: {
            // tab-N: 1 is the vault, 2.. are sessions in order.
            const target = ids[Number(action.slice(4)) - 1];
            if (!target) return false;
            activate(target);
            return true;
        }
    }
}

// Terminal keys: app shortcuts win, everything else goes to the shell.
function terminalKeyHandler(tab) {
    return (event) => {
        const action = shortcutFor(event);
        if (action) {
            if (runShortcut(action)) {
                event.preventDefault();
                return false;
            }
            return true;
        }
        // Windows/Linux: Ctrl+C copies when there is a selection (like
        // Windows Terminal); without one it is still SIGINT.
        if (!isMac() && event.type === 'keydown' && event.ctrlKey && !event.shiftKey &&
            !event.altKey && event.key.toLowerCase() === 'c' && tab.term.hasSelection()) {
            ClipboardSetText(tab.term.getSelection());
            tab.term.clearSelection();
            event.preventDefault();
            return false;
        }
        return true;
    };
}

// ---- Tabs ---------------------------------------------------------------------

function createTab(host, password) {
    const id = newId();

    const tabEl = document.createElement('div');
    tabEl.className = 'tab session-tab entering';
    tabEl.setAttribute('role', 'tab');
    const dot = document.createElement('span');
    dot.className = 'tab-status';
    const title = document.createElement('span');
    title.className = 'tab-title';
    title.textContent = host.name;
    const close = document.createElement('button');
    close.className = 'tab-close';
    close.title = t('tabs.close');
    close.append(icon('x'));
    tabEl.append(dot, title, close);
    tabEl.style.setProperty('--hue', hueFor(host.name));
    tabEl.addEventListener('click', () => activate(id));
    tabEl.addEventListener('auxclick', (e) => e.button === 1 && closeTab(id));
    close.addEventListener('click', (e) => {
        e.stopPropagation();
        closeTab(id);
    });
    tabEl.addEventListener('animationend', () => tabEl.classList.remove('entering'), {once: true});
    $('#session-tabs').append(tabEl);

    const pane = $('#session-template').content.firstElementChild.cloneNode(true);
    hydrateIcons(pane);
    applyTranslations(pane);
    pane.hidden = true;
    pane.style.setProperty('--hue', hueFor(host.name));
    $('.node-letter', pane).textContent = [...host.name][0]?.toUpperCase() ?? '?';
    $('#sessions').append(pane);

    const term = new Terminal({
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, "Cascadia Mono", "Cascadia Code", Consolas, "DejaVu Sans Mono", "Ubuntu Mono", "Liberation Mono", monospace',
        fontSize: 13,
        lineHeight: 1.2,
        cursorBlink: true,
        allowProposedApi: false,
        scrollback: 10000,
        theme: TERMINAL_THEME,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open($('.terminal-host', pane));

    const tab = {id, host, password, connId: null, status: 'connecting', stage: null, error: '', term, fit, tabEl, pane, unsubscribe: []};

    term.attachCustomKeyEventHandler(terminalKeyHandler(tab));
    term.onData((data) => tab.status === 'connected' && SendInput(tab.connId, data).catch(() => {}));
    term.onResize(({cols, rows}) => tab.status === 'connected' && ResizeSession(tab.connId, cols, rows).catch(() => {}));
    new ResizeObserver(() => activeTab === id && fitTab(tab)).observe($('.terminal-host', pane));

    $$('[data-session-action]', pane).forEach((button) => button.addEventListener('click', () => {
        if (button.dataset.sessionAction === 'close') closeTab(id);
        else start(tab);
    }));

    tabs.set(id, tab);
    return tab;
}

function activate(id) {
    activeTab = tabs.has(id) ? id : 'home';
    $('#tab-home').classList.toggle('active', activeTab === 'home');
    $('#home').hidden = activeTab !== 'home';
    tabs.forEach((tab) => {
        const active = tab.id === activeTab;
        tab.tabEl.classList.toggle('active', active);
        tab.pane.hidden = !active;
    });
    const tab = tabs.get(activeTab);
    if (tab) {
        requestAnimationFrame(() => {
            fitTab(tab);
            if (tab.status === 'connected') tab.term.focus();
        });
    }
}

function closeTab(id) {
    const tab = tabs.get(id);
    if (!tab) return;
    const ids = [...tabs.keys()];
    const next = ids[ids.indexOf(id) + 1] ?? ids[ids.indexOf(id) - 1] ?? 'home';

    stop(tab);
    tabs.delete(id);
    tab.tabEl.classList.add('leaving');
    tab.tabEl.addEventListener('animationend', () => tab.tabEl.remove(), {once: true});
    tab.term.dispose();
    tab.pane.remove();
    if (activeTab === id) activate(next);
}

function fitTab(tab) {
    if (tab.pane.hidden) return;
    try {
        tab.fit.fit();
    } catch {
        // Container not laid out yet.
    }
}

function fitActive() {
    const tab = tabs.get(activeTab);
    if (tab) fitTab(tab);
}

// ---- Connection lifecycle -------------------------------------------------

function start(tab) {
    stop(tab);
    tab.connId = newId();
    tab.status = 'connecting';
    tab.stage = null;
    tab.error = '';
    tab.term.reset();
    renderOverlay(tab);

    // Subscribe before Connect so no early event is missed.
    const id = tab.connId;
    const on = (name, cb) => tab.unsubscribe.push(EventsOn(`ssh:${name}:${id}`, cb));
    on('status', (stage) => onStage(tab, stage));
    on('data', (chunk) => tab.term.write(decodeBase64(chunk)));
    on('hostkey', (prompt) => onHostKey(tab, id, prompt));
    on('closed', (reason) => onClosed(tab, id, reason));

    fitTab(tab);
    const {cols, rows} = tab.term;
    Connect(id, tab.host.id, tab.password, cols, rows).catch((err) => onClosed(tab, id, err));
}

function stop(tab) {
    tab.unsubscribe.forEach((off) => off());
    tab.unsubscribe = [];
    if (tab.connId && (tab.status === 'connecting' || tab.status === 'connected')) {
        CloseSession(tab.connId);
    }
}

function onStage(tab, stage) {
    if (stage === 'ready') {
        tab.status = 'connected';
        renderOverlay(tab);
        fitTab(tab);
        ResizeSession(tab.connId, tab.term.cols, tab.term.rows).catch(() => {});
        if (activeTab === tab.id) tab.term.focus();
        return;
    }
    tab.stage = stage;
    renderOverlay(tab);
}

async function onHostKey(tab, connId, prompt) {
    activate(tab.id);
    const params = {host: prompt.host, type: prompt.keyType.toUpperCase(), fingerprint: prompt.fingerprint};
    const accepted = await openModal({
        title: t(prompt.changed ? 'hostkey.changedTitle' : 'hostkey.unknownTitle'),
        message: t(prompt.changed ? 'hostkey.changedMessage' : 'hostkey.unknownMessage', params),
        okLabel: t(prompt.changed ? 'hostkey.trustAnyway' : 'hostkey.trust'),
        danger: prompt.changed,
    });
    if (tab.connId === connId) AnswerHostKey(connId, Boolean(accepted)).catch(() => {});
}

function onClosed(tab, connId, reason) {
    if (tab.connId !== connId || !tabs.has(tab.id)) return;
    tab.unsubscribe.forEach((off) => off());
    tab.unsubscribe = [];
    const wasConnected = tab.status === 'connected';
    tab.error = reason ? translateError(reason) : '';
    tab.status = wasConnected ? 'closed' : 'error';
    renderOverlay(tab);
}

// ---- Rendering --------------------------------------------------------------

function renderOverlay(tab) {
    const overlay = $('.connect-overlay', tab.pane);
    const name = tab.host.name;

    tab.tabEl.dataset.status = tab.status;
    overlay.dataset.state = tab.status;

    $('.connect-title', overlay).textContent =
        tab.status === 'error' ? t('session.failed', {name})
            : tab.status === 'closed' ? t('session.closed', {name})
                : t('session.connectingTo', {name});
    const {host} = tab;
    $('.connect-target', overlay).textContent = `${host.username}@${host.address}${host.port === 22 ? '' : `:${host.port}`}`;
    $('.connect-error-message', overlay).textContent = tab.error;
    $('.connect-error-message', overlay).hidden = !tab.error;
    $('.retry-label', overlay).textContent = t(tab.status === 'closed' ? 'session.reconnect' : 'session.retry');

    const current = STEPS.indexOf(tab.stage);
    $$('.connect-steps li', overlay).forEach((li, i) => {
        li.classList.toggle('done', i < current || tab.status === 'connected');
        li.classList.toggle('active', i === current && tab.status === 'connecting');
        li.classList.toggle('failed', i === current && tab.status === 'error');
    });
}

function decodeBase64(chunk) {
    const binary = atob(chunk);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return bytes;
}
