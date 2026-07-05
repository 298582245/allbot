import asyncio
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))

from account_ql_plugin import AccountQLPlugin, normalize_schedules


ACCOUNTS = [{"id": 1, "account_name": "账号1", "expires_at": "2999-01-01T00:00:00Z"}]


class FakeContext:
    def __init__(self, fake=False, result=None, accounts=None, listens=None):
        self.fake = fake
        self.result = result or {"status": "success", "task_id": "task-1"}
        self.accounts = accounts or []
        self.listens = list(listens or [])
        self.listen_timeouts = []
        self.replies = []
        self.buttons = []
        self.run_ql_scripts = []
        self.scheduled_tasks = []
        self.union_id = "union-1"
        self.platform = "test"
        self.user_id = "user-1"

    def meta(self, key):
        return "true" if key == "fake" and self.fake else ""

    def config(self, _key, default=None):
        return default

    async def reply(self, text):
        self.replies.append(text)

    async def send_buttons(self, text, buttons):
        self.buttons.append({"text": text, "buttons": buttons})
        self.replies.append(text)

    async def listen(self, timeout):
        self.listen_timeouts.append(timeout)
        if not self.listens:
            raise RuntimeError("unexpected listen")
        return self.listens.pop(0)

    async def run_ql_script(self, **payload):
        self.run_ql_scripts.append(payload)
        return self.result

    async def list_platform_admins(self):
        return [{"platform": "test", "adapter_id": "adapter-1", "user_id": "admin-1"}]

    async def set_scheduled_task(self, **payload):
        self.scheduled_tasks.append(payload)
        return payload

    def _request(self, _action, _expected_action="account_response"):
        return self.accounts


def make_plugin(ql=None, account=None):
    return AccountQLPlugin({
        "prefix": "测试",
        "table_name": "test_accounts",
        "env_name": "TEST_COOKIE",
        "ql": {"script": "task.py", **(ql or {})},
        "account": account or {},
    })


class AccountQLPluginScheduleTest(unittest.TestCase):
    def test_normalize_schedules_supports_multiple_run_items(self):
        schedules = normalize_schedules("云盘", {
            "run": {"task_key": "ydyp-exchange", "content": "云盘一键抢兑"},
            "runs": [{"task_key": "ydyp-default-run", "cron": "13 8,15 * * *", "content": "云盘一键运行"}],
        })
        self.assertEqual([item["task_key"] for item in schedules[:2]], ["ydyp-exchange", "ydyp-default-run"])
        self.assertEqual(schedules[0]["content"], "云盘一键抢兑")
        self.assertEqual(schedules[1]["content"], "云盘一键运行")
        self.assertEqual(schedules[1]["cron"], "13 8,15 * * *")

    def test_ensure_schedules_uses_task_count_as_default_max_count(self):
        plugin = AccountQLPlugin({
            "prefix": "测试",
            "table_name": "test_accounts",
            "env_name": "TEST_COOKIE",
            "ql": {"script": "task.py"},
            "schedules": {
                "run": {"task_key": "task-1", "content": "测试一键抢兑"},
                "runs": [{"task_key": "task-2", "content": "测试一键运行"}],
                "expire_check": {"task_key": "task-3", "content": "测试过期检测"},
                "ck_check": {"task_key": "task-4", "content": "测试CK检测"},
            },
        })
        ctx = FakeContext()
        asyncio.run(plugin.ensure_schedules(ctx))

        self.assertEqual(len(ctx.scheduled_tasks), 4)
        self.assertEqual({item["max_count"] for item in ctx.scheduled_tasks}, {4})


class AccountQLPluginRunHookTest(unittest.TestCase):
    def test_fake_scheduled_run_submits_script_task_by_default(self):
        called = False

        async def after_run(_ctx, _accounts, _result, _helpers):
            nonlocal called
            called = True

        plugin = make_plugin({"after_run": after_run})
        ctx = FakeContext(fake=True, result={"status": "running", "task_id": "task-1"})
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "all_authorized", "测试一键运行"))

        self.assertIs(ctx.run_ql_scripts[0]["wait"], False)
        self.assertFalse(called)
        self.assertIn("任务已提交", ctx.replies[-1])

    def test_wait_scheduled_true_waits_and_calls_after_run(self):
        hook_args = {}

        async def after_run(ctx, accounts, result, helpers):
            hook_args.update({"ctx": ctx, "accounts": accounts, "result": result, "helpers": helpers})

        plugin = make_plugin({"wait_scheduled": True, "after_run": after_run})
        ctx = FakeContext(fake=True)
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "all_authorized", "测试一键运行"))

        self.assertIs(ctx.run_ql_scripts[0]["wait"], True)
        self.assertIs(hook_args["ctx"], ctx)
        self.assertIs(hook_args["accounts"], ACCOUNTS)
        self.assertEqual(hook_args["result"]["status"], "success")
        self.assertEqual(hook_args["helpers"].run_mode, "all_authorized")
        self.assertEqual(hook_args["helpers"].runMode, "all_authorized")
        self.assertEqual(hook_args["helpers"].title, "测试一键运行")
        self.assertIs(hook_args["helpers"].is_scheduled, True)
        self.assertIs(hook_args["helpers"].isScheduled, True)

    def test_wait_scheduled_false_skips_after_run(self):
        called = False

        async def after_run(_ctx, _accounts, _result, _helpers):
            nonlocal called
            called = True

        plugin = make_plugin({"wait_scheduled": False, "after_run": after_run})
        ctx = FakeContext(fake=True, result={"status": "running", "task_id": "task-2"})
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "all_authorized", "测试一键运行"))

        self.assertIs(ctx.run_ql_scripts[0]["wait"], False)
        self.assertFalse(called)
        self.assertIn("任务已提交", ctx.replies[-1])

    def test_after_run_camel_case_alias_is_supported(self):
        called = False

        async def after_run(_ctx, _accounts, _result, _helpers):
            nonlocal called
            called = True

        plugin = make_plugin({"wait_scheduled": True, "afterRun": after_run})
        ctx = FakeContext(fake=True)
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "all_authorized", "测试一键运行"))
        self.assertTrue(called)

    def test_after_run_errors_are_swallowed_and_final_reply_still_executes(self):
        async def after_run(_ctx, _accounts, _result, _helpers):
            raise RuntimeError("hook failed")

        plugin = make_plugin({"wait_scheduled": True, "after_run": after_run})
        ctx = FakeContext(fake=True)
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "all_authorized", "测试一键运行"))
        self.assertIn("运行完成", ctx.replies[-1])

    def test_failed_script_replies_summary_without_account_query(self):
        async def query(_account, _ctx, _index):
            raise AssertionError("失败脚本不应继续查询账号")

        plugin = make_plugin(account={"query": query})
        ctx = FakeContext(result={"status": "failed", "task_id": 9, "error": "exit status 1", "output": "line1\nline2"})
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "current_user", "测试运行"))
        self.assertIn("运行失败", ctx.replies[-1])
        self.assertIn("任务ID：9", ctx.replies[-1])
        self.assertIn("错误：exit status 1", ctx.replies[-1])
        self.assertIn("line2", ctx.replies[-1])

    def test_all_authorized_run_skips_post_run_account_query_and_shows_summary(self):
        async def query(_account, _ctx, _index):
            return "账号详情"

        plugin = make_plugin(account={"query": query})
        ctx = FakeContext()
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "all_authorized", "测试一键运行"))
        self.assertFalse(any("运行后账号信息" in str(reply) for reply in ctx.replies))
        self.assertRegex(ctx.replies[-1], r"✅测试生活运行完成！共运行1个账号，耗时\d+\.\d{3}秒")

    def test_current_user_run_keeps_post_run_account_query_reply(self):
        async def query(_account, _ctx, _index):
            return "账号详情"

        plugin = make_plugin(account={"query": query})
        ctx = FakeContext()
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "current_user", "测试运行"))
        self.assertIn("账号详情", ctx.replies[-1])

    def test_after_run_is_not_called_for_non_all_authorized_run_modes(self):
        called = False

        async def after_run(_ctx, _accounts, _result, _helpers):
            nonlocal called
            called = True

        plugin = make_plugin({"wait_scheduled": True, "after_run": after_run})
        ctx = FakeContext(fake=True)
        asyncio.run(plugin.run_accounts(ctx, ACCOUNTS, "single_account", "测试账号运行"))
        self.assertFalse(called)

    def test_single_account_query_skips_account_selection(self):
        async def query(account, _ctx, _index):
            return f"详情：{account['account_name']}"

        plugin = make_plugin(account={"query": query})
        ctx = FakeContext(accounts=ACCOUNTS)
        asyncio.run(plugin.query_mine(ctx))

        self.assertEqual(ctx.listen_timeouts, [])
        self.assertIn("详情：账号1", ctx.replies[-1])

    def test_query_account_selection_paginates_every_20_accounts(self):
        many_accounts = [{"id": i + 1, "account_name": f"账号{i + 1}"} for i in range(21)]

        async def query(account, _ctx, _index):
            return f"详情：{account['account_name']}"

        plugin = make_plugin(account={"query": query})
        ctx = FakeContext(accounts=many_accounts, listens=["n", "21"])
        ctx.platform = "telegram"
        asyncio.run(plugin.query_mine(ctx))

        self.assertIn("[20] 账号20", ctx.buttons[0]["text"])
        self.assertNotIn("[21] 账号21", ctx.buttons[0]["text"])
        self.assertIn("第 1/2 页", ctx.buttons[0]["text"])
        self.assertIn("[21] 账号21", ctx.buttons[1]["text"])
        self.assertIn("详情：账号21", ctx.replies[-1])

    def test_telegram_query_selection_prompt_sends_buttons_when_supported(self):
        plugin = make_plugin(account={"query": lambda *_args: "详情"})
        ctx = FakeContext(accounts=[{"id": 1, "account_name": "账号1"}, {"id": 2, "account_name": "账号2"}], listens=["0"])
        ctx.platform = "telegram"
        sent = []

        async def send_buttons(text, buttons):
            sent.append({"text": text, "buttons": buttons})

        ctx.sendButtons = send_buttons
        asyncio.run(plugin.select_accounts_for_query(ctx, ctx.accounts))

        self.assertEqual(len(sent), 1)
        self.assertIn("请输入要查询的账号", sent[0]["text"])
        self.assertEqual(sent[0]["buttons"][0][0], {"text": "[0] 全部查询", "value": "0", "user_id": "user-1"})

    def test_qq_office_query_selection_prompt_sends_buttons_when_supported(self):
        plugin = make_plugin(account={"query": lambda *_args: "详情"})
        ctx = FakeContext(accounts=[{"id": 1, "account_name": "账号1"}, {"id": 2, "account_name": "账号2"}], listens=["1"])
        ctx.platform = "qq_office"
        sent = []

        async def send_buttons(text, buttons):
            sent.append({"text": text, "buttons": buttons})

        ctx.sendButtons = send_buttons
        asyncio.run(plugin.select_accounts_for_query(ctx, ctx.accounts))

        self.assertEqual(len(sent), 1)
        self.assertEqual(sent[0]["buttons"][1][0], {"text": "[1] 账号1", "value": "1", "user_id": "user-1"})

    def test_account_management_menu_sends_buttons_with_text_fallback(self):
        plugin = make_plugin()
        ctx = FakeContext(accounts=ACCOUNTS, listens=["q"])
        asyncio.run(plugin.list_accounts(ctx))
        self.assertEqual(len(ctx.buttons), 1)
        self.assertIn("回复序号可操作账号", ctx.buttons[0]["text"])
        self.assertEqual(ctx.buttons[0]["buttons"][0][0], {"text": "1. 账号1", "value": "1"})

    def test_delete_confirmation_sends_y_cancel_buttons(self):
        plugin = make_plugin()
        ctx = FakeContext(accounts=ACCOUNTS, listens=["q"])
        asyncio.run(plugin.delete_account(ctx, ACCOUNTS[0]))
        self.assertEqual(len(ctx.buttons), 1)
        self.assertIn("回复 y 确认", ctx.buttons[0]["text"])
        self.assertEqual(ctx.buttons[0]["buttons"][0][0]["value"], "y")
        self.assertIn("已取消删除", ctx.replies[-1])


if __name__ == "__main__":
    unittest.main()
