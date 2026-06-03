# AllBot 项目安全审计报告

审计方式：使用 subagent 并行只读审计，未调用 Codex，未修改业务代码。
审计范围：后端 Go、前端 Web、插件/SDK/运行时、配置/依赖/备份/敏感文件。
审计日期：2026-05-18

## 总体结论

当前项目的核心风险集中在：

1. 默认弱口令 + 明文凭据存储
2. `config.db`、备份 zip、构建产物等敏感文件混入仓库
3. 管理端权限过大，登录后几乎等于 root 权限
4. OpenAPI 可在空 token 情况下公开调用
5. 插件、脚本、依赖安装能力存在高危代码执行面
6. 插件数据库查询存在 SQL 注入风险
7. 前端 token 存在 `localStorage`，且权限控制弱

如果这是生产或准生产环境，建议优先做密钥轮换和仓库清理。

---

## 高风险

### 1. 默认管理员账号密码暴露，且密码明文存储

#### 证据

- `main.go:79-93`
- `core/web/server.go:97-120`
- `core/config/system_settings.go:155-167`
- `core/config/database.go:273-285`
- `README.md:52-55`
- `QUICKSTART.md:15-18`
- `web-ui/src/views/Login.vue:49-50,70-73`

默认账号密码为：

```text
admin / admin123
```

前端登录页还直接展示并预填了默认账号密码。

#### 影响

- 首次部署或配置丢失时，攻击者可直接尝试默认口令。
- 管理员密码写入 `system_settings` 时是明文。
- 数据库或备份泄露后，管理员口令直接泄露。

#### 修复建议

- 移除默认口令回退逻辑。
- 首次启动强制设置管理员密码。
- 管理员密码改为哈希加盐存储。
- 文档和前端不要展示默认密码。
- 登录接口增加失败次数限制、限速和审计。

---

### 2. `config.db` 含真实敏感数据，且位于仓库根目录

#### 证据

- `config.db`
- `core/config/models.go:17-30`
- `core/config/database.go:45-228`
- `core/config/plugin_account.go:11-26,43-78,101-160`
- `core/config/script_run_log.go:10-23,67-143`

审计发现 `config.db` 是真实 SQLite 数据库，不只是配置模板。里面可能包含：

- QQ `access_token`
- Telegram `bot_token`
- 插件账号环境值
- 外部 API key
- 脚本运行输出和错误
- 用户绑定码
- 消息统计数据

#### 影响

拿到 `config.db` 就可能直接拿到机器人平台、第三方账号、插件账号和历史运行数据。

#### 修复建议

- 立即把 `config.db` 移出仓库。
- 已暴露的 token、API key、cookie、bot token 全部轮换。
- `.gitignore` 加入：

```gitignore
config.db
*.db
*.sqlite
*.sqlite3
```

- 生产环境密钥迁移到环境变量或专门 secret 存储。
- 数据库备份单独加密保存，不进入源码仓库。

---

### 3. 备份 zip、二进制、缓存和工作树混入仓库

#### 证据

- `allbot-linux-amd64`
- `.gocache/`
- `.claude/worktrees/shimmering-hugging-axolotl`
- `plugins/test2test.backup.zip`
- `plugins/test3.backup.zip`
- `plugins/test_plugin.backup.zip`
- `plugins/fxsh.backup-accountql-20260517-232741.zip`
- `plugins/xyyx.backup-accountql-20260517-232741.zip`

#### 影响

这些文件可能保留：

- 已删除的插件源码
- 旧 token
- cookie
- 账号配置
- 数据库副本
- 编译产物里的路径或调试信息

删除源码里的密钥但不清理备份包，仍然会造成泄露。

#### 修复建议

- 删除仓库内备份 zip、二进制、缓存、临时 worktree。
- 将这些路径加入 `.gitignore`。
- 对备份包中可能出现过的密钥全部轮换。
- 发布包、源码仓库、运行数据必须分离。

---

### 4. 管理端权限过大，登录后几乎拥有全部危险能力

#### 证据

- `core/web/server.go:46-79`
- `core/web/server.go:123-175`
- `core/web/server.go:181-240`
- `core/web/server.go:288-405`
- `core/web/server.go:453-599`
- `core/web/sdk.go:11-63`
- `core/web/open_api.go:115-234,419-465`
- `core/config/data_admin.go:72-305`
- `web-ui/src/router/index.js:4-46`
- `web-ui/src/views/Layout.vue:6-76`

管理端能力包括：

- 修改插件代码
- 修改 SDK 文件
- 修改 OpenAPI 脚本
- 修改数据库表结构和数据
- 安装/卸载 Python/Node 依赖
- 管理脚本任务
- 管理权限和系统设置

前端路由只判断是否登录，没有角色、权限或 scope。

#### 影响

一个管理员 token 泄露后，攻击者基本可以接管整个系统。

#### 修复建议

- 拆分权限域：
  - 只读配置
  - 插件管理
  - 数据库管理
  - 依赖管理
  - OpenAPI 管理
  - 脚本任务管理
  - 系统设置
- 后端必须做强制 RBAC/ABAC。
- 前端只做展示控制，不能作为权限边界。
- 高危操作加二次确认和审计日志。

---

### 5. OpenAPI 空 token 时可成为公开执行入口

#### 证据

- `core/web/plugin_create.go:101-103`
- `core/web/open_api.go:42-88`
- `core/web/open_api.go:95-109`
- `core/web/open_api.go:187-234`
- `core/web/open_api.go:673-760`
- `core/web/server.go:47-50`
- `core/web/server.go:890-901`
- `core/plugin/manager.go:537-663`

关键问题：

- `/api/open/` 不走管理端登录鉴权。
- endpoint token 为空时会跳过 token 校验。
- 新建插件时 OpenAPI token 默认可能为空。
- OpenAPI 可触发插件动作，例如数据库操作、发送消息等。
- token 还允许从 query 传入。

#### 影响

如果某个 OpenAPI endpoint 被启用但没有 token，外部请求可以直接调用插件能力。

#### 修复建议

- 启用 OpenAPI 时强制 token 非空。
- token 只允许从 header 传入，不允许 query。
- 空 token endpoint 直接拒绝启动或拒绝保存。
- 高危 action 做 allowlist。
- OpenAPI 增加请求限速和审计。

---

### 6. 插件入口路径和 `plugin.json` 信任边界不足

#### 证据

- `core/web/server.go:606-636`
- `core/web/server.go:642-780`
- `core/web/server.go:869-875`
- `core/plugin/manager.go:955-980`
- `core/plugin/manager.go:690-704`

问题点：

- 后台文件管理允许编辑 `.json`。
- `plugin.json` 中的 `entry` 会被插件管理器信任。
- `handlePluginCode` 根据 `pluginInfo.Entry` 拼接路径读取/写入代码。
- 解释器启动时也会使用 `plugin.Entry`。

#### 影响

如果 `plugin.json` 被恶意修改，可能导致：

- 读取插件目录外文件
- 写入非预期文件
- 执行非预期 Python/Node 脚本

#### 修复建议

- `entry` 必须做严格路径规范化。
- 禁止绝对路径、`..`、跨目录路径。
- `plugin.json` 不应作为普通文本文件随意在线编辑。
- 插件启动前检查入口文件必须位于插件根目录内。

---

### 7. 插件/脚本存在任意 JS 执行和远程脚本供应链风险

#### 证据

- `plugins/custom_reply/main.js:106-129,263-271`
- `plugins/xyyx/scripts/wqwl_new_星韵优选.js:37-59,95-100`
- `plugins/xyyx/scripts/wqwl_require.js:1-6,142-160,459-511`

发现：

- 自定义回复插件使用 `new Function(...)` 执行动态 JS。
- 部分脚本会从远程下载 JS，再本地 `require`。
- 插件运行环境没有看到强沙箱隔离。

#### 影响

这属于强 RCE 面：

- 恶意回复规则可执行代码。
- 远程脚本源被污染后，本地直接执行。
- 插件进程可接触本地文件、网络、环境变量和数据库能力。

#### 修复建议

- 禁用 `new Function` / `eval` 类动态执行。
- 远程脚本不要运行时下载执行。
- 需要外部脚本时使用固定版本、哈希校验、人工审核。
- 第三方插件按不可信代码处理，隔离运行权限。

---

### 8. 运行时依赖安装接口存在供应链 RCE 风险

#### 证据

- `core/web/server.go:550-603`
- `core/deps/manager.go:84-185,340-387`
- `core/plugin/manager.go:1004-1023`

系统支持：

- 后台安装/卸载 Python 包
- 后台安装/卸载 Node 包
- 插件加载时自动安装 `plugin.json` 中声明的依赖

#### 影响

恶意 npm/pip 包可通过安装脚本执行代码。
如果插件来源不可信，加载插件就可能触发供应链攻击。

#### 修复建议

- 生产环境禁用运行时安装依赖。
- 依赖安装和服务运行分离。
- 只允许白名单包名和固定版本。
- npm/pip 使用锁文件、哈希校验和内部镜像。
- 高危依赖操作加审计。

---

### 9. 插件数据库查询存在 SQL 注入风险

#### 证据

- `core/config/data_admin.go:507-551`
- `core/config/data_admin.go:740-767`
- `sdk/nodejs/allbot_direct.js:361-375`
- `sdk/python/allbot_direct.py:333-347`

问题点：

- 插件 SDK 允许传入 `where` / `order`。
- 后端会把 `where` 片段拼入 SQL。
- 当前只过滤了 `;`、`--`、`/*` 等粗粒度字符。

#### 影响

如果插件把用户输入拼进 `where`，可能造成：

- SQL 注入
- 越权查询
- 条件绕过
- 数据探测

#### 修复建议

- 不要接受原始 SQL 片段。
- 改为结构化过滤器：

```json
{
  "field": "userId",
  "op": "=",
  "value": "123"
}
```

- 字段名、操作符使用白名单。
- 值全部使用参数化查询。

---

### 10. 前端 token 存在 `localStorage`

#### 证据

- `web-ui/src/stores/auth.js:6-17,20-28`
- `web-ui/src/router/index.js:41-46`
- `web-ui/src/utils/request.js:12-18,29-38`

#### 影响

如果发生 XSS、浏览器插件劫持、同机用户读取浏览器数据，管理员 token 会直接泄露。

此外，前端路由守卫只看本地 token 是否存在，手动写入任意 token 也能进入页面，直到后端 401 才被踢出。

#### 修复建议

- 不要把长期 bearer token 存 `localStorage`。
- 改用短生命周期 token。
- 页面启动时向后端校验 session。
- 最好使用 httpOnly cookie + CSRF 防护，或短 token + refresh token 机制。
- 后端必须做权限判断，不能信任前端状态。

---

## 中风险

### 1. 管理端 CORS 过宽

#### 证据

- `core/web/server.go:890-949`

响应中设置：

```http
Access-Control-Allow-Origin: *
```

并允许：

```http
Authorization
X-Open-Token
```

#### 影响

这不是传统 cookie-CSRF，但会放大 token 泄露后的利用面。
恶意网页更容易跨站调用 API。

#### 修复建议

- CORS origin 改成白名单。
- 管理后台只允许可信前端域名。
- 如果未来改 cookie 会话，必须加 CSRF token。

---

### 2. OpenAPI 请求体读取无上限，存在 DoS 风险

#### 证据

- `core/web/open_api.go:51-58`

当前逻辑在 endpoint 匹配和 token 校验前就执行：

```go
io.ReadAll(r.Body)
```

#### 影响

攻击者可发送大 body，即使 token 错误，也会先消耗内存。

#### 修复建议

- 使用 `http.MaxBytesReader`。
- 先做路径匹配和鉴权，再读取 body。
- 限制单请求大小。

---

### 3. 插件 stdout/stderr 和脚本输出缓存无硬上限

#### 证据

- `core/plugin/manager.go:329-334`
- `core/plugin/manager.go:837-920`
- `core/config/script_run_log.go:67-143`

#### 影响

插件或脚本输出大量内容时，可能造成：

- 内存增长
- 数据库膨胀
- I/O 压力
- 日志泄露

#### 修复建议

- 输出做最大长度限制。
- 超出后截断。
- 数据库只保存摘要或最近 N KB。
- 详细日志写入受控日志系统。

---

### 4. 日志和脚本输出可能泄露敏感信息

#### 证据

- `core/web/logs.go:57-109,141-190`
- `core/deps/manager.go:114-117,169-175,355-384`
- `core/web/script_task.go:79-89`
- `web-ui/src/views/Logs.vue:33-46,73-79`
- `web-ui/src/views/ScriptTasks.vue:86-102,211-214`

#### 影响

日志可能包含：

- token
- cookie
- 请求体
- 第三方 API 响应
- 命令执行输出
- 堆栈路径

前端会直接展示这些内容。

#### 修复建议

- 后端统一敏感字段脱敏。
- 前端只展示摘要。
- 脚本输出做截断和敏感词过滤。
- 清空日志时同时支持磁盘日志清理。

---

### 5. 文件写入权限偏宽

#### 证据

- `core/web/server.go:530-536`
- `core/web/server.go:631-636`
- `core/web/server.go:719-724`
- `core/web/server.go:772-777`
- `core/web/open_api.go:453-463`
- `core/web/logs.go:86-131`

多个文件使用 `0644` 写入。

#### 影响

共享主机或备份泄露时，插件代码、配置、日志更容易被旁路读取。

#### 修复建议

- 敏感文件使用 `0600`。
- 日志目录和配置目录权限收紧。
- 不同类型文件分目录隔离。

---

### 6. 定时任务 cron 逐秒扫描，存在 CPU DoS 风险

#### 证据

- `core/router/scheduler.go:134-148`
- `core/web/scheduled_task.go:31-34,63-66`
- `core/router/router.go:427-430`

#### 影响

异常 cron 表达式可能导致大量循环扫描。
如果攻击者能反复提交任务，可能造成 CPU 压力。

#### 修复建议

- 使用成熟 cron 解析库。
- 限制扫描次数和时间。
- 保存任务时做更严格表达式校验。

---

### 7. 插件会话 key 未包含 pluginID，存在跨插件串扰

#### 证据

- `core/session/manager.go:31-86`
- `core/router/router.go:132-145`

当前会话 key 使用：

```text
userID:groupID
```

没有包含 `pluginID`。

#### 影响

同一用户/群组下，不同插件的等待会话可能互相覆盖或抢消息。

#### 修复建议

- session key 改为：

```text
pluginID:userID:groupID
```

- 限制同一用户并发等待会话数量。

---

### 8. Telegram bot token 在前端列表摘要中明文展示

#### 证据

- `web-ui/src/views/Adapters.vue:184-189`

#### 影响

能访问平台机器人页面的用户可直接看到 bot token。

#### 修复建议

- 列表只展示“已设置/未设置”。
- 敏感字段统一脱敏。
- 查看完整 token 需要单独权限和二次确认。

---

## 低风险 / 信息性风险

### 1. `web/index.html` 未看到 CSP

#### 证据

- `web/index.html:1-15`

#### 影响

如果未来出现 XSS，没有 CSP 作为额外防线。

#### 修复建议

服务端增加 CSP 响应头。

---

### 2. 文档暴露默认部署信息

#### 证据

- `README.md:52-60`
- `README.md:268-274`
- `QUICKSTART.md:33-40`
- `QUICKSTART.md:51-57`

#### 影响

方便攻击者枚举默认端口、默认账号、接入方式。

#### 修复建议

公共文档移除默认口令和具体敏感部署细节。

---

### 3. 客户端 IP 信任转发头

#### 证据

- `core/web/open_api.go:828-839`

#### 影响

如果没有可信反向代理校验，`X-Forwarded-For` 可被伪造，审计日志来源 IP 不可信。

#### 修复建议

只在请求来自可信代理时信任转发头。

---

## 已看到的正面安全实践

项目里也有一些好的点：

1. 多数 SQL 使用参数化查询。
   - `core/config/plugin_account.go`
   - `core/config/plugin_authorization.go`
   - `core/config/scheduled_task.go`

2. 数据库表名、字段名有正则校验。
   - `core/config/data_admin.go:70-101`

3. 部分路径有 root-bound 检查。
   - `core/web/server.go:850-867`
   - `core/web/sdk.go:73-90`
   - `core/web/open_api.go:617-660`
   - `core/plugin/manager.go:936-952`

4. OpenAPI 管理接口不直接返回 token，只返回 `has_token`。
   - `core/web/open_api.go:507-519`

5. OpenAPI token 比较使用了常量时间比较。
   - `core/web/open_api.go:746-825`

6. 适配器配置返回前有敏感字段脱敏。
   - `core/web/server.go:377-380,447-448`
   - `core/utils/security.go:11-89`

7. 外部命令多数使用 `exec.Command`，未看到明显 shell 字符串拼接。
   - `core/deps/manager.go`
   - `core/plugin/manager.go`

8. Telegram 输出做了部分 HTML 转义。
   - `core/router/router.go:544-573`

9. 前端没有发现明显 `v-html`、`innerHTML`、`eval`、`document.write` 直接 DOM 注入点。

---

## 建议修复优先级

### 第一优先级：立即处理

1. 轮换所有已进入 `config.db`、备份 zip、日志、插件配置里的 token、API key、cookie。
2. 删除仓库中的：
   - `config.db`
   - `*.backup.zip`
   - `allbot-linux-amd64`
   - `.gocache/`
   - `.claude/worktrees/`
3. 移除默认管理员密码。
4. 管理员密码改为哈希存储。
5. OpenAPI 强制 token，禁止空 token endpoint 启用。
6. 禁止 query 传 OpenAPI token。
7. 修复插件数据库 raw `where` SQL 拼接。

### 第二优先级：短期修复

1. 管理端引入权限分级。
2. 禁止运行时随意安装 npm/pip 依赖。
3. 插件 `entry` 路径做强校验。
4. 禁止在线编辑敏感文件，例如 `.env`、密钥文件、证书文件。
5. 前端 token 不再长期存 `localStorage`。
6. Telegram bot token、插件配置、日志展示统一脱敏。

### 第三优先级：持续加固

1. 增加 OpenAPI、登录、绑定码接口限速。
2. 增加请求体、脚本输出、日志大小限制。
3. 增加 CSP。
4. 管理端 CORS 改白名单。
5. 引入依赖审计和锁定策略。
6. 对第三方插件建立沙箱或最小权限运行模型。

---

## 最短整改路线

如果只先做一轮最小整改，建议按这个顺序：

1. 清理仓库敏感文件并轮换密钥
2. 移除默认口令，管理员密码哈希化
3. OpenAPI 禁止空 token
4. 修复插件 SQL 查询接口
5. 限制插件/脚本/依赖安装能力
6. 管理端做权限拆分和日志脱敏

这几项处理完，整体风险会明显下降。
