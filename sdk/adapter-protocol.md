# 自定义适配器协议 1

自定义适配器是由 AllBot 启动的 Node.js 或 Python 子进程。协议使用 UTF-8 JSON Lines：每一行是一个独立 JSON 对象；子进程 stdout 只能输出协议行，普通日志写 stderr。AllBot 不会为适配器另开监听端口。

AllBot 请求格式：

```json
{"id":"1","action":"send_text","data":{"target":"group_123","text":"hello"}}
```

成功响应：

```json
{"id":"1","ok":true,"data":{}}
```

失败响应：

```json
{"id":"1","ok":false,"error":"platform rejected message"}
```

子进程可在任意时间发送消息事件：

```json
{"event":"message","message":{"id":"m1","platform":"sample_node","user_id":"u1","nickname":"Name","group_id":"g1","content":"hello","medias":[],"media_only":false,"metadata":{"reply_target":"group_g1"}}}
```

`platform` 会由 AllBot 按清单 ID 覆盖，`adapter_id` 由 AllBot 注入。`metadata` 是字符串到字符串的映射，适合保存平台原始路由字段。接收消息至少提供 `user_id`、`content`，其他字段按平台场景提供。媒体 `type` 可用 `image`、`audio`、`video`、`file`。

AllBot 启动时先发送 `start`，`data.config` 是该实例的配置对象：

```json
{"id":"1","action":"start","data":{"protocol_version":"1","config":{"token":"..."}}}
```

适配器完成基本初始化后应尽快响应；长期连接和事件读取放在后台任务中。停止时收到 `stop`。请求 action 如下：

| action | data | `data` 响应 |
|---|---|---|
| `send_text` | `target`, `text` | `{}` |
| `send_image` | `target`, `url` | `{}` |
| `send_audio` | `target`, `url` | `{}` |
| `send_video` | `target`, `url` | `{}` |
| `send_file` | `target`, `path` | `{}` |
| `send_markdown` | `target`, `markdown` | `{}` |
| `send_rich` | `target`, `message` | `{}` |
| `send_buttons` | `target`, `text`, `buttons` | `{}` |
| `send_text_sequence` | `target`, `text`, `sequence` | `{}` |
| `get_user_info` | `user_id` | `user_id`, `nickname`, `avatar`, `extra` |
| `get_group_info` | `group_id` | `group_id`, `name`, `member_count`, `extra` |
| `at_user` | `group_id`, `user_id` | `{}` |
| `delete_message` | `message` | `{}` |
| `mute` | `group_id`, `user_id`, `duration_seconds` | `{}` |
| `http_callback` | `method`, `path`, `query`, `headers`, `body` | `status`, `headers`, `body` |

可选 action 只有在 `adapter.json` 的 `capabilities` 声明后才由 Host 调用；Webhook 回调需声明 `webhook`。当前 capability 名称为 `audio`、`video`、`markdown`、`rich`、`buttons`、`sequence`、`delete_message`、`mute`、`webhook`、`private`、`group`、`mention`。基础 `send_text`、`send_image`、`send_file`、用户/群信息和 @ 属于 Go 适配器基础接口。

声明 `webhook` 后，SDK 调用 `start(config, emit, context)`，其中 `context.callbackUrl`（Python 为 `context["callback_url"]`）是可注册到平台的完整 URL。AllBot 在已配置的 Web 监听端口上接收该 URL；每个实例 URL 都含随机令牌，令牌不放入用户配置且 API 不会返回。Node.js 实现 `handleHttpCallback(request)`，Python 实现 `handle_http_callback(request)`，返回 `{status, headers, body}`。`body` 当前按 UTF-8 文本处理，请求体上限为 2 MiB，回调处理超时为 25 秒。

Node.js SDK：`sdk/nodejs/allbot_adapter.js`，Python SDK：`sdk/python/allbot_adapter.py`。最小目录可参考 `examples/custom-adapters/`。运行时依赖 `node` 或 `python` 可执行文件在 AllBot 进程 PATH 中可用。
