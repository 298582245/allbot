# 插件编写 Skill

## 目标
- 让 Claude 在当前仓库里编写插件时，优先复用现有 `sdk/` 和 `plugins/` 的约定，而不是每次重新探索。
- 将插件的目录结构、入口写法、`plugin.json` 字段、常见开发模式、测试方式和交付清单固化为可复用流程。
- 适用于 Node.js 插件、Python 插件，以及基于 `account_ql_plugin` 的账号类插件。

## 什么时候使用
- 需要新增一个 AllBot 插件时。
- 需要修改现有插件的触发规则、配置项、账号逻辑、定时任务或脚本执行逻辑时。
- 需要按项目规范补齐 `plugin.json`、`main.js` / `main.py`、测试、示例配置时。
- 需要把一个青龙脚本封装成可在 AllBot 中运行的账号插件时。

## 先看什么
- 先查看 `sdk/nodejs/allbot_direct.js`、`sdk/nodejs/account_ql_plugin.js`。
- 再查看 `sdk/python/allbot_direct.py`、`sdk/python/account_ql_plugin.py`。
- 再查看 `plugins/` 下的三个代表性样例：
  - `custom_reply`：基础消息插件
  - `weather` / `kuwo_music`：Python 普通插件与账号类插件
  - `fxsh`：Node.js 账号类插件、定时任务、授权、脚本执行的完整样例
- 编写前至少对比 3 个现有实现，优先复用它们的字段名、调用方式和交互模式。

## 插件目录约定
- 每个插件放在 `plugins/<插件名>/`。
- 普通插件通常包含：
  - `main.js` 或 `main.py`
  - `plugin.json`
  - 可选 `scripts/`、辅助文件、测试文件
- 账号类插件通常额外包含：
  - `scripts/` 下的青龙脚本或辅助脚本
  - 清晰的账号解析函数
  - 查询、授权、运行、过期检测、CK 检测等入口
- 入口文件名要与 `plugin.json.entry` 保持一致。

## `plugin.json` 规则
- 必须保证字段完整、可直接被后台识别。
- 常见字段：
  - `name`：插件名称
  - `version`：版本号
  - `enabled`：是否启用
  - `runtime`：`nodejs` 或 `python`
  - `runtime_profile`：运行时画像，如 `python311`
  - `entry`：入口文件，如 `main.js` / `main.py`
  - `trigger`：触发正则
  - `priority`：优先级
  - `platforms`：支持的平台列表
  - `dependencies`：依赖声明
  - `access_control`：访问控制配置
  - `allowed_adapter_ids`：允许的适配器 ID 列表
  - `script_env`：脚本环境变量开关与变量名
  - `open_api`：开放接口配置
  - `user_config`：用户配置默认值
  - `user_config_schema`：用户配置表单
- `user_config_schema` 常见类型：
  - `divider`
  - `text`
  - `number`
  - `boolean`
  - `password`
  - `textarea`
- 普通插件重点关注 `trigger`、`dependencies`、`user_config`。
- 账号类插件重点关注 `trigger`、`priority`、`user_config_schema`、`script_env`、`open_api`、`access_control`。
- OpenAPI 插件需要保持 `open_api.enabled`、`open_api.method`、`open_api.path`、`open_api.runtime`、`open_api.runtime_profile` 和入口逻辑一致。
- 触发规则要与插件业务强匹配，建议用 `^...$` 锚定边界，避免和其他插件冲突。
- 用户配置项要和代码里的 `ctx.config()` / `ctx.config(key)` / `ctx.config(key, default)` 保持一致。

## 入口写法

### Node.js
- 入口文件一般通过 `require('../../sdk/nodejs/allbot_direct')` 或同类相对路径引入 SDK。
- 常见入口形式：
  - `runDirect((ctx) => new Plugin(ctx).main())`
  - `createAccountQLPlugin({ ... })`
- `console.log`、`console.warn` 等日志会在插件环境里重定向到 `stderr`，不要依赖它们输出机器人回复内容。

### Python
- 入口文件一般通过把 `sdk/python` 加入 `sys.path` 后导入 `allbot_direct`。
- 常见入口形式：
  - `run_direct(handle)`
  - `create_account_ql_plugin({...})`
- 主处理函数建议写成 `async def handle(ctx): ...`。

## SDK 使用规范

### 通用上下文对象
- 统一从 `ctx` 读取消息与身份信息：
  - `ctx.content` / `ctx.text`
  - `ctx.userId` / `ctx.user_id`
  - `ctx.groupId` / `ctx.group_id`
  - `ctx.unionId` / `ctx.union_id`
  - `ctx.platform`
  - `ctx.adapterId` / `ctx.adapter_id`
  - `ctx.points`
  - `ctx.config(...)`
  - `ctx.meta(...)`
- 判断消息类型时优先使用：
  - `ctx.isGroup()` / `ctx.is_group()`
  - `ctx.isPrivate()` / `ctx.is_private()`
  - `ctx.isAdmin()` / `ctx.is_admin()`
- 解析命令时优先使用：
  - `ctx.args('前缀')`

### 回复与消息发送
- 使用 `ctx.reply(...)` 回复当前消息。
- 使用 `ctx.sendText(...)` / `ctx.send_text(...)` 时保持语义等价。
- 需要主动推送时使用：
  - `ctx.sendMessage(...)` / `ctx.send_message(...)`
  - `ctx.push(...)`
- 发送图片和文件时使用：
  - `ctx.sendImage(...)` / `ctx.send_image(...)`
  - `ctx.sendFile(...)` / `ctx.send_file(...)`

### 交互式等待
- 需要用户继续输入时使用 `ctx.listen(timeout)`。
- 超时要返回可理解的提示，不要让流程悬空。
- 管理类操作建议使用“序号选择 + q 退出 + y 确认”的交互模式。

### 数据视图和数据库
- 需要在后台“数据管理”展示插件数据时，使用：
  - `ctx.setDataView(...)` / `ctx.set_data_view(...)`
- 自定义回答插件这类轻量场景可直接用：
  - `ctx.db.createTable(table, columns)`
  - `ctx.db.setView(table, options)`
  - `ctx.db.query(table, options)`
  - `ctx.db.first(table, options)`
  - `ctx.db.insert(table, values)`
  - `ctx.db.update(table, rowId, values)`
  - `ctx.db.delete(table, rowId)`
  - `ctx.db.clear(table)`
- 插件表通常是私有表，真实表名由后端自动加插件前缀。
- 查询推荐使用结构化条件，避免拼接复杂 `where` 字符串。
- 表结构、展示列、分组说明要保持清晰，便于后台维护。

### 账号、授权、定时任务和脚本
- 需要系统统一用户 ID 时使用 `ctx.getUnionId()` / `ctx.get_union_id()`。
- 需要积分扣减或增加时使用：
  - `ctx.consumePoints(...)` / `ctx.consume_points(...)`
  - `ctx.addPoints(...)` / `ctx.add_points(...)`
- 需要支付授权时使用 `ctx.pay.waitPay(...)` 或对应别名。
- 需要声明定时任务时使用：
  - `ctx.setScheduledTask(...)` / `ctx.set_scheduled_task(...)`
- 需要伪造消息进入正常路由时使用：
  - `ctx.fakeMessage(...)` / `ctx.fake_message(...)`
- 需要运行插件脚本或青龙脚本时使用：
  - `ctx.runScript(...)` / `ctx.run_script(...)`
  - `ctx.runQLScript(...)` / `ctx.run_ql_script(...)`
- 授权逻辑要统一记录 `expires_at`，并保留账号 metadata。

## 普通插件编写流程
1. 定义触发词和适用平台。
2. 设计 `plugin.json`。
3. 编写入口文件，优先支持：
   - 命令前缀
   - 参数解析
   - 交互式补参
   - 异常提示
4. 需要外部请求时再引入依赖，如 `axios`、`requests`。
5. 保持回复简洁、明确，避免一次输出过长。
6. 为关键分支补测试。

## 账号类插件编写流程
1. 先确定账号唯一键、展示名、环境变量名。
2. 编写账号解析函数，返回统一结构：
   - `envValue` / `env_value`
   - `uniqueKey` / `unique_key`
   - `displayName` / `display_name`
   - `remark`
   - `metadata`
3. 接入 `account_ql_plugin`，统一实现：
   - 登录
   - 账号管理
   - 查询
   - 运行
   - 一键运行 / 签到
   - 授权
   - 删除
   - CK 检测
   - 过期检测
4. 用 `ql` 配置声明青龙脚本路径、运行时、超时、环境变量构造函数、运行后回调。
5. 用 `schedules` 配置默认定时任务，任务名和触发内容要和业务术语一致。
6. 若支持授权支付，优先沿用现有积分支付或支付等待模式，不要自行造新协议。

## Node.js 账号类插件要点
- 参考 `plugins/fxsh/main.js`。
- 推荐结构：
  - 定义业务 API 类
  - 实现 `login`
  - 实现 `withdraw` / `run` / `query`
  - 调用 `createAccountQLPlugin({ ... })`
- `account` 配置里通常要提供：
  - `query(account, ctx, index)`：账号查询展示
  - `checkCk(account)`：CK 有效性检测
- `ql.env(ctx, accounts)` 应返回对象，键名就是青龙环境变量名。
- `afterRun(ctx, accounts, result, helpers)` 适合在定时任务完成后补充查询、通知和状态写回。

## Python 账号类插件要点
- 参考 `plugins/kuwo_music/main.py` 和 `sdk/python/account_ql_plugin.py`。
- `parse_input` / `parseInput` 要校验格式、清洗字段、生成唯一键。
- `query` 要输出简洁账号摘要，便于列表和详情展示。
- `script_runtime` 与 `task_script` 要和 `plugin.json` 保持一致。

## 常见写法模板

### 交互式命令模板
- 先回复引导语。
- 再等待用户输入。
- 再做校验。
- 再执行核心动作。
- 最后给出成功或失败提示。

### 账号保存模板
- 从原始输入中提取账号数据。
- 校验唯一键、CK、手机号或密码格式。
- 生成展示名和备注。
- 查询是否已有同账号记录。
- 保存时保留旧授权到期时间和状态。

### 定时任务模板
- 先声明默认 `cron`。
- 统一使用“管理员一键运行”作为定时触发内容。
- 任务执行后根据结果决定是否补发通知。

## 测试与验证
- 新增或修改插件后，至少补一个自动化测试或等效验证脚本。
- Node.js 插件优先补：
  - 命令解析
  - 账号解析
  - 运行参数
  - 定时任务回调
- Python 插件优先补：
  - 账号解析
  - 运行分支
  - 授权或查询输出
- 参考现有测试风格：
  - `sdk/nodejs/allbot_direct.test.js`
  - `sdk/python/test_account_ql_plugin.py`
- 验证重点：
  - 输入格式错误时的提示
  - 账号重复时的处理
  - 授权到期与过期检测
  - 运行模式分支
  - 定时任务触发内容

## 编写时的检查清单
- `plugin.json` 是否完整且字段一致。
- `entry` 是否与入口文件匹配。
- `trigger` 是否能准确命中目标消息。
- `user_config` 是否和代码读取逻辑一致。
- 是否复用 `sdk/` 里已有的上下文 API。
- 账号类插件是否统一走 `account_ql_plugin`。
- 是否提供查询、运行、授权、删除、检测的完整闭环。
- 是否补了测试或最小验证脚本。
- 是否避免引入和现有实现冲突的自定义协议。

## 生成插件时的输出要求
- 直接产出可运行代码，不要只给伪代码。
- 保持与仓库现有命名一致，优先使用小驼峰。
- 保持回复文案简洁、中文友好、可操作。
- 若需要新增文件，优先补齐插件目录、测试和必要脚本。
- 若涉及前端配置页联动，再同步检查后台和 Web UI 的字段是否一致。

## 推荐复用顺序
1. 先找最相似的现成插件。
2. 再找对应语言的 SDK。
3. 再补差异化业务逻辑。
4. 最后补测试和配置说明。

## 适用边界
- 只适用于当前仓库 `allbot` 的插件开发风格。
- 若插件属于完全不同的运行模型，先确认能否复用 `sdk/` 的上下文协议，再决定是否扩展。
