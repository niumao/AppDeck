const { ipcRenderer } = require('electron');

let currentCategoryId = '';
let currentSortMode = 'default';
let allApps = [];
let allCategories = [];
let cachePath = '';

// 初始化
document.addEventListener('DOMContentLoaded', async () => {
  // 获取缓存路径
  cachePath = await ipcRenderer.invoke('get-cache-path');
  
  // 加载数据
  await loadData();
  
  // 绑定事件
  bindEvents();
  
  // 检查更新
  checkForUpdates();
});

// 加载数据
async function loadData() {
  try {
    showLoading();
    
    // 加载分类
    allCategories = await ipcRenderer.invoke('get-categories');
    renderCategories();
    
    // 加载应用
    await loadApps();
    
    hideLoading();
  } catch (error) {
    console.error('加载数据失败:', error);
    hideLoading();
    showEmptyMessage();
  }
}

// 加载应用列表
async function loadApps() {
  if (currentSortMode === 'default') {
    allApps = await ipcRenderer.invoke('get-apps', currentCategoryId || null);
  } else if (currentSortMode === 'popular') {
    // 暂时也用 get-apps，后续可以添加按点击数排序的 IPC
    allApps = await ipcRenderer.invoke('get-apps', currentCategoryId || null);
    // 前端排序
    allApps.sort((a, b) => (b.clickCount || 0) - (a.clickCount || 0));
  }
  renderApps();
}

// 渲染分类列表
function renderCategories() {
  const categoryList = document.getElementById('categoryList');
  categoryList.innerHTML = '';
  
  allCategories.forEach(category => {
    const button = document.createElement('button');
    button.className = 'category-item';
    button.dataset.categoryId = category.id;
    
    button.innerHTML = `
      <span>${category.name}</span>
      <span class="badge" id="count-${category.id}">0</span>
    `;
    
    button.addEventListener('click', () => {
      selectCategory(category.id);
    });
    
    categoryList.appendChild(button);
  });
  
  // 更新应用数量
  updateCategoryCounts();
}

// 更新分类应用数量
async function updateCategoryCounts() {
  const allAppsData = await ipcRenderer.invoke('get-apps', null);
  
  // 总数
  document.getElementById('totalCount').textContent = allAppsData.length;
  
  // 各分类数量（需要遍历应用统计）
  const counts = {};
  for (const app of allAppsData) {
    const detail = await ipcRenderer.invoke('get-app-detail', app.id);
    if (detail && detail.categories) {
      detail.categories.forEach(cat => {
        counts[cat.id] = (counts[cat.id] || 0) + 1;
      });
    }
  }
  
  allCategories.forEach(category => {
    const countEl = document.getElementById(`count-${category.id}`);
    if (countEl) {
      countEl.textContent = counts[category.id] || 0;
    }
  });
}

// 渲染应用列表
function renderApps() {
  const appGrid = document.getElementById('appGrid');
  appGrid.innerHTML = '';
  
  if (allApps.length === 0) {
    showEmptyMessage();
    return;
  }
  
  hideEmptyMessage();
  
  allApps.forEach(app => {
    const card = createAppCard(app);
    appGrid.appendChild(card);
  });
}

// 创建应用卡片
function createAppCard(app) {
  const card = document.createElement('div');
  card.className = 'app-card';
  
  // 图标路径（从本地缓存）
  const iconPath = app.iconUrl ? `file://${cachePath}/icons/${app.iconUrl.split('/').pop()}` : '';
  
  card.innerHTML = `
    <img src="${iconPath}" alt="${app.name}" class="app-icon" onerror="this.src='data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 width=%22100%22 height=%22100%22><rect width=%22100%22 height=%22100%22 fill=%22%23ccc%22/></svg>'">
    <div class="app-name">${app.name}</div>
    <div class="app-stats">点击: ${app.clickCount || 0}</div>
  `;
  
  card.addEventListener('click', () => {
    showAppDetail(app.id);
  });
  
  return card;
}

// 显示应用详情
async function showAppDetail(appId) {
  const app = await ipcRenderer.invoke('get-app-detail', appId);
  if (!app) return;
  
  const modal = document.getElementById('appModal');
  const appDetail = document.getElementById('appDetail');
  
  const iconPath = app.iconUrl ? `file://${cachePath}/icons/${app.iconUrl.split('/').pop()}` : '';
  const bigPicPath = app.bigPicUrl ? `file://${cachePath}/bigpics/${app.bigPicUrl.split('/').pop()}` : '';
  
  // 分类标签
  const categoriesHtml = app.categories
    ? app.categories.map(cat => `<span class="category-tag">${cat.name}</span>`).join('')
    : '';
  
  appDetail.innerHTML = `
    <img src="${iconPath}" alt="${app.name}" class="app-detail-icon" onerror="this.style.display='none'">
    <div class="app-detail-name">${app.name}</div>
    <div class="app-detail-description">${app.description || '暂无描述'}</div>
    <div class="app-detail-categories">${categoriesHtml}</div>
    ${bigPicPath ? `<img src="${bigPicPath}" alt="${app.name}" class="app-detail-bigpic" onerror="this.style.display='none'">` : ''}
    <button class="btn-primary" onclick="launchApp('${app.id}', '${app.launchParam}')">启动应用</button>
  `;
  
  modal.style.display = 'block';
}

// 启动应用
async function launchApp(appId, launchParam) {
  try {
    await ipcRenderer.invoke('launch-app', appId, launchParam);
    closeModal('appModal');
    updateStatus('应用已启动');
  } catch (error) {
    console.error('启动失败:', error);
    updateStatus('启动失败: ' + error.message);
  }
}

// 选择分类
function selectCategory(categoryId) {
  currentCategoryId = categoryId;
  
  // 更新UI
  document.querySelectorAll('.category-item').forEach(btn => {
    btn.classList.remove('active');
  });
  
  if (categoryId) {
    document.querySelector(`[data-category-id="${categoryId}"]`).classList.add('active');
  } else {
    document.querySelector('[data-category-id=""]').classList.add('active');
  }
  
  // 重新加载应用
  loadApps();
}

// 搜索应用
function searchApps(keyword) {
  if (!keyword.trim()) {
    loadApps();
    return;
  }
  
  const filtered = allApps.filter(app =>
    app.name.toLowerCase().includes(keyword.toLowerCase()) ||
    (app.description && app.description.toLowerCase().includes(keyword.toLowerCase()))
  );
  
  allApps = filtered;
  renderApps();
}

// 检查更新
async function checkForUpdates() {
  try {
    const result = await ipcRenderer.invoke('check-update');
    if (result.hasUpdate) {
      showUpdateModal(result);
    }
  } catch (error) {
    console.error('检查更新失败:', error);
  }
}

// 显示更新对话框
function showUpdateModal(updateInfo) {
  const modal = document.getElementById('updateModal');
  const updateInfoDiv = document.getElementById('updateInfo');
  
  updateInfoDiv.innerHTML = `
    <p>发现新版本可用！</p>
    <p><strong>服务器版本:</strong> ${updateInfo.serverInfo.packageVersion}</p>
    <p><strong>本地版本:</strong> ${updateInfo.localInfo ? updateInfo.localInfo.packageVersion : '无'}</p>
    <p><strong>数据版本:</strong> ${updateInfo.serverInfo.dbVersion}</p>
    <p>是否立即下载更新？</p>
  `;
  
  modal.style.display = 'block';
}

// 下载更新
async function downloadUpdate() {
  closeModal('updateModal');
  updateStatus('正在下载更新...');
  
  try {
    await ipcRenderer.invoke('download-update');
    updateStatus('更新完成，正在刷新...');
    
    // 重新加载数据
    setTimeout(async () => {
      await loadData();
      updateStatus('更新成功');
    }, 1000);
  } catch (error) {
    console.error('更新失败:', error);
    updateStatus('更新失败: ' + error.message);
  }
}

// 绑定事件
function bindEvents() {
  // 刷新按钮
  document.getElementById('refreshBtn').addEventListener('click', () => {
    loadData();
  });
  
  // 检查更新按钮
  document.getElementById('updateBtn').addEventListener('click', () => {
    checkForUpdates();
  });
  
  // 搜索框
  const searchInput = document.getElementById('searchInput');
  let searchTimeout;
  searchInput.addEventListener('input', (e) => {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      searchApps(e.target.value);
    }, 300);
  });
  
  // 排序选项
  document.querySelectorAll('input[name="sort"]').forEach(radio => {
    radio.addEventListener('change', (e) => {
      currentSortMode = e.target.value;
      loadApps();
    });
  });
  
  // 下载数据按钮
  document.getElementById('downloadDataBtn').addEventListener('click', () => {
    downloadUpdate();
  });
  
  // 更新对话框按钮
  document.getElementById('downloadUpdateBtn').addEventListener('click', () => {
    downloadUpdate();
  });
  
  document.getElementById('cancelUpdateBtn').addEventListener('click', () => {
    closeModal('updateModal');
  });
  
  // 关闭模态框
  document.querySelectorAll('.close').forEach(closeBtn => {
    closeBtn.addEventListener('click', (e) => {
      const modal = e.target.closest('.modal');
      if (modal) {
        modal.style.display = 'none';
      }
    });
  });
  
  // 点击模态框外部关闭
  window.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal')) {
      e.target.style.display = 'none';
    }
  });
}

// UI 辅助函数
function showLoading() {
  document.getElementById('loadingMessage').style.display = 'block';
  document.getElementById('appGrid').style.display = 'none';
  document.getElementById('emptyMessage').style.display = 'none';
}

function hideLoading() {
  document.getElementById('loadingMessage').style.display = 'none';
  document.getElementById('appGrid').style.display = 'grid';
}

function showEmptyMessage() {
  document.getElementById('appGrid').style.display = 'none';
  document.getElementById('emptyMessage').style.display = 'block';
}

function hideEmptyMessage() {
  document.getElementById('emptyMessage').style.display = 'none';
}

function closeModal(modalId) {
  document.getElementById(modalId).style.display = 'none';
}

function updateStatus(message) {
  const statusText = document.getElementById('statusText');
  statusText.textContent = message;
  setTimeout(() => {
    statusText.textContent = '';
  }, 3000);
}

// 暴露给全局（用于内联事件）
window.launchApp = launchApp;

