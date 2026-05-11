"""
支付管理服务
"""
from typing import Dict, Optional
from sqlalchemy.orm import Session
from datetime import datetime
import uuid

from app.models.models import Order, License, LicenseType
from app.services.payment_base import PaymentProvider
from app.services.alipay_provider import AlipayProvider
from app.services.wechat_provider import WeChatPayProvider
from app.services.stripe_provider import StripeProvider


class PaymentService:
    """支付管理服务"""

    def __init__(self, config: Dict):
        """
        初始化支付服务

        Args:
            config: 支付配置
                {
                    "alipay": {"app_id": "...", "private_key": "...", ...},
                    "wechat": {"app_id": "...", "mch_id": "...", ...},
                    "stripe": {"api_key": "...", "webhook_secret": "..."}
                }
        """
        self.providers: Dict[str, PaymentProvider] = {}

        # 初始化支付宝
        if "alipay" in config:
            alipay_config = config["alipay"]
            self.providers["alipay"] = AlipayProvider(
                app_id=alipay_config["app_id"],
                private_key=alipay_config["private_key"],
                alipay_public_key=alipay_config["alipay_public_key"],
            )

        # 初始化微信支付
        if "wechat" in config:
            wechat_config = config["wechat"]
            self.providers["wechat"] = WeChatPayProvider(
                app_id=wechat_config["app_id"],
                mch_id=wechat_config["mch_id"],
                api_key=wechat_config["api_key"],
                notify_url=wechat_config["notify_url"],
            )

        # 初始化Stripe
        if "stripe" in config:
            stripe_config = config["stripe"]
            self.providers["stripe"] = StripeProvider(
                api_key=stripe_config["api_key"],
                webhook_secret=stripe_config["webhook_secret"],
            )

    async def create_order(
        self,
        db: Session,
        user_id: int,
        plugin_id: int,
        amount: int,
        license_type: LicenseType,
        payment_method: str,
    ) -> Order:
        """
        创建订单

        Args:
            db: 数据库会话
            user_id: 用户ID
            plugin_id: 插件ID
            amount: 金额（分）
            license_type: 授权类型
            payment_method: 支付方式

        Returns:
            订单对象
        """
        # 生成订单号
        order_no = self._generate_order_no()

        # 创建订单
        order = Order(
            order_no=order_no,
            user_id=user_id,
            plugin_id=plugin_id,
            amount=amount,
            license_type=license_type,
            payment_method=payment_method,
            payment_status="pending",
        )

        db.add(order)
        db.commit()
        db.refresh(order)

        return order

    async def create_payment(
        self,
        order: Order,
        subject: str,
        description: str,
        return_url: Optional[str] = None,
        notify_url: Optional[str] = None,
    ) -> Dict:
        """
        创建支付

        Args:
            order: 订单对象
            subject: 商品标题
            description: 商品描述
            return_url: 支付完成后跳转地址
            notify_url: 异步通知地址

        Returns:
            支付信息
        """
        provider = self.providers.get(order.payment_method)
        if not provider:
            raise ValueError(f"不支持的支付方式: {order.payment_method}")

        return await provider.create_payment(
            order_no=order.order_no,
            amount=order.amount,
            subject=subject,
            description=description,
            return_url=return_url,
            notify_url=notify_url,
        )

    async def verify_payment(self, payment_method: str, payment_data: Dict) -> bool:
        """
        验证支付回调

        Args:
            payment_method: 支付方式
            payment_data: 支付回调数据

        Returns:
            签名是否有效
        """
        provider = self.providers.get(payment_method)
        if not provider:
            return False

        return await provider.verify_payment(payment_data)

    async def handle_payment_callback(
        self, db: Session, order_no: str, payment_data: Dict
    ) -> bool:
        """
        处理支付回调

        Args:
            db: 数据库会话
            order_no: 订单号
            payment_data: 支付回调数据

        Returns:
            处理是否成功
        """
        # 查询订单
        order = db.query(Order).filter(Order.order_no == order_no).first()
        if not order:
            return False

        # 验证签名
        if not await self.verify_payment(order.payment_method, payment_data):
            return False

        # 更新订单状态
        order.payment_status = "paid"
        order.paid_at = datetime.utcnow()
        db.commit()

        # 生成授权证书
        await self._generate_license(db, order)

        return True

    async def query_payment(self, payment_method: str, order_no: str) -> Dict:
        """
        查询支付状态

        Args:
            payment_method: 支付方式
            order_no: 订单号

        Returns:
            支付状态信息
        """
        provider = self.providers.get(payment_method)
        if not provider:
            raise ValueError(f"不支持的支付方式: {payment_method}")

        return await provider.query_payment(order_no)

    async def refund_order(
        self, db: Session, order_no: str, reason: str
    ) -> Dict:
        """
        退款

        Args:
            db: 数据库会话
            order_no: 订单号
            reason: 退款原因

        Returns:
            退款结果
        """
        # 查询订单
        order = db.query(Order).filter(Order.order_no == order_no).first()
        if not order:
            raise ValueError("订单不存在")

        if order.payment_status != "paid":
            raise ValueError("订单未支付，无法退款")

        # 调用支付提供商退款
        provider = self.providers.get(order.payment_method)
        if not provider:
            raise ValueError(f"不支持的支付方式: {order.payment_method}")

        result = await provider.refund_payment(
            order_no=order.order_no,
            refund_amount=order.amount,
            reason=reason,
        )

        if result["success"]:
            # 更新订单状态
            order.payment_status = "refunded"
            db.commit()

            # 撤销授权证书
            await self._revoke_license(db, order)

        return result

    async def _generate_license(self, db: Session, order: Order):
        """生成授权证书"""
        # 生成授权密钥
        license_key = self._generate_license_key()

        # 计算过期时间
        expires_at = None
        if order.license_type == LicenseType.SUBSCRIPTION:
            # 订阅类型，1年后过期
            from datetime import timedelta
            expires_at = datetime.utcnow() + timedelta(days=365)

        # 创建授权证书
        license = License(
            license_key=license_key,
            plugin_id=order.plugin_id,
            user_id=order.user_id,
            device_id="",  # 首次激活时绑定设备
            license_type=order.license_type,
            expires_at=expires_at,
            is_active=True,
            signature=self._generate_signature(license_key),
        )

        db.add(license)
        db.commit()

    async def _revoke_license(self, db: Session, order: Order):
        """撤销授权证书"""
        licenses = (
            db.query(License)
            .filter(
                License.plugin_id == order.plugin_id,
                License.user_id == order.user_id,
                License.is_active == True,
            )
            .all()
        )

        for license in licenses:
            license.is_active = False

        db.commit()

    def _generate_order_no(self) -> str:
        """生成订单号"""
        timestamp = datetime.now().strftime("%Y%m%d%H%M%S")
        random_str = str(uuid.uuid4())[:8]
        return f"ORDER{timestamp}{random_str}"

    def _generate_license_key(self) -> str:
        """生成授权密钥"""
        return str(uuid.uuid4()).replace("-", "").upper()

    def _generate_signature(self, license_key: str) -> str:
        """生成签名"""
        import hashlib
        return hashlib.sha256(license_key.encode()).hexdigest()
