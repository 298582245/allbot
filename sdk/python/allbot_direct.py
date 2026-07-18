"""
AllBot Python Direct SDK

插件只需要定义一个异步 handle(ctx) 函数，并在入口调用 run_direct(handle)。
SDK 会把当前消息封装成 Context，开发者通过 ctx 读取消息、回复消息、等待下一句、声明数据视图。
"""

import asyncio
import importlib.util
import inspect
import json
import os
import sys
import time
from typing import Any, Callable, Dict, List, Optional


_current_context = None
_protocol_stdout = sys.stdout


def write_protocol_action(action: Dict[str, Any]) -> None:
    _protocol_stdout.write(json.dumps(action, ensure_ascii=False) + "\n")
    _protocol_stdout.flush()


class PluginStdoutProxy:
    encoding = getattr(sys.stderr, "encoding", None)
    errors = getattr(sys.stderr, "errors", None)

    def write(self, text: str) -> int:
        return sys.stderr.write(text)

    def flush(self) -> None:
        sys.stderr.flush()

    def isatty(self) -> bool:
        return sys.stderr.isatty()

    def writable(self) -> bool:
        return True


def redirect_stdout_to_stderr() -> None:
    if os.getenv("ALLBOT_PLUGIN_ID"):
        sys.stdout = PluginStdoutProxy()


redirect_stdout_to_stderr()


def normalize_button_rows(buttons: Any) -> List[List[Dict[str, str]]]:
    if not isinstance(buttons, list):
        return []
    rows: List[List[Dict[str, str]]] = []
    for row in buttons:
        if not isinstance(row, list):
            continue
        items: List[Dict[str, str]] = []
        for item in row:
            if isinstance(item, dict):
                text = str(item.get("text") or item.get("Text") or "")
                value = str(item.get("value") or item.get("Value") or "")
                user_id = str(item.get("user_id") or item.get("userId") or item.get("UserID") or "").strip()
            else:
                text = ""
                value = ""
                user_id = ""
            if text and value:
                button = {"text": text, "value": value}
                if user_id:
                    button["user_id"] = user_id
                items.append(button)
        if items:
            rows.append(items)
    return rows


def normalize_rich_parts(parts: Any) -> List[Dict[str, str]]:
    if not isinstance(parts, list):
        parts = [parts]
    result: List[Dict[str, str]] = []
    for part in parts:
        if isinstance(part, str):
            item = {"type": "text", "text": part}
        elif isinstance(part, dict):
            if "image" in part:
                item = {"type": "image", "url": str(part.get("image") or ""), "alt": str(part.get("alt") or "")}
            else:
                part_type = str(part.get("type") or ("markdown" if "markdown" in part else "image" if "url" in part else "text")).lower()
                if part_type == "text":
                    item = {"type": "text", "text": str(part.get("text") or part.get("content") or "")}
                elif part_type == "markdown":
                    item = {"type": "markdown", "markdown": str(part.get("markdown") or part.get("text") or part.get("content") or "")}
                elif part_type == "image":
                    item = {"type": "image", "url": str(part.get("url") or ""), "alt": str(part.get("alt") or "")}
                else:
                    continue
        else:
            continue
        if (item["type"] == "image" and item.get("url")) or (item["type"] == "text" and item.get("text")) or (item["type"] == "markdown" and item.get("markdown")):
            result.append(item)
    return result


class Context:
    """消息上下文，提供插件开发常用 API。"""

    def __init__(self, data: Dict[str, Any]):
        self.plugin_id = data.get("plugin_id", "")
        self.pluginId = self.plugin_id
        self.platform = data.get("platform", "")
        self.adapter_id = data.get("adapter_id", "")
        self.adapterId = self.adapter_id
        self.user_id = data.get("user_id", "")
        self.userId = self.user_id
        self.union_id = data.get("union_id", "")
        self.unionId = self.union_id
        self.points = int(data.get("points", 0) or 0)
        self.points_unit = data.get("points_unit") or "积分"
        self.pointsUnit = self.points_unit
        self.group_id = data.get("group_id", "")
        self.groupId = self.group_id
        self.content = data.get("content", "")
        self.text = self.content
        self.message_id = data.get("message_id", "")
        self.messageId = self.message_id
        self.admin = bool(data.get("is_admin", False))
        self.is_admin_value = self.admin
        self.metadata = data.get("metadata", {}) or {}
        self.event = build_event(self.metadata)
        self.user_config = data.get("user_config", {}) or {}
        self.userConfig = self.user_config
        self.access_control = data.get("access_control", {}) or {}
        self.accessControl = self.access_control
        self._request_seq = 0
        global _current_context
        _current_context = self
        self.db = Database(self)
        self.pay = PAY(self)
        self.web = WebRouter()

    def is_group(self) -> bool:
        """是否群聊消息。"""
        return bool(self.group_id)

    def is_private(self) -> bool:
        """是否私聊消息。"""
        return not self.group_id

    def chat_id(self) -> str:
        """当前会话 ID：群聊优先返回群号，私聊返回用户号。"""
        return self.group_id or self.user_id

    def is_admin(self) -> bool:
        """是否为后台配置的平台管理员。"""
        return self.admin

    def args(self, command: str = "") -> str:
        """去掉指令前缀并返回剩余内容。"""
        if not command:
            return self.content.strip()
        if self.content.startswith(command):
            return self.content[len(command):].strip()
        return ""

    async def reply(self, text: Any) -> bool:
        """回复当前消息。"""
        return self._send({"action": "reply", "text": str(text)})

    async def send_text(self, text: Any) -> bool:
        """reply 的别名，贴近常见机器人 SDK。"""
        return await self.reply(text)

    async def send_markdown(self, markdown: Any) -> bool:
        """回复当前消息，内容按 Markdown 发送。"""
        return self._send({"action": "send_markdown", "markdown": str(markdown)})

    async def sendMarkdown(self, markdown: Any) -> bool:
        """send_markdown 的 camelCase 别名。"""
        return await self.send_markdown(markdown)

    async def send_rich(self, parts: Any, **options: Any) -> bool:
        return self._send({
            "action": "send_rich",
            "parts": normalize_rich_parts(parts),
            "fallback_text": str(options.get("fallback_text") or options.get("fallbackText") or ""),
            "prefer": str(options.get("prefer") or "auto"),
        })

    async def sendRich(self, parts: Any, **options: Any) -> bool:
        return await self.send_rich(parts, **options)

    async def reply_rich(self, parts: Any, **options: Any) -> bool:
        return await self.send_rich(parts, **options)

    async def replyRich(self, parts: Any, **options: Any) -> bool:
        return await self.reply_rich(parts, **options)

    async def send_rich_message(self, **options: Any) -> Dict[str, Any]:
        target_platform = str(options.get("platform") or self.platform)
        return self._request({
            "action": "send_rich_message",
            "platform": target_platform,
            "adapter_id": self._adapter_id_for(options, target_platform),
            "user_id": str(options.get("user_id") or options.get("userId") or self.user_id),
            "group_id": str(options.get("group_id") or options.get("groupId") or ""),
            "union_id": str(options.get("union_id") or options.get("unionId") or ""),
            "parts": normalize_rich_parts(options.get("parts") or options.get("content") or []),
            "fallback_text": str(options.get("fallback_text") or options.get("fallbackText") or ""),
            "prefer": str(options.get("prefer") or "auto"),
        }, "send_rich_message_response")

    async def sendRichMessage(self, **options: Any) -> Dict[str, Any]:
        return await self.send_rich_message(**options)

    async def send_image_message(self, **options: Any) -> Dict[str, Any]:
        """主动向指定用户、群或 UnionID 发送图片。"""
        target_platform = str(options.get("platform") or self.platform)
        return self._request({
            "action": "send_image_message",
            "platform": target_platform,
            "adapter_id": self._adapter_id_for(options, target_platform),
            "user_id": str(options.get("user_id") or options.get("userId") or self.user_id),
            "group_id": str(options.get("group_id") or options.get("groupId") or ""),
            "union_id": str(options.get("union_id") or options.get("unionId") or ""),
            "url": str(options.get("image_url") or options.get("imageUrl") or options.get("url") or ""),
        }, "send_image_message_response")

    async def sendImageMessage(self, **options: Any) -> Dict[str, Any]:
        """send_image_message 的 camelCase 别名。"""
        return await self.send_image_message(**options)

    async def send_message(self, **options: Any) -> Dict[str, Any]:
        """主动发送私聊或群聊消息，用于定时通知。"""
        target_platform = str(options.get("platform") or self.platform)
        payload = {
            "action": "send_message",
            "platform": target_platform,
            "adapter_id": self._adapter_id_for(options, target_platform),
            "user_id": str(options.get("user_id") or options.get("userId") or self.user_id),
            "group_id": str(options.get("group_id") or options.get("groupId") or ""),
            "union_id": str(options.get("union_id") or options.get("unionId") or ""),
            "text": str(options.get("text") or options.get("content") or ""),
        }
        buttons = normalize_button_rows(options.get("buttons") or options.get("buttonRows") or [])
        if buttons:
            payload["buttons"] = buttons
        return self._request(payload, "send_message_response")

    async def send_buttons(self, text: Any, buttons: Any = None) -> bool:
        """发送按钮消息。"""
        return self._send({"action": "send_buttons", "text": str(text), "buttons": normalize_button_rows(buttons or [])})

    async def sendButtons(self, text: Any, buttons: Any = None) -> bool:
        """send_buttons 的 camelCase 别名。"""
        return await self.send_buttons(text, buttons)

    async def sendMessage(self, **options: Any) -> Dict[str, Any]:
        """send_message 的 camelCase 别名。"""
        return await self.send_message(**options)

    async def push(self, userId: str = "", groupId: str = "", content: Any = "", platform: str = "", adapterId: str = "", **options: Any) -> Dict[str, Any]:
        """主动向指定用户、群或 UnionID 发送消息，不回退当前消息用户。"""
        user_id = options["user_id"] if "user_id" in options else options.get("userId", userId)
        group_id = options["group_id"] if "group_id" in options else options.get("groupId", groupId)
        union_id = options["union_id"] if "union_id" in options else options.get("unionId", "")
        user_id, union_id = split_push_user_and_union_id(user_id, union_id)
        text = options["content"] if "content" in options else options.get("text", content)
        target_platform = options["platform"] if "platform" in options else platform
        adapter_id = options["adapter_id"] if "adapter_id" in options else options.get("adapterId", adapterId)
        payload = {
            "action": "send_message",
            "platform": str(target_platform or self.platform),
            "adapter_id": str(adapter_id or ""),
            "user_id": str(user_id or ""),
            "group_id": str(group_id or ""),
            "union_id": str(union_id or ""),
            "text": str(text or ""),
        }
        buttons = normalize_button_rows(options.get("buttons") or options.get("buttonRows") or [])
        if buttons:
            payload["buttons"] = buttons
        return self._request(payload, "send_message_response")

    async def send_image(self, image_url: str) -> bool:
        """发送图片 URL 或本地路径，具体支持取决于平台适配器。"""
        return self._send({"action": "send_image", "url": image_url})

    async def send_file(self, file_path: str) -> bool:
        """发送文件路径，具体支持取决于平台适配器。"""
        return self._send({"action": "send_file", "path": file_path})

    async def listen(self, timeout: int = 60) -> str:
        """等待同一用户/群的下一条消息，超时返回空字符串。"""
        self._send({"action": "listen", "timeout": timeout})
        line = sys.stdin.readline()
        if not line:
            return ""
        try:
            response = json.loads(line)
            if response.get("action") == "listen_response":
                return response.get("content", "")
        except json.JSONDecodeError:
            return ""
        return ""

    async def set_data_view(
        self,
        table_name: str,
        view_name: str = "",
        group_name: str = "插件数据",
        description: str = "",
        columns: Optional[List[str]] = None,
    ) -> bool:
        """设置插件数据表在后台“数据管理”中的展示视图。"""
        return self._send({
            "action": "set_data_view",
            "table_name": table_name,
            "view_name": view_name or table_name,
            "group_name": group_name,
            "description": description,
            "columns": columns or [],
        })

    async def setDataView(self, table_name: str, **options: Any) -> bool:
        """set_data_view 的 camelCase 别名。"""
        return await self.set_data_view(
            table_name,
            options.get("view_name") or options.get("viewName") or table_name,
            options.get("group_name") or options.get("groupName") or "插件数据",
            options.get("description") or "",
            options.get("columns") or [],
        )

    def meta(self, key: str, default: str = "") -> str:
        """获取平台原始扩展字段。"""
        return self.metadata.get(key, default)

    def _adapter_id_for(self, options: Dict[str, Any], target_platform: str = "") -> str:
        if "adapter_id" in options:
            return str(options.get("adapter_id") or "")
        if "adapterId" in options:
            return str(options.get("adapterId") or "")
        if not target_platform or target_platform == self.platform:
            return str(self.adapter_id or "")
        return ""

    def config(self, key: str = "", default: Any = "") -> Any:
        """获取后台为当前插件填写的用户配置。"""
        if not key:
            return self.user_config
        return self.user_config.get(key, default)

    async def get_union_id(self) -> str:
        """获取当前系统统一用户 ID；用户未注册时会返回注册/绑定引导错误。"""
        if self.union_id:
            return self.union_id
        data = self._request({"action": "get_union_id"}, "union_id_response")
        self.union_id = data.get("union_id", "")
        self.unionId = self.union_id
        self.points = int(data.get("points", 0) or 0)
        return self.union_id

    async def getUnionId(self) -> str:
        """get_union_id 的 camelCase 别名。"""
        return await self.get_union_id()

    async def consume_points(self, amount: int, **options: Any) -> int:
        data = self._request({
            "action": "points_consume",
            "union_id": str(options.get("union_id") or options.get("unionId") or self.union_id),
            "amount": int(amount or 0),
        }, "auth_response")
        self.points = int(data.get("points", 0) or 0)
        return self.points

    async def consumePoints(self, amount: int, **options: Any) -> int:
        return await self.consume_points(amount, **options)

    async def add_points(self, amount: int, **options: Any) -> int:
        data = self._request({
            "action": "points_add",
            "union_id": str(options.get("union_id") or options.get("unionId") or self.union_id),
            "amount": int(amount or 0),
        }, "auth_response")
        self.points = int(data.get("points", 0) or 0)
        return self.points

    async def addPoints(self, amount: int, **options: Any) -> int:
        return await self.add_points(amount, **options)

    async def set_access_control(self, config: Dict[str, Any]) -> Dict[str, Any]:
        """更新当前插件的访问控制配置。"""
        data = self._request({"action": "set_access_control", "access_control": normalize_access_control(config or {})}, "access_control_response")
        self.access_control = data or {}
        self.accessControl = self.access_control
        return self.access_control

    async def setAccessControl(self, config: Dict[str, Any]) -> Dict[str, Any]:
        """set_access_control 的 camelCase 别名。"""
        return await self.set_access_control(config)

    async def list_platform_admins(self, **options: Any) -> List[Dict[str, Any]]:
        """获取已启动平台上的管理员身份列表。"""
        data = self._request({
            "action": "list_platform_admins",
            "platform": str(options.get("platform") or ""),
        }, "platform_admins_response")
        return data if isinstance(data, list) else []

    async def listPlatformAdmins(self, **options: Any) -> List[Dict[str, Any]]:
        """list_platform_admins 的 camelCase 别名。"""
        return await self.list_platform_admins(**options)

    async def get_platform_admins(self, **options: Any) -> List[Dict[str, Any]]:
        return await self.list_platform_admins(**options)

    async def getPlatformAdmins(self, **options: Any) -> List[Dict[str, Any]]:
        return await self.list_platform_admins(**options)

    async def set_scheduled_task(self, **options: Any) -> Dict[str, Any]:
        """声明或更新当前插件关联的定时伪造消息任务。"""
        return self._request({
            "action": "set_scheduled_task",
            "task_key": str(options.get("task_key") or options.get("taskKey") or options.get("name") or ""),
            "name": str(options.get("name") or options.get("task_key") or options.get("taskKey") or ""),
            "description": str(options.get("description") or ""),
            "enabled": options.get("enabled", True) is not False,
            "pinned": bool(options.get("pinned", False)),
            "cron": "\n".join([str(item) for item in options.get("cron")]) if isinstance(options.get("cron"), list) else str(options.get("cron") or ""),
            "platform": str(options.get("platform") or self.platform),
            "adapter_id": str(options.get("adapter_id") or options.get("adapterId") or self.adapter_id),
            "user_id": str(options.get("user_id") or options.get("userId") or self.user_id),
            "group_id": str(options.get("group_id") or options.get("groupId") or self.group_id or ""),
            "content": str(options.get("content") or options.get("text") or ""),
            "max_count": int(options.get("max_count") or options.get("maxCount") or 0),
        }, "scheduled_task_response")

    async def setScheduledTask(self, **options: Any) -> Dict[str, Any]:
        """set_scheduled_task 的 camelCase 别名。"""
        return await self.set_scheduled_task(**options)

    async def fake_message(self, platform: str = "", user_id: str = "", group_id: str = "", content: str = "", adapter_id: str = "") -> bool:
        """伪造一条收到的用户消息，让系统按正常消息路由重新匹配插件。"""
        self._request({
            "action": "fake_message",
            "platform": platform or self.platform,
            "adapter_id": str(adapter_id or self.adapter_id),
            "user_id": str(user_id or self.user_id),
            "group_id": str(group_id or ""),
            "content": str(content or ""),
        }, "fake_message_response")
        return True

    async def fakeMessage(self, **options: Any) -> bool:
        """fake_message 的 camelCase 别名。"""
        return await self.fake_message(
            options.get("platform") or self.platform,
            options.get("user_id") or options.get("userId") or self.user_id,
            options.get("group_id") or options.get("groupId") or "",
            options.get("content") or options.get("text") or "",
            options.get("adapter_id") or options.get("adapterId") or self.adapter_id,
        )

    async def run_script(self, **options: Any) -> Dict[str, Any]:
        """运行插件目录内的 Node.js/Python 脚本，支持临时环境变量注入。"""
        return self._request({
            "action": "run_script",
            "runtime": str(options.get("runtime") or "nodejs"),
            "runtime_profile": str(options.get("runtime_profile") or options.get("runtimeProfile") or ""),
            "script": str(options.get("script") or options.get("path") or ""),
            "cwd": str(options.get("cwd") or ""),
            "env": normalize_env(options.get("env") or {}),
            "timeout": int(options.get("timeout") or 300),
            "wait": bool(options.get("wait")),
            "run_mode": str(options.get("run_mode") or options.get("runMode") or ""),
            "union_id": str(options.get("union_id") or options.get("unionId") or self.union_id),
        }, "script_response", allow_failure=True)

    async def runScript(self, **options: Any) -> Dict[str, Any]:
        """run_script 的 camelCase 别名。"""
        return await self.run_script(**options)

    async def run_ql_script(self, **options: Any) -> Dict[str, Any]:
        """青龙脚本友好包装：把账号 env_value 按换行注入到指定环境变量。"""
        env_name = str(options.get("env_name") or options.get("envName") or "").strip()
        if not env_name:
            raise RuntimeError("envName 不能为空")
        accounts = options.get("accounts")
        if not isinstance(accounts, list):
            accounts = []
        env = dict(options.get("env") or {})
        env[env_name] = "\n".join([str(item.get("env_value") or item.get("envValue") or "") for item in accounts if item.get("env_value") or item.get("envValue")])
        return await self.run_script(**{**options, "env": env})

    async def runQLScript(self, **options: Any) -> Dict[str, Any]:
        """run_ql_script 的 camelCase 别名。"""
        return await self.run_ql_script(**options)

    def _request(self, action: Dict[str, Any], expected_action: str = "db_response", allow_failure: bool = False) -> Any:
        """发送需要后端回包的请求。"""
        self._request_seq += 1
        request_id = f"{self._request_seq}"
        action["request_id"] = request_id
        self._send(action)

        line = sys.stdin.readline()
        if not line:
            raise RuntimeError("请求无响应")
        try:
            response = json.loads(line)
        except json.JSONDecodeError as error:
            raise RuntimeError("响应解析失败") from error
        if response.get("action") != expected_action:
            raise RuntimeError("响应类型不匹配")
        if response.get("request_id") != request_id:
            raise RuntimeError("响应 ID 不匹配")
        if not response.get("success") and not allow_failure:
            raise RuntimeError(response.get("error") or "请求失败")
        data = response.get("data")
        if allow_failure and not response.get("success"):
            if not isinstance(data, dict):
                data = {}
            data = dict(data)
            data.setdefault("status", "failed")
            data.setdefault("error", response.get("error") or "请求失败")
        return data

    def _send(self, action: Dict[str, Any]) -> bool:
        write_protocol_action(action)
        return True


class PAY:
    """支付等待封装，用于发起一次消费并等待支付结果。"""

    def __init__(self, ctx: Optional[Context] = None):
        self.ctx = ctx or _current_context

    async def wait_pay(self, subject: Any, amount_rmb: Any, timeout: int = 300, **options: Any) -> Dict[str, Any]:
        ctx = self.ctx or _current_context
        if ctx is None:
            raise RuntimeError("PAY 需要可用的 Context")
        timeout_value = int(options.pop("timeout_seconds", timeout) or 300)
        methods = options.get("methods")
        if not isinstance(methods, list):
            methods = []
        metadata = options.get("metadata")
        if not isinstance(metadata, dict):
            metadata = {}
        return ctx._request({
            "action": "payment_wait",
            "subject": str(subject or ""),
            "amount": str(amount_rmb),
            "timeout": timeout_value,
            "methods": [str(item) for item in methods if str(item)],
            "metadata": metadata,
            "remark": str(options.get("remark") or ""),
        }, "payment_response")

    async def waitPay(self, subject: Any, amount_rmb: Any, timeout: int = 300, **options: Any) -> Dict[str, Any]:
        """wait_pay 的 camelCase 别名。"""
        return await self.wait_pay(subject, amount_rmb, timeout, **options)


class Database:
    """当前插件的私有数据库封装，实际表名会自动加 plugin_<插件ID>_ 前缀。"""

    def __init__(self, ctx: Context):
        self.ctx = ctx

    async def create_table(self, table: str, columns: Optional[List[Any]] = None) -> str:
        """创建当前插件私有数据表。"""
        return self.ctx._request({"action": "db_create_table", "table": table, "db_columns": normalize_columns(columns or [])})

    async def set_view(self, table: str, **options: Any) -> bool:
        """设置当前插件私有表在后台“数据管理”里的中文视图。"""
        real_table = f"plugin_{self.ctx.plugin_id}_{table}"
        return await self.ctx.set_data_view(
            real_table,
            options.get("view_name") or options.get("viewName") or table,
            options.get("group_name") or options.get("groupName") or "插件数据",
            options.get("description") or "",
            options.get("columns") or [],
        )

    async def query(self, table: str, **options: Any) -> Dict[str, Any]:
        """查询当前插件私有表数据，推荐使用 filters 与 order_by/order_dir 传入结构化条件。"""
        return self.ctx._request({
            "action": "db_query",
            "table": table,
            "query": {
                "table": table,
                "where": options.get("where", ""),
                "args": options.get("args", []),
                "filters": normalize_query_filters(options.get("filters", options.get("filter"))),
                "order": normalize_query_order(options),
                "order_by": options.get("order_by") or options.get("orderBy") or "",
                "order_dir": options.get("order_dir") or options.get("orderDir") or "",
                "limit": options.get("limit", 0),
                "page": options.get("page", 1),
                "size": options.get("size", options.get("page_size", 20)),
            },
        })

    async def first(self, table: str, **options: Any) -> Optional[Dict[str, Any]]:
        """查询第一行数据，没有数据时返回 None。"""
        result = await self.query(table, **{**options, "limit": 1, "size": 1})
        rows = result.get("rows") or []
        return rows[0] if rows else None

    async def insert(self, table: str, values: Dict[str, Any]) -> int:
        """插入一行数据，返回新行 ID。"""
        return int(self.ctx._request({"action": "db_insert", "table": table, "values": values or {}}) or 0)

    async def update(self, table: str, row_id: int, values: Dict[str, Any]) -> bool:
        """按行 ID 更新数据。"""
        self.ctx._request({"action": "db_update", "table": table, "row_id": int(row_id), "values": values or {}})
        return True

    async def delete(self, table: str, row_id: int) -> bool:
        """按行 ID 删除数据。"""
        self.ctx._request({"action": "db_delete", "table": table, "row_id": int(row_id)})
        return True

    async def clear(self, table: str) -> bool:
        """清空当前插件私有表数据。"""
        self.ctx._request({"action": "db_clear", "table": table})
        return True

    async def createTable(self, table: str, columns: Optional[List[Any]] = None) -> str:
        """create_table 的 camelCase 别名。"""
        return await self.create_table(table, columns)

    async def setView(self, table: str, **options: Any) -> bool:
        """set_view 的 camelCase 别名。"""
        return await self.set_view(table, **options)


def build_event(metadata: Dict[str, str]) -> Optional[Dict[str, Any]]:
    is_event = metadata.get("message_type") == "event" or bool(metadata.get("event_name"))
    if not is_event:
        return None
    name = metadata.get("event_name") or metadata.get("qq_office_event_type") or ""
    if not name:
        return None
    return {
        "name": name,
        "eventName": name,
        "type": metadata.get("qq_office_event_type") or name,
        "timestamp": metadata.get("qq_office_timestamp") or "",
        "groupOpenid": metadata.get("qq_office_group_openid") or "",
        "group_openid": metadata.get("qq_office_group_openid") or "",
        "memberOpenid": metadata.get("qq_office_member_openid") or "",
        "member_openid": metadata.get("qq_office_member_openid") or "",
        "opMemberOpenid": metadata.get("qq_office_op_member_openid") or "",
        "op_member_openid": metadata.get("qq_office_op_member_openid") or "",
        "metadata": metadata,
    }


def normalize_env(env: Dict[str, Any]) -> Dict[str, str]:
    return {str(key): str(value) for key, value in env.items() if str(key)}


def normalize_columns(columns: Any) -> List[Dict[str, str]]:
    result = []
    source = columns if isinstance(columns, list) else []
    for item in source:
        if not isinstance(item, dict):
            continue
        name = str(item.get("name") or item.get("field") or item.get("column") or "").strip()
        if not name:
            continue
        result.append({"name": name, "type": str(item.get("type") or "TEXT").strip() or "TEXT"})
    return result


def split_push_user_and_union_id(user_id: Any, union_id: Any) -> tuple[str, str]:
    user_text = str(user_id or "").strip()
    union_text = str(union_id or "").strip()
    if not union_text and user_text.startswith("union:"):
        return "", user_text[len("union:"):].strip()
    if not union_text and user_text.startswith("union_id:"):
        return "", user_text[len("union_id:"):].strip()
    if not union_text and user_text.startswith("U_"):
        return "", user_text
    return user_text, union_text


def normalize_query_filters(filters: Any) -> List[Dict[str, Any]]:
    if not filters:
        return []
    source = filters if isinstance(filters, list) else [filters]
    result: List[Dict[str, Any]] = []
    for item in source:
        if not isinstance(item, dict):
            continue
        result.append({
            "field": str(item.get("field") or item.get("column") or ""),
            "op": str(item.get("op") or item.get("operator") or "="),
            "value": item.get("value"),
            "values": item.get("values") if isinstance(item.get("values"), list) else [],
        })
    return result


def normalize_query_order(options: Dict[str, Any]) -> Any:
    order = options.get("order")
    if isinstance(order, dict):
        return {
            "field": str(order.get("field") or order.get("column") or order.get("order_by") or order.get("orderBy") or ""),
            "direction": str(order.get("direction") or order.get("dir") or order.get("order_dir") or order.get("orderDir") or ""),
        }
    if options.get("order_by") or options.get("orderBy"):
        return {
            "field": str(options.get("order_by") or options.get("orderBy") or ""),
            "direction": str(options.get("order_dir") or options.get("orderDir") or ""),
        }
    return str(order or "")


def normalize_days(value: Any) -> List[int]:
    if isinstance(value, list):
        source = value
    else:
        source = str(value or "").split(",")
    result = []
    for item in source:
        try:
            result.append(int(str(item).strip()))
        except ValueError:
            pass
    return result


def parse_time_ms(value: Any) -> int:
    text = str(value or "").strip()
    if not text:
        return 0
    try:
        normalized = text.replace("Z", "+00:00")
        return int(__import__("datetime").datetime.fromisoformat(normalized).timestamp() * 1000)
    except Exception:
        return 0


def default_expiration_message(account: Dict[str, Any], days_left: int, expires_at: Any, title: str) -> str:
    name = account.get("account_name") or account.get("accountName") or account.get("remark") or account.get("env_name") or "账号"
    if days_left > 0:
        return f"【{title}提醒】{name} 将在 {days_left} 天后过期，请及时续费。"
    if days_left == 0:
        return f"【{title}提醒】{name} 今天到期，请及时续费。"
    return f"【{title}提醒】{name} 已过期，请续费后继续使用。"


def default_unauthorized_message(account: Dict[str, Any], title: str) -> str:
    name = account.get("account_name") or account.get("accountName") or account.get("remark") or account.get("env_name") or "账号"
    return f"【{title}提醒】{name} 尚未授权，请先完成授权后再使用。"


def default_ck_invalid_message(account: Dict[str, Any], title: str, state: Dict[str, Any]) -> str:
    name = account.get("account_name") or account.get("accountName") or account.get("remark") or account.get("env_name") or "账号"
    reason = state.get("reason") or state.get("message") or "CK 已失效"
    return f"【{title}提醒】{name} {reason}，请重新登录或更新 CK。"


def default_ck_check_error_message(account: Dict[str, Any], title: str, error: Exception) -> str:
    name = account.get("account_name") or account.get("accountName") or account.get("remark") or account.get("env_name") or "账号"
    return f"【{title}检测异常】{name} 检测失败：{error}"


def normalize_access_control(config: Dict[str, Any]) -> Dict[str, Any]:
    def list_value(value: Any) -> List[str]:
        if not isinstance(value, list):
            return []
        return [str(item) for item in value if str(item)]

    return {
        "inherit_system": bool(config.get("inherit_system", config.get("inheritSystem", False))),
        "whitelist_groups": list_value(config.get("whitelist_groups") or config.get("whitelistGroups")),
        "blocked_groups": list_value(config.get("blocked_groups") or config.get("blockedGroups")),
        "whitelist_user_ids": list_value(config.get("whitelist_user_ids") or config.get("whitelistUserIds")),
        "blocked_user_ids": list_value(config.get("blocked_user_ids") or config.get("blockedUserIds")),
    }


class WebRouter:
    def __init__(self) -> None:
        self.routes: List[Dict[str, Any]] = []

    def get(self, path: str):
        return self.add("GET", path)

    def post(self, path: str):
        return self.add("POST", path)

    def put(self, path: str):
        return self.add("PUT", path)

    def delete(self, path: str):
        return self.add("DELETE", path)

    def add(self, method: str, path: str):
        def decorator(handler: Callable[..., Any]):
            self.routes.append({"method": str(method or "").upper(), "path": normalize_web_path(path), "handler": handler})
            return handler
        return decorator

    def match(self, method: str, path: str) -> Optional[Dict[str, Any]]:
        normalized_method = str(method or "").upper()
        normalized_path = normalize_web_path(path)
        for route in self.routes:
            if route["method"] == normalized_method and route["path"] == normalized_path:
                return route
        return None


class WebRequest:
    def __init__(self, data: Dict[str, Any]) -> None:
        request = data.get("request") or {}
        self.method = str(data.get("method") or "").upper()
        self.path = normalize_web_path(data.get("path") or request.get("path") or "/")
        self.query = flatten_single_value(data.get("query") or request.get("query") or {})
        self.headers = flatten_single_value(data.get("headers") or request.get("headers") or {})
        self.body = data.get("body", request.get("body", ""))
        self.data = data.get("json") or data.get("form") or request.get("json") or request.get("form")

    async def json(self) -> Any:
        if self.data is not None:
            return self.data
        text = self.body if isinstance(self.body, str) else ""
        if not text.strip():
            return {}
        return json.loads(text)

    def json_response(self, data: Any, status: int = 200, headers: Optional[Dict[str, Any]] = None) -> "WebResponse":
        return WebResponse(data, status, headers or {})

    def jsonResponse(self, data: Any, status: int = 200, headers: Optional[Dict[str, Any]] = None) -> "WebResponse":
        return self.json_response(data, status, headers)


class WebResponse:
    def __init__(self, data: Any, status: int = 200, headers: Optional[Dict[str, Any]] = None) -> None:
        self.status_code = int(status or 200)
        self.headers = {"Content-Type": "application/json; charset=utf-8"}
        self.headers.update({str(key): str(value) for key, value in (headers or {}).items()})
        self.body = data

    def to_action(self) -> Dict[str, Any]:
        action = {"action": "web_response", "status": self.status_code, "headers": self.headers}
        if isinstance(self.body, str):
            action["body"] = self.body
        else:
            action["json"] = self.body
        return action


def normalize_web_path(path: Any) -> str:
    text = str(path or "/").strip()
    normalized = text if text.startswith("/") else f"/{text}"
    while "//" in normalized:
        normalized = normalized.replace("//", "/")
    normalized = normalized.rstrip("/") or "/"
    return normalized


def to_web_response_action(result: Any) -> Dict[str, Any]:
    if isinstance(result, WebResponse):
        return result.to_action()
    if isinstance(result, HTTPResponse):
        action = result.to_action()
        action["action"] = "web_response"
        return action
    if isinstance(result, dict) and result.get("action") == "web_response":
        return result
    return WebResponse(result if result is not None else {}).to_action()


class HTTPResponse:
    def __init__(self) -> None:
        self.status_code = 200
        self.headers = {"Content-Type": "application/json; charset=utf-8"}
        self.body = ""
        self.json_data = None
        self.has_json = False

    def status(self, code: int) -> "HTTPResponse":
        self.status_code = int(code or 200)
        return self

    def set_header(self, key: str, value: Any) -> "HTTPResponse":
        if key:
            self.headers[str(key)] = str(value)
        return self

    def json(self, data: Any, status_code: int = 0) -> "HTTPResponse":
        if status_code:
            self.status(status_code)
        self.json_data = data
        self.has_json = True
        self.set_header("Content-Type", "application/json; charset=utf-8")
        return self

    def send_json(self, data: Any, status_code: int = 0) -> "HTTPResponse":
        return self.json(data, status_code)

    def sendJson(self, data: Any, status_code: int = 0) -> "HTTPResponse":
        return self.json(data, status_code)

    def send(self, body: Any, status_code: int = 0) -> "HTTPResponse":
        if status_code:
            self.status(status_code)
        self.body = body if isinstance(body, str) else json.dumps(body, ensure_ascii=False)
        return self

    def to_action(self) -> Dict[str, Any]:
        action = {"action": "http_response", "status": self.status_code, "headers": self.headers, "body": self.body}
        if self.has_json:
            action["json"] = self.json_data
        return action


async def run_web_api_action(handler: Callable[..., Any], data: Dict[str, Any]) -> Dict[str, Any]:
    ctx = Context(data)
    result = handler(ctx)
    if inspect.isawaitable(result):
        await result
    req = WebRequest(data)
    route = ctx.web.match(req.method, req.path)
    if route is None:
        return WebResponse({"error": "插件 Web API 路由不存在"}, 404).to_action()
    route_handler = route["handler"]
    if len(inspect.signature(route_handler).parameters) >= 2:
        response = route_handler(req, ctx)
    else:
        response = route_handler(req)
    if inspect.isawaitable(response):
        response = await response
    return to_web_response_action(response)


async def run_openapi_action(handler: Callable[..., Any], data: Dict[str, Any]) -> HTTPResponse:
    ctx = Context(data)
    req = dict(data.get("request", {}) or {})
    req["query"] = flatten_single_value(req.get("query") or {})
    req["headers"] = flatten_single_value(req.get("headers") or {})
    req["body"] = req.get("json") or req.get("form") or req.get("body") or {}
    res = HTTPResponse()
    result = handler(ctx, req, res)
    if inspect.isawaitable(result):
        await result
    return res


def flatten_single_value(value: Any) -> Any:
    if not isinstance(value, dict):
        return value
    result: Dict[str, Any] = {}
    for key, item in value.items():
        if isinstance(item, list) and len(item) == 1:
            result[key] = item[0]
        else:
            result[key] = item
    return result


def run_openapi(handler: Callable[[Context, Dict[str, Any], HTTPResponse], Any]) -> None:
    try:
        input_line = sys.stdin.readline()
        if not input_line:
            sys.exit(1)
        data = json.loads(input_line)
        res = asyncio.run(run_openapi_action(handler, data))
        write_protocol_action(res.to_action())
    except Exception as error:
        write_protocol_action({"action": "http_response", "status": 500, "headers": {"Content-Type": "application/json; charset=utf-8"}, "json": {"error": str(error)}})
        sys.exit(1)


def runOpenAPI(handler: Callable[[Context, Dict[str, Any], HTTPResponse], Any]) -> None:
    run_openapi(handler)


def run_auto_openapi(entry_path: str) -> None:
    full_path = os.path.abspath(entry_path)
    spec = importlib.util.spec_from_file_location("allbot_openapi_plugin", full_path)
    if spec is None or spec.loader is None:
        raise RuntimeError("无法加载 Open API 插件入口")
    module = importlib.util.module_from_spec(spec)
    sys.modules["allbot_openapi_plugin"] = module
    spec.loader.exec_module(module)
    handler = getattr(module, "action", None) or getattr(module, "handle", None)
    if not callable(handler):
        raise RuntimeError("Open API 插件必须定义 action(ctx, req, res) 函数")
    run_openapi(handler)


if __name__ == "__main__" and len(sys.argv) >= 3 and sys.argv[1] == "openapi":
    try:
        run_auto_openapi(sys.argv[2])
    except Exception as error:
        write_protocol_action({"action": "http_response", "status": 500, "headers": {"Content-Type": "application/json; charset=utf-8"}, "json": {"error": str(error)}})
        sys.exit(1)


def run_direct(handler: Callable[[Context], Any]) -> None:
    """启动 Direct 插件。"""
    try:
        input_line = sys.stdin.readline()
        if not input_line:
            sys.exit(1)
        data = json.loads(input_line)
        if data.get("event_type") == "web_api":
            write_protocol_action(asyncio.run(run_web_api_action(handler, data)))
        else:
            ctx = Context(data)
            result = handler(ctx)
            if inspect.isawaitable(result):
                asyncio.run(result)
            write_protocol_action({"action": "done", "success": True})
    except Exception as error:
        write_protocol_action({"action": "done", "success": False, "error": str(error)})
        sys.exit(1)
