import asyncio
import os
import subprocess
import sys
import textwrap
import unittest

sys.path.insert(0, os.path.dirname(__file__))

from allbot_direct import Context, PAY


class PluginStdoutRedirectTest(unittest.TestCase):
    def run_python_sdk_snippet(self, source, plugin_env=False):
        env = os.environ.copy()
        if plugin_env:
            env["ALLBOT_PLUGIN_ID"] = "plugin-sdk-test"
        else:
            env.pop("ALLBOT_PLUGIN_ID", None)
        return subprocess.run(
            [sys.executable, "-c", textwrap.dedent(source)],
            cwd=os.path.dirname(__file__),
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_plugin_print_goes_to_stderr_and_protocol_stays_stdout(self):
        result = self.run_python_sdk_snippet(
            """
            from allbot_direct import Context
            print('debug')
            Context({'plugin_id': 'plugin-sdk-test'})._send({'action': 'reply', 'text': 'ok'})
            """,
            plugin_env=True,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, '{"action": "reply", "text": "ok"}\n')
        self.assertEqual(result.stderr, 'debug\n')

    def test_non_plugin_environment_keeps_stdout(self):
        result = self.run_python_sdk_snippet(
            """
            import allbot_direct
            print('normal')
            """,
            plugin_env=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, 'normal\n')
        self.assertEqual(result.stderr, '')


class PaySdkTest(unittest.TestCase):
    def make_context(self, data=None):
        payload = {"plugin_id": "plugin-sdk", "union_id": "union-sdk"}
        if data:
            payload.update(data)
        ctx = Context(payload)
        calls = []

        def fake_request(action, expected_action="db_response"):
            calls.append({"action": action, "expected_action": expected_action})
            return {"status": "paid", "order_no": "P1"}

        ctx._request = fake_request
        return ctx, calls

    def test_pay_with_context_sends_payment_wait(self):
        ctx, calls = self.make_context()
        result = asyncio.run(PAY(ctx).wait_pay("测试消费", "1.00", timeout=60, metadata={"k": "v"}))
        self.assertEqual(result["order_no"], "P1")
        self.assertEqual(calls[0]["expected_action"], "payment_response")
        self.assertEqual(calls[0]["action"], {
            "action": "payment_wait",
            "subject": "测试消费",
            "amount": "1.00",
            "timeout": 60,
            "methods": [],
            "metadata": {"k": "v"},
            "remark": "",
        })

    def test_pay_uses_current_context_and_camel_alias(self):
        _, calls = self.make_context()
        result = asyncio.run(PAY().waitPay("别名消费", 2, timeout=45, metadata={"alias": True}))
        self.assertEqual(result["status"], "paid")
        self.assertEqual(calls[0]["expected_action"], "payment_response")
        self.assertEqual(calls[0]["action"]["subject"], "别名消费")
        self.assertEqual(calls[0]["action"]["amount"], "2")
        self.assertEqual(calls[0]["action"]["timeout"], 45)
        self.assertEqual(calls[0]["action"]["metadata"], {"alias": True})

    def test_context_exposes_pay_helper(self):
        ctx, _ = self.make_context()
        self.assertIsInstance(ctx.pay, PAY)

    def test_run_script_sends_runtime_profile(self):
        ctx, calls = self.make_context()
        asyncio.run(ctx.run_script(runtime="python", runtime_profile="python310", script="task.py"))
        self.assertEqual(calls[0]["expected_action"], "script_response")
        self.assertEqual(calls[0]["action"]["runtime_profile"], "python310")

    def test_run_script_accepts_camel_runtime_profile(self):
        ctx, calls = self.make_context()
        asyncio.run(ctx.run_script(runtime="python", runtimeProfile="python311", script="task.py"))
        self.assertEqual(calls[0]["action"]["runtime_profile"], "python311")

    def test_push_sends_message_action(self):
        ctx, calls = self.make_context()
        result = asyncio.run(ctx.push("u1", "g1", "hello", "telegram", "2"))
        self.assertEqual(result["order_no"], "P1")
        self.assertEqual(calls[0]["expected_action"], "send_message_response")
        self.assertEqual(calls[0]["action"], {
            "action": "send_message",
            "platform": "telegram",
            "adapter_id": "2",
            "user_id": "u1",
            "group_id": "g1",
            "union_id": "",
            "text": "hello",
        })

    def test_push_omitted_adapter_id_stays_empty(self):
        ctx, calls = self.make_context({"platform": "telegram", "adapter_id": "9", "user_id": "current-user"})
        asyncio.run(ctx.push("u1", "", "hello"))
        self.assertEqual(calls[0]["expected_action"], "send_message_response")
        self.assertEqual(calls[0]["action"]["platform"], "telegram")
        self.assertEqual(calls[0]["action"]["adapter_id"], "")
        self.assertEqual(calls[0]["action"]["user_id"], "u1")
        self.assertEqual(calls[0]["action"]["group_id"], "")
        self.assertEqual(calls[0]["action"]["union_id"], "")

    def test_send_message_same_platform_inherits_context_adapter_id(self):
        ctx, calls = self.make_context({"platform": "qq_office", "adapter_id": "3", "user_id": "admin"})
        asyncio.run(ctx.send_message(platform="qq_office", userId="target", text="hi"))
        self.assertEqual(calls[0]["expected_action"], "send_message_response")
        self.assertEqual(calls[0]["action"]["platform"], "qq_office")
        self.assertEqual(calls[0]["action"]["adapter_id"], "3")
        self.assertEqual(calls[0]["action"]["user_id"], "target")

    def test_send_message_cross_platform_does_not_inherit_context_adapter_id(self):
        ctx, calls = self.make_context({"platform": "qq_office", "adapter_id": "3", "user_id": "admin"})
        asyncio.run(ctx.send_message(platform="telegram", unionId="U_qq_123", text="hi"))
        self.assertEqual(calls[0]["expected_action"], "send_message_response")
        self.assertEqual(calls[0]["action"]["platform"], "telegram")
        self.assertEqual(calls[0]["action"]["adapter_id"], "")
        self.assertEqual(calls[0]["action"]["union_id"], "U_qq_123")

    def test_send_message_explicit_adapter_id_still_wins(self):
        ctx, calls = self.make_context({"platform": "qq_office", "adapter_id": "3"})
        asyncio.run(ctx.send_message(platform="telegram", adapterId="8", userId="u1", text="hi"))
        self.assertEqual(calls[0]["expected_action"], "send_message_response")
        self.assertEqual(calls[0]["action"]["platform"], "telegram")
        self.assertEqual(calls[0]["action"]["adapter_id"], "8")
        self.assertEqual(calls[0]["action"]["user_id"], "u1")

    def test_push_accepts_snake_case_options(self):
        ctx, calls = self.make_context()
        asyncio.run(ctx.push(user_id="u2", group_id="g2", text="hi", platform="telegram", adapter_id="3"))
        self.assertEqual(calls[0]["action"], {
            "action": "send_message",
            "platform": "telegram",
            "adapter_id": "3",
            "user_id": "u2",
            "group_id": "g2",
            "union_id": "",
            "text": "hi",
        })

    def test_push_accepts_explicit_union_id(self):
        ctx, calls = self.make_context({"platform": "telegram"})
        asyncio.run(ctx.push(union_id="U_qq_123", content="hi"))
        self.assertEqual(calls[0]["action"]["user_id"], "")
        self.assertEqual(calls[0]["action"]["union_id"], "U_qq_123")
        self.assertEqual(calls[0]["action"]["platform"], "telegram")

    def test_push_treats_union_prefixed_user_id_as_union_id(self):
        ctx, calls = self.make_context({"platform": "telegram"})
        asyncio.run(ctx.push("union:U_qq_123", content="hi"))
        self.assertEqual(calls[0]["action"]["user_id"], "")
        self.assertEqual(calls[0]["action"]["union_id"], "U_qq_123")


if __name__ == "__main__":
    unittest.main()
