# AllBot Python SDK

极简的 Python 插件开发 SDK

## 安装

```bash
pip install allbot-sdk
```

## 快速开始

### 1. 创建插件目录

```
my-plugin/
  ├─ plugin.json
  └─ main.py
```

### 2. 配置文件（plugin.json）

```json
{
  "name": "我的插件",
  "version": "1.0.0",
  "runtime": "python",
  "entry": "main.py",
  "platforms": ["qq", "wechat", "telegram"],
  "trigger": "你好.*"
}
```

### 3. 插件代码（main.py）

```python
async def handle(ctx):
    """框架会调用这个函数，传入消息上下文"""
    if ctx.content == "你好":
        await ctx.reply("你好！我是机器人")
    elif ctx.content.startswith("你好 "):
        name = ctx.content[3:]
        await ctx.reply(f"你好，{name}！")
```

## API 参考

### Context 对象

```python
# 消息信息
ctx.platform        # 平台：'qq' | 'wechat' | 'telegram'
ctx.user_id         # 发送者 ID
ctx.group_id        # 群组 ID（私聊为空）
ctx.content         # 消息内容
ctx.message_id      # 消息 ID

# 发送消息
await ctx.reply("文本消息")
await ctx.send_image("https://example.com/image.png")
await ctx.send_file("/path/to/file")

# 连续对话
city = await ctx.listen(60)  # 等待 60 秒

# 获取信息
user_info = await ctx.get_user_info()
group_info = await ctx.get_group_info()

# 平台特定功能
await ctx.at_user(user_id)  # QQ/微信

# 数据存储
await ctx.storage.set("key", "value")
value = await ctx.storage.get("key")

# HTTP 请求
response = await ctx.http.get("https://api.example.com")
response = await ctx.http.post("https://api.example.com", data="...")
```

## 示例

### 多轮对话

```python
async def handle(ctx):
    if ctx.content == "注册":
        await ctx.reply("请输入用户名：")
        username = await ctx.listen(60)

        if not username:
            await ctx.reply("超时")
            return

        await ctx.reply("请输入密码：")
        password = await ctx.listen(60)

        if not password:
            await ctx.reply("超时")
            return

        # 保存用户信息
        await ctx.storage.set(f"user:{username}", password)
        await ctx.reply("注册成功！")
```

### 调用外部 API

```python
async def handle(ctx):
    if ctx.content.startswith("天气 "):
        city = ctx.content[3:]

        # 调用天气 API
        response = await ctx.http.get(f"https://api.weather.com/{city}")
        data = json.loads(response["body"])

        await ctx.reply(f"{city}的天气：{data['weather']}")
```
