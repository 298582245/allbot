<template>
  <div class="payment-orders page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <h2>订单管理</h2>
            <p>追踪支付订单、回调原文和状态事件，支持对易支付待支付订单手动查询。</p>
          </div>
          <el-button :loading="loading" @click="loadOrders">刷新</el-button>
        </div>
      </template>

      <div class="filter-panel desktop-filter-panel">
        <el-input v-model="filters.order_no" placeholder="订单号" clearable />
        <el-input v-model="filters.union_id" placeholder="用户 union_id" clearable />
        <el-input v-model="filters.plugin_id" placeholder="插件 ID" clearable />
        <el-select v-model="filters.status" placeholder="状态" clearable>
          <el-option label="pending" value="pending" />
          <el-option label="paid" value="paid" />
          <el-option label="failed" value="failed" />
          <el-option label="expired" value="expired" />
          <el-option label="cancelled" value="cancelled" />
        </el-select>
        <el-input v-model="filters.provider" placeholder="provider" clearable />
        <el-input v-model="filters.method" placeholder="method" clearable />
        <el-button type="primary" @click="handleSearch">查询</el-button>
      </div>

      <div class="mobile-filter-panel">
        <div class="mobile-search-row">
          <el-input v-model="filters.order_no" placeholder="输入订单号快速查询" clearable @keyup.enter="handleSearch" />
          <el-button type="primary" :loading="loading" @click="handleSearch">查</el-button>
          <el-button @click="mobileFiltersVisible = !mobileFiltersVisible">
            {{ mobileFiltersVisible ? '收起' : `筛选${advancedFilterCount ? `(${advancedFilterCount})` : ''}` }}
          </el-button>
        </div>
        <div v-show="mobileFiltersVisible" class="mobile-filter-extra">
          <el-input v-model="filters.union_id" placeholder="用户 union_id" clearable />
          <el-input v-model="filters.plugin_id" placeholder="插件 ID" clearable />
          <el-select v-model="filters.status" placeholder="状态" clearable>
            <el-option label="pending" value="pending" />
            <el-option label="paid" value="paid" />
            <el-option label="failed" value="failed" />
            <el-option label="expired" value="expired" />
            <el-option label="cancelled" value="cancelled" />
          </el-select>
          <el-input v-model="filters.provider" placeholder="provider" clearable />
          <el-input v-model="filters.method" placeholder="method" clearable />
          <el-button type="primary" @click="handleSearch">应用筛选</el-button>
        </div>
      </div>

      <div class="orders-action-bar">
        <span class="selected-count">已选 {{ selectedItems.length }} 项</span>
        <el-button
          size="small"
          type="danger"
          :disabled="!hasSelection || !!batchAction"
          :loading="batchAction === 'delete'"
          @click="deleteSelectedOrders"
        >批量删除</el-button>
      </div>

      <div class="orders-table-wrap desktop-orders-table" v-loading="loading">
        <el-table
          ref="desktopTableRef"
          :data="orders"
          border
          height="100%"
          empty-text="暂无支付订单"
          row-key="order_no"
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="50" reserve-selection />
          <el-table-column prop="order_no" label="订单号" min-width="190" show-overflow-tooltip />
          <el-table-column prop="subject" label="标题" min-width="150" show-overflow-tooltip />
          <el-table-column prop="union_id" label="用户" min-width="140" show-overflow-tooltip />
          <el-table-column prop="plugin_id" label="插件" min-width="120" show-overflow-tooltip />
          <el-table-column label="RMB" width="90">
            <template #default="{ row }">{{ formatAmount(row.amount_cents) }}</template>
          </el-table-column>
          <el-table-column prop="points_amount" label="积分" width="90" />
          <el-table-column prop="provider" label="渠道" width="90" />
          <el-table-column prop="method" label="方式" width="100" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" effect="plain">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" min-width="160">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="支付时间" min-width="160">
            <template #default="{ row }">{{ formatTime(row.paid_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="190" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openDetail(row.order_no)">详情</el-button>
              <el-button v-if="canQuery(row)" link type="warning" :loading="queryingOrder === row.order_no" @click="queryOrder(row.order_no)">查询</el-button>
              <el-button link type="danger" :loading="deletingOrderNo === row.order_no" @click="deleteOrder(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="mobile-orders-grid" v-loading="loading">
        <article
          v-for="row in orders"
          :key="orderKey(row)"
          class="mobile-order-card"
          :class="{ selected: isSelected(row) }"
        >
          <div class="mobile-order-head">
            <el-checkbox
              :model-value="isSelected(row)"
              @change="checked => toggleOrderSelection(row, checked)"
            />
            <div class="mobile-order-title">
              <strong>{{ row.subject || '未命名订单' }}</strong>
              <span>{{ row.order_no }}</span>
            </div>
            <el-tag :type="statusTagType(row.status)" effect="plain">{{ row.status }}</el-tag>
          </div>
          <div class="mobile-order-fields">
            <div><span>金额</span><strong>{{ formatAmount(row.amount_cents) }} RMB</strong></div>
            <div><span>积分</span><strong>{{ row.points_amount }}</strong></div>
            <div><span>用户</span><strong>{{ row.union_id }}</strong></div>
            <div><span>插件</span><strong>{{ row.plugin_id || '-' }}</strong></div>
            <div><span>渠道</span><strong>{{ row.provider }}</strong></div>
            <div><span>方式</span><strong>{{ row.method }}</strong></div>
            <div><span>创建时间</span><strong>{{ formatTime(row.created_at) }}</strong></div>
            <div><span>支付时间</span><strong>{{ formatTime(row.paid_at) }}</strong></div>
          </div>
          <div class="mobile-order-actions">
            <el-button size="small" type="primary" @click="openDetail(row.order_no)">详情</el-button>
            <el-button
              size="small"
              type="warning"
              :disabled="!canQuery(row)"
              :loading="queryingOrder === row.order_no"
              @click="queryOrder(row.order_no)"
            >查询</el-button>
            <el-button
              size="small"
              type="danger"
              :loading="deletingOrderNo === row.order_no"
              @click="deleteOrder(row)"
            >删除</el-button>
          </div>
        </article>
        <el-empty v-if="!loading && orders.length === 0" description="暂无支付订单" />
      </div>

      <div class="pagination-row">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          background
          @current-change="loadOrders"
        />
      </div>
    </el-card>

    <el-drawer v-model="detailVisible" title="支付订单详情" size="58%" class="order-detail-drawer">
      <div v-if="detail.order" class="detail-body">
        <section class="detail-section">
          <div class="section-title">订单信息</div>
          <div class="detail-grid">
            <div><span>订单号</span><strong>{{ detail.order.order_no }}</strong></div>
            <div><span>状态</span><strong>{{ detail.order.status }}</strong></div>
            <div><span>用户</span><strong>{{ detail.order.union_id }}</strong></div>
            <div><span>插件</span><strong>{{ detail.order.plugin_id || '-' }}</strong></div>
            <div><span>金额</span><strong>{{ formatAmount(detail.order.amount_cents) }} RMB</strong></div>
            <div><span>积分</span><strong>{{ detail.order.points_amount }}</strong></div>
            <div><span>渠道</span><strong>{{ detail.order.provider }}</strong></div>
            <div><span>方式</span><strong>{{ detail.order.method }}</strong></div>
            <div><span>第三方单号</span><strong>{{ detail.order.provider_order_no || '-' }}</strong></div>
            <div><span>创建时间</span><strong>{{ formatTime(detail.order.created_at) }}</strong></div>
            <div><span>过期时间</span><strong>{{ formatTime(detail.order.expired_at) }}</strong></div>
            <div><span>支付时间</span><strong>{{ formatTime(detail.order.paid_at) }}</strong></div>
          </div>
        </section>

        <section class="detail-section" v-if="detail.order.pay_url || detail.order.qrcode">
          <div class="section-title">支付信息</div>
          <el-input v-if="detail.order.pay_url" :model-value="detail.order.pay_url" readonly class="detail-input" />
          <el-input v-if="detail.order.qrcode" :model-value="detail.order.qrcode" readonly class="detail-input" />
        </section>

        <section class="detail-section">
          <div class="section-title">原始回调</div>
          <pre class="raw-box">{{ formatRawPayload(detail.order.notify_raw) || '暂无回调内容' }}</pre>
        </section>

        <section class="detail-section">
          <div class="section-title">事件日志</div>
          <el-timeline>
            <el-timeline-item v-for="event in detail.events" :key="event.id" :timestamp="formatTime(event.created_at)">
              <div class="event-title">{{ event.event_type }}：{{ formatRawPayload(event.message) || '-' }}</div>
              <pre v-if="event.payload" class="event-payload">{{ formatRawPayload(event.payload) }}</pre>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-if="!detail.events || detail.events.length === 0" description="暂无事件" />
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/request'

const loading = ref(false)
const queryingOrder = ref('')
const deletingOrderNo = ref('')
const batchAction = ref('')
const orders = ref([])
const selectedItems = ref([])
const desktopTableRef = ref(null)
const syncingDesktopSelection = ref(false)
const mobileFiltersVisible = ref(false)
const total = ref(0)
const currentPage = ref(1)
const pageSize = 20
const detailVisible = ref(false)
const detail = reactive({ order: null, events: [] })
const filters = reactive({ order_no: '', union_id: '', plugin_id: '', status: '', provider: '', method: '' })

const hasSelection = computed(() => selectedItems.value.length > 0)
const selectedOrderNoSet = computed(() => new Set(selectedItems.value.map(orderKey)))
const advancedFilterCount = computed(() => ['union_id', 'plugin_id', 'status', 'provider', 'method'].filter(key => String(filters[key] || '').trim()).length)

watch(orders, () => {
  const currentOrderNos = new Set(orders.value.map(orderKey))
  selectedItems.value = selectedItems.value.filter(item => currentOrderNos.has(orderKey(item)))
  nextTick(syncDesktopSelection)
})

const loadOrders = async () => {
  loading.value = true
  try {
    const params = {
      ...cleanFilters(filters),
      limit: pageSize,
      offset: (currentPage.value - 1) * pageSize
    }
    const data = await request.get('/payments/orders', { params })
    orders.value = Array.isArray(data.items) ? data.items : []
    total.value = Number(data.total || 0)
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  currentPage.value = 1
  loadOrders()
}

const openDetail = async (orderNo) => {
  const data = await request.get(`/payments/orders/${encodeURIComponent(orderNo)}`)
  detail.order = data.order || null
  detail.events = Array.isArray(data.events) ? data.events : []
  detailVisible.value = true
}

const queryOrder = async (orderNo) => {
  queryingOrder.value = orderNo
  try {
    const data = await request.post(`/payments/orders/${encodeURIComponent(orderNo)}/query`)
    ElMessage.success(data?.query?.status === 'paid' ? '订单已确认支付' : '已查询第三方状态')
    await loadOrders()
    if (detailVisible.value && detail.order?.order_no === orderNo) {
      await openDetail(orderNo)
    }
  } finally {
    queryingOrder.value = ''
  }
}

const deleteOrder = async (row) => {
  const orderNo = orderKey(row)
  await confirmDelete(`确定删除订单「${orderNo}」吗？`)
  deletingOrderNo.value = orderNo
  try {
    await request.delete(`/payments/orders/${encodeURIComponent(orderNo)}`)
    ElMessage.success('支付订单已删除')
    removeSelectedOrder(orderNo)
    closeDetailIfDeleted([orderNo])
    await loadOrders()
  } finally {
    deletingOrderNo.value = ''
  }
}

const deleteSelectedOrders = async () => {
  const targets = [...selectedItems.value]
  if (targets.length === 0) return

  await confirmDelete(`确定删除选中的 ${targets.length} 个支付订单吗？`)
  batchAction.value = 'delete'
  try {
    await Promise.all(
      targets.map(item => request.delete(`/payments/orders/${encodeURIComponent(item.order_no)}`))
    )
    ElMessage.success(`已删除 ${targets.length} 个支付订单`)
    closeDetailIfDeleted(targets.map(orderKey))
    clearSelection()
    await loadOrders()
  } finally {
    batchAction.value = ''
  }
}

const canQuery = (row) => row && row.provider === 'epay' && row.status === 'pending'

function orderKey(row) {
  return row?.order_no || ''
}

function handleSelectionChange(selection) {
  if (syncingDesktopSelection.value) return
  selectedItems.value = Array.isArray(selection) ? selection : []
}

function isSelected(row) {
  return selectedOrderNoSet.value.has(orderKey(row))
}

function toggleOrderSelection(row, checked) {
  const orderNo = orderKey(row)
  if (!orderNo) return

  if (checked) {
    if (!selectedOrderNoSet.value.has(orderNo)) selectedItems.value = [...selectedItems.value, row]
  } else {
    selectedItems.value = selectedItems.value.filter(item => orderKey(item) !== orderNo)
  }
  desktopTableRef.value?.toggleRowSelection(row, Boolean(checked))
}

function syncDesktopSelection() {
  syncingDesktopSelection.value = true
  desktopTableRef.value?.clearSelection()
  orders.value.forEach(row => {
    if (selectedOrderNoSet.value.has(orderKey(row))) desktopTableRef.value?.toggleRowSelection(row, true)
  })
  nextTick(() => {
    syncingDesktopSelection.value = false
  })
}

function clearSelection() {
  desktopTableRef.value?.clearSelection()
  selectedItems.value = []
}

function removeSelectedOrder(orderNo) {
  selectedItems.value = selectedItems.value.filter(item => orderKey(item) !== orderNo)
}

function closeDetailIfDeleted(orderNos) {
  const deleted = new Set(orderNos)
  if (detailVisible.value && detail.order?.order_no && deleted.has(detail.order.order_no)) {
    detailVisible.value = false
    detail.order = null
    detail.events = []
  }
}

function confirmDelete(title) {
  return ElMessageBox.confirm(
    `${title}\n\n删除后订单和事件日志会删除；积分流水不会删除；删除 pending 订单后，第三方回调可能无法再关联该订单。`,
    '删除支付订单',
    { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
  )
}

function cleanFilters(source) {
  const result = {}
  Object.entries(source).forEach(([key, value]) => {
    const text = String(value || '').trim()
    if (text) result[key] = text
  })
  return result
}

function formatAmount(cents) {
  const value = Number(cents || 0)
  return `${Math.floor(value / 100)}.${String(Math.abs(value % 100)).padStart(2, '0')}`
}

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

function formatRawPayload(value) {
  if (!value) return ''
  const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2)
  const decoded = decodeUnicodeEscapes(text)
  try {
    return JSON.stringify(JSON.parse(decoded), null, 2)
  } catch {
    return decoded
  }
}

function decodeUnicodeEscapes(text) {
  return String(text).replace(/\\u([0-9a-fA-F]{4})/g, (_, hex) => String.fromCharCode(parseInt(hex, 16)))
}

function statusTagType(status) {
  if (status === 'paid') return 'success'
  if (status === 'pending') return 'warning'
  if (status === 'failed' || status === 'expired' || status === 'cancelled') return 'danger'
  return 'info'
}

onMounted(loadOrders)
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.page-header h2 { margin: 0 0 6px; }
.page-header p { margin: 0; color: #909399; }
.filter-panel { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 10px; margin-bottom: 12px; }
.mobile-filter-panel { display: none; }
.orders-action-bar { display: flex; align-items: center; justify-content: flex-end; gap: 10px; margin-bottom: 12px; flex-shrink: 0; }
.selected-count { color: #909399; font-size: 13px; white-space: nowrap; }
.orders-table-wrap { flex: 1; min-height: 0; }
.mobile-orders-grid { display: none; }
.pagination-row { display: flex; justify-content: flex-end; padding-top: 12px; }
.detail-body { display: grid; gap: 16px; }
.detail-section { padding: 14px; border: 1px solid #ebeef5; border-radius: 10px; background: #fff; }
.section-title { margin-bottom: 12px; font-size: 15px; font-weight: 600; color: #303133; }
.detail-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.detail-grid div { min-width: 0; padding: 10px; border-radius: 8px; background: #f8fafc; }
.detail-grid span { display: block; margin-bottom: 6px; color: #909399; font-size: 12px; }
.detail-grid strong { display: block; color: #303133; word-break: break-all; }
.detail-input { margin-bottom: 8px; }
.raw-box,
.event-payload { max-height: 220px; margin: 0; padding: 10px; overflow: auto; border-radius: 8px; background: #0f172a; color: #dbeafe; font-size: 12px; line-height: 1.5; white-space: pre-wrap; word-break: break-word; }
.event-title { margin-bottom: 8px; color: #303133; font-weight: 600; }
@media (max-width: 768px) {
  .page-shell { height: calc(100dvh - 52px - 76px - 24px); overflow: hidden; }
  .page-card { height: 100%; min-height: 0; }
  .page-card :deep(.el-card__header) { padding: 10px 12px; flex-shrink: 0; }
  .page-card :deep(.el-card__body) { min-height: 0; padding: 10px 12px; overflow: hidden; }
  .page-header { align-items: center; flex-direction: row; gap: 8px; }
  .page-header h2 { margin: 0; font-size: 18px; }
  .page-header p { display: none; }
  .page-header > .el-button { width: auto; margin-left: 0; padding-inline: 10px; }
  .desktop-filter-panel { display: none; }
  .mobile-filter-panel { display: block; flex-shrink: 0; margin-bottom: 8px; }
  .mobile-search-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 8px; }
  .mobile-search-row .el-button { min-width: 44px; padding-inline: 10px; margin-left: 0; }
  .mobile-filter-extra { max-height: 34dvh; margin-top: 8px; padding: 8px; display: grid; gap: 8px; overflow: auto; border: 1px solid #ebeef5; border-radius: 10px; background: #f8fafc; }
  .mobile-filter-extra .el-button { width: 100%; margin-left: 0; }
  .orders-action-bar { display: grid; grid-template-columns: 1fr auto; align-items: center; margin-bottom: 8px; }
  .desktop-orders-table { display: none; }
  .mobile-orders-grid { flex: 1 1 0; min-height: 0; display: grid; align-content: start; gap: 10px; overflow-y: auto; overflow-x: hidden; padding-bottom: 8px; -webkit-overflow-scrolling: touch; }
  .mobile-order-card { padding: 12px; border: 1px solid #ebeef5; border-radius: 12px; background: #fff; box-shadow: 0 6px 18px rgba(15, 23, 42, 0.05); }
  .mobile-order-card.selected { border-color: #409eff; background: #f5faff; }
  .mobile-order-head { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: flex-start; gap: 10px; }
  .mobile-order-title { min-width: 0; display: grid; gap: 4px; }
  .mobile-order-title strong { color: #303133; word-break: break-word; overflow-wrap: anywhere; }
  .mobile-order-title span { color: #909399; font-size: 12px; word-break: break-word; overflow-wrap: anywhere; }
  .mobile-order-fields { margin-top: 10px; display: grid; gap: 7px; font-size: 12px; }
  .mobile-order-fields > div { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; min-width: 0; }
  .mobile-order-fields span { color: #909399; flex-shrink: 0; }
  .mobile-order-fields strong { min-width: 0; color: #303133; font-weight: 500; text-align: right; word-break: break-word; overflow-wrap: anywhere; }
  .mobile-order-actions { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; margin-top: 10px; padding-top: 10px; border-top: 1px solid #f0f2f5; }
  .mobile-order-actions .el-button { width: 100%; min-width: 0; margin-left: 0; padding-inline: 8px; }
  .pagination-row { justify-content: flex-start; overflow-x: auto; flex-shrink: 0; }
  .payment-orders :deep(.el-drawer) { width: 94vw !important; }
  .detail-grid { grid-template-columns: 1fr; }
  .mobile-orders-grid::-webkit-scrollbar,
  .pagination-row::-webkit-scrollbar { display: none; }
}
</style>
