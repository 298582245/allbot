# AllBot

AllBot 是一个基于 Go 的多平台机器人框架，内置 Web 管理后台、插件运行时、平台适配器、定时任务、Open API、数据管理和 Node.js/Python Direct SDK。项目目标是让机器人能力通过插件快速扩展，并在管理后台完成平台账号、插件配置、权限、依赖和运行任务的维护。

## 功能概览

- 多平台机器人接入：支持 QQ、Telegram、QQ 官方机器人、钉钉、飞书、微信公众号、Web 聊天室等适配器，同一平台可配置多个机器人实例。
- 插件系统：支持 Node.js 与 Python 插件，按正则触发、平台限制、机器人实例限制、优先级和访问控制执行。
- Web 管理后台：提供插件、适配器、Web 聊天、定时任务、脚本任务、数据、依赖、日志、权限、设置等页面。
- Direct SDK：为插件提供消息回复、监听、主动发送、数据库、定时任务、脚本运行、积分、管理员身份列表等能力。
- 定时任务：可伪造用户消息触发插件指令，支持手动任务、插件声明任务、多行 cron 表达式。
- Open API：可在后台创建 HTTP 接口，用 Node.js/Python 脚本处理外部请求。
- 数据管理：插件可创建私有数据表，并在后台配置可视化展示。
- 账号青龙封装：SDK 内置账号登录、授权、查询、运行、CK 检测、过期提醒等通用封装。

## 技术栈

- 后端：Go，SQLite（`modernc.org/sqlite`）
- 前端：Vue 3、Vite、Element Plus、Pinia
- 插件运行时：Node.js、Python
- 数据库：`config.db`，首次启动自动创建
- 管理后台静态文件：`web/`，由 `web-ui/` 构建生成，并在 Go 构建时嵌入二进制

## 快速启动

### 1. 准备环境

- Go：版本以 `go.mod` 为准
- Node.js：用于运行 Node.js 插件和构建前端
- Python：用于运行 Python 插件

### 2. 构建管理后台

如果改动了 `web-ui/`，需要先构建前端：

```powershell
npm --prefix web-ui install
npm --prefix web-ui run build
```

构建产物会输出到 `web/`，用于后续 Go 构建时嵌入二进制。

### 3. 构建后端

Windows：

```powershell
go test ./...
go build -o allbot.exe .
```

Linux：

```bash
go test ./...
go build -o allbot .
```

低配服务器建议不要在服务器现场编译，直接从 GitHub Release 下载与服务器架构匹配的 `allbot-v版本-linux-amd64` 或 `allbot-v版本-linux-arm64`。

### 4. 启动

Windows：

```powershell
.\allbot.exe --plugins=.\plugins
```

Linux：

```bash
./allbot --plugins=./plugins
```

默认 Web 端口是 `3000`。启动后访问：

```text
http://localhost:3000
```

首次启动会自动生成后台管理员密码，并输出到控制台。默认管理员账号为 `admin`。

如需修改端口，可设置环境变量 `ALLBOT_WEB_PORT`：

```powershell
$env:ALLBOT_WEB_PORT = "3001"
.\allbot.exe
```

管理后台默认使用二进制内嵌资源。自动更新只替换 `allbot.exe` / `allbot` 时，内嵌 Web UI 会随二进制一起更新，不会被运行目录残留的旧 `web/` 覆盖。

如需显式使用运行目录外部 `web/`，可设置 `ALLBOT_WEB_MODE=external`：

```powershell
$env:ALLBOT_WEB_MODE = "external"
.\allbot.exe
```

`ALLBOT_WEB_MODE` 仅支持 `embedded` 和 `external`。启用 `external` 后，外部 `web/` 不会随自动更新维护，需要用户自行更新并保证 `web/index.html` 存在。

## Linux 二进制部署（低配服务器推荐）

Release 已提供包含管理后台资源的 Linux 单文件程序，服务器不需要安装 Go，也不需要现场编译。Node.js 和 Python 仅在运行对应语言插件时需要安装。首次部署可保留仓库中的 `sdk/` 和 `openapis/`，只下载二进制替代源码编译。

```bash
cd /opt/allbot
# 在仓库根目录下载与服务器架构匹配的 Release 资产和 checksums-v版本.txt 后执行：
sha256sum -c checksums-v版本.txt --ignore-missing
chmod +x allbot-v版本-linux-amd64
mv allbot-v版本-linux-amd64 allbot
./allbot --plugins=./plugins
```

正式运行建议交由 systemd 管理，服务工作目录必须保持为 `/opt/allbot`：

```ini
[Unit]
Description=AllBot
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/allbot
ExecStart=/opt/allbot/allbot --plugins=/opt/allbot/plugins
Environment=ALLBOT_WEB_PORT=3000
Environment=ALLBOT_WEB_MODE=embedded
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
```

将内容保存为 `/etc/systemd/system/allbot.service` 后执行：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now allbot
sudo systemctl status allbot
sudo journalctl -u allbot -f
```

`config.db`、`plugins/`、`runtime/`、`openapis/`、`sdk/`、`logs/` 和 `backups/` 都保存在 `/opt/allbot`。升级前应备份整个目录；后台一键升级会校验 Release SHA256，并替换当前二进制。

## Docker Compose 部署

Dockerfile 会复用根目录已有的 `web/` 前端产物并嵌入 Go 二进制；如果改动了 `web-ui/`，需要先在本地或 CI 构建前端并提交更新后的 `web/`。构建阶段使用较小的 Alpine Go 镜像并限制 Go 编译并发，但服务器仍需下载构建工具链和运行时镜像。网络较慢或内存较小的服务器应优先使用上面的 Release 二进制部署。

### 1. 准备环境

服务器需要安装 Docker 和 Docker Compose 插件，并放行对外访问端口，默认端口为 `3000`。

### 2. 启动服务

在项目根目录执行：

```bash
docker compose up -d --build
```

查看状态和日志：

```bash
docker compose ps
docker compose logs -f allbot
```

启动后访问：

```text
http://服务器公网IP:3000
```

首次启动会在容器日志中输出管理员密码，默认管理员账号为 `admin`。

### 3. 修改端口

可以通过环境变量修改 Web 端口：

```bash
ALLBOT_WEB_PORT=3001 docker compose up -d --build
```

`docker-compose.yml` 默认绑定 `0.0.0.0`，可从服务器外部访问；如果云服务器还有安全组或防火墙，需要同步放行对应端口。

### 4. 数据持久化

默认使用命名卷 `allbot_data` 挂载到容器内 `/data`。容器重建后以下数据会保留：

```text
/data/config.db      SQLite 配置数据库
/data/allbot         Docker 模式实际运行的程序文件
/data/plugins/       插件目录
/data/runtime/       插件运行时依赖和升级临时文件
/data/openapis/      Open API 脚本与配置目录
/data/sdk/           插件 SDK 文件
/data/logs/          日志目录
/data/backups/       备份目录
```

查看命名卷位置：

```bash
docker volume inspect allbot_allbot_data
```

如果希望直接在宿主机编辑插件或 Open API，可把 `docker-compose.yml` 中的命名卷改成 bind mount，例如：

```yaml
volumes:
  - ./data:/data
```

`/data/sdk` 是持久化的插件 SDK 目录。镜像重新构建后，容器入口脚本会对比镜像 SDK 指纹和 `/data/sdk` 当前内容：如果确认 `/data/sdk` 未被用户修改，会自动同步镜像内新版 SDK；如果检测到用户修改过 `/data/sdk`，会跳过覆盖并在日志中提示。首次引入该同步机制时，如果已有 `/data/sdk` 与镜像内容不一致，也会保守跳过覆盖。如需强制使用镜像新版 SDK，请先备份 `/data/sdk`，再删除 `/data/sdk` 并重启容器。

### 5. Docker 模式升级

Compose 默认设置 `ALLBOT_UPDATE_MODE=docker`。管理员在系统设置页点击“一键升级”或向机器人发送「更新」时，程序会下载 Release 资产并写入升级请求；容器入口脚本检测到请求后，会备份旧的 `/data/allbot`，替换为新版程序并自动重启应用。

也可以通过重新构建镜像更新。建议先单独拉取基础镜像，这样 SSH 断开后再次执行仍可复用已下载的镜像层：

```bash
git pull
docker pull golang:1.26-alpine
docker pull debian:bookworm-slim
docker compose build --progress=plain allbot
docker compose up -d allbot
```

如果 SSH 会话不稳定，可在 `tmux` 或 `screen` 中执行构建；这只能避免终端断线终止任务，无法解决服务器 OOM、磁盘不足或镜像仓库网络异常。

重新构建镜像时，未被用户修改的 `/data/sdk` 会随镜像内 SDK 自动更新；如果日志提示 `/data/sdk` 已被修改，需要手动备份并删除 `/data/sdk` 后重启容器，才会重新初始化为镜像新版 SDK。

## 在线升级

管理后台的“系统设置”页面会检查 GitHub Release，并在发现当前平台可用资产时启用“一键升级”。平台管理员也可以向机器人发送「更新」触发升级。非 Docker 运行时，升级流程会下载新版二进制到 `runtime/update/`，启动临时更新器，关闭当前进程后备份旧程序并替换为新版程序，然后自动重新启动。Docker 运行时默认使用 `ALLBOT_UPDATE_MODE=docker`，由容器入口脚本替换 `/data/allbot` 并重启应用。

Release 资产名称需要包含 `allbot`、目标系统和架构，例如：

```text
allbot-windows-amd64.exe
allbot-linux-amd64
allbot-linux-arm64
```

Release 必须同时提供校验文件，命名为 `checksums-v版本号.txt`，例如 `checksums-v1.0.2.txt`。文件内容使用 SHA256 清单格式，每行包含 `sha256 文件名`：

```text
3b7c...  allbot-windows-amd64.exe
8a2d...  allbot-linux-amd64
```

一键升级会先下载目标二进制和 checksum 文件，只有 SHA256 校验通过后才会备份旧程序并替换新版程序。升级失败时可查看 `runtime/update/backup/` 中的旧程序备份。Docker 模式下旧程序备份默认位于 `/data/runtime/update/backup/`。

## 命令参数与运行文件

### 命令参数

```text
--plugins=./plugins    插件目录，默认 ./plugins
```

### 运行期文件

```text
config.db      SQLite 配置数据库
runtime/       插件运行时依赖、脚本环境和升级临时文件目录
plugins/       插件目录
openapis/      Open API 脚本与配置目录
sdk/           插件 SDK 文件
logs/          日志目录
backups/       备份目录
web/           管理后台构建产物，默认仅用于 Go 构建嵌入；运行时需 ALLBOT_WEB_MODE=external 才会读取
```

Docker 部署时，上述运行期文件位于容器内 `/data`，并通过 `allbot_data` 命名卷持久化。

## 管理后台

管理后台登录后可使用以下页面：

- 仪表盘：系统状态、消息统计、运行信息。
- 插件管理：查看、启用、禁用、重载、删除插件，编辑插件配置与代码。
- 平台配置：添加和维护机器人实例，按平台 schema 动态展示配置项。
- 开放接口：创建 HTTP API，使用 Node.js/Python 脚本处理外部请求。
- Web 聊天：浏览器用户通过 Web 聊天室适配器调用插件。
- 插件面板：查看和访问插件提供的 Web 面板。
- SDK 管理：查看 SDK 文件和开发参考。
- 数据管理：查看插件数据表、编辑数据视图、导入导出数据。
- 依赖管理：维护 Node.js/Python 运行时依赖。
- 定时任务：创建或编辑伪造消息任务，插件声明的任务也会显示在这里。
- 脚本任务：查看插件提交的脚本运行记录和输出。
- 关键字回复：维护内置或自定义关键字回复。
- 日志查看：查看系统运行日志。
- 权限控制：维护系统级或插件级访问控制。
- 系统设置：后台账号、平台管理员、插件目录、积分单位等设置。

## 平台机器人

AllBot 使用适配器注册表提供平台能力和配置 schema。当前内置平台包括：

| 平台标识 | 名称 | 主要配置 | 说明 |
| --- | --- | --- | --- |
| `qq` | QQ 第三方 | `framework`、`server_url`、`access_token`、`http_api_url`、`http_api_access_token` | 当前支持 NapCat 的 OneBot 11 正向通用 WebSocket；WS 与可选 HTTP API 分别配置访问令牌 |
| `telegram` | Telegram | `bot_token`、`proxy_url` | 基于 Telegram Bot API |
| `qq_office` | QQ 官方机器人 | `app_id`、`client_secret`、`api_base_url`、`token_url` | 腾讯 QQ 官方机器人接口 |
| `dingtalk` | 钉钉机器人（Stream） | `client_id`、`client_secret`、`robot_code`、`open_api_host`、`proxy_url` | 钉钉 Stream 模式，无需公网回调地址 |
| `feishu` | 飞书机器人 | `app_id`、`app_secret`、`verification_token`、`encrypt_key`、`callback_path` | 飞书长连接事件订阅与消息发送 |
| `wechat_official` | 微信公众号 | `app_id`、`app_secret`、`token`、`callback_path` | 微信公众号明文模式回调与客服消息 |
| `web` | Web 聊天室 | `smtp_host`、`smtp_port`、`smtp_username`、`smtp_password`、`smtp_from` | 浏览器用户通过 Web 聊天室调用插件 |

配置入口：后台 `平台配置` 页面。

QQ 第三方适配器的 `server_url` 必须填写 `ws://` 或 `wss://` 正向通用 WebSocket 地址；`access_token` 仅用于该 WebSocket。`http_api_url` 仅在需要将 action 固定走 HTTP API 时填写，其独立令牌通过 `http_api_access_token` 配置；HTTP API 不会复用 WebSocket 令牌。系统不会在 WS 与 HTTP 之间猜测或重放请求。当前不支持 OneBot 反向 WebSocket，也未声明支持 LLOneBot。

同一平台可以添加多个机器人实例。插件可以通过 `allowed_adapter_ids` 限定允许触发的机器人实例；定时任务也可以指定某个机器人实例执行。

## 插件开发

### 插件目录结构

一个 Direct 插件通常包含：

```text
plugins/demo/
  plugin.json
  main.js 或 main.py
```

### plugin.json 示例

```json
{
  "name": "示例插件",
  "version": "1.0.0",
  "runtime": "nodejs",
  "entry": "main.js",
  "platforms": ["qq", "telegram", "qq_office"],
  "allowed_adapter_ids": [],
  "priority": 0,
  "trigger": "^你好$",
  "enabled": true,
  "dependencies": {},
  "user_config_schema": [],
  "user_config": {},
  "access_control": {
    "inherit_system": true,
    "whitelist_groups": [],
    "blocked_groups": [],
    "whitelist_user_ids": [],
    "blocked_user_ids": []
  }
}
```

常用字段说明：

| 字段 | 说明 |
| --- | --- |
| `name` | 插件名称 |
| `version` | 插件版本 |
| `runtime` | `nodejs` 或 `python` |
| `entry` | 插件入口文件 |
| `platforms` | 允许触发的平台列表 |
| `allowed_adapter_ids` | 允许触发的机器人实例 ID 列表，空数组表示不限制 |
| `priority` | 优先级，匹配多个插件时数字越大越优先 |
| `trigger` | 正则触发表达式 |
| `enabled` | 是否启用 |
| `dependencies` | 插件依赖，由依赖管理器安装到运行时目录 |
| `user_config_schema` | 后台配置表单 schema |
| `user_config` | 用户配置值 |
| `access_control` | 插件访问控制 |

### Node.js 插件示例

```js
const { runDirect } = require('./allbot_direct')

runDirect(async (ctx) => {
  await ctx.reply(`你好，${ctx.userId}`)
})
```

如果 SDK 文件不在插件目录中，可从 `sdk/nodejs/allbot_direct.js` 复制或按项目插件模板引用。

### Python 插件示例

```python
from allbot_direct import run_direct

async def handle(ctx):
    await ctx.reply(f"你好，{ctx.user_id}")

run_direct(handle)
```

如果 SDK 文件不在插件目录中，可从 `sdk/python/allbot_direct.py` 复制或按项目插件模板引用。

## Direct SDK 常用能力

### 消息上下文

Node.js：

```js
ctx.pluginId
ctx.platform
ctx.adapterId
ctx.userId
ctx.groupId
ctx.content
ctx.metadata
ctx.userConfig
ctx.isAdmin()
```

Python：

```python
ctx.plugin_id
ctx.platform
ctx.adapter_id
ctx.user_id
ctx.group_id
ctx.content
ctx.metadata
ctx.user_config
ctx.is_admin()
```

### 回复与发送消息

Node.js：

```js
await ctx.reply('文本')
await ctx.sendMessage({ platform: 'qq', userId: '10001', text: '主动消息' })
await ctx.sendImage('https://example.com/a.png')
await ctx.sendFile('/path/to/file')
```

Python：

```python
await ctx.reply('文本')
await ctx.send_message(platform='qq', user_id='10001', text='主动消息')
await ctx.send_image('https://example.com/a.png')
await ctx.send_file('/path/to/file')
```

### 连续对话

```js
await ctx.reply('请输入内容：')
const text = await ctx.listen(60)
```

```python
await ctx.reply('请输入内容：')
text = await ctx.listen(60)
```

### 插件数据库

Node.js：

```js
await ctx.db.createTable('items', [{ name: 'name', type: 'TEXT' }])
const id = await ctx.db.insert('items', { name: 'demo' })
const rows = await ctx.db.query('items', { page: 1, size: 20 })
```

Python：

```python
await ctx.db.create_table('items', [{'name': 'name', 'type': 'TEXT'}])
id = await ctx.db.insert('items', {'name': 'demo'})
rows = await ctx.db.query('items', page=1, size=20)
```

### 定时任务

Node.js：

```js
await ctx.setScheduledTask({
  taskKey: 'daily-run',
  name: '每日运行',
  cron: '0 8 * * *',
  platform: ctx.platform,
  adapterId: ctx.adapterId,
  userId: ctx.userId,
  content: '你好'
})
```

Python：

```python
await ctx.set_scheduled_task(
    task_key='daily-run',
    name='每日运行',
    cron='0 8 * * *',
    platform=ctx.platform,
    adapter_id=ctx.adapter_id,
    user_id=ctx.user_id,
    content='你好',
)
```

说明：

- `task_key` / `taskKey` 用于区分同一插件声明的任务。
- 插件声明任务时，如果同一 `plugin_id + task_key` 已存在，后端会返回已有任务，不覆盖后台手动修改过的配置。
- `@once` 表示只允许手动执行，不参与自动 cron 调度。
- 支持 5 位或 6 位 cron，支持多行表达式。

### 获取平台管理员身份

基础 SDK 不会自动把任务身份改成管理员。插件如需管理员身份，应显式调用：

Node.js：

```js
const admins = await ctx.listPlatformAdmins()
```

Python：

```python
admins = await ctx.list_platform_admins()
```

返回项包含：

```json
{
  "platform": "qq",
  "adapter_id": "1",
  "user_id": "10001"
}
```

只会返回当前已启动平台上的管理员身份。

### 脚本运行

Node.js：

```js
await ctx.runScript({ runtime: 'nodejs', script: 'scripts/job.js', timeout: 300, wait: true })
await ctx.runQLScript({ runtime: 'nodejs', script: 'scripts/ql.js', envName: 'JD_COOKIE', accounts: [] })
```

Python：

```python
await ctx.run_script(runtime='python', script='scripts/job.py', timeout=300, wait=True)
await ctx.run_ql_script(runtime='python', script='scripts/ql.py', env_name='JD_COOKIE', accounts=[])
```

## 账号青龙插件封装

`sdk/nodejs/account_ql_plugin.js` 与 `sdk/python/account_ql_plugin.py` 提供了面向账号类青龙插件的封装，适合需要登录账号、授权、运行脚本、查询状态、CK 检测、过期提醒的插件。

内置命令包括：

```text
前缀登录
前缀账号 / 前缀管理
前缀查询
前缀运行
前缀一键运行 / 前缀签到
前缀授权
前缀删除
前缀CK检测
前缀过期检测
```

封装会使用插件私有表保存账号，并支持积分授权。声明定时任务时会显式获取已启动平台上的管理员身份，不会依赖最后一个插件使用者。

## 定时任务

后台 `定时任务` 页面可维护以下字段：

- 任务名称、任务 Key、备注
- 是否启用、是否置顶
- cron 表达式
- 平台与机器人实例
- 伪造用户 ID、群 ID
- 消息内容

执行时，系统会构造一条 fake message 投递给正常路由流程，因此会继续走插件匹配、权限控制和指令逻辑。

插件声明任务的来源为 `plugin`；后台手动创建的任务来源为 `user`。插件任务可以在后台编辑，之后不会被同一 `task_key` 的再次声明覆盖。

## Open API

Open API 用于把插件能力暴露为 HTTP 接口。后台可创建接口并配置：

- 接口 ID、名称、路径、方法
- 是否启用
- Token
- 运行时：`nodejs` 或 `python`
- 入口脚本和代码

Token 可从以下位置传入：

- Query 参数：`token`
- Header：`X-Open-Token`
- Header：`Authorization: Bearer <token>`
- JSON body：`token`
- Form body：`token`

接口脚本可使用 Open API 形式的 SDK 上下文处理请求并返回响应。

## 数据管理

插件可以通过 SDK 创建私有表，实际表名会自动加上插件前缀。后台 `数据管理` 页面可查看数据表、搜索、分页、导入导出，并可通过 `setDataView` / `set_data_view` 为表配置展示名称、分组、说明和列信息。

## 关键字回复与内置指令

系统内置部分关键字回复，包括：

```text
myid
注册
积分充值
绑定码
绑定
groupId
用户搜索
我的平台
插件列表
system
version
更新
重启
```

其中部分指令需要平台管理员身份。平台管理员可在后台 `系统设置` 中维护。

## 依赖管理

插件可在 `plugin.json` 的 `dependencies` 中声明运行依赖。框架会使用运行时目录统一管理依赖，避免每个插件重复维护环境。

- Node.js 依赖：运行在 `runtime/` 下的 Node 环境
- Python 依赖：运行在 `runtime/` 下的 Python 环境

后台 `依赖管理` 页面可查看和操作依赖安装状态。

## 前端开发

开发管理后台：

```powershell
npm --prefix web-ui install
npm --prefix web-ui run dev
```

Vite 开发服务默认端口为 `5173`，`/api` 会代理到后端，默认目标为 `http://localhost:3000`。

构建管理后台：

```powershell
npm --prefix web-ui run build
```

## 后端开发

运行测试：

```powershell
go test ./...
```

构建 Windows 可执行文件：

```powershell
go build -o allbot.exe .
```

发布流程会在打 tag 后通过 GitHub Actions 构建：

- Windows amd64
- Linux amd64
- Linux arm64

## 项目结构

```text
allbot/
  main.go                  程序入口
  go.mod                   Go 模块定义
  core/                    后端核心模块
    adapter/               平台适配器与注册表
    config/                SQLite 数据库、系统设置、插件数据、定时任务
    deps/                  运行时依赖管理
    plugin/                插件加载、执行、脚本任务
    router/                消息路由、关键词回复、定时任务调度
    session/               listen 会话管理
    types/                 核心数据结构
    web/                   Web API 与静态资源服务
  sdk/
    nodejs/                Node.js Direct SDK 与账号青龙封装
    python/                Python Direct SDK 与账号青龙封装
  Dockerfile               Docker 镜像构建文件
  docker-compose.yml       Docker Compose 部署文件
  docker-entrypoint.sh     Docker 容器入口脚本
  web-ui/                  Vue 管理后台源码
  web/                     管理后台构建产物
  plugins/                 插件目录
  openapis/                Open API 配置与脚本
  runtime/                 运行时依赖目录
  logs/                    日志目录
  sqls/                    数据库变更 SQL
```

## 常见问题

### 登录密码在哪里？

首次启动时会在控制台输出自动生成的管理员密码。登录后可在后台修改密码。

### Docker 部署后数据在哪里？

默认在 Docker 命名卷 `allbot_data` 中，容器内路径为 `/data`。可以通过 `docker volume inspect allbot_allbot_data` 查看宿主机实际位置；如果需要直接编辑文件，可改用 `./data:/data` 形式的 bind mount。

### 为什么修改了前端但页面没有变化？

需要重新构建 `web-ui`：

```powershell
npm --prefix web-ui run build
```

然后重启后端，或确认当前运行的二进制包含最新 `web/` 静态资源。

### 插件声明的定时任务为什么没有覆盖后台修改？

这是预期行为。同一插件的同一 `task_key` 已存在时，后端会直接返回已有任务，避免插件重载或再次触发时覆盖用户在后台手动调整过的配置。

### 定时任务为什么执行失败？

检查以下内容：

- cron 表达式是否合法。
- 平台是否有已启用并正在运行的机器人实例。
- `adapter_id` 是否属于所选平台。
- `user_id` 是否填写。
- 消息内容是否能匹配目标插件触发规则。
- 目标插件的访问控制是否允许该 fake message。

### 如何让账号类插件自动声明管理员定时任务？

使用 `account_ql_plugin` 封装。该封装会显式调用 `listPlatformAdmins` / `list_platform_admins` 获取已启动平台上的管理员身份，再声明定时任务。基础 `setScheduledTask` 不会自动改写身份。
