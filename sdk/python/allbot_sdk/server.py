"""
插件 HTTP 服务器（简化的 gRPC 实现）
"""
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
import threading


class PluginServer:
    """插件 HTTP 服务器"""

    def __init__(self, port: int, handle_func):
        self.port = port
        self.handle_func = handle_func
        self.server = None
        self.session_manager = None  # 由核心框架设置

    def start(self):
        """启动服务器"""
        handler = self._create_handler()
        self.server = HTTPServer(('localhost', self.port), handler)

        # 在后台线程运行
        thread = threading.Thread(target=self.server.serve_forever)
        thread.daemon = True
        thread.start()

        print(f"Plugin server started on port {self.port}")

    def stop(self):
        """停止服务器"""
        if self.server:
            self.server.shutdown()

    def _create_handler(self):
        """创建请求处理器"""
        server = self

        class PluginRequestHandler(BaseHTTPRequestHandler):
            def do_POST(self):
                if self.path == '/handle':
                    self._handle_message()
                elif self.path == '/listen':
                    self._handle_listen()
                elif self.path == '/reply':
                    self._handle_reply()
                else:
                    self.send_error(404)

            def _handle_message(self):
                """处理消息"""
                try:
                    content_length = int(self.headers['Content-Length'])
                    body = self.rfile.read(content_length)
                    req = json.loads(body)

                    # 创建 Context 对象
                    from allbot_sdk.context import Context
                    ctx = Context(
                        platform=req['platform'],
                        user_id=req['user_id'],
                        group_id=req['group_id'],
                        content=req['content'],
                        message_id=req['message_id'],
                        plugin_id=req['plugin_id'],
                        grpc_channel=None,  # 简化实现
                    )

                    # 调用插件的 handle 函数
                    import asyncio
                    asyncio.run(server.handle_func(ctx))

                    # 返回成功响应
                    self._send_json({'success': True, 'error': ''})

                except Exception as e:
                    self._send_json({'success': False, 'error': str(e)})

            def _handle_listen(self):
                """处理 listen 请求"""
                try:
                    content_length = int(self.headers['Content-Length'])
                    body = self.rfile.read(content_length)
                    req = json.loads(body)

                    # TODO: 实现 listen 逻辑
                    # 这需要与核心框架的 session manager 通信

                    self._send_json({'content': ''})

                except Exception as e:
                    self._send_json({'content': ''})

            def _handle_reply(self):
                """处理 reply 请求"""
                try:
                    content_length = int(self.headers['Content-Length'])
                    body = self.rfile.read(content_length)
                    req = json.loads(body)

                    # TODO: 实现 reply 逻辑
                    # 这需要与核心框架的 adapter 通信

                    self._send_json({'success': True, 'error': '', 'message_id': ''})

                except Exception as e:
                    self._send_json({'success': False, 'error': str(e), 'message_id': ''})

            def _send_json(self, data):
                """发送 JSON 响应"""
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps(data).encode())

            def log_message(self, format, *args):
                """禁用默认日志"""
                pass

        return PluginRequestHandler
