"""
AllBot Python SDK

提供极简的插件开发接口
"""

__version__ = "0.1.0"

from .context import Context
from .plugin import start_plugin

__all__ = ["Context", "start_plugin"]
