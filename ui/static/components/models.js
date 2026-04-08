// models.js - Model management component

async function renderModels() {
    try {
        const data = await fetchJSON('/config');
        const models = data.schema.models || {};
        const providers = (data.schema.providers || []).map(p => p.name);
        const entries = Object.entries(models);
        const content = document.getElementById('content');

        content.innerHTML = '';
        content.appendChild(
            h('div', {},
                h('div', { className: 'card-header', style: 'margin-bottom:1rem' },
                    h('h2', { className: 'card-title' }, `Models (${entries.length})`),
                    h('button', { className: 'btn-primary btn-sm', onclick: () => showModelForm(providers) }, '+ Add Model')
                ),
                h('div', { className: 'table-container' },
                    h('table', {},
                        h('thead', {},
                            h('tr', {},
                                h('th', {}, 'Alias'),
                                h('th', {}, 'Provider'),
                                h('th', {}, 'Upstream Model'),
                                h('th', {}, 'Type'),
                                h('th', {}, 'Transforms'),
                                h('th', {}, 'Actions')
                            )
                        ),
                        h('tbody', {},
                            ...entries.map(([name, mc]) => h('tr', {},
                                h('td', {}, name),
                                h('td', {}, mc.provider),
                                h('td', {}, mc.model),
                                h('td', {}, mc.type || 'auto'),
                                h('td', {},
                                    mc.kimi_tool_call_transform ? 'Kimi ' : '',
                                    mc.glm5_tool_call_transform ? 'GLM5 ' : '',
                                    mc.reasoning_split ? 'ReasoningSplit' : '',
                                    (!mc.kimi_tool_call_transform && !mc.glm5_tool_call_transform && !mc.reasoning_split) ? '—' : ''
                                ),
                                h('td', {},
                                    h('button', { className: 'btn-secondary btn-sm', style: 'margin-right:0.5rem',
                                        onclick: () => showModelForm(providers, name, mc) }, 'Edit'),
                                    h('button', { className: 'btn-danger btn-sm',
                                        onclick: () => deleteModel(name) }, 'Delete')
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

function showModelForm(providers, name = '', mc = null) {
    const isNew = !mc;
    const overlay = h('div', { style: 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.5);z-index:50;display:flex;align-items:center;justify-content:center' });

    const providerOptions = providers.map(p =>
        h('option', { value: p, ...(mc?.provider === p ? { selected: '' } : {}) }, p)
    );

    const form = h('div', { className: 'card', style: 'width:500px;max-height:90vh;overflow-y:auto' },
        h('h3', { className: 'card-title', style: 'margin-bottom:1rem' }, isNew ? 'Add Model' : `Edit: ${name}`),

        field('Model Alias', 'model-name', name, 'text', isNew ? 'e.g., gpt-4, claude-3' : ''),
        h('div', { className: 'form-group' },
            h('label', { for: 'model-provider' }, 'Provider'),
            h('select', { id: 'model-provider' },
                h('option', { value: '' }, 'Select provider...'),
                ...providerOptions
            )
        ),
        field('Upstream Model', 'model-upstream', mc?.model || '', 'text', 'Actual model ID on provider'),
        h('div', { className: 'form-group' },
            h('label', { for: 'model-type' }, 'Type'),
            h('select', { id: 'model-type' },
                h('option', { value: 'auto', ...(mc?.type === 'auto' || !mc?.type ? { selected: '' } : {}) }, 'auto'),
                h('option', { value: 'openai', ...(mc?.type === 'openai' ? { selected: '' } : {}) }, 'openai'),
                h('option', { value: 'anthropic', ...(mc?.type === 'anthropic' ? { selected: '' } : {}) }, 'anthropic')
            )
        ),

        h('div', { className: 'form-group' },
            h('label', { style: 'display:block;font-size:0.8rem;color:var(--text-secondary);margin-bottom:0.3rem;font-weight:500' }, 'Transforms'),
            h('div', { style: 'display:flex;flex-direction:column;gap:0.5rem;margin-top:0.5rem' },
                toggle('model-kimi', 'Kimi K2.5 tool call transform', mc?.kimi_tool_call_transform || false),
                toggle('model-glm5', 'GLM-5 XML tool call extract', mc?.glm5_tool_call_transform || false),
                toggle('model-reasoning', 'Reasoning split', mc?.reasoning_split || false)
            )
        ),

        h('div', { className: 'btn-group', style: 'margin-top:1.5rem' },
            h('button', { className: 'btn-primary', onclick: () => saveModel(isNew) }, 'Save'),
            h('button', { className: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancel')
        )
    );

    if (!isNew) document.getElementById('model-name')?.setAttribute('readonly', '');

    overlay.appendChild(form);
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
    document.body.appendChild(overlay);
}

async function saveModel(isNew) {
    const name = document.getElementById('model-name').value.trim();
    const provider = document.getElementById('model-provider').value;
    const model = document.getElementById('model-upstream').value.trim();
    const type = document.getElementById('model-type').value;
    const kimi = document.getElementById('model-kimi').checked;
    const glm5 = document.getElementById('model-glm5').checked;
    const reasoning = document.getElementById('model-reasoning').checked;

    if (!name || !provider || !model) {
        showToast('Name, provider, and upstream model are required', 'error');
        return;
    }

    const mc = {
        provider, model, type,
        kimi_tool_call_transform: kimi,
        glm5_tool_call_transform: glm5,
        reasoning_split: reasoning,
    };

    try {
        await fetchJSON('/config/models', {
            method: 'PATCH',
            body: JSON.stringify({ [name]: mc }),
        });
        showToast(`Model '${name}' ${isNew ? 'added' : 'updated'}`, 'success');
        document.querySelector('[style*="z-index:50"]')?.remove();
        updatePersistBadge();
        renderModels();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function deleteModel(name) {
    if (!confirm(`Delete model '${name}'?`)) return;
    try {
        await fetchJSON(`/config/models/${encodeURIComponent(name)}`, { method: 'DELETE' });
        showToast(`Model '${name}' deleted`, 'success');
        updatePersistBadge();
        renderModels();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function toggle(id, label, checked) {
    const stateEl = h('span', { className: 'toggle-state' }, checked ? 'ON' : 'OFF');
    const input = h('input', { type: 'checkbox', id, ...(checked ? { checked: '' } : {}) });
    // Update state text on toggle
    input.addEventListener('change', () => {
        stateEl.textContent = input.checked ? 'ON' : 'OFF';
    });
    return h('label', { className: 'toggle-row', for: id },
        input,
        h('span', { className: 'toggle-label' }, label),
        h('span', { className: 'toggle-switch' },
            stateEl,
            h('span', { className: 'toggle-slider' })
        )
    );
}

// Keep checkbox as alias for backward compat with fallback.js
function checkbox(id, label, checked) {
    return toggle(id, label, checked);
}
