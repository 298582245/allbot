# AllBot - 去中心化多平台机器人框架

极简、开放、商业友好的机器人框架

## 特性

- **极简插件开发**：单正则 + 单函数，零学习成本
- **多语言支持**：Python 和 Node.js 插件
- **连续对话**：内置 `listen()` 支持多轮对话
- **多平台适配**：统一 API 适配 QQ/微信/Telegram
- **全局依赖管理**：所有插件共享依赖，节省空间和时间 ✨
- **Web UI 管理**：可视化管理插件和系统 ✨
- **一键安装**：自动安装所有依赖 ✨
- **去中心化市场**：开发者自建市场，无平台抽成（Phase 3）
- **源码保护**：AES-256 加密 + RSA 签名 ✅

## 快速开始

### 1. 一键安装

#### Windows
```bash
# 以管理员身份运行
install.bat
```

#### Linux/Mac
```bash
chmod +x install.sh
./install.sh
```

安装脚本会自动：
- 检查并安装 Python 3.11
- 检查并安装 Node.js 20
- 创建 Python 虚拟环境
- 安装基础依赖
- 创建配置文件

### 2. 启动框架

```bash
# 启动 AllBot
go run main.go

# 或编译后运行
go build -o allbot
./allbot
```

启动后访问：
- **Web UI**：http://localhost:3000
- **默认账号**：admin / admin123

### 3. 创建插件

#### Python 插件示例

```
my-plugin/
  ├─ plugin.json
  └─ main.py
```

**plugin.json**
```json
{
  "name": "我的插件",
  "version": "1.0.0",
  "runtime": "python",
  "entry": "main.py",
  "platforms": ["qq", "wechat", "telegram"],
  "trigger": "你好.*",
  "dependencies": {
    "requests": "2.31.0"
  }
}
```

**main.py**
```python
async def handle(ctx):
    if ctx.content == "你好":
        await ctx.reply("你好！我是机器人")
    elif ctx.content.startswith("你好 "):
        name = ctx.content[3:]
        await ctx.reply(f"你好，{name}！")
```

**依赖自动安装**：
- 插件加载时，框架自动安装 `dependencies` 中声明的包
- 所有插件共享全局依赖，无需重复安装
- Python 依赖安装到 `runtime/.venv`
- Node.js 依赖安装到 `runtime/node_modules`

## 项目结构

```
allbot/
├─ core/                    # Go 核心框架
│   ├─ router/              # 消息路由器 ✅
│   ├─ plugin/              # 插件管理器 ✅
│   ├─ adapter/             # 平台适配器 ✅
│   ├─ session/             # 会话管理器 ✅
│   ├─ deps/                # 依赖管理器 ✅
│   ├─ web/                 # Web UI 服务 ✅
│   ├─ grpc/                # HTTP 通信客户端 ✅
│   ├─ crypto/              # 加密和授权 ✅
│   ├─ vfs/                 # 虚拟文件系统 ✅
│   └─ types/               # 数据类型 ✅
├─ sdk/
│   ├─ python/              # Python SDK ✅
│   └─ nodejs/              # Node.js SDK ✅
├─ proto/                   # gRPC 协议定义 ✅
├─ web/                     # Web UI 前端 ✅
│   └─ index.html           # 管理界面
├─ examples/weather/        # 示例插件 ✅
├─ runtime/                 # 运行时环境
│   ├─ .venv/               # Python 虚拟环境
│   ├─ node_modules/        # Node.js 全局依赖
│   ├─ python_deps.json     # Python 依赖清单
│   └─ package.json         # Node.js 依赖清单
├─ plugins/                 # 插件目录
├─ install.bat              # Windows 安装脚本 ✅
├─ install.sh               # Linux/Mac 安装脚本 ✅
├─ main.go                  # 主程序 ✅
├─ config.yml               # 配置文件
├─ project.md               # 设计文档 ✅
└─ README.md                # 使用文档 ✅
```

## Phase 1 + Phase 2 完成状态

✅ **已完成**：
- Go 核心框架（消息路由、会话管理、插件管理）
- HTTP 通信协议（核心框架 ↔ 插件）
- Python SDK（Context API + HTTP 服务器）
- Node.js SDK（Context API）
- QQ 平台适配器（基于 go-cqhttp）
- 示例插件（天气插件）
- **全局依赖管理系统**（Python + Node.js）
- **自动化安装脚本**（Windows + Linux/Mac）
- **Web UI 管理界面**（基础版 + API）
- **插件加密系统**（AES-256 + RSA 签名）
- **虚拟文件系统**（内存文件系统）
- **授权验证系统**（设备绑定 + License 管理）

⏳ **待实现**（Phase 3）：
- 完整的 Vue 3 + Element Plus 前端（开发者后台）
- 支付集成完善（支付宝/微信/Stripe SDK）

✅ **Phase 3 已完成**：
- **市场服务器模板**（FastAPI + PostgreSQL）
- **CLI 工具**（插件创建、发布、安装）
- **Docker 一键部署**
- **完整 API 文档**

## 插件开发

### Context API

```python
# 消息信息
ctx.platform        # 'qq' | 'wechat' | 'telegram'
ctx.user_id         # 发送者 ID
ctx.group_id        # 群组 ID（私聊为空）
ctx.content         # 消息内容

# 发送消息
await ctx.reply("文本")
await ctx.send_image("https://example.com/image.png")

# 连续对话
city = await ctx.listen(60)  # 等待 60 秒

# 数据存储
await ctx.storage.set("key", "value")
value = await ctx.storage.get("key")

# HTTP 请求
response = await ctx.http.get("https://api.example.com")
```

### 多轮对话示例

```python
async def handle(ctx):
    if ctx.content == "注册":
        await ctx.reply("请输入用户名：")
        username = await ctx.listen(60)

        if not username:
            await ctx.reply("超时")
            return

        await ctx.reply("请输入密码：")
        password = await ctx.listen(60)

        if not password:
            await ctx.reply("超时")
            return

        await ctx.reply("注册成功！")
```

### 依赖管理

插件在 `plugin.json` 中声明依赖，框架自动安装：

```json
{
  "dependencies": {
    "requests": "2.31.0",
    "beautifulsoup4": "4.12.0"
  }
}
```

**优势**：
- 所有插件共享依赖，节省磁盘空间
- 自动安装，无需手动操作
- 版本统一管理，避免冲突

## Web UI 管理界面

访问 http://localhost:3000 进入管理界面：

- **登录**：使用管理员账号登录
- **插件管理**：查看、启用、禁用插件
- **系统状态**：查看运行状态和统计信息
- **日志查看**：实时查看系统日志（开发中）

**API 端点**：
- `POST /api/login` - 登录
- `GET /api/plugins` - 插件列表
- `GET /api/system/status` - 系统状态

## 配置

### 命令行参数

```bash
--plugins=./plugins          # 插件目录
--qq-api=http://localhost:5700  # go-cqhttp API 地址
```

### 配置文件（config.yml）

```yaml
# 管理员账号
admin:
  username: admin
  password: admin123  # 首次启动后请修改

# Web UI 配置
web:
  port: 3000
  host: 0.0.0.0

# QQ 平台配置
qq:
  api_url: http://localhost:5700
  enabled: false

# 插件目录
plugins:
  dir: ./plugins
```

## 架构设计

详见 [project.md](project.md)

## 开发路线图

- [x] Phase 1（3个月）- 核心框架
- [x] Phase 2（2个月）- 依赖管理 + 自动化安装 + Web UI + 加密系统
- [ ] Phase 3（2个月）- 市场系统
- [ ] Phase 4（持续）- 生态建设

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License
