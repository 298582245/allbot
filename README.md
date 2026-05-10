# AllBot - 去中心化多平台机器人框架

极简、开放、商业友好的机器人框架

## 特性

- **极简插件开发**：单正则 + 单函数，零学习成本
- **多语言支持**：Python 和 Node.js 插件
- **连续对话**：内置 `listen()` 支持多轮对话
- **多平台适配**：统一 API 适配 QQ/微信/Telegram
- **去中心化市场**：开发者自建市场，无平台抽成
- **源码保护**：AES-256 加密（Phase 2 实现）

## 快速开始

### 1. 安装依赖

```bash
# 安装 Go（需要 1.19+）
go mod tidy

# 安装 Python SDK
cd sdk/python
pip install -e .

# 安装 Node.js SDK
cd sdk/nodejs
npm install
```

### 2. 启动框架

```bash
# 启动 AllBot
go run main.go --plugins=./plugins --qq-api=http://localhost:5700
```

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
  "trigger": "你好.*"
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

## 项目结构

```
allbot/
├─ core/                    # Go 核心框架
│   ├─ router/              # 消息路由器
│   ├─ plugin/              # 插件管理器
│   ├─ adapter/             # 平台适配器
│   ├─ session/             # 会话管理器（listen）
│   └─ types/               # 数据类型定义
├─ sdk/                     # 插件 SDK
│   ├─ python/              # Python SDK
│   └─ nodejs/              # Node.js SDK
├─ proto/                   # gRPC 协议定义
├─ examples/                # 示例插件
│   └─ weather/             # 天气插件
├─ main.go                  # 主程序入口
└─ project.md               # 项目设计文档
```

## Phase 1 完成状态

✅ **已完成**：
- Go 核心框架（消息路由、会话管理、插件管理）
- gRPC 通信协议定义
- Python SDK（Context API）
- Node.js SDK（Context API）
- QQ 平台适配器（基于 go-cqhttp）
- 示例插件（天气插件）

⏳ **待实现**（Phase 2）：
- gRPC 服务端/客户端实现
- 插件加密系统
- 虚拟文件系统
- 授权验证

⏳ **待实现**（Phase 3）：
- 市场服务器模板
- CLI 工具
- 支付集成

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

## 配置

### 命令行参数

```bash
--plugins=./plugins          # 插件目录
--qq-api=http://localhost:5700  # go-cqhttp API 地址
```

### 环境变量

```bash
ALLBOT_PLUGIN_ID=my-plugin   # 插件 ID（自动设置）
ALLBOT_GRPC_PORT=50051       # gRPC 端口（自动设置）
```

## 架构设计

详见 [project.md](project.md)

## 开发路线图

- [x] Phase 1（3个月）- 核心框架
- [ ] Phase 2（2个月）- 加密系统
- [ ] Phase 3（2个月）- 市场系统
- [ ] Phase 4（持续）- 生态建设

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License
