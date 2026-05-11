"""
Context 类 - 提供插件开发的统一 API
"""
from typing import Optional, Dict, Any


class Context:
    """消息上下文，提供统一的中间件 API"""

    def __init__(
        self,
        platform: str,
        user_id: str,
        group_id: str,
        content: str,
        message_id: str,
        plugin_id: str,
        grpc_channel: Optional[Any] = None,
    ):
        self.platform = platform  # 'qq' | 'wechat' | 'telegram'
        self.user_id = user_id
        self.group_id = group_id  # 私聊为空字符串
        self.content = content
        self.message_id = message_id
        self.plugin_id = plugin_id
        self._channel = grpc_channel
        self._stub = None  # gRPC stub，延迟初始化

    async def reply(self, text: str) -> bool:
        """回复消息

        Args:
            text: 消息内容

        Returns:
            是否成功
        """
        # TODO: 调用 gRPC ReplyRequest
        print(f"[Reply] {text}")
        return True

    async def send_image(self, image_url: str) -> bool:
        """发送图片

        Args:
            image_url: 图片 URL

        Returns:
            是否成功
        """
        # TODO: 调用 gRPC SendImageRequest
        print(f"[SendImage] {image_url}")
        return True

    async def send_file(self, file_path: str) -> bool:
        """发送文件

        Args:
            file_path: 文件路径

        Returns:
            是否成功
        """
        # TODO: 调用 gRPC SendFileRequest
        print(f"[SendFile] {file_path}")
        return True

    async def listen(self, timeout: int = 60) -> str:
        """等待用户的下一条消息（连续对话）

        Args:
            timeout: 超时时间（秒），默认 60 秒

        Returns:
            用户发送的消息内容，超时返回空字符串
        """
        # TODO: 调用 gRPC ListenRequest
        print(f"[Listen] Waiting for {timeout}s...")
        return ""

    async def get_user_info(self) -> Optional[Dict[str, Any]]:
        """获取用户信息

        Returns:
            用户信息字典，包含 user_id, nickname, avatar 等
        """
        # TODO: 调用 gRPC GetUserInfoRequest
        return {
            "user_id": self.user_id,
            "nickname": "User",
            "avatar": "",
        }

    async def get_group_info(self) -> Optional[Dict[str, Any]]:
        """获取群组信息

        Returns:
            群组信息字典，包含 group_id, name, member_count 等
        """
        if not self.group_id:
            return None

        # TODO: 调用 gRPC GetGroupInfoRequest
        return {
            "group_id": self.group_id,
            "name": "Group",
            "member_count": 0,
        }

    async def at_user(self, user_id: str) -> bool:
        """@某人（QQ/微信支持）

        Args:
            user_id: 用户 ID

        Returns:
            是否成功
        """
        if self.platform not in ["qq", "wechat"]:
            return False

        # TODO: 调用 gRPC AtUserRequest
        print(f"[AtUser] @{user_id}")
        return True

    # 数据存储
    class Storage:
        """插件数据存储（自动隔离）"""

        def __init__(self, plugin_id: str, channel: Optional[Any] = None):
            self.plugin_id = plugin_id
            self._channel = channel

        async def get(self, key: str) -> Optional[str]:
            """获取数据

            Args:
                key: 键

            Returns:
                值，不存在返回 None
            """
            # TODO: 调用 gRPC StorageGetRequest
            return None

        async def set(self, key: str, value: str) -> bool:
            """设置数据

            Args:
                key: 键
                value: 值

            Returns:
                是否成功
            """
            # TODO: 调用 gRPC StorageSetRequest
            return True

    @property
    def storage(self) -> Storage:
        """获取存储对象"""
        if not hasattr(self, "_storage"):
            self._storage = self.Storage(self.plugin_id, self._channel)
        return self._storage

    # HTTP 请求
    class Http:
        """HTTP 请求工具"""

        def __init__(self, channel: Optional[Any] = None):
            self._channel = channel

        async def get(self, url: str, headers: Optional[Dict[str, str]] = None) -> Dict[str, Any]:
            """HTTP GET 请求

            Args:
                url: 请求 URL
                headers: 请求头

            Returns:
                响应字典，包含 status_code, body, error
            """
            # TODO: 调用 gRPC HttpGetRequest
            return {"status_code": 200, "body": "", "error": ""}

        async def post(
            self, url: str, data: str, headers: Optional[Dict[str, str]] = None
        ) -> Dict[str, Any]:
            """HTTP POST 请求

            Args:
                url: 请求 URL
                data: 请求体
                headers: 请求头

            Returns:
                响应字典，包含 status_code, body, error
            """
            # TODO: 调用 gRPC HttpPostRequest
            return {"status_code": 200, "body": "", "error": ""}

    @property
    def http(self) -> Http:
        """获取 HTTP 对象"""
        if not hasattr(self, "_http"):
            self._http = self.Http(self._channel)
        return self._http
