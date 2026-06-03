# 翻译助手插件

跨平台翻译插件示例，支持中英互译。

## 功能特性

- ✅ 中英互译（自动检测语言）
- ✅ 跨平台支持（QQ、Telegram、微信）
- ✅ 使用免费翻译 API（LibreTranslate）
- ✅ 简洁的命令格式

## 使用方法

### 基本用法

```
翻译 hello world
translate 你好世界
fanyi How are you?
```

### 命令格式

支持三种触发词：
- `翻译 <文本>` - 中文命令
- `translate <文本>` - 英文命令
- `fanyi <文本>` - 拼音命令

### 示例对话

**用户**：翻译 hello
**机器人**：
```
🌐 英译中
原文：hello
译文：你好
```

**用户**：translate 机器人
**机器人**：
```
🌐 中译英
原文：机器人
译文：robot
```

## 技术实现

### 语言检测

通过正则表达式检测文本中是否包含中文字符：
```python
def contains_chinese(text):
    return bool(re.search(r'[一-鿿]', text))
```

### 翻译 API

使用 LibreTranslate 公共 API：
- API 地址：https://libretranslate.com/translate
- 免费使用，无需 API Key
- 支持多种语言对

### 依赖管理

插件依赖在 `plugin.json` 中声明：
```json
{
  "dependencies": {
    "requests": "2.31.0"
  }
}
```

框架会自动安装到全局 Python 环境。

## 跨平台兼容性

此插件展示了 AllBot 的跨平台特性：

1. **统一 API**：使用相同的 `ctx` API 在所有平台工作
2. **平台无关**：代码无需针对特定平台修改
3. **自动适配**：框架自动处理平台差异

### 平台支持

| 平台 | 状态 | 说明 |
|------|------|------|
| QQ | ✅ 支持 | 基于 go-cqhttp |
| Telegram | ✅ 支持 | Bot API 长轮询 |
| 微信 | 🚧 开发中 | 企业微信/公众号 |

## 安装部署

### 1. 复制插件到插件目录

```bash
cp -r examples/translator plugins/translator
```

### 2. 重启 AllBot

框架会自动：
- 加载插件配置
- 安装依赖（requests）
- 启动插件进程
- 注册消息路由

### 3. 测试插件

在任意支持的平台发送：
```
翻译 hello
```

## 自定义扩展

### 更换翻译 API

修改 `translate_text` 函数，替换为其他翻译服务：

```python
async def translate_text(text, from_lang, to_lang):
    # 使用百度翻译 API
    url = "https://fanyi-api.baidu.com/api/trans/vip/translate"
    # ... 实现代码
```

### 支持更多语言

修改语言检测逻辑：

```python
def detect_language(text):
    if re.search(r'[一-鿿]', text):
        return 'zh'
    elif re.search(r'[぀-ゟ゠-ヿ]', text):
        return 'ja'  # 日文
    elif re.search(r'[가-힯]', text):
        return 'ko'  # 韩文
    else:
        return 'en'
```

### 添加翻译历史

使用 Context Storage API：

```python
async def handle(ctx):
    # 保存翻译历史
    history = await ctx.storage.get("history") or []
    history.append({
        "text": text,
        "result": result,
        "time": time.time()
    })
    await ctx.storage.set("history", history[-10:])  # 保留最近10条
```

## 注意事项

1. **API 限制**：LibreTranslate 公共实例有速率限制，生产环境建议自建或使用付费 API
2. **网络超时**：翻译请求设置了 10 秒超时，避免长时间等待
3. **错误处理**：API 失败时会返回友好的错误提示

## 许可证

MIT License
