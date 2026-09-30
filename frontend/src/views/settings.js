import {ChangeMasterPassword, Export, Import} from '../../wailsjs/go/app/App';
import {LANGUAGES, t} from '../i18n';
import {getSettings, updateSettings} from '../lib/settings';
import {$, $$, notify, notifyError, openModal} from '../lib/ui';

const AUTO_LOCK_OPTIONS = [5, 15, 30, 60, 0];

let onHostsChanged = async () => {};

const actions = {
    async export() {
        const values = await openModal({
            title: t('dialog.exportTitle'),
            message: t('dialog.exportMessage'),
            fields: [
                {name: 'passphrase', label: t('dialog.passphrase')},
                {name: 'confirm', label: t('dialog.confirmPassphrase'), matches: 'passphrase'},
            ],
        });
        if (!values) return;
        const path = await Export(values.passphrase);
        if (path) notify(t('toast.exported', {path}));
    },

    async import() {
        const values = await openModal({
            title: t('dialog.importTitle'),
            message: t('dialog.importMessage'),
            fields: [{name: 'passphrase', label: t('dialog.passphrase')}],
        });
        if (!values) return;
        const count = await Import(values.passphrase);
        if (count < 0) return;
        notify(t('toast.imported', {count}));
        await onHostsChanged();
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
