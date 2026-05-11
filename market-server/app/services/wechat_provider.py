"""
微信支付提供商
"""
from typing import Dict, Optional
from datetime import datetime
import hashlib
import xml.etree.ElementTree as ET
from .payment_base import PaymentProvider


class WeChatPayProvider(PaymentProvider):
    """微信支付提供商"""

    def __init__(self, app_id: str, mch_id: str, api_key: str, notify_url: str):
        """
        初始化微信支付

        Args:
            app_id: 应用ID
            mch_id: 商户号
            api_key: API密钥
            notify_url: 回调地址
        """
        self.app_id = app_id
        self.mch_id = mch_id
        self.api_key = api_key
        self.notify_url = notify_url
        self.gateway = "https://api.mch.weixin.qq.com/pay/unifiedorder"

    async def create_payment(
        self,
        order_no: str,
        amount: int,
        subject: str,
        description: str,
        return_url: Optional[str] = None,
        notify_url: Optional[str] = None,
    ) -> Dict:
        """创建微信支付订单"""
        # 构建请求参数
        params = {
            "appid": self.app_id,
            "mch_id": self.mch_id,
            "nonce_str": self._generate_nonce_str(),
            "body": subject,
            "out_trade_no": order_no,
            "total_fee": str(amount),  # 金额（分）
            "spbill_create_ip": "127.0.0.1",
            "notify_url": notify_url or self.notify_url,
            "trade_type": "NATIVE",  # 扫码支付
        }

        # 生成签名
        sign = self._generate_sign(params)
        params["sign"] = sign

        # 构建XML请求
        xml_data = self._dict_to_xml(params)

        # 实际应用中需要调用微信支付API
        # 这里返回模拟数据
        return {
            "payment_url": f"weixin://wxpay/bizpayurl?pr={order_no}",
            "payment_id": order_no,
            "provider": "wechat",
            "code_url": f"weixin://wxpay/bizpayurl?pr={order_no}",  # 二维码URL
        }

    async def verify_payment(self, payment_data: Dict) -> bool:
        """验证微信支付回调签名"""
        sign = payment_data.pop("sign", None)
        if not sign:
            return False

        # 验证签名
        return self._verify_sign(payment_data, sign)

    async def query_payment(self, order_no: str) -> Dict:
        """查询微信支付状态"""
        # 构建查询请求
        params = {
            "appid": self.app_id,
            "mch_id": self.mch_id,
            "out_trade_no": order_no,
            "nonce_str": self._generate_nonce_str(),
        }

        # 生成签名
        sign = self._generate_sign(params)
        params["sign"] = sign

        # 实际应用中需要调用微信支付API
        # 这里返回模拟数据
        return {
            "status": "pending",
            "paid_at": None,
        }

    async def refund_payment(
        self, order_no: str, refund_amount: int, reason: str
    ) -> Dict:
        """微信支付退款"""
        # 构建退款请求
        params = {
            "appid": self.app_id,
            "mch_id": self.mch_id,
            "nonce_str": self._generate_nonce_str(),
            "out_trade_no": order_no,
            "out_refund_no": f"refund_{order_no}",
            "total_fee": str(refund_amount),
            "refund_fee": str(refund_amount),
            "refund_desc": reason,
        }

        # 生成签名
        sign = self._generate_sign(params)
        params["sign"] = sign

        # 实际应用中需要调用微信支付API
        # 这里返回模拟数据
        return {
            "success": True,
            "refund_id": f"refund_{order_no}",
        }

    def _generate_nonce_str(self) -> str:
        """生成随机字符串"""
        import random
        import string
        return "".join(random.choices(string.ascii_letters + string.digits, k=32))

    def _generate_sign(self, params: Dict) -> str:
        """生成签名"""
        # 排序参数
        sorted_params = sorted(params.items())
        # 拼接字符串
        sign_str = "&".join([f"{k}={v}" for k, v in sorted_params if v])
        # 添加API密钥
        sign_str += f"&key={self.api_key}"
        # MD5签名
        return hashlib.md5(sign_str.encode()).hexdigest().upper()

    def _verify_sign(self, params: Dict, sign: str) -> bool:
        """验证签名"""
        generated_sign = self._generate_sign(params)
        return generated_sign == sign

    def _dict_to_xml(self, data: Dict) -> str:
        """字典转XML"""
        root = ET.Element("xml")
        for key, value in data.items():
            elem = ET.SubElement(root, key)
            elem.text = str(value)
        return ET.tostring(root, encoding="unicode")
