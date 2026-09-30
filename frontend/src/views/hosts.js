import {DeleteHost, GetHost, ListHosts, ReadKeyFile, RevealSecret, SaveHost} from '../../wailsjs/go/app/App';
import {t} from '../i18n';
import {hueFor} from '../lib/color';
import {icon} from '../lib/icons';
import {$, $$, confirmDanger, notify, notifyError} from '../lib/ui';
import {connectTo} from './sessions';

let hosts = [];
// Host being edited (HostSummary) or null for a new one.
let editing = null;
// Private key chosen with the file picker in this form session.
let pickedKey = null;

// ---- List -----------------------------------------------------------------

export async function loadHosts() {
    try {
        hosts = await ListHosts();
        hosts.sort((a, b) => a.name.localeCompare(b.name));
        renderList();
    } catch (err) {
        notifyError(err);
    }
}

export function clearHosts() {
    hosts = [];
    $('#host-grid').replaceChildren();
    closePanel();
}

function renderList() {
    const term = $('#search-bar').value.trim();
    const needle = term.toLowerCase();
    const visible = hosts.filter((h) =>
        [h.name, h.address, h.username].some((v) => v.toLowerCase().includes(needle)),
    );

    $('#host-count').textContent = hosts.length || '';
    $('#empty-state').hidden = hosts.length > 0;
    $('.section-title').hidden = hosts.length === 0;
    $('#no-results').hidden = !(hosts.length && !visible.length);
    $('#no-results').textContent = t('hosts.noResults', {term});
    $('#host-grid').replaceChildren(...visible.map(hostCard));
}

// Built with DOM APIs (not innerHTML) so host names can't inject markup.
// Clicking a card connects; the pencil opens the editor.
function hostCard(host) {
    const card = document.createElement('div');
    card.className = 'host-card';
    card.tabIndex = 0;
    card.setAttribute('role', 'button');
    card.title = t('hosts.connect');
    if (editing?.id === host.id) card.classList.add('active');
    const connect = () => {
        card.classList.remove('launching');
        void card.offsetWidth; // restart the animation on repeated clicks
        card.classList.add('launching');
        connectTo(host);
    };
    card.addEventListener('click', connect);
    card.addEventListener('keydown', (e) => e.key === 'Enter' && connect());
    card.addEventListener('animationend', () => card.classList.remove('launching'));

    const avatar = document.createElement('div');
    avatar.className = 'host-avatar';
    avatar.style.setProperty('--hue', hueFor(host.name));
    avatar.textContent = [...host.name][0]?.toUpperCase() ?? '?';

    const text = document.createElement('div');
    text.className = 'host-text';
    const name = document.createElement('strong');
    name.textContent = host.name;
    const target = document.createElement('small');
    target.textContent = `${host.username}@${host.address}${host.port === 22 ? '' : `:${host.port}`}`;
    text.append(name, target);

    const edit = document.createElement('button');
    edit.type = 'button';
    edit.className = 'btn-icon host-edit';
    edit.title = t('hosts.edit');
    edit.append(icon('pencil'));
    edit.addEventListener('click', (e) => {
        e.stopPropagation();
        editHost(host.id);
    });

    card.append(avatar, text, edit, authBadge(host));
    return card;
}

function authBadge(host) {
    const hasSecret = host.authMethod === 'key' ? host.hasPrivateKey : host.hasPassword;
    const badge = icon(host.authMethod === 'key' ? 'key' : 'password', 'host-auth');
    badge.classList.toggle('dim', !hasSecret);
    badge.title = hasSecret
        ? t(host.authMethod === 'key' ? 'hosts.authKey' : 'hosts.authPassword')
        : t('hosts.noCredentials');
    return badge;
}

// ---- Editor panel ---------------------------------------------------------

const form = () => $('#host-form');
const authMethod = () => form().elements.authMethod.value;

function openPanel(host = null) {
    editing = host;
    pickedKey = null;
    const f = form();
    f.reset();
    f.elements.id.value = host?.id ?? '';
    if (host) {
        for (const field of ['name', 'address', 'port', 'username']) f.elements[field].value = host[field];
        f.elements.authMethod.value = host.authMethod;
    }
    $('.paste-key').open = false;
    $$('[data-reveal]').forEach((button) => setRevealed(button, false));
    $('#delete-host').hidden = !host;
    $('#panel-connect').hidden = !host;

    renderPanel();
    $('#host-panel').classList.add('open');
    $('#home').classList.add('panel-open');
    $('#host-panel').setAttribute('aria-hidden', 'false');
    renderList();
    f.elements.name.focus({preventScroll: true});
}

function closePanel() {
    editing = null;
    pickedKey = null;
    $('#host-panel').classList.remove('open');
    $('#home').classList.remove('panel-open');
    $('#host-panel').setAttribute('aria-hidden', 'true');
    $$('.host-card.active').forEach((el) => el.classList.remove('active'));
}

// Re-renders the dynamic parts of the panel (auth section, labels).
function renderPanel() {
    const f = form();
    const method = authMethod();
    // Stored secrets only count if the host keeps the same auth method.
    const same = editing?.authMethod === method;
    const storedPassword = same && editing?.hasPassword;
    const storedKey = same && editing?.hasPrivateKey;
    const storedPassphrase = same && editing?.hasKeyPassphrase;
    const pastedKey = f.elements.privateKey.value.trim() !== '';

    $('#panel-title').textContent = t(editing ? 'host.editTitle' : 'host.newTitle');
    $$('[data-auth]').forEach((el) => (el.hidden = el.dataset.auth !== method));

    f.elements.password.placeholder = storedPassword ? t('host.stored') : t('host.passwordPlaceholder');

    let name = t('host.keyNone');
    let detail = '';
    if (pickedKey) {
        name = pickedKey.name;
        detail = pickedKey.encrypted ? t('host.keyEncrypted') : '';
    } else if (pastedKey) {
        name = t('host.keyPasted');
    } else if (storedKey) {
        name = t('host.keyStored');
        detail = storedPassphrase ? t('host.keyEncrypted') : '';
    }
    $('#key-name').textContent = name;
    $('#key-detail').textContent = detail;
    $('#key-detail').hidden = !detail;
    $('.key-picker').classList.toggle('has-key', Boolean(pickedKey || pastedKey || storedKey));
    $('#pick-key').textContent = t(pickedKey || storedKey ? 'host.changeKey' : 'host.chooseKey');

    const keepsKey = !pickedKey && !pastedKey && storedKey;
    f.elements.keyPassphrase.placeholder = keepsKey && storedPassphrase ? t('host.stored') : '';
    f.elements.keyPassphrase.required = Boolean(pickedKey?.encrypted);
    $('#passphrase-hint').textContent = t(pickedKey?.encrypted ? 'host.keyPassphraseRequired' : 'host.keyPassphraseHint');
}

// ---- Show/hide secrets (eye button) ----------------------------------------

function setRevealed(button, revealed) {
    const input = button.previousElementSibling;
    input.type = revealed ? 'text' : 'password';
    button.replaceChildren(icon(revealed ? 'eyeOff' : 'eye'));
    button.title = t(revealed ? 'host.hide' : 'host.show');
    button.setAttribute('aria-pressed', String(revealed));
}

async function toggleReveal(button) {
    const input = button.previousElementSibling;
    const field = button.dataset.reveal;
    const revealing = input.type === 'password';
    // Stored secrets are fetched from the vault only on demand.
    const stored = editing && editing.authMethod === authMethod() &&
        (field === 'password' ? editing.hasPassword : editing.hasKeyPassphrase);
    if (revealing && !input.value && stored && !pickedKey) {
        try {
            input.value = await RevealSecret(editing.id, field);
        } catch (err) {
            notifyError(err);
            return;
        }
    }
    setRevealed(button, revealing);
}

async function editHost(id) {
    try {
        openPanel(await GetHost(id));
    } catch (err) {
        notifyError(err);
    }
}

async function pickKey() {
    try {
        const key = await ReadKeyFile();
        if (!key.name) return;
        pickedKey = key;
        form().elements.privateKey.value = '';
        form().elements.keyPassphrase.value = '';
        renderPanel();
        if (key.encrypted) form().elements.keyPassphrase.focus({preventScroll: true});
    } catch (err) {
        notifyError(err);
    }
}

async function deleteHost() {
    if (!editing) return;
    const ok = await confirmDanger(t('host.deleteTitle'), t('host.deleteMessage', {name: editing.name}), t('host.delete'));
    if (!ok) return;
    try {
        await DeleteHost(editing.id);
        closePanel();
        notify(t('host.deleted'));
        await loadHosts();
    } catch (err) {
        notifyError(err);
    }
}

async function onSubmit(event) {
    event.preventDefault();
    const data = Object.fromEntries(new FormData(form()));
    try {
        await SaveHost({
            id: data.id,
            name: data.name,
            address: data.address,
            port: Number.parseInt(data.port, 10),
            username: data.username,
            authMethod: data.authMethod,
            password: data.password ?? '',
            privateKey: pickedKey?.content ?? data.privateKey ?? '',
            keyPassphrase: data.keyPassphrase ?? '',
        });
        closePanel();
        notify(t('host.saved'));
        await loadHosts();
    } catch (err) {
        notifyError(err);
    }
}

async function connectEditing() {
    if (!editing) return;
    try {
        await connectTo(await GetHost(editing.id));
    } catch (err) {
        notifyError(err);
    }
}

export function openNewHost() {
    openPanel();
}

export function initHosts() {
    const f = form();
    f.addEventListener('submit', onSubmit);
    $$('input[name="authMethod"]', f).forEach((el) => el.addEventListener('change', renderPanel));
    f.elements.privateKey.addEventListener('input', () => {
        pickedKey = null;
        renderPanel();
    });

    $('#add-host-button').addEventListener('click', openNewHost);
    $('#empty-add-host').addEventListener('click', openNewHost);
    $('#close-panel').addEventListener('click', closePanel);
    $('#pick-key').addEventListener('click', pickKey);
    $('#delete-host').addEventListener('click', deleteHost);
    $('#panel-connect').addEventListener('click', connectEditing);
    $$('[data-reveal]').forEach((button) => {
        setRevealed(button, false);
        button.addEventListener('click', () => toggleReveal(button));
    });
    $('#search-bar').addEventListener('input', renderList);

    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape' && $('#host-panel').classList.contains('open') && !$('#modal').open) closePanel();
    });
    document.addEventListener('i18n:change', () => {
        renderList();
        renderPanel();
        $$('[data-reveal]').forEach((b) => setRevealed(b, b.previousElementSibling.type === 'text'));
    });
}
