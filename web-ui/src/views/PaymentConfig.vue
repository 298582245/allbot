<template>
  <div class="payment-config page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span class="title">支付配置</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看支付配置说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
          <div class="header-actions">
          </div>
        </div>
      </template>

      <el-form class="payment-form" :model="paymentSettings" label-width="140px" v-loading="loading">
        <section class="form-section">
          <div class="section-title">基础支付规则</div>
          <el-form-item label="积分兑换比例">
            <el-input-number v-model="paymentSettings.points_per_rmb" :min="1" :step="1" :precision="0" />
            <span class="hint">1 {{ paymentSettings.currency_unit || 'RMB' }} = {{ paymentSettings.points_per_rmb || 1 }} 积分</span>
          </el-form-item>
          <el-form-item label="金额单位">
            <el-input v-model="paymentSettings.currency_unit" maxlength="16" placeholder="RMB" class="compact-input" />
            <span class="hint">用于聊天提示里的支付金额单位，默认 RMB</span>
          </el-form-item>
          <el-form-item label="单笔支付上限">
            <el-input-number v-model="maxPaymentAmount" :min="0.01" :step="0.01" :precision="2" controls-position="right" />
            <span class="hint">单笔支付金额不能超过此上限，默认 9999.99 元</span>
          </el-form-item>
          <el-form-item label="同时支付个数">
            <el-input-number v-model="paymentSettings.max_pending_payments" :min="1" :max="999" :step="1" :precision="0" />
            <span class="hint">待支付订单达到上限时，新支付会提示稍后再试</span>
          </el-form-item>
          <el-form-item label="第三方支付">
            <el-switch v-model="paymentSettings.third_party_enabled" />
            <span class="hint">{{ paymentSettings.third_party_enabled ? '启用第三方支付' : '仅使用积分支付' }}</span>
          </el-form-item>
          <el-form-item label="隐藏支付链接">
            <el-switch v-model="paymentSettings.hide_pay_url" />
            <span class="hint">开启后仅发送二维码图片，不在聊天文本里展示支付链接</span>
          </el-form-item>
          <el-form-item label="二维码图片接口">
            <el-input v-model="paymentSettings.qrcode_base_url" placeholder="例如 https://api.example.com/qrcode?key=xxx&data={content}，或 /api/open/qrcode?token=xxx&text=" />
            <span class="hint">可填任意返回图片的二维码接口；用 {content} 表示支付内容，或以参数等号结尾。留空则使用带订单 token 的内置支付二维码接口 /api/open/payments/qrcode/...</span>
          </el-form-item>
        </section>

        <section class="form-section">
          <div class="section-title">易支付通道</div>
          <el-form-item label="易支付">
            <el-switch v-model="paymentSettings.epay.enabled" :disabled="!paymentSettings.third_party_enabled" />
            <span class="hint">{{ paymentSettings.epay.enabled ? '启用易支付通道' : '关闭易支付通道' }}</span>
          </el-form-item>
          <el-form-item label="版本">
            <el-select v-model="paymentSettings.epay.version" class="compact-input" :disabled="epayDisabled">
              <el-option label="V1" value="v1" />
              <el-option label="V2" value="v2" />
            </el-select>
          </el-form-item>
          <el-form-item label="apiurl">
            <el-input v-model="paymentSettings.epay.apiurl" placeholder="https://pay.example.com/" :disabled="epayDisabled" />
          </el-form-item>
          <el-form-item label="pid">
            <el-input v-model="paymentSettings.epay.pid" placeholder="易支付商户 ID" :disabled="epayDisabled" />
          </el-form-item>
          <el-form-item v-if="paymentSettings.epay.version === 'v1'" label="V1 key">
            <el-input
              v-model="paymentSettings.epay.key"
              type="password"
              show-password
              autocomplete="new-password"
              :disabled="epayDisabled"
              :placeholder="paymentSettings.epay.has_key ? '已保存，留空则保留现有 V1 key' : '请输入 V1 商户密钥'"
            />
          </el-form-item>
          <template v-if="paymentSettings.epay.version === 'v2'">
            <el-form-item label="V2 平台公钥">
              <el-input
                v-model="paymentSettings.epay.platform_public_key"
                type="textarea"
                :rows="4"
                :disabled="epayDisabled"
                :placeholder="paymentSettings.epay.has_platform_public_key ? '已保存，留空则保留现有 platform_public_key' : '请输入 platform_public_key'"
              />
            </el-form-item>
            <el-form-item label="V2 商户私钥">
              <el-input
                v-model="paymentSettings.epay.merchant_private_key"
                type="textarea"
                :rows="4"
                :disabled="epayDisabled"
                :placeholder="paymentSettings.epay.has_merchant_private_key ? '已保存，留空则保留现有 merchant_private_key' : '请输入 merchant_private_key'"
              />
            </el-form-item>
          </template>
          <el-form-item label="return_url">
            <el-input v-model="paymentSettings.epay.return_url" placeholder="支付完成后的同步跳转地址" :disabled="epayDisabled" />
          </el-form-item>
          <el-form-item label="notify_url">
            <el-input :model-value="notifyUrl" readonly placeholder="填写 return_url 后自动推导" />
            <span class="hint">第三方异步回调地址</span>
          </el-form-item>
          <el-form-item label="自动查询间隔">
            <el-input-number v-model="paymentSettings.epay_query_interval_seconds" :min="1" :max="300" :step="1" :precision="0" :disabled="epayDisabled" />
            <span class="hint">等待支付期间自动查询订单是否已支付，默认 5 秒</span>
          </el-form-item>
          <el-form-item label="提交标题">
            <el-input v-model="paymentSettings.epay_submit_subject" maxlength="128" placeholder="留空则使用插件传入的支付标题" :disabled="epayDisabled" />
            <span class="hint">配置后提交给易支付的商品名称会使用这里的标题，本地订单仍保留插件标题</span>
          </el-form-item>
        </section>

        <section class="form-section">
          <div class="section-title">支付宝账单通道</div>
          <el-alert
            type="info"
            show-icon
            :closable="false"
            title="需要支付宝开放平台账单查询权限；系统按账单入账金额匹配订单，同一时间窗口内重复金额会自动增加 0.01 元。"
            class="section-alert"
          />
          <el-form-item label="支付宝账单">
            <el-switch v-model="paymentSettings.alipay_bill.enabled" :disabled="!paymentSettings.third_party_enabled" />
            <span class="hint">{{ paymentSettings.alipay_bill.enabled ? '启用支付宝账单通道' : '关闭支付宝账单通道' }}</span>
          </el-form-item>
          <el-form-item label="gateway_url">
            <el-input v-model="paymentSettings.alipay_bill.gateway_url" placeholder="https://openapi.alipay.com/gateway.do" :disabled="alipayBillDisabled" />
          </el-form-item>
          <el-form-item label="app_id">
            <el-input v-model="paymentSettings.alipay_bill.app_id" placeholder="支付宝开放平台应用 app_id" :disabled="alipayBillDisabled" />
          </el-form-item>
          <el-form-item label="应用私钥">
            <el-input
              v-model="paymentSettings.alipay_bill.private_key"
              type="textarea"
              :rows="4"
              :disabled="alipayBillDisabled"
              :placeholder="paymentSettings.alipay_bill.has_private_key ? '已保存，留空则保留现有应用私钥' : '请输入应用私钥'"
            />
          </el-form-item>
          <el-form-item label="支付宝公钥">
            <el-input
              v-model="paymentSettings.alipay_bill.alipay_public_key"
              type="textarea"
              :rows="4"
              :disabled="alipayBillDisabled"
              :placeholder="paymentSettings.alipay_bill.has_alipay_public_key ? '已保存，留空则保留现有支付宝公钥' : '请输入支付宝公钥'"
            />
          </el-form-item>
          <el-form-item label="app_auth_token">
            <el-input
              v-model="paymentSettings.alipay_bill.app_auth_token"
              type="password"
              show-password
              autocomplete="new-password"
              :disabled="alipayBillDisabled"
              :placeholder="paymentSettings.alipay_bill.has_app_auth_token ? '已保存，留空则保留现有授权令牌' : '可选，服务商代商户查询时填写'"
            />
          </el-form-item>
          <el-form-item label="收款 UID">
            <el-input v-model="paymentSettings.alipay_bill.transfer_user_id" placeholder="支付宝收款账号 UID" :disabled="alipayBillDisabled" />
          </el-form-item>
          <el-form-item label="收款姓名">
            <el-input v-model="paymentSettings.alipay_bill.transfer_user_name" placeholder="可选，用于生成转账链接" :disabled="alipayBillDisabled" />
          </el-form-item>
          <el-form-item label="收款码地址">
            <div class="receipt-qr-row">
              <el-input v-model="paymentSettings.alipay_bill.receipt_qr_url" placeholder="https://qr.alipay.com/fkx...，配置后优先使用收款码生成支付二维码" :disabled="alipayBillDisabled" />
              <el-button :disabled="alipayBillDisabled" @click="triggerReceiptQRUpload">上传解析</el-button>
              <input ref="receiptQRFileInput" class="hidden-file-input" type="file" accept="image/*" @change="handleReceiptQRUpload" />
            </div>
            <span class="hint">可上传支付宝收款码图片自动识别；请以系统显示的精确金额到账匹配。</span>
          </el-form-item>
          <el-form-item label="收银台公网地址">
            <el-input v-model="paymentSettings.alipay_bill.cashier_base_url" placeholder="例如 https://pay.example.com，未配置收款码时用于支付宝内网页拉起付款" :disabled="alipayBillDisabled" />
            <span class="hint">需要公网可访问；未配置收款码且填写该地址时，二维码会打开 /api/open/payments/alipay-bill/cashier/...。</span>
          </el-form-item>
          <el-form-item label="查询回溯分钟">
            <el-input-number v-model="paymentSettings.alipay_bill.query_minutes_back" :min="1" :max="1440" :step="1" :precision="0" :disabled="alipayBillDisabled" />
            <span class="hint">每轮查询最近多少分钟的支付宝账单，默认 30</span>
          </el-form-item>
          <el-form-item label="检查间隔秒">
            <el-input-number v-model="paymentSettings.alipay_bill.check_interval_seconds" :min="5" :max="300" :step="1" :precision="0" :disabled="alipayBillDisabled" />
            <span class="hint">后台监控检查间隔，范围 5-300 秒</span>
          </el-form-item>
          <el-form-item label="订单超时秒">
            <el-input-number v-model="paymentSettings.alipay_bill.order_timeout_seconds" :min="60" :max="86400" :step="30" :precision="0" :disabled="alipayBillDisabled" />
          </el-form-item>
          <el-form-item label="账单页大小">
            <el-input-number v-model="paymentSettings.alipay_bill.bill_page_size" :min="1" :max="1000" :step="10" :precision="0" :disabled="alipayBillDisabled" />
          </el-form-item>
          <el-form-item label="匹配模式">
            <el-input model-value="金额唯一匹配" readonly class="compact-input" />
          </el-form-item>
        </section>

        <section class="form-section">
          <div class="section-header">
            <div class="section-title">支付方式</div>
            <el-button size="small" @click="addMethod">新增方式</el-button>
          </div>
          <el-table class="payment-methods-table" :data="paymentSettings.methods" border size="small">
            <el-table-column label="code" min-width="120">
              <template #default="{ row }">
                <el-input v-model="row.code" placeholder="points/alipay" />
              </template>
            </el-table-column>
            <el-table-column label="label" min-width="140">
              <template #default="{ row }">
                <el-input v-model="row.label" placeholder="显示名称" />
              </template>
            </el-table-column>
            <el-table-column label="provider" min-width="120">
              <template #default="{ row }">
                <el-select v-model="row.provider" allow-create filterable default-first-option placeholder="provider">
                  <el-option label="points" value="points" />
                  <el-option label="epay" value="epay" />
                  <el-option label="alipay_bill" value="alipay_bill" />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="enabled" width="110" align="center">
              <template #default="{ row }">
                <el-switch v-model="row.enabled" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="90" align="center">
              <template #default="{ $index }">
                <el-button type="danger" link @click="removeMethod($index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </section>

        <div class="form-actions">
          <el-button type="primary" :loading="saving" @click="handleSave">保存支付配置</el-button>
          <el-button @click="loadPaymentSettings">重置</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
defineOptions({ name: 'PaymentConfig' })
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled } from '@element-plus/icons-vue'
import jsQR from 'jsqr'
import request from '@/utils/request'

const loading = ref(false)
const saving = ref(false)
const receiptQRFileInput = ref(null)
const defaultMaxPaymentAmountCents = 999999
const paymentSettings = reactive(createDefaultPaymentSettings())
const maxPaymentAmount = computed({
  get: () => paymentSettings.max_payment_amount_cents / 100,
  set: value => {
    const amount = Number(value)
    paymentSettings.max_payment_amount_cents = Number.isFinite(amount) && amount > 0 ? Math.round(amount * 100) : defaultMaxPaymentAmountCents
  }
})

const pageDescription = '配置积分兑换、第三方支付通道和同时待支付订单数量。'
const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '支付配置说明', { confirmButtonText: '知道了', type: 'info' })
}

const epayDisabled = computed(() => !paymentSettings.third_party_enabled || !paymentSettings.epay.enabled)
const alipayBillDisabled = computed(() => !paymentSettings.third_party_enabled || !paymentSettings.alipay_bill.enabled)
const notifyUrl = computed(() => {
  const value = String(paymentSettings.epay.return_url || '').trim()
  if (!value) return ''
  try {
    const parsed = new URL(value)
    parsed.pathname = '/api/open/payments/notify/epay'
    parsed.search = ''
    parsed.hash = ''
    return parsed.toString()
  } catch {
    return ''
  }
})

const loadPaymentSettings = async () => {
  loading.value = true
  try {
    const data = await request.get('/payments/settings')
    Object.assign(paymentSettings, createDefaultPaymentSettings(), normalizePaymentSettings(data))
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  saving.value = true
  try {
    const saved = await request.put('/payments/settings', normalizePaymentSettings(paymentSettings))
    Object.assign(paymentSettings, createDefaultPaymentSettings(), normalizePaymentSettings(saved))
    ElMessage.success('支付配置已保存')
  } finally {
    saving.value = false
  }
}

const addMethod = () => {
  paymentSettings.methods.push({ code: '', label: '', provider: 'epay', enabled: true })
}

const removeMethod = (index) => {
  paymentSettings.methods.splice(index, 1)
}

const triggerReceiptQRUpload = () => {
  receiptQRFileInput.value?.click()
}

const handleReceiptQRUpload = async (event) => {
  const input = event.target
  const file = input.files && input.files[0]
  input.value = ''
  if (!file) return
  try {
    const content = await decodeQRCodeFile(file)
    if (!content) {
      ElMessage.error('未识别到二维码内容')
      return
    }
    paymentSettings.alipay_bill.receipt_qr_url = normalizeReceiptQRContent(content)
    ElMessage.success('收款码地址已识别')
  } catch (error) {
    ElMessage.error(error?.message || '收款码解析失败')
  }
}

onMounted(loadPaymentSettings)

function decodeQRCodeFile(file) {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => {
      try {
        const canvas = document.createElement('canvas')
        canvas.width = image.naturalWidth || image.width
        canvas.height = image.naturalHeight || image.height
        const context = canvas.getContext('2d', { willReadFrequently: true })
        if (!context || canvas.width <= 0 || canvas.height <= 0) {
          reject(new Error('图片读取失败'))
          return
        }
        context.drawImage(image, 0, 0, canvas.width, canvas.height)
        const imageData = context.getImageData(0, 0, canvas.width, canvas.height)
        const result = jsQR(imageData.data, imageData.width, imageData.height)
        resolve(result?.data || '')
      } catch (error) {
        reject(error)
      } finally {
        URL.revokeObjectURL(image.src)
      }
    }
    image.onerror = () => {
      URL.revokeObjectURL(image.src)
      reject(new Error('图片加载失败'))
    }
    image.src = URL.createObjectURL(file)
  })
}

function normalizeReceiptQRContent(content) {
  const value = String(content || '').trim()
  if (!value) return ''
  const extracted = extractAlipayReceiptQRURL(value)
  try {
    const parsed = new URL(extracted)
    parsed.searchParams.delete('a')
    parsed.searchParams.delete('m')
    return parsed.toString()
  } catch {
    return extracted
  }
}

function extractAlipayReceiptQRURL(value) {
  try {
    const parsed = new URL(value)
    if (['render.alipay.com', 'ds.alipay.com'].includes(parsed.hostname)) {
      const qrcode = parsed.searchParams.get('qrcode')
      if (qrcode) return qrcode
      const scheme = parsed.searchParams.get('scheme') || ''
      const match = scheme.match(/qrcode=([^&]+)/)
      if (match) {
        return decodeURIComponent(match[1])
      }
    }
    if (parsed.protocol === 'alipays:') {
      const qrcode = parsed.searchParams.get('qrcode')
      if (qrcode) return qrcode
    }
  } catch {}
  return value
}

function createDefaultPaymentSettings() {
  return {
    points_per_rmb: 100,
    max_payment_amount_cents: defaultMaxPaymentAmountCents,
    currency_unit: 'RMB',
    max_pending_payments: 10,
    epay_query_interval_seconds: 5,
    epay_submit_subject: '',
    third_party_enabled: false,
    hide_pay_url: false,
    qrcode_base_url: '',
    methods: [
      { code: 'points', label: '积分支付', provider: 'points', enabled: true },
      { code: 'alipay_transfer', label: '支付宝转账', provider: 'alipay_bill', enabled: false },
      { code: 'alipay', label: '支付宝', provider: 'epay', enabled: false },
      { code: 'wxpay', label: '微信支付', provider: 'epay', enabled: false },
      { code: 'qqpay', label: 'QQ 钱包', provider: 'epay', enabled: false }
    ],
    epay: {
      enabled: false,
      version: 'v1',
      apiurl: '',
      pid: '',
      key: '',
      sign_type: 'MD5',
      platform_public_key: '',
      merchant_private_key: '',
      return_url: '',
      has_key: false,
      has_platform_public_key: false,
      has_merchant_private_key: false
    },
    alipay_bill: {
      enabled: false,
      gateway_url: 'https://openapi.alipay.com/gateway.do',
      app_id: '',
      private_key: '',
      alipay_public_key: '',
      app_auth_token: '',
      has_private_key: false,
      has_alipay_public_key: false,
      has_app_auth_token: false,
      transfer_user_id: '',
      transfer_user_name: '',
      receipt_qr_url: '',
      cashier_base_url: '',
      query_minutes_back: 30,
      check_interval_seconds: 15,
      order_timeout_seconds: 300,
      bill_page_size: 100,
      match_mode: 'amount_unique'
    }
  }
}

function normalizePaymentSettings(value) {
  const source = value && typeof value === 'object' ? value : {}
  const defaults = createDefaultPaymentSettings()
  const epay = source.epay && typeof source.epay === 'object' ? source.epay : {}
  const alipayBill = source.alipay_bill && typeof source.alipay_bill === 'object' ? source.alipay_bill : {}
  const pointsPerRmb = Number(source.points_per_rmb || defaults.points_per_rmb)
  const maxPaymentAmount = Number(source.max_payment_amount_cents || defaults.max_payment_amount_cents)
  const maxPending = Number(source.max_pending_payments || defaults.max_pending_payments)
  const queryInterval = Number(source.epay_query_interval_seconds || defaults.epay_query_interval_seconds)
  const alipayQueryMinutesBack = Number(alipayBill.query_minutes_back || defaults.alipay_bill.query_minutes_back)
  const alipayCheckInterval = Number(alipayBill.check_interval_seconds || defaults.alipay_bill.check_interval_seconds)
  const alipayOrderTimeout = Number(alipayBill.order_timeout_seconds || defaults.alipay_bill.order_timeout_seconds)
  const alipayBillPageSize = Number(alipayBill.bill_page_size || defaults.alipay_bill.bill_page_size)
  const version = String(epay.version || defaults.epay.version).trim().toLowerCase()
  return {
    ...defaults,
    ...source,
    points_per_rmb: Number.isFinite(pointsPerRmb) && pointsPerRmb > 0 ? Math.trunc(pointsPerRmb) : defaults.points_per_rmb,
    max_payment_amount_cents: Number.isFinite(maxPaymentAmount) && maxPaymentAmount > 0 ? Math.trunc(maxPaymentAmount) : defaults.max_payment_amount_cents,
    currency_unit: String(source.currency_unit || defaults.currency_unit).trim() || defaults.currency_unit,
    max_pending_payments: Number.isFinite(maxPending) && maxPending > 0 ? Math.trunc(maxPending) : defaults.max_pending_payments,
    epay_query_interval_seconds: Number.isFinite(queryInterval) && queryInterval > 0 ? Math.trunc(queryInterval) : defaults.epay_query_interval_seconds,
    epay_submit_subject: String(source.epay_submit_subject || '').trim(),
    hide_pay_url: Boolean(source.hide_pay_url),
    qrcode_base_url: String(source.qrcode_base_url || '').trim(),
    methods: normalizePaymentMethods(source.methods, defaults.methods),
    epay: {
      ...defaults.epay,
      ...epay,
      enabled: Boolean(epay.enabled),
      version: ['v1', 'v2'].includes(version) ? version : defaults.epay.version,
      key: String(epay.key || ''),
      platform_public_key: String(epay.platform_public_key || ''),
      merchant_private_key: String(epay.merchant_private_key || ''),
      sign_type: version === 'v2' ? 'RSA' : 'MD5',
      has_key: Boolean(epay.has_key),
      has_platform_public_key: Boolean(epay.has_platform_public_key),
      has_merchant_private_key: Boolean(epay.has_merchant_private_key)
    },
    alipay_bill: {
      ...defaults.alipay_bill,
      ...alipayBill,
      enabled: Boolean(alipayBill.enabled),
      gateway_url: String(alipayBill.gateway_url || defaults.alipay_bill.gateway_url).trim(),
      app_id: String(alipayBill.app_id || '').trim(),
      private_key: String(alipayBill.private_key || ''),
      alipay_public_key: String(alipayBill.alipay_public_key || ''),
      app_auth_token: String(alipayBill.app_auth_token || ''),
      has_private_key: Boolean(alipayBill.has_private_key),
      has_alipay_public_key: Boolean(alipayBill.has_alipay_public_key),
      has_app_auth_token: Boolean(alipayBill.has_app_auth_token),
      transfer_user_id: String(alipayBill.transfer_user_id || '').trim(),
      transfer_user_name: String(alipayBill.transfer_user_name || '').trim(),
      receipt_qr_url: String(alipayBill.receipt_qr_url || '').trim(),
      cashier_base_url: String(alipayBill.cashier_base_url || '').trim().replace(/\/$/, ''),
      query_minutes_back: Number.isFinite(alipayQueryMinutesBack) && alipayQueryMinutesBack > 0 ? Math.trunc(alipayQueryMinutesBack) : defaults.alipay_bill.query_minutes_back,
      check_interval_seconds: Number.isFinite(alipayCheckInterval) && alipayCheckInterval >= 5 ? Math.trunc(alipayCheckInterval) : defaults.alipay_bill.check_interval_seconds,
      order_timeout_seconds: Number.isFinite(alipayOrderTimeout) && alipayOrderTimeout > 0 ? Math.trunc(alipayOrderTimeout) : defaults.alipay_bill.order_timeout_seconds,
      bill_page_size: Number.isFinite(alipayBillPageSize) && alipayBillPageSize > 0 ? Math.trunc(alipayBillPageSize) : defaults.alipay_bill.bill_page_size,
      match_mode: 'amount_unique'
    }
  }
}

function normalizePaymentMethods(methods, defaults) {
  const source = Array.isArray(methods) ? methods : []
  const normalized = source.map(item => normalizePaymentMethod(item)).filter(item => item.code && item.provider)
  const result = normalized.length > 0 ? normalized : []
  const seen = new Set(result.map(item => `${item.code.toLowerCase()}\u0000${item.provider.toLowerCase()}`))
  defaults.forEach(item => {
    const key = `${String(item.code || '').trim().toLowerCase()}\u0000${String(item.provider || '').trim().toLowerCase()}`
    if (!seen.has(key)) {
      seen.add(key)
      result.push({ ...item })
    }
  })
  return result
}

function normalizePaymentMethod(value) {
  const source = value && typeof value === 'object' ? value : {}
  const code = String(source.code || '').trim()
  return {
    code,
    label: String(source.label || code).trim(),
    provider: String(source.provider || '').trim(),
    enabled: Boolean(source.enabled)
  }
}
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.title-row { display: flex; align-items: center; gap: 6px; }
.title { font-size: 18px; font-weight: 600; }
.mobile-info-button { display: none; padding: 0; font-size: 16px; }
.subtitle { margin-top: 6px; color: #909399; font-size: 13px; line-height: 1.5; }
.payment-form { flex: 1; min-height: 0; overflow-y: auto; padding-right: 4px; }
.form-section { padding: 14px 16px 10px; margin-bottom: 14px; border: 1px solid #ebeef5; border-radius: 10px; background: #fff; }
.section-title { margin-bottom: 14px; font-size: 15px; font-weight: 600; color: #303133; }
.section-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.section-header .section-title { margin-bottom: 0; }
.section-alert { margin-bottom: 12px; }
.hint { margin-left: 10px; color: #999; }
.receipt-qr-row { display: flex; align-items: center; gap: 8px; width: 100%; }
.hidden-file-input { display: none; }
.compact-input { width: 180px; }
.payment-methods-table { width: 100%; }
.form-actions { position: sticky; bottom: 0; z-index: 2; display: flex; justify-content: flex-end; gap: 10px; padding: 12px 0 0; background: linear-gradient(180deg, rgba(255,255,255,0), #fff 28%); }
@media (max-width: 768px) {
  .page-shell { height: calc(100dvh - 52px - 76px - 24px); overflow: hidden; }
  .page-card :deep(.el-card__body) { padding: 12px; }
  .page-header { align-items: flex-start; flex-direction: column; }
  .title { font-size: 16px; }
  .mobile-info-button { display: inline-flex; }
  .subtitle { display: none; }
  .payment-form { padding-right: 0; }
  .form-section { padding: 12px; border-radius: 12px; }
  .payment-config :deep(.el-form-item) { display: block; margin-bottom: 16px; }
  .payment-config :deep(.el-form-item__label) { width: 100% !important; justify-content: flex-start; padding: 0 0 6px; font-weight: 600; }
  .payment-config :deep(.el-form-item__content) { margin-left: 0 !important; }
  .payment-config :deep(.el-input-number), .compact-input { width: 100%; }
  .hint { display: block; margin: 6px 0 0; }
  .form-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; padding-top: 10px; }
  .form-actions .el-button { width: 100%; margin-left: 0; }
}
</style>
