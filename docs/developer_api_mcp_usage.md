# AllBot 开发者接口与 MCP 使用说明

## 1. 管理员创建应用

登录 AllBot 管理后台，打开“开发者应用”，创建应用并按需授予能力。应用凭据统一称为 `appKey` 和 `appSecret`。创建成功时页面只展示一次 `appSecret`；请立即保存。后续可修改权限、重置 `appSecret` 或删除应用。重置会立即撤销旧 Secret 和由旧凭据签发的令牌；权限修改会立即作用于后续请求。

应用权限按业务模块授予，例如 `system`、`settings`、`adapters`、`users`。勾选一个模块后，即可调用该模块下所有已开放的接口，包括读取、创建、修改、删除和执行等操作；未授权模块默认拒绝。`*` 表示全部模块，不包含应用管理、管理员登录/密码等安全控制面，也不允许调用自定义 OpenAPI 脚本。

## 2. 开发者 HTTP API

面板地址作为服务根地址，例如 `https://allbot.example.com`。REST 开放 API 使用 `/api/v1/` 前缀。

### 换取应用令牌

```http
POST /api/v1/open-api/token
Content-Type: application/json

{"app_key":"<appKey>","app_secret":"<appSecret>"}
```

成功响应：

```json
{"access_token":"<token>","token_type":"Bearer","expires_in":86400}
```

令牌有效期为 24 小时。REST 业务接口只接受该端点签发的应用 Bearer 令牌：

```http
Authorization: Bearer <access_token>
```

REST 不接受 Basic、直接使用 `appSecret` 作为 Bearer 令牌或面板登录令牌。缺少令牌或使用 Basic 返回 `401 {"error":"缺少授权令牌"}`；无效/过期令牌返回 `401 {"error":"令牌无效或已过期"}`。

### 能力目录

`GET /api/v1/capabilities` 返回该应用当前已获授权且可调用的具体接口能力。应用权限在每次请求时重新读取，因此权限撤销不需要等待令牌过期。

### 管理业务接口

接口使用 `/api/v1/` 前缀，并映射到管理员后台同名业务路径。例如后台 `/api/plugins` 对应开发者接口 `/api/v1/plugins`，后台 `/api/users/{id}` 对应 `/api/v1/users/{id}`。请求方法和路径会映射到具体操作；应用只需获授对应模块即可调用该模块下所有已开放操作，未注册的路径仍默认拒绝。可用模块包括 `system`、`settings`、`plugins`、`adapters`、`users`、`replies`、`tasks`、`script_envs`、`dependencies`、`payments`、`images`、`logs`、`backups`、`data`、`statistics`、`sdk` 和 `open_apis`。

`settings` 模块只返回经筛选的一般设置，并只允许修改常规运行设置；管理员账号、平台管理员、登录访问码等安全字段仍不可读取或修改。

业务接口沿用后台 Handler 的请求与响应结构。对具体路径、请求体和响应字段，以当前后台业务接口契约为准；请只授予业务所需的模块。

## 3. MCP

MCP Streamable HTTP 地址为：

```text
https://allbot.example.com/api/v1/mcp
```

MCP 接受以下三种 Authorization：

```http
Authorization: Basic base64(appKey:appSecret)
```

或使用应用令牌：

```http
Authorization: Bearer <access_token>
```

MCP 也兼容面板登录 Bearer 令牌，该身份按管理员会话权限调用并以管理员主体审计。应用场景建议使用专用应用凭据。MCP 工具 `list_capabilities` 返回当前身份可用的具体接口能力；每项能力对应一个 `api_<resource>_<action>` 工具，接收 `method`、`path`、可选 `query` 与 JSON `body`。获得某个模块授权后，该模块对应的工具都会提供。文件/二进制请求使用 `raw_body_base64` 并提供原始 `content_type`（multipart 必须带 boundary）；二进制响应以包含 `content_type`、`encoding: base64` 和 `data` 的 JSON 文本返回。服务端会验证目标路径映射到该工具声明的具体操作。

REST 与 MCP 的鉴权入口不同：REST 只能使用应用令牌，MCP 才可直接使用应用 Basic 凭据或面板登录令牌。不得将凭证放入 URL 或 Query。生产环境应使用 HTTPS。

## 4. 示例

```bash
curl -X POST 'https://allbot.example.com/api/v1/open-api/token' \
  -H 'Content-Type: application/json' \
  -d '{"app_key":"<appKey>","app_secret":"<appSecret>"}'

curl 'https://allbot.example.com/api/v1/capabilities' \
  -H 'Authorization: Bearer <access_token>'
```

## 5. 错误与审计

认证失败为 401，能力不足为 403，未注册接口为 404。开发者调用会记录应用、协议、能力、来源地址和结果状态，不记录 Secret、访问令牌或请求体。破坏性及高风险操作在执行前要求审计存储可用。
