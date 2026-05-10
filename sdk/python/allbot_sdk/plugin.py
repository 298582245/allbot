"""
插件启动和 HTTP 服务
"""
import os
import sys
import argparse
import importlib.util
from .server import PluginServer


def start_plugin():
    """启动插件 HTTP 服务"""
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=50051, help="HTTP 端口")
    args = parser.parse_args()

    plugin_id = os.environ.get("ALLBOT_PLUGIN_ID", "unknown")
    port = args.port

    print(f"Starting plugin: {plugin_id} on port {port}")

    # 动态导入插件的 main.py
    main_path = os.path.join(os.getcwd(), "main.py")
    if not os.path.exists(main_path):
        print(f"Error: main.py not found at {main_path}")
        sys.exit(1)

    spec = importlib.util.spec_from_file_location("plugin_main", main_path)
    plugin_module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(plugin_module)

    # 获取 handle 函数
    if not hasattr(plugin_module, "handle"):
        print("Error: handle function not found in main.py")
        sys.exit(1)

    handle_func = plugin_module.handle

    # 启动 HTTP 服务器
    server = PluginServer(port, handle_func)
    server.start()

    print(f"Plugin {plugin_id} started successfully on port {port}")

    try:
        # 保持运行
        import time
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print(f"Plugin {plugin_id} shutting down...")
        server.stop()


if __name__ == "__main__":
    start_plugin()
