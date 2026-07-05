# AllBot 发布新版本流程

本文记录 AllBot 发布新版本的固定流程，后续发布时按本文检查和执行，避免遗漏版本号、Release Notes、构建产物和 GitHub Actions 步骤。

## 1. 发布前确认

1. 确认本次发布版本号，例如 `v1.0.2`。
2. 确认 `version.txt` 已写入对应版本的发布说明，标题格式必须为：

```text
AllBot v1.0.2
```

3. 确认本次要发布的代码范围。通常发布正式版本时，应把当前需要随版本发布的代码全部纳入提交。
4. 确认 GitHub Actions 发布工作流存在：`.github/workflows/release.yml`。

## 2. 修改版本号

需要同步修改以下文件：

### `core/version/version.go`

```go
var Version = "v1.0.2"
var Commit = "unknown"
var BuildTime = "unknown"
var BuildChannel = "release"
```

要求：

- `Version` 改为目标版本号。
- 正式发布时 `BuildChannel` 必须是 `release`，不要保留 `local` 或 `docker`。

### `build.bat`

```bat
set VERSION=v1.0.2
```

要求：

- 与 `core/version/version.go` 的版本号保持一致。

## 3. 检查 Release Notes

GitHub Actions 会从 `version.txt` 中读取当前 tag 对应的发布说明。

规则：

- tag 为 `v1.0.2` 时，`version.txt` 中必须存在标题 `AllBot v1.0.2`。
- 工作流会从该标题开始读取，直到下一个 `AllBot v` 标题前结束。
- 如果缺少对应标题，Release 工作流会失败。

## 4. 本地验证

发布前执行：

```bash
go test ./...
```

如果只修改文档，可以至少执行：

```bash
git diff --check
```

如果改动了前端源码，还需要先构建前端：

```bash
cd "D:/Desktop/program/java/AITest/allbot/web-ui" && npm run build
```

然后再回到项目根目录执行 Go 测试或构建。

## 5. 提交代码

查看状态和差异：

```bash
git status --short
git diff
```

提交本次发布相关改动：

```bash
git add <需要发布的文件>
git commit -m "发布 AllBot v1.0.2"
```

注意：

- 不要把 `.env`、密钥、临时文件、数据库文件纳入提交。
- 不要跳过 hooks。
- 不要使用 `git add .` 盲目提交，优先逐个添加相关文件。

## 6. 创建并推送 tag

确认当前提交就是要发布的提交后：

```bash
git tag v1.0.2
git push origin main
git push origin v1.0.2
```

推送 tag 后，GitHub Actions 会自动执行发布流程。

## 7. GitHub Actions 发布流程

`.github/workflows/release.yml` 的主要流程：

1. tag `v*` 推送触发。
2. 读取 `version.txt` 生成 Release Notes。
3. 执行：

```bash
go test ./...
```

4. 构建 Release 资产：

```text
allbot-v版本号-windows-amd64.exe
allbot-v版本号-linux-amd64
allbot-v版本号-linux-arm64
checksums-v版本号.txt
```

5. 发布 GitHub Release。

## 8. 发布后检查

发布完成后检查：

1. GitHub Actions 是否成功。
2. GitHub Release 是否生成。
3. Release 名称是否为 `AllBot v版本号`。
4. Release 内容是否来自 `version.txt`。
5. Release 资产是否包含：

```text
allbot-v版本号-windows-amd64.exe
allbot-v版本号-linux-amd64
allbot-v版本号-linux-arm64
checksums-v版本号.txt
```

6. `checksums-v版本号.txt` 是否包含所有二进制文件的 SHA256。

## 9. Actions 失败处理

如果 GitHub Actions 失败：

1. 先查看失败日志，定位是测试失败、Release Notes 缺失、构建失败还是上传失败。
2. 在本地修复问题。
3. 重新执行本地验证：

```bash
go test ./...
```

4. 提交修复：

```bash
git add <修复文件>
git commit -m "修复 v1.0.2 发布问题"
git push origin main
```

5. 如果失败的 tag 已经指向旧提交，需要让 tag 指向修复后的提交。该操作会改动远程 tag，执行前必须确认这是当前发布所需操作：

```bash
git tag -f v1.0.2
git push origin -f v1.0.2
```

注意：

- 只在 Release 尚未成功或明确需要重跑同一版本发布时更新 tag。
- 不要 force push `main`。
- 如果 Release 已经对外发布，优先考虑发布下一个补丁版本，而不是覆盖旧 tag。

## 10. 常见问题

### Release Notes 缺失

现象：Actions 提示 `version.txt missing release notes for v版本号`。

处理：在 `version.txt` 增加对应标题，例如：

```text
AllBot v1.0.2
```

### Linux Actions 测试失败但本地 Windows 通过

常见原因是测试写死了 Windows 资产名，例如 `allbot-windows-amd64.exe`。测试中需要按运行平台生成资产名：

```go
name := "allbot-" + runtime.GOOS + "-" + runtime.GOARCH
if runtime.GOOS == "windows" {
    name += ".exe"
}
```

### Node.js 版本警告

GitHub Actions 可能提示某些 action 使用的 Node.js 版本即将弃用。只要步骤没有失败，这通常只是警告，不代表发布失败。

### Docker 用户如何更新

Docker Compose 默认使用：

```text
ALLBOT_UPDATE_MODE=docker
```

管理员在系统设置页点击“一键升级”或向机器人发送「更新」时，程序会下载 Release 资产并写入升级请求，容器入口脚本会替换 `/data/allbot` 并重启应用。

也可以通过源码重新构建镜像：

```bash
git pull
docker compose up -d --build
```
