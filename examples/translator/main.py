# -*- coding: utf-8 -*-
"""
翻译助手插件示例

支持中英互译，展示 AllBot Python Direct SDK 的基础用法。
"""

import os
import re
import sys

import requests


sdk_path = os.path.join(os.path.dirname(__file__), "../../sdk/python")
sys.path.insert(0, sdk_path)

from allbot_direct import run_direct


async def handle(ctx):
    """处理翻译请求。"""
    content = ctx.content.strip()

    match = re.match(r"^(翻译|translate|fanyi)\s+(.+)$", content, re.IGNORECASE)
    if not match:
        await ctx.reply("❌ 格式错误\n用法：翻译 <文本>")
        return

    text = match.group(2).strip()

    if contains_chinese(text):
        result = await translate_text(text, "zh", "en")
        direction = "中译英"
    else:
        result = await translate_text(text, "en", "zh")
        direction = "英译中"

    if result:
        await ctx.reply(f"🌐 {direction}\n原文：{text}\n译文：{result}")
    else:
        await ctx.reply("❌ 翻译失败，请稍后重试")


def contains_chinese(text: str) -> bool:
    """检测文本是否包含中文。"""
    return bool(re.search(r"[一-鿿]", text))


async def translate_text(text: str, from_lang: str, to_lang: str):
    """使用 LibreTranslate 公共接口进行翻译。"""
    try:
        payload = {
            "q": text,
            "source": from_lang,
            "target": to_lang,
            "format": "text",
        }
        response = requests.post("https://libretranslate.com/translate", json=payload, timeout=10)
        if response.status_code == 200:
            data = response.json()
            return data.get("translatedText", "")

        print(f"翻译 API 返回错误: {response.status_code}", file=sys.stderr)
        return None
    except requests.exceptions.Timeout:
        print("翻译请求超时", file=sys.stderr)
        return None
    except Exception as error:
        print(f"翻译出错: {error}", file=sys.stderr)
        return None


if __name__ == "__main__":
    run_direct(handle)
