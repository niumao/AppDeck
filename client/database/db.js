const Database = require('better-sqlite3');
const fs = require('fs');

class AppDatabase {
  constructor(dbPath) {
    this.dbPath = dbPath;
    this.db = null;
    
    // 如果数据库文件存在，则打开
    if (fs.existsSync(dbPath)) {
      this.open();
    }
  }

  open() {
    if (!fs.existsSync(this.dbPath)) {
      throw new Error('数据库文件不存在');
    }
    this.db = new Database(this.dbPath);
  }

  close() {
    if (this.db) {
      this.db.close();
      this.db = null;
    }
  }

  isReady() {
    return this.db !== null;
  }

  // 获取所有分类
  getCategories() {
    if (!this.isReady()) return [];
    
    const stmt = this.db.prepare('SELECT * FROM categories ORDER BY order_num ASC');
    return stmt.all();
  }

  // 获取所有应用（可选按分类筛选）
  getApps(categoryId = null) {
    if (!this.isReady()) return [];
    
    if (categoryId) {
      // 通过关联表筛选
      const stmt = this.db.prepare(`
        SELECT DISTINCT a.* 
        FROM apps a
        JOIN app_categories ac ON a.id = ac.app_id
        WHERE ac.category_id = ?
        ORDER BY a.order_num ASC
      `);
      return stmt.all(categoryId);
    } else {
      // 获取所有应用
      const stmt = this.db.prepare('SELECT * FROM apps ORDER BY order_num ASC');
      return stmt.all();
    }
  }

  // 根据点击数排序获取应用
  getAppsByClickCount(limit = 20) {
    if (!this.isReady()) return [];
    
    const stmt = this.db.prepare(`
      SELECT * FROM apps 
      ORDER BY click_count DESC 
      LIMIT ?
    `);
    return stmt.all(limit);
  }

  // 获取应用详情（包含分类信息）
  getAppById(appId) {
    if (!this.isReady()) return null;
    
    const stmt = this.db.prepare('SELECT * FROM apps WHERE id = ?');
    const app = stmt.get(appId);
    
    if (app) {
      // 获取应用的所有分类
      const categoriesStmt = this.db.prepare(`
        SELECT c.* 
        FROM categories c
        JOIN app_categories ac ON c.id = ac.category_id
        WHERE ac.app_id = ?
        ORDER BY c.order_num ASC
      `);
      app.categories = categoriesStmt.all(appId);
    }
    
    return app;
  }

  // 获取应用的分类ID列表
  getAppCategoryIds(appId) {
    if (!this.isReady()) return [];
    
    const stmt = this.db.prepare(`
      SELECT category_id 
      FROM app_categories 
      WHERE app_id = ?
    `);
    return stmt.all(appId).map(row => row.category_id);
  }

  // 搜索应用
  searchApps(keyword) {
    if (!this.isReady()) return [];
    
    const stmt = this.db.prepare(`
      SELECT * FROM apps 
      WHERE name LIKE ? OR description LIKE ?
      ORDER BY order_num ASC
    `);
    const searchTerm = `%${keyword}%`;
    return stmt.all(searchTerm, searchTerm);
  }

  // 获取版本信息
  getVersion() {
    if (!this.isReady()) return null;
    
    const stmt = this.db.prepare('SELECT * FROM version WHERE id = 1');
    return stmt.get();
  }

  // 获取统计信息
  getStats() {
    if (!this.isReady()) return { totalApps: 0, totalCategories: 0 };
    
    const appsCount = this.db.prepare('SELECT COUNT(*) as count FROM apps').get();
    const categoriesCount = this.db.prepare('SELECT COUNT(*) as count FROM categories').get();
    
    return {
      totalApps: appsCount.count,
      totalCategories: categoriesCount.count,
    };
  }
}

module.exports = AppDatabase;

