# AllBot Python Direct SDK

AllBot 当前只保留 Direct 模式插件通信。插件进程通过 stdin 接收消息 JSON，通过 stdout 输出 JSON 行指令。

## 快速开始

### 1. 创建插件目录

```
my-plugin/
  ├─ plugin.json
  └─ main.py
```

### 2. 配置文件

```json
{
  "name": "我的插件",
  "version": "1.0.0",
  "runtime": "python",
  "entry": "main.py",
  "platforms": ["qq", "wechat", "telegram"],
  "trigger": "你好.*",
  "enabled": true
}
```

### 3. 插件代码

```python
import os
import sys

sdk_path = os.path.join(os.path.dirname(__file__), "../../sdk/python")
sys.path.insert(0, sdk_path)

from allbot_direct import run_direct


async def handle(ctx):
    if ctx.content.startswith("你好"):
        await ctx.reply("你好！我是机器人")


if __name__ == "__main__":
    run_direct(handle)
```

## Context 字段

```python
ctx.plugin_id      # 插件 ID
ctx.adapter_id     # 机器人适配器 ID，同一平台多机器人时用于区分
ctx.platform       # 平台：qq、wechat、telegram
ctx.user_id        # 发送者 ID
ctx.group_id       # 群组 ID，私聊为空
ctx.content        # 消息内容
ctx.text           # ctx.content 的别名
ctx.message_id     # 消息 ID
ctx.metadata       # 平台原始扩展数据
```

## Context 方法

```python
await ctx.reply("文本消息")
await ctx.send_text("文本消息")
await ctx.send_image("https://example.com/image.png")
await ctx.send_file("/path/to/file.txt")
reply = await ctx.listen(60)
await ctx.set_data_view("plugin_table", "数据视图", "插件数据", "说明", ["id", "name"])
await ctx.fake_message("telegram", "123456", "-100888888", "天气 北京")

await ctx.db.create_table("users", [
    {"name": "user_id", "type": "TEXT"},
    {"name": "nickname", "type": "TEXT"},
    {"name": "score", "type": "INTEGER"}
])
await ctx.db.set_view("users", view_name="用户积分", group_name="插件数据", description="插件保存的用户积分")
row_id = await ctx.db.insert("users", {"user_id": ctx.user_id, "nickname": "小明", "score": 10})
rows = await ctx.db.query("users", filters=[{"field": "user_id", "op": "=", "value": ctx.user_id}], order_by="id", order_dir="DESC", page=1, size=20)
first = await ctx.db.first("users", filters=[{"field": "user_id", "value": ctx.user_id}])
await ctx.db.update("users", row_id, {"score": 20})
await ctx.db.delete("users", row_id)

ctx.is_group()      # 是否群聊
ctx.is_private()    # 是否私聊
ctx.is_admin()      # 是否为后台设置的平台管理员
ctx.chat_id()       # 群聊返回群号，私聊返回用户号
ctx.args("天气")    # 去掉指令前缀后的参数
ctx.meta("chat_id") # 获取平台扩展字段
```

## 伪造收到消息

定时任务或插件内部联动时，可以用伪造消息让系统按正常消息流程重新匹配插件。

```python
await ctx.fake_message(
    platform="telegram",
    user_id="123456",
    group_id="-100888888",  # 私聊传空字符串
    content="天气 北京"
)
```

```javascript
await ctx.fakeMessage({
  platform: 'telegram',
  userId: '123456',
  groupId: '-100888888', // 私聊传空字符串
  content: '天气 北京'
});
```

## Node.js 速查

```javascript
const path = require('path');
const { runDirect } = require(path.join(__dirname, '../../sdk/nodejs/allbot_direct'));

async function handle(ctx) {
  if (ctx.content.startsWith('你好')) {
    await ctx.reply('你好！');
  }

  if (ctx.content === '管理命令' && !ctx.isAdmin()) {
    await ctx.reply('只有管理员可以使用该命令');
    return;
  }

  const keyword = ctx.args('查询');
  if (keyword) {
    await ctx.sendText(`正在查询：${keyword}`);
  }

  await ctx.db.createTable('records', [
    { name: 'user_id', type: 'TEXT' },
    { name: 'content', type: 'TEXT' }
  ]);
  await ctx.db.setView('records', { viewName: '查询记录', groupName: '插件数据' });
  const rowId = await ctx.db.insert('records', { user_id: ctx.userId, content: ctx.content });
  const page = await ctx.db.query('records', { filters: [{ field: 'user_id', value: ctx.userId }], orderBy: 'id', orderDir: 'DESC' });
  await ctx.db.update('records', rowId, { content: '已更新' });
}

runDirect(handle);
```

## 数据库封装说明

- `ctx.db` 只操作当前插件私有表，真实表名自动变成 `plugin_<插件ID>_<表名>`，避免不同插件互相覆盖。
- `create_table/createTable` 支持字段类型：`TEXT`、`INTEGER`、`REAL`、`BLOB`、`DATETIME`、`BOOLEAN`。
- `query` 推荐使用 `filters` 结构化条件和 `order_by/order_dir` 排序；兼容的 `where` 只支持字段、占位符和 `AND` 组成的安全表达式。
- 查询结果格式与后台数据管理一致：`rows`、`columns`、`total`、`page`、`size`。

## 多轮对话示例

```python
async def handle(ctx):
    if ctx.content == "注册":
        await ctx.reply("请输入用户名：")
        username = await ctx.listen(60)

        if not username:
            await ctx.reply("超时，已取消")
            return

        await ctx.reply(f"注册成功：{username}")
```

## 外部 API 示例

Direct SDK 不再内置 HTTP 代理。插件需要调用外部接口时，直接使用语言生态中的依赖，例如 `requests`。

```python
import requests


async def handle(ctx):
    if ctx.content.startswith("天气 "):
        city = ctx.content[3:].strip()
        response = requests.get(f"https://api.example.com/weather?city={city}", timeout=10)
        data = response.json()
        await ctx.reply(f"{city}的天气：{data['weather']}")
```
