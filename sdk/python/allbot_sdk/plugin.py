"""
插件启动和 gRPC 服务
"""
import os
import sys
import asyncio
import argparse
import importlib.util
from concurrent import futures
import grpc


def start_plugin():
    """启动插件 gRPC 服务"""
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=50051, help="gRPC 端口")
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

    # 启动 gRPC 服务器
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    # TODO: 添加 gRPC 服务实现
    server.add_insecure_port(f"[::]:{port}")
    server.start()

    print(f"Plugin {plugin_id} started successfully on port {port}")

    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        print(f"Plugin {plugin_id} shutting down...")
        server.stop(0)


if __name__ == "__main__":
    start_plugin()
