# AllBot 平台适配器开发说明

本文说明如何新增 Go 原生平台适配器。当前适配器已经采用目录化、manifest 注册和 loader 自动导入机制：新增平台时，原则上只需要在 `core/adapter/<platform>/` 下实现适配器和 `manifest.go`，再运行 `go generate ./core/adapter/_loader`。

## 当前目录结构

```text
core/adapter/
  adapter.go          # 兼容入口，导出 Adapter/UserInfo/GroupInfo 等类型别名
  _contract/          # 适配器接口契约，不是真实适配器
  _registry/          # 适配器注册中心，不是真实适配器
  _loader/            # 自动生成真实适配器 blank import，不是真实适配器
  _builtin/           # 兼容入口，转入 _loader，不是真实适配器
  qq/                 # QQ 适配器
  qq_office/          # QQ 官方机器人适配器
  telegram/           # Telegram 适配器
```

约定：

- `_` 开头目录是基础设施，不是真实适配器。
- 不以 `_` 开头且包含 `manifest.go` 的目录会被 `_loader` 识别为真实适配器。
- 新增适配器目录名必须和平台标识一致，例如 `discord`、`dingtalk`。

## 新增适配器流程

1. 新建目录：

```text
core/adapter/example/
```

2. 编写：

```text
core/adapter/example/adapter.go
core/adapter/example/manifest.go
core/adapter/example/adapter_test.go
```

3. 运行 loader 生成：

```bash
go generate ./core/adapter/_loader
```

4. 运行验证：

```bash
go test ./core/adapter/_loader ./core/adapter/example
go test ./...
go build -o "D:/Desktop/program/java/AITest/allbot/allbot.exe" .
```

新增适配器后不应该修改：

```text
core/config/manager.go
core/router/router.go
web-ui/src/views/Adapters.vue
web-ui/src/views/Plugins.vue
```

这些模块会通过 registry、manifest、配置 schema 和能力接口自动识别平台。

## 适配器职责

适配器只处理平台协议，不处理插件逻辑。

完整链路：

```text
平台消息
  ↓
平台适配器
  ↓
types.Message
  ↓
AdapterManager 注入 adapter_id 等元信息
  ↓
Router 分发给插件 / 内置回复
  ↓
适配器 SendMessage / SendImage / SendFile 发回平台
```

适配器需要负责：

- 连接平台或平台网关。
- 接收平台消息。
- 过滤空消息、自己发出的消息、无效事件。
- 转换成 `types.Message`。
- 调用 `messageHandler(msg)` 交给主路由。
- 实现文本、图片、文件、用户信息、群信息、@ 用户等接口。
- 声明 manifest：平台名、显示名、配置 schema、能力、配置解析器和构造器。
- 在 `Stop()` 中释放连接、停止 goroutine、清理等待中的请求。

适配器不应该负责：

- 插件匹配。
- 权限控制。
- 关键词回复。
- 定时任务。
- Web UI 配置保存。
- 修改插件平台复选框。

这些逻辑由其他模块基于 manifest 自动处理。

## 必须实现的接口

所有平台适配器都必须实现 `Adapter` 接口。

接口位置：

```text
core/adapter/_contract/contract.go
```

兼容引用位置：

```text
core/adapter/adapter.go
```

接口：

```go
type Adapter interface {
    GetPlatform() string
    SendMessage(target string, text string) error
    SendImage(target string, imageURL string) error
    SendFile(target string, filePath string) error
    GetUserInfo(userID string) (*UserInfo, error)
    GetGroupInfo(groupID string) (*GroupInfo, error)
    AtUser(groupID string, userID string) error
    Start() error
    Stop() error
    SetMessageHandler(handler func(*types.Message))
}
```

适配器子包里可以这样引用类型：

```go
import "github.com/allbot/allbot/core/adapter/_contract"

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo
```

## 可选能力接口

Router 不再写平台特殊逻辑。平台差异应通过可选能力接口下沉到适配器。

### ReplyTargetResolver

用于处理“回复收到的消息”时的目标格式。

```go
type ReplyTargetResolver interface {
    ReplyTarget(msg *types.Message) string
}
```

示例：

- QQ 群聊返回 `group_<groupID>`。
- Telegram 优先返回 `Metadata["chat_id"]`。
- QQ 官方优先返回 `Metadata["reply_target"]`，再 fallback 到 `group_`、`user_`、`dms_`。

### ReplyTextFormatter

用于处理群聊回复文本格式，例如 @ 用户。

```go
type ReplyTextFormatter interface {
    FormatReplyText(msg *types.Message, text string) string
}
```

示例：

- QQ 群聊拼接 `[CQ:at,qq=<userID>]`。
- Telegram 群聊拼接 HTML mention。
- QQ 官方保持原文，不拼 CQ 码或文本 @。

### SendTargetResolver

用于处理插件主动发送消息时的目标格式。

```go
type SendTargetResolver interface {
    SendTarget(userID string, groupID string) string
}
```

示例：

- QQ 群聊返回 `group_<groupID>`。
- Telegram 群聊返回 `groupID`。
- QQ 官方支持 `dms_`、`user_`、`group_` 前缀。

## 消息结构

适配器收到平台消息后，需要转换成 `types.Message`。

位置：

```text
core/types/types.go
```

结构：

```go
type Message struct {
    ID        string
    Platform  string
    AdapterID string
    UserID    string
    GroupID   string
    Content   string
    Metadata  map[string]string
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `ID` | 平台消息 ID。无法获取时可用平台事件 ID 或时间戳。 |
| `Platform` | 平台标识，必须和 manifest 里的 `Platform` 一致。 |
| `AdapterID` | 适配器不需要自己填，`AdapterManager` 会自动注入。 |
| `UserID` | 发送者用户 ID。 |
| `GroupID` | 群聊 ID。私聊时留空。 |
| `Content` | 文本内容。空内容通常不要继续分发。 |
| `Metadata` | 平台额外信息，例如 `chat_id`、`message_type`、`from_name`、`reply_target`。 |

`AdapterManager` 会在消息进入 Router 前自动补充：

```go
msg.AdapterID = adapterIDText
msg.Metadata["adapter_id"] = adapterIDText
msg.Metadata["adapter_platform"] = platform
msg.Metadata["adapter_remark"] = remark
msg.Metadata["adapter_description"] = description
```

适配器只需要填平台自身需要的元数据。

## manifest.go 写法

每个真实适配器目录必须有 `manifest.go`。

示例：

```go
package example

import (
    "encoding/json"
    "fmt"
    "strings"

    "github.com/allbot/allbot/core/adapter/_contract"
    "github.com/allbot/allbot/core/adapter/_registry"
)

type Config struct {
    Token string `json:"token"`
}

func init() {
    registry.Register(registry.Descriptor{
        Platform:    "example",
        DisplayName: "Example",
        Description: "Example 平台适配器",
        ConfigSchema: []registry.ConfigField{
            {
                Key:      "token",
                Label:    "Token",
                Type:     "password",
                Required: true,
                Help:     "Example 平台访问令牌",
            },
        },
        Capabilities: registry.Capabilities{
            SendText:       true,
            SendImage:      false,
            SendFile:       false,
            PrivateMessage: true,
            GroupMessage:   true,
            Mention:        false,
        },
        ParseConfig: parseConfigForRegistry,
        NewAdapter:  newAdapterFromRegistry,
    })
}

func parseConfigForRegistry(raw string) (interface{}, error) {
    var config Config
    if err := json.Unmarshal([]byte(raw), &config); err != nil {
        return nil, err
    }
    config.Token = strings.TrimSpace(config.Token)
    if config.Token == "" {
        return nil, fmt.Errorf("token 不能为空")
    }
    return &config, nil
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
    config, ok := parsed.(*Config)
    if !ok {
        return nil, fmt.Errorf("Example 配置类型错误: %T", parsed)
    }
    return NewExampleAdapter(config.Token), nil
}
```

manifest 会被 `_loader` 自动导入，`init()` 执行后平台会进入 registry。

## 配置 schema 字段

`ConfigSchema` 会提供给后端接口和 Web UI。

支持字段类型：

```text
text
password
number
boolean
textarea
select
```

字段定义：

```go
type ConfigField struct {
    Key         string      `json:"key"`
    Label       string      `json:"label"`
    Type        string      `json:"type"`
    Required    bool        `json:"required"`
    Placeholder string      `json:"placeholder,omitempty"`
    Default     interface{} `json:"default,omitempty"`
    Help        string      `json:"help,omitempty"`
}
```

注意：

- `Key` 必须和配置 JSON 字段一致。
- `Required` 会用于前端校验。
- `password` 只影响前端展示，不代表后端加密。
- schema 只描述表单，不应该包含真实密钥。

## adapter.go 基础模板

```go
package example

import (
    "fmt"
    "log"
    "sync"

    "github.com/allbot/allbot/core/adapter/_contract"
    "github.com/allbot/allbot/core/types"
)

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo

type ExampleAdapter struct {
    token          string
    messageHandler func(*types.Message)
    stopChan       chan struct{}
    stopOnce       sync.Once
}

func NewExampleAdapter(token string) *ExampleAdapter {
    return &ExampleAdapter{
        token:    token,
        stopChan: make(chan struct{}),
    }
}

func (a *ExampleAdapter) GetPlatform() string {
    return "example"
}

func (a *ExampleAdapter) SetMessageHandler(handler func(*types.Message)) {
    a.messageHandler = handler
}

func (a *ExampleAdapter) Start() error {
    if a.token == "" {
        return fmt.Errorf("Example token 不能为空")
    }
    go a.listen()
    log.Printf("Example Adapter 已启动")
    return nil
}

func (a *ExampleAdapter) Stop() error {
    a.stopOnce.Do(func() {
        close(a.stopChan)
    })
    log.Printf("Example Adapter 已停止")
    return nil
}

func (a *ExampleAdapter) listen() {
    for {
        select {
        case <-a.stopChan:
            return
        default:
            // 从平台读取消息。
        }
    }
}

func (a *ExampleAdapter) handleIncomingMessage(messageID, userID, groupID, content string) {
    if content == "" {
        return
    }
    msg := &types.Message{
        ID:       messageID,
        Platform: "example",
        UserID:   userID,
        GroupID:  groupID,
        Content:  content,
        Metadata: map[string]string{},
    }
    log.Printf("[接收][Example][%s]：%s", userID, content)
    if a.messageHandler != nil {
        a.messageHandler(msg)
    }
}

func (a *ExampleAdapter) SendMessage(target string, text string) error {
    log.Printf("[发送][Example][%s]：%s", target, text)
    return nil
}

func (a *ExampleAdapter) SendImage(target string, imageURL string) error {
    return fmt.Errorf("Example 图片发送暂未实现")
}

func (a *ExampleAdapter) SendFile(target string, filePath string) error {
    return fmt.Errorf("Example 文件发送暂未实现")
}

func (a *ExampleAdapter) GetUserInfo(userID string) (*UserInfo, error) {
    return &UserInfo{UserID: userID, Extra: map[string]string{}}, nil
}

func (a *ExampleAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
    return &GroupInfo{GroupID: groupID, Extra: map[string]string{}}, nil
}

func (a *ExampleAdapter) AtUser(groupID string, userID string) error {
    return a.SendMessage(groupID, "@"+userID)
}
```

## Start 实现要求

`Start()` 是适配器启动入口。

建议做这些事：

1. 校验配置是否完整。
2. 建立平台连接或校验平台凭据。
3. 启动接收消息的 goroutine。
4. 返回启动错误，方便后台显示失败原因。

不要在 `Start()` 里永久阻塞。

错误示例：

```go
func (a *ExampleAdapter) Start() error {
    for {
        // 永久循环会阻塞 AllBot 启动流程。
    }
}
```

正确示例：

```go
func (a *ExampleAdapter) Start() error {
    go a.listen()
    return nil
}
```

## Stop 实现要求

`Stop()` 必须可重复调用，不应该因为重复停止 panic。

建议使用：

```go
type ExampleAdapter struct {
    stopChan chan struct{}
    stopOnce sync.Once
}

func (a *ExampleAdapter) Stop() error {
    a.stopOnce.Do(func() {
        close(a.stopChan)
    })
    return nil
}
```

如果适配器有连接、HTTP 长轮询、WebSocket、等待响应的 channel，也要在 `Stop()` 中释放。

## target 格式建议

`SendMessage(target, text)` 中的 `target` 是适配器内部目标格式。

适配器可以自行定义，例如：

```text
user_<userID>
group_<groupID>
dms_<guildID>
<chatID>
```

但需要实现 `ReplyTargetResolver` 和 `SendTargetResolver`，让 Router 不需要知道平台细节。

推荐：

- 私聊：使用 `user_<id>` 或平台原始用户 ID。
- 群聊：使用 `group_<id>` 或平台原始群 ID。
- 特殊回复：在 `Metadata["reply_target"]` 中写完整目标。

## Metadata 建议

通用字段：

| 字段 | 说明 |
| --- | --- |
| `message_type` | `private`、`group`、`c2c`、`dms` 等。 |
| `reply_target` | 适配器完整回复目标。优先级最高。 |
| `from_name` | 发送者显示名，Telegram mention 等场景可用。 |
| `chat_id` | Telegram 等平台原始 chat ID。 |

平台私有字段建议加平台前缀，例如：

```text
qq_office_msg_id
qq_office_user_openid
qq_office_group_openid
```

## loader 生成规则

`core/adapter/_loader/generate.go` 会扫描：

```text
core/adapter/*/manifest.go
```

跳过：

```text
_ 开头目录
. 开头目录
```

生成：

```text
core/adapter/_loader/loader_gen.go
```

示例生成结果：

```go
package loader

import (
    _ "github.com/allbot/allbot/core/adapter/example"
    _ "github.com/allbot/allbot/core/adapter/qq"
)
```

新增、删除或重命名适配器目录后必须运行：

```bash
go generate ./core/adapter/_loader
```

## 验证要求

新增适配器后至少执行：

```bash
go generate ./core/adapter/_loader
go test ./core/adapter/_loader ./core/adapter/<platform>
go test ./...
go build -o "D:/Desktop/program/java/AITest/allbot/allbot.exe" .
```

如果改了 Web UI 或前端资源，还要执行：

```bash
npm --prefix "D:/Desktop/program/java/AITest/allbot/web-ui" run build
```

## 常见错误

### 忘记运行 go generate

现象：

- `GET /api/adapter-platforms` 看不到新平台。
- `manager.go` 报 `不支持的平台`。

解决：

```bash
go generate ./core/adapter/_loader
```

### manifest.go 缺失

`_loader` 只扫描有 `manifest.go` 的真实适配器目录。没有 manifest 的目录不会被注册。

### import cycle

不要让适配器目录 import `_builtin` 或 `_loader`。

推荐依赖方向：

```text
适配器目录 -> _contract
适配器目录 -> _registry
_loader -> 适配器目录
main/web -> _loader
```

### 在 Router 中加平台判断

不要为了新平台修改 `router.go` 的平台分支。应该实现：

```go
ReplyTargetResolver
ReplyTextFormatter
SendTargetResolver
```

### 在 Web UI 中写死平台

不要为了新平台修改：

```text
web-ui/src/views/Adapters.vue
web-ui/src/views/Plugins.vue
```

平台展示名和配置表单来自 manifest 的 `DisplayName` 和 `ConfigSchema`。
