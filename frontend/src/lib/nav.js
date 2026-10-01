import {$$} from './ui';

// Shows one of the main views. Views listen to 'view:change' to close their
// editor panel when the user leaves them.
export function showView(name) {
    $$('.view').forEach((view) => (view.hidden = view.id !== `view-${name}`));
    $$('.nav-item[data-view]').forEach((item) => item.classList.toggle('active', item.dataset.view === name));
    document.dispatchEvent(new CustomEvent('view:change', {detail: name}));
}
