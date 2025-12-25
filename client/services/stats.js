const axios = require('axios');
const fs = require('fs');
const path = require('path');

class StatsService {
  constructor(serverUrl, localDataPath) {
    this.serverUrl = serverUrl;
    this.statsPath = path.join(localDataPath, 'stats');
    this.clicksPath = path.join(this.statsPath, 'clicks.json');
    
    // 确保目录存在
    if (!fs.existsSync(this.statsPath)) {
      fs.mkdirSync(this.statsPath, { recursive: true });
    }
    
    // 初始化点击统计文件
    if (!fs.existsSync(this.clicksPath)) {
      this.saveClicks({});
    }
  }

  // 读取本地点击统计
  loadClicks() {
    try {
      const data = fs.readFileSync(this.clicksPath, 'utf8');
      return JSON.parse(data);
    } catch (error) {
      return {};
    }
  }

  // 保存本地点击统计
  saveClicks(clicks) {
    fs.writeFileSync(this.clicksPath, JSON.stringify(clicks, null, 2), 'utf8');
  }

  // 记录点击
  async recordClick(appId) {
    const clicks = this.loadClicks();
    clicks[appId] = (clicks[appId] || 0) + 1;
    this.saveClicks(clicks);
    
    // 检查是否需要上报（累积10次）
    const totalClicks = Object.values(clicks).reduce((sum, count) => sum + count, 0);
    if (totalClicks >= 10) {
      await this.uploadPendingStats();
    }
  }

  // 上报待处理的统计
  async uploadPendingStats() {
    const clicks = this.loadClicks();
    
    // 如果没有待上报的数据，直接返回
    if (Object.keys(clicks).length === 0) {
      return { success: true, message: '没有待上报的数据' };
    }
    
    try {
      // 转换为 API 需要的格式
      const clicksArray = Object.entries(clicks).map(([appId, count]) => ({
        appId,
        count,
      }));
      
      // 上报到服务器
      await axios.post(`${this.serverUrl}/api/v1/stats/clicks`, {
        clicks: clicksArray,
      });
      
      // 上报成功，清空本地数据
      this.saveClicks({});
      
      console.log('统计数据上报成功');
      return { success: true };
    } catch (error) {
      console.error('统计数据上报失败:', error.message);
      // 上报失败，保留本地数据，下次再试
      return { success: false, error: error.message };
    }
  }

  // 获取待上报统计数量
  getPendingCount() {
    const clicks = this.loadClicks();
    return Object.values(clicks).reduce((sum, count) => sum + count, 0);
  }
}

module.exports = StatsService;

