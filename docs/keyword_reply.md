# 内置指令开发说明

本文记录 `core/router/keyword_reply.go` 中内置指令的接入方式，避免新增指令时重复搜索。

## 相关文件

- `core/config/database.go`：`ensureBuiltinKeywordReplies` 负责把内置指令写入 `keyword_replies` 表。
- `core/router/keyword_reply.go`：`KeywordReplyManager` 负责匹配消息并分发到具体内置指令。
- `core/config/keyword_reply_test.go`：验证内置指令默认种子数据。
- `core/router/keyword_reply_test.go`：验证内置指令运行结果。

## 执行流程

1. `KeywordReplyManager.Handle` 调用 `database.ListKeywordReplies()` 读取启用的关键字回复。
2. `match` 判断消息内容是否命中关键字。
3. 如果 `AdminOnly` 为 true，非管理员消息会被消费但不回复。
4. 内置指令进入 `replyBuiltin`，普通回复按 `ReplyType` 发送文本、图片或文件。

## 新增内置指令步骤

### 1. 注册种子数据

在 `core/config/database.go` 的 `ensureBuiltinKeywordReplies` 中增加一项：

```go
{"指令名", "指令描述", true},
```

第三个字段是 `adminOnly`：

- `true`：仅平台管理员可用。
- `false`：普通用户可用。

默认会使用：

- `match_type = exact`
- `reply_type = builtin`
- `enabled = 1`
- `pinned = 1`
- `builtin = 1`

### 2. 支持带参数匹配

如果指令需要参数，在 `KeywordReplyManager.match` 中增加前缀匹配：

```go
if item.Builtin && item.Keyword == "指令名" {
    return content == "指令名" || strings.HasPrefix(content, "指令名 ")
}
```

不需要参数的内置指令不用改 `match`，默认精确匹配。

### 3. 增加分发

在 `replyBuiltin` 的 `switch keyword` 中增加分支：

```go
case "指令名":
    return m.sendText(adp, target, msg, m.xxx(msg))
```

如果指令需要异步对话、发图或调用适配器能力，可以像 `replyRechargePoints`、`replyPluginList` 一样单独返回 `error`。

### 4. 实现指令函数

推荐把业务逻辑写成 `KeywordReplyManager` 方法，便于复用数据库、管理员判断和回复格式：

```go
func (m *KeywordReplyManager) xxx(msg *types.Message) string {
    args := strings.Fields(strings.TrimSpace(strings.TrimPrefix(msg.Content, "指令名")))
    if len(args) != 1 {
        return "用法：指令名 <参数>"
    }
    return "执行结果"
}
```

常用工具方法：

- `m.pointsUnit()`：读取积分单位，默认“积分”。
- `m.database.GetUserAccount(platform, userID)`：查询平台账号。
- `m.database.EnsureUserAccount(platform, userID)`：不存在则注册平台账号。
- `m.database.ListUserAccountsByUnionID(unionID)`：查询 union_id 关联的全部平台账号。
- `m.database.GetUserPoints(unionID)`：查询 union_id 积分。
- `m.database.UserUnionExists(unionID)`：判断 union_id 是否存在。
- `m.sendText(adp, target, msg, text)`：按适配器规则格式化并发送文本。

## 测试要求

新增内置指令至少补两类测试：

1. `core/config/keyword_reply_test.go`
   - 确认指令被写入。
   - 确认 `Builtin`、`AdminOnly`、`Pinned`、`Enabled`、`MatchType`、`ReplyType` 符合预期。
2. `core/router/keyword_reply_test.go`
   - 使用 `newKeywordReplyTestManager` 创建内存数据库和 fake adapter。
   - 调用 `manager.Handle(&types.Message{...})`。
   - 检查 `fake.sentMessages()` 的回复内容。

管理员指令还要验证非管理员场景：命中后返回 `true`，但不发送回复。

## 当前内置指令示例

- `myid`：展示当前用户平台、用户号、UnionID 和积分。
- `注册`：注册当前平台账号。
- `积分充值`：普通用户自助充值；管理员可给指定 union_id 或 `平台:userId` 加积分。
- `用户搜索 <unionId>` / `用户搜索 <平台>:<用户号>` / `用户搜索 <平台> <用户号>`：管理员查询用户关联的平台账号、用户号和积分。
- `绑定码`：私聊生成跨平台绑定码。
- `绑定 <绑定码>`：私聊绑定到已有 union_id。
- `groupId`：群聊返回群 ID。
- `插件列表`：管理员交互式管理插件。
- `system`：管理员查看系统信息。
- `version`：查看版本和更新信息。
- `更新`：管理员触发一键升级。
- `重启`：管理员触发进程重启。
