# 插件 WebUI 扩展实施方案

## 目标

为 AllBot 增加通用插件可视化面板能力，让插件可以自带前端页面，并在主 WebUI 中以菜单入口和 iframe 面板方式打开。

本方案只补齐“插件前端面板机制”，不重新设计数据库、支付、消息发送等能力。现有插件 SDK 已经提供数据库、内置支付、消息发送等业务能力，插件前端通过插件 Web API 调用插件后端，插件后端继续使用现有 SDK 完成业务。

## 范围

### 本次实现

1. 插件 manifest 支持声明 WebUI。
2. 后端扫描并返回插件面板列表。
3. 后端托管插件自带的前端静态资源。
4. 主 WebUI 动态显示插件面板菜单入口。
5. 主 WebUI 新增通用 iframe 面板页。
6. 后端新增插件 Web API 入口，用于把浏览器请求转交给插件处理。
7. Node.js / Python 插件 SDK 增加 `web` 注册能力。
8. 补充一个最小示例插件，验证前端页面到插件 API 的完整链路。

### 本次不实现

1. 不新增通用数据库 API。
2. 不重新设计内置支付 API。
3. 不重新设计消息发送 API。
4. 不引入 Module Federation 或其他微前端框架。
5. 不让插件前端直接访问数据库、支付或消息发送能力。
6. 不做插件前端在线编译；插件作者自行打包前端产物。

## 总体架构

```text
插件 web/dist 页面
        ↓ 浏览器 HTTP
/api/plugin-web/{pluginId}/{path...}
        ↓ AllBot Web 登录态鉴权
AllBot 后端插件 Web API 路由
        ↓ 调用插件注册的 web handler
插件运行时 / 插件 SDK
        ↓
现有 SDK 能力：数据库、支付、消息发送、配置等
```

静态页面加载链路：

```text
主 WebUI 菜单
        ↓
/plugin-panels/{pluginId}
        ↓ iframe
/plugin-web/{pluginId}/index.html
        ↓
插件 web/dist/assets/...
```

## 插件 manifest 扩展

### 类型设计

在 `core/types/types.go` 中为插件配置增加 WebUI 字段。

建议新增结构：

```go
type PluginWebUIConfig struct {
    Enabled bool   `json:"enabled"`
    Title   string `json:"title"`
    Entry   string `json:"entry"`
    Icon    string `json:"icon,omitempty"`
    Order   int    `json:"order,omitempty"`
}
```

在 `types.PluginConfig` 中增加：

```go
WebUI PluginWebUIConfig `json:"web_ui,omitempty"`
```

在 `types.Plugin` 中增加：

```go
WebUI PluginWebUIConfig
```

### manifest 示例

```json
{
  "name": "自动发货商城",
  "version": "1.0.0",
  "runtime": "nodejs",
  "entry": "main.js",
  "enabled": true,
  "web_ui": {
    "enabled": true,
    "title": "商城管理",
    "entry": "web/dist/index.html",
    "icon": "Goods",
    "order": 100
  }
}
```

### 字段约束

| 字段 | 规则 |
| --- | --- |
| `enabled` | 为 `true` 时才显示面板 |
| `title` | 为空时回退到插件名称 |
| `entry` | 必须是相对插件目录的路径，不能是绝对路径，不能包含路径穿越 |
| `icon` | 可选；主 WebUI 不识别时使用默认图标 |
| `order` | 可选；同级菜单排序使用 |

## 后端接口设计

### 1. 获取插件面板列表

新增接口：

```text
GET /api/plugin-web/panels
```

返回示例：

```json
[
  {
    "plugin_id": "shop",
    "title": "商城管理",
    "entry_url": "/plugin-web/shop/index.html",
    "icon": "Goods",
    "order": 100,
    "enabled": true
  }
]
```

建议响应结构：

```go
type PluginWebPanel struct {
    PluginID string `json:"plugin_id"`
    Title    string `json:"title"`
    EntryURL string `json:"entry_url"`
    Icon     string `json:"icon,omitempty"`
    Order    int    `json:"order"`
    Enabled  bool   `json:"enabled"`
}
```

处理规则：

1. 从 `plugin.Manager.GetAllPlugins()` 获取已加载插件。
2. 过滤 `plugin.WebUI.Enabled == true` 的插件。
3. 校验 `entry` 是否安全且文件存在。
4. 返回可用面板列表。
5. 按 `order` 和插件名称排序。

### 2. 托管插件 WebUI 静态资源

新增路由：

```text
GET /plugin-web/{pluginId}/...
```

示例：

```text
/plugin-web/shop/index.html
/plugin-web/shop/assets/index.js
/plugin-web/shop/assets/index.css
```

处理规则：

1. 提取 `pluginId` 和资源相对路径。
2. 查询插件是否存在、启用，且声明了 `web_ui.enabled`。
3. 根据 `web_ui.entry` 计算 Web 根目录。
   - `entry = web/dist/index.html`
   - Web 根目录为 `web/dist`
4. 请求 `/plugin-web/shop/` 或 `/plugin-web/shop/index.html` 时返回 entry 文件。
5. 请求其他资源时只允许读取 Web 根目录下文件。
6. 使用 `filepath.Clean` 和绝对路径前缀校验防止路径穿越。
7. 文件不存在返回 404。

安全约束：

- 不允许访问插件根目录下的任意文件。
- 不允许 `../` 跳出 Web 根目录。
- 不允许读取目录。
- 路由经过现有 Web 登录态鉴权。

### 3. 插件 Web API 转发入口

新增路由：

```text
/api/plugin-web/{pluginId}/{path...}
```

示例：

```text
GET  /api/plugin-web/shop/products
POST /api/plugin-web/shop/products
POST /api/plugin-web/shop/orders/123/deliver
```

处理规则：

1. AllBot Web 鉴权通过后进入 handler。
2. 提取 `pluginId`、HTTP method、path、query、headers、body。
3. 查询插件是否存在并启用。
4. 转交给插件注册的 Web API handler。
5. 插件 handler 返回 JSON、状态码、响应头。
6. AllBot 后端写回浏览器。

## 插件 Web API 调用模型

### 推荐实现方式

当前插件运行模式主要是由 Go 进程启动 Node/Python 脚本，并通过 stdin/stdout JSON action 与 SDK 通信。因此插件 Web API 可以复用现有“插件动作调用”模型：

1. 浏览器请求 `/api/plugin-web/shop/products`。
2. Go 后端构造一个特殊消息 payload。
3. 调用 `plugin.Manager.ExecutePlugin(...)` 执行插件入口。
4. SDK 识别这是 Web API 请求。
5. 插件代码中注册的 `allbot.web.get('/products', handler)` 被调用。
6. handler 内继续使用现有 SDK 的 `db`、`pay`、`sendMessage` 等能力。
7. SDK 输出 `web_response` action。
8. Go 后端把结果返回给浏览器。

### Web API 请求上下文

传给插件的 payload 建议包含：

```json
{
  "event_type": "web_api",
  "plugin_id": "shop",
  "method": "GET",
  "path": "/products",
  "query": {
    "page": "1"
  },
  "headers": {
    "content-type": "application/json"
  },
  "body": "...",
  "admin": {
    "username": "admin"
  }
}
```

注意：首版可以只传必要字段：`event_type`、`method`、`path`、`query`、`body`。

### Web API 响应格式

插件 SDK 输出：

```json
{
  "action": "web_response",
  "status": 200,
  "headers": {
    "content-type": "application/json"
  },
  "body": {
    "items": []
  }
}
```

错误响应：

```json
{
  "action": "web_response",
  "status": 400,
  "body": {
    "error": "商品名称不能为空"
  }
}
```

Go 后端处理规则：

- `status` 默认 200。
- `body` 为对象时 JSON 编码。
- `body` 为字符串时按字符串输出。
- 不允许插件设置危险响应头；首版只允许 `content-type`、`content-disposition`。
- 未收到 `web_response` 时返回 500。

## SDK 扩展设计

### Node.js SDK

在 `sdk/nodejs/allbot_direct.js` 中增加 `web` 注册器。

插件用法：

```js
const { runDirect } = require('./allbot_direct')

runDirect(async (ctx) => {
  ctx.web.get('/products', async (req) => {
    const rows = await ctx.db.query({ table: 'shop_products' })
    return { items: rows }
  })

  ctx.web.post('/products', async (req) => {
    const body = await req.json()
    if (!body.name) {
      return req.jsonResponse({ error: '商品名称不能为空' }, 400)
    }
    await ctx.db.insert('shop_products', body)
    return { message: '保存成功' }
  })
})
```

建议 API：

```js
ctx.web.get(path, handler)
ctx.web.post(path, handler)
ctx.web.put(path, handler)
ctx.web.delete(path, handler)
```

`handler(req)` 中的 `req`：

```js
{
  method: 'GET',
  path: '/products',
  query: {},
  headers: {},
  body: '',
  json: async () => ({}),
  jsonResponse: (data, status = 200, headers = {}) => ({ ... })
}
```

SDK 行为：

1. `runDirect` 初始化 context。
2. 插件代码注册 web routes。
3. 如果输入 payload 是 `event_type=web_api`，SDK 不执行普通消息逻辑，而是匹配 web route。
4. 匹配成功后执行 handler。
5. 将 handler 返回值封装成 `web_response`。
6. 未匹配返回 404。

### Python SDK

在 `sdk/python/allbot_direct.py` 中提供类似能力。

插件用法：

```python
from allbot_direct import run_direct

async def main(ctx):
    @ctx.web.get('/products')
    async def products(req):
        rows = await ctx.db.query(table='shop_products')
        return {'items': rows}

    @ctx.web.post('/products')
    async def create_product(req):
        body = await req.json()
        if not body.get('name'):
            return req.json_response({'error': '商品名称不能为空'}, 400)
        await ctx.db.insert('shop_products', body)
        return {'message': '保存成功'}

run_direct(main)
```

实现逻辑与 Node.js SDK 保持一致。

## 主 WebUI 改造

### 1. API 封装

在 `web-ui/src/api/index.js` 增加：

```js
export const getPluginWebPanels = () => request({
  url: '/plugin-web/panels',
  method: 'get'
})
```

注意：因为全局 request 基础路径通常已有 `/api` 前缀，实际请求为 `/api/plugin-web/panels`。

### 2. 新增通用插件面板页

新增：

```text
web-ui/src/views/PluginPanel.vue
```

核心结构：

```vue
<template>
  <div class="plugin-panel-page">
    <iframe
      v-if="entryUrl"
      :src="entryUrl"
      class="plugin-panel-frame"
      sandbox="allow-scripts allow-forms allow-same-origin allow-downloads"
    />
  </div>
</template>
```

页面逻辑：

1. 从路由参数读取 `pluginId`。
2. 调用 `getPluginWebPanels()` 查找对应面板。
3. 设置 iframe `src = panel.entry_url`。
4. 找不到时显示“插件面板不存在或未启用”。

### 3. 动态菜单

在 `web-ui/src/views/Layout.vue` 中加载插件面板列表。

建议菜单结构：

```text
插件面板
  商城管理
  其他插件面板
```

点击跳转：

```text
/plugin-panels/{pluginId}
```

移动端菜单同步显示。

### 4. 路由

如果当前项目使用前端路由配置文件，新增：

```js
{
  path: '/plugin-panels/:pluginId',
  name: 'PluginPanel',
  component: () => import('@/views/PluginPanel.vue')
}
```

如果当前路由在 `App.vue` 或 Layout 中硬编码，需要按现有方式接入。

## 后端实现位置建议

### 修改文件

| 文件 | 改动 |
| --- | --- |
| `core/types/types.go` | 增加 `PluginWebUIConfig`，扩展 `PluginConfig` 和 `Plugin` |
| `core/plugin/manager.go` | 加载插件时映射 `web_ui` 字段，增加 Web API 执行方法 |
| `core/web/server.go` | 注册 `/api/plugin-web/panels`、`/api/plugin-web/`、`/plugin-web/` 路由 |
| `core/web/plugin_web.go` | 新增插件 WebUI 静态资源、面板列表和 API 转发 handler |
| `sdk/nodejs/allbot_direct.js` | 增加 `ctx.web` 路由注册和 `web_api` 分发 |
| `sdk/python/allbot_direct.py` | 增加对应 Python SDK 能力 |
| `web-ui/src/api/index.js` | 增加插件面板列表 API |
| `web-ui/src/views/Layout.vue` | 动态菜单 |
| `web-ui/src/views/PluginPanel.vue` | iframe 面板页 |
| `web-ui/FRONTEND_CHANGES.md` | 记录前端源码变更 |

### 新增后端文件建议

```text
core/web/plugin_web.go
core/web/plugin_web_test.go
```

## 后端关键函数建议

### 面板收集

```go
func (s *Server) pluginWebPanels() ([]PluginWebPanel, error)
```

职责：

- 读取插件列表。
- 过滤启用的 WebUI。
- 校验入口安全。
- 构造入口 URL。

### 静态文件路径解析

```go
func (s *Server) resolvePluginWebFile(pluginID, requestPath string) (string, error)
```

职责：

- 获取插件目录。
- 根据 entry 计算 Web 根目录。
- 将 requestPath 映射到文件。
- 做绝对路径前缀校验。

### API 转发

```go
func (s *Server) handlePluginWebAPI(w http.ResponseWriter, r *http.Request)
```

职责：

- 提取 pluginID 和 path。
- 读取 body。
- 调用 `pluginManager.ExecutePluginWebRequest(...)`。
- 写回响应。

### 插件执行方法

```go
type PluginWebRequest struct {
    Method  string            `json:"method"`
    Path    string            `json:"path"`
    Query   map[string]string `json:"query"`
    Headers map[string]string `json:"headers"`
    Body    json.RawMessage   `json:"body"`
}

type PluginWebResponse struct {
    Status  int               `json:"status"`
    Headers map[string]string `json:"headers"`
    Body    interface{}       `json:"body"`
}
```

`plugin.Manager` 增加：

```go
func (m *Manager) ExecutePluginWebRequest(pluginID string, req PluginWebRequest) (PluginWebResponse, error)
```

内部复用现有 `ExecutePlugin`，但传入特殊 payload。

## 鉴权设计

当前 `server.go` 已经用：

```go
s.corsMiddleware(s.authMiddleware(mux))
```

因此新增路由默认会经过现有 `authMiddleware`。

需要确认：

1. `/api/login`、支付回调等公开路由仍按现有白名单放行。
2. `/plugin-web/` 不加入公开白名单。
3. `/api/plugin-web/` 不加入公开白名单。
4. 未登录访问插件面板 API 返回 401。

首版权限模型：

- 只要登录 AllBot 后台，就能访问插件面板。
- 暂不做插件级权限细分。

后续可扩展：

```json
"web_ui": {
  "permissions": ["shop.manage"]
}
```

## 插件前端开发方式

插件作者自行维护前端项目：

```text
plugins/shop/web-src/
  package.json
  vite.config.js
  src/App.vue
```

构建输出到：

```text
plugins/shop/web/dist/
```

Vite 配置建议使用相对 base：

```js
export default defineConfig({
  base: './'
})
```

原因：插件 iframe 地址为：

```text
/plugin-web/shop/index.html
```

使用 `base: './'` 可以让资源按相对路径加载：

```text
/plugin-web/shop/assets/index.js
```

插件前端请求 API：

```js
const apiBase = '/api/plugin-web/shop'

export async function getProducts() {
  const resp = await fetch(`${apiBase}/products`)
  return await resp.json()
}
```

## 示例插件建议

新增一个最小示例，用于验证机制：

```text
plugins/webui-demo/
  plugin.json
  main.js
  web/dist/index.html
```

`plugin.json`：

```json
{
  "name": "WebUI 示例",
  "version": "1.0.0",
  "runtime": "nodejs",
  "entry": "main.js",
  "enabled": true,
  "web_ui": {
    "enabled": true,
    "title": "WebUI 示例",
    "entry": "web/dist/index.html"
  }
}
```

`main.js` 示例逻辑：

```js
const { runDirect } = require('../../sdk/nodejs/allbot_direct')

runDirect(async (ctx) => {
  ctx.web.get('/hello', async () => {
    return { message: 'hello from plugin web api' }
  })
})
```

`web/dist/index.html` 可以先用纯 HTML：

```html
<!doctype html>
<html>
<body>
  <h1>插件 WebUI 示例</h1>
  <button onclick="load()">调用插件 API</button>
  <pre id="output"></pre>
  <script>
    async function load() {
      const resp = await fetch('/api/plugin-web/webui-demo/hello')
      document.getElementById('output').textContent = JSON.stringify(await resp.json(), null, 2)
    }
  </script>
</body>
</html>
```

这样不依赖额外前端构建即可验证首版闭环。

## 测试计划

### Go 单元测试

新增或扩展：

```text
core/web/plugin_web_test.go
core/plugin/manager_test.go
```

覆盖：

1. manifest 中 `web_ui` 字段能正确加载到 `types.Plugin`。
2. `GET /api/plugin-web/panels` 只返回启用 WebUI 的插件。
3. WebUI entry 为空、路径穿越、文件不存在时不返回面板。
4. `GET /plugin-web/{pluginId}/index.html` 能返回插件静态文件。
5. 静态资源路径穿越被拒绝。
6. 未启用 WebUI 的插件不能访问静态资源。
7. `/api/plugin-web/{pluginId}/...` 能构造正确 Web API 请求。
8. 插件返回 `web_response` 后能正确写回状态码和 JSON。
9. 插件未返回响应时返回 500。
10. 未知插件返回 404 或 400。

### SDK 测试

Node.js：

```text
sdk/nodejs/allbot_direct.test.js
```

覆盖：

1. `ctx.web.get('/hello')` 能注册路由。
2. `event_type=web_api` 时能命中路由。
3. 未命中路由返回 404。
4. handler 抛错返回 500。
5. `req.json()` 能解析 JSON body。

Python：

```text
sdk/python/test_allbot_direct.py
```

覆盖同 Node.js。

### 前端验证

1. 插件面板列表能出现在菜单。
2. 点击菜单能打开 iframe 页面。
3. iframe 页面能加载插件静态资源。
4. iframe 页面能请求 `/api/plugin-web/{pluginId}/hello`。
5. 未登录时无法访问插件面板和 API。

## 本地验证命令

后端和 SDK：

```bash
cd "D:/Desktop/program/java/AITest/allbot" && go test ./core/types ./core/plugin ./core/web
```

全量 Go 测试：

```bash
cd "D:/Desktop/program/java/AITest/allbot" && go test ./...
```

Node SDK 测试：

```bash
cd "D:/Desktop/program/java/AITest/allbot/sdk/nodejs" && npm test
```

Python SDK 测试：

```bash
cd "D:/Desktop/program/java/AITest/allbot/sdk/python" && python -m pytest
```

前端构建：

```bash
cd "D:/Desktop/program/java/AITest/allbot/web-ui" && npm run build
```

重新编译嵌入前端资源的可执行文件：

```bash
cd "D:/Desktop/program/java/AITest/allbot" && go build -o allbot.exe .
```

## 实施顺序

### 第 1 步：manifest 扩展

1. 修改 `core/types/types.go`。
2. 增加 `PluginWebUIConfig`。
3. 插件加载时映射 `web_ui` 字段。
4. 补测试，确认旧插件不受影响。

验收：旧插件无 `web_ui` 字段时仍能正常加载。

### 第 2 步：插件面板列表接口

1. 新增 `core/web/plugin_web.go`。
2. 实现 `GET /api/plugin-web/panels`。
3. 注册路由。
4. 补测试。

验收：启用 WebUI 的插件能出现在接口返回中。

### 第 3 步：插件静态资源托管

1. 实现 `/plugin-web/{pluginId}/...`。
2. 做路径安全校验。
3. 支持 index fallback。
4. 补测试。

验收：能打开插件 `web/dist/index.html`，路径穿越会被拒绝。

### 第 4 步：主 WebUI 菜单和 iframe 页面

1. 新增 `PluginPanel.vue`。
2. 增加前端 API 封装。
3. Layout 加载插件面板菜单。
4. 移动端菜单同步支持。
5. 更新 `FRONTEND_CHANGES.md`。

验收：主 WebUI 可以点击菜单打开插件 iframe 页面。

### 第 5 步：插件 Web API 后端入口

1. 新增 `/api/plugin-web/{pluginId}/...`。
2. 定义 `PluginWebRequest` / `PluginWebResponse`。
3. `plugin.Manager` 增加 `ExecutePluginWebRequest`。
4. 复用现有插件执行和 action 通道。
5. 补测试。

验收：Go 后端可以把浏览器请求送进插件，并返回插件响应。

### 第 6 步：Node.js SDK Web 路由

1. `allbot_direct.js` 增加 `ctx.web`。
2. 支持 `get/post/put/delete`。
3. 支持 `event_type=web_api` 分发。
4. 输出 `web_response`。
5. 补测试。

验收：Node 插件能处理 `/api/plugin-web/{pluginId}/hello`。

### 第 7 步：Python SDK Web 路由

1. `allbot_direct.py` 增加 `ctx.web`。
2. 与 Node.js SDK 对齐。
3. 补测试。

验收：Python 插件也能处理 Web API。

### 第 8 步：示例插件和端到端验证

1. 增加最小 WebUI 示例插件。
2. 页面调用自身 `/hello` API。
3. 本地打开 WebUI 验证。
4. 运行全量测试、前端构建、重新编译。

验收：菜单出现示例插件，iframe 正常显示，按钮能拿到插件 API 返回。

## 风险与处理

### 1. 插件 Web API 每次请求都启动插件进程，性能可能一般

首版可以接受，因为实现简单，复用现有执行链路。

后续优化：

- 对声明 WebUI 的插件启动常驻 Web API worker。
- 或者为插件分配本地端口，AllBot 作为反向代理。

### 2. iframe 页面和主 WebUI 视觉不一致

首版接受。

后续优化：

- 提供插件前端模板。
- 提供 AllBot CSS 变量。
- 用 postMessage 传递主题信息。

### 3. 插件静态资源路径安全

必须严格校验：

- `entry` 只能是相对路径。
- 请求文件必须位于 Web 根目录下。
- 禁止目录访问和路径穿越。

### 4. 插件 Web API 响应格式不统一

由 SDK 统一封装 `web_response`，插件作者无需直接输出底层 action。

### 5. 插件前端无法直接使用 SDK

这是预期行为。浏览器只调用 HTTP API，真正 SDK 能力仍在插件后端运行环境里。

## 后续优化方向

1. 插件前端 SDK：封装 `request`、`toast`、`confirm`、主题信息。
2. 插件面板权限：支持只允许管理员或指定角色访问。
3. 插件面板分组：支持 `menu.group`。
4. 插件图标白名单：避免前端动态图标找不到。
5. 常驻插件 Web worker：提升高频后台接口性能。
6. 插件打包模板：提供 `create-allbot-plugin` 或示例目录。
7. 插件市场：上传插件 zip 后自动安装 WebUI 和后端逻辑。

## 最终结论

现有 SDK 已经覆盖数据库、支付和消息发送，因此本次核心不是重做业务能力，而是增加一条从主 WebUI 到插件后端的可视化通道：

```text
插件自带前端静态资源
        +
AllBot iframe 挂载
        +
/api/plugin-web/{pluginId}/... 转发
        +
SDK ctx.web 路由注册
```

这条链路打通后，商城插件就可以：

1. 自带 Vue 管理后台。
2. 在 AllBot 主菜单中显示“商城管理”。
3. 页面通过 HTTP 调用商城插件 Web API。
4. 商城插件 Web API 继续使用现有 SDK 的数据库、支付、消息发送能力。
5. 后续其他插件也能复用同一套 WebUI 扩展机制。
