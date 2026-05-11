# 支付集成系统文档

## 概述

AllBot市场服务器支持多种支付方式，包括支付宝、微信支付和Stripe。支付系统采用抽象设计，易于扩展新的支付提供商。

## 架构设计

### 核心组件

```
PaymentProvider (抽象基类)
├── AlipayProvider (支付宝)
├── WeChatPayProvider (微信支付)
└── StripeProvider (Stripe)

PaymentService (支付管理服务)
└── 统一管理所有支付提供商
```

### 支付流程

```
1. 用户选择插件和支付方式
   ↓
2. 创建订单 (POST /api/payment/orders)
   ↓
3. 生成支付链接
   ↓
4. 用户完成支付
   ↓
5. 支付平台回调 (POST /api/payment/notify/{payment_method})
   ↓
6. 验证签名并更新订单状态
   ↓
7. 自动生成授权证书
```

## API端点

### 1. 创建订单

**请求**:
```http
POST /api/payment/orders
Authorization: Bearer {token}
Content-Type: application/json

{
  "plugin_id": 1,
  "license_type": "one_time",  // 或 "subscription"
  "payment_method": "alipay"   // 或 "wechat", "stripe"
}
```

**响应**:
```json
{
  "order_no": "ORDER20260511123456abcd1234",
  "amount": 9900,
  "payment_info": {
    "payment_url": "https://...",
    "payment_id": "...",
    "provider": "alipay"
  }
}
```

### 2. 查询订单

**请求**:
```http
GET /api/payment/orders/{order_no}
Authorization: Bearer {token}
```

**响应**:
```json
{
  "order_no": "ORDER20260511123456abcd1234",
  "plugin_id": 1,
  "amount": 9900,
  "license_type": "one_time",
  "payment_method": "alipay",
  "payment_status": "paid",
  "paid_at": "2026-05-11T12:34:56Z",
  "created_at": "2026-05-11T12:30:00Z"
}
```

### 3. 查询用户订单列表

**请求**:
```http
GET /api/payment/orders
Authorization: Bearer {token}
```

**响应**:
```json
[
  {
    "order_no": "ORDER20260511123456abcd1234",
    "plugin_id": 1,
    "amount": 9900,
    "license_type": "one_time",
    "payment_method": "alipay",
    "payment_status": "paid",
    "paid_at": "2026-05-11T12:34:56Z",
    "created_at": "2026-05-11T12:30:00Z"
  }
]
```

### 4. 支付回调通知

**请求**:
```http
POST /api/payment/notify/{payment_method}
Content-Type: application/x-www-form-urlencoded (支付宝)
Content-Type: application/xml (微信支付)
Content-Type: application/json (Stripe)
```

**响应**:
- 支付宝: `"success"`
- 微信支付: `<xml><return_code><![CDATA[SUCCESS]]></return_code></xml>`
- Stripe: `{"status": "success"}`

### 5. 查询支付状态

**请求**:
```http
GET /api/payment/orders/{order_no}/status
Authorization: Bearer {token}
```

**响应**:
```json
{
  "order_no": "ORDER20260511123456abcd1234",
  "payment_status": "paid",
  "paid_at": "2026-05-11T12:34:56Z",
  "provider_status": {
    "status": "paid",
    "paid_at": "2026-05-11T12:34:56Z"
  }
}
```

### 6. 退款

**请求**:
```http
POST /api/payment/orders/{order_no}/refund
Authorization: Bearer {token}
Content-Type: application/json

{
  "reason": "用户申请退款"
}
```

**响应**:
```json
{
  "success": true,
  "refund_id": "refund_ORDER20260511123456abcd1234"
}
```

## 环境变量配置

在 `.env` 文件中配置支付参数：

```env
# 支付宝配置
ALIPAY_APP_ID=your_app_id
ALIPAY_PRIVATE_KEY=your_private_key
ALIPAY_PUBLIC_KEY=alipay_public_key

# 微信支付配置
WECHAT_APP_ID=your_app_id
WECHAT_MCH_ID=your_mch_id
WECHAT_API_KEY=your_api_key
WECHAT_NOTIFY_URL=https://your-domain.com/api/payment/notify/wechat

# Stripe配置
STRIPE_API_KEY=your_api_key
STRIPE_WEBHOOK_SECRET=your_webhook_secret

# 支付回调地址
PAYMENT_RETURN_URL=https://your-domain.com/payment/success
PAYMENT_NOTIFY_URL=https://your-domain.com/api/payment/notify
```

## 支付提供商集成

### 支付宝

1. 注册支付宝开放平台账号
2. 创建应用并获取 `app_id`
3. 生成RSA密钥对
4. 配置应用公钥到支付宝后台
5. 获取支付宝公钥

**文档**: https://opendocs.alipay.com/

### 微信支付

1. 注册微信商户平台账号
2. 获取 `app_id` 和 `mch_id`
3. 设置API密钥
4. 配置支付回调地址

**文档**: https://pay.weixin.qq.com/wiki/doc/api/

### Stripe

1. 注册Stripe账号
2. 获取API密钥（测试环境和生产环境）
3. 配置Webhook端点
4. 获取Webhook签名密钥

**文档**: https://stripe.com/docs/api

## 安全注意事项

1. **签名验证**: 所有支付回调必须验证签名
2. **HTTPS**: 生产环境必须使用HTTPS
3. **密钥保护**: 不要将密钥提交到版本控制
4. **金额校验**: 回调时验证订单金额
5. **幂等性**: 处理重复回调通知

## 扩展新的支付方式

1. 继承 `PaymentProvider` 基类
2. 实现所有抽象方法
3. 在 `PaymentService` 中注册新的提供商

示例：

```python
from app.services.payment_base import PaymentProvider

class NewPaymentProvider(PaymentProvider):
    async def create_payment(self, ...):
        # 实现创建支付逻辑
        pass

    async def verify_payment(self, ...):
        # 实现签名验证逻辑
        pass

    async def query_payment(self, ...):
        # 实现查询支付状态逻辑
        pass

    async def refund_payment(self, ...):
        # 实现退款逻辑
        pass
```

## 测试

### 单元测试

```bash
pytest tests/test_payment.py
```

### 集成测试

使用支付平台提供的沙箱环境进行测试。

## 常见问题

### Q: 支付回调没有收到？
A: 检查回调地址是否可公网访问，防火墙是否开放端口。

### Q: 签名验证失败？
A: 检查密钥配置是否正确，参数排序是否正确。

### Q: 退款失败？
A: 检查订单状态，确认是否已支付，退款金额是否正确。
