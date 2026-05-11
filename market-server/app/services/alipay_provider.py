"""
支付宝支付提供商
"""
from typing import Dict, Optional
from datetime import datetime
import hashlib
import urllib.parse
from .payment_base import PaymentProvider


class AlipayProvider(PaymentProvider):
    """支付宝支付提供商"""

    def __init__(self, app_id: str, private_key: str, alipay_public_key: str, gateway: str = "https://openapi.alipay.com/gateway.do"):
        """
        初始化支付宝支付

        Args:
            app_id: 应用ID
            private_key: 应用私钥
            alipay_public_key: 支付宝公钥
            gateway: 支付宝网关地址
        """
        self.app_id = app_id
        self.private_key = private_key
        self.alipay_public_key = alipay_public_key
        self.gateway = gateway

    async def create_payment(
        self,
        order_no: str,
        amount: int,
        subject: str,
        description: str,
        return_url: Optional[str] = None,
        notify_url: Optional[str] = None,
    ) -> Dict:
        """创建支付宝支付订单"""
        # 构建请求参数
        params = {
            "app_id": self.app_id,
            "method": "alipay.trade.page.pay",
            "format": "JSON",
            "charset": "utf-8",
            "sign_type": "RSA2",
            "timestamp": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "version": "1.0",
            "notify_url": notify_url,
            "return_url": return_url,
            "biz_content": {
                "out_trade_no": order_no,
                "product_code": "FAST_INSTANT_TRADE_PAY",
                "total_amount": f"{amount / 100:.2f}",  # 分转元
                "subject": subject,
                "body": description,
            },
        }

        # 生成签名
        sign = self._generate_sign(params)
        params["sign"] = sign

        # 构建支付URL
        payment_url = f"{self.gateway}?{urllib.parse.urlencode(params)}"

        return {
            "payment_url": payment_url,
            "payment_id": order_no,
            "provider": "alipay",
        }

    async def verify_payment(self, payment_data: Dict) -> bool:
        """验证支付宝回调签名"""
        sign = payment_data.pop("sign", None)
        if not sign:
            return False

        # 验证签名
        return self._verify_sign(payment_data, sign)

    async def query_payment(self, order_no: str) -> Dict:
        """查询支付宝支付状态"""
        # 构建查询请求
        params = {
            "app_id": self.app_id,
            "method": "alipay.trade.query",
            "format": "JSON",
            "charset": "utf-8",
            "sign_type": "RSA2",
            "timestamp": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "version": "1.0",
            "biz_content": {
                "out_trade_no": order_no,
            },
        }

        # 实际应用中需要调用支付宝API
        # 这里返回模拟数据
        return {
            "status": "pending",
            "paid_at": None,
        }

    async def refund_payment(
        self, order_no: str, refund_amount: int, reason: str
    ) -> Dict:
        """支付宝退款"""
        # 构建退款请求
        params = {
            "app_id": self.app_id,
            "method": "alipay.trade.refund",
            "format": "JSON",
            "charset": "utf-8",
            "sign_type": "RSA2",
            "timestamp": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "version": "1.0",
            "biz_content": {
                "out_trade_no": order_no,
                "refund_amount": f"{refund_amount / 100:.2f}",
                "refund_reason": reason,
            },
        }

        # 实际应用中需要调用支付宝API
        # 这里返回模拟数据
        return {
            "success": True,
            "refund_id": f"refund_{order_no}",
        }

    def _generate_sign(self, params: Dict) -> str:
        """生成签名"""
        # 排序参数
        sorted_params = sorted(params.items())
        # 拼接字符串
        sign_str = "&".join([f"{k}={v}" for k, v in sorted_params if v])
        # 使用RSA2签名（实际应用中需要使用真实的RSA签名）
        # 这里使用简单的MD5模拟
        return hashlib.md5(sign_str.encode()).hexdigest()

    def _verify_sign(self, params: Dict, sign: str) -> bool:
        """验证签名"""
        # 生成签名并比对
        generated_sign = self._generate_sign(params)
        return generated_sign == sign
