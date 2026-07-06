# Web 聊天插件独立会话方案

## 背景

`/chat` 当前是全局聊天室：用户输入进入统一消息流，再由路由按插件触发规则匹配。这样会带来几个问题：

- 左侧直接展示插件正则触发规则，普通用户难以理解。
- 多个插件共享同一条聊天历史，插件之间消息互相干扰。
- 不同插件无法提供不同的固定快捷指令、输入提示和说明。
- 已选中某个插件后，用户输入仍可能被全局触发规则分配给别的插件。

目标是把 `/chat` 改造成“插件会话入口”：左侧为可搜索插件列表，右侧为当前插件的独立对话。

## 产品语义

1. 用户进入 `/chat` 后先看到可用插件列表。
2. 左侧支持搜索插件和快捷指令。
3. 点击插件后进入该插件独立对话。
4. 每个插件拥有独立消息历史，插件之间消息不互通。
5. 当前插件可以展示自己的快捷指令、描述和输入框提示。
6. 用户输入只发送给当前选中的插件，不再依赖全局 trigger 匹配。
7. 机器人回复继续支持文本、Markdown、图片、按钮和富文本。
8. 用户发送端仍只支持文本。

## 前端形态

### 左侧插件列表

- 搜索框：支持按插件名称、插件 ID、展示标题、描述、关键词、快捷指令搜索。
- 插件卡片显示：
  - 展示标题
  - 简短描述
  - 快捷指令数量或部分快捷指令预览
  - 可选最近消息时间/未读提示

### 右侧插件对话

选中插件后右侧显示：

- 顶部：插件展示标题和描述。
- 快捷指令区：展示该插件的固定快捷指令。
- 消息区：只显示当前插件的消息。
- 输入框：使用插件配置的 placeholder，用户只能输入文本。

## 插件配置建议

在插件配置中新增可选 `web_chat` 字段，供插件作者适配 Web 聊天展示：

```json
{
  "web_chat": {
    "enabled": true,
    "title": "商城",
    "description": "浏览商品、购买商品、查看订单",
    "placeholder": "输入商品名称、序号或订单相关问题",
    "entry_text": "商品列表",
    "quick_actions": [
      { "label": "商品列表", "text": "商品列表" },
      { "label": "我的订单", "text": "我的订单" },
      { "label": "联系客服", "text": "联系客服" }
    ],
    "keywords": ["商城", "商品", "购买", "订单"]
  }
}
```

### 字段含义

- `enabled`：是否在 Web 聊天中展示，默认跟随插件启用状态。
- `title`：Web 聊天中显示的标题；为空时使用插件名称。
- `description`：插件用途说明。
- `placeholder`：当前插件输入框提示。
- `entry_text`：默认入口文本，可用于首次进入或“打开插件”。
- `quick_actions`：固定快捷指令。
- `keywords`：搜索关键词。

### 兼容老插件

没有 `web_chat` 配置时自动降级：

- `title = plugin.name`
- `description = "点击进入插件对话"`
- `entry_text` 尝试从简单 trigger 去掉 `^` 和 `$` 得到。
- 简单 trigger 生成一个“打开插件”快捷指令。
- 复杂正则不直接展示，避免用户看到难懂规则。

## 后端 API 设计

### 插件列表

扩展：

```http
GET /api/open/web-chat/plugins
```

返回示例：

```json
[
  {
    "id": "shop",
    "name": "商城插件",
    "title": "商城",
    "description": "浏览商品、购买商品、查看订单",
    "trigger": "^商品列表$",
    "entry_text": "商品列表",
    "placeholder": "输入商品名称、序号或订单相关问题",
    "quick_actions": [
      { "label": "商品列表", "text": "商品列表" },
      { "label": "我的订单", "text": "我的订单" }
    ],
    "keywords": ["商城", "商品", "订单"],
    "status": "running"
  }
]
```

### 查询消息

扩展：

```http
GET /api/open/web-chat/messages?plugin_id=shop&after_id=0&limit=50
```

要求：

- 必须按 `user_id + plugin_id` 查询。
- 默认 `limit=50`，最大 `limit=100`。
- 插件切换时只加载当前插件消息，不一次加载全部插件历史。

### 发送消息

扩展：

```http
POST /api/open/web-chat/messages
```

请求：

```json
{
  "plugin_id": "shop",
  "type": "text",
  "content": "商品列表"
}
```

要求：

- `plugin_id` 必填。
- 用户发送端只允许 `type=text`。
- 后端保存入站消息时写入 `plugin_id`。
- 后端只投递给指定插件，不再走全局 trigger 匹配。

### SSE 消息

SSE 推送消息必须包含 `plugin_id`：

```json
{
  "message_id": 123,
  "plugin_id": "shop",
  "message_type": "buttons",
  "content": "请选择商品",
  "rich_json": "..."
}
```

前端收到后：

- 如果 `plugin_id === activePluginId`，追加到当前消息区。
- 否则只更新左侧最近消息/未读状态，不插入当前对话。

## 路由与投递

需要新增指定插件执行能力，例如：

```go
HandleMessageForPlugin(msg, pluginID)
```

行为：

1. 校验插件存在、启用、运行中。
2. 校验当前 Web 用户有权限访问该插件。
3. 不进行全局 trigger 匹配。
4. 直接执行指定插件。
5. 插件回复保存为同一个 `plugin_id` 会话。

## 数据库方案

继续使用现有 `web_chat_messages` 表。该表已有 `plugin_id` 字段，应真正使用它。

新增索引迁移：

```sql
CREATE INDEX IF NOT EXISTS idx_web_chat_messages_user_plugin_message
ON web_chat_messages(user_id, plugin_id, message_id);
```

该索引用于：

```sql
SELECT *
FROM web_chat_messages
WHERE user_id = ?
  AND plugin_id = ?
  AND message_id > ?
ORDER BY message_id ASC
LIMIT ?;
```

SQLite 可支撑当前单机中小规模场景。约束：

- 不一次加载所有插件消息。
- 单次查询限制 50-100 条。
- 后续可加 Web 聊天消息保留天数清理。
- 保留用户消息限流，必要时改成 `user_id + plugin_id` 粒度。

## 快捷指令与动态按钮区分

### 固定快捷指令

来源于插件配置 `web_chat.quick_actions`。

特点：

- 进入插件会话即可看到。
- 固定不随上下文变化。
- 适合“商品列表”“我的订单”等常用入口。

### 动态按钮

来源于插件运行时 `ctx.sendButtons`。

特点：

- 绑定具体机器人回复。
- 可根据上下文动态变化。
- 适合商品选择、支付方式、确认/取消等场景。

两者都需要保留。

## 实施顺序

1. 保存并解析插件 `web_chat` 配置，扩展插件列表接口返回友好展示字段和快捷指令。
2. 前端左侧插件列表增加搜索，隐藏复杂 trigger，展示插件标题、描述和快捷指令。
3. 前端改成必须选中插件后才能聊天；切换插件加载对应消息历史。
4. `POST /messages`、`GET /messages` 支持 `plugin_id`，消息保存和查询按插件隔离。
5. 新增后端指定插件投递能力，用户输入直接发送给当前插件。
6. SSE 按 `plugin_id` 分流，当前会话追加，其他会话更新侧栏状态。
7. 增加 SQLite 索引迁移。
8. 补充后端和前端构建验证。

## 验收标准

- 左侧可搜索插件和快捷指令。
- 插件列表不再直接展示复杂正则规则。
- 点击不同插件，右侧消息历史独立。
- 在商城插件对话中输入内容只触发商城插件。
- 商城插件按钮和富文本在 Web 聊天中正常显示。
- 插件快捷指令可点击发送到当前插件会话。
- SQLite 查询走 `user_id + plugin_id + message_id` 索引。
