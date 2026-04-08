// config.js - Raw JSON config editor

async function renderRawConfig() {
    try {
        const resp = await fetch('/ui/api/config/raw');
        if (!resp.ok) throw new Error('Failed to load raw config: ' + resp.statusText);
        const rawText = await resp.text();

        // Pretty-print the raw JSON from disk
        let jsonStr;
        try {
            jsonStr = JSON.stringify(JSON.parse(rawText), null, 2);
        } catch (_) {
            jsonStr = rawText; // Not valid JSON — show as-is
        }

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
                h('textarea', { className: 'config-editor', id: 'raw-config' }, jsonStr)
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
