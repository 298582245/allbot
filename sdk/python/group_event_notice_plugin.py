import os
import sys

sys.path.insert(0, os.path.dirname(__file__))

from allbot_direct import run_direct


def get_event_name(ctx):
    event = ctx.event or {}
    return event.get("name") or ctx.metadata.get("event_name") or ctx.content


def display_member(ctx):
    event = ctx.event or {}
    return event.get("memberOpenid") or event.get("member_openid") or ctx.user_id or "未知成员"


async def handle(ctx):
    event_name = get_event_name(ctx)
    if event_name == "GROUP_MEMBER_ADD":
        await ctx.reply(f"欢迎新成员加入群聊：{display_member(ctx)}")
    elif event_name == "GROUP_MEMBER_REMOVE":
        await ctx.reply(f"成员已退出群聊：{display_member(ctx)}")
    elif event_name == "GROUP_ADD_ROBOT":
        await ctx.reply("机器人已加入本群，群事件通知插件已启用。")


if __name__ == "__main__":
    run_direct(handle)
