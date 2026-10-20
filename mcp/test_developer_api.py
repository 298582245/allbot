"""AllBot developer REST API and MCP smoke tests.

Run against a local AllBot instance with:
    python mcp/test_developer_api.py

Override the address with ALLBOT_URL when needed. The credentials below are
the three test applications supplied for the local instance.
"""

from __future__ import annotations

import base64
import json
import os
import unittest
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


BASE_URL = os.environ.get("ALLBOT_URL", "http://127.0.0.1:3000").rstrip("/")
APPS = {
    "limited": {
        "app_key": "ab_8Koxub54fC9kvuiCKp3fMNSl",
        "app_secret": "X7w1-nb1jR9MwlkntqApu7y3Tde6Y_qXg-UxLcfLzyg",
        "modules": {"system", "settings", "plugins", "adapters"},
    },
    "full": {
        "app_key": "ab_x3yQUCtgsVZvW1JhvbF2zpUQ",
        "app_secret": "jdHLxM1MP7PNPRDxhAw6T74tpWiq4L7oWCh_3YdmNjI",
    },
    "empty": {
        "app_key": "ab_PinddLpQznSzXtftknKIt2I4",
        "app_secret": "qy0xvvVKYzp986RJmGs7Jn5NNwwzIzttQooDh4Tfd1Q",
    },
}


def request_json(path: str, method: str = "GET", headers: dict | None = None, body: object | None = None):
    request_headers = {"Accept": "application/json", **(headers or {})}
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        request_headers.setdefault("Content-Type", "application/json")
    request = Request(BASE_URL + path, data=data, headers=request_headers, method=method)
    try:
        with urlopen(request, timeout=10) as response:
            status = response.status
            raw = response.read()
    except HTTPError as error:
        status = error.code
        raw = error.read()
    except URLError as error:
        raise AssertionError(f"无法连接 {BASE_URL}: {error.reason}") from error
    try:
        payload = json.loads(raw.decode("utf-8")) if raw else None
    except json.JSONDecodeError:
        payload = raw.decode("utf-8", errors="replace")
    return status, payload


def basic_auth(app: dict) -> str:
    value = f"{app['app_key']}:{app['app_secret']}".encode("utf-8")
    return "Basic " + base64.b64encode(value).decode("ascii")


def bearer(token: str) -> dict:
    return {"Authorization": f"Bearer {token}"}


def module_names(capabilities: list[dict]) -> set[str]:
    return {str(item.get("code", "")).split(":", 1)[0] for item in capabilities}


def mcp_request(auth: str, payload: dict):
    return request_json(
        "/api/v1/mcp",
        method="POST",
        headers={
            "Authorization": auth,
            "Accept": "application/json, text/event-stream",
        },
        body=payload,
    )


def mcp_result(payload: dict) -> dict:
    if not isinstance(payload, dict) or "error" in payload:
        raise AssertionError(f"MCP JSON-RPC 响应异常: {payload}")
    return payload.get("result", {})


class DeveloperAPIAndMCPTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tokens = {}
        for name, app in APPS.items():
            status, payload = request_json(
                "/api/v1/open-api/token",
                method="POST",
                body={"app_key": app["app_key"], "app_secret": app["app_secret"]},
            )
            if status != 200 or not isinstance(payload, dict) or not payload.get("access_token"):
                raise unittest.SkipTest(f"无法为 {name} 应用换取令牌，HTTP {status}: {payload}")
            cls.tokens[name] = payload["access_token"]

    def test_rest_authentication_contract(self):
        status, payload = request_json("/api/v1/capabilities")
        self.assertEqual(status, 401)
        self.assertEqual(payload.get("error"), "缺少授权令牌")

        status, payload = request_json(
            "/api/v1/capabilities",
            headers={"Authorization": basic_auth(APPS["limited"])},
        )
        self.assertEqual(status, 401)
        self.assertEqual(payload.get("error"), "缺少授权令牌")

        status, payload = request_json(
            "/api/v1/capabilities",
            headers=bearer(APPS["limited"]["app_secret"]),
        )
        self.assertEqual(status, 401)
        self.assertEqual(payload.get("error"), "令牌无效或已过期")

    def test_rest_capabilities_and_module_permissions(self):
        for name, expected in (("limited", APPS["limited"]["modules"]), ("empty", set())):
            status, payload = request_json(
                "/api/v1/capabilities", headers=bearer(self.tokens[name])
            )
            self.assertEqual(status, 200)
            self.assertIsInstance(payload, dict)
            capabilities = payload.get("data", payload)
            self.assertIsInstance(capabilities, list)
            modules = module_names(capabilities)
            if name == "limited":
                self.assertTrue(expected.issubset(modules), (expected, modules))
            else:
                self.assertEqual(modules, expected)

    def test_rest_permission_allow_and_deny(self):
        status, _ = request_json(
            "/api/v1/system/status", headers=bearer(self.tokens["limited"])
        )
        self.assertEqual(status, 200)

        status, payload = request_json(
            "/api/v1/users", headers=bearer(self.tokens["limited"])
        )
        self.assertEqual(status, 403)
        self.assertEqual(payload.get("error"), "权限不足")

        status, _ = request_json(
            "/api/v1/system/status", headers=bearer(self.tokens["empty"])
        )
        self.assertEqual(status, 403)

        status, _ = request_json(
            "/api/v1/system/status", headers=bearer(self.tokens["full"])
        )
        self.assertEqual(status, 200)

    def test_mcp_basic_tools_and_call(self):
        auth = basic_auth(APPS["limited"])
        initialize = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2025-06-18",
                "capabilities": {},
                "clientInfo": {"name": "allbot-local-test", "version": "1.0"},
            },
        }
        status, payload = mcp_request(auth, initialize)
        self.assertEqual(status, 200)
        self.assertIn("result", payload)

        status, payload = mcp_request(
            auth, {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}
        )
        self.assertEqual(status, 200)
        tools = mcp_result(payload).get("tools", [])
        names = {tool.get("name") for tool in tools}
        self.assertIn("list_capabilities", names)
        self.assertIn("api_system_read", names)
        self.assertNotIn("api_users_read", names)

        status, payload = mcp_request(
            auth,
            {
                "jsonrpc": "2.0",
                "id": 3,
                "method": "tools/call",
                "params": {"name": "api_system_read", "arguments": {"method": "GET", "path": "/api/v1/system/status"}},
            },
        )
        self.assertEqual(status, 200)
        result = mcp_result(payload)
        self.assertFalse(result.get("isError", False))

    def test_mcp_full_and_empty_tool_boundaries(self):
        for name in ("full", "empty"):
            status, payload = mcp_request(
                basic_auth(APPS[name]),
                {"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {}},
            )
            self.assertEqual(status, 200)
            names = {tool.get("name") for tool in mcp_result(payload).get("tools", [])}
            self.assertIn("list_capabilities", names)
            if name == "full":
                self.assertIn("api_users_read", names)
            else:
                self.assertEqual(names, {"list_capabilities"})


if __name__ == "__main__":
    unittest.main(verbosity=2)
