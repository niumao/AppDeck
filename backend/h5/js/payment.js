// H5 WeChat Payment Page
// Implements full specification from ui_spec_h5.md
//
// Error Handling:
// - bindDevice() handles both HTTP status codes (404/409/503) and JSON error responses
// - Checks error message content for fallback error type detection
// - Provides user-friendly Chinese error messages with retry options

const App = {
    // State
    token: null,
    userInfo: null,
    balanceInfo: null,
    packages: [],
    tenantPlan: null,
    activeClients: [],
    selectedPackageId: null,
    isLimitedMode: false,
    userClient: null,
    _pendingRetry: null,
    _balanceTimer: null,
    // QR Scan Login state
    hostId: null,
    machineCode: null,
    deviceName: null,
    deviceStatus: null, // "idle" | "in_use" | "in_use_by_self" | "offline" | null
    deviceClientId: null, // client ID for logout when in_use_by_self
    isQRScanEntry: false,
    oaQrcodeUrl: null, // OA QR code URL from check-follow response

    // API Configuration — auto-detect path prefix (e.g. /test/h5/ → /test/api/v1)
    apiBase: (function() {
        var match = window.location.pathname.match(/^(\/[^/]+)\/h5(\/|$)/);
        if (match && match[1] !== '/h5') {
            return match[1] + '/api/v1';
        }
        return '/api/v1';
    })(),

    // Initialize application
    init: function() {
        // Check if in WeChat environment
        if (!this.isWeChat()) {
            this.showError('请在微信中打开此页面', null, null, null);
            return;
        }

        // Check for QR scan entry (host_id + machine_code in URL)
        this.checkQRScanEntry();

        // Load token from URL fragment or localStorage
        this.loadToken();

        // Check OAuth flow
        this.checkOAuth();
    },

    // Check if in WeChat browser
    isWeChat: function() {
        const ua = navigator.userAgent.toLowerCase();
        return ua.indexOf('micromessenger') !== -1;
    },

    // Check for QR scan entry (host_id + machine_code params)
    checkQRScanEntry: function() {
        const urlParams = new URLSearchParams(window.location.search);
        const hostId = urlParams.get('host_id');
        const machineCode = urlParams.get('machine_code');

        console.log('[H5] Checking QR scan entry:', {
            url: window.location.href,
            search: window.location.search,
            host_id_param: hostId,
            machine_code_param: machineCode
        });

        if (hostId && machineCode) {
            // QR scan entry detected
            this.isQRScanEntry = true;
            this.hostId = hostId;
            this.machineCode = machineCode;

            console.log('[H5] QR scan entry detected from URL params');

            // Store in sessionStorage for persistence during OAuth redirect
            sessionStorage.setItem('h5_host_id', hostId);
            sessionStorage.setItem('h5_machine_code', machineCode);
        } else {
            // Check sessionStorage in case we returned from OAuth
            const storedHostId = sessionStorage.getItem('h5_host_id');
            const storedMachineCode = sessionStorage.getItem('h5_machine_code');

            console.log('[H5] Checking sessionStorage for QR scan data:', {
                stored_host_id: storedHostId,
                stored_machine_code: storedMachineCode
            });

            if (storedHostId && storedMachineCode) {
                this.isQRScanEntry = true;
                this.hostId = storedHostId;
                this.machineCode = storedMachineCode;

                console.log('[H5] QR scan entry detected from sessionStorage');
            } else {
                console.log('[H5] No QR scan entry detected');
            }
        }
    },

    // Load token from URL fragment or localStorage
    loadToken: function() {
        // Check URL fragment for token (from OAuth callback)
        const hash = window.location.hash;
        if (hash && hash.startsWith('#token=')) {
            this.token = hash.substring(7);
            localStorage.setItem('h5_token', this.token);
            // Clean URL
            window.history.replaceState(null, '', window.location.pathname + window.location.search);
        } else {
            this.token = localStorage.getItem('h5_token');
        }
    },

    // Check OAuth state and redirect if needed
    checkOAuth: function() {
        const urlParams = new URLSearchParams(window.location.search);
        const code = urlParams.get('code');
        const state = urlParams.get('state');

        // Force re-auth: visit /h5?reauth=1 to clear token and re-login
        if (urlParams.get('reauth')) {
            localStorage.removeItem('h5_token');
            this.token = null;
            window.history.replaceState(null, '', window.location.pathname);
            this.startOAuth();
            return;
        }

        if (code) {
            // OAuth callback - exchange code for token
            this.handleOAuthCallback(code, state);
        } else if (this.token) {
            // Has token - verify and load data
            this.loadUserData();
        } else {
            // No token - start OAuth flow
            this.startOAuth();
        }
    },

    // Start OAuth authorization flow
    startOAuth: function() {
        this.showRedirectState();

        const redirectUri = encodeURIComponent(window.location.href.split('?')[0].split('#')[0]);
        const state = 'h5_' + Date.now();

        // Include host_id and machine_code in OAuth URL if QR scan entry
        let oauthUrl = `${this.apiBase}/auth/oauth/url?redirect_uri=${redirectUri}&scope=snsapi_userinfo&state=${state}`;
        if (this.isQRScanEntry && this.hostId && this.machineCode) {
            oauthUrl += `&host_id=${encodeURIComponent(this.hostId)}&machine_code=${encodeURIComponent(this.machineCode)}`;
        }

        fetch(oauthUrl)
            .then(res => res.json())
            .then(data => {
                if (data.code === 0) {
                    // Redirect to WeChat OAuth
                    setTimeout(() => {
                        window.location.href = data.data.url;
                    }, 500);
                } else {
                    this.showError('获取授权链接失败', data.message, 'auth_denied', function() { App.startOAuth(); });
                }
            })
            .catch(err => {
                this.showError('网络错误', err.message, 'network', function() { App.startOAuth(); });
            });
    },

    // Handle OAuth callback
    handleOAuthCallback: function(code, state) {
        this.showLoading('正在验证授权...');

        fetch(`${this.apiBase}/auth/oauth/callback?code=${code}&state=${state}`)
            .then(res => res.json())
            .then(data => {
                if (data.code === 0 && data.data.token) {
                    this.token = data.data.token;
                    localStorage.setItem('h5_token', this.token);

                    // Clean URL
                    window.history.replaceState(null, '', window.location.pathname);

                    // Show upgrade success toast if returning from re-auth
                    if (localStorage.getItem('h5_reauth')) {
                        localStorage.removeItem('h5_reauth');
                        this.showToast('success', '授权成功');
                    }

                    // Load user data
                    this.loadUserData();
                } else {
                    this.showError('授权失败', data.message || '无法获取授权令牌', 'auth_denied', function() { App.startOAuth(); });
                }
            })
            .catch(err => {
                this.showError('网络错误', err.message, 'network', function() { App.startOAuth(); });
            });
    },

    // Load user data (me + balance + packages + clients)
    loadUserData: function() {
        this.showLoading('加载中...');

        // Load user profile
        fetch(`${this.apiBase}/auth/me`, {
            headers: { 'Authorization': `Bearer ${this.token}` }
        })
            .then(res => res.json())
            .then(data => {
                if (data.code === 0) {
                    this.userInfo = data.data;
                    this.isLimitedMode = (data.data.auth_level === 'basic');

                    // Auto-upgrade from basic to full auth (only once per session)
                    if (data.data.auth_level === 'basic' && !sessionStorage.getItem('h5_auto_reauth')) {
                        console.log('[H5] Auto-upgrading from basic auth to full auth');
                        sessionStorage.setItem('h5_auto_reauth', '1');
                        localStorage.removeItem('h5_token');
                        this.token = null;
                        this.authorizeNow();
                        throw new Error('SKIP'); // Skip further processing
                    }

                    // Load all data in parallel
                    return Promise.all([
                        this.fetchBalance(),
                        this.fetchPackages(),
                        this.fetchActiveClients(),
                        this.fetchTenantPlan()
                    ]);
                } else if (data.code === 401) {
                    // Token expired - show message and auto-redirect
                    this.showError('授权已过期', '正在重新跳转授权...', 'auth_expired', null);
                    throw new Error('SKIP'); // Skip further processing
                } else {
                    throw new Error(data.message || '获取用户信息失败');
                }
            })
            .then(() => {
                // When QR scan entry, fetch device status for login button
                if (this.isQRScanEntry && this.hostId && this.machineCode) {
                    return this.fetchDeviceStatus();
                }
            })
            .then(() => {
                // Render main page
                this.renderMainPage();
            })
            .catch(err => {
                // Skip error display for expected cases
                if (err.message !== 'SKIP' && err.message !== 'NOT_FOLLOWED') {
                    this.showError('加载失败', err.message, 'network', function() { App.loadUserData(); });
                }
            });
    },

    // Fetch balance (includes tenant balance when machine_code available)
    fetchBalance: function() {
        var url = this.apiBase + '/user/balance';
        if (this.isQRScanEntry && this.machineCode) {
            url += '?machine_code=' + encodeURIComponent(this.machineCode);
        }
        return fetch(url, {
            headers: { 'Authorization': 'Bearer ' + this.token }
        })
            .then(function(res) {
                if (res.status === 401) return { code: -1 };
                return res.json();
            })
            .then(function(data) {
                if (data.code === 0) {
                    App.balanceInfo = data.data;
                } else {
                    console.error('Failed to load balance:', data.message);
                    App.balanceInfo = { balance_cents: 0, remaining_minutes: 0 };
                }
            })
            .catch(function(err) {
                console.error('Network error loading balance:', err);
                App.balanceInfo = { balance_cents: 0, remaining_minutes: 0 };
            });
    },

    // Fetch packages
    fetchPackages: function() {
        return fetch(`${this.apiBase}/recharge/packages`)
            .then(res => res.json())
            .then(data => {
                if (data.code === 0) {
                    this.packages = (data.data || []).sort((a, b) => (a.sort_order || 0) - (b.sort_order || 0));
                } else {
                    console.error('Failed to load packages:', data.message);
                    this.packages = [];
                }
            })
            .catch(err => {
                console.error('Network error loading packages:', err);
                this.packages = [];
            });
    },

    // Fetch tenant plan info (QR scan entry only)
    fetchTenantPlan: function() {
        if (!this.isQRScanEntry || !this.machineCode) {
            return Promise.resolve();
        }
        return fetch(`${this.apiBase}/device/plan-info?machine_code=${encodeURIComponent(this.machineCode)}`)
            .then(res => res.json())
            .then(data => {
                if (data.code === 0) {
                    this.tenantPlan = data.data;
                } else {
                    console.error('Failed to load tenant plan:', data.message);
                    this.tenantPlan = null;
                }
            })
            .catch(err => {
                console.error('Network error loading tenant plan:', err);
                this.tenantPlan = null;
            });
    },

    // Fetch active clients
    fetchActiveClients: function() {
        return fetch(`${this.apiBase}/user/clients/active`, {
            headers: { 'Authorization': `Bearer ${this.token}` }
        })
            .then(res => {
                if (res.status === 401) return { code: -1 };
                return res.json();
            })
            .then(data => {
                if (data.code === 0) {
                    this.activeClients = (data.data || []).slice(0, 3); // Max 3
                } else {
                    console.error('Failed to load active clients:', data.message);
                    this.activeClients = [];
                }
            })
            .catch(err => {
                console.error('Network error loading active clients:', err);
                this.activeClients = [];
            });
    },

    // Retry fetchActiveClients with progressive delays to wait for DB commit
    // Used after device binding to ensure the newly bound device shows in the list
    fetchActiveClientsWithRetry: function(expectedMinCount = 0, maxRetries = 3) {
        const delays = [500, 1000, 2000]; // Progressive delays in ms
        let attempt = 0;

        const tryFetch = () => {
            console.log(`[H5] Fetching active clients (attempt ${attempt + 1}/${maxRetries + 1})`);

            return this.fetchActiveClients().then(() => {
                const currentCount = this.activeClients.length;
                console.log(`[H5] Active clients count: ${currentCount}, expected min: ${expectedMinCount}`);

                // If we have the expected number of clients, we're done
                if (currentCount >= expectedMinCount) {
                    console.log('[H5] Successfully fetched active clients');
                    return;
                }

                // If we haven't reached max retries, try again after delay
                if (attempt < maxRetries) {
                    const delay = delays[attempt];
                    attempt++;
                    console.log(`[H5] Retrying in ${delay}ms...`);

                    return new Promise(resolve => setTimeout(resolve, delay))
                        .then(() => tryFetch());
                }

                // Max retries reached
                console.log('[H5] Max retries reached, using current client list');
            });
        };

        return tryFetch();
    },

    // Fetch user-client info (with plan details)
    fetchUserClient: function() {
        return fetch(`${this.apiBase}/client/user-client`, {
            headers: { 'Authorization': `Bearer ${this.token}` }
        })
            .then(res => {
                if (res.status === 401 || res.status === 404) return { code: -1 };
                return res.json();
            })
            .then(data => {
                if (data.code === 0 && data.data) {
                    this.userClient = data.data;
                } else {
                    console.error('Failed to load user-client info:', data.message || 'No data');
                    this.userClient = null;
                }
            })
            .catch(err => {
                console.error('Network error loading user-client info:', err);
                this.userClient = null;
            });
    },

    // Fetch device status for login button display
    fetchDeviceStatus: function() {
        var url = this.apiBase + '/auth/device-status?host_id=' +
            encodeURIComponent(this.hostId) + '&machine_code=' +
            encodeURIComponent(this.machineCode);
        return fetch(url, {
            headers: { 'Authorization': 'Bearer ' + this.token }
        })
        .then(function(res) { return res.json(); })
        .then(function(data) {
            if (data.code === 0 && data.data) {
                App.deviceStatus = data.data.status; // "idle" | "in_use" | "in_use_by_self" | "offline"
                App.deviceName = data.data.device_name || App.machineCode;
                App.deviceClientId = data.data.client_id || null;
            }
        })
        .catch(function() {
            App.deviceStatus = null; // unknown, hide button
        });
    },

    // Check WeChat Official Account follow status (QR scan login flow)
    checkFollowStatus: function() {
        this.showLoading('正在检查关注状态...');

        return fetch(`${this.apiBase}/h5/check-follow`, {
            headers: { 'Authorization': `Bearer ${this.token}` }
        })
            .then(res => {
                if (res.status === 401) {
                    throw new Error('AUTH_EXPIRED');
                }
                return res.json();
            })
            .then(data => {
                console.log('[H5] Check follow response:', data);

                if (data.code === 0 && data.data) {
                    const followed = data.data.followed;
                    // Support both field names for QR code URL
                    const oaQrcodeUrl = data.data.oa_qrcode_url || data.data.follow_url;

                    if (oaQrcodeUrl) {
                        this.oaQrcodeUrl = oaQrcodeUrl;
                    }

                    if (followed) {
                        // User has followed, proceed to bind device
                        console.log('[H5] User has followed OA, proceeding to bind device');
                        return this.bindDevice();
                    } else {
                        // User has not followed, show follow gate
                        console.log('[H5] User has not followed OA, showing follow gate');
                        this.showFollowGate();
                        // Return rejected promise to prevent renderMainPage from being called
                        return Promise.reject(new Error('NOT_FOLLOWED'));
                    }
                } else {
                    throw new Error(data.message || '检查关注状态失败');
                }
            })
            .catch(err => {
                if (err.message === 'NOT_FOLLOWED') {
                    // This is expected, gate is already shown
                    throw err;
                } else if (err.message === 'AUTH_EXPIRED') {
                    this.showError('授权已过期', '正在重新跳转授权...', 'auth_expired', null);
                    throw err;
                } else {
                    console.error('[H5] Check follow error:', err);
                    this.showError('检查关注状态失败', err.message || '网络错误，请重试', 'network', function() { App.checkFollowStatus(); });
                    throw err;
                }
            });
    },

    // Retry follow check (called from follow gate retry button)
    retryFollowCheck: function() {
        // Show loading state on button
        const retryBtn = document.getElementById('follow-retry-btn');
        if (retryBtn) {
            retryBtn.disabled = true;
            retryBtn.innerHTML = '<span class="spinner-inline"></span> 正在检查...';
        }

        fetch(`${this.apiBase}/h5/check-follow`, {
            headers: { 'Authorization': `Bearer ${this.token}` }
        })
            .then(res => {
                if (res.status === 401) {
                    throw new Error('AUTH_EXPIRED');
                }
                return res.json();
            })
            .then(data => {
                console.log('[H5] Retry follow check response:', data);

                if (data.code === 0 && data.data) {
                    const followed = data.data.followed;

                    if (followed) {
                        // User has now followed, proceed to bind device
                        console.log('[H5] User has followed OA, proceeding to bind device');
                        this.showLoading('正在连接设备...');
                        this.bindDevice().then(() => {
                            this.renderMainPage();
                        }).catch(err => {
                            // Error already handled in bindDevice
                        });
                    } else {
                        // Still not followed, show shake animation and toast
                        console.log('[H5] User still has not followed OA');
                        this.showToast('error', '请先关注公众号');

                        // Shake animation on button
                        const btn = document.getElementById('follow-retry-btn');
                        if (btn) {
                            btn.classList.add('shake');
                            setTimeout(() => {
                                btn.classList.remove('shake');
                                btn.disabled = false;
                                btn.innerHTML = '已关注，重新登录';
                            }, 300);
                        }
                    }
                } else {
                    throw new Error(data.message || '检查关注状态失败');
                }
            })
            .catch(err => {
                console.error('[H5] Retry follow check error:', err);

                if (err.message === 'AUTH_EXPIRED') {
                    this.showError('授权已过期', '正在重新跳转授权...', 'auth_expired', null);
                } else {
                    this.showToast('error', '网络错误，请重试');

                    // Re-enable button
                    const btn = document.getElementById('follow-retry-btn');
                    if (btn) {
                        btn.disabled = false;
                        btn.innerHTML = '已关注，重新登录';
                    }
                }
            });
    },

    // Toggle help expand section
    toggleFollowHelp: function() {
        const helpExpand = document.getElementById('follow-help-expand');
        const helpLink = document.getElementById('follow-help-link');

        if (helpExpand && helpLink) {
            const isHidden = helpExpand.style.display === 'none' || !helpExpand.style.display;

            if (isHidden) {
                helpExpand.style.display = 'block';
                helpLink.textContent = '收起';
            } else {
                helpExpand.style.display = 'none';
                helpLink.textContent = '如何关注？';
            }
        }
    },

    // Show follow required gate screen
    showFollowGate: function() {
        const app = document.getElementById('app');

        // Use configured OA QR code URL or fallback
        const qrUrl = this.oaQrcodeUrl || 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjAwIiBoZWlnaHQ9IjIwMCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48cmVjdCB3aWR0aD0iMjAwIiBoZWlnaHQ9IjIwMCIgZmlsbD0iI0Y1RjVGNSIvPjx0ZXh0IHg9IjUwJSIgeT0iNTAlIiBmb250LXNpemU9IjE0IiBmaWxsPSIjOTk5IiB0ZXh0LWFuY2hvcj0ibWlkZGxlIiBkb21pbmFudC1iYXNlbGluZT0ibWlkZGxlIj5RUiBDb2RlPC90ZXh0Pjwvc3ZnPg==';

        console.log('[H5] showFollowGate called, oaQrcodeUrl:', this.oaQrcodeUrl);
        if (!this.oaQrcodeUrl) {
            console.warn('[H5] WARNING: No OA QR code URL configured. Using fallback placeholder. Please set WECHAT_OA_FOLLOW_URL in backend .env');
        }

        app.innerHTML = `
            <div class="follow-gate">
                <div class="follow-gate__container">
                    <img src="${qrUrl}"
                         alt="公众号二维码"
                         class="follow-gate__qr"
                         onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
                    <div class="follow-gate__qr-fallback" style="display:none;">
                        <div>二维码加载失败</div>
                        <div style="font-size: 12px; color: var(--color-text-secondary); margin-top: 8px;">请联系客服获取公众号信息</div>
                    </div>

                    <h1 class="follow-gate__title">请先关注公众号后再登录</h1>
                    <p class="follow-gate__subtitle">长按上方二维码识别并关注</p>

                    <button id="follow-retry-btn" class="follow-gate__btn" onclick="App.retryFollowCheck()">
                        已关注，重新登录
                    </button>

                    <a id="follow-help-link"
                       class="follow-gate__help-link"
                       href="javascript:void(0);"
                       onclick="App.toggleFollowHelp()">
                        如何关注？
                    </a>

                    <div id="follow-help-expand" class="follow-gate__help-expand" style="display:none;">
                        <div class="follow-gate__help-title">如何关注：</div>
                        <ol class="follow-gate__help-steps">
                            <li>长按上方二维码</li>
                            <li>点击「识别图中二维码」</li>
                            <li>在公众号页面点击「关注」</li>
                            <li>返回此页面，点击「已关注，重新登录」</li>
                        </ol>
                    </div>
                </div>
            </div>
        `;
    },

    // Bind device (QR scan login flow)
    bindDevice: function() {
        this.showLoading('正在连接设备...');

        // Validate parameters before making request
        if (!this.hostId || !this.machineCode) {
            console.error('[H5] Missing bind-device parameters:', { host_id: this.hostId, machine_code: this.machineCode });
            this.showError('参数错误', '缺少设备信息，请重新扫描二维码', 'bind_failed', null);
            return Promise.reject(new Error('Missing host_id or machine_code'));
        }

        if (!this.token) {
            console.error('[H5] Missing auth token for bind-device');
            this.showError('认证错误', '请先登录', 'bind_failed', function() { App.checkOAuth(); });
            return Promise.reject(new Error('Missing auth token'));
        }

        const requestUrl = `${this.apiBase}/auth/bind-device`;
        const requestBody = {
            host_id: this.hostId,
            machine_code: this.machineCode
        };

        console.log('[H5] Binding device - Request details:', {
            url: requestUrl,
            method: 'POST',
            body: requestBody,
            hasToken: !!this.token
        });

        return fetch(requestUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${this.token}`
            },
            body: JSON.stringify(requestBody)
        })
            .then(res => {
                console.log('[H5] Bind device HTTP response:', {
                    status: res.status,
                    statusText: res.statusText,
                    ok: res.ok,
                    url: res.url
                });

                // Store status for later error handling
                const status = res.status;

                // Handle non-JSON responses (e.g., 404 from missing route)
                const contentType = res.headers.get('content-type');
                if (!contentType || !contentType.includes('application/json')) {
                    console.error('[H5] Non-JSON response from bind-device:', { status, contentType });
                    // For 404, this likely means the backend route is not registered
                    if (status === 404) {
                        throw new Error('ROUTE_NOT_FOUND');
                    }
                    throw new Error(`Server returned non-JSON response (HTTP ${status})`);
                }

                return res.json().then(data => ({ status, data })).catch(parseErr => {
                    console.error('[H5] Failed to parse JSON response:', parseErr);
                    throw new Error(`Invalid JSON response (HTTP ${status})`);
                });
            })
            .then(({ status, data }) => {
                console.log('[H5] Bind device JSON response:', {
                    status,
                    code: data.code,
                    message: data.message,
                    data: data.data
                });

                // Check HTTP status codes first
                if (status === 403 && data.message === 'ERR_NOT_FOLLOWED') {
                    // User hasn't followed the WeChat Official Account
                    console.log('[H5] User not followed OA, extracting follow_url from bind-device error');
                    // Support both field names for QR code URL (same as checkFollowStatus)
                    const followUrl = data.data?.oa_qrcode_url || data.data?.follow_url;
                    if (followUrl) {
                        this.oaQrcodeUrl = followUrl;
                        console.log('[H5] Extracted OA QR code URL:', followUrl);
                    } else {
                        console.warn('[H5] No follow_url/oa_qrcode_url in bind-device 403 response');
                    }
                    throw new Error('NOT_FOLLOWED');
                } else if (status === 404) {
                    throw new Error('DEVICE_NOT_FOUND');
                } else if (status === 409) {
                    throw new Error('DEVICE_IN_USE');
                } else if (status === 503) {
                    throw new Error('DEVICE_OFFLINE');
                } else if (status >= 400) {
                    // Other error status codes
                    throw new Error(data.message || `Server error (HTTP ${status})`);
                }

                // Check response code in JSON
                if (data.code === 0) {
                    if (!data.data) {
                        console.error('[H5] Missing data in bind-device response');
                        throw new Error('Invalid response: missing data field');
                    }

                    this.deviceName = data.data.device_name;
                    this.deviceStatus = null; // Hide login button on re-render
                    const balanceCents = data.data.balance_cents || 0;
                    const bindStatus = data.data.status;
                    const sessionId = data.data.session_id; // New: billing session ID if status is "ok"

                    console.log('[H5] Device bound successfully:', {
                        device_name: this.deviceName,
                        balance_cents: balanceCents,
                        status: bindStatus,
                        session_id: sessionId
                    });

                    if (bindStatus === 'already_logged_in') {
                        // User already logged in on this device — just refresh data
                        this.showToast('success', `已在 ${this.deviceName} 上登录`);
                        return Promise.all([
                            this.fetchBalance(),
                            this.fetchActiveClientsWithRetry(1)
                        ]);
                    } else if (bindStatus === 'ok') {
                        // Session started — refresh balance to reflect remaining time
                        this.showToast('success', `已连接到 ${this.deviceName}`);
                        return Promise.all([
                            this.fetchBalance(),
                            this.fetchActiveClientsWithRetry(1)
                        ]);
                    } else if (bindStatus === 'zero_balance') {
                        // Balance == 0: no session created yet
                        // Session will auto-create on recharge callback
                        // Will show ZeroBalancePrompt in renderMainPage
                        // Auto-select cheapest package
                        if (this.packages.length > 0) {
                            const cheapest = this.packages.reduce((min, pkg) =>
                                pkg.price_cents < min.price_cents ? pkg : min
                            );
                            this.selectedPackageId = cheapest.id;
                            console.log('[H5] Zero balance detected, auto-selected cheapest package:', cheapest.id);
                        }
                        // Refresh active clients list to show the newly logged-in device
                        // Use retry mechanism to wait for DB commit (expect at least 1 device)
                        return this.fetchActiveClientsWithRetry(1);
                    }
                } else {
                    // Check error message for specific errors
                    const errMsg = (data.message || '').toLowerCase();
                    if (errMsg.includes('not found') || errMsg.includes('未找到')) {
                        throw new Error('DEVICE_NOT_FOUND');
                    } else if (errMsg.includes('in use') || errMsg.includes('已被占用') || errMsg.includes('already')) {
                        throw new Error('DEVICE_IN_USE');
                    } else if (errMsg.includes('offline') || errMsg.includes('离线')) {
                        throw new Error('DEVICE_OFFLINE');
                    } else {
                        throw new Error(data.message || 'Device binding failed');
                    }
                }
            })
            .catch(err => {
                console.error('[H5] Bind device error:', err);

                if (err.message === 'NOT_FOLLOWED') {
                    // User hasn't followed the OA, show follow gate
                    console.log('[H5] Showing follow gate from bind-device error');
                    this.showFollowGate();
                    throw err; // Prevent renderMainPage from being called
                } else if (err.message === 'ROUTE_NOT_FOUND') {
                    this.showError('服务器配置错误', '绑定设备的接口未找到 (404)。请联系技术支持检查后端路由配置。', 'bind_failed', function() { App.loadUserData(); });
                } else if (err.message === 'DEVICE_NOT_FOUND') {
                    this.showError(
                        '设备未找到',
                        '无法找到此设备。可能原因：\n1. 设备尚未启动或注册\n2. 二维码已过期\n3. 设备信息不正确\n\n请确保桌面应用正在运行，然后重试。',
                        'device_not_found',
                        function() { App.loadUserData(); }
                    );
                } else if (err.message === 'DEVICE_IN_USE') {
                    this.showError('设备已被占用', '另一个用户正在使用此设备。请等待该用户退出后再试。', 'bind_failed', null);
                } else if (err.message === 'DEVICE_OFFLINE') {
                    this.showError(
                        '设备离线',
                        '设备当前未连接到服务器。\n\n请确保：\n1. 桌面应用正在运行\n2. 网络连接正常\n3. 设备已成功连接到服务器\n\n然后重试。',
                        'device_offline',
                        function() { App.loadUserData(); }
                    );
                } else {
                    this.showError('连接设备失败', err.message || '未知错误', 'bind_failed', function() { App.loadUserData(); });
                }
                throw err; // Prevent renderMainPage from being called
            });
    },

    // Start polling balance every 60s to keep remaining time fresh
    startBalancePolling: function() {
        this.stopBalancePolling();
        var self = this;
        this._balanceTimer = setInterval(function() {
            self.fetchBalance().then(function() {
                self.updateBalanceDisplay();
            });
        }, 60000);
    },

    // Stop balance polling
    stopBalancePolling: function() {
        if (this._balanceTimer) {
            clearInterval(this._balanceTimer);
            this._balanceTimer = null;
        }
    },

    // Update balance display in-place without re-rendering the whole page
    updateBalanceDisplay: function() {
        var balanceCents = this.balanceInfo ? this.balanceInfo.balance_cents : 0;
        var remainingMinutes = this.balanceInfo ? (this.balanceInfo.remaining_minutes || 0) : 0;

        var icons = document.querySelectorAll('.balance-icon-item span:last-child');
        if (icons.length >= 2) {
            icons[0].textContent = balanceCents * 100;
            icons[1].textContent = remainingMinutes + '分钟';
        }
    },

    // Render main page
    renderMainPage: function() {
        const app = document.getElementById('app');

        // Profile header with balance (new blue gradient design)
        const profileHeaderHtml = this.renderProfileHeader();

        // Limited mode banner
        const bannerHtml = this.isLimitedMode ? this.renderLimitedBanner() : '';

        // Device login section (if QR scan entry with device params)
        let deviceLoginHtml = '';
        if (this.isQRScanEntry && this.hostId && this.machineCode) {
            deviceLoginHtml = this.renderDeviceLoginSection();
        }

        // Package grid
        const packagesHtml = this.renderPackageGrid();

        // Time package section
        const timePackagesHtml = this.renderTimePackages();

        // Pay button or auth button
        const actionHtml = this.isLimitedMode ? this.renderAuthButton() : this.renderPayButton();

        // Quit clients
        const quitClientsHtml = this.renderQuitClients();

        // Footer
        const footerHtml = this.renderFooter();

        app.innerHTML = `
            ${profileHeaderHtml}
            <div class="page-container">
                ${bannerHtml}
                ${deviceLoginHtml}
                ${packagesHtml}
                ${timePackagesHtml}
                ${quitClientsHtml}
                ${footerHtml}
            </div>
            ${actionHtml}
        `;

        this.bindQuitClientButtons();

        // Keep remaining time fresh while page is visible
        this.startBalancePolling();
    },

    // Get user level badge based on remaining minutes
    getUserLevel: function(minutes) {
        if (minutes < 600) {
            return { level: 'Lv1', color: '#999' };
        } else if (minutes < 6000) {
            return { level: 'Lv2', color: '#4CAF50' };
        } else if (minutes < 30000) {
            return { level: 'Lv3', color: '#2196F3' };
        } else if (minutes < 60000) {
            return { level: 'Lv4', color: '#9C27B0' };
        } else if (minutes < 180000) {
            return { level: 'Lv5', color: '#FF9800' };
        } else {
            return { level: 'Lv6', color: 'linear-gradient(135deg, #FF4444, #FFD700)' };
        }
    },

    // Render profile header with balance (new design)
    renderProfileHeader: function() {
        const nickname = this.isLimitedMode ? '微信用户' : ((this.userInfo && this.userInfo.nickname) || '微信用户');
        const avatarUrl = this.isLimitedMode
            ? 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"%3E%3Crect fill="%23CCCCCC" width="100" height="100"/%3E%3Ctext x="50" y="50" font-size="40" text-anchor="middle" dominant-baseline="central" fill="white"%3E%3F%3C/text%3E%3C/svg%3E'
            : ((this.userInfo && this.userInfo.avatar_url) || 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"%3E%3Crect fill="%2307C160" width="100" height="100"/%3E%3Ctext x="50" y="50" font-size="40" text-anchor="middle" dominant-baseline="central" fill="white"%3EU%3C/text%3E%3C/svg%3E');
        const maskedId = this.maskUserId(this.userInfo ? this.userInfo.id : null);
        const badge = this.isLimitedMode ? '<span class="profile-badge">Limited Mode</span>' : '';

        // Balance info — remaining_minutes already includes tenant minutes from backend
        const balanceCents = this.balanceInfo ? this.balanceInfo.balance_cents : 0;
        const remainingMinutes = this.balanceInfo ? (this.balanceInfo.remaining_minutes || 0) : 0;

        // Get user level based on remaining minutes
        const userLevel = this.getUserLevel(remainingMinutes);
        const levelStyle = userLevel.color.includes('gradient')
            ? `background: ${userLevel.color};`
            : `background-color: ${userLevel.color};`;

        return `
            <div class="profile-header">
                <div class="profile-row">
                    <img class="profile-avatar" src="${avatarUrl}" alt="avatar" loading="lazy">
                    <div class="profile-info">
                        <div class="profile-nickname">${this.escapeHtml(nickname)}${badge}</div>
                        <div class="profile-id">ID: ${maskedId}</div>
                    </div>
                </div>
                <div class="balance-row">
                    <div class="balance-row-label">账户余额</div>
                    <div class="balance-icons">
                        <span class="level-badge" style="${levelStyle}">${userLevel.level}</span>
                        <div class="balance-icon-item">
                            <span class="icon">💎</span>
                            <span>${balanceCents * 100}</span>
                        </div>
                        <div class="balance-icon-item">
                            <span class="icon">🕐</span>
                            <span>${remainingMinutes}分钟</span>
                        </div>
                    </div>
                </div>
            </div>
        `;
    },

    // Render limited mode banner
    renderLimitedBanner: function() {
        return `
            <div class="limited-banner" onclick="App.showAuthDialog()">
                <span class="limited-banner-icon">⚠️</span>
                <span>受限模式 - 授权以获取完整功能</span>
            </div>
        `;
    },

    // Render scan success banner (device connected)
    renderScanSuccessBanner: function() {
        setTimeout(function() {
            var banner = document.getElementById('scan-success-banner');
            if (banner) {
                banner.style.transition = 'opacity 0.5s';
                banner.style.opacity = '0';
                setTimeout(function() { banner.remove(); }, 500);
            }
        }, 1000);

        return `
            <div id="scan-success-banner" class="scan-success-banner">
                <span class="scan-success-icon">✓</span>
                <span>已连接到 ${this.escapeHtml(this.deviceName)}</span>
            </div>
        `;
    },

    // Render zero balance tip (balance == 0, QR scan entry)
    renderZeroBalancePrompt: function() {
        return `
            <div class="zero-balance-tip">
                <span class="zero-balance-tip-icon">💰</span>
                <span>余额为0，充值后自动登录 <strong>"${this.escapeHtml(this.deviceName)}"</strong></span>
            </div>
        `;
    },

    // Render device login section (QR scan entry)
    renderDeviceLoginSection: function() {
        var name = this.escapeHtml(this.deviceName || this.machineCode);
        var status = this.deviceStatus;

        if (status === 'in_use_by_self') {
            return '<div class="device-login-section">' +
                '<p class="device-login-name">设备: ' + name + '</p>' +
                '<button class="device-login-btn logout" onclick="App.doDeviceLogout()">下机</button>' +
                '</div>';
        }
        if (status === 'in_use') {
            return '<div class="device-login-section">' +
                '<p class="device-login-name">设备: ' + name + '</p>' +
                '<button class="device-login-btn disabled" disabled>设备使用中</button>' +
                '</div>';
        }
        if (status === 'offline') {
            return '<div class="device-login-section">' +
                '<p class="device-login-name">设备: ' + name + '</p>' +
                '<button class="device-login-btn disabled" disabled>设备离线</button>' +
                '</div>';
        }
        if (status === 'idle') {
            return '<div class="device-login-section">' +
                '<p class="device-login-name">设备: ' + name + '</p>' +
                '<button class="device-login-btn" onclick="App.doDeviceLogin()">登录此设备</button>' +
                '</div>';
        }
        // null/unknown — hide button
        return '';
    },

    // Handle device login button click
    doDeviceLogin: function() {
        this.checkFollowStatus()
            .then(function() { App.renderMainPage(); })
            .catch(function(err) {
                if (err.message !== 'NOT_FOLLOWED') {
                    App.showError('登录失败', err.message, 'bind_failed', null);
                }
            });
    },

    // Handle device logout ("下机") button click
    doDeviceLogout: function() {
        if (!this.deviceClientId) {
            this.showToast('error', '无法获取设备信息');
            return;
        }
        var clientId = this.deviceClientId;
        var deviceName = this.deviceName || this.machineCode;
        this.showQuitDialog(clientId, deviceName);
    },

    // Format minutes to hours and minutes display
    formatTimeBalance: function(minutes) {
        if (minutes <= 0) {
            return { display: '0分钟', hours: 0, mins: 0 };
        }

        const hours = Math.floor(minutes / 60);
        const mins = minutes % 60;

        let display = '';
        if (hours > 0) {
            display += `${hours}小时`;
            if (mins > 0) {
                display += `${mins}分钟`;
            }
        } else {
            display = `${mins}分钟`;
        }

        return { display, hours, mins };
    },

    // Balance display removed - merged into profile header

    // Render package grid (new Chinese style)
    renderPackageGrid: function() {
        // Filter packages with sort_order >= 300 (diamond packages)
        const diamondPackages = this.packages.filter(pkg => (pkg.sort_order || 0) >= 300);

        if (diamondPackages.length === 0) {
            return `
                <div class="package-section">
                    <div class="package-section-title">钻石套餐 <span class="package-section-subtitle">所有店通用</span></div>
                    <div style="text-align: center; padding: 24px; color: var(--color-text-secondary);">暂无钻石套餐</div>
                </div>
            `;
        }

        const disabledClass = this.isLimitedMode ? ' package-card-disabled' : '';

        let html = `
            <div class="package-section">
                <div class="package-section-title">钻石套餐 <span class="package-section-subtitle">所有店通用</span></div>
                <div class="package-grid">
        `;

        diamondPackages.forEach((pkg, index) => {
            html += this.renderPackageCard(pkg, disabledClass, index);
        });

        html += `
                </div>
            </div>
        `;

        return html;
    },

    // Render single package card (new Chinese style)
    renderPackageCard: function(pkg, disabledClass, index) {
        const amountYuan = (pkg.price_cents / 100).toFixed(0);
        const bonusYuan = pkg.bonus_cents > 0 ? (pkg.bonus_cents / 100).toFixed(0) : 0;
        const selected = this.selectedPackageId === pkg.id ? ' selected' : '';
        const onclick = this.isLimitedMode ? '' : `onclick="App.selectPackage('${pkg.id}')"`;

        // Badge logic: first package = 推荐, packages >= 300 = 巨划算
        let badge = '';
        if (index === 0) {
            badge = '<div class="package-badge recommended">推荐</div>';
        } else if (pkg.price_cents >= 30000) {
            badge = '<div class="package-badge best-deal">巨划算</div>';
        }

        return `
            <div class="package-card${selected}${disabledClass}" data-id="${pkg.id}" ${onclick}>
                ${badge}
                <div class="package-title">送${bonusYuan}元</div>
                <div class="package-price">¥${amountYuan}</div>
            </div>
        `;
    },

    // Render single time package item (list row style)
    renderTimePackageItem: function(pkg, disabledClass) {
        const amountYuan = (pkg.price_cents / 100).toFixed(0);
        const selected = this.selectedPackageId === pkg.id ? ' selected' : '';
        const onclick = this.isLimitedMode ? '' : `onclick="App.selectPackage('${pkg.id}')"`;

        // Left side description: use package name, or format gives_minutes
        let description = pkg.name || `¥${amountYuan}`;
        if (pkg.gives_minutes && pkg.gives_minutes > 0 && !pkg.name) {
            const timeDisplay = this.formatTimeBalance(pkg.gives_minutes);
            description = timeDisplay.display;
        }

        return `
            <div class="time-package-item${selected}${disabledClass}" data-id="${pkg.id}" ${onclick}>
                <div class="time-package-desc">${description}</div>
                <div class="time-package-price">¥${amountYuan}</div>
            </div>
        `;
    },

    // Render time packages section
    renderTimePackages: function() {
        // For QR scan entry: use tenant plan set's billing plans if available
        let timePackages = this.packages.filter(pkg => (pkg.sort_order || 0) < 300);

        // Add tenant plan set packages when QR scan entry
        if (this.isQRScanEntry && this.tenantPlan && this.tenantPlan.plans && this.tenantPlan.plans.length > 0) {
            const tenantPlans = this.tenantPlan.plans.map(plan => ({
                id: plan.id,
                name: plan.name || plan.description,
                price_cents: plan.price_cents,
                gives_minutes: plan.gives_minutes,
                sort_order: 0,
                _isTenantPlan: true
            }));
            timePackages = tenantPlans.concat(timePackages);
        }

        if (timePackages.length === 0) {
            return `
                <div class="time-package-section">
                    <div class="package-section-title">时长套餐 <span class="package-section-subtitle">此店套餐</span></div>
                    <div class="time-package-empty">暂无时长套餐</div>
                </div>
            `;
        }

        const disabledClass = this.isLimitedMode ? ' package-card-disabled' : '';

        let html = `
            <div class="time-package-section">
                <div class="package-section-title">时长套餐 <span class="package-section-subtitle">此店套餐</span></div>
                <div class="time-package-list">
        `;

        timePackages.forEach((pkg) => {
            html += this.renderTimePackageItem(pkg, disabledClass);
        });

        html += `
                </div>
            </div>
        `;

        return html;
    },

    // Render pay button (Chinese text)
    renderPayButton: function() {
        const disabled = !this.selectedPackageId ? ' disabled' : '';
        const pkg = this.findSelectedPackage();
        const amountText = pkg ? `¥${(pkg.price_cents / 100).toFixed(0)}` : '¥0';

        return `
            <div class="pay-button-container">
                <button class="pay-button" onclick="App.handlePay()" ${disabled ? 'disabled' : ''}>
                    立即支付 ${amountText}
                </button>
            </div>
        `;
    },

    // Render auth button (Chinese text)
    renderAuthButton: function() {
        return `
            <div class="pay-button-container">
                <button class="auth-button" onclick="App.showAuthDialog()">
                    授权后充值
                </button>
            </div>
        `;
    },

    // Render quit clients section
    renderQuitClients: function() {
        if (this.activeClients.length === 0) {
            return '';
        }

        let html = `
            <div class="quit-clients-section">
                <div class="quit-clients-title">活动中的客户端</div>
        `;

        this.activeClients.forEach(client => {
            const name = this.truncate(client.name, 20);
            html += `
                <button class="quit-client-button" data-client-id="${this.escapeHtml(client.id)}" data-client-name="${this.escapeHtml(client.name)}">
                    <span class="quit-client-name">${this.escapeHtml(name)}</span>
                    <span class="quit-client-icon">✕</span>
                </button>
            `;
        });

        html += '</div>';
        return html;
    },

    bindQuitClientButtons: function() {
        document.querySelectorAll('.quit-client-button[data-client-id]').forEach(btn => {
            btn.addEventListener('click', () => {
                App.showQuitDialog(btn.dataset.clientId, btn.dataset.clientName);
            });
        });
    },

    // Render footer
    renderFooter: function() {
        return `
            <div class="footer">
                微信安全支付
            </div>
        `;
    },

    // Select package
    selectPackage: function(packageId) {
        if (this.isLimitedMode) return;

        this.selectedPackageId = packageId;

        // Update UI - deselect all package cards AND time package items
        const allPackageElements = [
            ...document.querySelectorAll('.package-card'),
            ...document.querySelectorAll('.time-package-item')
        ];

        allPackageElements.forEach(element => {
            if (element.dataset.id === packageId) {
                element.classList.add('selected');
            } else {
                element.classList.remove('selected');
            }
        });

        // Update button text
        const pkg = this.findSelectedPackage();
        if (pkg) {
            const amountText = `¥${(pkg.price_cents / 100).toFixed(0)}`;
            const button = document.querySelector('.pay-button');
            if (button) {
                button.textContent = `立即支付 ${amountText}`;
                button.disabled = false;
            }
        }
    },

    // Handle pay button click
    handlePay: function() {
        if (!this.selectedPackageId) {
            this.showToast('error', '请选择充值套餐');
            return;
        }

        if (this.isLimitedMode) {
            this.showAuthDialog();
            return;
        }

        this.showLoading('正在创建订单...');

        // 构建请求体，如果是扫码入口则附带 machine_code 以支持租户套餐
        const createBody = { package_id: this.selectedPackageId };
        if (this.isQRScanEntry && this.machineCode) {
            createBody.machine_code = this.machineCode;
        }

        fetch(`${this.apiBase}/recharge/create-h5`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${this.token}`
            },
            body: JSON.stringify(createBody)
        })
            .then(res => res.json())
            .then(data => {
                if (data.code === 0) {
                    this.invokeWeChatPay(data.data);
                } else {
                    this.showError('创建订单失败', data.message, 'payment_failed', function() { App.handlePay(); });
                }
            })
            .catch(err => {
                this.showError('网络错误', err.message, 'network', function() { App.handlePay(); });
            });
    },

    // Invoke WeChat JSAPI payment
    invokeWeChatPay: function(paymentData) {
        if (typeof WeixinJSBridge === 'undefined') {
            this.showError('WeChat 支付未就绪', '请稍后再试', 'payment_failed', function() { App.handlePay(); });
            return;
        }

        const params = paymentData.jsapi_params;

        WeixinJSBridge.invoke('getBrandWCPayRequest', {
            appId: params.app_id,
            timeStamp: params.time_stamp,
            nonceStr: params.nonce_str,
            package: params.package,
            signType: params.sign_type,
            paySign: params.pay_sign
        }, (res) => {
            if (res.err_msg === 'get_brand_wcpay_request:ok') {
                // Payment success
                this.handlePaymentSuccess(paymentData.order_no);
            } else if (res.err_msg === 'get_brand_wcpay_request:cancel') {
                // Payment cancelled
                this.showToast('error', '支付已取消');
                this.renderMainPage();
            } else {
                // Payment failed
                this.showError('支付失败', res.err_msg || '请重试', 'payment_failed', function() { App.handlePay(); });
            }
        });
    },

    // Poll order status until paid or timeout
    pollOrderStatus: function(orderNo, maxAttempts, interval) {
        maxAttempts = maxAttempts || 10;
        interval = interval || 1500;
        var attempt = 0;
        var self = this;

        return new Promise(function(resolve, reject) {
            function poll() {
                attempt++;
                fetch(self.apiBase + '/recharge/poll/' + encodeURIComponent(orderNo), {
                    headers: { 'Authorization': 'Bearer ' + self.token }
                })
                    .then(function(res) { return res.json(); })
                    .then(function(data) {
                        if (data.code === 0 && data.data && data.data.isPaid === true) {
                            resolve(true);
                        } else if (attempt >= maxAttempts) {
                            console.warn('[H5] Order poll timeout after ' + maxAttempts + ' attempts');
                            resolve(false); // proceed anyway, balance may update later
                        } else {
                            setTimeout(poll, interval);
                        }
                    })
                    .catch(function(err) {
                        console.error('[H5] Order poll error:', err);
                        if (attempt >= maxAttempts) {
                            resolve(false);
                        } else {
                            setTimeout(poll, interval);
                        }
                    });
            }
            poll();
        });
    },

    // Handle payment success
    handlePaymentSuccess: function(orderNo) {
        this.showLoading('正在确认支付...');

        var self = this;
        // Wait for WeChat callback to process before fetching balance
        this.pollOrderStatus(orderNo).then(function() {
            return Promise.all([
                self.fetchBalance(),
                self.fetchUserClient()
            ]);
        }).then(function() {
            var pkg = self.packages.find(function(p) { return p.id === self.selectedPackageId; });
            if (pkg) {
                self.showPaymentSuccess(pkg);
            } else {
                self.showPaymentSuccess(null);
            }
        });
    },

    // Show payment success state
    showPaymentSuccess: function(pkg) {
        const app = document.getElementById('app');

        const amountText = pkg ? `¥${(pkg.price_cents / 100).toFixed(0)}` : '¥0';
        const bonusText = (pkg && pkg.bonus_cents > 0) ? `, Bonus +¥${(pkg.bonus_cents / 100).toFixed(0)}` : '';
        const remainMins = this.balanceInfo.remaining_minutes || 0;
        const balanceText = remainMins > 0
            ? `🕐${remainMins}分钟`
            : `💎${this.balanceInfo.balance_cents * 100}`;

        // Device Ready Banner (if QR scan entry with host_id)
        let deviceReadyBanner = '';
        if (this.isQRScanEntry && this.deviceName) {
            deviceReadyBanner = `
                <div class="device-ready-banner">
                    <span class="device-ready-icon">✓</span>
                    <div class="device-ready-text">
                        设备已就绪 - 计费已开始<br>
                        已登录到 "${this.escapeHtml(this.deviceName)}"
                    </div>
                </div>
            `;
        }

        // Build plan info section if available
        let planInfo = '';
        const remainingMinutes = this.balanceInfo.remaining_minutes || 0;
        const timeBalance = this.formatTimeBalance(remainingMinutes);

        if (this.userClient && this.userClient.plan) {
            const plan = this.userClient.plan;
            const planName = plan.name || 'Default Plan';
            const rateYuan = (plan.rate_cents / 100).toFixed(2);
            const unitDisplay = plan.unit === 'hour' ? '小时' : '分钟';

            planInfo = `
                <div class="success-plan-info">
                    <div class="success-plan-text">当前计费方案: ${this.escapeHtml(planName)} (¥${rateYuan}/${unitDisplay})</div>
                    <div class="success-balance-info">新余额: ${balanceText}</div>
                    ${remainingMinutes > 0 ? `<div class="success-time-info">可用时长: <strong>${timeBalance.display}</strong></div>` : ''}
                </div>
            `;
        } else {
            // Fallback when no plan info available - still show time balance if available
            planInfo = `
                <div class="success-plan-info">
                    <div class="success-balance-info">新余额: ${balanceText}</div>
                    ${remainingMinutes > 0 ? `<div class="success-time-info">可用时长: <strong>${timeBalance.display}</strong></div>` : ''}
                </div>
            `;
        }

        app.innerHTML = `
            <div class="success-state">
                <div class="success-icon">✓</div>
                <div class="success-title">Payment Successful!</div>
                <div class="success-detail">
                    Recharged ${amountText}${bonusText}
                </div>
                ${deviceReadyBanner}
                ${planInfo}
                <button class="success-button" onclick="App.handleSuccessDone()">Done</button>
            </div>
        `;
    },

    // Handle success done button (reload page keeping QR scan context)
    handleSuccessDone: function() {
        this.loadUserData();
    },

    // Show authorization dialog
    showAuthDialog: function() {
        const overlay = document.createElement('div');
        overlay.className = 'dialog-overlay';
        overlay.innerHTML = `
            <div class="dialog">
                <div class="dialog-title">Authorization Required</div>
                <div class="dialog-content">
                    To recharge, we need to verify your identity. Please authorize access to your WeChat profile.
                </div>
                <div class="dialog-actions">
                    <button class="dialog-button dialog-button-cancel" onclick="App.closeDialog()">Cancel</button>
                    <button class="dialog-button dialog-button-primary" onclick="App.authorizeNow()">Authorize Now</button>
                </div>
            </div>
        `;
        document.body.appendChild(overlay);

        // Close on backdrop click
        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) {
                this.closeDialog();
            }
        });
    },

    // Show quit confirmation dialog
    showQuitDialog: function(clientId, clientName) {
        const overlay = document.createElement('div');
        overlay.className = 'dialog-overlay';
        overlay.innerHTML = `
            <div class="dialog">
                <div class="dialog-title">退出 ${this.escapeHtml(clientName)}?</div>
                <div class="dialog-content">
                    这将结束计费会话并从设备登出用户。
                </div>
                <div class="dialog-actions">
                    <button class="dialog-button dialog-button-cancel">取消</button>
                    <button class="dialog-button dialog-button-primary">退出</button>
                </div>
            </div>
        `;
        document.body.appendChild(overlay);

        overlay.querySelector('.dialog-button-cancel').addEventListener('click', () => {
            App.closeDialog();
        });
        overlay.querySelector('.dialog-button-primary').addEventListener('click', () => {
            App.quitClient(clientId, clientName);
        });
        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) {
                this.closeDialog();
            }
        });
    },

    // Close dialog
    closeDialog: function() {
        const overlay = document.querySelector('.dialog-overlay');
        if (overlay) {
            overlay.remove();
        }
    },

    // Authorize now (re-auth)
    authorizeNow: function() {
        this.closeDialog();

        // Mark as re-auth flow
        localStorage.setItem('h5_reauth', '1');

        // Start OAuth with userinfo scope
        this.showRedirectState();

        const redirectUri = encodeURIComponent(window.location.href.split('?')[0].split('#')[0]);
        const state = 'h5up_' + Date.now();

        fetch(`${this.apiBase}/auth/oauth/url?redirect_uri=${redirectUri}&scope=snsapi_userinfo&state=${state}`)
            .then(res => res.json())
            .then(data => {
                if (data.code === 0) {
                    setTimeout(() => {
                        window.location.href = data.data.url;
                    }, 500);
                } else {
                    this.showError('获取授权链接失败', data.message, 'auth_denied', function() { App.authorizeNow(); });
                }
            })
            .catch(err => {
                this.showError('网络错误', err.message, 'network', function() { App.authorizeNow(); });
            });
    },

    // Quit client
    quitClient: function(clientId, clientName) {
        this.closeDialog();
        this.showQuittingState(clientName);

        fetch(`${this.apiBase}/user/clients/${clientId}/quit`, {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${this.token}`
            }
        })
            .then(res => res.json())
            .then(data => {
                if (data.code === 0) {
                    this.showToast('success', '客户端已断开，计费已停止');
                    // Refresh active clients and re-render
                    this.fetchActiveClients().then(() => {
                        this.renderMainPage();
                    });
                } else {
                    this.showError('无法关闭客户端', data.message, 'quit_failed', function() { App.quitClient(clientId, clientName); });
                }
            })
            .catch(err => {
                this.showError('网络错误', err.message, 'network', function() { App.quitClient(clientId, clientName); });
            });
    },

    // Show loading state
    showLoading: function(text) {
        const app = document.getElementById('app');
        app.innerHTML = `
            <div class="loading-state">
                <div class="spinner"></div>
                <div class="loading-text">${text || '加载中...'}</div>
            </div>
        `;
    },

    // Show redirect state
    showRedirectState: function() {
        const app = document.getElementById('app');
        app.innerHTML = `
            <div class="redirect-state">
                <div class="redirect-icon">💬</div>
                <div class="redirect-text">Redirecting to WeChat auth...</div>
                <div class="redirect-subtext">Please complete in WeChat</div>
            </div>
        `;
    },

    // Show quitting state
    showQuittingState: function(clientName) {
        const app = document.getElementById('app');
        app.innerHTML = `
            <div class="quitting-state">
                <div class="spinner"></div>
                <div class="quitting-text">Closing ${this.escapeHtml(clientName)}...</div>
            </div>
        `;
    },

    // Show error state with context-specific recovery
    showError: function(title, message, errorType, retryAction) {
        const app = document.getElementById('app');

        var buttonHtml = '';
        this._pendingRetry = null;

        // Determine error icon based on type
        var errorIcon = '✕';
        if (errorType === 'device_not_found' || errorType === 'device_offline') {
            errorIcon = '⚠';
        }

        if (errorType === 'auth_expired') {
            // Auto-redirect after 2s
            app.innerHTML = `
                <div class="error-state" data-error-type="${errorType}">
                    <div class="error-icon">${errorIcon}</div>
                    <div class="error-title">${title}</div>
                    <div class="error-message">${message || ''}</div>
                </div>
            `;
            setTimeout(function() {
                localStorage.removeItem('h5_token');
                App.token = null;
                App.startOAuth();
            }, 2000);
            return;
        }

        if (errorType && retryAction) {
            this._pendingRetry = retryAction;
            var label = 'Retry';
            if (errorType === 'auth_denied') label = 'Try Again';
            else if (errorType === 'payment_failed') label = 'Retry Payment';
            else if (errorType === 'device_not_found' || errorType === 'device_offline') label = 'Retry';
            buttonHtml = `<button class="error-button" onclick="App._pendingRetry()">${label}</button>`;
        }

        const errorTypeAttr = errorType ? ` data-error-type="${errorType}"` : '';
        app.innerHTML = `
            <div class="error-state"${errorTypeAttr}>
                <div class="error-icon">${errorIcon}</div>
                <div class="error-title">${title}</div>
                <div class="error-message">${message || ''}</div>
                ${buttonHtml}
            </div>
        `;
    },

    // Show toast notification
    showToast: function(type, message) {
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.innerHTML = `
            <span class="toast-icon">${type === 'success' ? '✓' : '✕'}</span>
            <span>${message}</span>
        `;
        document.body.appendChild(toast);

        setTimeout(() => {
            toast.remove();
        }, 2000);
    },

    // Utility: Mask user ID (show last 4 digits)
    maskUserId: function(id) {
        if (!id) return '****';
        const idStr = String(id);
        if (idStr.length <= 4) return idStr;
        return '****' + idStr.slice(-4);
    },

    // Utility: Truncate text
    truncate: function(text, maxLength) {
        if (!text) return '';
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
    },

    // Find the selected package from global packages or tenant plans
    findSelectedPackage: function() {
        if (!this.selectedPackageId) return null;
        let pkg = this.packages.find(p => p.id === this.selectedPackageId);
        if (!pkg && this.tenantPlan && this.tenantPlan.plans) {
            const tenantPlan = this.tenantPlan.plans.find(p => p.id === this.selectedPackageId);
            if (tenantPlan) {
                pkg = {
                    id: tenantPlan.id,
                    name: tenantPlan.name,
                    price_cents: tenantPlan.price_cents,
                    gives_minutes: tenantPlan.gives_minutes
                };
            }
        }
        return pkg;
    },

    // Utility: Escape HTML
    escapeHtml: function(text) {
        if (!text) return '';
        return String(text)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }
};

// Initialize app on DOM ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => App.init());
} else {
    App.init();
}

// Handle WeixinJSBridge ready
if (typeof WeixinJSBridge === 'undefined') {
    if (document.addEventListener) {
        document.addEventListener('WeixinJSBridgeReady', function() {
            // Bridge is now ready
        }, false);
    } else if (document.attachEvent) {
        document.attachEvent('WeixinJSBridgeReady', function() {
            // Bridge is now ready
        });
        document.attachEvent('onWeixinJSBridgeReady', function() {
            // Bridge is now ready
        });
    }
}
