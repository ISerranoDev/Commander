import {DeleteProject, GetProject, ListHosts, ListProjects, RevealCredential, SaveProject} from '../../wailsjs/go/app/App';
import {ClipboardSetText} from '../../wailsjs/runtime/runtime';
import {t} from '../i18n';
import {hueFor} from '../lib/color';
import {icon} from '../lib/icons';
import {showView} from '../lib/nav';
import {$, $$, confirmDanger, notify, notifyError} from '../lib/ui';

let projects = [];
let hosts = [];
// Project being edited (ProjectSummary) or null for a new one.
let editing = null;

// ---- List -----------------------------------------------------------------

export async function loadProjects() {
    try {
        const [p, h] = await Promise.all([ListProjects(), ListHosts()]);
        projects = (p ?? []).sort((a, b) => a.name.localeCompare(b.name));
        hosts = h ?? [];
        renderList();
    } catch (err) {
        notifyError(err);
    }
}

export function clearProjects() {
    projects = [];
    hosts = [];
    $('#project-grid').replaceChildren();
    closePanel();
}

const hostOf = (project) => hosts.find((h) => h.id === project.hostId);

function renderList() {
    const term = $('#project-search').value.trim();
    const needle = term.toLowerCase();
    const visible = projects.filter((p) =>
        [p.name, p.location, hostOf(p)?.name ?? '', ...p.credentials.map((c) => c.name)]
            .some((v) => v.toLowerCase().includes(needle)),
    );

    $('#project-count').textContent = projects.length || '';
    $('#project-empty').hidden = projects.length > 0;
    $('#project-title').hidden = projects.length === 0;
    $('#project-no-results').hidden = !(projects.length && !visible.length);
    $('#project-no-results').textContent = t('projects.noResults', {term});
    $('#project-grid').replaceChildren(...visible.map(projectCard));
}

// Built with DOM APIs (not innerHTML) so names can't inject markup.
function projectCard(project) {
    const card = document.createElement('div');
    card.className = 'project-card';
    card.tabIndex = 0;
    card.setAttribute('role', 'button');
    if (editing?.id === project.id) card.classList.add('active');
    const open = () => editProject(project.id);
    card.addEventListener('click', open);
    card.addEventListener('keydown', (e) => e.key === 'Enter' && open());

    const head = document.createElement('div');
    head.className = 'project-head';
    const name = document.createElement('strong');
    name.textContent = project.name;
    head.append(icon('briefcase', 'project-icon'), name);

    const host = hostOf(project);
    const hostLine = document.createElement('div');
    hostLine.className = 'project-host';
    if (host) {
        const dot = document.createElement('span');
        dot.className = 'project-host-dot';
        dot.style.setProperty('--hue', hueFor(host.name));
        const label = document.createElement('span');
        label.textContent = host.name;
        hostLine.append(dot, label);
    } else {
        hostLine.classList.add('none');
        hostLine.textContent = t('projects.noHost');
    }

    card.append(head, hostLine);
    if (project.location) {
        const location = document.createElement('small');
        location.className = 'project-location';
        location.textContent = project.location;
        card.append(location);
    }
    if (project.credentials.length) {
        const creds = document.createElement('small');
        creds.className = 'project-creds';
        creds.append(icon('key'), document.createTextNode(t('projects.credentialCount', {count: project.credentials.length})));
        card.append(creds);
    }
    return card;
}

// ---- Editor panel ---------------------------------------------------------

const form = () => $('#project-form');

function fillHostSelect(selected) {
    const option = (value, label) => {
        const el = document.createElement('option');
        el.value = value;
        el.textContent = label;
        return el;
    };
    const sorted = [...hosts].sort((a, b) => a.name.localeCompare(b.name));
    const select = form().elements.hostId;
    select.replaceChildren(option('', t('project.noHost')), ...sorted.map((h) => option(h.id, h.name)));
    select.value = hosts.some((h) => h.id === selected) ? selected : '';
}

// hostId preselects the host of a new project.
function openPanel(project = null, hostId = '') {
    editing = project;
    const f = form();
    f.reset();
    f.elements.id.value = project?.id ?? '';
    f.elements.name.value = project?.name ?? '';
    f.elements.location.value = project?.location ?? '';
    f.elements.extra.value = project?.extra ?? '';
    fillHostSelect(project ? project.hostId : hostId);
    $('#credential-list').replaceChildren(...(project?.credentials ?? []).map(credentialRow));
    $('#delete-project').hidden = !project;
    $('#project-panel-title').textContent = t(project ? 'project.editTitle' : 'project.newTitle');

    $('#project-panel').classList.add('open');
    $('#home').classList.add('panel-open');
    $('#project-panel').setAttribute('aria-hidden', 'false');
    renderList();
    f.elements.name.focus({preventScroll: true});
}

function closePanel() {
    editing = null;
    $('#project-panel').classList.remove('open');
    if (!$('#host-panel').classList.contains('open')) $('#home').classList.remove('panel-open');
    $('#project-panel').setAttribute('aria-hidden', 'true');
    $$('.project-card.active').forEach((el) => el.classList.remove('active'));
}

async function editProject(id) {
    try {
        openPanel(await GetProject(id));
    } catch (err) {
        notifyError(err);
    }
}

// Opens a project (or a new one for hostId) from another view.
export async function openProject(id, hostId = '') {
    showView('projects');
    if (id) await editProject(id);
    else openPanel(null, hostId);
}

// ---- Credentials ----------------------------------------------------------

function field(labelKey, control) {
    const label = document.createElement('label');
    label.className = 'field';
    const caption = document.createElement('span');
    caption.textContent = t(labelKey);
    label.append(caption, control);
    return label;
}

function textInput(className, value = '') {
    const input = document.createElement('input');
    input.type = 'text';
    input.className = className;
    input.value = value;
    input.spellcheck = false;
    return input;
}

function actionButton(name, title, onClick) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'input-action';
    button.title = title;
    button.append(icon(name));
    button.addEventListener('click', onClick);
    return button;
}

// cred is a CredentialSummary, or undefined for a new one. Its password is
// only fetched from the vault when shown or copied.
function credentialRow(cred) {
    const row = document.createElement('div');
    row.className = 'credential';
    row.dataset.id = cred?.id ?? '';
    row.dataset.stored = String(Boolean(cred?.hasPassword));

    const name = textInput('cred-name', cred?.name);
    name.placeholder = t('project.credentialNamePlaceholder');
    name.spellcheck = true;
    const remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'btn-icon credential-remove';
    remove.title = t('project.removeCredential');
    remove.append(icon('x'));
    remove.addEventListener('click', () => row.remove());
    const head = document.createElement('div');
    head.className = 'credential-head';
    head.append(field('project.credentialName', name), remove);

    const username = textInput('cred-username', cred?.username);
    const userGroup = document.createElement('span');
    userGroup.className = 'input-group';
    userGroup.append(username, actionButton('copy', t('project.copy'), () => copy(username.value)));

    const password = document.createElement('input');
    password.type = 'password';
    password.className = 'cred-password';
    password.placeholder = cred?.hasPassword ? t('host.stored') : '';
    const reveal = actionButton('eye', t('host.show'), () => toggleReveal(row, password, reveal));
    const passGroup = document.createElement('span');
    passGroup.className = 'input-group two-actions';
    passGroup.append(password, reveal, actionButton('copy', t('project.copy'), async () => {
        const value = await passwordOf(row, password);
        if (value !== null) copy(value);
    }));

    const comments = document.createElement('textarea');
    comments.className = 'cred-comments plain';
    comments.rows = 2;
    comments.value = cred?.comments ?? '';

    row.append(head, field('project.username', userGroup), field('project.password', passGroup), field('project.comments', comments));
    return row;
}

// The typed password, or the stored one fetched from the vault.
async function passwordOf(row, input) {
    if (input.value || row.dataset.stored !== 'true' || !editing) return input.value;
    try {
        input.value = await RevealCredential(editing.id, row.dataset.id);
        return input.value;
    } catch (err) {
        notifyError(err);
        return null;
    }
}

async function toggleReveal(row, input, button) {
    const revealing = input.type === 'password';
    if (revealing && (await passwordOf(row, input)) === null) return;
    input.type = revealing ? 'text' : 'password';
    button.replaceChildren(icon(revealing ? 'eyeOff' : 'eye'));
    button.title = t(revealing ? 'host.hide' : 'host.show');
    button.setAttribute('aria-pressed', String(revealing));
}

async function copy(text) {
    if (!text) return;
    try {
        await ClipboardSetText(text);
        notify(t('project.copied'));
    } catch (err) {
        notifyError(err);
    }
}

function addCredential() {
    const row = credentialRow();
    $('#credential-list').append(row);
    $('.cred-name', row).focus();
}

// ---- Save / delete --------------------------------------------------------

async function onSubmit(event) {
    event.preventDefault();
    const f = form();
    const credentials = $$('.credential', f).map((row) => ({
        id: row.dataset.id,
        name: $('.cred-name', row).value,
        username: $('.cred-username', row).value,
        password: $('.cred-password', row).value,
        comments: $('.cred-comments', row).value,
    }));
    try {
        await SaveProject({
            id: f.elements.id.value,
            name: f.elements.name.value,
            hostId: f.elements.hostId.value,
            location: f.elements.location.value,
            credentials,
            extra: f.elements.extra.value,
        });
        closePanel();
        notify(t('project.saved'));
        await loadProjects();
        document.dispatchEvent(new CustomEvent('projects:change'));
    } catch (err) {
        notifyError(err);
    }
}

async function deleteProject() {
    if (!editing) return;
    const ok = await confirmDanger(t('project.deleteTitle'), t('project.deleteMessage', {name: editing.name}), t('project.delete'));
    if (!ok) return;
    try {
        await DeleteProject(editing.id);
        closePanel();
        notify(t('project.deleted'));
        await loadProjects();
        document.dispatchEvent(new CustomEvent('projects:change'));
    } catch (err) {
        notifyError(err);
    }
}

export function initProjects() {
    form().addEventListener('submit', onSubmit);
    $('#add-project-button').addEventListener('click', () => openPanel());
    $('#empty-add-project').addEventListener('click', () => openPanel());
    $('#close-project-panel').addEventListener('click', closePanel);
    $('#add-credential').addEventListener('click', addCredential);
    $('#delete-project').addEventListener('click', deleteProject);
    $('#project-search').addEventListener('input', renderList);

    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape' && $('#project-panel').classList.contains('open') && !$('#modal').open) closePanel();
    });
    document.addEventListener('view:change', (event) => event.detail !== 'projects' && closePanel());
    document.addEventListener('hosts:change', loadProjects);
    document.addEventListener('i18n:change', () => {
        renderList();
        if (!$('#project-panel').classList.contains('open')) return;
        $('#project-panel-title').textContent = t(editing ? 'project.editTitle' : 'project.newTitle');
        fillHostSelect(form().elements.hostId.value);
    });
}
