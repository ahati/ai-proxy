// websearch.js - Web search configuration component

async function renderWebSearch() {
    try {
        const data = await fetchJSON('/config');
        const ws = data.schema.websearch || {};
        const content = document.getElementById('content');

        // Determine current service status
        let statusText = 'Disabled';
        let statusClass = 'badge-warning';
        if (ws.enabled) {
            statusText = 'Enabled \u00B7 ' + (ws.provider || 'ddg');
            statusClass = 'badge-success';
        }

        content.innerHTML = '';
        content.appendChild(
            h('div', {},
                h('h2', { className: 'card-title', style: 'margin-bottom:1.5rem' }, 'Web Search Configuration'),
                h('div', { id: 'ws-status', className: 'badge ' + statusClass, style: 'margin-bottom:1rem' }, statusText),
                h('div', { className: 'card' },
                    h('div', { className: 'form-group' },
                        toggle('ws-enabled', 'Enable Web Search', ws.enabled || false)
                    ),
                    h('div', { className: 'grid-2' },
                        h('div', { className: 'form-group' },
                            h('label', { for: 'ws-provider' }, 'Provider'),
                            h('select', { id: 'ws-provider', onChange: updateProviderFields },
                                h('option', { value: 'ddg', ...(ws.provider === 'ddg' || !ws.provider ? { selected: '' } : {}) }, 'DuckDuckGo (free, no API key)'),
                                h('option', { value: 'exa', ...(ws.provider === 'exa' ? { selected: '' } : {}) }, 'Exa'),
                                h('option', { value: 'brave', ...(ws.provider === 'brave' ? { selected: '' } : {}) }, 'Brave Search')
                            )
                        ),
                        field('Max Results', 'ws-max-results', String(ws.max_results || 10), 'number', 'Default: 10'),
                        field('Timeout (seconds)', 'ws-timeout', String(ws.timeout || 30), 'number', 'Default: 30'),
                        h('div', { className: 'form-group' })
                    ),
                    h('div', { id: 'ws-api-keys', style: 'margin-top:0.5rem' },
                        ...buildProviderKeyFields(ws)
                    ),
                    h('div', { className: 'btn-group', style: 'margin-top:1.5rem' },
                        h('button', { className: 'btn-primary', onClick: saveWebSearch }, 'Save & Apply'),
                        h('button', { className: 'btn-secondary', onClick: renderWebSearch }, 'Reset')
                    )
                )
            )
        );
    } catch (err) {
        renderError(err.message);
    }
}

/**
 * Builds API key input fields based on the selected provider.
 * Exa needs an API key; Brave needs an API key; DDG needs none.
 * @param {object} ws - Current websearch config.
 * @return {HTMLElement[]} Array of form group elements.
 */
function buildProviderKeyFields(ws) {
    const provider = document.getElementById('ws-provider');
    const selected = provider ? provider.value : (ws.provider || 'ddg');
    const fields = [];

    if (selected === 'exa') {
        fields.push(
            h('div', { className: 'grid-2' },
                field('Exa API Key', 'ws-exa-key', ws.exa_api_key || '', 'text', 'Enter your Exa API key'),
                h('div', { className: 'form-group' })
            )
        );
    }
    if (selected === 'brave') {
        fields.push(
            h('div', { className: 'grid-2' },
                field('Brave API Key', 'ws-brave-key', ws.brave_api_key || '', 'text', 'Enter your Brave Search API key'),
                h('div', { className: 'form-group' })
            )
        );
    }
    return fields;
}

/**
 * Updates the API key fields section when the provider dropdown changes.
 */
function updateProviderFields() {
    const container = document.getElementById('ws-api-keys');
    if (!container) return;

    // Preserve current input values before rebuild
    const exaKey = document.getElementById('ws-exa-key');
    const braveKey = document.getElementById('ws-brave-key');
    const savedExa = exaKey ? exaKey.value : '';
    const savedBrave = braveKey ? braveKey.value : '';

    const ws = { exa_api_key: savedExa, brave_api_key: savedBrave };
    container.replaceChildren(...buildProviderKeyFields(ws));
}

/**
 * Saves the websearch configuration and hot-reloads the service.
 */
async function saveWebSearch() {
    try {
        // Read current config to merge only the websearch section
        const data = await fetchJSON('/config');
        const schema = data.schema;

        const provider = document.getElementById('ws-provider').value;
        const exaKey = document.getElementById('ws-exa-key');
        const braveKey = document.getElementById('ws-brave-key');

        schema.websearch = {
            enabled: document.getElementById('ws-enabled').checked,
            provider: provider,
            exa_api_key: exaKey ? exaKey.value.trim() : '',
            brave_api_key: braveKey ? braveKey.value.trim() : '',
            max_results: parseInt(document.getElementById('ws-max-results').value) || 10,
            timeout: parseInt(document.getElementById('ws-timeout').value) || 30,
        };

        // Save config
        await fetchJSON('/config', {
            method: 'PUT',
            body: JSON.stringify(schema),
        });

        // Hot-reload the service
        try {
            await fetchJSON('/config/websearch/reload', { method: 'POST' });
        } catch (reloadErr) {
            showToast('Config saved but service reload failed: ' + reloadErr.message, 'error');
            updatePersistBadge();
            return;
        }

        showToast('Web search configuration saved and applied', 'success');
        updatePersistBadge();
        renderWebSearch();
    } catch (err) {
        showToast(err.message, 'error');
    }
}
