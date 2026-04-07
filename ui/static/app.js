// app.js - Main application entry point, SPA router, and shared state

const app = {
    config: null,
};

/**
 * Reload configuration from disk.
 */
async function reloadConfig() {
    try {
        await fetchJSON('/config/reload', { method: 'POST' });
        showToast('Config reloaded from disk', 'success');
        updatePersistBadge();
        navigateToCurrentRoute();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

/**
 * Save current config to disk.
 */
async function saveConfig() {
    try {
        await fetchJSON('/config/save', { method: 'POST' });
        showToast('Config saved to disk', 'success');
        updatePersistBadge();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

/**
 * Update the "unsaved changes" badge visibility.
 */
async function updatePersistBadge() {
    try {
        const data = await fetchJSON('/config');
        const badge = document.getElementById('persist-badge');
        if (badge) {
            badge.className = data.persisted ? 'badge badge-success hidden' : 'badge badge-warning';
            badge.textContent = data.persisted ? 'Saved' : 'Unsaved changes';
        }
    } catch (_) { /* ignore */ }
}

// SPA Router
const routes = {
    '#/status': renderStatus,
    '#/providers': renderProviders,
    '#/models': renderModels,
    '#/fallback': renderFallback,
    '#/config': renderRawConfig,
};

function navigateToCurrentRoute() {
    const hash = location.hash || '#/status';
    const render = routes[hash];
    if (render) {
        // Update active nav link
        document.querySelectorAll('.nav-link').forEach(link => {
            link.classList.toggle('active', link.getAttribute('href') === hash);
        });
        render();
    } else {
        location.hash = '#/status';
    }
}

// Initialize
window.addEventListener('hashchange', navigateToCurrentRoute);
window.addEventListener('DOMContentLoaded', () => {
    if (!location.hash) location.hash = '#/status';
    navigateToCurrentRoute();
    updatePersistBadge();
});
