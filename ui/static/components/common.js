// common.js - Shared utilities for the AI Proxy Manager UI

const API_BASE = '/ui/api';

/**
 * Fetch JSON from an API endpoint.
 * @param {string} url - Relative URL (e.g., '/config')
 * @param {object} options - Fetch options
 * @returns {Promise<any>} Parsed JSON response
 */
async function fetchJSON(url, options = {}) {
    const resp = await fetch(API_BASE + url, {
        headers: { 'Content-Type': 'application/json', ...options.headers },
        ...options,
    });
    const data = await resp.json();
    if (!resp.ok) {
        throw new Error(data.error || `HTTP ${resp.status}`);
    }
    return data;
}

/**
 * Show a toast notification.
 * @param {string} message - Message text
 * @param {string} type - 'success', 'error', or 'info'
 * @param {number} duration - Duration in ms (default 3000)
 */
function showToast(message, type = 'info', duration = 3000) {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(100%)';
        toast.style.transition = 'all 0.3s';
        setTimeout(() => toast.remove(), 300);
    }, duration);
}

/**
 * Render an error message in the content area.
 * @param {string} message - Error message
 */
function renderError(message) {
    const content = document.getElementById('content');
    content.innerHTML = `
        <div class="card">
            <div class="card-header"><h2 class="card-title" style="color: var(--danger);">Error</h2></div>
            <p>${escapeHtml(message)}</p>
        </div>`;
}

/**
 * Escape HTML special characters.
 * @param {string} str - String to escape
 * @returns {string} Escaped string
 */
function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

/**
 * Create a DOM element with attributes and children.
 * @param {string} tag - HTML tag name
 * @param {object} attrs - Attributes
 * @param {...(string|Node)} children - Child nodes or text
 * @returns {HTMLElement}
 */
function h(tag, attrs = {}, ...children) {
    const el = document.createElement(tag);
    for (const [key, val] of Object.entries(attrs)) {
        if (key === 'className') el.className = val;
        else if (key.startsWith('on')) el.addEventListener(key.slice(2).toLowerCase(), val);
        else el.setAttribute(key, val);
    }
    for (const child of children) {
        if (typeof child === 'string') el.appendChild(document.createTextNode(child));
        else if (child) el.appendChild(child);
    }
    return el;
}
