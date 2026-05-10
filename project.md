# AllBot - 去中心化多平台机器人框架

## 项目概述

AllBot 是一个开源的、去中心化的多平台机器人框架，支持插件商业化。框架核心使用 Go 语言开发，插件支持 Python 和 Node.js 编写。

### 核心特性

- **极简插件开发**：单正则触发 + 单函数处理，零学习成本
- **多语言支持**：Python 和 Node.js 插件，通过 gRPC 通信
- **去中心化市场**：开发者自建市场，无平台抽成
- **源码保护**：AES-256 加密 + 虚拟文件系统，保护付费插件
- **多平台适配**：统一 API 适配 QQ/微信/Telegram/Discord
- **连续对话**：内置 `listen()` 支持多轮对话

---

## 技术架构

### 1. 技术栈

| 组件 | 技术选型 | 理由 |
|-----|---------|------|
| 核心框架 | Go | 高性能、跨平台编译、易分发 |
| Web 管理界面 | Vue 3 + Element Plus | 现代化、易用、组件丰富 |
| 插件语言 | Python + Node.js | 生态丰富、易学易用 |
| 进程通信 | gRPC | 高效、跨语言、类型安全 |
| 插件加密 | AES-256 + RSA签名 | 行业标准、安全可靠 |
| 市场服务器 | FastAPI/NestJS | 开发者自选 |

### 2. 系统架构图

```
┌─────────────────────────────────────────────────┐
│  平台消息（QQ/微信/Telegram）                     │
└─────────────────┬───────────────────────────────┘
                  ↓
┌─────────────────────────────────────────────────┐
│  AllBot 核心框架（Go，开源）                      │
│  ┌───────────────────────────────────────────┐  │
│  │ 消息路由器                                 │  │
│  │ - 正则匹配插件                             │  │
│  │ - 会话管理（listen）                       │  │
│  └───────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────┐  │
│  │ 插件管理器                                 │  │
│  │ - 加载/卸载/热重载                         │  │
│  │ - 授权验证                                 │  │
│  │ - 虚拟文件系统                             │  │
│  └───────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────┐  │
│  │ 平台适配器                                 │  │
│  │ - QQ/微信/Telegram/Discord                │  │
│  └───────────────────────────────────────────┘  │
└─────────────────┬───────────────────────────────┘
                  ↓ gRPC
┌─────────────────────────────────────────────────┐
│  插件进程（Python/Node.js）                      │
│  ┌───────────────────────────────────────────┐  │
│  │ 加密的插件代码（运行时解密到内存）          │  │
│  └───────────────────────────────────────────┘  │
└─────────────────┬───────────────────────────────┘
                  ↓ HTTPS
┌─────────────────────────────────────────────────┐
│  开发者市场服务器（开发者自己部署）              │
│  - 插件列表/购买/下载                            │
│  - 授权验证服务                                  │
│  - 支付集成（支付宝/微信/Stripe）                │
└─────────────────────────────────────────────────┘
```

---

## 核心功能设计

### 1. 消息流转流程

```
收到消息
  ↓
优先检查是否有等待会话（listen）
  ↓ 是
  消息拦截，返回给等待的插件
  ↓ 否
遍历所有插件，正则匹配 trigger
  ↓
调用匹配的插件 handle(ctx)
  ↓
插件调用中间件 API（ctx.reply/send_image等）
  ↓
中间件调用平台适配器
  ↓
返回结果给用户
```

### 2. 插件系统

#### 插件结构

```
weather-plugin/
  ├─ plugin.json          # 配置文件（明文）
  ├─ main.py             # 主逻辑（加密）
  ├─ utils/              # 工具模块（加密）
  │   ├─ api.py
  │   └─ parser.py
  └─ assets/             # 资源文件（明文）
      └─ icon.png
```

#### plugin.json（极简配置）

```json
{
  "name": "天气插件",
  "version": "1.0.0",
  "runtime": "python",
  "entry": "main.py",
  "platforms": ["qq", "wechat", "telegram"],
  "trigger": "天气.*"
}
```

#### main.py（纯函数，无继承无装饰器）

```python
async def handle(ctx):
    """框架只调用这一个函数，插件自己解析命令"""
    content = ctx.content

    if content.startswith("天气预报"):
        parts = content.split()
        city = parts[1] if len(parts) > 1 else "北京"
        days = int(parts[2]) if len(parts) > 2 else 3

        forecast = await fetch_forecast(city, days)
        await ctx.reply(f"{city}未来{days}天：{forecast}")

    elif content.startswith("天气"):
        city = content[2:].strip() or "北京"
        weather = await fetch_weather(city)
        await ctx.reply(f"{city}的天气：{weather}")

        if ctx.platform == 'qq':
            await ctx.send_image(f"https://api.weather.com/{city}.png")
```

### 3. 中间件 API（统一接口）

```python
# 消息上下文
ctx.platform        # 'qq' | 'wechat' | 'telegram'
ctx.content         # 完整消息内容
ctx.user_id         # 发送者ID
ctx.group_id        # 群组ID（私聊为None）
ctx.message_id      # 消息ID

# 发送消息
await ctx.reply(text)
await ctx.send_image(url)
await ctx.send_file(path)

# 连续对话
city = await ctx.listen(60)  # 等待60秒，超时返回空字符串

# 获取信息
await ctx.get_user_info()
await ctx.get_group_info()

# 平台特定功能
await ctx.at_user(user_id)  # QQ/微信
await ctx.send_keyboard([...])  # Telegram

# 数据存储（自动隔离）
await ctx.storage.get(key)
await ctx.storage.set(key, value)

# HTTP请求
await ctx.http.get(url)
await ctx.http.post(url, data)
```

### 4. 连续对话（listen）

#### 功能特性

- **用户隔离**：只拦截同一用户（user_id + group_id）的消息
- **插件独占**：等待期间，该用户的消息不会触发其他插件
- **自动超时**：超时自动清理会话，返回空字符串
- **覆盖机制**：同一用户触发新插件会覆盖旧的等待状态

#### 使用示例

```python
# 简单对话
async def handle(ctx):
    if ctx.content == "天气":
        await ctx.reply("请输入城市名：")
        city = await ctx.listen(60)

        if not city:
            await ctx.reply("超时")
            return

        weather = await fetch_weather(city)
        await ctx.reply(f"{city}：{weather}")

# 多轮对话
async def handle(ctx):
    if ctx.content == "注册":
        await ctx.reply("请输入用户名：")
        username = await ctx.listen(60)
        if not username:
            return

        await ctx.reply("请输入密码：")
        password = await ctx.listen(60)
        if not password:
            return

        await ctx.reply(f"确认注册？\n用户名：{username}\n回复'是'确认")
        confirm = await ctx.listen(30)

        if confirm == "是":
            await register_user(username, password)
            await ctx.reply("注册成功！")
```

---

## 插件加密与保护

### 1. 加密方案

#### 打包格式（.allbot 文件）

```
weather-plugin.allbot (加密的 tar.gz)
  ├─ manifest.json          # 元信息（明文，用于展示）
  ├─ encrypted.dat          # 加密的代码包
  └─ signature.sig          # RSA 数字签名
```

#### 加密流程

```
开发者上传插件目录
  ↓
市场服务器扫描所有 .py/.js 文件
  ↓
打包成 tar.gz（包含所有子目录）
  ↓
AES-256 加密整个包
  ↓
RSA 签名防篡改
  ↓
生成 .allbot 文件
  ↓
用户购买后获得解密密钥
```

#### 运行时解密（虚拟文件系统）

```
用户启动插件
  ↓
验证授权（设备绑定 + 在线验证）
  ↓
解密插件包到内存（不写磁盘）
  ↓
创建虚拟文件系统（memfs）
  ↓
启动 Python/Node.js 进程，挂载虚拟文件系统
  ↓
插件正常 import，从虚拟文件系统加载
```

### 2. 授权验证

#### 授权证书格式

```json
{
  "plugin_id": "weather-pro",
  "user_id": "user123",
  "device_id": "abc-def-ghi",
  "license_key": "XXXX-XXXX-XXXX",
  "type": "subscription",
  "expires_at": "2027-05-10",
  "signature": "RSA签名"
}
```

#### 验证机制

- **设备绑定**：密钥绑定机器码（CPU ID + MAC 地址）
- **在线验证**：每24小时验证一次授权
- **离线容忍**：允许7天离线使用
- **签名验证**：防止授权证书篡改

### 3. 安全性评估

| 攻击方式 | 防护措施 | 保护强度 |
|---------|---------|---------|
| 直接读取 .allbot 文件 | AES-256 加密 | ⭐⭐⭐⭐ |
| 内存 dump | 进程隔离 + 反调试 | ⭐⭐⭐ |
| 修改授权验证代码 | 核心框架闭源 + 签名验证 | ⭐⭐⭐⭐ |
| 破解设备绑定 | 硬件指纹 + 在线验证 | ⭐⭐⭐⭐ |
| 分享解密后的代码 | 水印 + 定期验证 | ⭐⭐⭐ |

**现实评估**：
- 低价插件（<50元）：基本够用，破解成本 > 购买成本
- 中价插件（50-200元）：较好保护
- 高价插件（>200元）：建议云端执行核心逻辑

---

## 去中心化市场

### 1. 市场架构

```
用户机器人实例
  ├─ AllBot 核心（开源）
  ├─ 订阅的市场列表
  │   ├─ https://dev-a.com/market (开发者A的市场)
  │   ├─ https://dev-b.com/market (开发者B的市场)
  │   └─ https://community.com/market (社区市场)
  └─ 已安装插件
      ├─ plugin-weather (加密)
      └─ plugin-translate (开源)
```

### 2. 市场标准 API

```
GET  /api/plugins              # 插件列表
GET  /api/plugins/:id          # 插件详情
GET  /api/plugins/:id/download # 下载插件
POST /api/plugins/:id/purchase # 购买插件
POST /api/plugins/verify       # 验证授权
GET  /api/plugins/check-updates # 检查更新
```

### 3. 使用流程

```bash
# 1. 订阅市场
allbot market add https://market.example.com --token xxx

# 2. 搜索插件
allbot plugin search 天气

# 3. 安装插件（自动处理付费）
allbot plugin install weather-pro

# 4. 查看已安装
allbot plugin list

# 5. 更新插件
allbot plugin update weather-pro
```

### 4. 商业模式

| 角色 | 收入来源 | 成本 |
|-----|---------|------|
| 框架开发者（你） | 捐赠/赞助/企业版 | 开发维护 |
| 插件开发者 | 插件销售 100% | 服务器（市场+授权验证） |
| 用户 | - | 购买插件 |

---

## 开发工具链

### 1. CLI 工具

```bash
# 安装 AllBot CLI
npm install -g @allbot/cli

# 创建插件项目
allbot create my-plugin --lang python

# 本地测试（明文运行）
allbot dev

# 打包插件（自动加密）
allbot build

# 发布到自己的市场
allbot publish --market https://my-market.com --token xxx
```

### 2. 市场服务器模板

提供开源的市场服务器模板，开发者可以一键部署：

```bash
# 使用 Docker 部署
git clone https://github.com/allbot/market-server
cd market-server
docker-compose up -d

# 配置支付
vim config.yml
# payment:
#   alipay:
#     app_id: xxx
#   wechat:
#     mch_id: xxx
```

---

## 项目目录结构

```
allbot/
├─ core/                    # Go 核心框架
│   ├─ router/              # 消息路由
│   ├─ plugin/              # 插件管理
│   ├─ adapter/             # 平台适配器
│   ├─ session/             # 会话管理（listen）
│   ├─ crypto/              # 加密解密
│   └─ vfs/                 # 虚拟文件系统
├─ sdk/                     # 插件 SDK
│   ├─ python/              # Python SDK
│   └─ nodejs/              # Node.js SDK
├─ market-server/           # 市场服务器模板
│   ├─ api/                 # API 服务
│   ├─ payment/             # 支付集成
│   └─ admin/               # 管理后台
├─ cli/                     # 命令行工具
├─ examples/                # 示例插件
│   ├─ weather/             # 天气插件
│   ├─ translate/           # 翻译插件
│   └─ chatgpt/             # ChatGPT 插件
└─ docs/                    # 文档
    ├─ plugin-dev.md        # 插件开发指南
    ├─ market-setup.md      # 市场搭建指南
    └─ api-reference.md     # API 参考
```

---

## 实现优先级

### Phase 1（3个月）- 核心框架 ✅

- [x] Go 核心框架架构设计
- [x] 消息路由器（正则匹配）
- [x] 会话管理器（listen）
- [x] gRPC 插件通信协议
- [x] Python SDK
- [x] Node.js SDK
- [x] QQ 适配器（go-cqhttp）
- [x] 基础授权验证

### Phase 2（2个月）- 加密系统 + Web UI + 自动化

**核心功能**：
- [ ] gRPC 服务端/客户端实现
- [ ] 插件加密/解密
- [ ] 虚拟文件系统
- [ ] 设备绑定
- [ ] 在线验证服务
- [ ] 授权证书管理

**用户体验优化**：
- [ ] Web UI 管理界面（Vue 3 + Element Plus）
  - 管理员登录
  - 插件管理（安装/卸载/启用/禁用）
  - 平台配置（QQ/微信/Telegram）
  - 日志查看
  - 系统监控
- [ ] 一键安装脚本
  - Windows: install.bat
  - Linux/Mac: install.sh
  - 自动安装 Go、Python、Node.js 依赖
- [ ] 全局依赖管理系统
  - Python 虚拟环境（所有插件共享）
  - Node.js 全局 node_modules
  - 插件声明依赖，框架自动安装

### Phase 3（2个月）- 市场系统

- [ ] 市场服务器模板（开源）
- [ ] 标准 API 实现
- [ ] 支付集成（支付宝/微信）
- [ ] 开发者后台
- [ ] CLI 工具

### Phase 4（持续）- 生态建设

- [ ] 更多平台适配器（微信/Telegram/Discord）
- [ ] 官方插件示例
- [ ] 完善文档
- [ ] 社区建设
- [ ] 企业版功能

---

## 用户体验优化设计

### 1. Web UI 管理界面

#### 技术栈
- **前端**：Vue 3 + Element Plus + TypeScript
- **后端**：Go（嵌入到核心框架）
- **通信**：RESTful API + WebSocket（实时日志）

#### 功能模块

```
Web UI 管理界面
├─ 登录页面
│   └─ 管理员账号密码验证
├─ 仪表盘
│   ├─ 系统状态（CPU/内存/运行时间）
│   ├─ 消息统计（今日消息数/插件调用次数）
│   └─ 快速操作
├─ 插件管理
│   ├─ 已安装插件列表
│   ├─ 插件详情（名称/版本/状态/触发规则）
│   ├─ 启用/禁用/卸载
│   └─ 安装新插件（本地上传/市场安装）
├─ 平台配置
│   ├─ QQ 配置（go-cqhttp API 地址）
│   ├─ 微信配置
│   └─ Telegram 配置
├─ 日志查看
│   ├─ 实时日志（WebSocket）
│   ├─ 日志过滤（级别/插件/平台）
│   └─ 日志导出
└─ 系统设置
    ├─ 管理员密码修改
    ├─ 全局依赖管理
    └─ 备份/恢复
```

#### 安全设计
- **JWT 认证**：登录后颁发 JWT Token
- **密码加密**：bcrypt 加密存储
- **HTTPS**：生产环境强制 HTTPS
- **CSRF 防护**：Token 验证

#### 部署方式
```
allbot
├─ allbot.exe          # Go 编译的可执行文件（内嵌 Web UI）
├─ config.yml          # 配置文件
└─ plugins/            # 插件目录
```

访问：`http://localhost:3000`（默认端口）

### 2. 自动化安装方案

#### Windows 安装脚本（install.bat）

```batch
@echo off
echo ========================================
echo AllBot 自动安装脚本
echo ========================================

REM 1. 检查并安装 Python
python --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [1/4] 正在下载 Python...
    curl -o python-installer.exe https://www.python.org/ftp/python/3.11.0/python-3.11.0-amd64.exe
    echo [1/4] 正在安装 Python...
    python-installer.exe /quiet InstallAllUsers=1 PrependPath=1
    del python-installer.exe
) else (
    echo [1/4] Python 已安装
)

REM 2. 检查并安装 Node.js
node --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [2/4] 正在下载 Node.js...
    curl -o node-installer.msi https://nodejs.org/dist/v20.0.0/node-v20.0.0-x64.msi
    echo [2/4] 正在安装 Node.js...
    msiexec /i node-installer.msi /quiet
    del node-installer.msi
) else (
    echo [2/4] Node.js 已安装
)

REM 3. 安装 AllBot SDK
echo [3/4] 正在安装 AllBot Python SDK...
pip install -e sdk/python

echo [3/4] 正在安装 AllBot Node.js SDK...
cd sdk/nodejs && npm install && npm run build && cd ../..

REM 4. 创建虚拟环境（全局依赖）
echo [4/4] 正在创建全局依赖环境...
python -m venv .venv
call .venv\Scripts\activate.bat
pip install grpcio grpcio-tools

echo ========================================
echo 安装完成！
echo 启动命令：allbot.exe
echo Web UI：http://localhost:3000
echo ========================================
pause
```

#### Linux/Mac 安装脚本（install.sh）

```bash
#!/bin/bash
echo "========================================"
echo "AllBot 自动安装脚本"
echo "========================================"

# 1. 检查并安装 Python
if ! command -v python3 &> /dev/null; then
    echo "[1/4] 正在安装 Python..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install python@3.11
    else
        sudo apt-get update
        sudo apt-get install -y python3.11 python3-pip
    fi
else
    echo "[1/4] Python 已安装"
fi

# 2. 检查并安装 Node.js
if ! command -v node &> /dev/null; then
    echo "[2/4] 正在安装 Node.js..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install node@20
    else
        curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
        sudo apt-get install -y nodejs
    fi
else
    echo "[2/4] Node.js 已安装"
fi

# 3. 安装 AllBot SDK
echo "[3/4] 正在安装 AllBot Python SDK..."
pip3 install -e sdk/python

echo "[3/4] 正在安装 AllBot Node.js SDK..."
cd sdk/nodejs && npm install && npm run build && cd ../..

# 4. 创建虚拟环境（全局依赖）
echo "[4/4] 正在创建全局依赖环境..."
python3 -m venv .venv
source .venv/bin/activate
pip install grpcio grpcio-tools

echo "========================================"
echo "安装完成！"
echo "启动命令：./allbot"
echo "Web UI：http://localhost:3000"
echo "========================================"
```

### 3. 全局依赖管理系统

#### 设计原理

**问题**：每个插件都安装依赖会导致：
- 磁盘空间浪费（相同依赖重复安装）
- 安装时间长
- 版本冲突

**解决方案**：全局依赖 + 插件声明

#### Python 全局依赖管理

```
allbot/
├─ .venv/                    # 全局 Python 虚拟环境
│   ├─ Lib/site-packages/    # 所有插件共享的依赖
│   └─ Scripts/python.exe
├─ runtime/
│   └─ python_deps.json      # 已安装的依赖清单
└─ plugins/
    └─ weather/
        ├─ plugin.json       # 声明依赖
        └─ main.py
```

**插件声明依赖**（plugin.json）：
```json
{
  "name": "天气插件",
  "runtime": "python",
  "dependencies": {
    "requests": "^2.31.0",
    "beautifulsoup4": "^4.12.0"
  }
}
```

**框架自动安装**：
1. 插件加载时，读取 `dependencies`
2. 检查 `runtime/python_deps.json`，判断是否已安装
3. 未安装则自动执行：`pip install requests==2.31.0`
4. 更新 `python_deps.json`

**启动插件时**：
```bash
# 使用全局虚拟环境的 Python
.venv/Scripts/python.exe plugins/weather/main.py
```

#### Node.js 全局依赖管理

```
allbot/
├─ runtime/
│   ├─ node_modules/         # 全局 node_modules
│   └─ package.json          # 全局依赖清单
└─ plugins/
    └─ translate/
        ├─ plugin.json
        └─ main.js
```

**插件声明依赖**（plugin.json）：
```json
{
  "name": "翻译插件",
  "runtime": "nodejs",
  "dependencies": {
    "axios": "^1.6.0",
    "cheerio": "^1.0.0"
  }
}
```

**框架自动安装**：
1. 插件加载时，读取 `dependencies`
2. 合并到 `runtime/package.json`
3. 执行：`npm install --prefix runtime`

**启动插件时**：
```bash
# 设置 NODE_PATH 指向全局 node_modules
NODE_PATH=runtime/node_modules node plugins/translate/main.js
```

#### 依赖管理 API

```go
// core/deps/manager.go
type DependencyManager struct {
    pythonEnv  string
    nodeModules string
}

func (dm *DependencyManager) InstallPythonDeps(deps map[string]string) error {
    for pkg, version := range deps {
        if !dm.isPythonPackageInstalled(pkg, version) {
            cmd := exec.Command(dm.pythonEnv+"/bin/pip", "install", pkg+"=="+version)
            if err := cmd.Run(); err != nil {
                return err
            }
        }
    }
    return nil
}

func (dm *DependencyManager) InstallNodeDeps(deps map[string]string) error {
    // 更新 runtime/package.json
    // 执行 npm install
    return nil
}
```

#### 优势对比

| 方案 | 磁盘占用 | 安装时间 | 版本管理 |
|-----|---------|---------|---------|
| 每个插件独立安装 | 高（重复安装） | 慢 | 复杂 |
| **全局依赖管理** | 低（共享依赖） | 快 | 简单 |

**示例**：
- 10 个插件都用 `requests`
- 独立安装：10 × 5MB = 50MB
- 全局管理：1 × 5MB = 5MB

---

## 技术难点与解决方案

### 1. 多文件插件加密

**问题**：插件可能有多个文件和子目录，如何加密？

**解决方案**：
- 打包整个插件目录为 tar.gz
- AES-256 加密整个包
- 运行时解密到虚拟文件系统（memfs）
- Python/Node.js 通过自定义 import hook 从虚拟文件系统加载

### 2. 连续对话的并发控制

**问题**：多个用户同时使用 listen，如何隔离？

**解决方案**：
- 使用 `user_id:group_id` 作为会话 key
- 每个会话独立的 Channel
- 超时自动清理
- 新会话覆盖旧会话

### 3. 跨语言通信

**问题**：Go 核心如何调用 Python/Node.js 插件？

**解决方案**：
- 使用 gRPC 作为通信协议
- 插件启动独立进程
- 通过 protobuf 定义统一接口
- 支持双向流式通信

### 4. 平台差异屏蔽

**问题**：不同平台 API 差异大，如何统一？

**解决方案**：
- 定义统一的中间件 API
- 每个平台实现 Adapter 接口
- 平台特定功能通过 `ctx.platform` 判断
- 提供降级方案（不支持的功能返回错误）

---

## 对比其他框架

| 特性 | nonebot | astrbot | AllBot |
|-----|---------|---------|--------|
| 插件开发 | 复杂（事件系统） | 中等 | 极简（单函数） |
| 多语言支持 | 仅 Python | 仅 Python | Python + Node.js |
| 商业化 | 不支持 | 不支持 | 内置支持 |
| 市场机制 | 中心化 | 中心化 | 去中心化 |
| 源码保护 | 无 | 无 | AES-256 加密 |
| 连续对话 | 需要手动管理 | 需要手动管理 | 内置 listen() |
| 部署 | 需要 Python 环境 | 需要 Python 环境 | 单文件可执行 |
| 类型安全 | 弱类型 | 弱类型 | 强类型（Go核心） |

---

## 开源策略

### 开源部分

- ✅ Go 核心框架（完全开源）
- ✅ Python/Node.js SDK（完全开源）
- ✅ 市场服务器模板（完全开源）
- ✅ CLI 工具（完全开源）
- ✅ 示例插件（完全开源）

### 闭源部分（可选）

- ❌ 企业版功能（集群部署、监控、审计）
- ❌ 高级加密模块（更强的反调试）
- ❌ 官方市场服务（提供托管服务）

### 盈利模式

1. **捐赠/赞助**：GitHub Sponsors、爱发电
2. **企业版**：提供企业级功能和技术支持
3. **官方市场**：提供托管的插件市场服务（收取服务费）
4. **培训/咨询**：提供插件开发培训和定制开发服务

---

## 总结

AllBot 是一个**极简、开放、商业友好**的机器人框架：

- **极简**：单正则 + 单函数，零学习成本
- **开放**：开源核心，去中心化市场
- **商业友好**：内置加密和授权，开发者可以赚钱

核心优势：
1. 插件开发体验极佳（比 nonebot/astrbot 简单10倍）
2. 支持插件商业化（开发者可以卖插件赚钱）
3. 去中心化（无平台抽成，开发者自建市场）
4. 多语言支持（Python + Node.js）
5. 源码保护（AES-256 加密 + 虚拟文件系统）
6. 连续对话（内置 listen() 函数）

目标用户：
- 想要快速开发机器人的开发者
- 想要通过插件赚钱的开发者
- 需要商业化机器人解决方案的企业
