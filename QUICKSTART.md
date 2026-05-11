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
2. 使用默认账号登录：
   - 用户名：`admin`
   - 密码：`admin123`

## 配置平台适配器

### 方式一：通过 Web UI（推荐）

1. 登录后切换到"平台配置"标签
2. 点击"添加平台"按钮
3. 选择平台类型（QQ/微信/Telegram）
4. 填写配置信息
5. 选择"启用"状态
6. 点击"保存" - **配置立即生效，无需重启！**

### QQ 平台配置示例

**前置条件**：需要先安装并运行 [go-cqhttp](https://github.com/Mrs4s/go-cqhttp)

**配置项**：
- **API 地址**：`http://localhost:5700`（go-cqhttp 的 HTTP API 地址）
- **监听地址**：`:8080`（AllBot 接收消息的端口）
- **状态**：启用

**go-cqhttp 配置**：
```yaml
# config.yml
servers:
  - http:
      host: 0.0.0.0
      port: 5700
      post:
        - url: http://localhost:8080  # AllBot 的监听地址
          secret: ''
```

### 微信平台配置

**配置项**：
- **App ID**：微信公众号/企业微信的 App ID
- **App Secret**：对应的 App Secret
- **状态**：启用

> 注意：微信适配器尚未实现，配置后暂时无法使用

### Telegram 平台配置

**配置项**：
- **Bot Token**：从 @BotFather 获取的 Bot Token
- **状态**：启用

> 注意：Telegram 适配器尚未实现，配置后暂时无法使用

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
from allbot_sdk import Plugin, Message

plugin = Plugin(
    name="示例插件",
    version="1.0.0",
    description="这是一个示例插件"
)

@plugin.on_message(r"^你好")
async def handle_hello(msg: Message):
    await msg.reply("你好！我是机器人")

if __name__ == "__main__":
    plugin.run()
```

### Node.js 插件示例

```javascript
const { Plugin } = require('allbot-sdk');

const plugin = new Plugin({
    name: '示例插件',
    version: '1.0.0',
    description: '这是一个示例插件'
});

plugin.onMessage(/^你好/, async (msg) => {
    await msg.reply('你好！我是机器人');
});

plugin.run();
```

## 技术架构

- **核心框架**：Go 1.21+
- **Web UI**：原生 HTML/CSS/JavaScript（Vue 3 版本开发中）
- **数据库**：SQLite 3
- **插件通信**：HTTP + JSON
- **支持语言**：Python 3.7+、Node.js 14+

## 更多信息

- 动态配置系统详细文档：[DYNAMIC_CONFIG.md](DYNAMIC_CONFIG.md)
- 项目架构文档：[project.md](project.md)
- Phase 3 实现总结：[PHASE3_SUMMARY.md](PHASE3_SUMMARY.md)

## 许可证

MIT License
