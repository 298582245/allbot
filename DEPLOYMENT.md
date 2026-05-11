# AllBot 部署指南

## 快速开始（5 分钟部署）

### 前置要求

- Go 1.21+
- Python 3.7+ 或 Node.js 14+（用于插件开发）
- Git

### 一键部署

#### Windows

```bash
# 1. 克隆仓库
git clone https://github.com/yourusername/allbot.git
cd allbot

# 2. 编译
go build -o allbot.exe

# 3. 启动
allbot.exe --plugins=./plugins
```

#### Linux/Mac

```bash
# 1. 克隆仓库
git clone https://github.com/yourusername/allbot.git
cd allbot

# 2. 编译
go build -o allbot

# 3. 启动
./allbot --plugins=./plugins
```

### 访问管理后台

启动后访问：http://localhost:3000

**默认账号**：
- 用户名：`admin`
- 密码：`admin123`

---

## 配置平台适配器

### 方式一：Web UI 配置（推荐）

1. 登录管理后台
2. 点击左侧菜单"平台配置"
3. 点击"添加平台"按钮
4. 选择平台类型并填写配置
5. 启用并保存 - **配置立即生效！**

### 方式二：直接修改数据库

```bash
# 使用 SQLite 客户端
sqlite3 config.db

# 查看配置
SELECT * FROM adapters;

# 添加配置
INSERT INTO adapters (platform, enabled, config, created_at, updated_at)
VALUES ('qq', 1, '{"api_url":"http://localhost:5700","listen_addr":":8080"}', datetime('now'), datetime('now'));
```

---

## 平台配置详解

### QQ 平台

**前置条件**：
1. 下载 [go-cqhttp](https://github.com/Mrs4s/go-cqhttp/releases)
2. 配置 go-cqhttp 的 `config.yml`：

```yaml
servers:
  - http:
      host: 0.0.0.0
      port: 5700
      post:
        - url: http://localhost:8080  # AllBot 监听地址
          secret: ''
```

3. 启动 go-cqhttp

**AllBot 配置**：
- API 地址：`http://localhost:5700`
- 监听地址：`:8080`

### Telegram 平台

**前置条件**：
1. 在 Telegram 中搜索 @BotFather
2. 发送 `/newbot` 创建 Bot
3. 获取 Bot Token

**AllBot 配置**：
- Bot Token：`123456789:ABCdefGHIjklMNOpqrsTUVwxyz`

**无需其他配置**，AllBot 使用长轮询自动接收消息。

### 微信平台

**状态**：开发中

---

## 安装插件

### 方式一：复制示例插件

```bash
# 复制天气插件
cp -r examples/weather plugins/weather

# 复制翻译插件
cp -r examples/translator plugins/translator

# 重启 AllBot（或等待自动重载）
```

### 方式二：使用 CLI 工具（开发中）

```bash
# 从市场安装
allbot plugin install weather

# 从本地安装
allbot plugin install ./my-plugin
```

---

## 开发插件

### Python 插件示例

**目录结构**：
```
my-plugin/
├── plugin.json
└── main.py
```

**plugin.json**：
```json
{
  "name": "我的插件",
  "version": "1.0.0",
  "description": "插件描述",
  "runtime": "python",
  "entry": "main.py",
  "platforms": ["qq", "telegram", "wechat"],
  "trigger": "^你好.*",
  "dependencies": {
    "requests": "2.31.0"
  }
}
```

**main.py**：
```python
async def handle(ctx):
    await ctx.reply("你好！我是机器人")
```

### Node.js 插件示例

**plugin.json**：
```json
{
  "name": "我的插件",
  "version": "1.0.0",
  "runtime": "nodejs",
  "entry": "index.js",
  "trigger": "^hello.*"
}
```

**index.js**：
```javascript
module.exports = async function handle(ctx) {
    await ctx.reply("Hello! I'm a bot");
}
```

---

## 生产环境部署

### Docker 部署

**Dockerfile**：
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o allbot

FROM alpine:latest

RUN apk add --no-cache python3 nodejs npm

WORKDIR /app
COPY --from=builder /app/allbot .
COPY --from=builder /app/web ./web
COPY --from=builder /app/plugins ./plugins

EXPOSE 3000

CMD ["./allbot", "--plugins=/app/plugins"]
```

**构建和运行**：
```bash
# 构建镜像
docker build -t allbot:latest .

# 运行容器
docker run -d \
  -p 3000:3000 \
  -v $(pwd)/plugins:/app/plugins \
  -v $(pwd)/config.db:/app/config.db \
  --name allbot \
  allbot:latest
```

### Systemd 服务（Linux）

**创建服务文件** `/etc/systemd/system/allbot.service`：
```ini
[Unit]
Description=AllBot Service
After=network.target

[Service]
Type=simple
User=allbot
WorkingDirectory=/opt/allbot
ExecStart=/opt/allbot/allbot --plugins=/opt/allbot/plugins
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

**启动服务**：
```bash
sudo systemctl daemon-reload
sudo systemctl enable allbot
sudo systemctl start allbot
sudo systemctl status allbot
```

### Nginx 反向代理

**配置文件** `/etc/nginx/sites-available/allbot`：
```nginx
server {
    listen 80;
    server_name bot.example.com;

    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

---

## 性能优化

### 1. 数据库优化

```bash
# 定期清理数据库
sqlite3 config.db "VACUUM;"

# 创建索引（已自动创建）
sqlite3 config.db "CREATE INDEX IF NOT EXISTS idx_adapters_platform ON adapters(platform);"
```

### 2. 插件性能

- 使用全局依赖管理，避免重复安装
- 限制插件并发数
- 设置合理的超时时间

### 3. 系统资源

**推荐配置**：
- CPU：2 核心
- 内存：2GB
- 磁盘：10GB

**实际占用**：
- 核心框架：< 100MB
- 每个插件：< 50MB
- 数据库：< 10MB

---

## 监控和日志

### 查看日志

**方式一：Web UI**
- 访问管理后台
- 点击"日志查看"
- 实时查看系统日志

**方式二：命令行**
```bash
# 启动时输出到文件
./allbot --plugins=./plugins > allbot.log 2>&1

# 实时查看
tail -f allbot.log
```

### 系统监控

**查看系统状态**：
- 访问管理后台仪表盘
- 查看运行时间、插件数、消息数

**API 监控**：
```bash
# 获取系统状态
curl -H "Authorization: Bearer <token>" http://localhost:3000/api/system/status
```

---

## 备份和恢复

### 备份

```bash
# 备份配置数据库
cp config.db config.db.backup

# 备份插件
tar -czf plugins.tar.gz plugins/

# 完整备份
tar -czf allbot-backup-$(date +%Y%m%d).tar.gz \
  config.db \
  plugins/ \
  web/
```

### 恢复

```bash
# 恢复配置
cp config.db.backup config.db

# 恢复插件
tar -xzf plugins.tar.gz

# 重启服务
systemctl restart allbot
```

---

## 故障排查

### 问题 1：启动失败

**症状**：`AllBot 启动失败`

**解决方案**：
1. 检查端口占用：`netstat -ano | findstr :3000`
2. 检查 Go 版本：`go version`
3. 查看详细日志

### 问题 2：插件无法加载

**症状**：插件列表为空

**解决方案**：
1. 检查插件目录：`ls -la plugins/`
2. 检查 plugin.json 格式
3. 查看插件日志

### 问题 3：平台连接失败

**症状**：适配器显示"已停止"

**解决方案**：
1. 检查平台配置是否正确
2. 测试网络连接
3. 查看适配器日志

### 问题 4：Web UI 无法访问

**症状**：浏览器无法打开管理后台

**解决方案**：
1. 检查 AllBot 是否启动
2. 检查防火墙设置
3. 尝试 `http://127.0.0.1:3000`

---

## 安全建议

### 1. 修改默认密码

首次登录后立即修改管理员密码：
1. 访问"系统设置"
2. 点击"修改密码"
3. 设置强密码

### 2. 配置防火墙

```bash
# 仅允许本地访问（推荐）
# 在 Nginx 中配置访问控制

# 或使用 iptables
iptables -A INPUT -p tcp --dport 3000 -s 127.0.0.1 -j ACCEPT
iptables -A INPUT -p tcp --dport 3000 -j DROP
```

### 3. 使用 HTTPS

配置 Nginx SSL：
```nginx
server {
    listen 443 ssl;
    server_name bot.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

### 4. 定期更新

```bash
# 拉取最新代码
git pull origin master

# 重新编译
go build -o allbot

# 重启服务
systemctl restart allbot
```

---

## 常见问题

### Q: 如何添加新平台？

A: 在 Web UI 的"平台配置"中点击"添加平台"，选择平台类型并填写配置。

### Q: 配置修改后需要重启吗？

A: 不需要！动态配置系统支持热重载，修改后立即生效。

### Q: 如何开发自己的插件？

A: 参考 `examples/` 目录下的示例插件，创建 `plugin.json` 和处理函数即可。

### Q: 支持哪些平台？

A: 目前支持 QQ（go-cqhttp）和 Telegram（Bot API），微信正在开发中。

### Q: 如何查看日志？

A: 在 Web UI 的"日志查看"页面可以实时查看系统日志。

### Q: 插件依赖如何管理？

A: 在 `plugin.json` 中声明依赖，框架会自动安装到全局环境。

---

## 获取帮助

- **文档**：查看 `README.md` 和 `COMPLETE_SUMMARY.md`
- **示例**：参考 `examples/` 目录
- **问题反馈**：https://github.com/yourusername/allbot/issues
- **社区讨论**：https://github.com/yourusername/allbot/discussions

---

## 更新日志

### v1.0.0 (2026-05-11)

**核心功能**：
- ✅ Go 核心框架
- ✅ Python/Node.js SDK
- ✅ QQ/Telegram 平台适配器
- ✅ 动态配置系统
- ✅ Vue 3 + Element Plus 管理后台
- ✅ 全局依赖管理
- ✅ 插件加密系统
- ✅ 去中心化市场

**示例插件**：
- ✅ 天气插件
- ✅ 翻译插件

---

**最后更新**：2026-05-11
