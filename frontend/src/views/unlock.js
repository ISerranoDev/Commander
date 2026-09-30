import {CreateVault, DeleteLegacyFile, Unlock} from '../../wailsjs/go/app/App';
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime';
import {t} from '../i18n';
import {$, $$, confirmDanger, notify, notifyError} from '../lib/ui';

let status = null;
let onUnlocked = () => {};

/** Shows the create/unlock screen for the given vault status. */
export function showUnlock(vaultStatus, callback) {
    status = vaultStatus;
    onUnlocked = callback;
    $('#unlock-form').reset();
    render();
    $('#app-screen').hidden = true;
    $('#unlock-screen').hidden = false;
    $('#unlock-form').elements.password.focus();
}

function render() {
    if (!status) return;
    const creating = !status.exists;
    const form = $('#unlock-form');

    $('#unlock-title').textContent = t(creating ? 'unlock.titleCreate' : 'unlock.titleUnlock');
    $('#unlock-subtitle').textContent = t(creating ? 'unlock.subtitleCreate' : 'unlock.subtitleUnlock');
    $('#unlock-submit').textContent = t(creating ? 'unlock.create' : 'unlock.unlock');
    $$('[data-create-only]').forEach((el) => (el.hidden = !creating));
    $$('[data-legacy-only]').forEach((el) => (el.hidden = !(creating && status.legacyFile)));
    $('#legacy-path').textContent = status.legacyFile || '';
    form.elements.confirm.required = creating;
}

async function onSubmit(event) {
    event.preventDefault();
    const form = event.target;
    const {password, confirm} = Object.fromEntries(new FormData(form));
    const submit = $('#unlock-submit');
    submit.disabled = true;
    try {
        if (!status.exists) {
            if (password !== confirm) {
                notifyError(t('toast.mismatch'));
                return;
            }
            const importLegacy = Boolean(status.legacyFile) && form.elements.importLegacy.checked;
            const imported = await CreateVault(password, importLegacy);
            if (importLegacy) await offerLegacyCleanup(imported, status.legacyFile);
        } else {
            await Unlock(password);
        }
        form.reset();
        status = null;
        onUnlocked();
    } catch (err) {
        form.elements.password.select();
        notifyError(err);
    } finally {
        submit.disabled = false;
    }
}

async function offerLegacyCleanup(imported, path) {
    notify(t('unlock.legacyImported', {count: imported}));
    const remove = await confirmDanger(
        t('unlock.deleteLegacyTitle'),
        t('unlock.deleteLegacyMessage', {path}),
        t('host.delete'),
    );
    if (!remove) return;
    try {
        await DeleteLegacyFile();
        notify(t('unlock.legacyDeleted'));
    } catch (err) {
        notifyError(err);
    }
}

export function initUnlock() {
    $('#unlock-form').addEventListener('submit', onSubmit);
    // Open the author's page in the system browser, not inside the app.
    $('#author-link').addEventListener('click', (event) => {
        event.preventDefault();
        BrowserOpenURL(event.currentTarget.href);
    });
    document.addEventListener('i18n:change', render);
}
