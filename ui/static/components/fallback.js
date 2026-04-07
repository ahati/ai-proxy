// fallback.js - Fallback configuration component

async function renderFallback() {
    try {
        const data = await fetchJSON('/config');
        const fb = data.schema.fallback || {};
        const providers = (data.schema.providers || []).map(p => p.name);
        const content = document.getElementById('content');

        content.innerHTML = '';
        content.appendChild(
            h('div', {},
                h('h2', { className: 'card-title', style: 'margin-bottom:1.5rem' }, 'Fallback Configuration'),
                h('div', { className: 'card' },
                    h('div', { className: 'form-group' },
                        toggle('fb-enabled', 'Enable Fallback', fb.enabled || false)
                    ),
                    h('div', { className: 'grid-2' },
                        h('div', { className: 'form-group' },
                            h('label', { for: 'fb-provider' }, 'Provider'),
                            h('select', { id: 'fb-provider' },
                                h('option', { value: '' }, 'Select provider...'),
                                ...providers.map(p =>
                                    h('option', { value: p, ...(fb.provider === p ? { selected: '' } : {}) }, p)
                                )
                            )
                        ),
                        field('Model', 'fb-model', fb.model || '', 'text', '{model} placeholder supported'),
                        h('div', { className: 'form-group' },
                            h('label', { for: 'fb-type' }, 'Type'),
                            h('select', { id: 'fb-type' },
                                h('option', { value: '', ...(!fb.type ? { selected: '' } : {}) }, '(use provider default)'),
                                h('option', { value: 'auto', ...(fb.type === 'auto' ? { selected: '' } : {}) }, 'auto'),
                                h('option', { value: 'openai', ...(fb.type === 'openai' ? { selected: '' } : {}) }, 'openai'),
                                h('option', { value: 'anthropic', ...(fb.type === 'anthropic' ? { selected: '' } : {}) }, 'anthropic')
                            )
                        ),
                        h('div', { className: 'form-group' })
                    ),
                    h('div', { style: 'display:flex;gap:0.5rem;margin-top:1rem' },
                        checkbox('fb-kimi', 'Kimi K2.5 tool call transform', fb.kimi_tool_call_transform || false),
                        checkbox('fb-glm5', 'GLM-5 XML tool call extract', fb.glm5_tool_call_transform || false),
                        checkbox('fb-reasoning', 'Reasoning split', fb.reasoning_split || false)
                    ),
                    h('div', { className: 'btn-group', style: 'margin-top:1.5rem' },
                        h('button', { className: 'btn-primary', onclick: saveFallback }, 'Save Fallback'),
                        h('button', { className: 'btn-secondary', onclick: renderFallback }, 'Reset')
                    )
                )
            )
        );
    } catch (err) {
        renderError(err.message);
    }
}

async function saveFallback() {
    // Get current config and only update the fallback section
    try {
        const data = await fetchJSON('/config');
        const schema = data.schema;
        schema.fallback = {
            enabled: document.getElementById('fb-enabled').checked,
            provider: document.getElementById('fb-provider').value,
            model: document.getElementById('fb-model').value.trim(),
            type: document.getElementById('fb-type').value,
            kimi_tool_call_transform: document.getElementById('fb-kimi').checked,
            glm5_tool_call_transform: document.getElementById('fb-glm5').checked,
            reasoning_split: document.getElementById('fb-reasoning').checked,
        };

        await fetchJSON('/config', {
            method: 'PUT',
            body: JSON.stringify(schema),
        });
        showToast('Fallback configuration saved', 'success');
        updatePersistBadge();
    } catch (err) {
        showToast(err.message, 'error');
    }
}
