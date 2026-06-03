# AllBot GitHub Release 版本检查与自更新任务清单

## 目标

基于 GitHub Release 实现 AllBot 版本检查和自更新能力。

仓库：

```text
https://github.com/298582245/allbot
```

用户发送：

```text
version
```

回复当前版本、最新版本和 Release 更新内容。

用户发送：

```text
更新
```

提示当前版本、最新版本、新版本信息，并等待用户发送 `y` 确认升级。

---

## 目标交互

### version 指令

如果有新版本：

```text
AllBot v1.0.0

版本信息：
当前版本：v1.0.0
最新版本：v1.0.1

更新内容：
1. 新增 QQ 官方机器人适配器
2. 优化插件平台配置
3. 修复图片发送问题

发送「更新」可升级到最新版本。
```

如果已经是最新版：

```text
AllBot v1.0.1

版本信息：
当前版本：v1.0.1
最新版本：v1.0.1

当前已是最新版本。
```

如果 GitHub 没有 Release：

```text
AllBot v1.0.0

版本信息：
当前版本：v1.0.0
最新版本：未检测到 Release

请先在 GitHub 发布版本。
```

### 更新 指令

如果存在新版本：

```text
当前版本：v1.0.0
最新版本：v1.0.1

新版本信息：
1. 新增 QQ 官方机器人适配器
2. 优化插件平台配置
3. 修复图片发送问题

发送 y 确认升级，其他内容取消。
```

用户回复：

```text
y
```

开始升级：

```text
开始升级到 v1.0.1，请稍候...
```

升级成功：

```text
升级完成，正在重启 AllBot...
```

用户回复其他内容：

```text
已取消升级。
```

---

## GitHub Release 规范

正式版本只认 GitHub Release，不直接使用 main/master 分支作为最新版本。

推荐 tag：

```text
v1.0.0
v1.0.1
v1.1.0
```

Release assets 建议命名：

```text
allbot-windows-amd64.exe
allbot-linux-amd64
allbot-linux-arm64
checksums.txt
```

可选：

```text
allbot-darwin-amd64
allbot-darwin-arm64
```

Release body 示例：

```md
## 更新内容

1. 新增 QQ 官方机器人适配器
2. 插件平台支持动态适配器
3. 修复 QQ 官方图片发送问题
4. 优化日志显示
```

---

## 阶段 1：版本信息基础模块（已完成）

### 1.1 新增版本包

新增：

```text
core/version/version.go
```

内容：

```go
package version

var Version = "v1.0.0"
var Commit = "unknown"
var BuildTime = "unknown"
```

### 1.2 编译注入版本

编译命令示例：

```bash
go build -ldflags "-X github.com/allbot/allbot/core/version.Version=v1.0.1" -o allbot.exe .
```

后续 GitHub Actions 负责自动注入：

```text
Version = tag
Commit = git sha
BuildTime = 当前时间
```

### 1.3 替换现有 FrameworkVersion

检查当前 `version` 内置指令使用的版本来源。

目标：

- `version` 指令显示 `core/version.Version`。
- 保留原本框架版本常量时，需要统一来源，避免多个版本号不同步。

---

## 阶段 2：GitHub Release 查询模块（已完成）

### 2.1 新增 updater 包

新增目录：

```text
core/updater/
```

建议文件：

```text
core/updater/github.go
core/updater/version_compare.go
core/updater/types.go
core/updater/github_test.go
core/updater/version_compare_test.go
```

### 2.2 GitHub API

请求：

```text
GET https://api.github.com/repos/298582245/allbot/releases/latest
```

解析字段：

```go
type ReleaseInfo struct {
    Version string
    Name    string
    Body    string
    URL     string
    Assets  []ReleaseAsset
}

type ReleaseAsset struct {
    Name        string
    DownloadURL string
    Size        int64
}
```

GitHub JSON 字段：

```json
{
  "tag_name": "v1.0.1",
  "name": "v1.0.1",
  "body": "更新内容...",
  "html_url": "https://github.com/298582245/allbot/releases/tag/v1.0.1",
  "assets": [
    {
      "name": "allbot-windows-amd64.exe",
      "browser_download_url": "https://..."
    }
  ]
}
```

### 2.3 版本比较

实现：

```go
CompareVersion(current, latest string) int
```

返回：

```text
-1 当前版本低于最新版本
 0 相等
 1 当前版本高于最新版本
```

支持：

```text
v1.0.0
1.0.0
v1.0.1
v1.0.1-beta
```

第一版可以只支持稳定版本：

```text
v数字.数字.数字
```

不支持复杂语义版本时，要返回明确错误。

### 2.4 测试

覆盖：

- 正常解析 latest release。
- 没有 release。
- GitHub API 错误。
- assets 解析。
- `v1.0.0` < `v1.0.1`。
- `v1.0.1` = `1.0.1`。
- 非法版本报错。

---

## 阶段 3：version 指令升级为完整版本信息（已完成）

### 3.1 找到现有内置 version 指令

需要搜索：

```text
version
FrameworkVersion
版本
```

重点文件可能在：

```text
core/router/keyword_reply.go
core/router/system_info_fallback.go
core/types/types.go
```

### 3.2 修改 version 回复内容

逻辑：

1. 读取当前版本：`core/version.Version`。
2. 请求 GitHub latest release。
3. 比较当前版本和最新版本。
4. 生成回复文本。

### 3.3 超时和失败处理

GitHub 查询要有超时，例如：

```text
5s - 10s
```

如果失败，回复：

```text
AllBot v1.0.0

版本信息：
当前版本：v1.0.0
最新版本：获取失败

失败原因：xxx
```

### 3.4 测试

需要可以注入 mock release client，避免测试依赖真实 GitHub。

覆盖：

- 有新版本。
- 已是最新版本。
- 获取失败。
- 没有 Release。

---

## 阶段 4：新增“更新”确认流程

### 4.1 新增内置指令

新增触发：

```text
更新
```

可选支持：

```text
update
升级
```

### 4.2 确认流程

用户发送：

```text
更新
```

系统检查最新 Release。

如果有新版本：

```text
当前版本：v1.0.0
最新版本：v1.0.1

新版本信息：
...

发送 y 确认升级，其他内容取消。
```

然后等待当前用户在当前会话回复。

用户回复：

```text
y
```

继续升级。

其他回复：

```text
已取消升级。
```

超时：

```text
升级确认超时，已取消。
```

### 4.3 等待实现方式

优先复用现有 session/listen 机制。

如果现有内置回复不能直接调用 listen，可以新增：

```text
core/router/update_confirm.go
```

或者在 `KeywordReplyManager` 中新增等待确认逻辑。

### 4.4 测试

覆盖：

- 发送更新，有新版本，提示确认。
- 回复 `y` 触发升级。
- 回复其他内容取消。
- 超时取消。
- 已是最新版时不进入确认。

---

## 阶段 5：选择更新模式

需要区分：

```text
二进制运行
源码运行 / go run
```

### 5.1 二进制运行检测

可以使用：

```go
os.Executable()
```

如果可执行文件路径在系统临时 go-build 目录，通常是 `go run`。

Windows 可能类似：

```text
...\AppData\Local\Temp\go-build...
```

Linux 可能类似：

```text
/tmp/go-build...
```

### 5.2 源码模式检测

如果当前工作目录是 Git 仓库，并且当前可执行文件看起来像 go-build 临时文件，可以认定为源码运行。

### 5.3 更新模式

```go
type UpdateMode string

const (
    UpdateModeBinary UpdateMode = "binary"
    UpdateModeSource UpdateMode = "source"
)
```

---

## 阶段 6：二进制自更新

### 6.1 匹配 asset

根据：

```go
runtime.GOOS
runtime.GOARCH
```

匹配文件：

| GOOS | GOARCH | asset |
| --- | --- | --- |
| windows | amd64 | `allbot-windows-amd64.exe` |
| linux | amd64 | `allbot-linux-amd64` |
| linux | arm64 | `allbot-linux-arm64` |
| darwin | amd64 | `allbot-darwin-amd64` |
| darwin | arm64 | `allbot-darwin-arm64` |

### 6.2 下载

下载到临时文件：

```text
allbot.exe.new
allbot.new
```

### 6.3 校验 checksums

如果 Release 包含 `checksums.txt`，则校验 SHA256。

格式：

```text
<sha256>  allbot-windows-amd64.exe
```

第一版可以先下载但不强制校验；最终版建议必须校验。

### 6.4 Windows 替换

Windows 无法覆盖正在运行的 exe。

生成：

```text
update-allbot.bat
```

流程：

```bat
timeout /t 2
move /y allbot.exe allbot.exe.bak
move /y allbot.exe.new allbot.exe
start allbot.exe
```

AllBot 调用脚本后退出。

### 6.5 Linux 替换

Linux 可使用：

```text
chmod +x allbot.new
mv allbot allbot.bak
mv allbot.new allbot
./allbot
```

也建议用外部 shell 脚本统一处理。

### 6.6 回滚

如果替换失败：

- 保留 `.bak`。
- 输出明确错误。
- 不删除旧程序。

---

## 阶段 7：源码模式更新

### 7.1 检查 Git 仓库

执行：

```bash
git rev-parse --is-inside-work-tree
```

### 7.2 检查本地修改

执行：

```bash
git status --porcelain
```

如果有本地修改：

```text
检测到本地有未提交修改，为避免覆盖代码，已取消自动更新。
```

### 7.3 拉取代码

执行：

```bash
git fetch --tags
git pull --ff-only
```

### 7.4 编译

Windows：

```bash
go build -o allbot.exe .
```

Linux：

```bash
go build -o allbot .
```

### 7.5 重启

源码模式建议第一版只提示：

```text
源码已更新并编译完成，请手动重启 AllBot。
```

后续再实现自动重启。

---

## 阶段 8：GitHub Actions Release 自动构建（已完成）

新增：

```text
.github/workflows/release.yml
```

触发：

```bash
git tag v1.0.1
git push origin v1.0.1
```

Actions 做：

1. 检出代码。
2. 编译 Windows amd64。
3. 编译 Linux amd64。
4. 编译 Linux arm64。
5. 可选编译 macOS。
6. 生成 `checksums.txt`。
7. 创建 GitHub Release。
8. 上传 assets。

### 8.1 编译命令示例

Windows：

```bash
GOOS=windows GOARCH=amd64 go build -ldflags "-X github.com/allbot/allbot/core/version.Version=${TAG}" -o allbot-windows-amd64.exe .
```

Linux：

```bash
GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/allbot/allbot/core/version.Version=${TAG}" -o allbot-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -ldflags "-X github.com/allbot/allbot/core/version.Version=${TAG}" -o allbot-linux-arm64 .
```

---

## 阶段 9：Web UI 展示更新信息（可选）

后续可以在设置页或仪表盘增加：

```text
当前版本
最新版本
检查更新按钮
升级按钮
```

第一版先只做聊天指令，不做 Web UI。

---

## 验收标准

### version 指令

- 能显示当前版本。
- 能显示 GitHub latest release。
- 能显示 Release body。
- GitHub 失败时有明确错误。

### 更新 指令

- 有新版本时提示确认。
- `y` 触发升级。
- 其他内容取消。
- 没有新版本时不进入确认。

### 二进制更新

- 能匹配当前系统 asset。
- 能下载新版本。
- Windows 不直接覆盖运行中的 exe。
- Linux 能替换并保留备份。

### 源码更新

- 能识别 go run / 源码模式。
- 有未提交修改时拒绝更新。
- 能执行 git pull 和 go build。

### Release

- 打 tag 后能自动生成 GitHub Release。
- Release assets 命名与程序匹配。
- checksums.txt 可用于校验。

---

## 推荐执行顺序

1. 阶段 1：版本信息基础模块。
2. 阶段 2：GitHub Release 查询模块。
3. 阶段 3：version 指令完整版本信息。
4. 阶段 8：GitHub Actions Release 自动构建。
5. 阶段 4：更新确认流程。
6. 阶段 5：更新模式识别。
7. 阶段 6：二进制自更新。
8. 阶段 7：源码模式更新。
9. 阶段 9：Web UI 展示更新信息。

建议先做检查和显示，再做真正替换文件的升级逻辑。
