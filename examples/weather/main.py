# -*- coding: utf-8 -*-
"""
天气查询插件示例

展示如何使用 AllBot Python Direct SDK 开发插件。
"""

import os
import sys


sdk_path = os.path.join(os.path.dirname(__file__), "../../sdk/python")
sys.path.insert(0, sdk_path)

from allbot_direct import run_direct


async def handle(ctx):
    """处理消息的主函数。"""
    content = ctx.content.strip()

    if content.startswith("天气预报"):
        parts = content.split()
        city = parts[1] if len(parts) > 1 else "北京"
        days = parse_days(parts[2] if len(parts) > 2 else "3")

        forecast = await fetch_forecast(city, days)
        await ctx.reply(f"{city}未来{days}天天气预报：\n{forecast}")
        return

    if content.startswith("天气"):
        city = ctx.args("天气")

        if not city:
            await ctx.reply("请输入城市名：")
            city = await ctx.listen(60)

            if not city:
                await ctx.reply("超时，已取消")
                return

            city = city.strip()

        weather = await fetch_weather(city)
        await ctx.reply(f"{city}的天气：{weather}")
        return

    await ctx.reply(
        "天气插件使用方法：\n"
        "1. 天气 <城市> - 查询实时天气\n"
        "2. 天气预报 <城市> <天数> - 查询天气预报\n"
        "3. 天气 - 交互式查询"
    )


def parse_days(value: str) -> int:
    """解析预报天数，限制在 1 到 7 天。"""
    try:
        days = int(value)
    except ValueError:
        return 3
    return max(1, min(days, 7))


async def fetch_weather(city: str) -> str:
    """获取实时天气（示例实现）。"""
    return "晴天 25°C，空气质量良好"


async def fetch_forecast(city: str, days: int) -> str:
    """获取天气预报（示例实现）。"""
    forecast_data = []
    for index in range(days):
        forecast_data.append(f"第{index + 1}天：晴转多云 20-28°C")
    return "\n".join(forecast_data)


if __name__ == "__main__":
    run_direct(handle)
