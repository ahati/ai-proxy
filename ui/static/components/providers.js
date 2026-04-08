// providers.js - Provider management component

async function renderProviders() {
    try {
        const data = await fetchJSON('/config');
        const providers = data.schema.providers || [];
        const content = document.getElementById('content');

        content.innerHTML = '';
        content.appendChild(
            h('div', {},
                h('div', { className: 'card-header', style: 'margin-bottom:1rem' },
                    h('h2', { className: 'card-title' }, 'Providers'),
                    h('button', { className: 'btn-primary btn-sm', onclick: () => showProviderForm() }, '+ Add Provider')
                ),
                h('div', { className: 'table-container' },
                    h('table', {},
                        h('thead', {},
                            h('tr', {},
                                h('th', {}, 'Name'),
                                h('th', {}, 'Protocols'),
                                h('th', {}, 'API Key'),
                                h('th', {}, 'Actions')
                            )
                        ),
                        h('tbody', {},
                            ...providers.map(p => h('tr', {},
                                h('td', {}, p.name),
                                h('td', {}, Object.keys(p.endpoints || {}).join(', ')),
                                h('td', {}, formatAPIKeyDisplay(p)),
                                h('td', {},
                                    h('button', { className: 'btn-secondary btn-sm', style: 'margin-right:0.5rem',
                                        onclick: () => showProviderForm(p) }, 'Edit'),
                                    h('button', { className: 'btn-danger btn-sm',
                                        onclick: () => deleteProvider(p.name) }, 'Delete')
                                )
                            ))
                        )
                    )
                )
            )
        );
    } catch (err) {
        renderError(err.message);
    }
}

function showProviderForm(provider = null) {
    const isNew = !provider;
    const content = document.getElementById('content');
    const overlay = h('div', { style: 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.5);z-index:50;display:flex;align-items:center;justify-content:center' });

    // For editing, fetch raw config to get exact values as written in JSON
    if (!isNew && provider) {
        fetchRawProviderConfig(provider.name).then(rawProvider => {
            renderProviderForm(overlay, rawProvider, isNew);
        }).catch(() => {
            // Fallback to using the provider object from table if raw fetch fails
            renderProviderForm(overlay, provider, isNew);
        });
    } else {
        renderProviderForm(overlay, null, isNew);
    }

    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
    document.body.appendChild(overlay);
}

async function fetchRawProviderConfig(providerName) {
    const data = await fetchJSON('/config/raw');
    const provider = data.providers.find(p => p.name === providerName);
    if (!provider) {
        throw new Error(`Provider '${providerName}' not found in raw config`);
    }
    return provider;
}

function renderProviderForm(overlay, provider, isNew) {
    const form = h('div', { className: 'card', style: 'width:500px;max-height:90vh;overflow-y:auto' },
        h('h3', { className: 'card-title', style: 'margin-bottom:1rem' }, isNew ? 'Add Provider' : `Edit: ${provider.name}`),

        field('Name', 'provider-name', provider?.name || '', 'text', isNew ? '' : 'readonly'),
        field('OpenAI Endpoint', 'provider-openai', provider?.endpoints?.openai || '', 'url', 'https://api.example.com/v1/chat/completions'),
        field('Anthropic Endpoint', 'provider-anthropic', provider?.endpoints?.anthropic || '', 'url', 'https://api.anthropic.com/v1/messages'),
        field('Default Protocol', 'provider-default', provider?.default || '', 'text', 'openai (if multiple endpoints)'),
        field('API Key', 'provider-apikey', provider?.apiKey || '', 'text', 'API key or ${ENV_VAR} syntax'),
        field('Env API Key', 'provider-envkey', provider?.envApiKey || '', 'text', 'ENV_VAR_NAME'),

        h('div', { className: 'btn-group', style: 'margin-top:1.5rem' },
            h('button', { className: 'btn-primary', onclick: () => saveProvider(isNew) }, 'Save'),
            h('button', { className: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancel')
        )
    );

    overlay.appendChild(form);
}

async function saveProvider(isNew) {
    const name = document.getElementById('provider-name').value.trim();
    const openai = document.getElementById('provider-openai').value.trim();
    const anthropic = document.getElementById('provider-anthropic').value.trim();
    const def = document.getElementById('provider-default').value.trim();
    const apikey = document.getElementById('provider-apikey').value.trim();
    const envkey = document.getElementById('provider-envkey').value.trim();

    if (!name) { showToast('Name is required', 'error'); return; }

    const endpoints = {};
    if (openai) endpoints.openai = openai;
    if (anthropic) endpoints.anthropic = anthropic;

    if (Object.keys(endpoints).length === 0) {
        showToast('At least one endpoint is required', 'error');
        return;
    }

    const provider = { name, endpoints, default: def, apiKey: apikey, envApiKey: envkey };

    try {
        await fetchJSON('/config/providers', {
            method: 'PATCH',
            body: JSON.stringify([provider]),
        });
        showToast(`Provider '${name}' ${isNew ? 'added' : 'updated'}`, 'success');
        document.querySelector('[style*="z-index:50"]')?.remove();
        updatePersistBadge();
        renderProviders();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function deleteProvider(name) {
    if (!confirm(`Delete provider '${name}'? Models using it will block the deletion.`)) return;
    try {
        await fetchJSON(`/config/providers/${encodeURIComponent(name)}`, { method: 'DELETE' });
        showToast(`Provider '${name}' deleted`, 'success');
        updatePersistBadge();
        renderProviders();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function field(label, id, value, type = 'text', placeholder = '') {
    return h('div', { className: 'form-group' },
        h('label', { for: id }, label),
        h('input', { type, id, value, placeholder })
    );
}

function formatAPIKeyDisplay(provider) {
    if (provider.apiKey) {
        // apiKey can be actual key or ${VAR} interpolation
        return provider.apiKey;
    }
    if (provider.envApiKey) {
        return `(env: ${provider.envApiKey})`;
    }
    return '(none)';
}
