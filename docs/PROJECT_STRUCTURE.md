# AllBot 项目框架说明

本文用于快速定位项目模块、理解消息处理主链路，以及说明后续如何新增内置指令和常见功能。

## 一、项目总体框架

```text
allbot/
├── main.go                         # 程序入口
├── core/                           # Go 后端核心
│   ├── adapter/                    # 各平台适配器
│   │   ├── dingtalk/               # 钉钉
│   │   ├── feishu/                 # 飞书
│   │   ├── qq/
│   │   ├── qq_office/              # QQ 官方
│   │   ├── telegram/
│   │   └── wechat_official/
│   ├── router/                     # 消息路由、插件触发、关键字回复
│   │   ├── router.go               # 主消息路由
│   │   ├── keyword_reply.go        # 关键字回复编排层
│   │   └── builtin/                # 内置指令业务逻辑
│   ├── config/                     # SQLite 数据库、配置、迁移、数据模型
│   ├── plugin/                     # 插件管理、插件进程
│   ├── payment/                    # 支付、积分、订单
│   ├── updater/                    # 一键更新
│   ├── backup/                     # 备份恢复
│   ├── deps/                       # Node/Python 运行时依赖管理
│   ├── web/                        # 后端 Web API
│   ├── session/                    # 连续对话/session
│   ├── types/                      # 公共类型
│   └── utils/                      # 通用工具
├── web-ui/                         # Vue 前端源码
│   ├── src/
│   │   ├── api/                    # 前端 API 封装
│   │   ├── router/                 # 前端路由
│   │   ├── views/                  # 页面
│   │   ├── components/             # 组件
│   │   ├── stores/                 # Pinia 状态
│   │   └── styles/                 # 样式
│   └── FRONTEND_CHANGES.md         # 前端源码修改记录
├── web/                            # 前端构建产物，被 Go 嵌入
├── sdk/                            # 插件 SDK 示例
│   ├── nodejs/
│   └── python/
├── sqls/                           # 数据库迁移 SQL
└── docs/                           # 文档
```

## 二、消息处理主链路

```text
平台适配器收到消息
        ↓
core/router/router.go
        ↓
系统访问控制 / session 拦截
        ↓
关键字回复 KeywordReplyManager
        ↓
普通关键字回复 或 内置指令
        ↓
插件匹配与执行
```

主要位置：

- 主路由：`core/router/router.go`
- 关键字回复入口：`core/router/keyword_reply.go`
- 内置指令业务：`core/router/builtin/`

`keyword_reply.go` 只负责编排：

- 加载数据库关键字
- 判断是否匹配
- 判断管理员权限
- 解析 adapter 和回复目标
- 把内置指令分发给 `builtin`

真正的内置指令逻辑放在 `core/router/builtin/`。

## 三、内置指令框架

### 目录

```text
core/router/builtin/
├── context.go          # 内置指令运行上下文
├── map.go              # 指令注册、匹配、分发
├── identity.go         # myid、注册、绑定码、绑定、groupid
├── user_search.go      # 用户搜索
├── recharge.go         # 积分充值
├── plugin_list.go      # 插件列表
├── version.go          # version
├── update.go           # 更新
├── restart.go          # 重启
├── system.go           # system 系统信息
├── system_info_*.go    # 不同系统的内存/磁盘采集
```

### 已有内置指令

| 指令 | 文件 | 说明 |
|---|---|---|
| `myid` | `builtin/identity.go` | 查看当前用户身份、UnionID、积分 |
| `注册` | `builtin/identity.go` | 注册当前平台账号 |
| `绑定码` | `builtin/identity.go` | 私聊获取跨平台绑定码 |
| `绑定` | `builtin/identity.go` | 使用绑定码绑定账号 |
| `groupId` / `groupid` | `builtin/identity.go` | 返回当前群 ID |
| `用户搜索` | `builtin/user_search.go` | 管理员查询用户 UnionID、积分、关联账号 |
| `积分充值` | `builtin/recharge.go` | 用户充值积分，管理员手动加积分 |
| `插件列表` | `builtin/plugin_list.go` | 管理员交互式管理插件 |
| `version` / `v` | `builtin/version.go` | 查看当前版本和最新版本 |
| `更新` | `builtin/update.go` | 管理员触发一键更新 |
| `重启` | `builtin/restart.go` | 管理员触发进程重启 |
| `system` | `builtin/system.go` | 管理员查看系统信息 |

## 四、怎么新增一个内置指令

假设要新增指令：`帮助`。

### 1. 在 `core/router/builtin/` 新建或选择分类文件

如果是简单身份类，可以放到：

```text
core/router/builtin/identity.go
```

如果是新业务，建议新建：

```text
core/router/builtin/help.go
```

### 2. 写 handler

统一写法：

```go
package builtin

func replyHelp(ctx *Context) error {
	return ctx.SendText("这里是帮助内容")
}
```

常用能力都从 `ctx` 取：

```go
ctx.Database        // 数据库
ctx.Message         // 当前消息
ctx.Target          // 回复目标
ctx.SendText(...)   // 文本回复
ctx.IsAdmin()       // 是否管理员
ctx.ListenText(...) // 连续对话输入
ctx.ReplyButtons(...) // 按钮回复
ctx.SendImage(...)  // 发送图片
```

### 3. 到 `map.go` 注册指令

在 `commands` 里加：

```go
"帮助": {Name: "帮助", Handler: replyHelp},
```

### 4. 如果需要特殊匹配，在 `Match` 里补规则

普通精确匹配不用加特殊规则。

如果指令支持参数，例如：

```text
帮助
帮助 插件
帮助 支付
```

则需要在 `builtin.Match` 里加类似规则：

```go
if strings.EqualFold(item.Keyword, "帮助") {
	return content == "帮助" || strings.HasPrefix(content, "帮助 ")
}
```

已有特殊匹配包括：

- `myid` 大小写不敏感
- `groupid` 大小写不敏感
- `version` 大小写不敏感，并兼容数据库正则
- `绑定 xxx`
- `积分充值 xxx`
- `用户搜索 xxx`

### 5. 到数据库种子里注册

内置关键字种子在：

```text
core/config/database.go
```

找到 `ensureBuiltinKeywordReplies`，新增一项：

```go
{keyword: "帮助", content: "帮助", matchType: "exact", description: "查看帮助信息", adminOnly: false},
```

字段含义：

| 字段 | 说明 |
|---|---|
| `keyword` | 数据库里展示的关键字 |
| `content` | 分发到 builtin 的指令名 |
| `matchType` | `exact` 或 `regex` |
| `description` | 后台说明 |
| `adminOnly` | 是否管理员专用 |

如果是管理员指令：

```go
adminOnly: true
```

注意：管理员指令如果非管理员触发，当前行为是“消费消息但不回复”，不要随便改。

### 6. 写测试

优先加到：

```text
core/router/keyword_reply_test.go
```

如果只是检查内置关键字是否种入数据库，加到：

```text
core/config/keyword_reply_test.go
```

### 7. 验证

只改后端时：

```bash
go test ./core/router
go test ./core/config
go test ./...
go build -o allbot.exe .
```

不需要构建前端。

## 五、内置指令开发规范

### 1. `builtin` 不能反向 import `router`

这是为了避免包循环。

正确方式：

```text
router → builtin
builtin → config/payment/plugin/types/updater 等底层包
```

不要这样：

```text
builtin → router
```

如果内置指令需要 router 的能力，通过 `Context` 回调注入。

例如：

- 发送文本：`ctx.SendText`
- 发送按钮：`ctx.ReplyButtons`
- 发送图片：`ctx.SendImage`
- 连续对话：`ctx.ListenText`
- 重启锁：`ctx.ReserveRestart` / `ctx.ReleaseRestart`

### 2. 内置指令统一返回 `error`

推荐：

```go
func replyXXX(ctx *Context) error {
	return ctx.SendText("回复内容")
}
```

异步场景：

```go
go func() {
	_ = ctx.SendText("异步回复")
}()
return nil
```

### 3. 管理员指令的权限

数据库层已有 `adminOnly`，`KeywordReplyManager.Handle` 会提前拦截。

但部分指令内部仍保留二次判断，比如：

```go
if !ctx.IsAdmin() {
	return ctx.SendText("仅平台管理员可使用xxx")
}
```

目前保持现有行为即可，新指令建议：

- 数据库种子设置 `adminOnly: true`
- handler 内可以再二次检查，特别是更新、插件管理、系统操作类

### 4. 回复文本不要在 builtin 里直接操作 adapter

不要这样：

```go
adp.SendMessage(...)
```

应该：

```go
ctx.SendText(...)
```

按钮回复：

```go
ctx.ReplyButtons(text, buttons)
```

图片：

```go
ctx.SendImage(url)
```

这样可以继续复用 QQ 官方、Telegram 的按钮链路和 fallback 逻辑。

## 六、普通关键字回复和内置指令的区别

### 普通关键字回复

数据来自数据库 `keyword_replies`：

- `reply_type = text/image/audio`
- `builtin = false`

处理位置：

```text
core/router/keyword_reply.go
```

逻辑：

- 文本：`SendMessage`
- 图片：`SendImage`
- 音频/文件：`SendFile`

### 内置指令

数据库中：

```text
reply_type = builtin
builtin = true
```

处理流程：

```text
keyword_reply.go
    ↓
builtin.Match
    ↓
builtin.Dispatch
    ↓
对应 replyXXX handler
```

## 七、插件系统框架

插件相关核心位置：

```text
core/plugin/manager.go        # 插件管理
core/router/router.go         # 插件匹配和执行
core/web/plugin_web.go        # 插件 Web 面板相关 API
sdk/nodejs/                   # Node.js 插件 SDK
sdk/python/                   # Python 插件 SDK
```

插件消息执行大致流程：

```text
消息进入 router
        ↓
关键字回复没消费
        ↓
matchPlugins 匹配插件
        ↓
执行插件
        ↓
插件返回文本/按钮/图片/支付/数据库动作等
```

插件访问控制使用：

```text
types.AccessControlConfig
config.NormalizeAccessControlConfig
```

插件列表内置指令 `插件列表` 也是直接操作插件启停和访问控制。

## 八、支付和积分框架

核心位置：

```text
core/payment/
core/config/payment*
core/web/payment*
```

常见能力：

- 支付设置：`config.PaymentSettings`
- 支付方式筛选：`payment.EnabledMethods`
- 支付服务：`payment.NewService(database)`
- 等待支付：`service.WaitPay`
- RMB 金额解析：`payment.ParseRMBToCents`
- 积分换算：`config.CalculatePointsAmount`
- 支付成功入账：`database.CreditPaymentPoints`

内置 `积分充值` 位于：

```text
core/router/builtin/recharge.go
```

它复用了支付模块，不自己实现支付逻辑。

## 九、数据库和迁移框架

核心位置：

```text
core/config/database.go       # 表结构、初始化、内置数据
core/config/*.go              # 各业务配置和查询
sqls/                         # SQL 迁移文件
```

如果新增字段/表：

1. 改 `core/config/database.go` 初始化逻辑
2. 新增迁移 SQL：

```text
sqls/014_xxx.sql
```

3. 写对应测试：

```text
core/config/xxx_test.go
```

内置关键字种子也在：

```text
core/config/database.go
```

## 十、Web 后端 API 框架

核心位置：

```text
core/web/server.go            # Web server 和路由注册
core/web/*.go                 # 各业务 API
```

常见模块：

```text
backup.go
logs.go
settings.go
statistics.go
runtime_profiles.go
script_env.go
script_task.go
plugin_web.go
feishu_callback.go
```

新增后端接口时一般流程：

1. 在 `core/web/xxx.go` 写 handler
2. 在 `server.go` 注册路由
3. 调用 `core/config` 或业务 service
4. 写 `core/web/xxx_test.go`

## 十一、前端框架

前端源码：

```text
web-ui/src/
```

常见位置：

```text
web-ui/src/api/index.js       # API 封装
web-ui/src/router/index.js    # 页面路由
web-ui/src/views/             # 页面
web-ui/src/components/        # 组件
web-ui/src/stores/            # 状态管理
web-ui/src/styles/            # 公共样式
```

构建产物：

```text
web/
```

注意：本项目 Go 程序会加载内置 web 资源。

所以如果改了：

```text
web-ui/src/
```

必须：

```bash
cd "D:/Desktop/program/java/AITest/allbot/web-ui" && npm run build
cd "D:/Desktop/program/java/AITest/allbot" && go build -o allbot.exe .
```

并且要记录到：

```text
web-ui/FRONTEND_CHANGES.md
```

如果只改 Go 后端：

- 先跑 Go 测试
- 验证通过后 `go build -o allbot.exe .`
- 不需要构建前端

如果只因为前端构建产物需要重新编译 `allbot.exe`：

- 不需要跑 Go 测试
- 直接 `go build -o allbot.exe .`

## 十二、适配器框架

适配器目录：

```text
core/adapter/
```

各平台实现：

```text
core/adapter/dingtalk/
core/adapter/feishu/
core/adapter/qq/
core/adapter/qq_office/
core/adapter/telegram/
core/adapter/wechat_official/
```

公共接口在：

```text
core/adapter/adapter.go
core/types/types.go
```

适配器通常负责：

- 平台消息接收
- 转成统一 `types.Message`
- 发送文本
- 发送图片
- 发送文件
- 解析回复目标
- 格式化回复文本
- 按钮能力或 fallback

QQ 官方和 Telegram 的按钮回复链路需要特别注意，不要绕过现有 fallback。

## 十三、测试建议

常用测试范围：

```bash
go test ./core/router
go test ./core/config
go test ./core/payment ./core/updater
go test ./core/web
go test ./...
```

改内置指令优先跑：

```bash
go test ./core/router
go test ./core/config
```

改支付相关：

```bash
go test ./core/payment ./core/router
```

改 Web API：

```bash
go test ./core/web
```

改前端：

- 先 `npm run build`
- 再 `go build -o allbot.exe .`

## 十四、以后开发时的快速定位

| 想改什么 | 去哪里 |
|---|---|
| 新增内置指令 | `core/router/builtin/` + `core/config/database.go` |
| 改普通关键字回复 | `core/router/keyword_reply.go` |
| 改消息路由 | `core/router/router.go` |
| 改插件执行 | `core/plugin/` + `core/router/router.go` |
| 改插件列表指令 | `core/router/builtin/plugin_list.go` |
| 改积分充值 | `core/router/builtin/recharge.go` |
| 改用户注册/绑定 | `core/router/builtin/identity.go` |
| 改用户搜索 | `core/router/builtin/user_search.go` |
| 改版本/更新/重启 | `builtin/version.go`、`update.go`、`restart.go` |
| 改系统信息 | `builtin/system.go`、`system_info_*.go` |
| 改数据库表 | `core/config/database.go` + `sqls/` |
| 改后台 API | `core/web/` |
| 改前端页面 | `web-ui/src/views/` |
| 改前端 API | `web-ui/src/api/index.js` |
| 改前端路由 | `web-ui/src/router/index.js` |
| 改平台适配器 | `core/adapter/<platform>/` |
| 改 SDK 示例 | `sdk/nodejs/`、`sdk/python/` |

## 十五、一句话记忆

后端主链路看 `core/router`，内置指令写 `core/router/builtin`，数据库种子看 `core/config/database.go`，前端源码看 `web-ui/src`，构建产物在 `web`，插件 SDK 在 `sdk`。
