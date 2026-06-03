# AllBot 适配器插件化重构任务清单

## 目标

让新增适配器时只需要在 `core/adapter/` 下新增适配器实现和声明文件，不再到处修改 `core/config/manager.go`、`core/router/router.go`、`web-ui/src/views/Adapters.vue`、`web-ui/src/views/Plugins.vue` 等业务文件。

最终新增适配器的理想流程：

1. 新建 `core/adapter/<platform>/` 目录。
2. 编写适配器实现、配置解析、平台 manifest。
3. 执行注册生成或更新 loader。
4. 执行测试和编译。
5. 后端自动识别平台，前端自动显示配置表单和插件平台选项。

## 总体设计

采用：

- 适配器注册中心
- 平台 manifest
- 配置 schema
- 适配器能力接口
- Web UI 动态平台列表
- 后续再迁移为一个适配器一个目录

不优先采用外部进程式动态适配器，避免一次性引入过大的进程管理、RPC 协议和 Windows 兼容成本。

---

## 阶段 1：建立适配器注册中心（已完成）

### 1.1 新增适配器描述结构

新增目录：

```text
core/adapter/registry/
```

新增文件：

```text
core/adapter/registry/descriptor.go
core/adapter/registry/registry.go
```

需要定义：

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

type Capabilities struct {
    SendText       bool `json:"send_text"`
    SendImage      bool `json:"send_image"`
    SendFile       bool `json:"send_file"`
    PrivateMessage bool `json:"private_message"`
    GroupMessage   bool `json:"group_message"`
    Mention        bool `json:"mention"`
}

type Descriptor struct {
    Platform     string        `json:"platform"`
    DisplayName  string        `json:"display_name"`
    Description  string        `json:"description"`
    ConfigSchema []ConfigField `json:"config_schema"`
    Capabilities Capabilities `json:"capabilities"`

    ParseConfig func(raw string) (interface{}, error) `json:"-"`
    NewAdapter  func(config interface{}) (adapter.Adapter, error) `json:"-"`
}
```

注意事项：

- 需要避免 import cycle。
- 如果 `registry` 引用 `adapter.Adapter` 导致循环，需要把接口迁移到 `core/adapter/contract`，或者先把 registry 放在 `core/adapter` 包内做过渡。
- 第一阶段建议尽量小步实现，不立刻大规模移动文件。

### 1.2 实现注册方法

需要提供：

```go
func Register(desc Descriptor)
func Get(platform string) (Descriptor, bool)
func List() []Descriptor
```

要求：

- `platform` 不能为空。
- 重复注册同一 platform 要 panic 或返回明确错误。
- `List()` 返回稳定顺序，建议按 `DisplayName` 或 `Platform` 排序。

### 1.3 验证

新增或更新测试：

```text
core/adapter/registry/registry_test.go
```

覆盖：

- 注册成功。
- 查询成功。
- 查询不存在返回 false。
- 重复注册有明确行为。
- 列表顺序稳定。

---

## 阶段 2：适配器配置和创建走注册中心

### 2.1 给现有适配器补 manifest 注册

先不移动文件，仅为现有平台补注册入口。

涉及平台：

```text
qq
telegram
qq_office
```

建议新增：

```text
core/adapter/manifest_qq.go
core/adapter/manifest_telegram.go
core/adapter/manifest_qq_office.go
```

每个 manifest 提供：

- `Platform`
- `DisplayName`
- `Description`
- `ConfigSchema`
- `Capabilities`
- `ParseConfig`
- `NewAdapter`

### 2.2 统一配置解析

现状：

- `core/config/database.go` 有 `ParseQQConfig`、`ParseTelegramConfig`、`ParseQQOfficeConfig`。
- `core/config/manager.go` 按 platform switch 创建适配器。

目标：

- 注册中心负责从 platform 找到解析器和构造器。
- `manager.go` 不再出现新增平台专用 case。

可能步骤：

1. 将现有解析函数保留在 `core/config/database.go`，manifest 暂时引用它。
2. `manager.go` 改为：

```go
desc, ok := registry.Get(config.Platform)
if !ok {
    return fmt.Errorf("不支持的平台: %s", config.Platform)
}
parsed, err := desc.ParseConfig(config.Config)
if err != nil {
    return fmt.Errorf("解析 %s 配置失败: %w", desc.DisplayName, err)
}
adp, err := desc.NewAdapter(parsed)
if err != nil {
    return err
}
```

3. 后续阶段再把各平台 config 迁入适配器目录。

### 2.3 验证

运行：

```bash
go test ./core/config ./core/adapter
```

检查：

- QQ 适配器仍能启动。
- Telegram 适配器仍能启动。
- QQ 官方适配器仍能启动。
- 未知平台返回明确错误。

---

## 阶段 3：Router 去平台硬编码

### 3.1 新增可选能力接口

建议新增在适配器接口附近：

```text
core/adapter/adapter.go
```

新增：

```go
type ReplyTargetResolver interface {
    ReplyTarget(msg *types.Message) string
}

type ReplyTextFormatter interface {
    FormatReplyText(msg *types.Message, text string) string
}
```

含义：

- `ReplyTargetResolver`：适配器自己决定如何从消息得到回复目标。
- `ReplyTextFormatter`：适配器自己决定群聊是否加 @、用什么格式加 @。

### 3.2 现有适配器实现能力接口

QQ：

- 群聊回复目标用 `GroupID`。
- 私聊回复目标用 `UserID`。
- 群聊文本前加 CQ at：`[CQ:at,qq=<user>]`。

Telegram：

- 群聊回复目标用 `GroupID`。
- 私聊回复目标用 `UserID`。
- 群聊文本使用 HTML mention。

QQ 官方：

- 优先使用 `msg.Metadata["reply_target"]`。
- 其次 group 用 `group_<group_openid>`。
- 其次 user 用 `user_<user_openid>`。
- 不额外拼 CQ 码或 @ 文本。

### 3.3 修改 Router 调用点

替换现有：

```go
replyTarget(msg)
mentionReplyText(msg, text)
```

改成：

```go
target := resolveReplyTarget(adp, msg)
text := formatReplyText(adp, msg, text)
```

保留默认 fallback，保证未实现接口的适配器仍可工作。

### 3.4 清理平台判断

逐步删除 `router.go`、`keyword_reply.go` 中类似：

```go
if msg.Platform == "qq_office" { ... }
switch msg.Platform { ... }
```

目标：

- 平台差异进入适配器内部。
- Router 只负责插件匹配和调用发送能力。

### 3.5 验证

运行：

```bash
go test ./core/router ./core/adapter
```

重点测试：

- QQ 群聊仍加 CQ at。
- QQ 官方群聊不加 CQ at。
- QQ 官方 C2C 使用 `user_xxx|msg_xxx`。
- QQ 官方群聊使用 `group_xxx|msg_xxx`。
- Telegram 群聊 mention 不回退。

---

## 阶段 4：后端暴露平台列表接口

### 4.1 新增接口

新增后端 API：

```text
GET /api/adapter-platforms
```

返回注册中心中的平台描述。

示例：

```json
[
  {
    "platform": "qq_office",
    "display_name": "QQ 官方机器人",
    "description": "腾讯 QQ 官方机器人",
    "config_schema": [
      {
        "key": "app_id",
        "label": "App ID",
        "type": "text",
        "required": true
      },
      {
        "key": "client_secret",
        "label": "Client Secret",
        "type": "password",
        "required": true
      }
    ],
    "capabilities": {
      "send_text": true,
      "send_image": true,
      "send_file": false,
      "private_message": true,
      "group_message": true
    }
  }
]
```

### 4.2 注意安全和脱敏

- API 只返回 schema，不返回真实配置值。
- `password` 类型字段只影响前端渲染。

### 4.3 验证

新增测试：

```text
core/web/adapter_platforms_test.go
```

覆盖：

- 接口返回 200。
- 包含现有平台。
- 不包含函数类型字段。
- QQ 官方包含 app_id / client_secret schema。

---

## 阶段 5：Web UI 适配器配置页动态化

### 5.1 修改 `web-ui/src/api/index.js`

新增：

```js
export const getAdapterPlatforms = () => request.get('/adapter-platforms')
```

### 5.2 修改 `web-ui/src/views/Adapters.vue`

目标：

- 平台下拉选项来自 `/api/adapter-platforms`。
- 配置表单根据 `config_schema` 自动渲染。
- 不再硬编码 QQ / Telegram / QQ 官方配置字段。

字段类型建议支持：

```text
text
password
number
boolean
textarea
select
```

第一阶段至少支持：

```text
text
password
```

### 5.3 配置 JSON 兼容

现有数据库中配置仍是 JSON 字符串，例如：

```json
{
  "app_id": "xxx",
  "client_secret": "xxx"
}
```

前端保存时仍提交同样结构，不改变数据库表结构。

### 5.4 验证

运行：

```bash
npm --prefix "D:/Desktop/program/java/AITest/allbot/web-ui" run build
```

手工检查：

- 新建 QQ 官方机器人适配器时字段正确。
- 编辑已有适配器时已有字段能回显。
- QQ / Telegram 原有配置不丢失。

---

## 阶段 6：Web UI 插件平台配置动态化

### 6.1 修改 `web-ui/src/views/Plugins.vue`

目标：

- 插件支持平台复选框来自 `/api/adapter-platforms`。
- 平台标签显示名来自注册中心。
- 新建插件默认平台可由后端 template defaults 提供。

需要替换：

```js
const pluginPlatformOptions = [...]
const pluginPlatformNames = ...
```

改为：

```js
const pluginPlatformOptions = ref([])
const pluginPlatformNames = computed(...)
```

### 6.2 默认平台策略

建议：

- 后端 `plugin_create.go` 继续提供默认 platforms。
- 默认 platforms 可从 registry 中筛选 `Capabilities.SendText == true` 的平台。
- 当前默认可保持：`qq`、`qq_office`、`telegram`。

### 6.3 验证

运行：

```bash
npm --prefix "D:/Desktop/program/java/AITest/allbot/web-ui" run build
```

手工检查：

- `/plugins` 平台复选框包含注册中心平台。
- 现有插件的 `platforms` 能正常显示。
- 保存后 `plugin.json` 正确写入平台数组。

---

## 阶段 7：插件创建默认平台改为 registry 驱动

### 7.1 修改 `core/web/plugin_create.go`

现状可能有：

```go
var defaultPluginPlatforms = []string{"qq", "qq_office", "telegram"}
```

目标：

```go
func defaultPluginPlatforms() []string {
    platforms := registry.List()
    result := make([]string, 0)
    for _, item := range platforms {
        if item.Capabilities.SendText {
            result = append(result, item.Platform)
        }
    }
    return result
}
```

如果需要排除某些平台，可以在 Descriptor 加：

```go
DefaultForPlugins bool
```

### 7.2 验证

运行：

```bash
go test ./core/web
```

检查：

- 新建插件默认平台包含 QQ 官方。
- 后端模板接口返回 defaults.platforms。

---

## 阶段 8：适配器目录化迁移

### 8.1 目标结构

将：

```text
core/adapter/qq_adapter.go
core/adapter/telegram_adapter.go
core/adapter/qq_office_adapter.go
```

逐步迁移为：

```text
core/adapter/qq/
  adapter.go
  config.go
  manifest.go
  target.go
  adapter_test.go

core/adapter/telegram/
  adapter.go
  config.go
  manifest.go
  target.go
  adapter_test.go

core/adapter/qq_office/
  adapter.go
  config.go
  manifest.go
  token.go
  gateway.go
  event.go
  target.go
  media.go
  api.go
  adapter_test.go
```

### 8.2 迁移原则

- 一个平台一个提交点或一个阶段。
- 先迁移 QQ 官方，因为它最复杂且最能验证新结构。
- 每迁移一个平台都跑对应测试。
- 保持 public 行为不变。

### 8.3 兼容导入路径

若已有代码依赖 `adapter.NewQQOfficeAdapter`，需要同步改为 registry 创建，避免保留旧 re-export。

---

## 阶段 9：生成 adapter loader

### 9.1 背景

Go 不会自动编译子目录包。即使新增：

```text
core/adapter/discord/
```

也必须被某处 import，init 注册才会执行。

### 9.2 新增 loader 生成器

新增：

```text
core/adapter/loader/generate.go
core/adapter/loader/loader_gen.go
```

生成逻辑：

- 扫描 `core/adapter/*/manifest.go`。
- 跳过 `_template`、`registry`、`contract`、`loader` 等目录。
- 生成 blank import：

```go
package loader

import (
    _ "github.com/allbot/allbot/core/adapter/qq"
    _ "github.com/allbot/allbot/core/adapter/telegram"
    _ "github.com/allbot/allbot/core/adapter/qq_office"
)
```

### 9.3 主程序固定引入 loader

只需要在合适位置永久引入一次：

```go
import _ "github.com/allbot/allbot/core/adapter/loader"
```

之后新增适配器无需再改业务代码，只需要重新生成 loader。

### 9.4 验证

运行：

```bash
go generate ./core/adapter/loader
go test ./...
go build -o "D:/Desktop/program/java/AITest/allbot/allbot.exe" .
```

---

## 阶段 10：适配器开发模板和文档

### 10.1 新增模板目录

新增：

```text
core/adapter/_template/
  adapter.go
  config.go
  manifest.go
  target.go
  adapter_test.go
```

模板说明：

- 如何定义 platform。
- 如何实现 `Adapter` 接口。
- 如何声明配置 schema。
- 如何实现 `ReplyTargetResolver`。
- 如何实现 `ReplyTextFormatter`。
- 如何注册 manifest。

### 10.2 开发者流程

文档中写明：

```text
1. 复制 core/adapter/_template 为 core/adapter/<platform>
2. 修改 manifest.go
3. 实现 adapter.go
4. 实现配置解析
5. 补测试
6. 运行 go generate ./core/adapter/loader
7. 运行 go test ./...
8. 编译 allbot.exe
```

### 10.3 注意事项

- 不要直接改 `router.go` 添加平台判断。
- 不要直接改 `Adapters.vue` 添加平台字段。
- 不要直接改 `Plugins.vue` 添加平台选项。
- 平台特殊逻辑必须放在适配器自己的目录。

---

## 验收标准

### 功能验收

- 新增适配器无需修改 `core/config/manager.go`。
- 新增适配器无需修改 `core/router/router.go`。
- 新增适配器无需修改 `web-ui/src/views/Adapters.vue`。
- 新增适配器无需修改 `web-ui/src/views/Plugins.vue`。
- 管理后台适配器页自动显示新平台。
- 插件平台配置自动显示新平台。
- Router 使用适配器能力接口处理 target 和文本格式。

### 兼容验收

- QQ 现有收发行为不变。
- Telegram 现有收发行为不变。
- QQ 官方 C2C/群聊/DMS 文本行为不变。
- QQ 官方图片发送行为不变。
- 现有数据库适配器配置可继续使用。
- 现有插件 `platforms` 字段可继续使用。

### 验证命令

每阶段至少运行相关测试。最终全量验证：

```bash
go test ./...
npm --prefix "D:/Desktop/program/java/AITest/allbot/web-ui" run build
go build -o "D:/Desktop/program/java/AITest/allbot/allbot.exe" .
```

---

## 风险点

### import cycle

迁移 registry 和子目录时最容易出现 Go import cycle。

规避方式：

- 提前拆出 `contract` 包。
- `registry` 只依赖 `contract`。
- 具体适配器依赖 `contract` 和 `registry`。
- 主程序或 loader blank import 具体适配器。

### 前端动态表单兼容

`Adapters.vue` 从硬编码改成 schema 渲染，可能影响已有配置编辑。

规避方式：

- 保持配置 JSON 结构不变。
- 先支持 text/password 两类字段。
- 保留 unknown 字段，不要保存时丢弃。

### Router 行为变化

把平台逻辑下沉到适配器，可能改变回复目标或 mention 行为。

规避方式：

- 先补测试再改。
- 对 QQ、Telegram、QQ 官方分别覆盖私聊、群聊、图片、关键词回复。

### loader 生成遗漏

新增子目录但忘记跑 generate，会导致平台不注册。

规避方式：

- 在测试中检查关键平台已注册。
- 构建或启动时输出已注册平台列表。
- 后续可在 CI 或本地验证中加入 loader 检查。

---

## 建议执行顺序

1. 阶段 1：建立 registry。
2. 阶段 2：manager 走 registry。
3. 阶段 3：router 走适配器能力接口。
4. 阶段 4：后端提供 `/api/adapter-platforms`。
5. 阶段 5：`Adapters.vue` 动态配置表单。
6. 阶段 6：`Plugins.vue` 动态平台列表。
7. 阶段 7：插件创建默认平台 registry 驱动。
8. 阶段 8：适配器目录化迁移。
9. 阶段 9：loader 生成。
10. 阶段 10：适配器开发模板。
