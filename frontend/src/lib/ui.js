import {t, translateError} from '../i18n';
import {icon} from './icons';

export const $ = (selector, root = document) => root.querySelector(selector);
export const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];

// ---- Toasts ---------------------------------------------------------------

function toast(message, type) {
    const el = document.createElement('div');
    el.className = `toast toast-${type}`;
    const text = document.createElement('span');
    text.textContent = message;
    el.append(icon(type === 'error' ? 'alert' : 'check'), text);
    $('#toasts').append(el);

    requestAnimationFrame(() => el.classList.add('visible'));
    setTimeout(() => {
        el.classList.remove('visible');
        el.addEventListener('transitionend', () => el.remove(), {once: true});
    }, type === 'error' ? 6000 : 3500);
}

export const notify = (message) => toast(message, 'success');
export const notifyError = (err) => toast(translateError(err), 'error');

// ---- Modal ----------------------------------------------------------------

/**
 * Opens the shared modal. With `fields` it collects inputs (password unless
 * the field sets `type`) and resolves to {name: value}; without, it is a
 * confirmation and resolves to true. Resolves to null when cancelled.
 *
 * A field with `matches: 'other'` must equal the field named `other`.
 */
export function openModal({title, message = '', fields = [], okLabel = t('dialog.ok'), danger = false}) {
    const dialog = $('#modal');
    const form = $('#modal-form');
    const ok = $('#modal-ok');

    $('#modal-title').textContent = title;
    $('#modal-message').textContent = message;
    $('#modal-message').hidden = !message;
    ok.textContent = okLabel;
    ok.className = `btn ${danger ? 'btn-danger' : 'btn-primary'}`;
    $('#modal-cancel').textContent = t('dialog.cancel');

    $('#modal-fields').replaceChildren(...fields.map(({name, label, type = 'password', value = ''}) => {
        const wrapper = document.createElement('label');
        wrapper.className = 'field';
        const caption = document.createElement('span');
        caption.textContent = label;
        const input = document.createElement('input');
        input.type = type;
        input.name = name;
        input.value = value;
        input.required = true;
        wrapper.append(caption, input);
        return wrapper;
    }));

    return new Promise((resolve) => {
        const finish = (value) => {
            form.removeEventListener('submit', onSubmit);
            $('#modal-cancel').removeEventListener('click', onCancel);
            dialog.removeEventListener('cancel', onCancel);
            form.reset();
            dialog.close();
            resolve(value);
        };
        const onSubmit = (event) => {
            event.preventDefault();
            const values = Object.fromEntries(new FormData(form));
            if (fields.some((f) => f.matches && values[f.name] !== values[f.matches])) {
                notifyError(t('toast.mismatch'));
                return;
            }
            finish(fields.length ? values : true);
        };
        const onCancel = (event) => {
            event.preventDefault();
            finish(null);
        };

        form.addEventListener('submit', onSubmit);
        $('#modal-cancel').addEventListener('click', onCancel);
        dialog.addEventListener('cancel', onCancel);
        dialog.showModal();
        const first = $('input', dialog);
        (first ?? ok).focus();
        first?.select();
    });
}

export const confirmDanger = (title, message, okLabel) => openModal({title, message, okLabel, danger: true});
