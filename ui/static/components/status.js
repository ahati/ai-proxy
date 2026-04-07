// status.js - Status page component

async function renderStatus() {
    try {
        const data = await fetchJSON('/status');
        const content = document.getElementById('content');

        content.innerHTML = '';
        content.appendChild(
            h('div', {},
                h('h2', { className: 'card-title', style: 'margin-bottom:1.5rem' },
                    h('span', { className: `status-dot ${data.status === 'healthy' ? 'healthy' : 'warning'}` }),
                    'Server Status'
                ),
                h('div', { className: 'stats' },
                    statCard(data.uptime, 'Uptime'),
                    statCard(data.providers, 'Providers'),
                    statCard(data.models, 'Models'),
                    statCard(data.status, 'Status')
                ),
                h('div', { className: 'card', style: 'margin-top:1.5rem' },
                    h('h3', { className: 'card-title', style: 'margin-bottom:0.75rem' }, 'Configuration'),
                    infoRow('Config File', data.config.configFile || '(none)'),
                    infoRow('Last Loaded', data.config.loadedAt ? new Date(data.config.loadedAt).toLocaleString() : 'N/A'),
                    infoRow('Persisted', data.config.persisted ? 'Yes' : 'No (in-memory only)'),
                    infoRow('Started At', new Date(data.startedAt).toLocaleString())
                )
            )
        );
    } catch (err) {
        renderError(err.message);
    }
}

function statCard(value, label) {
    return h('div', { className: 'stat' },
        h('div', { className: 'stat-value' }, String(value)),
        h('div', { className: 'stat-label' }, label)
    );
}

function infoRow(label, value) {
    return h('div', { style: 'display:flex;justify-content:space-between;padding:0.4rem 0;border-bottom:1px solid var(--border)' },
        h('span', { style: 'color:var(--text-secondary);font-size:0.85rem' }, label),
        h('span', { style: 'font-size:0.85rem' }, value)
    );
}
