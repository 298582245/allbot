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
        self.user_config = data.get("user_config", {}) or {}
        self.userConfig = self.user_config
        self.access_control = data.get("access_control", {}) or {}
        self.accessControl = self.access_control
        self._request_seq = 0
        self.db = Database(self)

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

    async def send_message(self, **options: Any) -> Dict[str, Any]:
        """主动发送私聊或群聊消息，用于定时通知。"""
        return self._request({
            "action": "send_message",
            "platform": str(options.get("platform") or self.platform),
            "adapter_id": str(options.get("adapter_id") or options.get("adapterId") or self.adapter_id),
            "user_id": str(options.get("user_id") or options.get("userId") or self.user_id),
            "group_id": str(options.get("group_id") or options.get("groupId") or ""),
            "union_id": str(options.get("union_id") or options.get("unionId") or ""),
            "text": str(options.get("text") or options.get("content") or ""),
        }, "send_message_response")

    async def sendMessage(self, **options: Any) -> Dict[str, Any]:
        """send_message 的 camelCase 别名。"""
        return await self.send_message(**options)

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
            "script": str(options.get("script") or options.get("path") or ""),
            "cwd": str(options.get("cwd") or ""),
            "env": normalize_env(options.get("env") or {}),
            "timeout": int(options.get("timeout") or 300),
            "wait": bool(options.get("wait")),
            "run_mode": str(options.get("run_mode") or options.get("runMode") or ""),
            "union_id": str(options.get("union_id") or options.get("unionId") or self.union_id),
        }, "script_response")

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

    def _request(self, action: Dict[str, Any], expected_action: str = "db_response") -> Any:
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
        if not response.get("success"):
            raise RuntimeError(response.get("error") or "请求失败")
        return response.get("data")

    def _send(self, action: Dict[str, Any]) -> bool:
        sys.stdout.write(json.dumps(action, ensure_ascii=False) + "\n")
        sys.stdout.flush()
        return True


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


def normalize_env(env: Dict[str, Any]) -> Dict[str, str]:
    return {str(key): str(value) for key, value in env.items() if str(key)}


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
        sys.stdout.write(json.dumps(res.to_action(), ensure_ascii=False) + "\n")
        sys.stdout.flush()
    except Exception as error:
        sys.stdout.write(json.dumps({"action": "http_response", "status": 500, "headers": {"Content-Type": "application/json; charset=utf-8"}, "json": {"error": str(error)}}, ensure_ascii=False) + "\n")
        sys.stdout.flush()
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
        sys.stdout.write(json.dumps({"action": "http_response", "status": 500, "headers": {"Content-Type": "application/json; charset=utf-8"}, "json": {"error": str(error)}}, ensure_ascii=False) + "\n")
        sys.stdout.flush()
        sys.exit(1)


def run_direct(handler: Callable[[Context], Any]) -> None:
    """启动 Direct 插件。"""
    try:
        input_line = sys.stdin.readline()
        if not input_line:
            sys.exit(1)
        ctx = Context(json.loads(input_line))
        asyncio.run(handler(ctx))
        sys.stdout.write(json.dumps({"action": "done", "success": True}, ensure_ascii=False) + "\n")
        sys.stdout.flush()
    except Exception as error:
        sys.stdout.write(json.dumps({"action": "done", "success": False, "error": str(error)}, ensure_ascii=False) + "\n")
        sys.stdout.flush()
        sys.exit(1)
