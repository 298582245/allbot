# 用户自定义适配器规划

## 目标

允许用户通过独立适配器目录接入尚未内置的平台，不需要修改或重新编译 AllBot 的 Go 源码。适配器以 `adapter.json` 描述元信息和配置表单，以 `main` 入口实现平台连接和协议转换；首批 SDK 提供 Node.js 与 Python。自定义适配器运行期间不得要求 AllBot 新增监听端口。

## 当前架构依据

- Go 适配器契约位于 `core/adapter/_contract/contract.go`，基础接口包括平台标识、文本/图片/文件发送、用户/群信息、@、启动/停止和消息回调；另外通过可选接口声明音频、视频、Markdown、富文本、按钮、撤回、禁言、回复目标解析等能力。
- 适配器管理器位于 `core/config/manager.go`，负责依据平台注册信息创建适配器、注入适配器实例 ID 并将收到的统一消息交给路由。
- 插件管理器位于 `core/plugin/manager.go`。插件作为独立子进程运行，通过 stdin 接收 JSON 行、stdout 返回 JSON 行指令；Node.js/Python SDK 已有 Direct 模式实现。这是运行自定义代码并避免新增端口的可复用基础。
- 适配器配置表单当前由平台 manifest 的 schema 描述，前端从后端读取平台及配置字段；实例配置与平台定义需要区分。

## 建议方案

新增独立的用户适配器运行时，不把脚本解释器嵌入 Go，也不把第三方 Go 插件加载进主进程。每个适配器实例由 AllBot 启动一个受管子进程，通过版本化 JSON Lines 协议双向通信：AllBot 向适配器发起生命周期、消息发送和查询请求；适配器把平台收到的消息、状态和请求结果事件发回 AllBot。协议复用插件子进程的 stdio 通道模式，但适配器协议独立命名、独立版本化，不直接复用插件 action 集合。

```text
平台 API / WebSocket / 长轮询
          ↕
用户适配器子进程（main + Node.js/Python SDK）
          ↕ stdin/stdout JSON Lines（请求 ID、协议版本）
AllBot 自定义适配器 Host
          ↕
现有 AdapterManager → types.Message → Router
```

子进程隔离可避免用户脚本阻塞 Go 消息路由，也让用户可自行升级适配器代码；AllBot 版本升级只需保持协议兼容。运行时应提供启动超时、请求超时、崩溃重启策略、停止信号和日志隔离（协议只写 stdout，普通日志写 stderr），并限制消息大小和并发请求数。

## 目录与清单

建议运行目录按适配器定义分组、实例数据隔离：

```text
data/adapters/<adapter-id>/
  adapter.json
  adapter.data            # 适配器配置数据
  main.js                 # 或 main.py，由 runtime/entry 指定
  ...                     # 用户适配器自己的代码和依赖
```

`adapter.data` 与 `adapter.json`、入口文件放在同一个适配器目录，作为该适配器的配置数据文件，与插件目录中的 `plugin.data` 对齐。文件内容由 AllBot 按 schema 校验后写入，通过临时文件替换并限制文件权限；敏感字段在 API 返回和日志中必须脱敏。若同一适配器定义需要支持多个机器人账号，`adapter.data` 应以实例 ID 为键保存各实例配置，避免实例之间互相覆盖。数据库仍保存 AllBot 分配的实例 ID、启停状态、备注、排序等系统元数据；设计实现时避免把完整配置同时存入数据库和 `adapter.data` 造成双写不一致。

`adapter.json` 初版建议字段：

```json
{
  "id": "example-platform",
  "name": "Example 平台",
  "version": "1.0.0",
  "author": "作者",
  "description": "平台适配器说明",
  "runtime": "nodejs",
  "entry": "main.js",
  "protocol_version": "1",
  "config_schema": [
    { "key": "token", "label": "访问令牌", "type": "password", "required": true }
  ],
  "capabilities": ["text", "image", "file"]
}
```

其中 `id` 是稳定的平台/适配器类型标识，必须唯一且限制为安全字符；`name` 是前端显示名称。用户新增的每个机器人账号仍由 AllBot 分配独立的实例 ID，不能与 manifest 的适配器 ID 混为一谈。`runtime`、`entry` 用来启动用户代码；`protocol_version` 用于兼容性校验；配置字段沿用现有适配器 schema 的 `text/password/number/boolean/textarea/select` 类型，并允许 `required/default/placeholder/help/options` 等表单元数据。密钥只存放在对应实例的 `adapter.data`，不回写到 `adapter.json`。

用户提到的“适配器代码（针对前端的）”建议拆成两个概念：平台接入逻辑放 `main.js`/`main.py`；AllBot 前端仅通过 `adapter.json` 的 schema 动态生成实例配置表单。首期不加载任意前端 JavaScript/Vue 代码，避免用户代码进入管理员页面执行；若将来确需扩展界面，再设计受限的 iframe/声明式 UI 协议。

## 统一适配器能力

Host 在 Go 内实现现有 `adapter.Adapter`，并桥接 SDK 协议，使内置适配器和用户适配器都继续使用当前 Router，不让业务路由知道进程协议。

首版必须桥接基础契约：

| AllBot 能力 | 自定义协议/SDK 方向 |
|---|---|
| 启动、停止、健康状态 | 生命周期事件与运行状态 |
| 设置收到消息回调 | `message` 事件；字段对应 `types.Message`：平台、用户/群组、昵称、内容、媒体、消息 ID、metadata |
| 文本、图片、文件发送 | 带 request ID 的 `send_text`、`send_image`、`send_file` 请求及成功/失败响应 |
| 用户信息、群信息、@ 用户 | `get_user_info`、`get_group_info`、`at_user` 请求及结构化结果 |
| 回复目标与主动发送目标 | manifest/实例关联的平台标识，加 SDK 的目标对象；适配器可用消息 metadata 保留平台专有路由信息 |

可选能力通过 manifest 声明并由 Host 按能力接口桥接：音频、视频、Markdown、富文本、按钮、撤回、禁言、消息序号、机器人身份。没有声明的能力继续按现有语义跳过或 fallback，不得让不支持能力造成消息主链路失败。字段需要在协议文档中给出类型、可空语义、错误格式和样例。

“暴露基础适配器功能”指暴露 AllBot 的适配器契约及其可选能力，不等于把 Go 内部对象或数据库直接交给用户脚本。SDK 应提供类型化方法和统一错误对象；平台原始字段由 `metadata` 扩展承载。这样用户脚本只依赖稳定协议，不依赖 Go 结构布局。

## 无新增端口的回调设计

普通接入优先采用适配器主动连接平台的 WebSocket、长轮询或平台 SDK，不涉及 AllBot 入站端口。若平台强制要求 Webhook，Host 应在现有 Web 服务上注册受控回调路由，例如基于适配器实例的 `/api/adapter-callback/<token>`；这仍使用已经配置的 Web 监听端口，不额外监听端口。实现前需确认现有反向代理/鉴权/CSRF 路由约定，并为回调令牌提供随机生成、轮换、常量时间比较、请求大小限制和速率限制。Webhook 不可直接映射到用户脚本提供的任意路径。

## SDK 与示例

建议新增：

```text
sdk/adapter-protocol.md
sdk/nodejs/allbot_adapter.js
sdk/python/allbot_adapter.py
examples/custom-adapters/nodejs-example/
  adapter.json
  main.js
examples/custom-adapters/python-example/
  adapter.json
  main.py
```

SDK 封装协议读写、request ID 匹配、消息事件分发、生命周期、日志输出、超时/错误映射和能力声明。示例使用虚构平台或可本地模拟的平台接口，展示连接、收到消息转换、回复消息及用户配置读取。SDK 目录现有文件主要服务插件，应保持向后兼容；适配器入口另设模块，避免将插件 API 与适配器 API 混在一起。

## 后端与前端实施阶段

1. **协议定稿**：定义 manifest schema、协议版本、请求/响应/事件、错误语义、消息结构和能力映射；补充兼容策略。
2. **Go Host 与注册发现**：扫描适配器目录、校验 manifest 和入口路径、向平台目录 API 暴露动态适配器、实现进程管理及 `adapter.Adapter` 桥接。不得改变默认监听地址或新增端口。
3. **配置持久化与生命周期**：支持创建/编辑/启停/删除适配器实例，存储 adapter type ID、AllBot 实例 ID、启用状态、备注和用户参数；删除/升级时处理运行进程和配置迁移。
4. **Node.js/Python SDK 与示例**：按同一协议实现两种 SDK，并提供可运行的最小样例和协议测试工具。
5. **前端管理**：扩展现有适配器管理页，显示名称、类型 ID、版本、作者、介绍、状态；基于 schema 展示用户需填参数。安装/导入目录能力与实例配置分开，避免把代码编辑器和密钥表单混成一个配置对象。
6. **安全与运维**：限制目录穿越、入口文件、依赖执行范围、子进程资源和日志泄漏；增加运行状态、启动错误、重启和卸载行为说明。

## 主要改动位置预估

- `core/adapter/`：协议 Host、外部适配器桥接与目录扫描；Go AdapterManager 仍统一管理实例。
- `core/config/`：适配器类型定义和实例参数保存/读取；评估是否需要迁移数据库字段，尽量沿用 `Platform` 作为类型 ID。
- `core/web/`：适配器目录/元数据 API、运行状态，以及复用现有 Web Server 的 Webhook 路由。
- `web-ui/src/views/Adapters.vue` 与 API 封装：动态显示用户适配器并渲染 schema 表单。
- `sdk/` 与 `examples/`：协议文档、Node.js/Python SDK、样例。

具体文件需在协议和配置模型确定后再定；本规划不预设数据库字段名或 HTTP 路由最终形式。

## 风险与需要在实施前验证的点

- 现有主程序可嵌入 Web 静态文件，但自定义适配器代码应放在可写数据目录，不应依赖升级包内文件。
- 多个实例可能共享一个适配器目录，但配置、进程、日志和回调凭据必须按实例隔离。
- 运行时依赖是否随 AllBot 依赖管理器安装、还是用户自行安装，需要产品决策；SDK 首版应明确 Node/Python 可执行文件发现规则。
- Webhook 平台需要公网可达的现有 Web 服务地址；“不新增端口”不能替代网络反向代理配置。
- 自定义脚本拥有与 AllBot 进程用户相近的系统权限。子进程隔离能提升故障隔离，但不是强安全沙箱；管理员应明确安装和运行来源。
- 配置表单 schema 需兼容未知字段，且密码字段在回读 API 时应脱敏，避免前端重新加载时泄漏密钥。

## 建议验收标准

- 用户安装一个只含 `adapter.json`、入口代码的 Node.js 或 Python 适配器后，可在现有适配器页面创建多个实例并启动，不需重编译 Go。
- 收到的消息按统一 `types.Message` 进入现有路由；插件、关键词回复和内置指令无需新增自定义适配器分支。
- 现有 Router 可通过 Host 调用文本、图片、文件发送和声明的可选能力；不支持能力遵循现有跳过/fallback 约定。
- 进程异常退出有可见状态和错误日志，停止/删除实例能终止进程并释放等待中的请求。
- 协议只经 stdin/stdout 通信；Webhook（如实现）复用当前 Web Server 端口。
- 两种 SDK 使用同一协议测试集，旧插件 SDK 与插件运行方式不受影响。
