# AllBot 平台适配器开发说明

本文说明如何在 `core/adapter` 中新增一个 Go 原生平台适配器。适配器负责把外部平台的消息转换成 AllBot 内部消息，并把 AllBot 的回复发送回对应平台。

## 适配器职责

适配器只处理平台协议，不处理插件逻辑。

完整链路如下：

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
- 实现文本、图片、文件、用户信息、群信息、@用户等接口。
- 在 `Stop()` 中释放连接、停止 goroutine、清理等待中的请求。

适配器不应该负责：

- 插件匹配。
- 权限控制。
- 关键词回复。
- 定时任务。
- Web UI 配置保存。

这些逻辑由其他模块处理。

## 必须实现的接口

所有平台适配器都必须实现 `Adapter` 接口。

位置：`core/adapter/adapter.go`

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

## 消息结构

适配器收到平台消息后，需要转换成 `types.Message`。

位置：`core/types/types.go`

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
| `ID` | 平台消息 ID，无法获取时可用时间戳或平台事件 ID。 |
| `Platform` | 平台标识，例如 `qq`、`telegram`。必须和配置里的 `platform` 一致。 |
| `AdapterID` | 不需要适配器自己填，`AdapterManager` 会自动注入。 |
| `UserID` | 发送者用户 ID。 |
| `GroupID` | 群聊 ID。私聊时留空。 |
| `Content` | 文本内容。空内容通常不要继续分发。 |
| `Metadata` | 平台额外信息，例如 `chat_id`、`message_type`、`from_name`。 |

`AdapterManager` 会在消息进入 Router 前自动补充：

```go
msg.AdapterID = adapterIDText
msg.Metadata["adapter_id"] = adapterIDText
msg.Metadata["adapter_platform"] = platform
msg.Metadata["adapter_remark"] = remark
msg.Metadata["adapter_description"] = description
```

所以适配器只需要填平台自身需要的元数据。

## 文件命名建议

新增平台时，在 `core/adapter` 下创建对应文件：

```text
core/adapter/dingtalk_adapter.go
core/adapter/discord_adapter.go
core/adapter/feishu_adapter.go
```

类型命名建议：

```go
type DingTalkAdapter struct {}
func NewDingTalkAdapter(...) *DingTalkAdapter {}
```

## 基础实现模板

下面是一个最小结构模板。实际平台需要根据协议实现连接、收消息和发消息。

```go
package adapter

import (
    "fmt"
    "log"
    "sync"

    "github.com/allbot/allbot/core/types"
)

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
            // 读取到消息后调用 a.handleIncomingMessage(...)
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
    // 调用平台发送文本接口。
    return nil
}

func (a *ExampleAdapter) SendImage(target string, imageURL string) error {
    // 调用平台发送图片接口。
    return fmt.Errorf("Example 图片发送暂未实现")
}

func (a *ExampleAdapter) SendFile(target string, filePath string) error {
    // 调用平台发送文件接口。
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

不要在 `Start()` 里永久阻塞。错误示例：

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

`Stop()` 需要可以重复调用，避免重复关闭 channel 导致 panic。

建议使用：

```go
stopOnce sync.Once
```

如果适配器持有连接、请求等待表、定时器，需要在 `Stop()` 中释放。

参考 `QQAdapter.Stop()`：

- 关闭停止信号。
- 关闭连接。
- 清理 pending 请求。
- 删除运行中的适配器实例。

## 接收消息要求

接收平台消息时需要注意：

1. 只处理真实用户消息。
2. 忽略空文本。
3. 忽略自己发出的消息，避免自触发循环。
4. 群聊消息必须填 `GroupID`。
5. 私聊消息 `GroupID` 留空。
6. 平台特殊字段放入 `Metadata`。
7. 调用 `messageHandler` 前不要做耗时插件逻辑。

示例：

```go
msg := &types.Message{
    ID:       messageID,
    Platform: "telegram",
    UserID:   userID,
    Content:  text,
    Metadata: map[string]string{
        "chat_id": chatID,
        "from_name": fromName,
    },
}

if chatType == "group" || chatType == "supergroup" {
    msg.GroupID = chatID
}

if a.messageHandler != nil {
    a.messageHandler(msg)
}
```

## 发送消息要求

`SendMessage(target, text)` 是 Router 和插件最常用的发送入口。

`target` 的含义由适配器决定，但要保持稳定：

- Telegram 当前使用 `chat_id`。
- QQ 当前私聊使用用户 ID，群聊使用 `group_<groupID>`。

如果平台需要区分私聊和群聊，可以参考 QQ：

```go
messageType := "private"
targetID := target
if strings.HasPrefix(target, "group_") {
    messageType = "group"
    targetID = strings.TrimPrefix(target, "group_")
}
```

发送前建议记录日志：

```go
log.Printf("[发送][平台名][%s]：%s", target, text)
```

## 并发与超时要求

适配器通常会同时处理接收消息、发送消息、API 响应等待等逻辑，需要注意并发安全。

建议：

- 共享连接写入使用 `writeMu` 串行化。
- 共享 map 使用 `mu` 保护。
- API 请求等待必须设置超时。
- 网络客户端必须设置超时。
- 平台异常重试需要退避，避免刷屏和高频请求。
- `messageHandler` 可能触发插件执行，不要在持锁状态下调用。

错误示例：

```go
a.mu.Lock()
a.messageHandler(msg) // 不要持锁调用外部逻辑。
a.mu.Unlock()
```

正确示例：

```go
a.mu.Lock()
// 只读写适配器内部状态。
a.mu.Unlock()

if a.messageHandler != nil {
    a.messageHandler(msg)
}
```

## 日志规范

为了让 `/logs` 页面可读，建议使用统一日志格式。

接收消息：

```go
log.Printf("[接收][平台名][%s]：%s", userID, content)
```

发送消息：

```go
log.Printf("[发送][平台名][%s]：%s", target, text)
```

异常：

```go
log.Printf("[WARN][平台名] 获取消息异常: %v", err)
log.Printf("[INFO][平台名] 网络连接已恢复")
```

注意不要直接输出 token、access token、secret 等敏感字段。

## 配置接入步骤

新增适配器不只改 `core/adapter`，还需要接入配置和管理器。

### 1. 新增配置结构

位置：`core/config/models.go`

```go
type ExampleConfig struct {
    Token string `json:"token"`
    APIURL string `json:"api_url,omitempty"`
}
```

### 2. 新增配置解析函数

位置：`core/config/manager.go` 或配置解析相关文件。

```go
func ParseExampleConfig(config string) (*ExampleConfig, error) {
    var exampleConfig ExampleConfig
    if err := json.Unmarshal([]byte(config), &exampleConfig); err != nil {
        return nil, err
    }
    return &exampleConfig, nil
}
```

### 3. 在 AdapterManager 中注册平台

位置：`core/config/manager.go`

在 `startAdapter` 的 `switch config.Platform` 中增加分支：

```go
case "example":
    exampleConfig, err := ParseExampleConfig(config.Config)
    if err != nil {
        return fmt.Errorf("解析 Example 配置失败: %w", err)
    }
    adp = adapter.NewExampleAdapter(exampleConfig.Token, exampleConfig.APIURL)
```

### 4. 前端配置页面增加平台选项

如果 Web UI 需要配置该平台，需要在适配器页面增加对应平台和配置字段。

当前适配器配置最终会保存为 `AdapterConfig.Config` 字符串，后端再按 `platform` 解析成具体配置。

## 测试建议

新增适配器后至少验证：

1. 配置为空时启动失败，并返回明确错误。
2. `Start()` 能启动接收循环且不阻塞。
3. `Stop()` 可重复调用且不会 panic。
4. 收到私聊消息时能生成正确 `types.Message`。
5. 收到群聊消息时能填充 `GroupID`。
6. 空消息或自身消息不会进入 `messageHandler`。
7. `SendMessage()` 会按平台协议构造正确请求。
8. 网络/API 超时时能返回错误，不会永久卡住。

推荐命令：

```bash
go test ./core/adapter ./core/config
go test ./...
```

如果改动了 Go 代码，验证通过后再编译：

```bash
go build -o "D:/Desktop/program/java/AITest/allbot/allbot.exe" .
```

## 现有实现参考

| 文件 | 可参考内容 |
| --- | --- |
| `adapter.go` | 适配器接口和用户/群信息结构。 |
| `telegram_adapter.go` | Bot Token 校验、Webhook 清理、长轮询、发送文本、命令归一化。 |
| `qq_adapter.go` | WebSocket 连接、请求响应匹配、pending map、发送回声过滤、群聊 target 规则。 |
| `core/config/manager.go` | 适配器配置加载、启动、停止、消息元信息注入。 |
| `core/config/models.go` | 平台配置结构定义。 |

## 开发检查清单

新增适配器提交前确认：

- [ ] 实现了 `Adapter` 接口的全部方法。
- [ ] `GetPlatform()` 返回值和数据库配置 `platform` 一致。
- [ ] `Start()` 不阻塞主流程。
- [ ] `Stop()` 可重复调用。
- [ ] 网络请求和 API 等待都有超时。
- [ ] 共享状态有锁保护。
- [ ] 不持锁调用 `messageHandler`。
- [ ] 群聊消息正确填充 `GroupID`。
- [ ] 平台必要字段放入 `Metadata`。
- [ ] 日志不输出敏感凭据。
- [ ] 已在 `AdapterManager.startAdapter` 注册平台。
- [ ] 已补充配置结构和解析逻辑。
- [ ] 已运行本地测试。
