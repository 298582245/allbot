# AllBot Windows 测试指南

## 快速测试步骤

### 1. 启动 AllBot 核心框架

在项目根目录打开命令行（CMD或PowerShell），运行：

```bash
# 直接运行编译好的可执行文件
.\allbot.exe
```

启动成功后会看到：
```
AllBot 启动成功！
- Web UI: http://localhost:3000
```

**注意**：保持这个窗口运行，不要关闭。

### 2. 访问 Web UI 管理界面

打开浏览器，访问：http://localhost:3000

**默认登录账号**：
- 用户名：`admin`
- 密码：`admin123`

### 3. 配置平台适配器

登录后，点击左侧菜单"平台配置"：

#### 配置 QQ 平台（基于 go-cqhttp）

1. 点击"添加平台"
2. 选择平台：QQ
3. 填写配置：
   ```json
   {
     "host": "127.0.0.1",
     "port": 5700,
     "access_token": ""
   }
   ```
4. 启用开关打开
5. 点击"保存"

**前置条件**：需要先安装并运行 go-cqhttp
- 下载：https://github.com/Mrs4s/go-cqhttp/releases
- 配置 go-cqhttp 的 HTTP 通信地址为 `http://127.0.0.1:5700`

#### 配置 Telegram 平台

1. 点击"添加平台"
2. 选择平台：Telegram
3. 填写配置：
   ```json
   {
     "bot_token": "your_bot_token_here"
   }
   ```
4. 启用开关打开
5. 点击"保存"

**获取 Bot Token**：
- 在 Telegram 中找 @BotFather
- 发送 `/newbot` 创建机器人
- 获取 token

### 4. 测试插件功能

#### 方式一：使用现有示例插件

项目已包含两个示例插件：

**天气插件** (`examples/weather/`)：
- 触发词：`天气 [城市名]`
- 示例：发送"天气 北京"

**翻译插件** (`examples/translator/`)：
- 自动检测中英文并翻译
- 示例：发送"Hello"会翻译成中文

在 Web UI 的"插件管理"页面可以看到这些插件的状态。

#### 方式二：创建测试插件

1. 在 `plugins` 目录创建新文件夹，如 `test-plugin`
2. 创建 `plugin.json`：
```json
{
  "name": "测试插件",
  "version": "1.0.0",
  "runtime": "python",
  "entry": "main.py",
  "platforms": ["qq", "telegram"],
  "trigger": "测试.*"
}
```

3. 创建 `main.py`：
```python
async def handle(ctx):
    await ctx.reply(f"收到消息：{ctx.content}")
    await ctx.reply(f"平台：{ctx.platform}")
    await ctx.reply(f"用户ID：{ctx.user_id}")
```

4. 重启 AllBot，插件会自动加载

### 5. 发送测试消息

在配置好的平台（QQ或Telegram）中：

1. **测试天气插件**：
   - 发送：`天气 北京`
   - 预期：返回北京的天气信息

2. **测试翻译插件**：
   - 发送：`Hello World`
   - 预期：返回中文翻译

3. **测试自定义插件**：
   - 发送：`测试一下`
   - 预期：返回消息内容、平台和用户ID

### 6. 查看日志

在 Web UI 的"日志查看"页面可以实时查看系统日志：
- 插件加载日志
- 消息处理日志
- 错误日志

日志每3秒自动刷新。

## 常见问题

### Q1: AllBot 启动失败？

**检查端口占用**：
```bash
netstat -ano | findstr :3000
```

如果端口被占用，可以修改 `main.go` 中的端口号，然后重新编译：
```bash
go build -o allbot.exe
```

### Q2: 插件没有响应？

1. 检查插件是否加载成功（Web UI - 插件管理）
2. 检查触发规则是否匹配
3. 查看日志是否有错误信息
4. 确认平台适配器已启用

### Q3: Python 插件报错？

确保已安装 Python 依赖：
```bash
# 检查 Python 版本
python --version

# 安装依赖
pip install requests
```

### Q4: 无法连接 go-cqhttp？

1. 确认 go-cqhttp 正在运行
2. 检查 go-cqhttp 的配置文件中 HTTP 通信地址
3. 确认防火墙没有阻止连接

### Q5: Telegram Bot 不响应？

1. 确认 Bot Token 正确
2. 检查网络连接（Telegram API 需要访问外网）
3. 确认 Bot 已启动（/start 命令）

## 测试 Phase 3 新功能

### 测试市场服务器（可选）

如果要测试支付集成和开发者后台：

1. **安装 Docker Desktop**（Windows版）

2. **启动市场服务器**：
```bash
cd market-server
docker-compose up -d
```

3. **访问开发者后台**：
   - 地址：http://localhost:8000/admin
   - 需要先构建前端：
     ```bash
     cd admin-ui
     npm install
     npm run build
     ```

4. **测试支付功能**：
   - 创建插件
   - 设置价格
   - 创建订单
   - 查看订单列表

## 性能测试

### 消息处理性能

使用测试脚本发送大量消息：

```python
# test_performance.py
import asyncio
import aiohttp

async def send_message(session, content):
    # 模拟发送消息到 AllBot
    pass

async def main():
    async with aiohttp.ClientSession() as session:
        tasks = [send_message(session, f"测试{i}") for i in range(100)]
        await asyncio.gather(*tasks)

asyncio.run(main())
```

### 并发插件测试

同时触发多个插件，观察响应时间和资源占用。

## 下一步

测试完成后，你可以：

1. **开发自己的插件**
   - 参考 `examples/` 目录的示例
   - 查看插件开发文档

2. **部署到生产环境**
   - 参考 `DEPLOYMENT.md`
   - 配置真实的平台账号

3. **搭建插件市场**
   - 部署市场服务器
   - 配置支付方式
   - 发布插件

## 技术支持

遇到问题？
- 查看日志：Web UI - 日志查看
- 查看文档：README.md, QUICKSTART.md
- 提交 Issue：GitHub Issues

祝测试顺利！🎉
