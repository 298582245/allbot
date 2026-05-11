"""
支付服务抽象基类
"""
from abc import ABC, abstractmethod
from typing import Dict, Optional
from datetime import datetime


class PaymentProvider(ABC):
    """支付提供商抽象基类"""

    @abstractmethod
    async def create_payment(
        self,
        order_no: str,
        amount: int,
        subject: str,
        description: str,
        return_url: Optional[str] = None,
        notify_url: Optional[str] = None,
    ) -> Dict:
        """
        创建支付订单

        Args:
            order_no: 订单号
            amount: 金额（分）
            subject: 商品标题
            description: 商品描述
            return_url: 支付完成后跳转地址
            notify_url: 异步通知地址

        Returns:
            包含支付信息的字典，至少包含：
            - payment_url: 支付页面URL
            - payment_id: 支付平台订单ID
        """
        pass

    @abstractmethod
    async def verify_payment(self, payment_data: Dict) -> bool:
        """
        验证支付回调签名

        Args:
            payment_data: 支付平台回调数据

        Returns:
            签名是否有效
        """
        pass

    @abstractmethod
    async def query_payment(self, order_no: str) -> Dict:
        """
        查询支付状态

        Args:
            order_no: 订单号

        Returns:
            支付状态信息，至少包含：
            - status: 支付状态 (pending/paid/failed)
            - paid_at: 支付时间（如果已支付）
        """
        pass

    @abstractmethod
    async def refund_payment(
        self, order_no: str, refund_amount: int, reason: str
    ) -> Dict:
        """
        退款

        Args:
            order_no: 订单号
            refund_amount: 退款金额（分）
            reason: 退款原因

        Returns:
            退款结果，至少包含：
            - success: 是否成功
            - refund_id: 退款单号
        """
        pass
