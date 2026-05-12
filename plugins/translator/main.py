"""
翻译助手插件
支持中英互译，展示跨平台兼容性
"""
import re
import requests


async def handle(ctx):
    """处理翻译请求"""
    content = ctx.content.strip()

    # 提取要翻译的文本
    # 支持格式：翻译 hello / translate hello / fanyi 你好 / fanyi你好
    match = re.match(r'^(翻译|translate|fanyi)\s*(.+)$', content, re.IGNORECASE)
    if not match:
        await ctx.reply("❌ 格式错误\n用法：翻译 <文本>")
        return

    text = match.group(2).strip()

    # 检测语言并翻译
    if contains_chinese(text):
        # 中文 -> 英文
        result = await translate_text(text, 'zh', 'en')
        if result:
            await ctx.reply(f"🌐 中译英\n原文：{text}\n译文：{result}")
    else:
        # 英文 -> 中文
        result = await translate_text(text, 'en', 'zh')
        if result:
            await ctx.reply(f"🌐 英译中\n原文：{text}\n译文：{result}")


def contains_chinese(text):
    """检测文本是否包含中文"""
    return bool(re.search(r'[\u4e00-\u9fff]', text))


async def translate_text(text, from_lang, to_lang):
    """
    使用免费翻译 API 进行翻译
    这里使用 LibreTranslate 的公共实例作为示例
    """
    try:
        # 使用 LibreTranslate 公共 API
        url = "https://libretranslate.com/translate"

        payload = {
            "q": text,
            "source": from_lang,
            "target": to_lang,
            "format": "text"
        }

        response = requests.post(url, json=payload, timeout=10)

        if response.status_code == 200:
            data = response.json()
            return data.get('translatedText', '')
        else:
            print(f"翻译 API 返回错误: {response.status_code}", file=sys.stderr)
            return None

    except requests.exceptions.Timeout:
        print("翻译请求超时", file=sys.stderr)
        return None
    except Exception as e:
        print(f"翻译出错: {e}", file=sys.stderr)
        return None


# 直接执行模式入口
if __name__ == '__main__':
    import sys
    import os

    # 添加SDK路径
    sdk_path = os.path.join(os.path.dirname(__file__), '../../sdk/python')
    sys.path.insert(0, sdk_path)

    from allbot_direct import run_direct

    # 运行插件
    run_direct(handle)
