# 多版本运行环境维护路线

## 背景

部分插件或脚本依赖特定 Node.js/Python 版本，当前项目只有一套全局运行环境，容易出现解释器版本不兼容和依赖冲突。运行环境维护目标是让插件、开放接口和脚本任务可以选择指定运行环境，同时保留旧插件的默认行为。

## 第一步：手动配置多版本解释器

目标：先解决“指定解释器版本运行”的核心问题。

- 增加运行环境 Profile 配置，支持 Node.js 与 Python。
- 每个 Profile 包含名称、运行时类型、版本说明、解释器路径、启用状态和默认标记。
- 插件、开放接口和脚本任务可通过 `runtime_profile` 指定运行环境。
- 未指定 `runtime_profile` 的旧配置继续使用默认 Node.js/Python 环境。
- 后台提供 Profile 管理和可用性测试。
- 不自动下载解释器，由用户自行安装并配置路径。

## 第二步：依赖按环境隔离

目标：解决不同插件之间依赖版本冲突。

- Node.js 每个 Profile 使用独立 `node_modules`。
- Python 每个 Profile 使用独立虚拟环境。
- 支持插件级独立依赖环境选项，例如 `dependency_scope: plugin`。
- 安装依赖时根据 Profile 或插件独立环境写入对应目录。
- 插件运行时设置正确的 `NODE_PATH`、`PATH`、`PYTHONPATH` 等环境变量。

## 第三步：自动安装运行时

目标：降低用户准备多版本解释器的成本。

- 后台选择 Node.js/Python 版本后自动下载、解压和初始化。
- 支持配置下载源、代理和校验信息。
- Windows 下处理 Node.js、Python、pip、npm 的路径与权限问题。
- 初始化对应 Profile 的依赖目录或虚拟环境。
- 失败时保留可诊断日志，避免破坏现有默认环境。

## 推荐实施顺序

1. 先实现手动配置解释器路径，保证插件可以按指定版本运行。
2. 再实现依赖隔离，解决包版本冲突。
3. 最后实现自动下载安装，提升易用性。

## 兼容原则

- 旧插件不需要修改即可继续运行。
- 默认 Node.js/Python Profile 自动映射到当前行为。
- 配置了 `runtime_profile` 的插件优先使用指定 Profile。
- 指定 Profile 不存在或不可用时应返回明确错误，不静默降级。

## Linux 托管解释器扩展计划

### 目标

在保留现有 Windows 自动下载能力的基础上，让 Linux 服务端也可以通过 `managed` Profile 自动下载项目内托管解释器，并继续把所有解释器、依赖和虚拟环境限制在 `runtime/` 目录内。

### 平台模型

`architecture` 字段继续兼容旧配置，但语义扩展为“目标平台”。第一阶段支持：

| 值 | 系统 | CPU | 说明 |
| --- | --- | --- | --- |
| `win-x64` | Windows | amd64 | 当前已支持 |
| `win-arm64` | Windows | arm64 | Node 可支持，Python 视分发源限制 |
| `linux-x64` | Linux | amd64 | Linux 主目标 |
| `linux-arm64` | Linux | arm64 | ARM 服务器目标 |

后端必须以 AllBot 服务端所在平台为准，根据 `runtime.GOOS` 和 `runtime.GOARCH` 推断当前 target。前端不能再使用浏览器 `navigator.platform` 判断服务端平台。

建议新增接口：

```http
GET /api/runtime-profiles/targets
```

返回当前服务端 target、Go OS/Arch，以及每种 runtime 可选择的平台。`managed` Profile 初始化时必须匹配当前服务端 target，禁止在 Windows 服务端初始化 Linux 解释器，反之亦然。

### Node.js Linux 下载方案

Node.js 使用官方便携包：

- `node-v<version>-linux-x64.tar.xz`
- `node-v<version>-linux-arm64.tar.xz`

下载源继续使用 `nodejs.org/dist`，哈希继续读取 `SHASUMS256.txt` 并校验 SHA-256。

Linux 解压后路径：

- 解释器：`<interpreterRoot>/bin/node`
- npm：`<interpreterRoot>/bin/npm`

依赖安装和插件执行时，需要把 `<interpreterRoot>/bin` 前置到 `PATH`，保证 npm 脚本和插件子进程优先使用当前托管 Node。

### Python Linux 下载方案

不建议使用 python.org 源码包自动编译，因为会依赖系统编译工具链、耗时长且跨发行版不可控。

推荐使用 `python-build-standalone` 作为 Linux 便携 Python 来源：

| target | python-build-standalone 目标 |
| --- | --- |
| `linux-x64` | `x86_64-unknown-linux-gnu` |
| `linux-arm64` | `aarch64-unknown-linux-gnu` |

第一阶段只支持 glibc Linux。Alpine/musl 暂不承诺自动托管，建议继续使用手动路径 Profile。

Python 下载后需要探测候选解释器路径，例如：

- `install/bin/python3`
- `install/bin/python`
- `bin/python3`
- `bin/python`

初始化校验顺序：

1. 基础 Python 执行 `--version`。
2. 基础 Python 执行 `-m venv --help`。
3. 使用基础 Python 创建 `runtime/envs/<profileID>/.venv`。
4. 校验 `.venv/bin/python --version`。
5. 校验 `.venv/bin/python -m pip --version`。
6. 如果 pip 不可用但 `ensurepip` 可用，可执行 `python -m ensurepip --upgrade` 后重试。

失败时返回中文错误，不允许静默降级到系统 Python。

### 跨平台路径规则

必须集中封装以下路径，避免 Windows/Linux 判断散落：

- managed Node executable
- managed Python base executable
- npm path
- venv Python path
- venv pip path
- PATH 注入目录

Windows：

- `.venv/Scripts/python.exe`
- `.venv/Scripts/pip.exe`
- `node.exe`
- `npm.cmd`

Linux：

- `.venv/bin/python`
- `.venv/bin/pip`
- `bin/node`
- `bin/npm`

Python 插件执行时建议注入：

- `PYTHONUTF8=1`
- `VIRTUAL_ENV=<profile venv>`
- `PATH=<profile venv bin>:<old PATH>`

Node 插件执行时建议注入：

- `NODE_PATH=<profile node_modules>`
- `PATH=<managed node bin>:<old PATH>`

### 下载器改造

当前下载器以 zip 为主。Linux 支持需要扩展为多归档类型：

- zip
- tar.xz
- tar.gz
- 如 Python 发行包必须使用 zstd，再补 tar.zst

安全解包要求：

- 禁止绝对路径。
- 禁止 `..` 路径穿越。
- 限制文件数量。
- 限制总解压体积。
- 禁止设备文件、FIFO 等特殊文件。
- symlink/hardlink 只能指向解压根目录内部。
- 解压、哈希、版本校验通过后再替换最终目录。
- `force=true` 时先备份旧目录，失败回滚。

### 前端改造

`RuntimeProfiles.vue` 后续应从后端目标接口加载能力：

- 默认选择服务端 `current_target`。
- 架构下拉由后端返回值生成。
- `managed` Node 展示官方 `nodejs.org` 下载源说明。
- `managed` Python 展示 `python-build-standalone` 下载源说明。
- 未初始化或 npm/pip 不可用时，在依赖页禁用安装按钮并提示先初始化。

### 实施顺序

1. 新增后端 target 推断和 `/api/runtime-profiles/targets`。
2. 前端改为后端平台能力驱动。
3. 扩展下载器支持 tar.xz/tar.gz 安全解包。
4. 实现 Node Linux 官方包下载、npm 路径和 PATH 注入。
5. 实现 Python Linux `python-build-standalone` 下载、venv 和 pip 初始化。
6. 补充 Go/Web/前端验证。
7. 在 Linux 实机做 Node/Python 插件、OpenAPI、脚本任务冒烟验证。

### 风险与取舍

- Python Linux 没有 python.org 官方通用便携包，推荐使用 `python-build-standalone`，但它不是 Python 官方发行。
- Alpine/musl 兼容性复杂，第一阶段不建议自动托管。
- Linux 包较大，初始化接口应使用后台任务和进度轮询，避免前端切路由中断。
- tar 解包必须严格限制路径、链接、体积和文件数量，避免归档包风险。
