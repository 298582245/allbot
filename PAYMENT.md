# AllBot 通用支付方案

## 1. 背景与目标

当前内置支付只有积分支付，插件只能通过 SDK 调用积分扣减能力。新的支付体系需要把“支付方式”抽象成可配置、可扩展的统一能力，让插件作者只关心一次消费是否成功。

目标：

1. 在 SDK 中提供通用 `PAY` 类，插件可通过 `const pay = new PAY()` 或 `new PAY(ctx)` 发起支付。
2. 插件调用 `pay.waitPay(消费名称, 消费金额, 等待时间)` 后，系统自动完成询价、支付方式选择、扣积分或第三方下单、等待支付结果。
3. 系统设置增加“积分与 RMB 兑换比例”，默认 `1 RMB = 100 积分`。
4. 支付方式可自定义扩展，首个第三方支付渠道接入“易支付”。
5. 易支付支持 V1 与 V2 两种商户加密方式，支持用户自定义支付类型值，例如 `alipay`、`wxpay`、`qqpay` 或其他渠道自定义值。
6. 支付回调、订单状态、积分扣减必须幂等，避免重复扣款、重复加分或重复返回成功。

非目标：

1. 阶段一只实现支付设置、订单表、支付事件、积分流水、后台接口和系统设置页兑换比例字段。
2. 阶段一不实现 SDK `PAY` 类、不实现 `payment_wait` action、不实现易支付真实下单/回调闭环。
3. 本阶段不把支付与具体插件权益强绑定，`PAY.waitPay` 只返回消费成功/失败，插件自己决定成功后发放什么服务。
4. 本阶段不直接修改现有积分充值命令，但后续可以复用订单系统补充“充值积分”场景。

> 说明：本仓库本地 `pay/SDK_V1`、`pay/SDK_V2/SDK` 内的 PHP SDK 仅作为易支付协议参考，最终 Go、Node.js、Python 实现均自行实现签名、下单、查询和回调处理，不引入 PHP 运行依赖。已基于本地 PHP SDK 确认本文的 V1/V2 接口、签名与回调细节；阶段三接入真实下单/回调时仍需用 mock 与真实商户环境验证。

## 2. 现有代码复用点

后续实现应尽量复用现有模式：

1. 用户积分模型：`core/config/user_identity.go`
   - 当前积分按 `union_id` 聚合。
   - `ConsumeUserPoints(unionID, amount)` 用于扣积分。
   - `AddUserPoints(unionID, amount)` 用于加积分。
2. 插件上下文：`core/router/router.go`
   - 插件执行时已传入 `plugin_id`、`platform`、`adapter_id`、`user_id`、`union_id`、`points`、`points_unit`。
   - 支付订单应以 `union_id` 作为用户主体，同时保存平台与用户 ID 便于追踪和通知。
3. SDK 调用模式：
   - Node.js：`sdk/nodejs/allbot_direct.js`
   - Python：`sdk/python/allbot_direct.py`
   - 已有 `_request(action, expectedAction)` 与 `ctx.listen(timeout)`，适合新增 `payment_wait` action。
4. 系统设置：
   - 后端：`core/config/system_settings.go`、`core/web/settings.go`
   - 前端：`web-ui/src/views/Settings.vue`
   - 可在系统设置里增加积分兑换比例，也可新建支付设置页管理第三方支付配置。
5. Web 路由：`core/web/server.go`
   - 普通后台接口走 `/api/...`。
   - 第三方回调应走 `/api/open/...`，避免要求第三方携带后台登录 Token。

## 3. 核心业务流程

### 3.1 插件调用方式

Node.js 推荐写法：

```js
const { runDirect, PAY } = require('allbot_direct')

runDirect(async (ctx) => {
  const pay = new PAY(ctx)
  const result = await pay.waitPay('高级功能调用', 9.9, 300)
  if (!result.success) {
    await ctx.reply(result.message || '支付失败')
    return
  }
  await ctx.reply('支付成功，开始执行高级功能')
})
```

兼容用户期望的简写：

```js
const pay = new PAY()
await pay.waitPay('高级功能调用', 9.9, 300)
```

实现方式：SDK 在 `runDirect` 创建 `Context` 后，把当前上下文绑定到 `PAY` 的当前运行实例。`new PAY()` 自动取当前上下文；`new PAY(ctx)` 用于显式绑定和测试。

Python 同步提供：

```python
from allbot_direct import run_direct, PAY

async def handle(ctx):
    pay = PAY(ctx)
    result = await pay.wait_pay("高级功能调用", 9.9, 300)
    if not result["success"]:
        await ctx.reply(result.get("message") or "支付失败")
        return
    await ctx.reply("支付成功，开始执行高级功能")

run_direct(handle)
```

### 3.2 用户交互文本

插件调用：

```js
await pay.waitPay('高级功能调用', 9.9, 300)
```

系统向用户发送：

```text
当前消费 9.90 RMB（990 积分）
请选择支付方式
1. 积分支付
2. 支付宝
3. 微信支付
4. QQ钱包

PS：发送对应数字进行选择，订单将在 300 秒后超时。
```

选择积分支付：

1. 系统检查当前用户 `union_id`。
2. 按兑换比例计算需要扣除的积分。
3. 调用 `ConsumeUserPoints` 扣除积分。
4. 扣除成功则 `waitPay` 返回成功。
5. 余额不足或扣除失败则返回失败。

选择第三方支付：

1. 系统根据支付方式找到对应 provider，例如易支付。
2. 创建本地订单，状态为 `pending`。
3. 调用 provider 下单，保存第三方订单号、支付链接、二维码内容。
4. 向用户发送订单号、支付二维码或支付链接。
5. 系统等待回调或主动查询结果，最长等待 `waitPay` 传入的超时时间。
6. 回调验证通过并确认订单支付成功后，唤醒等待中的 `waitPay`，返回成功。
7. 超时前未支付则返回失败，并把订单标记为 `expired` 或 `timeout`。

### 3.3 积分兑换规则

系统设置新增字段：

```text
积分兑换比例：1 RMB = 100 积分
```

内部建议保存为：

```json
{
  "points_per_rmb": 100
}
```

金额计算规则：

1. `PAY.waitPay` 的消费金额单位为 RMB。
2. 后端统一把 RMB 转为“分”保存，避免浮点误差。
3. 积分金额按 `ceil(金额分 * points_per_rmb / 100)` 计算，避免小数金额被少扣。
4. 展示时保留两位 RMB，例如 `9.90 RMB（990 积分）`。

默认值：

```text
1 RMB = 100 积分
```

示例：

| RMB | points_per_rmb | 扣除积分 |
| --- | --- | --- |
| 1.00 | 100 | 100 |
| 9.90 | 100 | 990 |
| 0.01 | 100 | 1 |
| 10.00 | 50 | 500 |

## 4. 总体架构

新增支付模块建议分为四层：

```text
插件 SDK PAY 类
  ↓ payment_wait action
插件运行时 / Router 支付协调器
  ↓
PaymentService 订单服务
  ↓
PaymentProvider 支付渠道适配器
  ├─ points 内置积分支付
  └─ epay 易支付
```

职责划分：

1. `PAY` 类：给插件作者提供简单 API，不暴露订单细节。
2. 插件运行时：接收 `payment_wait` action，把当前插件上下文交给支付服务。
3. `PaymentService`：负责金额换算、订单创建、状态流转、等待回调、幂等处理。
4. `PaymentProvider`：负责不同支付渠道的下单、签名、回调解析、订单查询。
5. WebUI：负责配置积分兑换比例、启用支付方式、配置易支付商户信息、查看订单。

## 5. SDK PAY 类设计

### 5.1 Node.js API

```ts
class PAY {
  constructor(ctx?: Context)

  waitPay(
    subject: string,
    amountRmb: number | string,
    timeoutSeconds?: number,
    options?: {
      metadata?: Record<string, any>
      methods?: string[]
      remark?: string
    }
  ): Promise<PaymentResult>
}

type PaymentResult = {
  success: boolean
  orderNo: string
  subject: string
  amountRmb: string
  pointsAmount: number
  method: string
  provider: string
  status: string
  message: string
  paidAt?: string
  raw?: any
}
```

### 5.2 Python API

```python
class PAY:
    def __init__(self, ctx=None): ...

    async def wait_pay(self, subject, amount_rmb, timeout_seconds=300, **options): ...
    async def waitPay(self, subject, amount_rmb, timeout_seconds=300, **options): ...
```

### 5.3 SDK action

新增插件 action：

```json
{
  "action": "payment_wait",
  "request_id": "...",
  "subject": "高级功能调用",
  "amount_rmb": "9.90",
  "timeout": 300,
  "methods": [],
  "metadata": {}
}
```

响应：

```json
{
  "action": "payment_response",
  "request_id": "...",
  "success": true,
  "error": "",
  "data": {
    "order_no": "P202606031234560001",
    "subject": "高级功能调用",
    "amount_rmb": "9.90",
    "points_amount": 990,
    "method": "points",
    "provider": "points",
    "status": "paid",
    "message": "支付成功"
  }
}
```

## 6. 支付方式抽象

### 6.1 PaymentProvider 接口

Go 后端建议定义统一接口：

```go
type PaymentProvider interface {
    Code() string
    DisplayName() string
    CreateOrder(ctx context.Context, req PaymentCreateRequest) (*PaymentProviderOrder, error)
    VerifyNotify(r *http.Request) (*PaymentNotifyResult, error)
    QueryOrder(ctx context.Context, order *PaymentOrder) (*PaymentQueryResult, error)
}
```

核心结构：

```go
type PaymentCreateRequest struct {
    OrderNo     string
    Subject     string
    AmountCents int64
    Method      string
    NotifyURL   string
    ReturnURL   string
    ClientIP    string
    Metadata    map[string]string
}

type PaymentProviderOrder struct {
    ProviderOrderNo string
    PayURL          string
    QRCode          string
    Raw             string
}

type PaymentNotifyResult struct {
    OrderNo         string
    ProviderOrderNo string
    AmountCents     int64
    Paid            bool
    PaidAt          time.Time
    Raw             string
}
```

### 6.2 内置支付方式

#### points：积分支付

- provider：`points`
- method：`points`
- 不创建第三方订单。
- 直接调用 `ConsumeUserPoints(unionID, pointsAmount)`。
- 成功后订单状态记为 `paid`。

#### epay：易支付

- provider：`epay`
- method：由用户配置，常见值为：
  - `alipay`：支付宝
  - `wxpay`：微信支付
  - `qqpay`：QQ钱包
- method 支持自定义字符串和值，不限制为固定枚举。

## 7. 易支付接入方案

### 7.1 WebUI 配置项

建议新增“支付设置”页面，或在系统设置中增加支付区块。

基础配置：

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `payment.enabled` | 是否启用第三方支付 | false |
| `payment.points_per_rmb` | 积分兑换比例，1 RMB 对应多少积分 | 100 |
| `payment.methods` | 启用的支付方式列表 | points |

易支付配置：

| 字段 | 说明 |
| --- | --- |
| `epay.enabled` | 是否启用易支付 |
| `epay.base_url` | 易支付网关地址，例如用户自建网关地址 |
| `epay.merchant_id` | 商户 ID，常见字段名可能是 `pid` 或 `merchant_id` |
| `epay.version` | `v1` 或 `v2` |
| `epay.v1_md5_key` | V1 商户 MD5 密钥 |
| `epay.v2_platform_public_key` | V2 平台公钥，用于验签 |
| `epay.v2_merchant_private_key` | V2 商户私钥，用于签名 |
| `epay.supported_methods` | 支持的支付方式，可自定义 code 和名称 |
| `epay.notify_url` | 回调地址，系统自动生成展示，不建议手填 |
| `epay.return_url` | 用户支付完成后的跳转地址，可选 |

支付方式配置示例：

```json
[
  { "code": "points", "label": "积分支付", "provider": "points", "enabled": true },
  { "code": "alipay", "label": "支付宝", "provider": "epay", "enabled": true },
  { "code": "wxpay", "label": "微信支付", "provider": "epay", "enabled": true },
  { "code": "qqpay", "label": "QQ钱包", "provider": "epay", "enabled": false },
  { "code": "custompay", "label": "自定义支付", "provider": "epay", "enabled": false }
]
```

### 7.2 易支付 V1 适配

V1 方案使用商户 MD5 密钥，已基于 `pay/SDK_V1` 确认以下细节。

配置：

| 字段 | 说明 |
| --- | --- |
| `apiurl` | 易支付接口地址，SDK 直接拼接路径 |
| `pid` | 商户 ID |
| `key` | 商户 MD5 密钥 |
| `sign_type` | 固定 `MD5` |

接口路径：

| 能力 | 路径 |
| --- | --- |
| 页面跳转下单 | `submit.php` |
| API 下单 | `mapi.php` |
| 订单查询 | `api.php?act=order` |
| 订单退款 | `api.php?act=refund` |

下单字段：

- `type`：支付类型，例如 `alipay`、`wxpay`、`qqpay` 或渠道自定义值。
- `notify_url`：异步回调地址。
- `return_url`：同步跳转地址。
- `out_trade_no`：本地订单号。
- `name`：商品或消费名称。
- `money`：金额，RMB 字符串，保留两位。

回调字段：

- `out_trade_no`：本地订单号。
- `trade_no`：易支付订单号。
- `trade_status`：支付状态，成功为 `TRADE_SUCCESS`。
- `type`：支付类型。
- `money`：支付金额。
- `sign`：签名。
- `sign_type`：签名类型。

回调响应：处理成功返回纯文本 `success`，失败返回纯文本 `fail`。

签名规则：参数名升序排序，排除 `sign`、`sign_type` 和空值，按 `k=v&k=v` 拼接后直接追加商户密钥，计算 MD5 并输出小写十六进制。

### 7.3 易支付 V2 适配

V2 方案使用平台公钥与商户私钥，已基于 `pay/SDK_V2/SDK` 确认以下细节。

配置：

| 字段 | 说明 |
| --- | --- |
| `apiurl` | 易支付接口地址，SDK 直接拼接路径 |
| `pid` | 商户 ID |
| `platform_public_key` | 平台公钥，用于响应和回调验签 |
| `merchant_private_key` | 商户私钥，用于请求签名 |
| `sign_type` | 固定 `RSA` |

接口路径：

| 能力 | 路径 |
| --- | --- |
| 页面跳转下单 | `api/pay/submit` |
| API 下单 | `api/pay/create` |
| 订单查询 | `api/pay/query` |
| 订单退款 | `api/pay/refund` |

请求字段与 V1 相同，包含 `type`、`notify_url`、`return_url`、`out_trade_no`、`name`、`money`；请求构建时自动补充 `pid`、`timestamp`、`sign`、`sign_type=RSA`。

签名规则：参数名升序排序，排除数组值、空值、`sign`、`sign_type`，按 `k=v&k=v` 拼接待签名字符串；使用商户私钥执行 RSA-SHA256 签名，签名结果 Base64 编码。

验签规则：使用平台公钥对待签名字符串执行 RSA-SHA256 验签；`timestamp` 必须存在，允许窗口为 300 秒。接口响应 `code == 0` 才视为业务成功，成功响应仍需验签后才能使用。回调响应同 V1：处理成功返回纯文本 `success`，失败返回纯文本 `fail`。

### 7.4 回调地址

建议地址：

```text
/api/open/payments/notify/epay
```

可选同步跳转地址：

```text
/api/open/payments/return/epay
```

回调处理规则：

1. 只接受已启用 provider 的回调。
2. 先解析 provider 回调，再查找本地订单。
3. 校验订单号存在、金额一致、状态可流转。
4. 校验签名或验签结果。
5. 幂等更新订单状态。
6. 唤醒等待中的 `PAY.waitPay`。
7. 按易支付文档返回指定成功文本，例如 `success`，具体值待文档核验。

## 8. 数据库设计

### 8.1 payment_orders

```sql
CREATE TABLE IF NOT EXISTS payment_orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_no TEXT NOT NULL UNIQUE,
  plugin_id TEXT NOT NULL DEFAULT '',
  union_id TEXT NOT NULL,
  platform TEXT NOT NULL DEFAULT '',
  adapter_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  group_id TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL,
  amount_cents INTEGER NOT NULL,
  points_amount INTEGER NOT NULL,
  provider TEXT NOT NULL,
  method TEXT NOT NULL,
  status TEXT NOT NULL,
  provider_order_no TEXT NOT NULL DEFAULT '',
  pay_url TEXT NOT NULL DEFAULT '',
  qrcode TEXT NOT NULL DEFAULT '',
  notify_raw TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}',
  expired_at DATETIME NOT NULL,
  paid_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

索引：

```sql
CREATE INDEX IF NOT EXISTS idx_payment_orders_union_id ON payment_orders(union_id);
CREATE INDEX IF NOT EXISTS idx_payment_orders_status ON payment_orders(status);
CREATE INDEX IF NOT EXISTS idx_payment_orders_provider_order_no ON payment_orders(provider, provider_order_no);
CREATE INDEX IF NOT EXISTS idx_payment_orders_created_at ON payment_orders(created_at);
```

状态建议：

| 状态 | 说明 |
| --- | --- |
| `created` | 已创建，尚未选择支付方式 |
| `pending` | 第三方订单已创建，等待支付 |
| `paid` | 已支付成功 |
| `failed` | 支付失败 |
| `expired` | 等待超时或订单过期 |
| `cancelled` | 用户取消 |

### 8.2 payment_events

建议增加订单事件表，方便排查回调和状态变更：

```sql
CREATE TABLE IF NOT EXISTS payment_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_no TEXT NOT NULL,
  event_type TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 8.3 point_transactions

当前系统只有 `user_points` 余额，没有积分流水。支付涉及真实金额，建议新增积分流水：

```sql
CREATE TABLE IF NOT EXISTS point_transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  union_id TEXT NOT NULL,
  delta INTEGER NOT NULL,
  balance_after INTEGER NOT NULL,
  source TEXT NOT NULL,
  source_id TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

用途：

1. 记录积分支付扣减。
2. 记录第三方支付充值到账。
3. 记录人工调整。
4. 后续支持退款、冲正、审计。

阶段一新增三类支付相关流水与记录：

1. `payment_orders`：保存本地支付订单主记录。
2. `payment_events`：保存订单状态流转、回调、渠道信息更新等事件。
3. `point_transactions`：保存积分变动流水，后续积分支付和充值到账统一复用。

## 9. 后端模块落点

建议新增文件：

```text
core/config/payment.go              # 支付订单、支付配置、积分流水 CRUD
core/payment/service.go             # PaymentService，订单流程协调
core/payment/provider.go            # PaymentProvider 接口
core/payment/provider_points.go     # 积分支付 provider
core/payment/provider_epay.go       # 易支付 provider
core/payment/wait_hub.go            # 回调唤醒 waitPay 的等待中心
core/web/payment.go                 # 后台支付设置、订单列表、回调入口
```

需要修改文件：

```text
core/config/database.go             # 初始化支付订单表、事件表、流水表
core/config/system_settings.go      # 增加 points_per_rmb 或 PaymentSettings
core/web/server.go                  # 注册支付后台接口和 open 回调接口
core/plugin/manager.go              # 增加 payment_wait action 分发
core/router/router.go               # 构造 PaymentService 所需上下文，复用 reply/listen/sendMessage
sdk/nodejs/allbot_direct.js         # 增加 PAY 类和 payment_wait action
sdk/python/allbot_direct.py         # 增加 PAY 类和 payment_wait action
web-ui/src/views/Settings.vue       # 增加积分兑换比例字段
web-ui/src/views/PaymentSettings.vue# 可选，第三方支付配置页面
```

## 10. 后台接口设计

后台管理接口：

```text
GET  /api/payments/settings          # 获取支付设置
PUT  /api/payments/settings          # 保存支付设置
GET  /api/payments/orders            # 查询订单列表
GET  /api/payments/orders/{orderNo}  # 查询订单详情
POST /api/payments/orders/{orderNo}/query # 主动查询第三方订单状态
```

开放回调接口：

```text
POST /api/open/payments/notify/epay
GET  /api/open/payments/return/epay
```

后台设置保存要求：

1. `points_per_rmb` 必须大于 0。
2. `epay.base_url` 必须是完整 URL。
3. `epay.version` 只能是 `v1` 或 `v2`。
4. V1 必须填写 MD5 密钥。
5. V2 必须填写平台公钥和商户私钥。
6. 支付方式 code 不能为空，同一 provider 内不能重复。
7. 密钥类字段不在订单详情和普通 GET 响应中明文返回，可用 `has_key` 标记是否已配置。

## 11. `PAY.waitPay` 后端执行细节

### 11.1 统一流程

1. 校验插件上下文：必须有 `union_id`。
2. 校验 `subject` 非空、`amount_rmb > 0`、`timeout > 0`。
3. 读取支付设置与启用方式。
4. 计算 `amount_cents` 和 `points_amount`。
5. 发送支付方式选择提示。
6. 调用现有 `listenFunc(timeout)` 等待用户选择。
7. 用户选择非法或超时，返回失败。
8. 用户选择积分支付，执行积分扣减流程。
9. 用户选择第三方支付，执行第三方下单和等待流程。

### 11.2 积分支付流程

1. 创建本地订单，provider=`points`，method=`points`。
2. 调用 `ConsumeUserPoints(unionID, pointsAmount)`。
3. 成功：订单状态更新为 `paid`，写入积分流水，返回成功。
4. 失败：订单状态更新为 `failed`，返回失败原因。

余额判断以 `ConsumeUserPoints` 为准，不在 SDK 侧判断，避免并发下余额不一致。

### 11.3 第三方支付流程

1. 创建本地订单，provider=`epay`，method=`alipay/wxpay/qqpay/...`。
2. 调用 `EpayProvider.CreateOrder`。
3. 保存 `provider_order_no`、`pay_url`、`qrcode`。
4. 向用户发送：

```text
订单已创建
订单号：P202606031234560001
支付方式：支付宝
支付金额：9.90 RMB
支付链接：https://...
二维码：...
请在 300 秒内完成支付。
```

5. `PaymentWaitHub` 注册 `order_no -> channel`。
6. 回调成功时，订单服务更新订单为 `paid` 并通知 channel。
7. `waitPay` 收到成功结果，返回成功。
8. 超时未支付，订单状态更新为 `expired`，返回失败。

### 11.4 超时与迟到回调

如果 `waitPay` 已超时，但第三方之后仍通知支付成功：

1. 如果订单未过期且金额有效，可标记 `paid`，但插件侧已经返回失败，无法继续同步执行。
2. 如果订单已标记 `expired`，应记录 `paid_late` 事件，并按运营规则处理。
3. 建议后续增加“迟到支付通知”给管理员或用户，避免用户付款但插件未发放服务。

初版建议：第三方订单超时时间与 `waitPay` 等待时间保持一致，下单时尽量传入订单有效期，减少迟到支付。

## 12. 幂等与一致性规则

1. `payment_orders.order_no` 全局唯一。
2. 同一个订单只能从 `pending` 流转到 `paid` 一次。
3. 回调重复到达时，如果订单已 `paid`，直接返回 provider 需要的成功文本，不重复处理。
4. 金额不一致的回调不得更新为成功。
5. provider 订单号不一致时不得更新为成功。
6. 积分扣减与订单状态必须尽量在同一事务内完成。
7. 如果现有 `AddUserPoints` / `ConsumeUserPoints` 事务边界不足，应新增内部方法支持在支付事务中变更积分。
8. 所有订单状态变化写入 `payment_events`。

## 13. WebUI 方案

### 13.1 系统设置新增字段

在 `web-ui/src/views/Settings.vue` 的“插件配置”或新增“支付配置”区块中增加：

```text
积分兑换比例：1 RMB = [100] 积分
```

保存到后端：

```json
{
  "points_per_rmb": 100
}
```

### 13.2 支付设置页面

建议新增页面“支付设置”：

1. 支付总开关。
2. 积分支付开关。
3. 积分兑换比例。
4. 易支付开关。
5. 易支付地址。
6. 商户 ID。
7. 商户加密方式：V1 / V2。
8. V1 MD5 密钥。
9. V2 平台公钥。
10. V2 商户私钥。
11. 支付方式列表：支持新增、删除、自定义 code、显示名称、启用状态。
12. 回调地址展示和复制。

### 13.3 订单管理页面

建议新增页面“支付订单”：

1. 按订单号、用户、插件、状态、支付方式筛选。
2. 展示 RMB 金额、积分金额、provider、method、状态、创建时间、支付时间。
3. 查看订单原始回调和事件日志。
4. 对 `pending` 订单支持手动查询第三方状态。

## 14. 易支付配置示例

```json
{
  "enabled": true,
  "points_per_rmb": 100,
  "methods": [
    { "code": "points", "label": "积分支付", "provider": "points", "enabled": true },
    { "code": "alipay", "label": "支付宝", "provider": "epay", "enabled": true },
    { "code": "wxpay", "label": "微信支付", "provider": "epay", "enabled": true },
    { "code": "qqpay", "label": "QQ钱包", "provider": "epay", "enabled": false }
  ],
  "epay": {
    "enabled": true,
    "base_url": "https://pay.example.com",
    "merchant_id": "1000",
    "version": "v2",
    "v1_md5_key": "",
    "v2_platform_public_key": "-----BEGIN PUBLIC KEY-----...",
    "v2_merchant_private_key": "-----BEGIN PRIVATE KEY-----..."
  }
}
```

## 15. 实施阶段

### 阶段一：基础模型与设置

1. 新增支付配置结构。
2. 新增系统设置字段 `points_per_rmb`，默认 100。
3. 新增 `payment_orders`、`payment_events`、`point_transactions` 表。
4. 新增订单 CRUD 与状态流转方法。
5. 增加后台设置接口和 WebUI 字段。

验收：

- 可以保存积分兑换比例。
- 可以创建、查询、更新本地订单。
- 单元测试覆盖非法金额、非法比例、订单号唯一性。

### 阶段二：SDK PAY 与积分支付

1. Node.js SDK 增加 `PAY` 类。
2. Python SDK 增加 `PAY` 类。
3. 插件运行时增加 `payment_wait` action。
4. Router 复用 `reply` 与 `listen` 完成支付方式选择。
5. 完成积分支付扣减和结果返回。

验收：

- 插件可调用 `pay.waitPay('测试消费', 1, 60)`。
- 用户选择 `1` 后扣除 100 积分。
- 余额不足时返回失败。
- 超时未选择时返回失败。

### 阶段三：易支付接入

1. 实现 `EpayProvider`。
2. 支持 V1 MD5 签名。
3. 支持 V2 私钥签名与平台公钥验签。
4. 实现下单、回调、查询。
5. WebUI 支持易支付配置和自定义支付方式。

验收：

- 选择支付宝/微信/QQ钱包后能生成易支付订单。
- 回调成功后 `waitPay` 返回成功。
- 重复回调不重复处理。
- 金额不一致、签名失败、订单不存在时拒绝处理。

### 阶段四：订单管理与补偿能力

1. 新增订单管理页面。
2. 支持手动查询第三方订单状态。
3. 支持查看订单事件。
4. 增加迟到回调记录和管理员提示。

验收：

- 管理员可追踪每一笔支付。
- 可定位第三方回调失败或重复回调原因。
- 可查看积分流水。

## 16. 测试计划

后端测试：

```text
core/config/payment_test.go
core/payment/service_test.go
core/payment/provider_epay_test.go
core/web/payment_test.go
```

覆盖用例：

1. RMB 到积分换算。
2. 默认比例 `1:100`。
3. 非法金额拒绝。
4. 非法兑换比例拒绝。
5. 积分支付成功扣减。
6. 积分不足失败。
7. 订单超时失败。
8. 第三方下单成功保存支付链接和二维码。
9. 回调签名失败拒绝。
10. 回调金额不一致拒绝。
11. 重复回调幂等。
12. 支付成功只唤醒对应订单。
13. 订单事件写入。
14. 积分流水写入。

前端验证：

```bash
cd D:/Desktop/program/java/AITest/allbot/web-ui
npm run build
```

Go 验证：

```bash
go test ./...
```

项目记忆要求：如果后续改动 Go 代码且验证通过，再编译：

```bash
go build -o D:/Desktop/program/java/AITest/allbot/allbot.exe .
```

## 17. 风险与待确认项

1. 易支付文档当前无法读取，V1/V2 具体字段和签名算法必须在编码前核验。
2. `PAY.waitPay` 是同步等待模型，第三方支付可能存在用户支付很慢、回调迟到的问题，需要订单过期策略。
3. 当前积分变更方法自带事务，支付订单状态与积分变更如需强一致，可能要新增事务内积分变更方法。
4. 如果机器人平台不支持直接发送二维码图片，只能发送二维码链接或文本内容。
5. `new PAY()` 自动绑定上下文依赖插件进程单次消息模型；如果未来插件进程改成长驻并发，需要强制使用 `new PAY(ctx)`。
6. 支付密钥属于敏感配置，后端保存和前端展示需要避免明文泄漏。
7. 第三方支付成功但插件已超时返回失败时，需要运营补偿策略。

## 18. 推荐最终接口效果

插件作者只需要写：

```js
const pay = new PAY()
const result = await pay.waitPay('使用高级查询', 2.5, 180)
if (!result.success) return
// 支付成功后继续执行业务
```

用户看到：

```text
当前消费 2.50 RMB（250 积分）
请选择支付方式
1. 积分支付
2. 支付宝
3. 微信支付

PS：发送对应数字进行选择，订单将在 180 秒后超时。
```

系统内部完成：

```text
选择 1：扣积分 → 成功/失败
选择 2/3：易支付下单 → 发送订单号和二维码 → 等回调 → 成功/失败
```

这个设计把插件 API、积分支付、易支付和后续自定义 provider 解耦，后续新增其他支付渠道时只需要实现新的 `PaymentProvider`，WebUI 增加对应配置即可。