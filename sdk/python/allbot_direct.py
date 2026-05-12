"""
AllBot Python SDK - Direct Mode (stdin/stdout)

支持流式通信协议：
- 插件通过 stdout 发送 JSON 行指令
- reply: 立即发送消息
- listen: 等待用户输入，从 stdin 读取响应
- done: 执行结束
"""

import sys
import json
from typing import List


class Context:
    """消息上下文"""

    def __init__(self, data: dict):
        self.plugin_id = data.get('plugin_id', '')
        self.platform = data.get('platform', '')
        self.user_id = data.get('user_id', '')
        self.group_id = data.get('group_id', '')
        self.content = data.get('content', '')
        self.message_id = data.get('message_id', '')
        self.metadata = data.get('metadata', {})

    async def reply(self, text: str) -> bool:
        """立即发送回复消息"""
        action = {"action": "reply", "text": text}
        sys.stdout.write(json.dumps(action, ensure_ascii=False) + "\n")
        sys.stdout.flush()
        return True

    async def send_image(self, image_url: str) -> bool:
        """发送图片（暂未实现）"""
        return False

    async def listen(self, timeout: int = 60) -> str:
        """等待用户输入

        发送 listen 指令后阻塞，直到收到用户回复或超时
        """
        action = {"action": "listen", "timeout": timeout}
        sys.stdout.write(json.dumps(action, ensure_ascii=False) + "\n")
        sys.stdout.flush()

        # 从 stdin 读取用户回复
        line = sys.stdin.readline()
        if not line:
            return ""

        try:
            response = json.loads(line)
            if response.get("action") == "listen_response":
                return response.get("content", "")
        except json.JSONDecodeError:
            pass

        return ""


def run_direct(handler):
    """直接执行模式入口

    Args:
        handler: 异步消息处理函数，接收Context参数
    """
    try:
        # 从stdin读取第一行消息JSON
        input_line = sys.stdin.readline()
        if not input_line:
            sys.exit(1)

        message_data = json.loads(input_line)

        # 创建上下文
        ctx = Context(message_data)

        # 执行处理器
        import asyncio
        asyncio.run(handler(ctx))

        # 发送完成信号
        done = {"action": "done", "success": True}
        sys.stdout.write(json.dumps(done, ensure_ascii=False) + "\n")
        sys.stdout.flush()

    except Exception as e:
        # 输出错误
        error = {"action": "done", "success": False, "error": str(e)}
        sys.stdout.write(json.dumps(error, ensure_ascii=False) + "\n")
        sys.stdout.flush()
        sys.exit(1)
