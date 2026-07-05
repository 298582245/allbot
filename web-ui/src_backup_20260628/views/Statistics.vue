<template>
  <div class="statistics-page page-shell" v-loading="loading">
    <div class="stats-sticky-header">
      <div class="statistics-toolbar">
        <div>
          <h2>数据统计</h2>
          <p>聚焦支付、脚本、图床、备份和系统资源，消息总量只展示跨天/跨月趋势。</p>
        </div>
        <el-button type="primary" :loading="loading" @click="loadAll">刷新</el-button>
      </div>

      <div class="mobile-chart-tabs">
        <button :class="{ active: mobileTab === 'overview' }" @click="mobileTab = 'overview'">概览</button>
        <button :class="{ active: mobileTab === 'trend' }" @click="mobileTab = 'trend'">趋势</button>
        <button :class="{ active: mobileTab === 'storage' }" @click="mobileTab = 'storage'">存储</button>
      </div>
    </div>

    <section class="business-overview" v-show="showOverviewCharts">
      <div class="overview-hero">
        <div class="hero-label">业务收入</div>
        <div class="hero-value">{{ formatMoney(payments.paid_amount_cents) }}</div>
        <div class="hero-footer">
          <span>今日 {{ formatMoney(payments.today_paid_amount_cents) }}</span>
          <span>{{ formatCompact(payments.paid_orders) }} 笔成功订单</span>
        </div>
      </div>
      <div class="overview-strip">
        <div v-for="item in overviewItems" :key="item.title" class="overview-item">
          <div class="overview-item-title">{{ item.title }}</div>
          <div class="overview-item-value">{{ item.value }}</div>
          <div class="overview-item-desc">{{ item.desc }}</div>
        </div>
      </div>
    </section>

    <el-row :gutter="16" class="chart-row" v-show="showOverviewCharts">
      <el-col :xs="24" :lg="12">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header">
              <span>支付订单状态</span>
              <el-tag effect="plain">{{ formatCompact(payments.total_orders) }} 单</el-tag>
            </div>
          </template>
          <div ref="paymentChartRef" class="chart" v-show="hasPaymentData"></div>
          <el-empty v-show="!hasPaymentData" description="暂无支付数据" :image-size="80" />
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="12">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header">
              <span>脚本运行结果</span>
              <el-tag effect="plain">成功率 {{ formatPercent(scriptTasks.success, scriptTasks.total) }}%</el-tag>
            </div>
          </template>
          <div ref="taskChartRef" class="chart"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="chart-row" v-show="showOverviewCharts">
      <el-col :xs="24" :lg="14">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header">
              <span>系统模块状态</span>
              <el-tag effect="plain">插件 / 机器人</el-tag>
            </div>
          </template>
          <div ref="systemChartRef" class="chart"></div>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="10">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header">
              <span>图床类型分布</span>
              <el-tag effect="plain">{{ formatCompact(images.total_assets) }} 张</el-tag>
            </div>
          </template>
          <div ref="imageTypeChartRef" class="chart" v-show="hasImageData"></div>
          <el-empty v-show="!hasImageData" description="暂无图片数据" :image-size="80" />
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="chart-row" v-show="showTrendCharts">
      <el-col :xs="24">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header trend-header">
              <span>消息总量趋势</span>
              <div class="trend-controls">
                <el-button size="small" plain :loading="messageTrendLoading" @click="loadMessageTrend">刷新</el-button>
                <el-radio-group v-model="messageTrendGranularity" size="small" @change="handleGranularityChange">
                  <el-radio-button label="day">按日</el-radio-button>
                  <el-radio-button label="month">按月</el-radio-button>
                </el-radio-group>
                <el-date-picker
                  v-if="messageTrendGranularity === 'day'"
                  v-model="messageTrendRange"
                  type="daterange"
                  size="small"
                  value-format="YYYY-MM-DD"
                  range-separator="至"
                  start-placeholder="开始日期"
                  end-placeholder="结束日期"
                  :disabled-date="disableFutureDate"
                  @change="loadMessageTrend"
                />
                <el-date-picker
                  v-else
                  v-model="messageTrendRange"
                  type="monthrange"
                  size="small"
                  value-format="YYYY-MM"
                  range-separator="至"
                  start-placeholder="开始月份"
                  end-placeholder="结束月份"
                  :disabled-date="disableFutureDate"
                  @change="loadMessageTrend"
                />
              </div>
            </div>
          </template>
          <div ref="messageTrendChartRef" class="chart message-trend-chart"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="chart-row" v-show="showTrendCharts">
      <el-col :xs="24">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header trend-header">
              <span>插件触发趋势</span>
              <div class="trend-controls">
                <el-tag effect="plain">总触发 {{ formatCompact(pluginTriggerTrend.total) }} 次</el-tag>
                <el-button size="small" plain :loading="pluginTrendLoading" @click="loadPluginTriggerTrend">刷新</el-button>
                <el-radio-group v-model="pluginTrendGranularity" size="small" @change="handlePluginTrendGranularityChange">
                  <el-radio-button label="day">按日</el-radio-button>
                  <el-radio-button label="month">按月</el-radio-button>
                </el-radio-group>
                <el-date-picker
                  v-if="pluginTrendGranularity === 'day'"
                  v-model="pluginTrendRange"
                  type="daterange"
                  size="small"
                  value-format="YYYY-MM-DD"
                  range-separator="至"
                  start-placeholder="开始日期"
                  end-placeholder="结束日期"
                  :disabled-date="disableFutureDate"
                  @change="loadPluginTriggerTrend"
                />
                <el-date-picker
                  v-else
                  v-model="pluginTrendRange"
                  type="monthrange"
                  size="small"
                  value-format="YYYY-MM"
                  range-separator="至"
                  start-placeholder="开始月份"
                  end-placeholder="结束月份"
                  :disabled-date="disableFutureDate"
                  @change="loadPluginTriggerTrend"
                />
              </div>
            </div>
          </template>
          <div ref="pluginTriggerChartRef" class="chart plugin-trigger-chart"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="chart-row" v-show="showStorageChartGroup">
      <el-col :xs="24">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header">
              <span>存储占用对比</span>
              <el-tag effect="plain">图片 / 备份 / 日志</el-tag>
            </div>
          </template>
          <div ref="storageChartRef" class="chart storage-chart" v-show="hasStorageData"></div>
          <el-empty v-show="!hasStorageData" description="暂无存储数据" :image-size="80" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { BarChart, FunnelChart, GaugeChart, LineChart, PieChart, RadarChart, TreemapChart } from 'echarts/charts'
import { GridComponent, LegendComponent, RadarComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { ElMessage } from 'element-plus'
import { getMessageTotalTrend, getPluginTriggerTrend, getStatisticsOverview } from '@/api'

echarts.use([BarChart, FunnelChart, GaugeChart, LineChart, PieChart, RadarChart, TreemapChart, GridComponent, LegendComponent, RadarComponent, TooltipComponent, CanvasRenderer])

const loading = ref(false)
const overview = ref(createEmptyOverview())
const messageTrend = ref({ labels: [], totals: [], privateTotals: [], groupTotals: [], points: [] })
const messageTrendGranularity = ref('day')
const messageTrendRange = ref(defaultDayRange())
const pluginTriggerTrend = ref({ labels: [], plugins: [], total: 0, points: [] })
const pluginTrendGranularity = ref('day')
const pluginTrendRange = ref(defaultDayRange())
const paymentChartRef = ref(null)
const taskChartRef = ref(null)
const systemChartRef = ref(null)
const imageTypeChartRef = ref(null)
const messageTrendChartRef = ref(null)
const pluginTriggerChartRef = ref(null)
const storageChartRef = ref(null)
let paymentChart = null
let taskChart = null
let systemChart = null
let imageTypeChart = null
let messageTrendChart = null
let pluginTriggerChart = null
let storageChart = null
let resizeObserver = null

const messageTrendLoading = ref(false)
const pluginTrendLoading = ref(false)
const isMobile = ref(typeof window !== 'undefined' ? window.matchMedia('(max-width: 768px)').matches : false)
const mobileTab = ref('overview')

const system = computed(() => overview.value.system || {})
const payments = computed(() => overview.value.payments || {})
const images = computed(() => overview.value.images || {})
const scriptTasks = computed(() => overview.value.script_tasks || {})
const backups = computed(() => overview.value.backups || {})
const logs = computed(() => overview.value.logs || {})

const overviewItems = computed(() => [
  { title: '脚本运行', value: formatCompact(scriptTasks.value.total), desc: `失败 ${formatCompact(scriptTasks.value.failed)} 次，今日 ${formatCompact(scriptTasks.value.today)} 次` },
  { title: '图床资产', value: formatCompact(images.value.total_assets), desc: formatSize(images.value.total_size_bytes) },
  { title: '备份文件', value: formatCompact(backups.value.file_count), desc: formatSize(backups.value.total_size_bytes) },
  { title: '插件启用率', value: `${formatPercent(system.value.enabled_plugin_count, system.value.plugin_count)}%`, desc: `${system.value.enabled_plugin_count || 0}/${system.value.plugin_count || 0} 已启用` }
])

const hasPaymentData = computed(() => {
  const p = payments.value
  return Number(p.paid_orders || 0) + Number(p.pending_orders || 0) + Number(p.failed_orders || 0) + Number(p.expired_orders || 0) + Number(p.cancelled_orders || 0) > 0
})
const hasImageData = computed(() => (images.value.by_content_type || []).some(item => Number(item.count || 0) > 0))
const hasStorageData = computed(() => Number(images.value.total_size_bytes || 0) > 0 || Number(backups.value.total_size_bytes || 0) > 0 || Number(logs.value.total_size_bytes || 0) > 0)
const showOverviewCharts = computed(() => !isMobile.value || mobileTab.value === 'overview')
const showTrendCharts = computed(() => !isMobile.value || mobileTab.value === 'trend')
const showStorageChartGroup = computed(() => !isMobile.value || mobileTab.value === 'storage')

const loadAll = async () => {
  loading.value = true
  try {
    const [overviewData, trendData, pluginTrendData] = await Promise.all([getStatisticsOverview(), fetchMessageTrend(), fetchPluginTriggerTrend()])
    overview.value = normalizeOverview(overviewData)
    messageTrend.value = normalizeMessageTrend(trendData)
    pluginTriggerTrend.value = normalizePluginTriggerTrend(pluginTrendData)
    await nextTick()
    renderCharts()
  } finally {
    loading.value = false
  }
}

const loadMessageTrend = async () => {
  if (!isValidTrendRange()) return
  messageTrendLoading.value = true
  try {
    messageTrend.value = normalizeMessageTrend(await fetchMessageTrend())
    await nextTick()
    renderMessageTrendChart()
  } catch (error) {
    ElMessage.error(error?.message || '获取消息总量趋势失败')
  } finally {
    messageTrendLoading.value = false
  }
}

function fetchMessageTrend() {
  const range = Array.isArray(messageTrendRange.value) ? messageTrendRange.value : []
  return getMessageTotalTrend({ granularity: messageTrendGranularity.value, start: range[0] || '', end: range[1] || '' })
}

const loadPluginTriggerTrend = async () => {
  if (!isValidPluginTrendRange()) return
  pluginTrendLoading.value = true
  try {
    pluginTriggerTrend.value = normalizePluginTriggerTrend(await fetchPluginTriggerTrend())
    await nextTick()
    renderPluginTriggerChart()
  } catch (error) {
    ElMessage.error(error?.message || '获取插件触发趋势失败')
  } finally {
    pluginTrendLoading.value = false
  }
}

function fetchPluginTriggerTrend() {
  const range = Array.isArray(pluginTrendRange.value) ? pluginTrendRange.value : []
  return getPluginTriggerTrend({ granularity: pluginTrendGranularity.value, start: range[0] || '', end: range[1] || '', limit: 8 })
}

function handleGranularityChange() {
  messageTrendRange.value = messageTrendGranularity.value === 'day' ? defaultDayRange() : defaultMonthRange()
  loadMessageTrend()
}

function handlePluginTrendGranularityChange() {
  pluginTrendRange.value = pluginTrendGranularity.value === 'day' ? defaultDayRange() : defaultMonthRange()
  loadPluginTriggerTrend()
}

function isValidTrendRange() {
  return isValidRange(messageTrendRange, messageTrendGranularity)
}

function isValidPluginTrendRange() {
  return isValidRange(pluginTrendRange, pluginTrendGranularity)
}

function isValidRange(rangeRef, granularityRef) {
  const range = Array.isArray(rangeRef.value) ? rangeRef.value : []
  if (range.length !== 2 || !range[0] || !range[1]) return false
  const maxLength = granularityRef.value === 'day' ? 15 : 12
  const actualLength = granularityRef.value === 'day' ? getDayRangeLength(range[0], range[1]) : getMonthRangeLength(range[0], range[1])
  if (actualLength > maxLength) {
    ElMessage.warning(granularityRef.value === 'day' ? '按日统计最多选择 15 天' : '按月统计最多选择 12 个月')
    rangeRef.value = granularityRef.value === 'day' ? defaultDayRange() : defaultMonthRange()
    return false
  }
  return true
}

function renderCharts() {
  renderPaymentChart()
  renderTaskChart()
  renderSystemChart()
  renderImageTypeChart()
  renderMessageTrendChart()
  renderPluginTriggerChart()
  renderStorageChart()
}

function renderPaymentChart() {
  if (!paymentChartRef.value) return
  if (!paymentChart) paymentChart = echarts.init(paymentChartRef.value)
  paymentChart.resize()
  const data = normalizePieData([
    { name: '已支付', value: Number(payments.value.paid_orders || 0) },
    { name: '待支付', value: Number(payments.value.pending_orders || 0) },
    { name: '失败', value: Number(payments.value.failed_orders || 0) },
    { name: '已过期', value: Number(payments.value.expired_orders || 0) },
    { name: '已取消', value: Number(payments.value.cancelled_orders || 0) }
  ])
  paymentChart.setOption({
    color: ['#22c55e', '#f59e0b', '#ef4444', '#64748b', '#94a3b8'],
    tooltip: { trigger: 'item', formatter: '{b}<br/>{c} 单 ({d}%)' },
    legend: { bottom: 0, type: 'scroll' },
    series: [{ name: '支付订单', type: 'pie', radius: ['48%', '70%'], center: ['50%', '43%'], avoidLabelOverlap: true, itemStyle: { borderRadius: 8, borderColor: '#fff', borderWidth: 2 }, label: { formatter: '{b}\n{d}%' }, data }]
  }, true)
}

function renderTaskChart() {
  if (!taskChartRef.value) return
  if (!taskChart) taskChart = echarts.init(taskChartRef.value)
  taskChart.resize()
  const successRate = formatPercent(scriptTasks.value.success, scriptTasks.value.total)
  const solidColor = successRate >= 90 ? '#22c55e' : successRate >= 70 ? '#f59e0b' : '#ef4444'
  taskChart.setOption({
    tooltip: { formatter: `脚本成功率<br/>${successRate}%<br/>失败 ${formatCompact(scriptTasks.value.failed)} / 总计 ${formatCompact(scriptTasks.value.total)}` },
    series: [
      { type: 'gauge', radius: '90%', center: ['50%', '58%'], startAngle: 200, endAngle: -20, min: 0, max: 100, axisLine: { lineStyle: { width: 18, color: [[0.5, '#ef4444'], [0.7, '#f59e0b'], [0.9, '#84cc16'], [1, '#22c55e']] } }, progress: { show: false }, pointer: { show: true, icon: 'path://M1,0 L0.15,-0.3 L0,-0.5 L0,0.5 L0.15,0.3 Z', length: '68%', width: 6, itemStyle: { color: '#475569' } }, anchor: { show: true, size: 14, itemStyle: { color: '#475569', borderColor: '#fff', borderWidth: 3 } }, axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false }, detail: { valueAnimation: true, formatter: '{value}%', color: solidColor, fontSize: 38, fontWeight: 800, offsetCenter: [0, '16%'] }, title: { offsetCenter: [0, '44%'], color: '#64748b', fontSize: 13 }, data: [{ value: successRate, name: `成功 ${formatCompact(scriptTasks.value.success)} / 总计 ${formatCompact(scriptTasks.value.total)}` }] }
    ]
  }, true)
}

function renderSystemChart() {
  if (!systemChartRef.value) return
  if (!systemChart) systemChart = echarts.init(systemChartRef.value)
  const pluginRate = formatPercent(system.value.enabled_plugin_count, system.value.plugin_count)
  const adapterRate = formatPercent(system.value.running_adapter_count, system.value.adapter_count)
  const paymentRate = formatPercent(payments.value.paid_orders, payments.value.total_orders)
  const scriptRate = formatPercent(scriptTasks.value.success, scriptTasks.value.total)
  const backupRate = backups.value.enabled ? 100 : 0
  const radarItems = [
    { name: '插件', value: pluginRate },
    { name: '机器人', value: adapterRate },
    { name: '支付', value: paymentRate },
    { name: '脚本', value: scriptRate },
    { name: '备份', value: backupRate }
  ]
  systemChart.setOption({
    color: ['#6366f1', '#22c55e'],
    tooltip: { trigger: 'item', formatter: () => radarItems.map(item => `${item.name}: ${item.value}%`).join('<br/>') },
    legend: { bottom: 0 },
    radar: { center: ['50%', '47%'], radius: '68%', splitNumber: 4, axisName: { color: '#475569', fontSize: 12, formatter: name => `${name}\n${radarItems.find(item => item.name === name)?.value || 0}%` }, splitLine: { lineStyle: { color: ['#e2e8f0'] } }, splitArea: { areaStyle: { color: ['rgba(99, 102, 241, 0.04)', 'rgba(20, 184, 166, 0.04)'] } }, axisLine: { lineStyle: { color: '#cbd5e1' } }, indicator: radarItems.map(item => ({ name: item.name, max: 100 })) },
    series: [{ name: '系统健康度', type: 'radar', areaStyle: { opacity: 0.18 }, lineStyle: { width: 3 }, symbolSize: 7, label: { show: true, formatter: params => `${params.value}%`, color: '#6366f1', fontSize: 11 }, data: [{ name: '当前状态', value: radarItems.map(item => item.value) }] }]
  }, true)
}

function renderImageTypeChart() {
  if (!imageTypeChartRef.value) return
  if (!imageTypeChart) imageTypeChart = echarts.init(imageTypeChartRef.value)
  imageTypeChart.resize()
  const data = normalizePieData((images.value.by_content_type || []).map(item => ({ name: normalizeContentType(item.name), value: Number(item.count || 0), sizeBytes: Number(item.size_bytes || 0) })))
  imageTypeChart.setOption({
    color: ['#8b5cf6', '#06b6d4', '#ec4899', '#22c55e', '#f59e0b', '#64748b'],
    tooltip: { trigger: 'item', formatter: params => `${params.name}<br/>${params.value} 张 (${params.percent}%)${params.data.sizeBytes ? `<br/>${formatSize(params.data.sizeBytes)}` : ''}` },
    legend: { bottom: 0, type: 'scroll' },
    series: [{ name: '图床类型', type: 'pie', roseType: 'radius', radius: ['18%', '72%'], center: ['50%', '43%'], itemStyle: { borderRadius: 8, borderColor: '#fff', borderWidth: 2 }, label: { formatter: '{b}\n{c}' }, data }]
  }, true)
}

function renderMessageTrendChart() {
  if (!messageTrendChartRef.value) return
  if (!messageTrendChart) messageTrendChart = echarts.init(messageTrendChartRef.value)
  const labels = messageTrend.value.labels || []
  const totals = messageTrend.value.totals || []
  const privateTotals = messageTrend.value.privateTotals || []
  const groupTotals = messageTrend.value.groupTotals || []
  const barWidth = messageTrendGranularity.value === 'day' ? 26 : 34
  messageTrendChart.setOption({
    color: ['#6366f1', '#14b8a6'],
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: params => {
        const index = params?.[0]?.dataIndex || 0
        const label = params?.[0]?.axisValue || ''
        const lines = [`${label}<br/>合计 ${formatCompact(totals[index] || 0)} 条`]
        params.forEach(item => lines.push(`${item.marker}${item.seriesName}: ${formatCompact(item.value)} 条`))
        return lines.join('<br/>')
      }
    },
    legend: { bottom: 0 },
    grid: { left: 54, right: 24, top: 28, bottom: 64 },
    xAxis: { type: 'category', data: labels, axisTick: { show: false }, axisLine: { lineStyle: { color: '#cbd5e1' } }, axisLabel: { rotate: labels.length > 8 ? 30 : 0, color: '#64748b' } },
    yAxis: { type: 'value', minInterval: 1, name: '消息数', splitLine: { lineStyle: { type: 'dashed', color: '#e2e8f0' } } },
    series: [
      { name: '私聊', type: 'bar', stack: 'messages', barWidth, showBackground: true, backgroundStyle: { color: 'rgba(148, 163, 184, 0.12)', borderRadius: [10, 10, 0, 0] }, itemStyle: { color: barVerticalGradient('#a5b4fc', '#6366f1') }, emphasis: { itemStyle: { color: barVerticalGradient('#c7d2fe', '#4f46e5') } }, data: privateTotals },
      { name: '群聊', type: 'bar', stack: 'messages', barWidth, itemStyle: { borderRadius: [10, 10, 0, 0], color: barVerticalGradient('#5eead4', '#0f766e') }, emphasis: { itemStyle: { color: barVerticalGradient('#99f6e4', '#0d9488') } }, data: groupTotals }
    ]
  }, true)
}

function renderPluginTriggerChart() {
  if (!pluginTriggerChartRef.value) return
  if (!pluginTriggerChart) pluginTriggerChart = echarts.init(pluginTriggerChartRef.value)
  const labels = pluginTriggerTrend.value.labels || []
  const plugins = pluginTriggerTrend.value.plugins || []
  const points = pluginTriggerTrend.value.points || []
  const pointTotals = new Map(points.map(item => [item.label, Number(item.total || 0)]))
  const palette = ['#6366f1', '#14b8a6', '#8b5cf6', '#f97316', '#ec4899', '#22c55e', '#06b6d4', '#f59e0b', '#64748b', '#818cf8', '#84cc16', '#ef4444']
  pluginTriggerChart.setOption({
    color: palette,
    tooltip: {
      trigger: 'axis',
      formatter: params => {
        const label = params?.[0]?.axisValue || ''
        const lines = [`${label}<br/>合计 ${formatCompact(pointTotals.get(label) || 0)} 次`]
        params.forEach(item => {
          const plugin = plugins.find(row => row.plugin_id === item.seriesName)
          const displayName = plugin?.plugin_name && plugin.plugin_name !== item.seriesName ? `（${plugin.plugin_name}）` : ''
          lines.push(`${item.marker}${item.seriesName}${displayName}: ${formatCompact(item.value)} 次`)
        })
        return lines.join('<br/>')
      }
    },
    legend: { type: 'scroll', bottom: 0 },
    grid: { left: 54, right: 24, top: 30, bottom: 64 },
    xAxis: { type: 'category', boundaryGap: false, data: labels, axisTick: { show: false }, axisLine: { lineStyle: { color: '#cbd5e1' } }, axisLabel: { rotate: labels.length > 8 ? 30 : 0, color: '#64748b' } },
    yAxis: { type: 'value', minInterval: 1, name: '触发次数', splitLine: { lineStyle: { type: 'dashed', color: '#e2e8f0' } } },
    series: plugins.length > 0 ? plugins.map((plugin, index) => ({
      name: plugin.plugin_id,
      type: 'line',
      stack: 'pluginTriggers',
      smooth: true,
      symbolSize: 6,
      lineStyle: { width: 2 },
      areaStyle: { opacity: 0.16, color: barVerticalGradient(palette[index % palette.length], 'rgba(255,255,255,0.08)') },
      emphasis: { focus: 'series' },
      data: Array.isArray(plugin.counts) ? plugin.counts : []
    })) : [{ name: '暂无数据', type: 'line', smooth: true, areaStyle: { opacity: 0.12 }, data: labels.map(() => 0) }]
  }, true)
}

function renderStorageChart() {
  if (!storageChartRef.value) return
  if (!storageChart) storageChart = echarts.init(storageChartRef.value)
  storageChart.resize()
  const storageItems = [
    { name: '图床图片', realBytes: Number(images.value.total_size_bytes || 0), itemStyle: { color: '#14b8a6' } },
    { name: '备份文件', realBytes: Number(backups.value.total_size_bytes || 0), itemStyle: { color: '#6366f1' } },
    { name: '日志文件', realBytes: Number(logs.value.total_size_bytes || 0), itemStyle: { color: '#f59e0b' } }
  ].filter(item => item.realBytes > 0)
  const total = storageItems.reduce((sum, item) => sum + item.realBytes, 0)
  const data = total > 0 ? normalizeStorageTreemapItems(storageItems, total) : [{ name: '暂无数据', value: 1, realBytes: 0, itemStyle: { color: '#e2e8f0' } }]
  storageChart.setOption({
    tooltip: {
      formatter: params => {
        if (total <= 0) return '暂无数据'
        const bytes = params.data.realBytes
        const pct = (bytes / total * 100).toFixed(1)
        return `${params.name}<br/>${formatSize(bytes)} (${pct}%)`
      }
    },
    series: [{
      type: 'treemap',
      data,
      roam: false,
      nodeClick: false,
      breadcrumb: { show: false },
      top: 4,
      bottom: 4,
      left: 4,
      right: 4,
      label: {
        show: true,
        formatter: params => {
          if (total <= 0) return '暂无数据'
          const bytes = params.data.realBytes
          const pct = (bytes / total * 100).toFixed(1)
          return `${params.name}\n${formatSize(bytes)} · ${pct}%`
        },
        color: '#fff',
        fontSize: 14,
        fontWeight: 600,
        lineHeight: 20,
        overflow: 'truncate'
      },
      upperLabel: { show: false },
      itemStyle: { borderColor: '#fff', borderWidth: 3, gapWidth: 3 }
    }]
  }, true)
}

function normalizeOverview(data) {
  return { ...createEmptyOverview(), ...(data || {}) }
}

function normalizeStorageTreemapItems(items, total) {
  const minRatio = 0.08
  const minValue = total * minRatio
  const smallItems = items.filter(item => item.realBytes / total < minRatio)
  const largeItems = items.filter(item => item.realBytes / total >= minRatio)
  const reserved = smallItems.length * minValue
  const largeTotal = largeItems.reduce((sum, item) => sum + item.realBytes, 0)
  return items.map(item => {
    const isSmall = item.realBytes / total < minRatio
    const value = isSmall || largeTotal <= 0 ? minValue : item.realBytes / largeTotal * Math.max(total - reserved, minValue)
    return { ...item, value }
  })
}

function normalizeMessageTrend(data) {
  const labels = Array.isArray(data?.labels) ? data.labels : []
  const totals = alignNumberArray(data?.totals, labels.length)
  const privateTotals = Array.isArray(data?.private_totals) ? alignNumberArray(data.private_totals, labels.length) : totals.slice()
  const groupTotals = Array.isArray(data?.group_totals) ? alignNumberArray(data.group_totals, labels.length) : labels.map(() => 0)
  return { labels, totals, privateTotals, groupTotals, points: Array.isArray(data?.points) ? data.points : [] }
}

function alignNumberArray(value, length) {
  const source = Array.isArray(value) ? value : []
  return Array.from({ length }, (_, index) => Number(source[index] || 0))
}

function normalizePluginTriggerTrend(data) {
  return {
    labels: Array.isArray(data?.labels) ? data.labels : [],
    plugins: Array.isArray(data?.plugins) ? data.plugins : [],
    total: Number(data?.total || 0),
    points: Array.isArray(data?.points) ? data.points : []
  }
}

function createEmptyOverview() {
  return { system: {}, payments: {}, images: { by_content_type: [] }, script_tasks: {}, backups: {}, logs: {} }
}

function normalizePieData(data) {
  const valid = data.filter(item => Number(item.value || 0) > 0)
  return valid.length > 0 ? valid : [{ name: '暂无数据', value: 0 }]
}

function normalizeContentType(type) {
  const value = String(type || '未知类型')
  return value.replace('image/', '').toUpperCase()
}

function defaultDayRange() {
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - 6)
  return [formatDate(start), formatDate(end)]
}

function defaultMonthRange() {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth() - 5, 1)
  return [formatMonth(start), formatMonth(end)]
}

function getDayRangeLength(start, end) {
  return Math.floor((new Date(end).getTime() - new Date(start).getTime()) / 86400000) + 1
}

function getMonthRangeLength(start, end) {
  const [startYear, startMonth] = start.split('-').map(Number)
  const [endYear, endMonth] = end.split('-').map(Number)
  return (endYear - startYear) * 12 + endMonth - startMonth + 1
}

function disableFutureDate(date) {
  return date.getTime() > Date.now()
}

function formatCompact(value) {
  const count = Number(value || 0)
  if (count < 1000) return String(count)
  if (count < 10000) return `${(count / 1000).toFixed(2)}k`
  return `${(count / 10000).toFixed(2)}w`
}

function formatPercent(value, total) {
  const numerator = Number(value || 0)
  const denominator = Number(total || 0)
  if (denominator <= 0) return 0
  return Math.min(100, Math.round((numerator / denominator) * 100))
}

function formatMoney(cents) {
  return `¥${(Number(cents || 0) / 100).toFixed(2)}`
}

function formatSize(size) {
  const value = Number(size || 0)
  if (value <= 0) return '0 B'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(2)} MB`
  return `${(value / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function formatDate(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatMonth(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  return `${year}-${month}`
}

function barVerticalGradient(from, to) {
  return { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: from }, { offset: 1, color: to }] }
}

function resizeCharts() {
  isMobile.value = typeof window !== 'undefined' ? window.matchMedia('(max-width: 768px)').matches : false
  ;[paymentChart, taskChart, systemChart, imageTypeChart, messageTrendChart, pluginTriggerChart, storageChart].forEach(chart => chart?.resize())
}

watch(mobileTab, () => {
  nextTick(() => resizeCharts())
})

onMounted(() => {
  loadAll()
  window.addEventListener('resize', resizeCharts)
  resizeObserver = new ResizeObserver(() => resizeCharts())
  const pageEl = document.querySelector('.statistics-page')
  if (pageEl) resizeObserver.observe(pageEl)
})

onUnmounted(() => {
  window.removeEventListener('resize', resizeCharts)
  resizeObserver?.disconnect()
  resizeObserver = null
  ;[paymentChart, taskChart, systemChart, imageTypeChart, messageTrendChart, pluginTriggerChart, storageChart].forEach(chart => chart?.dispose())
  paymentChart = null
  taskChart = null
  systemChart = null
  imageTypeChart = null
  messageTrendChart = null
  pluginTriggerChart = null
  storageChart = null
})
</script>

<style scoped>
.statistics-page { width: 100%; }
.statistics-toolbar { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
.statistics-toolbar h2 { margin: 0; color: var(--text-primary); font-size: 22px; font-weight: 700; font-family: var(--font-heading); }
.statistics-toolbar p { margin: 6px 0 0; color: var(--text-tertiary); font-size: 13px; }
.business-overview { display: grid; grid-template-columns: 340px 1fr; gap: 16px; margin-bottom: 16px; }
.overview-hero { min-height: 172px; padding: 24px; display: flex; flex-direction: column; justify-content: space-between; color: #fff; border-radius: var(--radius-lg); background: linear-gradient(135deg, #1e1b4b 0%, var(--brand-600) 55%, var(--brand-400) 100%); box-shadow: var(--shadow-lg), 0 0 32px rgba(99, 102, 241, 0.12); box-sizing: border-box; }
.hero-label { font-size: 13px; opacity: 0.82; }
.hero-value { margin-top: 16px; font-size: 42px; font-weight: 800; font-family: var(--font-heading); line-height: 1; letter-spacing: -1px; }
.hero-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 18px; font-size: 12px; opacity: 0.9; }
.overview-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.overview-item { min-height: 172px; padding: 18px; display: flex; flex-direction: column; justify-content: center; border-radius: var(--radius-lg); background: var(--bg-surface); border: 1px solid var(--border-subtle); box-shadow: var(--shadow-sm); box-sizing: border-box; transition: box-shadow var(--transition-normal); }
.overview-item:hover { box-shadow: var(--shadow-md); }
.overview-item-title { color: var(--text-secondary); font-size: 13px; }
.overview-item-value { margin-top: 10px; color: var(--text-primary); font-size: 28px; font-weight: 800; font-family: var(--font-heading); line-height: 1.1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; letter-spacing: -0.02em; }
.overview-item-desc { margin-top: 8px; color: var(--text-tertiary); font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.chart-row { margin-top: 16px; row-gap: 16px; }
.chart-card { border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); }
.chart-card-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-weight: 600; }
.trend-header { align-items: flex-start; }
.trend-controls { display: flex; align-items: center; justify-content: flex-end; gap: 10px; flex-wrap: wrap; }
.chart { width: 100%; height: 360px; }
.message-trend-chart { height: 340px; }
.plugin-trigger-chart { height: 380px; }
.storage-chart { height: 220px; }
.mobile-chart-tabs { display: none; }
@media (max-width: 1200px) {
  .business-overview { grid-template-columns: 1fr; }
  .overview-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 768px) {
  .statistics-page { height: calc(100dvh - 52px - 76px - 24px); overflow-y: auto; overflow-x: hidden; padding-right: 2px; }
  .stats-sticky-header {
    position: sticky;
    top: 0;
    z-index: 100;
    background: var(--bg-surface, #fff);
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border-subtle, #ebeef5);
  }
  .stats-sticky-header .statistics-toolbar { flex-direction: row; align-items: center; margin-bottom: 10px; }
  .stats-sticky-header .statistics-toolbar p { display: none; }
  .stats-sticky-header .statistics-toolbar .el-button { width: auto; flex-shrink: 0; }
  .stats-sticky-header .mobile-chart-tabs { margin-bottom: 0; }
  .business-overview { gap: 8px; margin-bottom: 12px; }
  .overview-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .overview-hero { min-height: 96px; padding: 14px; }
  .overview-item { min-height: 88px; padding: 12px; }
  .hero-value { margin-top: 6px; font-size: 26px; }
  .hero-footer { flex-direction: row; align-items: center; gap: 12px; margin-top: 8px; }
  .overview-item-value { margin-top: 6px; font-size: 22px; }
  .overview-item-desc { margin-top: 4px; }
  .trend-header { flex-direction: column; }
  .trend-controls, .trend-controls :deep(.el-date-editor) { width: 100%; }
  .mobile-chart-tabs {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
  }
  .mobile-chart-tabs button {
    flex: 1;
    padding: 8px 0;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    color: var(--text-secondary);
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all var(--transition-normal);
  }
  .mobile-chart-tabs button.active {
    background: var(--brand-600);
    border-color: var(--brand-600);
    color: #fff;
  }
  .chart { height: 260px; }
  .message-trend-chart { height: 320px; }
  .plugin-trigger-chart { height: 320px; }
  .storage-chart { height: 240px; }
}
</style>
