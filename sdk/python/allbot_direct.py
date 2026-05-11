"""
AllBot Python SDK - Direct Mode (stdin/stdout)

无端口直接执行模式，支持并发
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
        self._replies: List[str] = []

    async def reply(self, text: str) -> bool:
        """回复消息"""
        self._replies.append(text)
        return True

    async def send_image(self, image_url: str) -> bool:
        """发送图片（暂未实现）"""
        return False

    async def listen(self, timeout: int = 60) -> str:
        """等待用户输入（暂未实现）"""
        return ""


def run_direct(handler):
    """直接执行模式入口

    Args:
        handler: 异步消息处理函数，接收Context参数
    """
    try:
        # 从stdin读取消息JSON
        input_data = sys.stdin.read()
        message_data = json.loads(input_data)

        # 创建上下文
        ctx = Context(message_data)

        # 执行处理器
        import asyncio
        asyncio.run(handler(ctx))

        # 输出结果到stdout
        result = {
            'success': True,
            'error': '',
            'replies': ctx._replies
        }
        print(json.dumps(result), flush=True)

    except Exception as e:
        # 输出错误
        result = {
            'success': False,
            'error': str(e),
            'replies': []
        }
        print(json.dumps(result), flush=True)
        sys.exit(1)
