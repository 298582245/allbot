# 动态配置系统实现总结

## 实现时间
2026-05-10

## 问题背景

用户反馈：平台配置（QQ/微信/Telegram）存储在 `config.yml` 和命令行参数中，修改配置需要重启机器人，体验不佳。

**用户原话**：
> "忘记说了，平台这些配置就应该存在数据库啊，然后在webui就可以重新配置啊，不然我修改配置还得重启机器人？？？？"

## 解决方案

实现了基于数据库的动态配置系统，支持通过 Web UI 修改平台配置并实时生效，无需重启机器人。

## 核心实现

### 1. 配置数据模型（core/config/models.go）

定义了适配器配置的数据结构：

```go
type AdapterConfig struct {
    ID        int64     // 配置 ID
    Platform  string    // 平台名称：qq, wechat, telegram
    Enabled   bool      // 是否启用
    Config    string    // JSON 配置
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

不同平台的配置结构：
- **QQ**: `api_url`（go-cqhttp API 地址）、`listen_addr`（监听地址）
- **微信**: `app_id`、`app_secret`
- **Telegram**: `bot_token`

### 2. 数据库操作（core/config/database.go）

使用 SQLite 存储配置：

**核心功能**：
- `GetAllAdapters()` - 获取所有适配器配置
- `GetAdapter(platform)` - 获取指定平台配置
- `SaveAdapter(adapter)` - 保存/更新配置
- `DeleteAdapter(platform)` - 删除配置

**数据库表结构**：
```sql
CREATE TABLE adapters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    platform TEXT NOT NULL UNIQUE,
    enabled INTEGER NOT NULL DEFAULT 0,
    config TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

### 3. 适配器管理器（core/config/manager.go）

实现了热重载机制：

**核心功能**：
- `LoadAndStartAdapters()` - 启动时加载所有启用的适配器
- `ReloadAdapter(platform)` - 重新加载指定适配器（停止旧的，启动新的）
- `SaveAdapterConfig()` - 保存配置并自动重新加载
- `StopAdapter(platform)` - 停止适配器
- `StopAll()` - 停止所有适配器

**热重载流程**：
1. 用户在 Web UI 修改配置
2. 保存到数据库
3. 停止旧的适配器实例
4. 使用新配置创建并启动新的适配器实例
5. 无需重启整个机器人进程

### 4. Web API（core/web/server.go）

新增配置管理 API：

```
GET  /api/adapters           - 获取所有适配器配置
POST /api/adapters           - 创建/更新适配器配置
GET  /api/adapters/{platform} - 获取指定平台配置
DELETE /api/adapters/{platform} - 删除配置
```

**API 响应示例**：
```json
{
  "id": 1,
  "platform": "qq",
  "enabled": true,
  "config": "{\"api_url\":\"http://localhost:5700\",\"listen_addr\":\":8080\"}",
  "running": true,
  "created_at": "2026-05-10T10:00:00Z",
  "updated_at": "2026-05-10T10:30:00Z"
}
```

### 5. Web UI（web/index.html）

新增"平台配置"标签页：

**功能**：
- 查看所有平台配置列表
- 显示启用/禁用状态和运行状态
- 添加新平台配置
- 编辑现有配置
- 删除配置
- 实时生效提示

**界面特性**：
- 模态框表单，根据平台类型动态显示配置项
- 状态徽章显示启用状态和运行状态
- 操作成功/失败消息提示
- 5 秒自动刷新

### 6. 主程序改造（main.go）

**变更**：
- 移除命令行参数 `--qq-api`
- 初始化配置数据库 `config.db`
- 创建适配器管理器
- 从数据库加载并启动适配器
- 关闭时停止所有适配器

**启动流程**：
```
1. 初始化配置数据库
2. 初始化默认配置（如果不存在）
3. 初始化依赖管理器
4. 创建会话管理器和消息路由器
5. 加载插件
6. 创建适配器管理器
7. 从数据库加载并启动所有启用的适配器
8. 启动 Web UI
```

## 使用方式

### 1. 首次启动

```bash
go run main.go --plugins=./plugins
```

系统会自动创建 `config.db` 并初始化默认 QQ 配置（禁用状态）。

### 2. 配置平台

1. 访问 http://localhost:3000
2. 使用用户名 `admin` 和首次启动时控制台输出的随机密码登录
3. 切换到"平台配置"标签
4. 点击"添加平台"或编辑现有配置
5. 填写配置信息并选择"启用"
6. 点击"保存" - **配置立即生效，无需重启**

### 3. 修改配置

1. 在"平台配置"标签中点击"编辑"
2. 修改配置项
3. 点击"保存" - **适配器自动重启，新配置立即生效**

### 4. 禁用平台

1. 编辑配置，将状态改为"禁用"
2. 保存后适配器自动停止

## 技术亮点

1. **零停机配置更新**：修改配置后自动重启适配器，不影响其他运行中的适配器和插件
2. **数据库持久化**：配置存储在 SQLite，重启后自动恢复
3. **类型安全**：不同平台的配置使用强类型结构体
4. **并发安全**：适配器管理器使用 `sync.RWMutex` 保护并发访问
5. **用户友好**：Web UI 提供直观的配置界面，实时反馈操作结果

## 文件变更清单

### 新增文件
- `core/config/models.go` - 配置数据模型
- `core/config/database.go` - 数据库操作
- `core/config/manager.go` - 适配器管理器
- `sqls/001_create_adapters_table.sql` - 数据库表结构

### 修改文件
- `main.go` - 使用配置数据库替代命令行参数
- `core/web/server.go` - 添加配置管理 API
- `core/adapter/qq_adapter.go` - 修复监听地址参数
- `web/index.html` - 添加平台配置管理界面

### 运行时文件
- `config.db` - SQLite 配置数据库（自动创建）

## 依赖

新增 Go 依赖：
```go
import _ "github.com/mattn/go-sqlite3"
```

需要安装：
```bash
go get github.com/mattn/go-sqlite3
```

## 后续优化建议

1. **配置验证**：保存前验证配置的有效性（如测试 API 连接）
2. **配置历史**：记录配置变更历史，支持回滚
3. **批量操作**：支持批量启用/禁用多个平台
4. **配置导入导出**：支持配置的备份和迁移
5. **更多平台**：实现微信和 Telegram 适配器

## 总结

成功实现了动态配置系统，解决了用户反馈的"修改配置需要重启"的问题。现在用户可以通过 Web UI 随时修改平台配置，配置立即生效，大大提升了使用体验。

**核心价值**：
- ✅ 无需重启即可修改配置
- ✅ 配置持久化存储
- ✅ Web UI 可视化管理
- ✅ 支持多平台配置
- ✅ 热重载机制
