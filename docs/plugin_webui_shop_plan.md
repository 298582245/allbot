# 插件 WebUI 扩展与自动发货商城方案

## 背景

AllBot 目前已经有 Vue 实现的主 WebUI，打包后作为静态资源嵌入 Go 程序。对于商城、卡密、自动发货、会员、积分兑换等插件型功能，如果直接把页面写进主 `web-ui/src`，每次新增或修改插件后台都需要重新构建主前端并重新编译 `allbot.exe`。

商城系统本身适合以插件形式实现，因为它需要复用 AllBot 的机器人消息、插件执行、内置支付、用户账号和订单通知能力。但商城又强依赖可视化后台，例如商品管理、卡密导入导出、订单查询、补发、库存管理等。因此更推荐先设计一套通用的“插件 WebUI 扩展机制”，商城作为第一个落地插件。

## 总体建议

不要把商城后台硬编码进主 WebUI。

推荐方案是：

```text
插件 manifest 声明 web_ui
        ↓
后端暴露插件静态资源 /plugin-web/{pluginId}/...
        ↓
主 Vue WebUI 自动生成菜单
        ↓
PluginPanel.vue 用 iframe 加载插件页面
        ↓
插件通过 /api/plugin-web/{pluginId}/... 调用自己的后端接口
        ↓
商城插件复用内置支付，支付成功后自动发货
```

主 WebUI 只负责发现、导航、挂载和基础上下文传递；具体页面由插件自己提供。

## 为什么优先使用 iframe

首版建议使用 iframe 加载插件前端，而不是微前端或动态加载 Vue 组件。

原因：

1. 主 WebUI 已经是打包后的静态应用，不适合运行时动态 import 未知插件组件。
2. 插件可以独立使用 Vue、React、纯 HTML 或其他前端技术。
3. 插件 CSS、路由、依赖和主 WebUI 隔离，不容易互相污染。
4. 插件升级时只需要替换插件目录下的前端产物，不需要修改主 WebUI。
5. 对第三方插件开发者更友好，降低插件生态门槛。
6. 插件页面加载失败不会直接破坏主 WebUI。

微前端 Module Federation 可以作为后续增强，但不建议作为首版方案。它会引入依赖共享、版本冲突、构建配置复杂度和运行时故障隔离问题。

## 插件目录结构建议

以商城插件为例：

```text
plugins/shop/
  plugin.json
  main.js 或 main.py
  web/
    dist/
      index.html
      assets/
        index.js
        index.css
  migrations/
    001_create_shop_tables.sql
    002_add_delivery_rules.sql
```

其中：

- `plugin.json` 声明插件基础信息和 WebUI 入口。
- `main.js` 或 `main.py` 处理机器人消息、支付回调、自动发货逻辑。
- `web/dist` 存放插件自己的前端构建产物。
- `migrations` 存放插件数据库迁移 SQL。

## plugin.json 扩展建议

新增 `web_ui` 字段：

```json
{
  "id": "shop",
  "name": "自动发货商城",
  "version": "1.0.0",
  "web_ui": {
    "enabled": true,
    "title": "商城管理",
    "entry": "web/dist/index.html",
    "menu": {
      "group": "plugins",
      "icon": "Goods",
      "order": 100
    }
  }
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `enabled` | 是否启用插件 WebUI |
| `title` | 在主 WebUI 菜单中显示的名称 |
| `entry` | 插件前端入口文件，相对插件目录 |
| `menu.group` | 菜单分组，首版可固定为 `plugins` |
| `menu.icon` | 菜单图标名称，可选 |
| `menu.order` | 菜单排序权重，可选 |

## 后端能力设计

### 1. 插件 WebUI 静态资源服务

新增静态资源路由：

```text
GET /plugin-web/{pluginId}/...
```

示例：

```text
/plugin-web/shop/index.html
/plugin-web/shop/assets/index.js
/plugin-web/shop/assets/index.css
```

后端根据插件 ID 找到插件目录，再根据 `plugin.json.web_ui.entry` 确定允许访问的 WebUI 根目录。

首版建议约束：

- 只允许访问插件声明的 WebUI 目录。
- 禁止路径穿越。
- `index.html` 找不到时返回 404。
- 静态资源使用常规缓存头即可。

### 2. 插件 Web API 入口

新增插件 API 路由：

```text
/api/plugin-web/{pluginId}/{path...}
```

示例：

```text
GET    /api/plugin-web/shop/products
POST   /api/plugin-web/shop/products
PUT    /api/plugin-web/shop/products/{id}
DELETE /api/plugin-web/shop/products/{id}

GET    /api/plugin-web/shop/orders
POST   /api/plugin-web/shop/orders/{id}/deliver
POST   /api/plugin-web/shop/import
GET    /api/plugin-web/shop/export
```

主程序负责：

1. 校验插件是否存在且启用。
2. 校验插件是否声明了 Web API 能力。
3. 将请求转发给插件对应 handler。
4. 统一处理错误、日志和权限边界。

### 3. 插件 WebUI 列表接口

主 WebUI 需要获取可显示的插件面板列表。

建议新增：

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
    "group": "plugins",
    "icon": "Goods",
    "order": 100
  }
]
```

主 WebUI 根据该接口动态生成菜单。

### 4. 插件数据库迁移

商城一定需要自己的数据表。建议允许插件携带 migrations，并由主程序统一执行。

新增核心记录表：

```sql
CREATE TABLE IF NOT EXISTS plugin_migrations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  plugin_id TEXT NOT NULL,
  version TEXT NOT NULL,
  file_name TEXT NOT NULL,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(plugin_id, file_name)
);
```

执行规则：

1. 插件加载时扫描 `plugins/{pluginId}/migrations/*.sql`。
2. 查询 `plugin_migrations`，跳过已执行文件。
3. 按文件名排序执行。
4. 执行成功后记录。
5. 执行失败时阻止插件启用，并展示错误。

## 主 WebUI 改造建议

### 1. 新增通用插件面板页面

新增页面：

```text
web-ui/src/views/PluginPanel.vue
```

路由：

```text
/plugin-panels/:pluginId
```

页面核心逻辑：

```vue
<template>
  <div class="plugin-panel-page">
    <iframe
      :src="entryUrl"
      class="plugin-panel-frame"
      sandbox="allow-scripts allow-forms allow-same-origin allow-downloads"
    />
  </div>
</template>
```

### 2. 菜单动态加载插件面板

主布局加载 `/api/plugin-web/panels`，将返回项追加到菜单中。

例如：

```text
插件扩展
  - 商城管理
  - 卡密管理
  - 会员中心
```

### 3. 插件页面上下文传递

首版可以让插件前端直接请求自己的 API：

```js
fetch('/api/plugin-web/shop/products')
```

后续可以通过 `postMessage` 给 iframe 传上下文：

```json
{
  "type": "allbot:init",
  "pluginId": "shop",
  "apiBase": "/api/plugin-web/shop",
  "theme": "light",
  "locale": "zh-CN"
}
```

插件前端可封装一个轻量 SDK：

```js
const allbot = await createAllBotPluginClient()
await allbot.request('/products')
allbot.toast.success('保存成功')
```

首版不必强制实现 SDK，先保证 iframe + API 能工作。

## 商城插件功能设计

### 1. 商品管理

字段建议：

- 商品 ID
- 商品名称
- 商品描述
- 商品价格
- 商品状态：上架、下架
- 发货类型：
  - 卡密库存
  - 固定文本
  - 文件或链接
  - 人工处理
- 限购规则
- 可用平台或群
- 创建时间
- 更新时间

### 2. 库存管理

自动发货常见需求：

- 批量导入卡密
- 导出未售卡密
- 导出已售卡密
- 重复卡密检测
- 库存数量统计
- 导入批次记录
- 卡密售出后自动标记

### 3. 订单管理

字段建议：

- 商城订单号
- 内置支付订单号
- 用户平台
- 用户 ID
- 群 ID
- 商品 ID
- 商品名称
- 数量
- 金额
- 支付状态
- 发货状态
- 发货内容摘要
- 创建时间
- 支付时间
- 发货时间

后台功能：

- 查询订单
- 按状态筛选
- 查看支付信息
- 查看发货结果
- 手动补发
- 标记异常

### 4. 发货规则

支付成功后自动发货：

1. 商城插件创建商城订单。
2. 调用 AllBot 内置支付创建支付单。
3. 用户完成支付。
4. 内置支付模块产生支付成功事件。
5. 商城插件收到事件。
6. 商城插件根据商品类型发货。
7. 发货结果写入订单表。
8. 通过机器人私聊或群消息通知用户。

## 内置支付集成建议

商城插件不应该自己实现支付底层逻辑，而是调用 AllBot 内置支付能力。

建议核心提供插件可调用的支付 API：

```go
CreatePaymentOrder(req PaymentCreateRequest) (PaymentOrder, error)
GetPaymentOrder(orderNo string) (PaymentOrder, error)
RegisterPaymentSuccessHandler(pluginID string, handler func(PaymentEvent) error)
```

支付单需要绑定业务信息：

```json
{
  "plugin_id": "shop",
  "business_type": "shop_order",
  "business_id": "shop_order_123"
}
```

支付成功事件示例：

```json
{
  "event": "payment.success",
  "order_no": "P202607030001",
  "plugin_id": "shop",
  "business_type": "shop_order",
  "business_id": "shop_order_123",
  "amount": 990,
  "paid_at": "2026-07-03T12:00:00+08:00"
}
```

商城插件只处理业务逻辑：

- 校验订单是否存在。
- 校验是否已发货，避免重复发货。
- 扣减库存。
- 发送发货内容。
- 更新发货状态。

## 安全与边界建议

虽然插件 WebUI 使用 iframe 隔离，但仍需注意边界：

1. 静态资源只能从插件声明目录读取，防止路径穿越。
2. 插件 Web API 只能访问对应插件 ID 下的 handler。
3. 插件 API 应复用主 WebUI 登录态，避免未登录访问后台。
4. iframe 建议使用 sandbox，按需开启能力。
5. 插件前端不要直接接触数据库，只通过后端 API 操作。
6. 支付成功事件必须保证幂等，避免重复发货。
7. 卡密导入导出要考虑大文件和敏感数据展示。

## 不推荐方案

### 1. 直接把商城写入主 WebUI

不推荐原因：

- 每次改商城页面都要重新构建主前端。
- 每个带后台的插件都要改主仓库。
- 插件无法独立发布。
- 主 WebUI 会越来越臃肿。
- 不利于后续插件生态。

### 2. 首版使用 Module Federation

不推荐首版使用，原因：

- 构建复杂。
- Vue、Element Plus、Pinia、Router 依赖共享容易冲突。
- 第三方插件开发门槛高。
- 插件加载失败可能影响主应用体验。

可以作为后续高级模式，而不是首版基础能力。

## 实施阶段建议

### 第一阶段：通用插件 WebUI 能力

目标：任何插件都可以声明并展示自己的后台页面。

任务：

1. 扩展 `plugin.json`，支持 `web_ui` 字段。
2. 后端支持 `/plugin-web/{pluginId}/...` 静态资源。
3. 后端支持 `/api/plugin-web/panels` 面板列表。
4. 主 WebUI 支持动态菜单。
5. 新增 `PluginPanel.vue`，iframe 加载插件页面。
6. 插件页面可直接请求 `/api/plugin-web/{pluginId}/...`。

### 第二阶段：插件 Web API 注册机制

目标：插件可以提供自己的后台接口。

任务：

1. 设计插件 Web API handler 注册接口。
2. Node.js SDK 支持注册 Web API。
3. Python SDK 支持注册 Web API。
4. 后端统一转发 `/api/plugin-web/{pluginId}/...`。
5. 加入基础鉴权、错误格式和日志。

### 第三阶段：插件数据库迁移

目标：插件可以携带并自动执行 SQL migration。

任务：

1. 新增 `plugin_migrations` 表。
2. 插件加载时扫描 migrations。
3. 按顺序执行未执行 SQL。
4. 失败时阻止插件启用并展示错误。
5. 在 WebUI 插件详情页展示 migration 状态。

### 第四阶段：商城插件

目标：实现自动发货商城。

任务：

1. 设计商城数据表。
2. 实现商品管理。
3. 实现卡密库存导入导出。
4. 实现订单管理。
5. 接入内置支付创建支付单。
6. 接收支付成功事件并自动发货。
7. 后台支持补发和异常处理。

### 第五阶段：生态完善

目标：让更多插件可以复用可视化能力。

任务：

1. 提供插件 WebUI 模板。
2. 提供插件前端 SDK。
3. 支持菜单分组和图标。
4. 支持插件页面主题适配。
5. 支持插件权限声明。

## 推荐结论

商城系统应该作为插件实现，但不应该把商城后台直接写进主 WebUI。

推荐先做通用插件 WebUI 扩展能力：

- 插件自带前端构建产物。
- 主 WebUI 动态发现并挂载。
- iframe 隔离加载插件页面。
- 插件通过专属 API 操作后端。
- 商城复用内置支付和机器人消息能力。

这样商城只是第一个案例，后续任何需要可视化后台的插件都可以复用同一套机制。
