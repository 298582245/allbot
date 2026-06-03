# AllBot Web UI

基于 Vue 3 + Element Plus 的现代化管理后台。

## 功能特性

- ✅ **登录认证** - JWT Token 认证
- ✅ **仪表盘** - 系统状态、统计图表、快速操作
- ✅ **插件管理** - 查看、启动、停止、删除插件
- ✅ **平台配置** - 动态配置 QQ/Telegram/微信适配器
- ✅ **日志查看** - 实时日志流（模拟）
- ✅ **系统设置** - 管理员配置、Web UI 设置

## 技术栈

- **Vue 3** - 渐进式 JavaScript 框架
- **Element Plus** - Vue 3 组件库
- **Vue Router** - 路由管理
- **Pinia** - 状态管理
- **Axios** - HTTP 客户端
- **Vite** - 构建工具

## 开发

### 安装依赖

```bash
cd web-ui
npm install
```

### 启动开发服务器

```bash
npm run dev
```

访问 http://localhost:5173

开发服务器会自动代理 API 请求到 `http://localhost:3000`

### 构建生产版本

```bash
npm run build
```

构建产物会输出到 `../web/` 目录，Go 后端会自动提供这些静态文件。

## 项目结构

```
web-ui/
├── src/
│   ├── api/              # API 接口封装
│   ├── components/       # 公共组件
│   ├── router/           # 路由配置
│   ├── stores/           # Pinia 状态管理
│   ├── utils/            # 工具函数
│   ├── views/            # 页面组件
│   │   ├── Login.vue     # 登录页
│   │   ├── Layout.vue    # 布局组件
│   │   ├── Dashboard.vue # 仪表盘
│   │   ├── Plugins.vue   # 插件管理
│   │   ├── Adapters.vue  # 平台配置
│   │   ├── Logs.vue      # 日志查看
│   │   └── Settings.vue  # 系统设置
│   ├── App.vue           # 根组件
│   └── main.js           # 入口文件
├── index.html            # HTML 模板
├── vite.config.js        # Vite 配置
└── package.json          # 依赖配置
```

## 页面说明

### 登录页面

- 管理员账号：默认用户名为 `admin`，首次启动会在控制台输出随机密码
- JWT Token 认证
- 自动跳转到仪表盘

### 仪表盘

- 系统状态卡片（运行时间、插件数、运行中、消息数）
- 插件状态列表
- 平台状态列表
- 快速操作按钮
- 自动刷新（每 5 秒）

### 插件管理

- 插件列表展示
- 启动/停止插件
- 删除插件
- 查看插件详情（名称、版本、运行时、端口、状态、支持平台）

### 平台配置

- 适配器列表展示
- 添加/编辑/删除适配器
- 启用/禁用开关（实时生效）
- 支持 QQ、Telegram、微信配置
- 配置表单动态切换

### 日志查看

- 实时日志流（模拟）
- 日志级别高亮（INFO/WARN/ERROR/DEBUG）
- 刷新和清空功能
- 自动滚动

### 系统设置

- 管理员账号管理
- 修改密码
- Web UI 配置（端口、自动刷新）
- 插件配置（目录、自动加载）
- 系统信息展示

## API 接口

所有 API 请求都会自动添加 `Authorization: Bearer <token>` 头。

### 认证

- `POST /api/login` - 登录

### 系统

- `GET /api/system/status` - 获取系统状态

### 插件

- `GET /api/plugins` - 获取插件列表

### 适配器

- `GET /api/adapters` - 获取适配器列表
- `POST /api/adapters` - 创建/更新适配器
- `GET /api/adapters/:platform` - 获取适配器详情
- `DELETE /api/adapters/:platform` - 删除适配器

## 开发说明

### 添加新页面

1. 在 `src/views/` 创建新组件
2. 在 `src/router/index.js` 添加路由
3. 在 `src/views/Layout.vue` 添加菜单项

### 添加新 API

1. 在 `src/api/index.js` 添加 API 函数
2. 使用 `request` 工具发送请求

### 状态管理

使用 Pinia 管理全局状态：

```javascript
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
console.log(authStore.token)
authStore.logout()
```

## 部署

### 开发环境

```bash
# 启动 Go 后端
go run main.go --plugins=./plugins

# 启动 Vue 前端（另一个终端）
cd web-ui
npm run dev
```

访问 http://localhost:5173

### 生产环境

```bash
# 构建前端
cd web-ui
npm run build

# 启动 Go 后端（会自动提供静态文件）
cd ..
go run main.go --plugins=./plugins
```

访问 http://localhost:3000

## 注意事项

1. **开发模式**：前端和后端分别运行，前端通过代理访问后端 API
2. **生产模式**：前端构建后，Go 后端直接提供静态文件
3. **Token 存储**：JWT Token 存储在 localStorage
4. **路由守卫**：未登录用户会自动跳转到登录页
5. **自动刷新**：仪表盘每 5 秒自动刷新数据

## 许可证

MIT License
