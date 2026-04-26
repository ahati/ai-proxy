// fallback.js - Fallback configuration component

async function renderFallback() {
    try {
        const data = await fetchJSON('/config');
        const fb = data.schema.fallback || {};
        const providers = (data.schema.providers || []).map(p => p.name);
        const content = document.getElementById('content');

        // Helper for number input field
        const numField = (label, id, value, placeholder) =>
            h('div', { className: 'form-group' },
                h('label', { for: id, style: 'font-size:0.8rem' }, label),
                h('input', {
                    type: 'number',
                    id,
                    value: value !== undefined && value !== null ? value : '',
                    placeholder,
                    style: 'width:100%'
                })
            );

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

                    // Sampling Parameters Section (collapsible)
                    h('details', {
                        id: 'fb-sampling-details',
                        style: 'margin-top:1rem',
                        ...(fb.sampling_params ? { open: '' } : {})
                    },
                        h('summary', {
                            style: 'cursor:pointer;font-size:0.9rem;color:var(--text-secondary);margin-bottom:0.5rem;user-select:none'
                        }, 'Sampling Parameters'),
                        h('div', { style: 'padding:0.5rem 0' },
                            toggle('fb-sampling-override', 'Override client values (default: config wins)',
                                fb.sampling_params?.override !== false),
                            h('div', { className: 'grid-2', style: 'margin-top:0.75rem' },
                                numField('Temperature', 'fb-sampling-temp', fb.sampling_params?.temperature, '0-2'),
                                numField('Top P', 'fb-sampling-topp', fb.sampling_params?.top_p, '0-1'),
                                numField('Top K', 'fb-sampling-topk', fb.sampling_params?.top_k, '1-100'),
                                numField('Presence Penalty', 'fb-sampling-presence', fb.sampling_params?.presence_penalty, '-2 to 2'),
                                numField('Frequency Penalty', 'fb-sampling-freq', fb.sampling_params?.frequency_penalty, '-2 to 2')
                            )
                        )
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

// Helper to collect sampling params from fallback form
function collectFallbackSamplingParams() {
    const tempEl = document.getElementById('fb-sampling-temp');
    const topPEl = document.getElementById('fb-sampling-topp');
    const topKEl = document.getElementById('fb-sampling-topk');
    const presenceEl = document.getElementById('fb-sampling-presence');
    const freqEl = document.getElementById('fb-sampling-freq');
    const overrideEl = document.getElementById('fb-sampling-override');

    const params = {};

    if (tempEl?.value !== '') params.temperature = parseFloat(tempEl.value);
    if (topPEl?.value !== '') params.top_p = parseFloat(topPEl.value);
    if (topKEl?.value !== '') params.top_k = parseInt(topKEl.value);
    if (presenceEl?.value !== '') params.presence_penalty = parseFloat(presenceEl.value);
    if (freqEl?.value !== '') params.frequency_penalty = parseFloat(freqEl.value);

    // Only set override if false (true is default)
    if (overrideEl && !overrideEl.checked) params.override = false;

    return Object.keys(params).length > 0 ? params : undefined;
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
            sampling_params: collectFallbackSamplingParams(),
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
