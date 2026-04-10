// logs.js - In-memory logs viewer component

// Cache for parsed JSON to avoid double-parsing during hydration.
// Keyed by auto-incremented ID; cleared each time buildSection runs.
const _jsonTreeCache = new Map();
let _jsonTreeCacheCounter = 0;

let logState = {
    search: '',
    method: '',
    status: '',
    page: 1,
    perPage: 50,
    selectedIds: new Set(),
    config: { enabled: true, capacity: 2000, count: 0 }
};

// ==================== List View ====================

async function renderLogs() {
    const content = document.getElementById('content');
    content.innerHTML = '<div class="loading">Loading logs...</div>';

    try {
        logState.config = await fetchJSON('/logs/config');
    } catch (_) {
        logState.config = { enabled: false, capacity: 0, count: 0 };
    }

    if (!logState.config.enabled) {
        renderLogsDisabled(content);
        return;
    }

    renderLogsUI(content);
}

function renderLogsDisabled(content) {
    content.innerHTML = '';
    content.appendChild(
        h('div', {},
            h('h2', { className: 'card-title', style: 'margin-bottom:1.5rem' }, 'In-Memory Logs'),
            h('div', { className: 'card' },
                h('p', { style: 'color:var(--text-secondary);margin-bottom:1rem' }, 'In-memory logging is currently disabled.'),
                h('button', { className: 'btn-primary', onClick: () => enableLogs() }, 'Enable In-Memory Logging')
            )
        )
    );
}

async function enableLogs() {
    try {
        await fetchJSON('/logs/config', {
            method: 'PUT',
            body: JSON.stringify({ enabled: true, capacity: 2000 })
        });
        showToast('In-memory logging enabled', 'success');
        renderLogs();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function renderLogsUI(content) {
    const params = new URLSearchParams({
        page: logState.page,
        per_page: logState.perPage,
    });
    if (logState.search) params.set('search', logState.search);
    if (logState.method) params.set('method', logState.method);
    if (logState.status) params.set('status', logState.status);

    let data;
    try {
        data = await fetchJSON('/logs?' + params.toString());
    } catch (err) {
        renderError(err.message);
        return;
    }

    const totalPages = Math.ceil(data.total / logState.perPage) || 1;
    const hasSelection = logState.selectedIds.size > 0;

    content.innerHTML = '';
    content.appendChild(
        h('div', {},
            h('div', { style: 'display:flex;justify-content:space-between;align-items:center;margin-bottom:1rem' },
                h('h2', { className: 'card-title' }, 'In-Memory Logs'),
                h('button', { className: 'btn-secondary btn-sm', onClick: () => toggleLogSettings() }, '\u2699 Settings')
            ),
            h('div', { id: 'logs-settings', className: 'logs-settings', style: 'display:none' },
                h('div', { className: 'card' },
                    h('div', { className: 'grid-2' },
                        h('div', { className: 'form-group' },
                            h('label', {}, 'Enabled'),
                            h('label', { className: 'toggle-row' },
                                h('input', { type: 'checkbox', id: 'logs-enabled', checked: '' }),
                                h('span', { className: 'toggle-switch' },
                                    h('span', { className: 'toggle-slider' }),
                                    h('span', { className: 'toggle-state' }, 'ON')
                                )
                            )
                        ),
                        h('div', { className: 'form-group' },
                            h('label', {}, 'Capacity'),
                            h('input', { type: 'number', id: 'logs-capacity', value: String(logState.config.capacity), min: '10' })
                        )
                    ),
                    h('div', { style: 'display:flex;gap:0.5rem;margin-top:0.75rem;align-items:center' },
                        h('button', { className: 'btn-primary btn-sm', onClick: () => applyLogSettings() }, 'Apply'),
                        h('span', { id: 'logs-stats', style: 'color:var(--text-secondary);font-size:0.8rem' },
                            'Count: ' + logState.config.count + ' / Capacity: ' + logState.config.capacity
                        )
                    )
                )
            ),
            h('div', { className: 'logs-toolbar' },
                h('input', { type: 'text', id: 'logs-search', placeholder: 'Search logs...', value: logState.search, onInput: debounceLogSearch }),
                h('select', { id: 'logs-method', onChange: (e) => { logState.method = e.target.value; logState.page = 1; renderLogs(); } },
                    h('option', { value: '' }, 'Any Method'),
                    h('option', { value: 'GET', ...(logState.method === 'GET' ? { selected: '' } : {}) }, 'GET'),
                    h('option', { value: 'POST', ...(logState.method === 'POST' ? { selected: '' } : {}) }, 'POST')
                ),
                h('select', { id: 'logs-status', onChange: (e) => { logState.status = e.target.value; logState.page = 1; renderLogs(); } },
                    h('option', { value: '' }, 'Any Status'),
                    h('option', { value: '2xx', ...(logState.status === '2xx' ? { selected: '' } : {}) }, '2xx'),
                    h('option', { value: '4xx', ...(logState.status === '4xx' ? { selected: '' } : {}) }, '4xx'),
                    h('option', { value: '5xx', ...(logState.status === '5xx' ? { selected: '' } : {}) }, '5xx')
                ),
                h('button', { className: 'btn-primary btn-sm', onClick: showFlushModal },
                    'Flush to Disk' + (hasSelection ? ' (' + logState.selectedIds.size + ' selected)' : '')
                ),
                h('button', { className: 'btn-danger btn-sm', onClick: showClearConfirm }, 'Clear All')
            ),
            h('div', { className: 'table-container' },
                buildLogsTable(data.logs)
            ),
            h('div', { className: 'logs-pagination' },
                h('button', {
                    className: 'btn-secondary btn-sm',
                    ...(logState.page <= 1 ? { disabled: '' } : {}),
                    onClick: () => { if (logState.page > 1) { logState.page--; renderLogs(); } }
                }, '\u2190 Prev'),
                h('span', { style: 'color:var(--text-secondary);font-size:0.85rem' },
                    'Page ' + logState.page + ' of ' + totalPages + ' (' + data.total + ' entries)'
                ),
                h('button', {
                    className: 'btn-secondary btn-sm',
                    ...(logState.page >= totalPages ? { disabled: '' } : {}),
                    onClick: () => { if (logState.page < totalPages) { logState.page++; renderLogs(); } }
                }, 'Next \u2192')
            )
        )
    );
}

function buildLogsTable(logs) {
    if (logs.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'card';
        empty.innerHTML = '<p style="color:var(--text-secondary);text-align:center;padding:2rem">No log entries found</p>';
        return empty;
    }

    const table = document.createElement('table');
    table.className = 'logs-table';
    table.innerHTML = '<thead><tr>' +
        '<th style="width:30px"><input type="checkbox" id="select-all-logs"></th>' +
        '<th>Time</th><th>Request ID</th><th>Method</th><th>Path</th><th>Status</th><th>Duration</th><th>Client IP</th>' +
        '</tr></thead><tbody></tbody>';

    const tbody = table.querySelector('tbody');
    for (const log of logs) {
        const tr = document.createElement('tr');
        tr.style.cursor = 'pointer';

        const isChecked = logState.selectedIds.has(log.request_id);
        const status = getLogStatus(log);
        const statusStyle = status >= 400 ? 'color:var(--danger)' : status >= 200 ? 'color:var(--success)' : '';

        tr.innerHTML =
            '<td><input type="checkbox" class="log-checkbox" data-id="' + escapeHtml(log.request_id) + '" ' + (isChecked ? 'checked' : '') + '></td>' +
            '<td>' + formatLogTime(log.started_at) + '</td>' +
            '<td title="' + escapeHtml(log.request_id) + '">' + escapeHtml(log.request_id.substring(0, 12)) + '...</td>' +
            '<td><span class="badge-method">' + escapeHtml(log.method) + '</span></td>' +
            '<td>' + escapeHtml(log.path) + '</td>' +
            '<td style="' + statusStyle + '">' + (status || '\u2014') + '</td>' +
            '<td>' + (log.duration_ms ? log.duration_ms + 'ms' : '\u2014') + '</td>' +
            '<td>' + escapeHtml(log.client_ip || '\u2014') + '</td>';

        const checkbox = tr.querySelector('.log-checkbox');
        checkbox.addEventListener('click', (e) => e.stopPropagation());
        checkbox.addEventListener('change', (e) => {
            if (e.target.checked) {
                logState.selectedIds.add(log.request_id);
            } else {
                logState.selectedIds.delete(log.request_id);
            }
        });

        tr.addEventListener('click', () => {
            location.hash = '#/logs/' + log.request_id;
        });

        tbody.appendChild(tr);
    }

    const selectAll = table.querySelector('#select-all-logs');
    selectAll.addEventListener('change', (e) => {
        const checkboxes = table.querySelectorAll('.log-checkbox');
        checkboxes.forEach(cb => {
            cb.checked = e.target.checked;
            if (e.target.checked) {
                logState.selectedIds.add(cb.dataset.id);
            } else {
                logState.selectedIds.delete(cb.dataset.id);
            }
        });
    });

    return table;
}

function getLogStatus(log) {
    if (log.upstream_response && log.upstream_response.status_code) return log.upstream_response.status_code;
    if (log.downstream_response && log.downstream_response.status_code) return log.downstream_response.status_code;
    return 0;
}

function formatLogTime(isoString) {
    if (!isoString) return '\u2014';
    const d = new Date(isoString);
    const now = new Date();
    const diffMs = now - d;
    if (diffMs < 60000) return Math.floor(diffMs / 1000) + 's ago';
    if (diffMs < 3600000) return Math.floor(diffMs / 60000) + 'm ago';
    return d.toLocaleTimeString();
}

let searchTimeout;
function debounceLogSearch(e) {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
        logState.search = e.target.value;
        logState.page = 1;
        renderLogs();
    }, 300);
}

function toggleLogSettings() {
    const panel = document.getElementById('logs-settings');
    if (panel) {
        panel.style.display = panel.style.display === 'none' ? 'block' : 'none';
    }
}

async function applyLogSettings() {
    const enabledEl = document.getElementById('logs-enabled');
    const capacityEl = document.getElementById('logs-capacity');
    const enabled = enabledEl ? enabledEl.checked : true;
    const capacity = capacityEl ? parseInt(capacityEl.value) : 2000;

    try {
        await fetchJSON('/logs/config', {
            method: 'PUT',
            body: JSON.stringify({ enabled: enabled, capacity: capacity < 10 ? 10 : capacity })
        });
        showToast('Log settings updated', 'success');
        renderLogs();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

// ==================== Detail View ====================

async function renderLogDetail(requestId) {
    const content = document.getElementById('content');
    content.innerHTML = '<div class="loading">Loading log entry...</div>';

    let entry;
    try {
        entry = await fetchJSON('/logs/' + encodeURIComponent(requestId));
    } catch (err) {
        renderError(err.message);
        return;
    }

    const status = getLogStatus(entry);
    const statusLabel = status >= 200 && status < 300 ? 'Success' :
                        status >= 300 && status < 400 ? 'Redirect' :
                        status >= 400 && status < 500 ? 'Client Error' :
                        status >= 500 ? 'Server Error' : 'N/A';

    content.innerHTML = '';
    content.appendChild(
        h('div', {},
            // Back button + header
            h('div', { style: 'display:flex;align-items:center;gap:1rem;margin-bottom:1rem' },
                h('button', { className: 'btn-secondary btn-sm', onClick: () => { location.hash = '#/logs'; } }, '\u2190 Back to Logs'),
                h('h2', { className: 'card-title', style: 'margin:0' }, 'Log Detail')
            ),
            // Summary card
            h('div', { className: 'log-detail-summary card' },
                h('div', { className: 'log-detail-summary-grid' },
                    h('div', { className: 'log-detail-field' },
                        h('span', { className: 'log-detail-label' }, 'Request ID'),
                        h('span', { className: 'log-detail-value', style: 'font-family:monospace;font-size:0.8rem' }, escapeHtml(entry.request_id))
                    ),
                    h('div', { className: 'log-detail-field' },
                        h('span', { className: 'log-detail-label' }, 'Method'),
                        h('span', { className: 'log-detail-value' }, h('span', { className: 'badge-method' }, escapeHtml(entry.method)))
                    ),
                    h('div', { className: 'log-detail-field' },
                        h('span', { className: 'log-detail-label' }, 'Path'),
                        h('span', { className: 'log-detail-value', style: 'font-family:monospace;font-size:0.8rem' }, escapeHtml(entry.path))
                    ),
                    h('div', { className: 'log-detail-field' },
                        h('span', { className: 'log-detail-label' }, 'Status'),
                        h('span', { className: 'log-detail-value' },
                            h('span', { className: 'badge-status ' + (status >= 400 ? 'status-error' : status >= 200 ? 'status-ok' : 'status-unknown') },
                                (status || '\u2014') + ' ' + statusLabel
                            )
                        )
                    ),
                    h('div', { className: 'log-detail-field' },
                        h('span', { className: 'log-detail-label' }, 'Started At'),
                        h('span', { className: 'log-detail-value' }, entry.started_at ? new Date(entry.started_at).toLocaleString() : '\u2014')
                    ),
                    h('div', { className: 'log-detail-field' },
                        h('span', { className: 'log-detail-label' }, 'Duration'),
                        h('span', { className: 'log-detail-value' }, entry.duration_ms ? entry.duration_ms + ' ms' : '\u2014')
                    ),
                    h('div', { className: 'log-detail-field' },
                        h('span', { className: 'log-detail-label' }, 'Client IP'),
                        h('span', { className: 'log-detail-value', style: 'font-family:monospace;font-size:0.8rem' }, escapeHtml(entry.client_ip || '\u2014'))
                    )
                )
            ),
            // Request sections
            buildSection('Downstream Request', 'request', entry.downstream_request, false),
            buildSection('Upstream Request', 'request', entry.upstream_request, false),
            buildSection('Upstream Response', 'response', entry.upstream_response, true),
            buildSection('Downstream Response', 'response', entry.downstream_response, true)
        )
    );
}

// ==================== Section Builder ====================

function buildSection(title, type, data, hasChunks) {
    if (!data) {
        // Show empty section placeholder
        const wrapper = document.createElement('div');
        wrapper.className = 'log-section';
        wrapper.innerHTML =
            '<div class="log-section-header log-section-empty">' +
            '<span class="log-section-title">' + escapeHtml(title) + '</span>' +
            '<span class="log-section-badge log-section-badge-empty">No data</span>' +
            '</div>';
        return wrapper;
    }

    const section = document.createElement('div');
    section.className = 'log-section';

    // Build subtitle info
    let subtitle = '';
    if (type === 'response' && data.status_code) {
        subtitle = data.status_code;
    }
    if (data.chunks && data.chunks.length > 0) {
        subtitle += (subtitle ? ' \u00B7 ' : '') + data.chunks.length + ' chunks';
    }

    // Header
    const header = document.createElement('div');
    header.className = 'log-section-header';
    header.innerHTML =
        '<div class="log-section-header-left">' +
        '<span class="log-section-toggle">\u25B6</span>' +
        '<span class="log-section-title">' + escapeHtml(title) + '</span>' +
        (subtitle ? '<span class="log-section-badge">' + escapeHtml(subtitle) + '</span>' : '') +
        '</div>' +
        '<span class="log-section-chevron">\u25BC</span>';
    header.onclick = () => {
        const body = section.querySelector('.log-section-body');
        const toggle = header.querySelector('.log-section-toggle');
        const chevron = header.querySelector('.log-section-chevron');
        const isOpen = body.style.display !== 'none';
        body.style.display = isOpen ? 'none' : 'block';
        toggle.textContent = isOpen ? '\u25B6' : '\u25BC';
        chevron.textContent = isOpen ? '\u25BC' : '\u25B6';
    };

    // Body
    const body = document.createElement('div');
    body.className = 'log-section-body';
    body.style.display = 'none'; // collapsed by default

    let bodyHtml = '';

    // Headers table
    if (data.headers && Object.keys(data.headers).length > 0) {
        bodyHtml += '<div class="log-section-part">';
        bodyHtml += '<div class="log-section-part-header">Headers</div>';
        bodyHtml += '<table class="log-headers-table"><tbody>';
        for (const [k, v] of Object.entries(data.headers)) {
            bodyHtml += '<tr><td class="log-header-key">' + escapeHtml(k) + '</td><td class="log-header-val">' + escapeHtml(v) + '</td></tr>';
        }
        bodyHtml += '</tbody></table></div>';
    }

    // Body (JSON)
    if (data.body) {
        const bodyStr = typeof data.body === 'string' ? data.body : JSON.stringify(data.body, null, 2);
        let parsed;
        try { parsed = JSON.parse(bodyStr); } catch (_) { parsed = undefined; }

        bodyHtml += '<div class="log-section-part">';
        bodyHtml += '<div class="log-section-part-header">Body <button class="btn-copy" onclick="copyLogData(this)">Copy</button></div>';

        if (parsed !== undefined) {
            // Store parsed result to avoid re-parsing during hydration
            const cacheId = '__jt_' + (++_jsonTreeCacheCounter);
            _jsonTreeCache.set(cacheId, parsed);
            _jsonTreeCache.set(cacheId + '_raw', bodyStr);

            bodyHtml += '<div class="log-json-toolbar">';
            bodyHtml += '<button class="btn-tree-action" onclick="jsonTreeExpandAll(this)">Expand All</button>';
            bodyHtml += '<button class="btn-tree-action" onclick="jsonTreeCollapseAll(this)">Collapse All</button>';
            bodyHtml += '</div>';
            bodyHtml += '<div class="log-json-tree-root" data-tree-cache="' + cacheId + '"></div>';
        } else {
            // Fallback: non-JSON content renders as plain pre block
            bodyHtml += '<pre class="log-json-block" data-raw="' + escapeHtml(bodyStr) + '">' + escapeHtml(bodyStr) + '</pre>';
        }

        bodyHtml += '</div>';
    }

    // SSE Chunks
    if (hasChunks && data.chunks && data.chunks.length > 0) {
        bodyHtml += '<div class="log-section-part">';
        bodyHtml += '<div class="log-section-part-header">SSE Chunks (' + data.chunks.length + ')</div>';
        bodyHtml += '<div class="log-chunks-list">';

        for (let i = 0; i < data.chunks.length; i++) {
            const chunk = data.chunks[i];
            const chunkData = chunk.data ? (typeof chunk.data === 'string' ? chunk.data : JSON.stringify(chunk.data, null, 2)) : (chunk.raw || '');
            const isTruncated = chunkData.length > 300;

            bodyHtml += '<div class="log-chunk-item">';
            bodyHtml += '<div class="log-chunk-header" onclick="this.parentElement.classList.toggle(\'expanded\')">';
            bodyHtml += '<span class="log-chunk-num">#' + (i + 1) + '</span>';
            bodyHtml += '<span class="log-chunk-offset">' + chunk.offset_ms + 'ms</span>';
            bodyHtml += '<span class="log-chunk-event">' + escapeHtml(chunk.event || '\u2014') + '</span>';
            bodyHtml += '<span class="log-chunk-preview">' + escapeHtml(chunkData.substring(0, 80)) + (isTruncated ? '...' : '') + '</span>';
            bodyHtml += '<span class="log-chunk-expand">\u25BC</span>';
            bodyHtml += '</div>';
            bodyHtml += '<div class="log-chunk-body">';
            bodyHtml += '<pre class="log-json-block">' + syntaxHighlightJSON(chunkData) + '</pre>';
            bodyHtml += '</div></div>';
        }

        bodyHtml += '</div></div>';
    }

    body.innerHTML = bodyHtml;

    // Hydrate JSON tree placeholders into interactive collapsible trees
    body.querySelectorAll('.log-json-tree-root').forEach(root => {
        const cacheId = root.getAttribute('data-tree-cache');
        const parsed = cacheId ? _jsonTreeCache.get(cacheId) : undefined;
        if (parsed === undefined) return;

        // Collapse root by default if it has many keys/items
        const collapsed = (isObject(parsed) && Object.keys(parsed).length > 5) ||
                          (Array.isArray(parsed) && parsed.length > 5);
        root.replaceChildren(renderJSONTree(parsed, null, !collapsed));

        // Free cached value
        if (cacheId) { _jsonTreeCache.delete(cacheId); _jsonTreeCache.delete(cacheId + '_raw'); }
    });

    section.appendChild(header);
    section.appendChild(body);
    return section;
}

// ==================== Utilities ====================

function syntaxHighlightJSON(str) {
    try {
        // Try to parse and re-format
        const obj = JSON.parse(str);
        str = JSON.stringify(obj, null, 2);
    } catch (_) {
        // Not valid JSON, return as-is (escaped)
        return escapeHtml(str);
    }

    // Escape HTML first
    let escaped = escapeHtml(str);

    // Apply syntax highlighting via regex
    // JSON strings (in quotes)
    escaped = escaped.replace(/(&quot;[^&]*?&quot;)\s*:/g, '<span class="json-key">$1</span>:');
    escaped = escaped.replace(/:\s*(&quot;[^&]*?&quot;)/g, ': <span class="json-string">$1</span>');
    // Booleans and null
    escaped = escaped.replace(/:\s*(true|false|null)/g, ': <span class="json-bool">$1</span>');
    // Numbers
    escaped = escaped.replace(/:\s*(-?\d+\.?\d*)/g, ': <span class="json-number">$1</span>');

    return escaped;
}

function copyLogData(btn) {
    const container = btn.parentElement.nextElementSibling;
    if (!container) return;
    // Support both old <pre> blocks and new JSON tree roots
    const cacheId = container.getAttribute('data-tree-cache');
    const raw = (cacheId ? _jsonTreeCache.get(cacheId + '_raw') : null) || container.getAttribute('data-raw') || container.textContent;
    navigator.clipboard.writeText(raw).then(() => {
        btn.textContent = 'Copied!';
        setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
    }).catch(() => {
        showToast('Failed to copy', 'error');
    });
}

// ==================== JSON Tree Renderer ====================

/**
 * Checks if a value is a plain object (not array, not null).
 * @param {*} val - Value to check.
 * @return {boolean} True if val is a plain object.
 */
function isObject(val) {
    return val !== null && typeof val === 'object' && !Array.isArray(val);
}

/**
 * Renders a parsed JSON value as an interactive, collapsible DOM tree.
 * Objects and arrays become toggleable nodes; leaf values render inline with colors.
 *
 * @param {*} value - The parsed JSON value to render.
 * @param {string|null} keyName - Optional key name for this node (e.g., "model", "[0]").
 * @param {boolean} expanded - Whether to start expanded (true) or collapsed (false).
 * @return {HTMLElement} A DOM element representing the tree node.
 */
function renderJSONTree(value, keyName, expanded) {
    const node = document.createElement('div');
    node.className = 'json-tree-node';

    if (isObject(value) || Array.isArray(value)) {
        // Shared branch for both objects and arrays
        const isArr = Array.isArray(value);
        const entries = isArr
            ? value.map((v, i) => ['[' + i + ']', v])
            : Object.entries(value);
        const count = entries.length;
        const label = isArr ? count + ' item' + (count !== 1 ? 's' : '') : count + ' key' + (count !== 1 ? 's' : '');
        const bracket = isArr ? ['[', ']'] : ['{', '}'];

        const children = document.createElement('div');
        children.className = 'json-tree-children';
        if (!expanded) children.classList.add('json-tree-collapsed');

        const row = document.createElement('span');
        row.className = 'json-tree-row';

        if (keyName !== null) {
            row.appendChild(jsonTreeKeyEl(keyName));
            row.appendChild(jsonTreeSepEl());
        }

        const toggle = document.createElement('span');
        toggle.className = 'json-tree-toggle';
        toggle.textContent = expanded ? '\u25BC' : '\u25B6';
        row.appendChild(toggle);

        const preview = document.createElement('span');
        preview.className = 'json-tree-preview';
        preview.textContent = ' ' + bracket[0] + label + bracket[1];
        if (expanded) preview.classList.add('json-tree-collapsed');
        row.appendChild(preview);

        // Toggle click handler
        row.style.cursor = 'pointer';
        row.addEventListener('click', () => {
            const isOpen = !children.classList.contains('json-tree-collapsed');
            children.classList.toggle('json-tree-collapsed', isOpen);
            toggle.textContent = isOpen ? '\u25B6' : '\u25BC';
            preview.classList.toggle('json-tree-collapsed', !isOpen);
        });

        // Render children
        for (const [k, v] of entries) {
            children.appendChild(renderJSONTree(v, k, false));
        }

        node.appendChild(row);
        node.appendChild(children);

    } else {
        // Leaf value — render inline
        const row = document.createElement('span');
        row.className = 'json-tree-row';

        if (keyName !== null) {
            row.appendChild(jsonTreeKeyEl(keyName));
            row.appendChild(jsonTreeSepEl());
        }

        row.appendChild(jsonTreeValueEl(value));
        node.appendChild(row);
    }

    return node;
}

/**
 * Creates a styled key element for the JSON tree.
 * @param {string} key - The key name.
 * @return {HTMLElement} Styled key span.
 */
function jsonTreeKeyEl(key) {
    const el = document.createElement('span');
    el.className = 'json-tree-key';
    el.textContent = key;
    return el;
}

/**
 * Creates a separator element for the JSON tree.
 * @return {HTMLElement} Separator span.
 */
function jsonTreeSepEl() {
    const el = document.createElement('span');
    el.className = 'json-tree-separator';
    el.textContent = ': ';
    return el;
}

/**
 * Creates a styled value element for the JSON tree based on value type.
 * Strings are truncated to 300 chars with the full value in a tooltip.
 * @param {*} value - The value to render.
 * @return {HTMLElement} Styled value span.
 */
function jsonTreeValueEl(value) {
    const el = document.createElement('span');
    if (typeof value === 'string') {
        el.className = 'json-tree-value json-tree-string';
        const MAX_LEN = 300;
        if (value.length > MAX_LEN) {
            el.textContent = '"' + value.substring(0, MAX_LEN) + '..."';
            el.title = value;
        } else {
            el.textContent = '"' + value + '"';
        }
    } else if (typeof value === 'number') {
        el.className = 'json-tree-value json-tree-number';
        el.textContent = String(value);
    } else if (typeof value === 'boolean') {
        el.className = 'json-tree-value json-tree-bool';
        el.textContent = String(value);
    } else if (value === null) {
        el.className = 'json-tree-value json-tree-null';
        el.textContent = 'null';
    } else {
        el.className = 'json-tree-value';
        el.textContent = String(value);
    }
    return el;
}

/**
 * Expands all collapsible nodes in the JSON tree.
 * Bound to the "Expand All" toolbar button.
 * @param {HTMLElement} btn - The clicked button element.
 */
function jsonTreeExpandAll(btn) {
    const treeRoot = btn.closest('.log-section-part').querySelector('.log-json-tree-root');
    if (!treeRoot) return;
    treeRoot.querySelectorAll('.json-tree-children').forEach(c => { c.classList.remove('json-tree-collapsed'); });
    treeRoot.querySelectorAll('.json-tree-toggle').forEach(t => { t.textContent = '\u25BC'; });
    treeRoot.querySelectorAll('.json-tree-preview').forEach(p => { p.classList.add('json-tree-collapsed'); });
}

/**
 * Collapses all collapsible nodes in the JSON tree.
 * Bound to the "Collapse All" toolbar button.
 * @param {HTMLElement} btn - The clicked button element.
 */
function jsonTreeCollapseAll(btn) {
    const treeRoot = btn.closest('.log-section-part').querySelector('.log-json-tree-root');
    if (!treeRoot) return;
    treeRoot.querySelectorAll('.json-tree-children').forEach(c => { c.classList.add('json-tree-collapsed'); });
    treeRoot.querySelectorAll('.json-tree-toggle').forEach(t => { t.textContent = '\u25B6'; });
    treeRoot.querySelectorAll('.json-tree-preview').forEach(p => { p.classList.remove('json-tree-collapsed'); });
}

// ==================== Flush Modal ====================

function showFlushModal() {
    const hasSelection = logState.selectedIds.size > 0;
    const count = hasSelection ? logState.selectedIds.size : logState.config.count;

    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

    const dialog = document.createElement('div');
    dialog.className = 'modal-dialog';
    dialog.innerHTML =
        '<h3 class="modal-title">Flush Logs to Disk</h3>' +
        '<p style="color:var(--text-secondary);margin-bottom:1rem;font-size:0.85rem">' +
        (hasSelection ? 'Flush ' + logState.selectedIds.size + ' selected entries' : 'Flush all ' + count + ' entries') +
        '</p>' +
        '<div class="form-group"><label>Target Directory</label><input type="text" id="flush-directory" placeholder="/path/to/logs"></div>' +
        '<div class="modal-actions">' +
        '<button class="btn-secondary" onclick="this.closest(\'.modal-overlay\').remove()">Cancel</button>' +
        '<button class="btn-primary" id="flush-confirm-btn">Flush</button>' +
        '</div>';

    overlay.appendChild(dialog);
    document.body.appendChild(overlay);

    setTimeout(() => {
        const input = document.getElementById('flush-directory');
        if (input) input.focus();
    }, 100);

    document.getElementById('flush-confirm-btn').onclick = async () => {
        const dir = document.getElementById('flush-directory').value.trim();
        if (!dir) {
            showToast('Directory is required', 'error');
            return;
        }

        const btn = document.getElementById('flush-confirm-btn');
        btn.disabled = true;
        btn.textContent = 'Flushing...';

        try {
            const reqBody = { directory: dir };
            if (hasSelection) {
                reqBody.ids = Array.from(logState.selectedIds);
            }
            const result = await fetchJSON('/logs/flush', {
                method: 'POST',
                body: JSON.stringify(reqBody)
            });
            showToast(result.message, 'success');
            overlay.remove();
        } catch (err) {
            showToast(err.message, 'error');
            btn.disabled = false;
            btn.textContent = 'Flush';
        }
    };
}

// ==================== Clear Confirm ====================

function showClearConfirm() {
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

    const dialog = document.createElement('div');
    dialog.className = 'modal-dialog';
    dialog.innerHTML =
        '<h3 class="modal-title">Clear All Logs</h3>' +
        '<p style="color:var(--text-secondary);margin-bottom:1rem;font-size:0.85rem">' +
        'Clear all ' + logState.config.count + ' log entries? This cannot be undone. Entries will NOT be written to disk.' +
        '</p>' +
        '<div class="modal-actions">' +
        '<button class="btn-secondary" onclick="this.closest(\'.modal-overlay\').remove()">Cancel</button>' +
        '<button class="btn-danger" id="clear-confirm-btn">Clear All</button>' +
        '</div>';

    overlay.appendChild(dialog);
    document.body.appendChild(overlay);

    document.getElementById('clear-confirm-btn').onclick = async () => {
        const btn = document.getElementById('clear-confirm-btn');
        btn.disabled = true;
        try {
            const result = await fetchJSON('/logs', { method: 'DELETE' });
            showToast(result.message, 'success');
            logState.selectedIds.clear();
            overlay.remove();
            renderLogs();
        } catch (err) {
            showToast(err.message, 'error');
            btn.disabled = false;
        }
    };
}
