"""
天气查询插件示例

展示如何使用 AllBot SDK 开发插件
"""

import sys
import os

# 添加SDK路径
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../sdk/python'))

# 检查运行模式
if '--mode=direct' in sys.argv:
    # 直接执行模式（无端口）
    from allbot_direct import run_direct, Context
else:
    # HTTP服务器模式（兼容旧版）
    from allbot_sdk import Context, start_plugin


async def handle(ctx: Context):
    """处理消息的主函数"""
    content = ctx.content

    # 天气预报 北京 3
    if content.startswith("天气预报"):
        parts = content.split()
        city = parts[1] if len(parts) > 1 else "北京"
        days = int(parts[2]) if len(parts) > 2 else 3

        forecast = await fetch_forecast(city, days)
        await ctx.reply(f"{city}未来{days}天天气预报：\n{forecast}")

    # 天气 北京
    elif content.startswith("天气"):
        city = content[2:].strip()

        if not city:
            # 使用连续对话
            await ctx.reply("请输入城市名：")
            city = await ctx.listen(60)

            if not city:
                await ctx.reply("超时，已取消")
                return

        weather = await fetch_weather(city)
        await ctx.reply(f"{city}的天气：{weather}")

        # QQ 平台发送天气图片
        if ctx.platform == "qq":
            await ctx.send_image(f"https://api.weather.com/{city}.png")

    # 帮助信息
    else:
        await ctx.reply(
            "天气插件使用方法：\n"
            "1. 天气 <城市> - 查询实时天气\n"
            "2. 天气预报 <城市> <天数> - 查询天气预报\n"
            "3. 天气 - 交互式查询"
        )


async def fetch_weather(city: str) -> str:
    """获取实时天气（示例实现）"""
    # 实际应用中应该调用真实的天气 API
    # response = await ctx.http.get(f"https://api.weather.com/{city}")

    # 模拟数据
    return f"晴天 25°C，空气质量良好"


async def fetch_forecast(city: str, days: int) -> str:
    """获取天气预报（示例实现）"""
    # 实际应用中应该调用真实的天气 API

    # 模拟数据
    forecast_data = []
    for i in range(days):
        forecast_data.append(f"第{i+1}天：晴转多云 20-28°C")

    return "\n".join(forecast_data)


if __name__ == '__main__':
    # 根据运行模式选择启动方式
    if '--mode=direct' in sys.argv:
        # 直接执行模式（无端口，支持并发）
        run_direct(handle)
    else:
        # HTTP服务器模式（兼容旧版）
        start_plugin()
