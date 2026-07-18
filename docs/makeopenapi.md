# AllBot 开放接口编写指南

本文说明如何使用 AllBot 的 OpenAPI 脚本能力创建 HTTP 开放接口，并给出 `PushMessage` 第三方通知接入示例。

## 1. 目录结构

每个开放接口独占 `openapis/<id>/` 目录：

```text
openapis/
└── 示例接口ID/
    ├── config.json       # 接口声明
    └── 入口脚本.js       # Node.js 入口；Python 接口使用 .py
```

仓库中的最小示例位于：

```text
openapis/test/config.json
openapis/test/test.js
```

Node.js 接口推荐结构：

```text
openapis/MyEndpoint/
├── config.json
├── MyEndpoint.js
└── MyEndpoint.test.js    # 可选，但建议提供无需启动 AllBot 的自动测试
```

目录名必须与 `config.json` 的 `id` 完全一致。

## 2. config.json 配置

Node.js 接口示例：

```json
{
  "id": "MyEndpoint",
  "name": "示例开放接口",
  "path": "MyEndpoint",
  "method": "POST",
  "enabled": false,
  "token": "REPLACE_WITH_YOUR_OPENAPI_TOKEN",
  "runtime": "nodejs",
  "runtime_profile": "",
  "entry": "MyEndpoint.js",
  "description": "接口用途说明",
  "ip_whitelist": null
}
```

字段说明：

| 字段 | 含义 |
|---|---|
| `id` | 接口唯一 ID，同时也是 `openapis/<id>/` 目录名。只能包含字母、数字、横线和下划线。 |
| `name` | 后台显示名称；留空时使用 `id`。 |
| `path` | 对外路径片段。只能是单个词，只能包含字母、数字、横线和下划线，不能包含 `/` 或 `\`。 |
| `method` | HTTP 方法，支持 `GET`、`HEAD`、`POST`、`PUT`、`PATCH`、`DELETE`、`OPTIONS`。 |
| `enabled` | 是否启用。提交示例时建议设为 `false`，部署后在后台确认 Token 等配置再启用。 |
| `token` | OpenAPI 鉴权 Token，不能为空。不要提交真实生产凭证。 |
| `runtime` | 脚本运行时，支持 `nodejs`、`python`；内置接口使用 `builtin`。 |
| `runtime_profile` | 可选的运行环境 Profile；留空表示使用默认运行环境。内置接口不能设置该字段。 |
| `entry` | 相对于当前接口目录的入口文件。不能使用绝对路径，也不能通过 `..` 越出接口目录。 |
| `description` | 后台展示的接口用途和部署说明。 |
| `ip_whitelist` | 接口级 IP 白名单。`null` 或省略表示继承全局设置；可配置 IP、CIDR，或仅使用 `['*']` 允许全部来源。空数组无效。 |

实际调用地址固定为：

```text
http://服务器:端口/api/open/{path}
```

例如 `path` 为 `PushMessage`：

```text
http://127.0.0.1:8080/api/open/PushMessage
```

接口路径和 HTTP 方法必须同时匹配；接口未启用、路径错误或方法错误时不会执行脚本。

## 3. Token 鉴权

AllBot 在执行脚本前检查 Token。支持以下来源：

1. Query：`?token=实际Token`
2. 请求头：`X-Open-Token: 实际Token`
3. 请求头：`Authorization: Bearer 实际Token`
4. 请求头：`Authorization: 实际Token`
5. JSON 正文：`{ "token": "实际Token" }`
6. `application/x-www-form-urlencoded` 正文中的 `token`

如果请求同时提供多个 Token 来源，所有非空 Token 必须与接口配置完全一致。任意一个来源不一致，网关都会在脚本执行前返回 HTTP 401。

鉴权通过后，网关会先脱敏再把请求交给脚本：

- Query 中的 `token` 变为 `***`。
- `Authorization` 和 `X-Open-Token` 变为 `***`。
- JSON 或表单中的 `token` 变为 `***`。
- 原始正文中出现的已识别 Token 也会替换为 `***`。

因此脚本不应再次读取或校验真实 Token，也不应在响应、日志或错误信息中回显凭证。

`access_token` 不是 AllBot OpenAPI 支持的鉴权字段。只提供 `access_token` 且没有其他有效 Token 来源时会得到 HTTP 401，脚本没有机会兼容。

## 4. Node.js 入口协议

入口文件使用 CommonJS 导出异步 `action` 函数：

```js
module.exports.action = async function action(ctx, req, res) {
  res.status(200).json({ ok: true })
}
```

函数签名：

```text
action(ctx, req, res)
```

### 4.1 req 请求对象

常用字段：

| 字段 | 含义 |
|---|---|
| `req.method` | HTTP 方法，例如 `POST`。 |
| `req.path` / `req.raw_path` | 网关传入的接口路径信息。 |
| `req.query` | Query 参数对象。只有一个值时 SDK 会展开为字符串；同名参数重复时保留为数组。 |
| `req.headers` | 请求头对象。单值数组同样会展开；请求头名称以网关传入结果为准。 |
| `req.body` | `application/json` 通常为解析后的 JSON 对象，表单请求通常为解析后的表单对象；其他正文类型或解析失败时可能为原始字符串；没有正文时为空对象。 |

JSON 请求示例：

```http
POST /api/open/MyEndpoint?token=实际Token HTTP/1.1
Content-Type: application/json

{"message":"hello"}
```

对应脚本通常可以读取：

```js
req.method
req.query.token       // 已脱敏为 ***
req.body.message      // hello
```

Query 参数可能是字符串或数组。对于只能出现一次的参数，应显式拒绝数组，避免重复参数带来歧义。

### 4.2 res 响应对象

返回 JSON：

```js
res.status(200).json({ ok: true, data: {} })
```

返回文本：

```js
res.status(200).send('ok')
```

也可以直接使用：

```js
res.json({ ok: true })
res.send('ok')
```

未显式设置状态码时默认为 HTTP 200。脚本应完成一次明确响应；脚本未捕获的异常会由执行链路作为 OpenAPI 执行失败处理。

## 5. 主动发送消息

OpenAPI 请求不是平台收到的聊天消息，默认上下文没有可直接复用的发送目标。主动发送时应显式指定适配器实例和群组或用户目标。只要提供 `adapterId`，AllBot 就会从后台配置自动识别该实例所属平台；也可以额外提供 `platform`，用于校验平台与适配器是否匹配。

使用 `ctx.push()`：

```js
await ctx.push({
  adapterId: '3',
  groupId: 'group_openid-abc',
  content: '群聊消息'
})
```

显式指定平台的私聊：

```js
await ctx.push({
  platform: 'qq_office',
  adapterId: '3',
  userId: 'user_openid-xyz',
  content: '私聊消息'
})
```

也可以使用 `ctx.sendMessage()`：

```js
await ctx.sendMessage({
  platform: 'telegram',
  adapterId: '5',
  groupId: '-1001234567890',
  content: '消息内容'
})
```

参数含义：

| 参数 | 含义 |
|---|---|
| `platform` | 可选平台标识，例如 `qq`、`qq_office`、`telegram`、`dingtalk`；省略时根据 `adapterId` 自动识别。 |
| `adapterId` | **AllBot 后台中的适配器实例 ID，不是平台账号。** |
| `groupId` | 平台原始群组标识，可以是群号、OpenID、负数或其他非空字符串。 |
| `userId` | 平台原始用户标识，可以是 QQ 号、OpenID 或其他非空字符串。 |
| `content` / `text` | 文本消息内容。 |

发送到明确目标时不要同时传 `groupId` 和 `userId`。不要自行假定目标 ID 是正整数，应将平台提供的原始 ID 按字符串传入。`ctx.push()` 和 `ctx.sendMessage()` 最终都通过 AllBot 的消息发送链路执行，发送失败时会抛出异常，应由脚本按调用方所需协议处理。

## 6. 部署与启用

1. 将完整接口目录复制到 AllBot 工作目录的 `openapis/` 下。
2. 确认目录名、`config.json` 中的 `id` 和入口文件名一致。
3. 启动或重启 AllBot，使接口配置可被读取。
4. 在 AllBot 后台的开放接口管理中找到该接口。
5. 将占位 Token 替换为足够随机的实际 Token。
6. 按部署环境设置全局或接口级 IP 白名单。
7. 启用接口。
8. 使用 `/api/open/{path}`、配置的方法和 Token 发起测试请求。

IP 白名单注意事项：

- `ip_whitelist: null`：继承全局白名单。
- `ip_whitelist: ["192.0.2.10", "198.51.100.0/24"]`：仅允许指定地址或网段。
- `ip_whitelist: ["*"]`：允许所有来源。
- 经过反向代理时，客户端 IP 识别还取决于后台可信代理配置；不要仅设置转发请求头而忽略可信代理。

## 7. 常见问题排查

### HTTP 401：Open API token 无效

依次检查：

- 是否使用了 `token`，而不是仅使用 `access_token`。
- Token 是否与后台当前值完全一致，是否含多余空格。
- 是否同时在 Query、Header、Body 提供了 Token；如果是，所有值必须一致。
- 接口 Token 是否仍为示例占位值。

401 发生在脚本执行前，修改脚本不能兼容错误或缺失的鉴权字段。

### HTTP 403：客户端 IP 不在白名单

检查接口级 `ip_whitelist`、全局白名单、实际来源 IP和反向代理可信代理配置。接口级白名单一旦设置，就覆盖全局白名单。

### 404：接口不存在或未启用

检查：

- `enabled` 是否为 `true`。
- URL 是否为 `/api/open/{path}`。
- 请求方法是否与 `method` 一致。
- `config.json` 的 `id` 是否与目录名一致。
- `path` 是否仅包含允许字符且没有斜杠。

### HTTP 500 或 Open API 执行失败

检查：

- `runtime` 和 `runtime_profile` 对应的运行环境是否可用。
- `entry` 文件是否存在、是否位于接口目录内。
- Node.js 文件是否正确导出 `action(ctx, req, res)`。
- AllBot 日志中的脚本异常或消息发送失败信息。
- `adapterId` 是否为已启用的 AllBot 适配器实例 ID，目标群号或 QQ 号是否有效。

建议先运行纯脚本语法检查和单元测试，再启动 AllBot 做真实 HTTP 验收。

## 8. PushMessage 第三方通知接入

仓库提供：

```text
openapis/PushMessage/config.json
openapis/PushMessage/PushMessage.js
openapis/PushMessage/PushMessage.test.js
```

接口读取：

- JSON 正文 `message`：要发送的非空文本。
- Query `platform`：可选平台标识，例如 `qq`、`qq_office`、`telegram`、`dingtalk`；不传时由 `adapter_id` 自动识别。
- Query `adapter_id`：AllBot 后台中的适配器实例 ID，按非空字符串传递。
- Query `group_id`：平台原始群组 ID，按非空字符串传递。
- Query `user_id`：平台原始用户 ID，按非空字符串传递。

`group_id` 和 `user_id` 必须且只能提供一个。目标 ID 不限制为数字，QQ 官方 OpenID、Telegram 负数群 ID、钉钉用户 ID 等都会原样传入。接口显式调用 `ctx.push()`；默认只传 `adapterId` 让 AllBot 自动识别平台，也支持显式传入 `platform` 校验平台归属。

### 8.1 必须配置双 Token

第三方固定发起：

```text
POST ${GOBOT_URL}?access_token=${GOBOT_TOKEN}&${GOBOT_QQ}
```

AllBot 不识别 `access_token`。在“不修改第三方固定请求格式、不修改 AllBot 系统代码”的约束下，必须在 `GOBOT_QQ` 中额外提供同值的 `token`：

```text
GOBOT_URL=http://服务器:端口/api/open/PushMessage
GOBOT_TOKEN=实际OpenAPI Token

# 推荐：根据适配器实例自动识别平台，发送群消息
GOBOT_QQ=token=实际OpenAPI Token&adapter_id=适配器实例ID&group_id=平台群组ID

# 根据适配器实例自动识别平台，发送私聊消息
GOBOT_QQ=token=实际OpenAPI Token&adapter_id=适配器实例ID&user_id=平台用户ID

# 可选：显式指定平台并校验适配器归属
GOBOT_QQ=token=实际OpenAPI Token&platform=qq_office&adapter_id=适配器实例ID&group_id=群OpenID
```

最终 URL 类似：

```text
http://服务器:端口/api/open/PushMessage?access_token=实际OpenAPI Token&token=实际OpenAPI Token&adapter_id=适配器实例ID&group_id=平台群组ID
```

> **双 Token 是当前“不改系统代码”方案的必要条件。** `access_token` 仅用于满足第三方固定格式，PushMessage 脚本会忽略它；额外的 `token` 才用于 AllBot OpenAPI 网关鉴权。如果第三方环境无法在 `GOBOT_QQ` 中重复配置 Token，请求会在脚本运行前返回 HTTP 401，纯脚本无法解决。

如果 Token 或其他参数包含 URL 特殊字符，应按第三方配置能力进行 URL 编码。更推荐使用只包含 URL 安全字符的随机 Token。

### 8.2 群聊请求示例

以下示例不传 `platform`，AllBot 会根据适配器实例 `3` 自动识别为 `qq_office`：

```http
POST /api/open/PushMessage?access_token=实际Token&token=实际Token&adapter_id=3&group_id=group_openid-abc HTTP/1.1
Content-Type: application/json

{"message":"任务执行完成"}
```

成功响应始终为 HTTP 200：

```json
{
  "status": "ok",
  "retcode": 0,
  "errmsg": "",
  "data": {
    "adapterId": "3",
    "groupId": "group_openid-abc"
  }
}
```

### 8.3 私聊请求示例

也可以显式传入平台。平台必须与适配器实例的实际归属一致：

```http
POST /api/open/PushMessage?access_token=实际Token&token=实际Token&platform=qq_office&adapter_id=3&user_id=user_openid-xyz HTTP/1.1
Content-Type: application/json

{"message":"任务执行失败，请检查日志"}
```

成功响应：

```json
{
  "status": "ok",
  "retcode": 0,
  "errmsg": "",
  "data": {
    "platform": "qq_office",
    "adapterId": "3",
    "userId": "user_openid-xyz"
  }
}
```

### 8.4 参数或发送失败响应

为兼容第三方现有的 `$.post` 和 `retcode === 100` 判断，脚本遇到参数错误或 `ctx.push()` 发送失败时仍返回 HTTP 200：

```json
{
  "status": "failed",
  "retcode": 100,
  "errmsg": "具体错误信息",
  "data": null
}
```

`retcode` 位于响应顶层。HTTP 200 仅用于兼容调用方分支逻辑，业务是否成功应以顶层 `retcode` 为准。网关鉴权失败和 IP 白名单拒绝仍分别由 AllBot 在脚本执行前返回 HTTP 401、403，不会转换为上述兼容响应。

## 9. PushMessage 本地验证

在仓库根目录执行：

```bash
node --check "openapis/PushMessage/PushMessage.js"
node "openapis/PushMessage/PushMessage.test.js"
node -e "const c=require('./openapis/PushMessage/config.json'); if(c.id!=='PushMessage'||c.path!=='PushMessage'||c.method!=='POST'||c.runtime!=='nodejs'||c.entry!=='PushMessage.js'||c.enabled!==false) process.exit(1)"
git diff --check
```

测试会使用假的 `ctx.push`、`req` 和 `res`，不会向任何平台发送消息。只有在本地 AllBot 和目标平台适配器正在运行，并已明确提供真实 Token、适配器实例 ID 与目标时，才进行真实群聊或私聊 HTTP 验收。
