(function() {
    'use strict';

    // Configuration
    const CONFIG = {
        BASE_URL: window.location.origin + '/tenant',
        TOKEN_KEY: 'tenant_token',
        TENANT_INFO_KEY: 'tenant_info'
    };

    // Auth Module
    const Auth = {
        login: async function(username, password) {
            const response = await fetch(CONFIG.BASE_URL + '/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ username, password })
            });

            const data = await response.json();

            if (response.status === 403) {
                throw new Error('Account is disabled');
            }

            if (!response.ok || data.code !== 0) {
                throw new Error(data.message || 'Login failed');
            }

            localStorage.setItem(CONFIG.TOKEN_KEY, data.data.token);
            localStorage.setItem(CONFIG.TENANT_INFO_KEY, JSON.stringify(data.data.tenant));

            return data.data;
        },

        logout: function() {
            localStorage.removeItem(CONFIG.TOKEN_KEY);
            localStorage.removeItem(CONFIG.TENANT_INFO_KEY);
            App.renderLogin();
        },

        getToken: function() {
            return localStorage.getItem(CONFIG.TOKEN_KEY);
        },

        isLoggedIn: function() {
            return !!this.getToken();
        },

        getTenantInfo: function() {
            const info = localStorage.getItem(CONFIG.TENANT_INFO_KEY);
            return info ? JSON.parse(info) : null;
        }
    };

    // API Module
    const API = {
        fetchWithAuth: async function(url, options = {}) {
            const token = Auth.getToken();

            const headers = {
                ...options.headers,
                'Authorization': 'Bearer ' + token
            };

            const response = await fetch(CONFIG.BASE_URL + url, {
                ...options,
                headers
            });

            if (response.status === 401 || response.status === 403) {
                UI.showToast('Session expired or access denied', 'error');
                Auth.logout();
                throw new Error('Unauthorized');
            }

            const data = await response.json();

            if (data.code !== 0) {
                throw new Error(data.message || 'Request failed');
            }

            return data.data;
        },

        getMe: async function() {
            return await this.fetchWithAuth('/me');
        },

        getDevices: async function() {
            return await this.fetchWithAuth('/devices');
        },

        getRevenue: async function(dateFrom, dateTo, deviceId) {
            let url = '/revenue?date_from=' + dateFrom + '&date_to=' + dateTo;
            if (deviceId) {
                url += '&registered_device_id=' + deviceId;
            }
            return await this.fetchWithAuth(url);
        }
    };

    // Utility Functions
    const Utils = {
        formatMoney: function(cents) {
            return '¥' + (cents / 100).toFixed(2);
        },

        formatDuration: function(minutes) {
            const hours = Math.floor(minutes / 60);
            const mins = minutes % 60;
            return hours + 'h ' + mins + 'm';
        },

        getTodayDate: function() {
            const today = new Date();
            const year = today.getFullYear();
            const month = String(today.getMonth() + 1).padStart(2, '0');
            const day = String(today.getDate()).padStart(2, '0');
            return year + '-' + month + '-' + day;
        },

        escapeHtml: function(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
    };

    // UI Module
    const UI = {
        renderLogin: function() {
            const app = document.getElementById('app');
            app.innerHTML = `
                <div class="login-container">
                    <div class="login-card">
                        <h1 class="login-title">Tenant Dashboard</h1>
                        <div id="login-error" class="error-message hidden"></div>
                        <form id="login-form">
                            <div class="form-group">
                                <label for="username">Username</label>
                                <input type="text" id="username" name="username" required autocomplete="username">
                            </div>
                            <div class="form-group">
                                <label for="password">Password</label>
                                <input type="password" id="password" name="password" required autocomplete="current-password">
                            </div>
                            <button type="submit" class="btn" id="login-btn">Login</button>
                        </form>
                    </div>
                </div>
            `;

            const form = document.getElementById('login-form');
            form.addEventListener('submit', async function(e) {
                e.preventDefault();

                const username = document.getElementById('username').value;
                const password = document.getElementById('password').value;
                const btn = document.getElementById('login-btn');
                const errorDiv = document.getElementById('login-error');

                errorDiv.classList.add('hidden');
                btn.disabled = true;
                btn.textContent = 'Logging in...';

                try {
                    await Auth.login(username, password);
                    App.renderDashboard();
                } catch (error) {
                    errorDiv.textContent = error.message;
                    errorDiv.classList.remove('hidden');
                } finally {
                    btn.disabled = false;
                    btn.textContent = 'Login';
                }
            });
        },

        renderDashboard: function() {
            const tenantInfo = Auth.getTenantInfo();
            const todayDate = Utils.getTodayDate();

            const app = document.getElementById('app');
            app.innerHTML = `
                <div class="dashboard-container">
                    <div class="dashboard-header">
                        <h1>Tenant Dashboard</h1>
                        <div class="tenant-info">
                            <span class="tenant-name">${Utils.escapeHtml(tenantInfo.display_name || tenantInfo.username)}</span>
                            <button class="btn-logout" id="logout-btn">Logout</button>
                        </div>
                    </div>
                    <div class="dashboard-content">
                        <div class="date-range-section">
                            <div class="date-range-controls">
                                <div class="date-input-group">
                                    <label for="date-from">From Date</label>
                                    <input type="date" id="date-from" value="${todayDate}">
                                </div>
                                <div class="date-input-group">
                                    <label for="date-to">To Date</label>
                                    <input type="date" id="date-to" value="${todayDate}">
                                </div>
                                <button class="btn-query" id="query-btn">Query</button>
                            </div>
                        </div>
                        <div class="summary-cards">
                            <div class="summary-card income">
                                <div class="summary-card-title">Total Income</div>
                                <div class="summary-card-value" id="total-income">¥0.00</div>
                            </div>
                            <div class="summary-card sessions">
                                <div class="summary-card-title">Total Sessions</div>
                                <div class="summary-card-value" id="total-sessions">0</div>
                            </div>
                            <div class="summary-card duration">
                                <div class="summary-card-title">Total Duration</div>
                                <div class="summary-card-value" id="total-duration">0h 0m</div>
                            </div>
                        </div>
                        <div class="device-table-section">
                            <h2>Device Details</h2>
                            <div id="table-content">
                                <div class="loading-container">
                                    <div class="spinner"></div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            `;

            document.getElementById('logout-btn').addEventListener('click', function() {
                Auth.logout();
            });

            document.getElementById('query-btn').addEventListener('click', function() {
                App.loadRevenueData();
            });

            App.loadRevenueData();
        },

        renderDeviceTable: function(devices) {
            const tableContent = document.getElementById('table-content');

            if (!devices || devices.length === 0) {
                tableContent.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">📊</div>
                        <div class="empty-state-text">No device data available</div>
                    </div>
                `;
                return;
            }

            let totalIncome = 0;
            let totalSessions = 0;
            let totalDuration = 0;

            const rows = devices.map(function(device) {
                totalIncome += device.total_income_cents || 0;
                totalSessions += device.session_count || 0;
                totalDuration += device.total_duration_minutes || 0;

                return `
                    <tr>
                        <td>${Utils.escapeHtml(device.device_name || 'N/A')}</td>
                        <td>${Utils.escapeHtml(device.machine_code || 'N/A')}</td>
                        <td>${device.session_count || 0}</td>
                        <td class="income-cell">${Utils.formatMoney(device.total_income_cents || 0)}</td>
                        <td>${Utils.formatDuration(device.total_duration_minutes || 0)}</td>
                    </tr>
                `;
            }).join('');

            tableContent.innerHTML = `
                <table class="device-table">
                    <thead>
                        <tr>
                            <th>Device Name</th>
                            <th>Machine Code</th>
                            <th>Sessions</th>
                            <th>Income</th>
                            <th>Duration</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${rows}
                    </tbody>
                    <tfoot>
                        <tr>
                            <td colspan="2">Total</td>
                            <td>${totalSessions}</td>
                            <td class="income-cell">${Utils.formatMoney(totalIncome)}</td>
                            <td>${Utils.formatDuration(totalDuration)}</td>
                        </tr>
                    </tfoot>
                </table>
            `;

            document.getElementById('total-income').textContent = Utils.formatMoney(totalIncome);
            document.getElementById('total-sessions').textContent = totalSessions;
            document.getElementById('total-duration').textContent = Utils.formatDuration(totalDuration);
        },

        showLoading: function() {
            const tableContent = document.getElementById('table-content');
            if (tableContent) {
                tableContent.innerHTML = `
                    <div class="loading-container">
                        <div class="spinner"></div>
                    </div>
                `;
            }
        },

        showToast: function(message, type) {
            const toast = document.createElement('div');
            toast.className = 'toast' + (type ? ' ' + type : '');
            toast.textContent = message;
            document.body.appendChild(toast);

            setTimeout(function() {
                toast.remove();
            }, 3000);
        }
    };

    // App Module
    const App = {
        init: function() {
            if (Auth.isLoggedIn()) {
                this.renderDashboard();
            } else {
                this.renderLogin();
            }
        },

        renderLogin: function() {
            UI.renderLogin();
        },

        renderDashboard: function() {
            UI.renderDashboard();
        },

        loadRevenueData: async function() {
            const dateFrom = document.getElementById('date-from').value;
            const dateTo = document.getElementById('date-to').value;
            const queryBtn = document.getElementById('query-btn');

            if (!dateFrom || !dateTo) {
                UI.showToast('Please select both dates', 'error');
                return;
            }

            queryBtn.disabled = true;
            queryBtn.textContent = 'Loading...';
            UI.showLoading();

            try {
                const data = await API.getRevenue(dateFrom, dateTo);
                UI.renderDeviceTable(data.devices || []);
            } catch (error) {
                UI.showToast('Failed to load revenue data: ' + error.message, 'error');
                document.getElementById('table-content').innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">⚠️</div>
                        <div class="empty-state-text">Failed to load data</div>
                    </div>
                `;
            } finally {
                queryBtn.disabled = false;
                queryBtn.textContent = 'Query';
            }
        }
    };

    // Initialize app when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            App.init();
        });
    } else {
        App.init();
    }
})();
