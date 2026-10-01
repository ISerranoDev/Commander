import en from './en';
import es from './es';

const dictionaries = {en, es};
export const LANGUAGES = [
    {code: 'en', label: 'English'},
    {code: 'es', label: 'Español'},
];

let current = 'en';

const lookup = (dict, key) => key.split('.').reduce((node, part) => node?.[part], dict);

/** Translates key, replacing {placeholders} with params. Falls back to English. */
export function t(key, params = {}) {
    const text = lookup(dictionaries[current], key) ?? lookup(en, key) ?? key;
    return text.replace(/\{(\w+)}/g, (_, name) => params[name] ?? '');
}

export const getLanguage = () => current;

export function detectLanguage() {
    const preferred = (navigator.languages ?? [navigator.language]).map((l) => l.slice(0, 2));
    return preferred.find((code) => code in dictionaries) ?? 'en';
}

/** Switches language, re-translates static markup and notifies views. */
export function setLanguage(code) {
    current = code in dictionaries ? code : 'en';
    document.documentElement.lang = current;
    applyTranslations();
    document.dispatchEvent(new CustomEvent('i18n:change'));
}

/**
 * Static markup is translated through attributes:
 *   data-i18n="key"             -> textContent
 *   data-i18n-placeholder="key" -> placeholder
 *   data-i18n-title="key"       -> title
 */
export function applyTranslations(root = document) {
    root.querySelectorAll('[data-i18n]').forEach((el) => (el.textContent = t(el.dataset.i18n)));
    root.querySelectorAll('[data-i18n-placeholder]').forEach((el) => (el.placeholder = t(el.dataset.i18nPlaceholder)));
    root.querySelectorAll('[data-i18n-title]').forEach((el) => (el.title = t(el.dataset.i18nTitle)));
}

// Go returns English error strings; map the known ones to translation keys.
const backendErrors = {
    'wrong password or corrupted file': 'wrongPassword',
    'vault is locked': 'locked',
    'vault already exists': 'exists',
    'host not found': 'hostNotFound',
    'group not found': 'groupNotFound',
    'project not found': 'projectNotFound',
    'project name is required': 'projectNameRequired',
    'group name is required': 'groupNameRequired',
    'password must be at least 8 characters': 'weakPassword',
    'name is required': 'nameRequired',
    'address is required': 'addressRequired',
    'port must be between 1 and 65535': 'portRange',
    'username is required': 'usernameRequired',
    'private key is required': 'keyRequired',
    'invalid authentication method': 'invalidAuth',
    'invalid private key': 'invalidKey',
    'this private key requires a passphrase': 'passphraseNeeded',
    'wrong private key passphrase': 'wrongPassphrase',
    'file is too large to be a private key': 'keyTooLarge',
    'unsupported language': 'invalidLanguage',
    'unsupported auto-lock interval': 'invalidAutoLock',
    'host key rejected': 'hostKeyRejected',
    'authentication failed': 'authFailed',
    'connection timed out': 'timeout',
    'session not found': 'sessionNotFound',
    'no file chosen': 'noFileChosen',
};

// Network errors carry addresses in the message; match them by fragment.
const errorFragments = [
    ['connection refused', 'refused'],
    ['no such host', 'dns'],
    ['i/o timeout', 'timeout'],
    ['network is unreachable', 'unreachable'],
    ['no route to host', 'unreachable'],
];

function translateLine(line) {
    if (backendErrors[line]) return t(`errors.${backendErrors[line]}`);
    const fragment = errorFragments.find(([text]) => line.includes(text));
    return fragment ? t(`errors.${fragment[1]}`) : line;
}

export function translateError(err) {
    const message = typeof err === 'string' ? err : err?.message ?? String(err);
    // Validation errors arrive joined by newlines.
    return message
        .split('\n')
        .map(translateLine)
        .join('\n');
}
