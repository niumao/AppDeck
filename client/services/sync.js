const axios = require('axios');
const fs = require('fs');
const path = require('path');
const AdmZip = require('adm-zip');

class SyncService {
  constructor(serverUrl, localDataPath, cachePath, db) {
    this.serverUrl = serverUrl;
    this.localDataPath = localDataPath;
    this.cachePath = cachePath;
    this.db = db;
    this.packageInfoPath = path.join(localDataPath, 'package_info.json');
    this.dbPath = path.join(localDataPath, 'app.db');
  }

  // 获取本地包信息
  getLocalPackageInfo() {
    if (fs.existsSync(this.packageInfoPath)) {
      const data = fs.readFileSync(this.packageInfoPath, 'utf8');
      return JSON.parse(data);
    }
    return null;
  }

  // 保存本地包信息
  saveLocalPackageInfo(info) {
    fs.writeFileSync(
      this.packageInfoPath,
      JSON.stringify(info, null, 2),
      'utf8'
    );
  }

  // 检查是否需要同步
  async checkAndSync() {
    const localInfo = this.getLocalPackageInfo();
    
    // 首次启动，需要下载
    if (!localInfo || !fs.existsSync(this.dbPath)) {
      console.log('首次启动，开始下载数据...');
      return this.downloadAndUpdate();
    }
    
    // 检查服务器版本
    const hasUpdate = await this.checkUpdate();
    if (hasUpdate) {
      console.log('发现新版本');
    }
    
    return { hasUpdate };
  }

  // 检查更新
  async checkUpdate() {
    try {
      const response = await axios.get(`${this.serverUrl}/api/v1/client/package-info`);
      const serverInfo = response.data.data;
      
      const localInfo = this.getLocalPackageInfo();
      
      // 比对版本号
      if (!localInfo || serverInfo.packageVersion > localInfo.packageVersion) {
        return {
          hasUpdate: true,
          serverInfo,
          localInfo,
        };
      }
      
      return { hasUpdate: false };
    } catch (error) {
      console.error('检查更新失败:', error.message);
      return { hasUpdate: false, error: error.message };
    }
  }

  // 下载并更新
  async downloadAndUpdate() {
    try {
      console.log('开始下载更新...');
      
      // 1. 下载数据库文件
      console.log('下载数据库...');
      await this.downloadDatabase();
      
      // 2. 下载资源包
      console.log('下载资源包...');
      await this.downloadResources();
      
      // 3. 获取服务器包信息并保存
      const response = await axios.get(`${this.serverUrl}/api/v1/client/package-info`);
      const serverInfo = response.data.data;
      
      this.saveLocalPackageInfo({
        packageVersion: serverInfo.packageVersion,
        dbVersion: serverInfo.dbVersion,
        downloadedAt: new Date().toISOString(),
      });
      
      // 4. 重新打开数据库
      if (this.db) {
        this.db.close();
        this.db.open();
      }
      
      console.log('更新完成');
      return { success: true };
    } catch (error) {
      console.error('更新失败:', error.message);
      throw error;
    }
  }

  // 下载数据库文件
  async downloadDatabase() {
    const response = await axios.get(
      `${this.serverUrl}/api/v1/client/database`,
      { responseType: 'arraybuffer' }
    );
    
    // 保存到临时文件
    const tempPath = this.dbPath + '.tmp';
    fs.writeFileSync(tempPath, response.data);
    
    // 关闭现有数据库连接
    if (this.db) {
      this.db.close();
    }
    
    // 替换数据库文件
    if (fs.existsSync(this.dbPath)) {
      fs.unlinkSync(this.dbPath);
    }
    fs.renameSync(tempPath, this.dbPath);
  }

  // 下载资源包
  async downloadResources() {
    const response = await axios.get(
      `${this.serverUrl}/api/v1/client/resources`,
      { responseType: 'arraybuffer' }
    );
    
    // 保存 ZIP 文件
    const zipPath = path.join(this.localDataPath, 'resources.zip');
    fs.writeFileSync(zipPath, response.data);
    
    // 解压
    const zip = new AdmZip(zipPath);
    zip.extractAllTo(this.cachePath, true);
    
    // 删除 ZIP 文件
    fs.unlinkSync(zipPath);
    
    console.log('资源包解压完成');
  }
}

module.exports = SyncService;

