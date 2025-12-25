-- AppDeck 数据库初始化脚本
-- MySQL 5.7+ / MariaDB 10.2+

-- 创建数据库
CREATE DATABASE IF NOT EXISTS appdeck DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE appdeck;

-- 应用表
CREATE TABLE IF NOT EXISTS apps (
  id VARCHAR(36) PRIMARY KEY COMMENT '应用UUID',
  name VARCHAR(255) NOT NULL COMMENT '应用名称',
  description TEXT COMMENT '应用描述',
  icon_url VARCHAR(500) COMMENT '图标URL',
  big_pic_url VARCHAR(500) COMMENT '大图URL',
  launch_param VARCHAR(255) NOT NULL COMMENT '启动参数',
  order_num INT DEFAULT 0 COMMENT '排序序号',
  click_count INT DEFAULT 0 COMMENT '点击次数',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  INDEX idx_order (order_num),
  INDEX idx_click_count (click_count)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='应用表';

-- 分类表
CREATE TABLE IF NOT EXISTS categories (
  id VARCHAR(36) PRIMARY KEY COMMENT '分类UUID',
  name VARCHAR(100) NOT NULL COMMENT '分类名称',
  order_num INT DEFAULT 0 COMMENT '排序序号',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  INDEX idx_order (order_num),
  UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='分类表';

-- 应用分类关联表（多对多）
CREATE TABLE IF NOT EXISTS app_categories (
  app_id VARCHAR(36) NOT NULL COMMENT '应用ID',
  category_id VARCHAR(36) NOT NULL COMMENT '分类ID',
  PRIMARY KEY (app_id, category_id),
  FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE,
  FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE,
  INDEX idx_category (category_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='应用分类关联表';

-- 服务器数据版本表
CREATE TABLE IF NOT EXISTS version (
  id INT PRIMARY KEY DEFAULT 1 COMMENT '固定ID=1',
  version INT NOT NULL DEFAULT 1 COMMENT '数据版本号',
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='服务器数据版本表';

-- 客户端包版本表
CREATE TABLE IF NOT EXISTS client_package (
  id INT PRIMARY KEY DEFAULT 1 COMMENT '固定ID=1',
  package_version INT NOT NULL DEFAULT 0 COMMENT '客户端包版本号',
  db_version INT NOT NULL DEFAULT 0 COMMENT '包含的数据版本号',
  generated_at TIMESTAMP NULL COMMENT '生成时间',
  db_size BIGINT DEFAULT 0 COMMENT '数据库文件大小(字节)',
  resources_size BIGINT DEFAULT 0 COMMENT '资源包大小(字节)',
  status ENUM('none', 'generating', 'ready', 'error') DEFAULT 'none' COMMENT '生成状态'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户端包版本表';

-- 初始化版本表
INSERT INTO version (id, version) VALUES (1, 1) ON DUPLICATE KEY UPDATE id=id;

-- 初始化客户端包表
INSERT INTO client_package (id) VALUES (1) ON DUPLICATE KEY UPDATE id=id;


