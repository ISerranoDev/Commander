import {
    ArrangeHosts,
    DeleteGroup,
    DeleteHost,
    GetHost,
    ListGroups,
    ListHosts,
    ReadKeyFile,
    ReorderGroups,
    ListProjects,
    RevealSecret,
    SaveGroup,
    SaveHost,
} from '../../wailsjs/go/app/App';
import {t} from '../i18n';
import {hueFor} from '../lib/color';
import {icon} from '../lib/icons';
import {$, $$, confirmDanger, notify, notifyError, openModal} from '../lib/ui';
import {openProject} from './projects';
import {connectTo} from './sessions';

let hosts = [];
let groups = [];
// Project names by host ID, so searching a project finds its host.
let projectsByHost = new Map();
// Current search, lowercased.
let needle = '';
// Host being edited (HostSummary) or null for a new one.
let editing = null;
// Private key chosen with the file picker in this form session.
let pickedKey = null;
// Item being dragged: {type: 'host' | 'group', el, dropped}.
let drag = null;

// ---- List -----------------------------------------------------------------

// Hosts and groups come in display order; dragging changes it.
export async function loadHosts() {
    try {
        const [h, g, p] = await Promise.all([ListHosts(), ListGroups(), ListProjects()]);
        hosts = h ?? [];
        groups = g ?? [];
        setProjects(p ?? []);
        renderList();
    } catch (err) {
        notifyError(err);
    }
}

function setProjects(projects) {
    projectsByHost = new Map();
    for (const p of [...projects].sort((a, b) => a.name.localeCompare(b.name))) {
        if (!p.hostId) continue;
        if (!projectsByHost.has(p.hostId)) projectsByHost.set(p.hostId, []);
        projectsByHost.get(p.hostId).push(p);
    }
}

async function reloadProjects() {
    try {
        setProjects((await ListProjects()) ?? []);
        renderList();
        if (editing) renderHostProjects();
    } catch (err) {
        notifyError(err);
    }
}

const projectsOf = (host) => projectsByHost.get(host.id) ?? [];
const matchedProjects = (host) => (needle ? projectsOf(host).filter((p) => p.name.toLowerCase().includes(needle)) : []);

export function clearHosts() {
    hosts = [];
    groups = [];
    projectsByHost = new Map();
    $('#host-groups').replaceChildren();
    closePanel();
}

function renderList() {
    if (drag) return; // a reload mid-drag would drop the dragged element
    const term = $('#search-bar').value.trim();
    needle = term.toLowerCase();
    const searching = needle !== '';
    const visible = hosts.filter((h) =>
        [h.name, h.address, h.username].some((v) => v.toLowerCase().includes(needle)) || matchedProjects(h).length,
    );

    $('#host-count').textContent = hosts.length || '';
    $('#empty-state').hidden = hosts.length > 0 || groups.length > 0;
    $('#no-results').hidden = !(searching && hosts.length && !visible.length);
    $('#no-results').textContent = t('hosts.noResults', {term});

    // Each group in order, then the ungrouped hosts (also those whose group is gone).
    const sections = [];
    for (const group of groups) {
        const members = visible.filter((h) => h.groupId === group.id);
        if (searching && !members.length) continue;
        sections.push(hostSection(group, members, searching));
    }
    const known = new Set(groups.map((g) => g.id));
    const ungrouped = visible.filter((h) => !known.has(h.groupId));
    if (ungrouped.length) {
        sections.push(hostSection(null, ungrouped, searching));
    } else if (!searching && groups.length) {
        // Hidden drop target so hosts can be dragged out of every group.
        sections.push(hostSection(null, [], searching));
    }
    $('#host-groups').replaceChildren(...sections);
}

// One full-width row per group; `group` is null for the ungrouped hosts.
function hostSection(group, members, searching) {
    const section = document.createElement('section');
    section.className = 'host-section';
    section.dataset.groupId = group?.id ?? '';
    if (!group && !members.length) section.classList.add('drop-only');

    const header = document.createElement('header');
    header.className = 'section-header';
    const title = document.createElement('h2');
    title.className = 'section-title';
    title.textContent = group ? group.name : t(groups.length ? 'groups.ungrouped' : 'hosts.title');
    header.append(title);

    if (group) {
        section.classList.add('is-group');
        const count = document.createElement('span');
        count.className = 'section-count';
        count.textContent = members.length || '';

        const actions = document.createElement('div');
        actions.className = 'section-actions';
        actions.append(
            iconButton('plus', t('groups.addHost'), () => openPanel(null, group.id)),
            iconButton('pencil', t('groups.rename'), () => renameGroup(group)),
            iconButton('trash', t('groups.delete'), () => deleteGroup(group)),
        );
        header.append(count, actions);

        if (!searching) {
            const grip = icon('grip', 'section-grip');
            grip.title = t('groups.drag');
            header.prepend(grip);
            header.draggable = true;
            header.addEventListener('dragstart', (e) => startDrag(e, 'group', section));
            header.addEventListener('dragend', endDrag);
        }
    }

    const grid = document.createElement('div');
    grid.className = 'host-grid';
    grid.dataset.empty = t('groups.dropHere');
    grid.append(...members.map((h) => hostCard(h, !searching)));

    section.append(header, grid);
    return section;
}

function iconButton(name, title, onClick) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'btn-icon';
    button.title = title;
    button.append(icon(name));
    button.addEventListener('click', onClick);
    return button;
}

// ---- Drag and drop --------------------------------------------------------
//
// The dragged element is moved live in the DOM while hovering; on drop the
// DOM order is saved. A cancelled drag re-renders from the saved state.

const DRAG_TYPE = 'application/x-commander-item';

function startDrag(event, type, el) {
    event.stopPropagation();
    drag = {type, el, dropped: false};
    event.dataTransfer.effectAllowed = 'move';
    // Custom type so text fields don't accept the drop.
    event.dataTransfer.setData(DRAG_TYPE, type);
    // Deferred so the drag image is taken before the element is dimmed.
    requestAnimationFrame(() => {
        if (drag?.el !== el) return;
        el.classList.add('dragging');
        $('#host-groups').classList.add(`dragging-${type}`);
    });
}

function onDragOver(event) {
    if (!drag) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    if (drag.type === 'host') moveHost(event);
    else moveGroup(event);
}

function moveHost(event) {
    const section = event.target.closest?.('.host-section');
    if (!section) return;
    const grid = $('.host-grid', section);
    const before = [...grid.children].find((card) => {
        if (card === drag.el) return false;
        const r = card.getBoundingClientRect();
        // Cards flow in rows: anything in a lower row, or left half of this row.
        return event.clientY < r.top || (event.clientY <= r.bottom && event.clientX < r.left + r.width / 2);
    });
    if (before) {
        if (before.previousElementSibling !== drag.el) grid.insertBefore(drag.el, before);
    } else if (grid.lastElementChild !== drag.el) {
        grid.append(drag.el);
    }
}

// Groups stay above the ungrouped section, which is always last.
function moveGroup(event) {
    const container = $('#host-groups');
    const others = $$('.host-section.is-group', container).filter((s) => s !== drag.el);
    const before = others.find((s) => {
        const r = s.getBoundingClientRect();
        return event.clientY < r.top + r.height / 2;
    }) ?? $('.host-section:not(.is-group)', container);
    if (before) {
        if (before.previousElementSibling !== drag.el) container.insertBefore(drag.el, before);
    } else if (container.lastElementChild !== drag.el) {
        container.append(drag.el);
    }
}

function onDrop(event) {
    if (!drag) return;
    event.preventDefault();
    drag.dropped = true;
}

async function endDrag() {
    const current = drag;
    if (!current) return;
    drag = null;
    current.el.classList.remove('dragging');
    $('#host-groups').classList.remove('dragging-host', 'dragging-group');
    if (!current.dropped) {
        renderList();
        return;
    }
    try {
        if (current.type === 'host') {
            const layout = $$('.host-card', $('#host-groups')).map((card) => ({
                id: card.dataset.id,
                groupId: card.closest('.host-section').dataset.groupId,
            }));
            await ArrangeHosts(layout);
        } else {
            await ReorderGroups($$('.host-section.is-group').map((s) => s.dataset.groupId));
        }
    } catch (err) {
        notifyError(err);
    }
    await loadHosts();
}

// ---- Groups ---------------------------------------------------------------

async function askGroupName(title, okLabel, value = '') {
    const values = await openModal({
        title,
        okLabel,
        fields: [{name: 'name', label: t('groups.name'), type: 'text', value}],
    });
    return values?.name.trim() || null;
}

async function createGroup() {
    const name = await askGroupName(t('groups.newTitle'), t('groups.create'));
    if (!name) return;
    try {
        await SaveGroup({id: '', name});
        notify(t('groups.saved'));
        await loadHosts();
        $$('.host-section.is-group').at(-1)?.scrollIntoView({block: 'nearest', behavior: 'smooth'});
    } catch (err) {
        notifyError(err);
    }
}

async function renameGroup(group) {
    const name = await askGroupName(t('groups.renameTitle'), t('groups.rename'), group.name);
    if (!name || name === group.name) return;
    try {
        await SaveGroup({id: group.id, name});
        notify(t('groups.saved'));
        await loadHosts();
    } catch (err) {
        notifyError(err);
    }
}

async function deleteGroup(group) {
    const ok = await confirmDanger(t('groups.deleteTitle'), t('groups.deleteMessage', {name: group.name}), t('groups.deleteOk'));
    if (!ok) return;
    try {
        await DeleteGroup(group.id);
        notify(t('groups.deleted'));
        await loadHosts();
    } catch (err) {
        notifyError(err);
    }
}

// Built with DOM APIs (not innerHTML) so host names can't inject markup.
// Clicking a card connects; the pencil opens the editor; dragging moves it.
function hostCard(host, draggable) {
    const card = document.createElement('div');
    card.className = 'host-card';
    card.dataset.id = host.id;
    if (draggable) {
        card.draggable = true;
        card.addEventListener('dragstart', (e) => startDrag(e, 'host', card));
        card.addEventListener('dragend', endDrag);
    }
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
    const matches = matchedProjects(host);
    if (matches.length) {
        const match = document.createElement('small');
        match.className = 'host-project-match';
        match.textContent = t('hosts.projectMatch', {names: matches.map((p) => p.name).join(', ')});
        text.append(match);
    }

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

// groupId preselects the group of a new host.
function openPanel(host = null, groupId = '') {
    editing = host;
    pickedKey = null;
    const f = form();
    f.reset();
    f.elements.id.value = host?.id ?? '';
    fillGroupSelect(host ? host.groupId : groupId);
    if (host) {
        for (const field of ['name', 'address', 'port', 'username']) f.elements[field].value = host[field];
        f.elements.authMethod.value = host.authMethod;
    }
    $('.paste-key').open = false;
    $$('[data-reveal]').forEach((button) => setRevealed(button, false));
    $('#delete-host').hidden = !host;
    renderHostProjects();
    $('#panel-connect').hidden = !host;

    renderPanel();
    $('#host-panel').classList.add('open');
    $('#home').classList.add('panel-open');
    $('#host-panel').setAttribute('aria-hidden', 'false');
    renderList();
    f.elements.name.focus({preventScroll: true});
}

function fillGroupSelect(selected) {
    const select = form().elements.groupId;
    const option = (value, label) => {
        const el = document.createElement('option');
        el.value = value;
        el.textContent = label;
        return el;
    };
    select.replaceChildren(option('', t('groups.none')), ...groups.map((g) => option(g.id, g.name)));
    select.value = groups.some((g) => g.id === selected) ? selected : '';
}

// Projects assigned to the host being edited, each opening its editor.
function renderHostProjects() {
    $('#host-projects-section').hidden = !editing;
    if (!editing) return;
    const list = projectsByHost.get(editing.id) ?? [];
    $('#host-projects').replaceChildren(...list.map((project) => {
        const item = document.createElement('button');
        item.type = 'button';
        item.className = 'host-project';
        const name = document.createElement('span');
        name.textContent = project.name;
        item.append(icon('briefcase'), name);
        if (project.location) {
            const location = document.createElement('small');
            location.textContent = project.location;
            item.append(location);
        }
        item.addEventListener('click', () => openProject(project.id));
        return item;
    }));
}

function closePanel() {
    editing = null;
    pickedKey = null;
    $('#host-panel').classList.remove('open');
    if (!$('#project-panel').classList.contains('open')) $('#home').classList.remove('panel-open');
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
    const count = projectsOf(editing).length;
    const message = t('host.deleteMessage', {name: editing.name}) + (count ? t('host.deleteProjects', {count}) : '');
    const ok = await confirmDanger(t('host.deleteTitle'), message, t('host.delete'));
    if (!ok) return;
    try {
        await DeleteHost(editing.id);
        closePanel();
        notify(t('host.deleted'));
        await loadHosts();
        document.dispatchEvent(new CustomEvent('hosts:change'));
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
            groupId: data.groupId ?? '',
            password: data.password ?? '',
            privateKey: pickedKey?.content ?? data.privateKey ?? '',
            keyPassphrase: data.keyPassphrase ?? '',
        });
        closePanel();
        notify(t('host.saved'));
        await loadHosts();
        document.dispatchEvent(new CustomEvent('hosts:change'));
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

function initDragAndDrop() {
    const container = $('#host-groups');
    container.addEventListener('dragover', onDragOver);
    container.addEventListener('drop', onDrop);
    // Dropped anywhere else: ignore it (endDrag then reverts).
    document.addEventListener('drop', (e) => drag && e.preventDefault());
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
    $('#add-group-button').addEventListener('click', createGroup);
    initDragAndDrop();
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
    $('#host-add-project').addEventListener('click', () => editing && openProject(null, editing.id));
    document.addEventListener('projects:change', reloadProjects);
    document.addEventListener('view:change', (event) => event.detail !== 'hosts' && closePanel());

    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape' && $('#host-panel').classList.contains('open') && !$('#modal').open) closePanel();
    });
    document.addEventListener('i18n:change', () => {
        renderList();
        if ($('#host-panel').classList.contains('open')) fillGroupSelect(form().elements.groupId.value);
        renderPanel();
        $$('[data-reveal]').forEach((b) => setRevealed(b, b.previousElementSibling.type === 'text'));
    });
}
