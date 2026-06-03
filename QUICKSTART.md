# AllBot 快速使用指南

## 启动程序

```bash
# Windows
allbot.exe --plugins=./plugins

# Linux/Mac
./allbot --plugins=./plugins
```

## 访问 Web UI

1. 打开浏览器访问：http://localhost:3000
2. 使用管理员账号登录：
   - 用户名：`admin`
   - 密码：首次启动时控制台输出的随机密码

## 配置平台适配器

### 方式一：通过 Web UI（推荐）

1. 登录后切换到"平台配置"标签
2. 点击"添加平台"按钮
3. 选择平台类型（QQ/微信/Telegram）
4. 填写配置信息
5. 选择"启用"状态
6. 点击"保存" - **配置立即生效，无需重启！**

### QQ 平台配置示例

**前置条件**：需要先安装并运行 NapCat，并开启 OneBot 正向 WebSocket 服务。

**配置项**：
- **服务地址**：`ws://127.0.0.1:3001`（NapCat 提供的 OneBot WebSocket 地址）
- **访问令牌**：如果 NapCat 配置了 Access Token，这里填写同一个值；没有配置可留空
- **状态**：启用

AllBot 不再本地开启 QQ 回调端口，而是主动连接 NapCat。本地测试时只要 NapCat 服务地址能从本机访问即可。

### 微信平台配置

**配置项**：
- **App ID**：微信公众号/企业微信的 App ID
- **App Secret**：对应的 App Secret
- **状态**：启用

> 注意：微信适配器尚未实现，配置后暂时无法使用

### Telegram 平台配置

**前置条件**：需要先从 [@BotFather](https://t.me/BotFather) 创建 Bot 并获取 Token

**配置项**：
- **Bot Token**：从 @BotFather 获取的 Bot Token（格式：`123456789:ABCdefGHIjklMNOpqrsTUVwxyz`）
- **状态**：启用

**功能特性**：
- ✅ 支持私聊和群组消息
- ✅ 长轮询接收消息（无需 Webhook）
- ✅ 发送文本、图片、文件
- ✅ 获取群组信息
- ✅ @提及用户

**创建 Bot 步骤**：
1. 在 Telegram 中搜索 @BotFather
2. 发送 `/newbot` 命令
3. 按提示设置 Bot 名称和用户名
4. 获取 Bot Token
5. 在 AllBot Web UI 中配置 Token 并启用

## 插件管理

### 安装插件

```bash
# 使用 CLI 工具（需要先实现）
python cli/allbot.py plugin install <plugin-name>

# 或手动放置到 plugins 目录
mkdir -p plugins/my-plugin
# 将插件文件放入该目录
```

### 查看插件

在 Web UI 的"插件管理"标签中可以查看所有已安装的插件及其运行状态。

## 配置文件

### 数据库文件

- **config.db**：存储平台配置、插件配置等
- 位置：程序运行目录
- 类型：SQLite 数据库

### 运行时目录

- **runtime/**：存储 Python 虚拟环境和 Node.js 依赖
  - `runtime/python/venv/`：全局 Python 虚拟环境
  - `runtime/nodejs/node_modules/`：全局 Node.js 依赖

## 常见问题

### 1. 修改配置后需要重启吗？

**不需要！** 这是动态配置系统的核心特性。在 Web UI 中修改平台配置后，适配器会自动重启并应用新配置，无需重启整个程序。

### 2. Python 环境初始化失败怎么办？

如果看到 "初始化 Python 环境失败" 的警告，不影响程序运行。只是 Python 插件暂时无法使用，Web UI 和配置管理功能正常。

解决方法：
- 确保系统已安装 Python 3.7+
- 检查 Python 是否在 PATH 中
- 手动创建虚拟环境：`python -m venv runtime/python/venv`

### 3. 如何查看日志？

程序日志直接输出到控制台。如需保存日志：

```bash
# Windows
allbot.exe --plugins=./plugins > allbot.log 2>&1

# Linux/Mac
./allbot --plugins=./plugins > allbot.log 2>&1
```

### 4. 如何停止程序？

在控制台按 `Ctrl+C` 即可优雅退出。程序会自动：
- 停止所有适配器
- 停止所有插件进程
- 关闭数据库连接

## 开发插件

### Python 插件示例

```python
import os
import sys

sdk_path = os.path.join(os.path.dirname(__file__), "../../sdk/python")
sys.path.insert(0, sdk_path)

from allbot_direct import run_direct


async def handle(ctx):
    if ctx.content.startswith("你好"):
        await ctx.reply("你好！我是机器人")

if __name__ == "__main__":
    run_direct(handle)
```

### Node.js 插件示例

```javascript
const path = require('path');

const sdkPath = path.join(__dirname, '../../sdk/nodejs');
const { runDirect } = require(path.join(sdkPath, 'allbot_direct'));

async function handle(ctx) {
    if ((ctx.content || '').startsWith('你好')) {
        await ctx.reply('你好！我是机器人');
    }
}

runDirect(handle);
```

## 技术架构

- **核心框架**：Go 1.21+
- **Web UI**：原生 HTML/CSS/JavaScript（Vue 3 版本开发中）
- **数据库**：SQLite 3
- **插件通信**：Direct stdin/stdout JSON 行协议
- **支持语言**：Python 3.7+、Node.js 14+

## 更多信息

- 动态配置系统详细文档：[DYNAMIC_CONFIG.md](DYNAMIC_CONFIG.md)
- 项目架构文档：[project.md](project.md)
- Phase 3 实现总结：[PHASE3_SUMMARY.md](PHASE3_SUMMARY.md)

## 许可证

MIT License
