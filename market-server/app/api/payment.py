"""
支付相关API
"""
from fastapi import APIRouter, Depends, HTTPException, status, Request
from sqlalchemy.orm import Session
from typing import Optional
from pydantic import BaseModel

from app.core.database import get_db
from app.main import get_current_active_user
from app.models.models import User, Plugin, Order, LicenseType
from app.services.payment_service import PaymentService
import os

router = APIRouter(prefix="/api/payment", tags=["payment"])

# 初始化支付服务
payment_config = {
    "alipay": {
        "app_id": os.getenv("ALIPAY_APP_ID", ""),
        "private_key": os.getenv("ALIPAY_PRIVATE_KEY", ""),
        "alipay_public_key": os.getenv("ALIPAY_PUBLIC_KEY", ""),
    },
    "wechat": {
        "app_id": os.getenv("WECHAT_APP_ID", ""),
        "mch_id": os.getenv("WECHAT_MCH_ID", ""),
        "api_key": os.getenv("WECHAT_API_KEY", ""),
        "notify_url": os.getenv("WECHAT_NOTIFY_URL", ""),
    },
    "stripe": {
        "api_key": os.getenv("STRIPE_API_KEY", ""),
        "webhook_secret": os.getenv("STRIPE_WEBHOOK_SECRET", ""),
    },
}

payment_service = PaymentService(payment_config)


class CreateOrderRequest(BaseModel):
    """创建订单请求"""
    plugin_id: int
    license_type: str  # "one_time" or "subscription"
    payment_method: str  # "alipay", "wechat", or "stripe"


class RefundRequest(BaseModel):
    """退款请求"""
    reason: str


@router.post("/orders")
async def create_order(
    request: CreateOrderRequest,
    current_user: User = Depends(get_current_active_user),
    db: Session = Depends(get_db),
):
    """创建订单"""
    # 查询插件
    plugin = db.query(Plugin).filter(Plugin.id == request.plugin_id).first()
    if not plugin:
        raise HTTPException(status_code=404, detail="插件不存在")

    # 计算金额
    license_type = LicenseType(request.license_type)
    if license_type == LicenseType.ONE_TIME:
        amount = plugin.price
    else:
        # 默认按年付
        amount = plugin.yearly_price

    # 创建订单
    order = await payment_service.create_order(
        db=db,
        user_id=current_user.id,
        plugin_id=plugin.id,
        amount=amount,
        license_type=license_type,
        payment_method=request.payment_method,
    )

    # 创建支付
    payment_info = await payment_service.create_payment(
        order=order,
        subject=plugin.name,
        description=plugin.description or "",
        return_url=os.getenv("PAYMENT_RETURN_URL", "http://localhost:8000/payment/success"),
        notify_url=os.getenv("PAYMENT_NOTIFY_URL", "http://localhost:8000/api/payment/notify"),
    )

    return {
        "order_no": order.order_no,
        "amount": order.amount,
        "payment_info": payment_info,
    }


@router.get("/orders/{order_no}")
async def get_order(
    order_no: str,
    current_user: User = Depends(get_current_active_user),
    db: Session = Depends(get_db),
):
    """查询订单"""
    order = db.query(Order).filter(Order.order_no == order_no).first()
    if not order:
        raise HTTPException(status_code=404, detail="订单不存在")

    # 验证权限
    if order.user_id != current_user.id:
        raise HTTPException(status_code=403, detail="无权访问此订单")

    return {
        "order_no": order.order_no,
        "plugin_id": order.plugin_id,
        "amount": order.amount,
        "license_type": order.license_type,
        "payment_method": order.payment_method,
        "payment_status": order.payment_status,
        "paid_at": order.paid_at,
        "created_at": order.created_at,
    }


@router.get("/orders")
async def list_orders(
    current_user: User = Depends(get_current_active_user),
    db: Session = Depends(get_db),
):
    """查询用户订单列表"""
    orders = (
        db.query(Order)
        .filter(Order.user_id == current_user.id)
        .order_by(Order.created_at.desc())
        .all()
    )

    return [
        {
            "order_no": order.order_no,
            "plugin_id": order.plugin_id,
            "amount": order.amount,
            "license_type": order.license_type,
            "payment_method": order.payment_method,
            "payment_status": order.payment_status,
            "paid_at": order.paid_at,
            "created_at": order.created_at,
        }
        for order in orders
    ]


@router.post("/notify/{payment_method}")
async def payment_notify(
    payment_method: str,
    request: Request,
    db: Session = Depends(get_db),
):
    """支付回调通知"""
    # 获取回调数据
    if payment_method == "alipay":
        payment_data = dict(await request.form())
    elif payment_method == "wechat":
        # 微信支付使用XML格式
        body = await request.body()
        # 解析XML（实际应用中需要实现XML解析）
        payment_data = {}
    elif payment_method == "stripe":
        # Stripe使用JSON格式
        payment_data = await request.json()
    else:
        raise HTTPException(status_code=400, detail="不支持的支付方式")

    # 提取订单号
    order_no = payment_data.get("out_trade_no") or payment_data.get("order_no")
    if not order_no:
        raise HTTPException(status_code=400, detail="缺少订单号")

    # 处理支付回调
    success = await payment_service.handle_payment_callback(
        db=db,
        order_no=order_no,
        payment_data=payment_data,
    )

    if not success:
        raise HTTPException(status_code=400, detail="支付回调处理失败")

    # 返回成功响应（不同支付平台要求不同的响应格式）
    if payment_method == "alipay":
        return "success"
    elif payment_method == "wechat":
        return "<xml><return_code><![CDATA[SUCCESS]]></return_code></xml>"
    else:
        return {"status": "success"}


@router.post("/orders/{order_no}/refund")
async def refund_order(
    order_no: str,
    request: RefundRequest,
    current_user: User = Depends(get_current_active_user),
    db: Session = Depends(get_db),
):
    """退款"""
    # 查询订单
    order = db.query(Order).filter(Order.order_no == order_no).first()
    if not order:
        raise HTTPException(status_code=404, detail="订单不存在")

    # 验证权限
    if order.user_id != current_user.id:
        raise HTTPException(status_code=403, detail="无权操作此订单")

    # 执行退款
    try:
        result = await payment_service.refund_order(
            db=db,
            order_no=order_no,
            reason=request.reason,
        )
        return result
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.get("/orders/{order_no}/status")
async def query_payment_status(
    order_no: str,
    current_user: User = Depends(get_current_active_user),
    db: Session = Depends(get_db),
):
    """查询支付状态"""
    # 查询订单
    order = db.query(Order).filter(Order.order_no == order_no).first()
    if not order:
        raise HTTPException(status_code=404, detail="订单不存在")

    # 验证权限
    if order.user_id != current_user.id:
        raise HTTPException(status_code=403, detail="无权访问此订单")

    # 查询支付状态
    payment_status = await payment_service.query_payment(
        payment_method=order.payment_method,
        order_no=order_no,
    )

    return {
        "order_no": order.order_no,
        "payment_status": order.payment_status,
        "paid_at": order.paid_at,
        "provider_status": payment_status,
    }
