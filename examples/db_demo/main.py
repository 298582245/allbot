import os
import sys

sdk_path = os.path.join(os.path.dirname(__file__), "../../sdk/python")
sys.path.insert(0, sdk_path)

from allbot_direct import run_direct


async def ensure_table(ctx):
    await ctx.db.create_table("bills", [
        {"name": "user_id", "type": "TEXT"},
        {"name": "chat_id", "type": "TEXT"},
        {"name": "title", "type": "TEXT"},
        {"name": "amount", "type": "REAL"},
    ])
    await ctx.db.set_view(
        "bills",
        view_name="记账示例",
        group_name="插件数据",
        description="数据库示例插件保存的记账数据",
        columns=["id", "user_id", "chat_id", "title", "amount", "created_at"],
    )


async def handle(ctx):
    await ensure_table(ctx)

    if ctx.content.startswith("记账"):
        text = ctx.args("记账")
        parts = text.split()
        if len(parts) < 2:
            await ctx.reply("用法：记账 午饭 18.5")
            return

        title = parts[0]
        try:
            amount = float(parts[1])
        except ValueError:
            await ctx.reply("金额必须是数字，例如：记账 午饭 18.5")
            return

        row_id = await ctx.db.insert("bills", {
            "user_id": ctx.user_id,
            "chat_id": ctx.chat_id(),
            "title": title,
            "amount": amount,
        })
        await ctx.reply(f"已记账 #{row_id}：{title} {amount:.2f} 元")
        return

    if ctx.content.startswith("账单"):
        result = await ctx.db.query(
            "bills",
            filters=[{"field": "chat_id", "value": ctx.chat_id()}],
            order_by="id",
            order_dir="DESC",
            page=1,
            size=5,
        )
        rows = result.get("rows") or []
        if not rows:
            await ctx.reply("暂无账单，发送：记账 午饭 18.5")
            return

        total = sum(float(row.get("amount") or 0) for row in rows)
        lines = ["最近 5 条账单："]
        for row in rows:
            lines.append(f"#{row.get('id')} {row.get('title')} {float(row.get('amount') or 0):.2f} 元")
        lines.append(f"小计：{total:.2f} 元")
        await ctx.reply("\n".join(lines))
        return

    if ctx.content.startswith("清空账单"):
        await ctx.db.clear("bills")
        await ctx.reply("账单示例表已清空")


if __name__ == "__main__":
    run_direct(handle)
