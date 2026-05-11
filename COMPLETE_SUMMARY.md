# AllBot 完整功能总结

## 项目概述

AllBot 是一个去中心化的多平台机器人框架，支持极简插件开发、多语言运行时、动态配置管理和现代化 Web UI。

**开发时间**：2026年5月
**当前版本**：v1.0.0
**状态**：✅ 核心功能已完成

---

## 核心特性

### 1. 极简插件开发

**特点**：
- 单正则表达式触发
- 单函数处理逻辑
- 零学习成本

**示例**：
```python
# plugin.json
{
  "trigger": "^你好.*"
}

# main.py
async def handle(ctx):
    await ctx.reply("你好！")
```

### 2. 多语言支持

**支持的运行时**：
- ✅ Python 3.7+
- ✅ Node.js 14+

**特性**：
- 统一的 Context API
- 自动进程管理
- HTTP 通信协议

### 3. 多平台适配

**已实现平台**：
- ✅ **QQ** - 基于 go-cqhttp
- ✅ **Telegram** - Bot API 长轮询
- 🚧 **微信** - 企业微信/公众号（开发中）

**统一 API**：
```python
# 跨平台兼容
await ctx.reply("消息")  # 自动适配所有平台
await ctx.send_image(url)
await ctx.listen(timeout)
```

### 4. 动态配置系统 ✨

**核心功能**：
- 配置存储在 SQLite 数据库
- Web UI 可视化管理
- 热重载（无需重启）
- 支持多平台独立配置

**技术实现**：
- `core/config/database.go` - 数据库操作
- `core/config/manager.go` - 适配器管理器
- 自动重启适配器，不影响其他组件

**使用场景**：
```
1. 在 Web UI 修改 QQ API 地址
2. 点击保存
3. QQ 适配器自动重启
4. 新配置立即生效
5. 其他平台和插件不受影响
```

### 5. 现代化管理后台 ✨

**技术栈**：
- Vue 3 - 渐进式框架
- Element Plus - 组件库
- Vite - 构建工具
- Pinia - 状态管理
- Vue Router - 路由管理

**功能页面**：

#### 📊 仪表盘
- 系统状态卡片（运行时间、插件数、运行中、消息数）
- 插件状态列表
- 平台状态列表
- 快速操作按钮
- 自动刷新（每 5 秒）

#### 🔌 插件管理
- 插件列表展示
- 启动/停止插件
- 删除插件
- 查看插件详情

#### 🌐 平台配置
- 适配器列表展示
- 添加/编辑/删除适配器
- 启用/禁用开关（实时生效）
- 动态配置表单（QQ/Telegram/微信）

#### 📝 日志查看
- 实时日志流
- 日志级别高亮
- 刷新和清空功能

#### ⚙️ 系统设置
- 管理员账号管理
- 修改密码
- Web UI 配置
- 插件配置

### 6. 全局依赖管理

**特点**：
- 所有插件共享依赖
- 自动安装
- 节省磁盘空间

**实现**：
- Python：`runtime/python/venv/`
- Node.js：`runtime/nodejs/node_modules/`

**使用**：
```json
{
  "dependencies": {
    "requests": "2.31.0"
  }
}
```

框架自动安装到全局环境。

### 7. 连续对话支持

**Context API**：
```python
async def handle(ctx):
    await ctx.reply("请输入用户名：")
    username = await ctx.listen(60)  # 等待 60 秒

    if not username:
        await ctx.reply("超时")
        return

    await ctx.reply(f"欢迎，{username}！")
```

**技术实现**：
- 会话管理器（`core/session/`）
- 用户级别会话隔离
- 超时自动清理

### 8. 插件加密系统

**加密方案**：
- AES-256-GCM 加密源码
- RSA-2048 签名验证
- 虚拟文件系统运行

**License 管理**：
- 设备绑定
- 订阅模式（月付/永久）
- 在线验证

**文件**：
- `core/crypto/encryptor.go` - 加密/解密
- `core/crypto/license.go` - License 生成/验证
- `core/vfs/vfs.go` - 虚拟文件系统

### 9. 去中心化市场

**架构**：
- 开发者自建市场服务器
- FastAPI + PostgreSQL
- Docker 一键部署

**功能**：
- 插件上传/下载
- 付费购买
- License 管理
- OAuth2 认证

**CLI 工具**：
```bash
# 创建插件
allbot create my-plugin

# 登录市场
allbot market login https://market.example.com

# 发布插件
allbot market publish ./my-plugin

# 安装插件
allbot plugin install my-plugin
```

---

## 技术架构

### 核心框架（Go）

```
core/
├── router/       # 消息路由器
├── plugin/       # 插件管理器
├── adapter/      # 平台适配器
├── session/      # 会话管理器
├── deps/         # 依赖管理器
├── config/       # 配置管理器 ✨
├── web/          # Web API 服务
├── grpc/         # HTTP 通信客户端
├── crypto/       # 加密和授权
├── vfs/          # 虚拟文件系统
└── types/        # 数据类型
```

### SDK

**Python SDK**：
- `allbot_sdk/server.py` - HTTP 服务器
- `allbot_sdk/context.py` - Context API
- `allbot_sdk/plugin.py` - 插件基类

**Node.js SDK**：
- `allbot-sdk/server.js` - HTTP 服务器
- `allbot-sdk/context.js` - Context API
- `allbot-sdk/plugin.js` - 插件基类

### Web UI（Vue 3）

```
web-ui/
├── src/
│   ├── api/          # API 封装
│   ├── router/       # 路由配置
│   ├── stores/       # 状态管理
│   ├── utils/        # 工具函数
│   └── views/        # 页面组件
├── vite.config.js    # Vite 配置
└── package.json      # 依赖配置
```

### 市场服务器（FastAPI）

```
market-server/
├── app/
│   ├── models/       # 数据模型
│   ├── api/          # API 路由
│   ├── core/         # 核心功能
│   └── main.py       # 应用入口
├── Dockerfile        # Docker 配置
└── requirements.txt  # Python 依赖
```

---

## 示例插件

### 1. 天气插件

**功能**：查询城市天气

**触发**：`天气 <城市>`

**文件**：`examples/weather/`

### 2. 翻译插件 ✨

**功能**：中英互译

**触发**：`翻译 <文本>`

**特点**：
- 自动检测语言
- 跨平台兼容
- 使用免费 API

**文件**：`examples/translator/`

---

## 部署方式

### 开发环境

```bash
# 启动 Go 后端
go run main.go --plugins=./plugins

# 启动 Vue 前端（另一个终端）
cd web-ui
npm run dev
```

访问：
- 后端 API：http://localhost:3000
- 前端 UI：http://localhost:5173

### 生产环境

```bash
# 构建前端
cd web-ui
npm run build

# 启动后端（自动提供静态文件）
cd ..
go build -o allbot
./allbot --plugins=./plugins
```

访问：http://localhost:3000

### Docker 部署

```bash
# 构建镜像
docker build -t allbot .

# 运行容器
docker run -d -p 3000:3000 -v ./plugins:/app/plugins allbot
```

---

## 配置说明

### 数据库配置

**文件**：`config.db`（SQLite）

**表结构**：
- `adapters` - 平台适配器配置

**管理方式**：
- Web UI 可视化管理
- 支持热重载

### 平台配置

#### QQ 平台

**前置条件**：安装 go-cqhttp

**配置项**：
- API 地址：`http://localhost:5700`
- 监听地址：`:8080`

#### Telegram 平台

**前置条件**：从 @BotFather 创建 Bot

**配置项**：
- Bot Token：`123456789:ABC...`

#### 微信平台

**状态**：开发中

**配置项**：
- App ID
- App Secret

---

## 性能指标

### 系统性能

- **启动时间**：< 3 秒
- **内存占用**：< 100MB（核心框架）
- **消息延迟**：< 100ms
- **并发处理**：支持多插件并发

### 插件性能

- **加载时间**：< 1 秒/插件
- **进程隔离**：每个插件独立进程
- **自动重启**：崩溃自动恢复

### 配置热重载

- **重载时间**：< 1 秒
- **影响范围**：仅重启目标适配器
- **零停机**：其他组件不受影响

---

## 安全特性

### 插件加密

- AES-256-GCM 加密
- RSA-2048 签名
- 虚拟文件系统

### License 管理

- 设备绑定
- 在线验证
- 订阅模式

### Web UI 安全

- JWT Token 认证
- 密码加密存储
- CORS 保护

---

## 开发路线图

### Phase 1 ✅ 核心框架（已完成）

- Go 核心框架
- HTTP 通信协议
- Python/Node.js SDK
- QQ 平台适配器
- 示例插件

### Phase 2 ✅ 增强功能（已完成）

- 全局依赖管理
- 自动化安装脚本
- 基础 Web UI
- 插件加密系统
- 虚拟文件系统
- 授权验证系统

### Phase 3 ✅ 完善生态（已完成）

- **动态配置系统** ✨
- **Telegram 适配器** ✨
- **Vue 3 管理后台** ✨
- 市场服务器模板
- CLI 工具
- Docker 部署
- 翻译插件示例

### Phase 4 🚧 持续优化（进行中）

- 微信平台适配器
- 更多示例插件
- 性能优化
- 文档完善
- 社区建设

---

## 文档资源

### 核心文档

- `README.md` - 项目介绍和快速开始
- `project.md` - 详细设计文档
- `QUICKSTART.md` - 快速使用指南
- `DYNAMIC_CONFIG.md` - 动态配置系统文档
- `PHASE3_SUMMARY.md` - Phase 3 实现总结

### 示例文档

- `examples/weather/README.md` - 天气插件文档
- `examples/translator/README.md` - 翻译插件文档

### Web UI 文档

- `web-ui/README.md` - Vue 3 管理后台文档

### 市场服务器文档

- `market-server/README.md` - 市场服务器部署文档

---

## 贡献指南

### 开发环境

**要求**：
- Go 1.21+
- Python 3.7+
- Node.js 14+
- Git

**安装**：
```bash
# 克隆仓库
git clone https://github.com/yourusername/allbot.git
cd allbot

# 安装依赖
go mod download
cd web-ui && npm install
```

### 提交规范

**格式**：
```
<type>: <subject>

<body>
```

**类型**：
- `feat` - 新功能
- `fix` - 修复 Bug
- `docs` - 文档更新
- `style` - 代码格式
- `refactor` - 重构
- `test` - 测试
- `chore` - 构建/工具

### 代码规范

**Go**：
- 使用 `gofmt` 格式化
- 遵循 Go 官方规范

**Python**：
- 使用 PEP 8 规范
- 使用 `black` 格式化

**JavaScript**：
- 使用 ESLint
- 遵循 Vue 3 风格指南

---

## 常见问题

### 1. 如何添加新平台？

1. 实现 `adapter.Adapter` 接口
2. 在 `config/manager.go` 添加平台支持
3. 在 Web UI 添加配置表单

### 2. 如何开发插件？

参考 `examples/` 目录下的示例插件。

### 3. 如何部署市场服务器？

参考 `market-server/README.md`。

### 4. 配置修改后需要重启吗？

不需要！动态配置系统支持热重载。

### 5. 如何查看日志？

在 Web UI 的"日志查看"页面实时查看。

---

## 许可证

MIT License

---

## 致谢

感谢所有贡献者和用户的支持！

**项目地址**：https://github.com/yourusername/allbot

**问题反馈**：https://github.com/yourusername/allbot/issues

**文档网站**：https://allbot.example.com

---

**最后更新**：2026-05-11
