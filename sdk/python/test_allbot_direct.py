import asyncio
import json
import os
import subprocess
import sys
import textwrap
import unittest

sys.path.insert(0, os.path.dirname(__file__))

from allbot_direct import Context, PAY, WebRequest, WebResponse


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

        def fake_request(action, expected_action="db_response", allow_failure=False):
            calls.append({"action": action, "expected_action": expected_action, "allow_failure": allow_failure})
            return {"status": "paid", "order_no": "P1"}

        ctx._request = fake_request
        return ctx, calls

    def test_send_markdown_outputs_protocol_action(self):
        ctx, _ = self.make_context()
        actions = []
        ctx._send = lambda action: actions.append(action) or True
        asyncio.run(ctx.send_markdown("**hi**"))
        asyncio.run(ctx.sendMarkdown("## title"))
        self.assertEqual(actions, [
            {"action": "send_markdown", "markdown": "**hi**"},
            {"action": "send_markdown", "markdown": "## title"},
        ])

    def test_send_rich_outputs_protocol_action(self):
        ctx, _ = self.make_context()
        actions = []
        ctx._send = lambda action: actions.append(action) or True
        asyncio.run(ctx.send_rich(["中文", {"image": "https://example.com/a.png", "alt": "图"}, {"markdown": "**价格**"}], fallbackText="中文 图", prefer="markdown"))
        asyncio.run(ctx.replyRich([{"url": "https://example.com/b.png"}]))
        self.assertEqual(actions, [
            {"action": "send_rich", "parts": [{"type": "text", "text": "中文"}, {"type": "image", "url": "https://example.com/a.png", "alt": "图"}, {"type": "markdown", "markdown": "**价格**"}], "fallback_text": "中文 图", "prefer": "markdown"},
            {"action": "send_rich", "parts": [{"type": "image", "url": "https://example.com/b.png", "alt": ""}], "fallback_text": "", "prefer": "auto"},
        ])

    def test_send_rich_message_outputs_request(self):
        ctx, calls = self.make_context({"platform": "qq_office", "adapter_id": "7", "user_id": "current"})
        asyncio.run(ctx.sendRichMessage(platform="qq_office", userId="u1", groupId="g1", unionId="U1", parts=["你好"], prefer="split"))
        self.assertEqual(calls[0]["expected_action"], "send_rich_message_response")
        self.assertEqual(calls[0]["action"], {
            "action": "send_rich_message",
            "platform": "qq_office",
            "adapter_id": "7",
            "user_id": "u1",
            "group_id": "g1",
            "union_id": "U1",
            "parts": [{"type": "text", "text": "你好"}],
            "fallback_text": "",
            "prefer": "split",
        })

    def test_send_buttons_outputs_protocol_action(self):
        ctx, _ = self.make_context()
        actions = []
        ctx._send = lambda action: actions.append(action) or True
        asyncio.run(ctx.send_buttons("请选择", [[{"text": "A", "value": "1", "userId": "u1"}]]))
        asyncio.run(ctx.sendButtons("继续", [[{"text": "B", "value": "2"}, {"text": "", "value": "x"}]]))
        self.assertEqual(actions, [
            {"action": "send_buttons", "text": "请选择", "buttons": [[{"text": "A", "value": "1", "user_id": "u1"}]]},
            {"action": "send_buttons", "text": "继续", "buttons": [[{"text": "B", "value": "2"}]]},
        ])

    def test_send_message_includes_buttons_when_provided(self):
        ctx, calls = self.make_context({"platform": "telegram", "adapter_id": "1"})
        asyncio.run(ctx.send_message(platform="telegram", userId="u1", text="hi", buttons=[[{"text": "A", "value": "1"}, {"text": "", "value": "x"}]]))
        self.assertEqual(calls[0]["action"]["buttons"], [[{"text": "A", "value": "1"}]])

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

    def test_context_exposes_pay_helper_and_web_router(self):
        ctx, _ = self.make_context()
        self.assertIsInstance(ctx.pay, PAY)

        @ctx.web.get('/orders')
        def route(req):
            return WebResponse({"ok": True})

        self.assertEqual(ctx.web.match('GET', 'orders')["path"], '/orders')

    def test_web_request_parses_json_body_and_normalizes_data(self):
        req = WebRequest({"method": "post", "path": "orders/", "query": {"page": ["1"]}, "headers": {"accept": ["json"]}, "body": '{"id":1}'})
        self.assertEqual(req.method, 'POST')
        self.assertEqual(req.path, '/orders')
        self.assertEqual(req.query, {"page": "1"})
        self.assertEqual(req.headers, {"accept": "json"})
        self.assertEqual(asyncio.run(req.json()), {"id": 1})
        self.assertEqual(WebResponse({"ok": True}, 201).to_action(), {
            "action": "web_response",
            "status": 201,
            "headers": {"Content-Type": "application/json; charset=utf-8"},
            "json": {"ok": True},
        })

    def test_run_direct_dispatches_web_api_requests_to_registered_route(self):
        source = """
        from allbot_direct import run_direct
        async def handle(ctx):
            @ctx.web.post('/orders')
            async def route(req):
                data = await req.json()
                return req.json_response({'path': req.path, 'data': data}, 201)
        run_direct(handle)
        """
        result = subprocess.run(
            [sys.executable, "-c", textwrap.dedent(source)],
            cwd=os.path.dirname(__file__),
            env={**os.environ, "ALLBOT_PLUGIN_ID": "plugin-sdk-test"},
            input='{"event_type":"web_api","method":"POST","path":"/orders","body":"{\\"sku\\":\\"A\\"}"}\n',
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout), {
            "action": "web_response",
            "status": 201,
            "headers": {"Content-Type": "application/json; charset=utf-8"},
            "json": {"path": "/orders", "data": {"sku": "A"}},
        })

    def test_context_exposes_event_from_metadata_event_name(self):
        ctx, _ = self.make_context({"metadata": {
            "message_type": "event",
            "event_name": "GROUP_MEMBER_ADD",
            "qq_office_timestamp": "123456",
            "qq_office_group_openid": "group-openid",
            "qq_office_member_openid": "member-openid",
        }})
        self.assertEqual(ctx.event["name"], "GROUP_MEMBER_ADD")
        self.assertEqual(ctx.event["eventName"], "GROUP_MEMBER_ADD")
        self.assertEqual(ctx.event["groupOpenid"], "group-openid")
        self.assertEqual(ctx.event["member_openid"], "member-openid")
        self.assertEqual(ctx.event["timestamp"], "123456")

    def test_context_event_is_none_for_normal_qq_metadata(self):
        ctx, _ = self.make_context({"metadata": {"qq_office_event_type": "GROUP_MESSAGE_CREATE"}})
        self.assertIsNone(ctx.event)

    def test_run_script_sends_runtime_profile(self):
        ctx, calls = self.make_context()
        asyncio.run(ctx.run_script(runtime="python", runtime_profile="python310", script="task.py"))
        self.assertEqual(calls[0]["expected_action"], "script_response")
        self.assertEqual(calls[0]["action"]["runtime_profile"], "python310")

    def test_run_script_accepts_camel_runtime_profile(self):
        ctx, calls = self.make_context()
        asyncio.run(ctx.run_script(runtime="python", runtimeProfile="python311", script="task.py"))
        self.assertEqual(calls[0]["action"]["runtime_profile"], "python311")

    def test_run_script_returns_failed_result_without_raising(self):
        ctx = Context({"plugin_id": "plugin-sdk", "union_id": "union-sdk"})
        calls = []

        def fake_request(action, expected_action="db_response", allow_failure=False):
            calls.append({"action": action, "expected_action": expected_action, "allow_failure": allow_failure})
            return {"status": "failed", "task_id": 7, "error": "exit status 1"}

        ctx._request = fake_request
        result = asyncio.run(ctx.run_script(runtime="nodejs", script="task.js", wait=True))
        self.assertEqual(result["status"], "failed")
        self.assertEqual(result["error"], "exit status 1")
        self.assertTrue(calls[0]["allow_failure"])

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
