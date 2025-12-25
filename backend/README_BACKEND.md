# AppDeck Backend

AppDeck 后端服务，使用 Go + Gin + MySQL + GORM 实现。

## 功能特性

- MySQL 数据库存储应用、分类和关联关系
- 支持应用的多对多分类关联
- SQLite 数据库生成器（导出客户端数据）
- 图片资源打包服务（ZIP）
- 双版本管理（服务器数据版本 + 客户端包版本）
- RESTful API（客户端 + 管理）
- 点击统计收集

## 环境要求

- Go 1.21+
- MySQL 5.7+ / MariaDB 10.2+

## 快速开始

### 1. 安装依赖

```bash
cd backend
go mod download
```

### 2. 配置数据库

复制环境变量配置文件：

```bash
cp env.example .env
```

编辑 `.env` 文件，配置数据库连接信息。

### 3. 初始化数据库

执行数据库初始化脚本：

```bash
mysql -u root -p < database/schema.sql
```

或者设置环境变量 `AUTO_MIGRATE=true` 让程序自动迁移表结构。

### 4. 启动服务

```bash
go run main.go
```

服务默认启动在 `http://localhost:8080`

## 项目结构

```
backend/
├── main.go                 # 主入口
├── go.mod                  # 依赖管理
├── config/                 # 配置
├── models/                 # 数据模型
├── database/               # 数据库
│   ├── schema.sql          # 初始化脚本
│   ├── mysql.go            # MySQL 连接
│   └── sqlite_generator.go # SQLite 生成器
├── services/               # 业务逻辑
├── handlers/               # HTTP 处理器
│   ├── client/             # 客户端 API
│   └── admin/              # 管理 API
├── middleware/             # 中间件
├── data/                   # 数据存储
│   ├── icons/              # 图标文件
│   ├── bigpics/            # 大图文件
│   ├── client.db           # 生成的客户端数据库
│   └── resources.zip       # 资源包
└── static/                 # 管理前端静态文件
```

## API 端点

### 客户端 API

- `GET /api/v1/client/package-info` - 获取客户端包信息
- `GET /api/v1/client/database` - 下载 SQLite 数据库
- `GET /api/v1/client/resources` - 下载资源 ZIP 包
- `GET /api/v1/version` - 检查服务器数据版本
- `GET /api/v1/apps/full` - 获取完整应用数据
- `GET /api/v1/icons/:filename` - 获取图标文件
- `GET /api/v1/bigpics/:filename` - 获取大图文件
- `POST /api/v1/stats/clicks` - 上报点击统计

### 管理 API

**应用管理**
- `GET /api/v1/admin/apps` - 获取应用列表
- `POST /api/v1/admin/apps` - 创建应用
- `PUT /api/v1/admin/apps/:id` - 更新应用
- `DELETE /api/v1/admin/apps/:id` - 删除应用
- `PUT /api/v1/admin/apps/reorder` - 调整应用顺序

**分类管理**
- `GET /api/v1/admin/categories` - 获取分类列表
- `POST /api/v1/admin/categories` - 创建分类
- `PUT /api/v1/admin/categories/:id` - 更新分类
- `DELETE /api/v1/admin/categories/:id` - 删除分类
- `PUT /api/v1/admin/categories/reorder` - 调整分类顺序

**客户端包管理**
- `GET /api/v1/admin/package-status` - 获取客户端包状态
- `POST /api/v1/admin/generate-client-package` - 生成客户端包

## 开发指南

### 添加新的 API

1. 在 `models/` 中定义数据模型
2. 在 `services/` 中实现业务逻辑
3. 在 `handlers/` 中创建 HTTP 处理器
4. 在 `main.go` 中注册路由

### 生成客户端包

管理员通过管理界面或 API 手动触发：

```bash
curl -X POST http://localhost:8080/api/v1/admin/generate-client-package
```

生成过程：
1. 从 MySQL 导出数据到 SQLite（data/client.db）
2. 打包图片资源到 ZIP（data/resources.zip）
3. 更新客户端包版本信息

## 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| SERVER_HOST | 服务器监听地址 | 0.0.0.0 |
| SERVER_PORT | 服务器端口 | 8080 |
| DB_HOST | MySQL 主机 | localhost |
| DB_PORT | MySQL 端口 | 3306 |
| DB_USER | 数据库用户 | appdeck |
| DB_PASSWORD | 数据库密码 | appdeck |
| DB_NAME | 数据库名称 | appdeck |
| AUTO_MIGRATE | 自动迁移表结构 | false |

## 许可证

MIT


