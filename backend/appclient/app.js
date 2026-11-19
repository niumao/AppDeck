// ═══════════════════════════════════════════════════════════════════════════
// GLOBAL STATE
// ═══════════════════════════════════════════════════════════════════════════

let currentPage = 'home';
let currentCategory = null;
let allApps = [];
let categories = [];
let isLoggedIn = false;
let isFreePlay = false;
let dataLoaded = false;
let currentUser = null;
let qrRefreshTimer = null;
let cachedQRInfo = null;
let cachedQRTime = 0;
let promoPlan = null;
let appVersion = '1.0.0';
let authToken = null;
let activeSessionId = null;
let countdownTimer = null;
let otherSessionCount = 0;
let billingBalanceCents = 0;
let billingPlanRateCents = 0;
let billingPlanUnitSeconds = 3600;
let billingServerTs = 0;
let billingRemainingBase = 0;
let billingBalanceSource = '';
let billingTenantMinutes = 0;
let billingGlobalCents = 0;
let cachedRankings = null;
let cachedRankingsDate = null;
let deviceName = '';

// Screensaver state
let screensaverEnabled = false;
let screensaverTimeout = 5; // minutes
let screensaverVideoHash = '';
let screensaverHasVideo = false;
let screensaverIdleTimer = null;
let screensaverActive = false;
let screensaverSettingsTimer = null;

// Multi-video screensaver state
let defaultVideoId = '';
let defaultVideoHash = '';
let tenantVideoId = '';
let tenantVideoHash = '';
let tenantStartDate = '';
let tenantEndDate = '';
let tenantStartTime = '';
let tenantEndTime = '';

// ═══════════════════════════════════════════════════════════════════════════
// API HELPERS
// ═══════════════════════════════════════════════════════════════════════════

function getQueryParam(name) {
    const urlParams = new URLSearchParams(window.location.search);
    return urlParams.get(name);
}

const HOST_ID = getQueryParam('host_id') || '';
const MACHINE_CODE = getQueryParam('machine_code') || '';
var isTvMode = getQueryParam('tv') === '1';

async function apiGet(path, extraHeaders = {}) {
    const response = await fetch(path, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
            ...extraHeaders
        }
    });

    if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const result = await response.json();

    if (result.code !== 0) {
        throw new Error(result.message || 'API error');
    }

    return result.data;
}

async function apiPost(path, body) {
    const response = await fetch(path, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(body)
    });

    if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const result = await response.json();

    if (result.code !== 0) {
        throw new Error(result.message || 'API error');
    }

    return result.data;
}

// ═══════════════════════════════════════════════════════════════════════════
// UTILITIES
// ═══════════════════════════════════════════════════════════════════════════

function getImageURL(type, filename) {
    if (!filename || !filename.trim()) return '';
    if (filename.startsWith('http')) return filename;
    return `/api/v1/${type}/${filename}`;
}

function resolveIconURL(app) {
    if (app.launchType === 'retroarch' && app.retroarchRom) {
        return 'retroarch://thumb/' + app.retroarchRom;
    }
    if (app.iconUrl) return getImageURL('icons', app.iconUrl);
    if (app.launchType === 'android' && app.launchParam) return 'pkg://' + app.launchParam;
    if (app.launchType === 'eden' && app.launchParam) {
        var parts = app.launchParam.split('/');
        var xciFile = parts[parts.length - 1];
        var baseName = xciFile.replace(/\.(xci|nsp)$/i, '');
        return 'eden://icon/' + encodeURIComponent(baseName);
    }
    return '';
}

function createPlaceholderIcon(name) {
    const letter = (name && name.length > 0) ? name.charAt(0).toUpperCase() : '?';
    const svg = `
        <svg width="200" height="200" xmlns="http://www.w3.org/2000/svg">
            <defs>
                <linearGradient id="grad" x1="0%" y1="0%" x2="100%" y2="100%">
                    <stop offset="0%" style="stop-color:#5b9bd5;stop-opacity:1" />
                    <stop offset="100%" style="stop-color:#5bccf5;stop-opacity:1" />
                </linearGradient>
            </defs>
            <rect width="200" height="200" fill="url(#grad)"/>
            <text x="50%" y="50%" font-size="80" fill="#fff" text-anchor="middle" dominant-baseline="central" font-family="Arial">${letter}</text>
        </svg>
    `;
    return 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg);
}

function escapeHtml(text) {
    const map = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#039;'
    };
    return text.replace(/[&<>"']/g, m => map[m]);
}

function showToast(message, type = 'info', duration = 3000) {
    const container = document.getElementById('toastContainer');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    container.appendChild(toast);

    setTimeout(() => {
        toast.style.animation = 'slideOutRight 0.3s ease';
        setTimeout(() => {
            if (toast.parentNode) {
                container.removeChild(toast);
            }
        }, 300);
    }, duration);
}

// Persistent (not auto-dismissed) status pill for the login-time save
// restore, since we don't know up front how long it'll take.
//
// completed/total describe restore *operations*, not games -- the backend
// stores one zip bundle per user covering every Eden title at once, so
// there's no per-game count to show. Don't render them as a fraction; that
// reads as "only 1 of your games was restored" when actually every title
// sharing the on-device profile came back together.
function updateSaveSyncStatus(completed, total, done) {
    const el = document.getElementById('saveSyncStatus');
    const textEl = document.getElementById('saveSyncStatusText');
    if (!el || !textEl) return;

    el.classList.remove('fading');
    el.style.display = 'flex';

    if (done) {
        el.classList.add('done');
        textEl.textContent = '存档已同步';
        setTimeout(() => {
            el.classList.add('fading');
            setTimeout(() => { el.style.display = 'none'; }, 300);
        }, 1500);
    } else {
        el.classList.remove('done');
        textEl.textContent = '正在恢复存档...';
    }
}

// ═══════════════════════════════════════════════════════════════════════════
// INITIALIZATION
// ═══════════════════════════════════════════════════════════════════════════

document.addEventListener('DOMContentLoaded', async () => {
    // Initialize systems
    initModals();
    initNavigation();
    initGamepadNav();

    // Load initial data
    await loadInitialData();
    dataLoaded = true;
    updateAuthUI();

    // Set page from hash or default
    const hash = window.location.hash.slice(1) || 'home';
    setPage(hash);

    // Start countdown timer if login state was restored from cache
    if (isLoggedIn && billingServerTs > 0) {
        startCountdownDisplay();
    }
});

// ═══════════════════════════════════════════════════════════════════════════
// DATA LOADING
// ═══════════════════════════════════════════════════════════════════════════

async function loadInitialData() {
    // Restore login state from native cache FIRST (sync, before any async)
    // This covers WebView reload (error, display switch, manual refresh)
    if (typeof NativeBridge !== 'undefined' && NativeBridge.getDeviceState) {
        try {
            const stateJson = NativeBridge.getDeviceState();
            if (stateJson) {
                const state = JSON.parse(stateJson);
                if (state.free_play) {
                    isFreePlay = true;
                    isLoggedIn = true;
                }
                if (state.state === 'logged_in' && state.user) {
                    isLoggedIn = true;
                    currentUser = { nickname: state.user.nickname, avatarURL: state.user.avatar_url };
                    authToken = state.token || null;
                    activeSessionId = state.session_id || null;
                    billingBalanceCents = state.balance_cents || 0;
                    billingPlanRateCents = state.plan_rate_cents || 0;
                    billingPlanUnitSeconds = state.plan_unit_seconds || 3600;
                    billingServerTs = state.server_ts || 0;
                    billingRemainingBase = state.remaining_seconds || 0;
                    billingBalanceSource = state.balance_source || '';
                    otherSessionCount = state.other_session_count || 0;
                    billingTenantMinutes = state.tenant_balance_minutes || 0;
                    billingGlobalCents = state.global_balance_cents || 0;
                }
            }
        } catch (e) {
            console.log('Failed to restore state from native cache:', e);
        }
    }

    // Check freeplay mode FIRST (before any other await) to avoid race with WS idle event
    try {
        const regData = await apiGet(`/api/v1/device/register?host_id=${HOST_ID}&machine_code=${MACHINE_CODE}`);
        if (regData.free_play) {
            isFreePlay = true;
            isLoggedIn = true;
        }
        deviceName = regData.device_name || '';
    } catch (err) {
        console.log('Device register check failed:', err);
    }

    try {
        // Fetch full app data
        const fullData = await apiGet('/api/v1/apps/full');

        appVersion = fullData.version || '1.0.0';
        categories = fullData.categories || [];
        allApps = fullData.apps || [];

        // Enrich apps with category info
        const categoryMap = {};
        categories.forEach(cat => {
            categoryMap[cat.id] = cat.name;
        });

        allApps = allApps.map(app => {
            const categoryId = app.categoryIds && app.categoryIds.length > 0 ? app.categoryIds[0] : null;
            return {
                ...app,
                categoryId: categoryId,
                categoryName: categoryId ? categoryMap[categoryId] : null
            };
        });

        // Build category appCount
        categories = categories.map(cat => {
            const appCount = allApps.filter(app => app.categoryIds && app.categoryIds.includes(cat.id)).length;
            return { ...cat, appCount };
        });

        // Load promo plan
        try {
            promoPlan = await apiGet('/api/v1/plans/promo');
        } catch (err) {
            console.log('No promo plan available');
            promoPlan = null;
        }

        // Initialize screensaver
        initScreensaver();

    } catch (error) {
        console.error('Failed to load data:', error);
        showToast('数据加载失败', 'error');
    }
}

async function refreshData() {
    const logoBadge = document.querySelector('.logo-badge');

    try {
        logoBadge.classList.add('refreshing');

        // Reload full data (clear caches)
        cachedRankings = null;
        await loadInitialData();

        // Re-render current page
        setPage(currentPage);

        showToast('数据已刷新', 'success');

    } catch (error) {
        console.error('Refresh failed:', error);
        showToast('刷新失败', 'error');
    } finally {
        logoBadge.classList.remove('refreshing');
    }
}

// ═══════════════════════════════════════════════════════════════════════════
// NAVIGATION
// ═══════════════════════════════════════════════════════════════════════════

function initNavigation() {
    // Logo badge click handler
    const logoBadge = document.getElementById('logoBadge');
    logoBadge.addEventListener('click', () => {
        refreshData();
    });

    // Nav tabs
    const navTabs = document.querySelectorAll('.nav-tab');
    navTabs.forEach(tab => {
        tab.addEventListener('click', () => {
            const page = tab.dataset.page;
            setPage(page);
        });
    });

    // Hash change handler
    window.addEventListener('hashchange', () => {
        const hash = window.location.hash.slice(1) || 'home';
        setPage(hash);
    });
}

function setPage(page) {
    const prevPage = currentPage;
    currentPage = page;
    window.location.hash = page;
    clearGamepadFocus();

    // Clear QR timer when leaving home page
    if (prevPage === 'home' && page !== 'home') {
        if (qrRefreshTimer) {
            clearTimeout(qrRefreshTimer);
            qrRefreshTimer = null;
        }
    }

    // Update nav tabs
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.classList.toggle('active', tab.dataset.page === page);
    });

    // Render page
    const container = document.getElementById('pageContainer');
    container.innerHTML = '';

    if (page === 'home') {
        renderHomePage();
    } else if (page === 'library') {
        renderLibraryPage();
    } else if (page === 'rankings') {
        renderRankingsPage();
    }
}

// ═══════════════════════════════════════════════════════════════════════════
// HOME PAGE
// ═══════════════════════════════════════════════════════════════════════════

function renderHomePage() {
    const container = document.getElementById('pageContainer');

    const page = document.createElement('div');
    page.className = 'home-page';

    // Left content area (scrollable)
    const leftContent = document.createElement('div');
    leftContent.className = 'home-left-content';

    // Top row: Hero + Grid
    const topRow = document.createElement('div');
    topRow.className = 'home-top-row';

    const hero = renderHeroCard();
    topRow.appendChild(hero);

    const center = renderHomeCenter();
    topRow.appendChild(center);

    leftContent.appendChild(topRow);

    // Category bar (5 buttons)
    const categoryBar = renderCategoryBar();
    leftContent.appendChild(categoryBar);

    // All-Games Grid (4x3 = 12 items)
    const allGamesGrid = document.createElement('div');
    allGamesGrid.className = 'all-games-grid';
    allGamesGrid.id = 'allGamesGrid';
    const maxGridItems = 12; // 4 columns × 3 rows
    allApps.slice(7, 7 + maxGridItems).forEach(app => {
        const card = createAppCardHome(app);
        allGamesGrid.appendChild(card);
    });
    leftContent.appendChild(allGamesGrid);

    // "全部游戏" long button after grid
    const showAllBtn = document.createElement('button');
    showAllBtn.className = 'show-category-btn';
    showAllBtn.textContent = '全部游戏';
    showAllBtn.addEventListener('click', () => setPage('library'));
    showAllBtn.tabIndex = 0;
    leftContent.appendChild(showAllBtn);

    page.appendChild(leftContent);

    // Right sidebar (fixed)
    const sidebar = renderInfoSidebar();
    page.appendChild(sidebar);

    container.appendChild(page);
}

// Hero Card - SINGLE STATIC FEATURED CARD
function renderHeroCard() {
    const hero = document.createElement('div');
    hero.className = 'hero-card';
    hero.id = 'heroCard';
    hero.tabIndex = 0;

    if (allApps.length > 0) {
        const app = allApps[0]; // Always show first app

        const cover = document.createElement('img');
        cover.className = 'hero-cover';
        cover.src = getImageURL('bigpics', app.bigPicUrl) || resolveIconURL(app) || createPlaceholderIcon(app.name);
        cover.alt = app.name;
        cover.onerror = function() { this.onerror = null; this.src = createPlaceholderIcon(app.name); };

        const overlay = document.createElement('div');
        overlay.className = 'hero-overlay';

        const title = document.createElement('h2');
        title.className = 'hero-title';
        title.textContent = app.name;

        const tags = document.createElement('div');
        tags.className = 'hero-tags';
        if (app.categoryName) {
            const tag = document.createElement('span');
            tag.className = 'hero-tag';
            tag.textContent = app.categoryName;
            tags.appendChild(tag);
        }

        overlay.appendChild(title);
        overlay.appendChild(tags);

        hero.appendChild(cover);
        hero.appendChild(overlay);

        // Click to open detail
        hero.addEventListener('click', () => openAppDetail(app));
    }

    return hero;
}

// Home Center - Grid ONLY
function renderHomeCenter() {
    const center = document.createElement('div');
    center.className = 'home-center';

    // Home grid (3x2)
    const grid = document.createElement('div');
    grid.className = 'home-grid';
    grid.id = 'homeGrid';

    const gridApps = allApps.slice(1, 7);
    gridApps.forEach(app => {
        const card = createAppCardHome(app);
        grid.appendChild(card);
    });

    center.appendChild(grid);

    return center;
}

function renderCategoryBar() {
    const bar = document.createElement('div');
    bar.className = 'category-bar';
    bar.id = 'categoryBar';

    // Build pills array: "全部游戏" + first 3 real categories + "排行榜"
    const pills = [];

    // "全部游戏" button
    pills.push({
        label: '全部游戏',
        class: 'all-games',
        action: () => setPage('library')
    });

    // Use first 3 real categories from backend
    const categoryColors = ['two-player', 'family', 'new-games'];

    categories.slice(0, 3).forEach((cat, idx) => {
        pills.push({
            label: cat.name,
            class: categoryColors[idx] || 'all-games',
            categoryId: cat.id,
            action: () => {
                currentCategory = cat.id;
                setPage('library');
                setTimeout(() => selectCategory(cat.id), 50);
            }
        });
    });

    // "排行榜" button
    pills.push({
        label: '排行榜',
        class: 'rankings',
        action: () => setPage('rankings')
    });

    pills.forEach(p => {
        const pill = document.createElement('button');
        pill.className = `category-pill ${p.class}`;
        pill.textContent = p.label;
        pill.addEventListener('click', p.action);
        pill.tabIndex = 0;

        bar.appendChild(pill);
    });

    return bar;
}

function createAppCardHome(app) {
    const card = document.createElement('div');
    card.className = 'app-card';
    card.tabIndex = 0;
    card.dataset.appId = app.id;

    const icon = document.createElement('img');
    icon.className = 'app-card-icon';
    icon.src = resolveIconURL(app) || createPlaceholderIcon(app.name);
    icon.alt = app.name || '';
    icon.onerror = () => {
        icon.src = createPlaceholderIcon(app.name);
    };

    const name = document.createElement('div');
    name.className = 'app-card-name';
    name.textContent = app.name;

    card.appendChild(icon);
    card.appendChild(name);

    // Add category tags
    if (app.categoryName) {
        const tags = document.createElement('div');
        tags.className = 'app-card-tags';

        const tag = document.createElement('span');
        tag.className = 'app-card-tag';
        tag.textContent = app.categoryName;
        tags.appendChild(tag);

        card.appendChild(tags);
    }

    card.addEventListener('click', () => openAppDetail(app));

    return card;
}

// ═══════════════════════════════════════════════════════════════════════════
// INFO SIDEBAR
// ═══════════════════════════════════════════════════════════════════════════

function renderInfoSidebar() {
    const sidebar = document.createElement('div');
    sidebar.className = 'info-sidebar';

    if (isFreePlay) {
        // Freeplay mode: show free badge instead of QR login
        const freeBox = document.createElement('div');
        freeBox.className = 'qr-login';
        freeBox.id = 'qrLogin';
        const freeTitle = document.createElement('h3');
        freeTitle.className = 'qr-login-title';
        freeTitle.textContent = '免费畅玩模式';
        freeBox.appendChild(freeTitle);
        sidebar.appendChild(freeBox);
    } else if (isLoggedIn && currentUser) {
        // Logged in: show user card with avatar
        const userCard = renderUserCard();
        sidebar.appendChild(userCard);
    } else {
        // QR Login section
        const qr = renderQRLogin();
        sidebar.appendChild(qr);
    }

    // Controller Guide
    const guide = renderControllerGuide();
    sidebar.appendChild(guide);

    // Billing Bar
    if (promoPlan && !isFreePlay) {
        const billing = renderBillingBar();
        sidebar.appendChild(billing);
    }

    // Device Info
    const device = renderDeviceInfo();
    sidebar.appendChild(device);

    // Health Reminder
    const health = renderHealthReminder();
    sidebar.appendChild(health);

    // Service Hotline
    const hotline = renderServiceHotline();
    sidebar.appendChild(hotline);

    return sidebar;
}

function renderUserCard() {
    const container = document.createElement('div');
    container.className = 'qr-login';
    container.id = 'qrLogin';

    const title = document.createElement('h3');
    title.className = 'qr-login-title';
    title.textContent = escapeHtml(currentUser.nickname || '用户');
    container.appendChild(title);

    if (currentUser.avatarURL) {
        const avatar = document.createElement('img');
        avatar.src = currentUser.avatarURL;
        avatar.style.cssText = 'width:80px;height:80px;border-radius:50%;margin:8px auto;display:block;';
        container.appendChild(avatar);
    }

    var logoutBtn = document.createElement('button');
    logoutBtn.className = 'user-card-logout';
    logoutBtn.textContent = '退出登录';
    logoutBtn.addEventListener('click', function() {
        if (typeof NativeBridge !== 'undefined' && NativeBridge.logout) {
            NativeBridge.logout();
        }
    });
    container.appendChild(logoutBtn);

    return container;
}

function renderQRLogin() {
    const container = document.createElement('div');
    container.className = 'qr-login';
    container.id = 'qrLogin';

    const title = document.createElement('h3');
    title.className = 'qr-login-title';
    title.textContent = '微信扫码登录';
    container.appendChild(title);

    const qrBox = document.createElement('div');
    qrBox.className = 'qr-code';
    qrBox.id = 'qrCodeBox';

    const spinner = document.createElement('div');
    spinner.className = 'qr-spinner';
    qrBox.appendChild(spinner);

    container.appendChild(qrBox);

    // Load QR code
    loadQRCode(qrBox);

    return container;
}

async function loadQRCode(qrBox) {
    try {
        const now = Date.now();
        // Reuse cached QR if less than 80 seconds old
        if (cachedQRInfo && (now - cachedQRTime) < 80 * 1000) {
            renderQRImage(qrBox, cachedQRInfo);
            return;
        }

        const ts = Math.floor(Date.now() / 1000);
        const qrInfo = await apiGet(`/api/v1/auth/oa-qrcode?host_id=${HOST_ID}&machine_code=${MACHINE_CODE}&ts=${ts}`);
        cachedQRInfo = qrInfo;
        cachedQRTime = Date.now();
        renderQRImage(qrBox, qrInfo);

    } catch (error) {
        console.error('Failed to load QR code:', error);
        qrBox.innerHTML = '<div class="qr-retry" onclick="loadQRCode(document.getElementById(\'qrCodeBox\'))">重试</div>';
    }
}

function renderQRImage(qrBox, qrInfo) {
    const img = document.createElement('img');
    img.src = qrInfo.qr_url;
    img.alt = 'QR Code';

    qrBox.innerHTML = '';
    qrBox.appendChild(img);

    // Clear existing timer and set new one
    if (qrRefreshTimer) {
        clearTimeout(qrRefreshTimer);
        qrRefreshTimer = null;
    }

    // Set refresh timer for 90 seconds
    qrRefreshTimer = setTimeout(() => {
        if (currentPage === 'home') {
            cachedQRInfo = null; // force fresh fetch
            loadQRCode(qrBox);
        }
    }, 90 * 1000);
}

function renderControllerGuide() {
    const guide = document.createElement('div');
    guide.className = 'controller-guide';

    const title = document.createElement('h3');
    title.className = 'controller-guide-title';
    title.textContent = '移动';
    guide.appendChild(title);

    // Simplified gamepad icon
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 120 80');
    svg.innerHTML = `
        <rect x="10" y="20" width="100" height="40" rx="20" fill="none" stroke="#f5a623" stroke-width="2"/>
        <circle cx="35" cy="40" r="8" fill="#4caf50"/>
        <circle cx="85" cy="40" r="8" fill="#ff3232"/>
    `;
    guide.appendChild(svg);

    const text = document.createElement('div');
    text.className = 'controller-guide-text';
    text.innerHTML = 'A(绿)=确认<br>B(红)=返回<br>LB+RB+START=游戏强制退出<br>长按SELECT+↑/↓=音量调节<br>游戏长按返键显示菜单';
    guide.appendChild(text);

    return guide;
}

function renderBillingBar() {
    const bar = document.createElement('div');
    bar.className = 'billing-bar';

    if (promoPlan.duration_display) {
        // Use server-provided duration display
        const rate = (promoPlan.price_cents / 100).toFixed(2);
        bar.textContent = `${promoPlan.duration_display} ${promoPlan.price_cents}钻石(${rate}元)`;
    } else {
        // Fallback to original format
        const rate = (promoPlan.price_cents / 100).toFixed(2);
        bar.textContent = `每小时${promoPlan.price_cents}钻石(${rate}元)`;
    }

    return bar;
}

function renderDeviceInfo() {
    const info = document.createElement('div');
    info.className = 'device-info';

    const machineCodeShort = MACHINE_CODE ? MACHINE_CODE.slice(0, 8) : 'N/A';
    const displayName = deviceName || machineCodeShort;
    info.innerHTML = `设备名: ${escapeHtml(displayName)}<br>版本号: ${escapeHtml(appVersion)}`;

    return info;
}

function renderHealthReminder() {
    const reminder = document.createElement('div');
    reminder.className = 'health-reminder';

    const title = document.createElement('h3');
    title.className = 'health-reminder-title';
    title.textContent = '友情提示';
    reminder.appendChild(title);

    const text = document.createElement('div');
    text.className = 'health-reminder-text';
    text.textContent = '合理安排时间，享受健康生活，适度游戏益脑，沉迷游戏伤身。';
    reminder.appendChild(text);

    return reminder;
}

function renderServiceHotline() {
    const hotline = document.createElement('div');
    hotline.className = 'service-hotline';
    hotline.innerHTML = '客服热线: <span>15701387907</span>';
    return hotline;
}

// ═══════════════════════════════════════════════════════════════════════════
// LIBRARY PAGE
// ═══════════════════════════════════════════════════════════════════════════

function renderLibraryPage() {
    const container = document.getElementById('pageContainer');

    const mainContent = document.createElement('div');
    mainContent.className = 'main-content';

    // Sidebar
    const sidebar = document.createElement('div');
    sidebar.className = 'library-sidebar';

    const sidebarTitle = document.createElement('h2');
    sidebarTitle.className = 'sidebar-title';
    sidebarTitle.textContent = 'All Games';
    sidebar.appendChild(sidebarTitle);

    const categoryList = document.createElement('div');
    categoryList.className = 'category-list';
    categoryList.id = 'categoryList';
    sidebar.appendChild(categoryList);

    mainContent.appendChild(sidebar);

    // Library main
    const libraryMain = document.createElement('div');
    libraryMain.className = 'library-main';

    const libraryContent = document.createElement('div');
    libraryContent.className = 'library-content';
    libraryContent.id = 'libraryContent';
    libraryMain.appendChild(libraryContent);

    mainContent.appendChild(libraryMain);

    container.appendChild(mainContent);

    // Render categories and apps
    renderCategories();
    if (currentCategory === null) {
        renderApps(allApps);
    } else {
        const filtered = allApps.filter(app => app.categoryIds && app.categoryIds.includes(currentCategory));
        renderApps(filtered);
    }
}

function renderCategories() {
    const categoryList = document.getElementById('categoryList');
    if (!categoryList) return;

    categoryList.innerHTML = '';

    // All Games
    const allCategory = document.createElement('button');
    allCategory.className = 'category-item' + (currentCategory === null ? ' active' : '');
    allCategory.innerHTML = `
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
            <rect x="3" y="3" width="7" height="7" rx="1" stroke="currentColor" stroke-width="2"/>
            <rect x="14" y="3" width="7" height="7" rx="1" stroke="currentColor" stroke-width="2"/>
            <rect x="3" y="14" width="7" height="7" rx="1" stroke="currentColor" stroke-width="2"/>
            <rect x="14" y="14" width="7" height="7" rx="1" stroke="currentColor" stroke-width="2"/>
        </svg>
        <span>All Games</span>
    `;
    allCategory.addEventListener('click', () => selectCategory(null));
    categoryList.appendChild(allCategory);

    // Other categories
    categories.forEach(cat => {
        const item = document.createElement('button');
        item.className = 'category-item' + (currentCategory === cat.id ? ' active' : '');
        item.innerHTML = `
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <path d="M3 7h18M3 12h18M3 17h18" stroke="currentColor" stroke-width="2"/>
            </svg>
            <span>${escapeHtml(cat.name)} (${cat.appCount})</span>
        `;
        item.addEventListener('click', () => selectCategory(cat.id));
        categoryList.appendChild(item);
    });
}

function selectCategory(categoryId) {
    currentCategory = categoryId;
    renderCategories();

    let apps;
    if (categoryId === null) {
        apps = allApps;
    } else {
        apps = allApps.filter(app => app.categoryIds && app.categoryIds.includes(categoryId));
    }
    renderApps(apps || []);
}

function renderApps(apps) {
    const content = document.getElementById('libraryContent');
    if (!content) return;

    if (apps.length === 0) {
        content.innerHTML = '<div class="empty-state"><p>No games</p></div>';
        return;
    }

    content.innerHTML = '';
    apps.forEach(app => {
        const card = createAppCard(app);
        content.appendChild(card);
    });
}

function createAppCard(app) {
    const card = document.createElement('div');
    card.className = 'app-card';
    card.tabIndex = 0;
    card.setAttribute('role', 'button');
    card.setAttribute('aria-label', `Open ${app.name}`);

    const icon = document.createElement('img');
    icon.className = 'app-card-icon';
    icon.alt = app.name || '';
    icon.src = resolveIconURL(app) || createPlaceholderIcon(app.name);
    icon.onerror = () => {
        icon.src = createPlaceholderIcon(app.name);
    };

    const name = document.createElement('div');
    name.className = 'app-card-name';
    name.textContent = app.name;

    card.appendChild(icon);
    card.appendChild(name);

    card.addEventListener('click', () => openAppDetail(app));
    card.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            openAppDetail(app);
        }
    });

    return card;
}

// ═══════════════════════════════════════════════════════════════════════════
// RANKINGS PAGE
// ═══════════════════════════════════════════════════════════════════════════

function renderRankingsPage() {
    const container = document.getElementById('pageContainer');

    const page = document.createElement('div');
    page.className = 'rankings-page';

    // Header with breadcrumb and back indicator
    const header = document.createElement('div');
    header.className = 'rankings-header';

    // Breadcrumb
    const breadcrumb = document.createElement('div');
    breadcrumb.className = 'breadcrumb';
    breadcrumb.innerHTML = '<a href="#home">首页</a> > 排行榜';
    breadcrumb.querySelector('a').addEventListener('click', (e) => {
        e.preventDefault();
        setPage('home');
    });
    header.appendChild(breadcrumb);

    // Back indicator
    const backInd = document.createElement('div');
    backInd.className = 'back-indicator';
    backInd.innerHTML = '返回 <span class="b-button">B</span>';
    header.appendChild(backInd);

    page.appendChild(header);

    // Rankings columns
    const columns = document.createElement('div');
    columns.className = 'rankings-columns';
    columns.id = 'rankingsColumns';

    // Load rankings (cached after first fetch)
    function renderRankingsData(hot, trending, regional) {
        columns.innerHTML = '';
        const hotList = renderRankList('热门榜', hot || []);
        hotList.id = 'rankListHot';
        columns.appendChild(hotList);

        const trendList = renderRankList('飙升榜', trending || []);
        trendList.id = 'rankListTrending';
        columns.appendChild(trendList);

        if (regional) {
            const regionalList = renderRankList('地区榜', regional);
            regionalList.id = 'rankListRegional';
            columns.appendChild(regionalList);
        }

        hotList.classList.add('focused');
    }

    var today = new Date().toDateString();
    if (cachedRankings && cachedRankingsDate === today) {
        renderRankingsData(cachedRankings.hot, cachedRankings.trending, cachedRankings.regional);
    } else {
        apiGet('/api/v1/rankings', MACHINE_CODE ? { 'X-Machine-Code': MACHINE_CODE } : {})
            .then(({ hot, trending, regional }) => {
                cachedRankings = { hot, trending, regional };
                cachedRankingsDate = today;
                renderRankingsData(hot, trending, regional);
            }).catch(err => {
                console.error('Failed to load rankings:', err);
                columns.innerHTML = '<div class="empty-state"><p>Failed to load rankings</p></div>';
            });
    }

    page.appendChild(columns);
    container.appendChild(page);
}

function renderRankList(title, apps) {
    const list = document.createElement('div');
    list.className = 'rank-list';

    const titleEl = document.createElement('h2');
    titleEl.className = 'rank-list-title';
    titleEl.textContent = title;
    list.appendChild(titleEl);

    apps.forEach((app, idx) => {
        const entry = document.createElement('div');
        entry.className = `rank-entry rank-${idx + 1}`;
        entry.tabIndex = 0;
        entry.dataset.rank = idx + 1;

        const rankNum = document.createElement('div');
        rankNum.className = 'rank-number';
        rankNum.textContent = idx + 1;
        entry.appendChild(rankNum);

        const icon = document.createElement('img');
        icon.className = 'rank-icon';
        icon.src = resolveIconURL(app) || createPlaceholderIcon(app.name);
        icon.alt = app.name || '';
        icon.onerror = () => {
            icon.src = createPlaceholderIcon(app.name);
        };
        entry.appendChild(icon);

        const info = document.createElement('div');
        info.className = 'rank-info';

        const name = document.createElement('div');
        name.className = 'rank-name';
        name.textContent = app.name;
        info.appendChild(name);

        entry.appendChild(info);
        entry.addEventListener('click', () => openAppDetail(app));

        list.appendChild(entry);
    });

    return list;
}

// ═══════════════════════════════════════════════════════════════════════════
// MODAL SYSTEM
// ═══════════════════════════════════════════════════════════════════════════

function initModals() {
    const modals = ['appModal', 'aboutModal'];

    modals.forEach(modalId => {
        const modal = document.getElementById(modalId);
        const overlay = modal.querySelector('.modal-overlay');
        const closeBtn = modal.querySelector('.modal-close');

        if (!modal.classList.contains('non-dismissible')) {
            overlay.addEventListener('click', () => closeModal(modalId));
        }

        if (closeBtn) {
            closeBtn.addEventListener('click', () => closeModal(modalId));
        }

        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && !modal.classList.contains('non-dismissible')) {
                closeModal(modalId);
            }
        });
    });
}

function openModal(modalId) {
    const modal = document.getElementById(modalId);
    modal.classList.add('active');
    // Auto-focus primary button for gamepad
    setTimeout(() => {
        const btn = modal.querySelector('.btn-primary');
        if (btn) setGamepadFocus(btn);
    }, 100);
}

function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    modal.classList.remove('active');
}

// App Detail Modal
function openAppDetail(app) {
    const content = document.getElementById('appModalContent');

    content.innerHTML = '';

    const bigPicSrc = getImageURL('bigpics', app.bigPicUrl);
    if (bigPicSrc) {
        const bigPic = document.createElement('img');
        bigPic.className = 'app-detail-bigpic';
        bigPic.alt = app.name || '';
        bigPic.src = bigPicSrc;
        content.appendChild(bigPic);
    }

    const name = document.createElement('h2');
    name.className = 'app-detail-name';
    name.textContent = app.name;

    const desc = document.createElement('p');
    desc.className = 'app-detail-desc';
    desc.textContent = app.description || 'No description available';

    const tags = document.createElement('div');
    tags.className = 'app-detail-tags';
    if (app.categoryName) {
        const tag = document.createElement('span');
        tag.className = 'app-tag';
        tag.textContent = app.categoryName;
        tags.appendChild(tag);
    }

    content.appendChild(name);
    content.appendChild(desc);
    content.appendChild(tags);

    const actions = document.createElement('div');
    actions.className = 'app-detail-actions';

    const launchBtn = document.createElement('button');
    launchBtn.className = 'btn-primary';
    launchBtn.textContent = '启动游戏';
    launchBtn.addEventListener('click', () => {
        if (!isLoggedIn && !isFreePlay) {
            showToast('请先登录', 'warning');
            return;
        }
        if (window.NativeBridge && window.NativeBridge.launchGame) {
            const launchData = {
                id: app.id, name: app.name,
                launchType: app.launchType || 'moonlight',
                launchParam: app.launchParam || '',
                retroarchCore: app.retroarchCore || '',
                retroarchRom: app.retroarchRom || '',
                authToken: authToken || '',
                sessionId: activeSessionId || '',
                freePlay: isFreePlay,
                hostId: HOST_ID, machineCode: MACHINE_CODE
            };
            console.log('Launching game:', app.name, 'type:', app.launchType, 'param:', app.launchParam);
            apiPost('/api/v1/stats/clicks', { machineCode: MACHINE_CODE, clicks: [{ appId: app.id, count: 1 }] }).catch(() => {});
            window.NativeBridge.launchGame(JSON.stringify(launchData));
            closeModal('appModal');
        } else {
            showToast('此设备不支持启动游戏', 'warning');
        }
    });

    const closeBtn = document.createElement('button');
    closeBtn.className = 'btn-secondary';
    closeBtn.textContent = '关闭';
    closeBtn.addEventListener('click', () => closeModal('appModal'));

    actions.appendChild(launchBtn);
    actions.appendChild(closeBtn);
    content.appendChild(actions);

    openModal('appModal');
}

// ═══════════════════════════════════════════════════════════════════════════
// NATIVE EVENT BRIDGE (Android WebView ↔ Native)
// ═══════════════════════════════════════════════════════════════════════════

const LOGGED_IN_LABELS = ['手感滚烫', '五星玩家', '今日上上签'];

function updateAuthUI() {
    // Top-nav status label
    const userIcon = document.getElementById('userIcon');
    if (userIcon) {
        const loggedIn = isLoggedIn && currentUser;
        userIcon.textContent = loggedIn
            ? LOGGED_IN_LABELS[Math.floor(Math.random() * LOGGED_IN_LABELS.length)]
            : '扫码开玩';
        userIcon.classList.toggle('shiny', !!loggedIn);
    }

    // Re-render sidebar QR section based on login state
    const qrLogin = document.getElementById('qrLogin');
    if (qrLogin) {
        if (isLoggedIn && currentUser) {
            qrLogin.innerHTML = '';
            const userCard = document.createElement('div');
            userCard.className = 'qr-login';
            userCard.id = 'qrLogin';
            userCard.innerHTML = '<h3 class="qr-login-title">' + escapeHtml(currentUser.nickname || '用户') + '</h3>';
            if (currentUser.avatarURL) {
                const avatar = document.createElement('img');
                avatar.src = currentUser.avatarURL;
                avatar.style.cssText = 'width:80px;height:80px;border-radius:50%;margin:8px auto;display:block;';
                userCard.appendChild(avatar);
            }
            var logoutBtn = document.createElement('button');
            logoutBtn.className = 'user-card-logout';
            logoutBtn.textContent = '退出登录';
            logoutBtn.addEventListener('click', function() {
                if (typeof NativeBridge !== 'undefined' && NativeBridge.logout) {
                    NativeBridge.logout();
                }
            });
            userCard.appendChild(logoutBtn);
            qrLogin.parentNode.replaceChild(userCard, qrLogin);
        }
    }
}

function displayRemainingTime(seconds) {
    var el = document.getElementById('billingRemaining');
    if (!el) {
        // Create the element inside the billing bar if it exists
        var bar = document.querySelector('.billing-bar');
        if (!bar) return;
        el = document.createElement('div');
        el.id = 'billingRemaining';
        el.style.cssText = 'margin-top:4px;font-size:13px;color:#ffd700;';
        bar.appendChild(el);
    }
    var mins = Math.floor(seconds / 60);
    var secs = seconds % 60;
    el.textContent = '剩余时间: ' + mins + '分' + (secs < 10 ? '0' : '') + secs + '秒';
}

function stopCountdownTimer() {
    if (countdownTimer) {
        clearTimeout(countdownTimer);
        countdownTimer = null;
    }
}

function computeRemainingSeconds() {
    if (!billingServerTs) return 0;
    var elapsed = (Date.now() / 1000) - billingServerTs;
    if (elapsed < 0) elapsed = 0;
    // billingRemainingBase already combines tenant + global balance (divided across
    // sibling sessions) server-side, using each balance's real rate. Re-deriving it
    // here from billingPlanRateCents would be wrong whenever tenant is the active
    // source, since that plan's rate is 0 (tenant billing is flat per-minute).
    return Math.max(0, Math.floor(billingRemainingBase - elapsed));
}

function setBillingParams(data) {
    if (data.balance_cents !== undefined) billingBalanceCents = data.balance_cents;
    if (data.plan_rate_cents !== undefined) billingPlanRateCents = data.plan_rate_cents;
    if (data.plan_unit_seconds !== undefined) billingPlanUnitSeconds = data.plan_unit_seconds;
    if (data.server_ts !== undefined) billingServerTs = data.server_ts;
    if (data.other_session_count !== undefined) otherSessionCount = data.other_session_count;
    if (data.balance_source !== undefined) billingBalanceSource = data.balance_source;
    if (data.remaining_seconds !== undefined) billingRemainingBase = data.remaining_seconds;
    if (data.tenant_balance_minutes !== undefined) billingTenantMinutes = data.tenant_balance_minutes;
    if (data.global_balance_cents !== undefined) billingGlobalCents = data.global_balance_cents;
    startCountdownDisplay();
}

function startCountdownDisplay() {
    stopCountdownTimer();
    function tick() {
        var remaining = computeRemainingSeconds();
        displayRemainingTime(remaining);
        if (remaining > 0) {
            countdownTimer = setTimeout(tick, 1000);
        }
    }
    tick();
}

window.onNativeEvent = function(type, data) {
    console.log('[STATE] onNativeEvent type=' + type + ' state=' + (data && data.state) + ' event=' + (data && data.event) + ' isLoggedIn=' + isLoggedIn + ' dataLoaded=' + dataLoaded);
    if (!dataLoaded) {
        console.log('[STATE] DROPPED: dataLoaded=false');
        return;
    }
    if (type === 'device_state_change') {
        if (data.event === 'balance_correction') {
            // balance_correction from server (recharge or sibling login)
            setBillingParams(data);
        } else if (data.state === 'logged_in' && data.user) {
            console.log('[STATE] → LOGIN: user=' + data.user.nickname);
            isLoggedIn = true;
            currentUser = { nickname: data.user.nickname, avatarURL: data.user.avatar_url };
            authToken = data.token || null;
            activeSessionId = data.session_id || null;
            updateAuthUI();
            // Initialize countdown from login/reconnect data
            setBillingParams(data);
            if (currentPage === 'home') setPage('home');
        } else if (data.state === 'idle' || data.state === 'need_recharge') {
            console.log('[STATE] → IDLE/LOGOUT: event=' + data.event + ' isFreePlay=' + isFreePlay);
            console.trace('[STATE] idle trace');
            stopCountdownTimer();
            otherSessionCount = 0;
            billingServerTs = 0;
            billingBalanceCents = 0;
            billingRemainingBase = 0;
            billingBalanceSource = '';
            billingTenantMinutes = 0;
            billingGlobalCents = 0;
            if (!isFreePlay) {
                isLoggedIn = false;
                currentUser = null;
                authToken = null;
                activeSessionId = null;
                updateAuthUI();
                if (currentPage === 'home') setPage('home');
            }
        }
    } else if (type === 'balance_update') {
        setBillingParams(data);
    } else if (type === 'session_end') {
        console.log('[STATE] → SESSION_END');
        console.trace('[STATE] session_end trace');
        stopCountdownTimer();
        otherSessionCount = 0;
        billingServerTs = 0;
        billingBalanceCents = 0;
        billingRemainingBase = 0;
        billingBalanceSource = '';
        if (!isFreePlay) {
            isLoggedIn = false;
            currentUser = null;
            authToken = null;
            activeSessionId = null;
            updateAuthUI();
            if (currentPage === 'home') setPage('home');
        }
    } else if (type === 'free_play_change') {
        isFreePlay = data.free_play === true;
        if (isFreePlay) {
            isLoggedIn = true;
        } else if (!currentUser) {
            isLoggedIn = false;
        }
        // Re-render current page to update sidebar (QR vs free-play badge)
        setPage(currentPage);
    } else if (type === 'save_sync_status') {
        updateSaveSyncStatus(data.completed, data.total, data.done === true);
    }
};

window.onGameExited = function(data) {
    showToast('游戏已退出', 'info');
    resetScreensaverTimer();
};

window.onGameLaunchError = function(errorMsg) {
    showToast(errorMsg, 'error');
};

// ═══════════════════════════════════════════════════════════════════════════
// GAMEPAD NAVIGATION
// ═══════════════════════════════════════════════════════════════════════════

let gamepadFocusEl = null;

function initGamepadNav() {
    document.addEventListener('keydown', (e) => {
        if (!isTvMode && screensaverActive) {
            hideScreensaver();
            return;
        }

        const dir = { ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right' }[e.key];
        if (dir) {
            e.preventDefault();
            spatialNavigate(dir);
            return;
        }
        if (e.key === 'Enter') {
            e.preventDefault();
            clickFocusedElement();
            return;
        }
        if (e.key === 'Escape') {
            e.preventDefault();
            goBack();
            return;
        }
    });
}

// Called from Android native layer
window.onGamepadDirection = function(dir) {
    if (!isTvMode) {
        resetScreensaverTimer();
        if (screensaverActive) { hideScreensaver(); return; }
    }
    spatialNavigate(dir);
};

var navPages = ['home', 'library', 'rankings'];

window.onGamepadButton = function(btn) {
    if (!isTvMode) {
        resetScreensaverTimer();
        if (screensaverActive) { hideScreensaver(); return; }
    }
    if (btn === 'a') clickFocusedElement();
    else if (btn === 'b') goBack();
    else if (btn === 'l1' || btn === 'r1') {
        var idx = navPages.indexOf(currentPage);
        if (idx === -1) idx = 0;
        if (btn === 'l1') idx = (idx - 1 + navPages.length) % navPages.length;
        else idx = (idx + 1) % navPages.length;
        setPage(navPages[idx]);
    }
};

function getFocusableElements() {
    // If modal is open, only return elements inside the modal
    var activeModal = document.querySelector('.modal.active');
    if (activeModal) {
        var buttons = activeModal.querySelectorAll('.btn-primary, .btn-secondary');
        return Array.from(buttons).filter(function(el) {
            return el.offsetWidth > 0 && el.offsetHeight > 0;
        });
    }

    var selectors = [
        '#logoBadge',
        '.nav-tab',
        '#heroCard',
        '.home-grid .app-card',
        '.category-pill',
        '.all-games-grid .app-card',
        '.show-category-btn',
        '.category-item',
        '.library-content .app-card',
        '.rank-entry',
        '.user-card-logout'
    ];

    var elements = document.querySelectorAll(selectors.join(','));
    return Array.from(elements).filter(function(el) {
        return el.offsetWidth > 0 && el.offsetHeight > 0;
    });
}

function spatialNavigate(direction) {
    var focusable = getFocusableElements();
    if (focusable.length === 0) return;

    // If nothing focused, focus first element
    if (!gamepadFocusEl || !document.contains(gamepadFocusEl)) {
        setGamepadFocus(focusable[0]);
        return;
    }

    var currentRect = gamepadFocusEl.getBoundingClientRect();
    var cx = currentRect.left + currentRect.width / 2;
    var cy = currentRect.top + currentRect.height / 2;

    var bestEl = null;
    var bestScore = Infinity;

    for (var i = 0; i < focusable.length; i++) {
        var el = focusable[i];
        if (el === gamepadFocusEl) continue;

        var rect = el.getBoundingClientRect();
        var ex = rect.left + rect.width / 2;
        var ey = rect.top + rect.height / 2;

        var dx = ex - cx;
        var dy = ey - cy;

        var axisDistance, crossDistance;
        var inDirection = false;

        switch (direction) {
            case 'up':
                axisDistance = cy - ey;
                crossDistance = Math.abs(dx);
                inDirection = axisDistance > 0;
                break;
            case 'down':
                axisDistance = ey - cy;
                crossDistance = Math.abs(dx);
                inDirection = axisDistance > 0;
                break;
            case 'left':
                axisDistance = cx - ex;
                crossDistance = Math.abs(dy);
                inDirection = axisDistance > 0;
                break;
            case 'right':
                axisDistance = ex - cx;
                crossDistance = Math.abs(dy);
                inDirection = axisDistance > 0;
                break;
        }

        if (!inDirection) continue;

        // Score: primary axis distance + cross-axis penalty
        var score = axisDistance + crossDistance * 3;

        if (score < bestScore) {
            bestScore = score;
            bestEl = el;
        }
    }

    if (bestEl) {
        setGamepadFocus(bestEl);
    }
}

function setGamepadFocus(el) {
    if (gamepadFocusEl) {
        gamepadFocusEl.classList.remove('gamepad-focus');
    }
    gamepadFocusEl = el;
    el.classList.add('gamepad-focus');
    el.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'smooth' });
}

function clearGamepadFocus() {
    if (gamepadFocusEl) {
        gamepadFocusEl.classList.remove('gamepad-focus');
        gamepadFocusEl = null;
    }
}

function clickFocusedElement() {
    if (gamepadFocusEl) {
        gamepadFocusEl.click();
    }
}

function goBack() {
    // Close modal if open
    var activeModal = document.querySelector('.modal.active');
    if (activeModal && !activeModal.classList.contains('non-dismissible')) {
        closeModal(activeModal.id);
        return;
    }
    // Go back to home from other pages
    if (currentPage !== 'home') {
        setPage('home');
    }
}

// ═══════════════════════════════════════════════════════════════════════════
// SCREENSAVER
// ═══════════════════════════════════════════════════════════════════════════

function initScreensaver() {
    if (isTvMode) {
        console.log('TV mode: JS screensaver disabled');
        return;
    }
    fetchScreensaverSettings();
    setupIdleDetection();
    // Re-fetch settings every 5 minutes to pick up admin changes
    screensaverSettingsTimer = setInterval(fetchScreensaverSettings, 5 * 60 * 1000);
}

async function fetchScreensaverSettings() {
    try {
        // Pass machine_code to get multi-video config if available
        var url = '/api/v1/screensaver/settings';
        if (MACHINE_CODE) {
            url += '?machine_code=' + encodeURIComponent(MACHINE_CODE);
        }
        const data = await apiGet(url);
        screensaverEnabled = data.enabled;
        screensaverTimeout = data.timeout || 5;
        screensaverHasVideo = data.has_video;

        // Multi-video: update default and tenant video info
        if (data.default_video_id) {
            defaultVideoId = data.default_video_id;
            defaultVideoHash = data.default_video_hash || '';
            screensaverHasVideo = true;
        }
        if (data.tenant_video_id) {
            tenantVideoId = data.tenant_video_id;
            tenantVideoHash = data.tenant_video_hash || '';
            tenantStartDate = data.start_date || '';
            tenantEndDate = data.end_date || '';
            tenantStartTime = data.start_time || '';
            tenantEndTime = data.end_time || '';
            screensaverHasVideo = true;
        }

        // Backward compat: if no video IDs but has old-style video_hash
        if (!defaultVideoId && !tenantVideoId) {
            var video = document.getElementById('screensaverVideo');
            if (video && data.video_hash && data.video_hash !== screensaverVideoHash) {
                screensaverVideoHash = data.video_hash;
                video.src = '/api/v1/screensaver/video?h=' + screensaverVideoHash;
                video.load();
            }
        }

        resetScreensaverTimer();
    } catch (e) {
        console.log('Screensaver settings not available');
    }
}

function setupIdleDetection() {
    const events = ['touchstart', 'mousemove', 'mousedown', 'keydown', 'click', 'scroll'];
    events.forEach(function(evt) {
        document.addEventListener(evt, onUserActivity, { passive: true });
    });
    resetScreensaverTimer();
}

function onUserActivity() {
    if (screensaverActive) {
        hideScreensaver();
    } else {
        resetScreensaverTimer();
    }
}

function resetScreensaverTimer() {
    if (screensaverIdleTimer) {
        clearTimeout(screensaverIdleTimer);
        screensaverIdleTimer = null;
    }
    if (screensaverEnabled && screensaverHasVideo) {
        screensaverIdleTimer = setTimeout(showScreensaver, screensaverTimeout * 60 * 1000);
    }
}

function isTenantVideoActive() {
    if (!tenantVideoId) return false;

    var now = new Date();

    // Check date range
    if (tenantStartDate) {
        var start = new Date(tenantStartDate + 'T00:00:00');
        if (now < start) return false;
    }
    if (tenantEndDate) {
        var end = new Date(tenantEndDate + 'T23:59:59');
        if (now > end) return false;
    }

    // Check time range
    if (tenantStartTime || tenantEndTime) {
        var hhmm = ('0' + now.getHours()).slice(-2) + ':' + ('0' + now.getMinutes()).slice(-2);
        var startT = tenantStartTime || '00:00';
        var endT = tenantEndTime || '23:59';
        if (hhmm < startT || hhmm > endT) return false;
    }

    return true;
}

function getScreensaverVideoUrl() {
    // Multi-video: pick tenant video if active, else global default
    var videoId, videoHash;
    if (isTenantVideoActive()) {
        videoId = tenantVideoId;
        videoHash = tenantVideoHash;
    } else if (defaultVideoId) {
        videoId = defaultVideoId;
        videoHash = defaultVideoHash;
    }

    if (videoId && videoHash) {
        // Android: local-only, never stream from server
        if (typeof NativeBridge !== 'undefined' && NativeBridge.getScreensaverVideoUrl) {
            return NativeBridge.getScreensaverVideoUrl(videoId, videoHash);
        }
        // Non-Android (browser testing): stream from server
        return '/api/v1/screensaver/videos/' + videoId + '?h=' + videoHash;
    }

    // Backward compat: fall back to old single-video endpoint
    if (screensaverVideoHash) {
        return '/api/v1/screensaver/video?h=' + screensaverVideoHash;
    }
    return '';
}

function showScreensaver() {
    if (screensaverActive || !screensaverEnabled || !screensaverHasVideo) return;

    var overlay = document.getElementById('screensaverOverlay');
    var video = document.getElementById('screensaverVideo');
    if (!overlay || !video) return;

    var videoUrl = getScreensaverVideoUrl();
    if (!videoUrl) return;

    screensaverActive = true;
    overlay.classList.add('active');

    // Update video source if it changed
    var currentSrc = video.getAttribute('src') || '';
    if (currentSrc !== videoUrl) {
        video.src = videoUrl;
        video.load();
    }
    // Robust loop: ended event + stall detection + error recovery
    if (!video._loopHandler) {
        video._loopHandler = true;
        video.addEventListener('ended', function() {
            console.log('Screensaver video ended, restarting');
            video.currentTime = 0;
            video.play().catch(function(e) { console.log('Screensaver loop restart failed:', e); });
        });
        video.addEventListener('error', function() {
            console.log('Screensaver video error, reloading in 2s');
            setTimeout(function() {
                if (screensaverActive) {
                    video.src = getScreensaverVideoUrl();
                    video.load();
                    video.play().catch(function(e) { console.log('Screensaver error recovery failed:', e); });
                }
            }, 2000);
        });
        // Stall detection: if video pauses unexpectedly while screensaver is active, restart
        video._stallCheck = setInterval(function() {
            if (screensaverActive && video.paused && !video.ended) {
                console.log('Screensaver video stalled (paused=' + video.paused + ' time=' + video.currentTime + '), restarting');
                video.play().catch(function(e) {
                    // If play fails, reload the source
                    console.log('Screensaver stall recovery play failed, reloading');
                    video.src = getScreensaverVideoUrl();
                    video.load();
                    video.play().catch(function() {});
                });
            }
        }, 5000);
    }
    video.play().catch(function(e) { console.log('Screensaver video play failed:', e); });
}

function hideScreensaver() {
    var overlay = document.getElementById('screensaverOverlay');
    var video = document.getElementById('screensaverVideo');
    if (!overlay || !video) return;

    screensaverActive = false;
    overlay.classList.remove('active');
    video.pause();
    resetScreensaverTimer();
}

// Called from native side (MainActivity.dispatchTouchEvent)
window.onScreensaverReset = function() {
    if (screensaverActive) {
        hideScreensaver();
    } else {
        resetScreensaverTimer();
    }
};
