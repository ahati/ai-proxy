// config.js - Raw JSON config editor

async function renderRawConfig() {
    try {
        // Fetch both raw config (from disk) and metadata in parallel
        const [rawText, metadata] = await Promise.all([
            fetch('/config/raw').then(r => {
                if (!r.ok) throw new Error(`Failed to load raw config: ${r.statusText}`);
                return r.text();
            }),
            fetchJSON('/config')
        ]);

        const content = document.getElementById('content');

        content.innerHTML = '';
        content.appendChild(
            h('div', {},
                h('div', { className: 'card-header', style: 'margin-bottom:1rem' },
                    h('h2', { className: 'card-title' }, 'Raw Configuration'),
                    h('div', { className: 'btn-group' },
                        h('button', { className: 'btn-secondary btn-sm', onclick: () => { navigator.clipboard.writeText(document.getElementById('raw-config').value); showToast('Copied to clipboard', 'info'); } }, 'Copy'),
                        h('button', { className: 'btn-primary btn-sm', onclick: applyRawConfig }, 'Apply')
                    )
                ),
                h('textarea', { className: 'config-editor', id: 'raw-config' }, rawText),
                h('div', { style: 'margin-top:0.75rem;display:flex;gap:1rem;align-items:center' },
                    h('span', { style: 'font-size:0.8rem;color:var(--text-secondary)' },
                        `Loaded: ${new Date(metadata.loadedAt).toLocaleString()}`,
                        metadata.persisted ? '' : ' (unsaved changes)'
                    )
                )
            )
        );
    } catch (err) {
        renderError(err.message);
    }
}

async function applyRawConfig() {
    const textarea = document.getElementById('raw-config');
    try {
        JSON.parse(textarea.value); // validate JSON
    } catch (e) {
        showToast(`Invalid JSON: ${e.message}`, 'error');
        return;
    }

    try {
        await fetchJSON('/config', {
            method: 'PUT',
            body: textarea.value,
        });
        showToast('Configuration applied', 'success');
        updatePersistBadge();
    } catch (err) {
        showToast(err.message, 'error');
    }
}
