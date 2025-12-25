const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');
const fs = require('fs');
const Database = require('./database/db');
const SyncService = require('./services/sync');
const StatsService = require('./services/stats');

let mainWindow;
let db;
let syncService;
let statsService;

// 数据目录
const userDataPath = app.getPath('userData');
const localDataPath = path.join(userDataPath, 'local_data');
const cachePath = path.join(localDataPath, 'cache');

// 确保目录存在
if (!fs.existsSync(localDataPath)) {
  fs.mkdirSync(localDataPath, { recursive: true });
}
if (!fs.existsSync(cachePath)) {
  fs.mkdirSync(cachePath, { recursive: true });
}
if (!fs.existsSync(path.join(cachePath, 'icons'))) {
  fs.mkdirSync(path.join(cachePath, 'icons'), { recursive: true });
}
if (!fs.existsSync(path.join(cachePath, 'bigpics'))) {
  fs.mkdirSync(path.join(cachePath, 'bigpics'), { recursive: true });
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      nodeIntegration: true,
      contextIsolation: false,
    },
  });

  mainWindow.loadFile('renderer/index.html');

  // 开发模式打开开发者工具
  if (process.argv.includes('--dev')) {
    mainWindow.webContents.openDevTools();
  }

  mainWindow.on('closed', function () {
    mainWindow = null;
  });
}

app.whenReady().then(() => {
  // 初始化数据库
  const dbPath = path.join(localDataPath, 'app.db');
  db = new Database(dbPath);

  // 初始化服务
  const serverUrl = process.env.SERVER_URL || 'http://localhost:8080';
  syncService = new SyncService(serverUrl, localDataPath, cachePath, db);
  statsService = new StatsService(serverUrl, localDataPath);

  createWindow();

  // 检查是否需要首次同步
  syncService.checkAndSync().catch(err => {
    console.error('同步失败:', err);
  });

  app.on('activate', function () {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on('window-all-closed', function () {
  // 退出前上报统计
  statsService.uploadPendingStats().then(() => {
    if (process.platform !== 'darwin') app.quit();
  });
});

// IPC 处理器

// 获取所有应用
ipcMain.handle('get-apps', async (event, categoryId = null) => {
  return db.getApps(categoryId);
});

// 获取所有分类
ipcMain.handle('get-categories', async () => {
  return db.getCategories();
});

// 获取应用详情
ipcMain.handle('get-app-detail', async (event, appId) => {
  return db.getAppById(appId);
});

// 启动应用
ipcMain.handle('launch-app', async (event, appId, launchParam) => {
  // 记录点击统计
  await statsService.recordClick(appId);
  
  // 调用 CALLGG 命令启动应用
  const { exec } = require('child_process');
  return new Promise((resolve, reject) => {
    exec(`CALLGG ${launchParam}`, (error, stdout, stderr) => {
      if (error) {
        console.error(`启动失败: ${error}`);
        reject(error);
        return;
      }
      resolve({ success: true });
    });
  });
});

// 检查更新
ipcMain.handle('check-update', async () => {
  return syncService.checkUpdate();
});

// 下载更新
ipcMain.handle('download-update', async () => {
  return syncService.downloadAndUpdate();
});

// 获取本地包信息
ipcMain.handle('get-local-package-info', async () => {
  return syncService.getLocalPackageInfo();
});

// 获取缓存路径
ipcMain.handle('get-cache-path', async () => {
  return cachePath;
});

