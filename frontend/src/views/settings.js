import {ChangeMasterPassword, ChooseExportFile, ChooseImportFile, ExportHosts, ImportHosts} from '../../wailsjs/go/app/App';
import {LANGUAGES, t} from '../i18n';
import {getSettings, updateSettings} from '../lib/settings';
import {$, $$, notify, notifyError, openModal} from '../lib/ui';

const AUTO_LOCK_OPTIONS = [5, 15, 30, 60, 0];

let onHostsChanged = async () => {};

const actions = {
    // Step 1: pick where to save (native dialog). Step 2: passphrase.
    async export() {
        const file = await ChooseExportFile();
        if (!file.name) return;
        const values = await openModal({
            title: t('dialog.exportTitle'),
            message: t('dialog.exportMessage', {path: file.path}),
            fields: [
                {name: 'passphrase', label: t('dialog.passphrase')},
                {name: 'confirm', label: t('dialog.confirmPassphrase'), matches: 'passphrase'},
            ],
            okLabel: t('dialog.exportButton'),
        });
        if (!values) return;
        const path = await ExportHosts(values.passphrase);
        notify(t('toast.exported', {path}));
    },

    // Step 1: pick the backup file (native dialog). Step 2: passphrase,
    // asked again on a wrong one without choosing the file again.
    async import() {
        const file = await ChooseImportFile();
        if (!file.name) return;
        for (;;) {
            const values = await openModal({
                title: t('dialog.importTitle'),
                message: t('dialog.importMessage', {name: file.name, size: formatSize(file.size)}),
                fields: [{name: 'passphrase', label: t('dialog.passphrase')}],
                okLabel: t('dialog.importButton'),
            });
            if (!values) return;
            try {
                const count = await ImportHosts(values.passphrase);
                notify(t('toast.imported', {count}));
                await onHostsChanged();
                return;
            } catch (err) {
                notifyError(err);
                if (!String(err).includes('wrong password')) return;
            }
        }
    },

    async 'change-password'() {
        const values = await openModal({
            title: t('dialog.changeTitle'),
            fields: [
                {name: 'current', label: t('dialog.currentPassword')},
                {name: 'next', label: t('dialog.newPassword')},
                {name: 'confirm', label: t('dialog.confirmNewPassword'), matches: 'next'},
            ],
        });
        if (!values) return;
        await ChangeMasterPassword(values.current, values.next);
        notify(t('toast.passwordChanged'));
    },
};

function formatSize(bytes) {
    return bytes < 1024 * 1024 ? `${Math.max(1, Math.round(bytes / 1024))} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** Fills every language <select> (settings page and unlock screen). */
function renderLanguageSelects() {
    const {language} = getSettings();
    $$('.language-select').forEach((select) => {
        select.replaceChildren(...LANGUAGES.map(({code, label}) => new Option(label, code, false, code === language)));
    });
}

function renderAutoLock() {
    const {autoLockMinutes} = getSettings();
    $('#setting-autolock').replaceChildren(...AUTO_LOCK_OPTIONS.map((n) =>
        new Option(n ? t('settings.minutes', {n}) : t('settings.never'), n, false, n === autoLockMinutes),
    ));
}

export function renderSettings(status) {
    renderLanguageSelects();
    renderAutoLock();
    if (status) $('#setting-location').textContent = status.dataDir;
}

async function save(patch) {
    try {
        await updateSettings(patch);
    } catch (err) {
        notifyError(err);
        renderSettings();
    }
}

export function initSettings({onChanged}) {
    onHostsChanged = onChanged;

    $$('.language-select').forEach((select) =>
        select.addEventListener('change', () => save({language: select.value})),
    );
    $('#setting-autolock').addEventListener('change', (e) =>
        save({autoLockMinutes: Number(e.target.value)}),
    );
    $$('#view-settings [data-action]').forEach((button) => {
        button.addEventListener('click', async () => {
            try {
                await actions[button.dataset.action]();
            } catch (err) {
                notifyError(err);
            }
        });
    });

    document.addEventListener('i18n:change', () => renderSettings());
}
