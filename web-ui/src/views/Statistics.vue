<template>
  <div class="statistics-page page-shell" v-loading="loading">
    <div class="statistics-toolbar">
      <div>
        <h2>数据统计</h2>
        <p>聚焦支付、脚本、图床、备份和系统资源，消息总量只展示跨天/跨月趋势。</p>
      </div>
      <el-button type="primary" :loading="loading" @click="loadAll">刷新</el-button>
    </div>

    <section class="business-overview">
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

    <el-row :gutter="16" class="chart-row">
      <el-col :xs="24" :lg="12">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header">
              <span>支付订单状态</span>
              <el-tag effect="plain">{{ formatCompact(payments.total_orders) }} 单</el-tag>
            </div>
          </template>
          <div ref="paymentChartRef" class="chart"></div>
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

    <el-row :gutter="16" class="chart-row">
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
          <div ref="imageTypeChartRef" class="chart"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="chart-row">
      <el-col :xs="24">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header trend-header">
              <span>消息总量趋势</span>
              <div class="trend-controls">
                <el-button size="small" plain @click="loadMessageTrend">刷新</el-button>
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

    <el-row :gutter="16" class="chart-row">
      <el-col :xs="24">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header trend-header">
              <span>插件触发趋势</span>
              <div class="trend-controls">
                <el-tag effect="plain">总触发 {{ formatCompact(pluginTriggerTrend.total) }} 次</el-tag>
                <el-button size="small" plain @click="loadPluginTriggerTrend">刷新</el-button>
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

    <el-row :gutter="16" class="chart-row">
      <el-col :xs="24">
        <el-card class="chart-card" shadow="never">
          <template #header>
            <div class="chart-card-header">
              <span>存储占用对比</span>
              <el-tag effect="plain">图片 / 备份</el-tag>
            </div>
          </template>
          <div ref="storageChartRef" class="chart storage-chart"></div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import * as echarts from 'echarts/core'
import { BarChart, FunnelChart, GaugeChart, LineChart, PieChart, RadarChart, TreemapChart } from 'echarts/charts'
import { GridComponent, LegendComponent, RadarComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { ElMessage } from 'element-plus'
import { getMessageTotalTrend, getPluginTriggerTrend, getStatisticsOverview } from '@/api'

echarts.use([BarChart, FunnelChart, GaugeChart, LineChart, PieChart, RadarChart, TreemapChart, GridComponent, LegendComponent, RadarComponent, TooltipComponent, CanvasRenderer])

const loading = ref(false)
const overview = ref(createEmptyOverview())
const messageTrend = ref({ labels: [], totals: [] })
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

const system = computed(() => overview.value.system || {})
const payments = computed(() => overview.value.payments || {})
const images = computed(() => overview.value.images || {})
const scriptTasks = computed(() => overview.value.script_tasks || {})
const backups = computed(() => overview.value.backups || {})

const overviewItems = computed(() => [
  { title: '脚本运行', value: formatCompact(scriptTasks.value.total), desc: `今日 ${formatCompact(scriptTasks.value.today)} 次` },
  { title: '图床资产', value: formatCompact(images.value.total_assets), desc: formatSize(images.value.total_size_bytes) },
  { title: '备份文件', value: formatCompact(backups.value.file_count), desc: formatSize(backups.value.total_size_bytes) },
  { title: '插件启用率', value: `${formatPercent(system.value.enabled_plugin_count, system.value.plugin_count)}%`, desc: `${system.value.enabled_plugin_count || 0}/${system.value.plugin_count || 0} 已启用` }
])

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
  try {
    messageTrend.value = normalizeMessageTrend(await fetchMessageTrend())
    await nextTick()
    renderMessageTrendChart()
  } catch (error) {
    ElMessage.error(error?.message || '获取消息总量趋势失败')
  }
}

function fetchMessageTrend() {
  const range = Array.isArray(messageTrendRange.value) ? messageTrendRange.value : []
  return getMessageTotalTrend({ granularity: messageTrendGranularity.value, start: range[0] || '', end: range[1] || '' })
}

const loadPluginTriggerTrend = async () => {
  if (!isValidPluginTrendRange()) return
  try {
    pluginTriggerTrend.value = normalizePluginTriggerTrend(await fetchPluginTriggerTrend())
    await nextTick()
    renderPluginTriggerChart()
  } catch (error) {
    ElMessage.error(error?.message || '获取插件触发趋势失败')
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
  const successRate = formatPercent(scriptTasks.value.success, scriptTasks.value.total)
  taskChart.setOption({
    tooltip: { formatter: `脚本成功率<br/>${successRate}%` },
    series: [
      { type: 'gauge', radius: '88%', center: ['50%', '55%'], startAngle: 210, endAngle: -30, min: 0, max: 100, progress: { show: true, width: 18, itemStyle: { color: barGradient('#22d3ee', '#2563eb') } }, axisLine: { lineStyle: { width: 18, color: [[1, '#e2e8f0']] } }, pointer: { icon: 'roundRect', length: '58%', width: 8, itemStyle: { color: '#1f2937' } }, axisTick: { distance: -28, splitNumber: 2, lineStyle: { color: '#94a3b8', width: 1 } }, splitLine: { distance: -32, length: 12, lineStyle: { color: '#64748b', width: 2 } }, axisLabel: { distance: -10, color: '#64748b', fontSize: 11 }, detail: { valueAnimation: true, formatter: '{value}%', color: '#111827', fontSize: 34, fontWeight: 800, offsetCenter: [0, '34%'] }, title: { offsetCenter: [0, '58%'], color: '#64748b', fontSize: 13 }, data: [{ value: successRate, name: `成功 ${formatCompact(scriptTasks.value.success)} / 总计 ${formatCompact(scriptTasks.value.total)}` }] }
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
    color: ['#2563eb', '#22c55e'],
    tooltip: { trigger: 'item', formatter: () => radarItems.map(item => `${item.name}: ${item.value}%`).join('<br/>') },
    legend: { bottom: 0 },
    radar: { center: ['50%', '47%'], radius: '68%', splitNumber: 4, axisName: { color: '#475569', fontSize: 12, formatter: name => `${name}\n${radarItems.find(item => item.name === name)?.value || 0}%` }, splitLine: { lineStyle: { color: ['#e2e8f0'] } }, splitArea: { areaStyle: { color: ['rgba(37, 99, 235, 0.04)', 'rgba(20, 184, 166, 0.04)'] } }, axisLine: { lineStyle: { color: '#cbd5e1' } }, indicator: radarItems.map(item => ({ name: item.name, max: 100 })) },
    series: [{ name: '系统健康度', type: 'radar', areaStyle: { opacity: 0.18 }, lineStyle: { width: 3 }, symbolSize: 7, label: { show: true, formatter: params => `${params.value}%`, color: '#1d4ed8', fontSize: 11 }, data: [{ name: '当前状态', value: radarItems.map(item => item.value) }] }]
  }, true)
}

function renderImageTypeChart() {
  if (!imageTypeChartRef.value) return
  if (!imageTypeChart) imageTypeChart = echarts.init(imageTypeChartRef.value)
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
  messageTrendChart.setOption({
    color: ['#2563eb'],
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, formatter: params => `${params[0].name}<br/>${formatCompact(params[0].value)} 条` },
    grid: { left: 54, right: 24, top: 28, bottom: 42 },
    xAxis: { type: 'category', data: labels, axisTick: { show: false }, axisLine: { lineStyle: { color: '#cbd5e1' } }, axisLabel: { rotate: labels.length > 8 ? 30 : 0, color: '#64748b' } },
    yAxis: { type: 'value', minInterval: 1, name: '消息数', splitLine: { lineStyle: { type: 'dashed', color: '#e2e8f0' } } },
    series: [{ name: '消息总数', type: 'bar', barWidth: messageTrendGranularity.value === 'day' ? 26 : 34, showBackground: true, backgroundStyle: { color: 'rgba(148, 163, 184, 0.12)', borderRadius: [10, 10, 0, 0] }, itemStyle: { borderRadius: [10, 10, 0, 0], color: barVerticalGradient('#93c5fd', '#1d4ed8') }, emphasis: { itemStyle: { color: barVerticalGradient('#67e8f9', '#2563eb') } }, data: totals }]
  }, true)
}

function renderPluginTriggerChart() {
  if (!pluginTriggerChartRef.value) return
  if (!pluginTriggerChart) pluginTriggerChart = echarts.init(pluginTriggerChartRef.value)
  const labels = pluginTriggerTrend.value.labels || []
  const plugins = pluginTriggerTrend.value.plugins || []
  const points = pluginTriggerTrend.value.points || []
  const pointTotals = new Map(points.map(item => [item.label, Number(item.total || 0)]))
  const palette = ['#2563eb', '#14b8a6', '#8b5cf6', '#f97316', '#ec4899', '#22c55e', '#06b6d4', '#f59e0b', '#64748b', '#6366f1', '#84cc16', '#ef4444']
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
  const data = normalizeTreemapData([
    { name: '图床图片', value: Number(images.value.total_size_bytes || 0), itemStyle: { color: '#14b8a6' } },
    { name: '备份文件', value: Number(backups.value.total_size_bytes || 0), itemStyle: { color: '#3b82f6' } }
  ])
  storageChart.setOption({
    tooltip: { formatter: info => `${info.name}<br/>${formatSize(info.value)}` },
    series: [{ type: 'treemap', roam: false, nodeClick: false, breadcrumb: { show: false }, left: 8, right: 8, top: 8, bottom: 8, label: { show: true, formatter: params => `${params.name}\n${formatSize(params.value)}`, color: '#fff', fontSize: 16, fontWeight: 800 }, upperLabel: { show: false }, itemStyle: { borderColor: '#fff', borderWidth: 4, gapWidth: 4, borderRadius: 14 }, data }]
  }, true)
}

function normalizeOverview(data) {
  return { ...createEmptyOverview(), ...(data || {}) }
}

function normalizeMessageTrend(data) {
  return { labels: Array.isArray(data?.labels) ? data.labels : [], totals: Array.isArray(data?.totals) ? data.totals : [] }
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
  return { system: {}, payments: {}, images: { by_content_type: [] }, script_tasks: {}, backups: {} }
}

function normalizePieData(data) {
  const valid = data.filter(item => Number(item.value || 0) > 0)
  return valid.length > 0 ? valid : [{ name: '暂无数据', value: 0 }]
}

function normalizePositiveData(data) {
  const valid = data.filter(item => Number(item.value || 0) > 0)
  return valid.length > 0 ? valid : [{ name: '暂无数据', value: 1, itemStyle: { color: '#cbd5e1' } }]
}

function normalizeTreemapData(data) {
  const valid = data.filter(item => Number(item.value || 0) > 0)
  return valid.length > 0 ? valid : [{ name: '暂无数据', value: 1, itemStyle: { color: '#cbd5e1' } }]
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

function barGradient(from, to) {
  return { type: 'linear', x: 0, y: 0, x2: 1, y2: 0, colorStops: [{ offset: 0, color: from }, { offset: 1, color: to }] }
}

function barVerticalGradient(from, to) {
  return { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: from }, { offset: 1, color: to }] }
}

function resizeCharts() {
  ;[paymentChart, taskChart, systemChart, imageTypeChart, messageTrendChart, pluginTriggerChart, storageChart].forEach(chart => chart?.resize())
}

onMounted(() => {
  loadAll()
  window.addEventListener('resize', resizeCharts)
})

onUnmounted(() => {
  window.removeEventListener('resize', resizeCharts)
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
.statistics-toolbar h2 { margin: 0; color: #1f2937; font-size: 22px; }
.statistics-toolbar p { margin: 6px 0 0; color: #909399; font-size: 13px; }
.business-overview { display: grid; grid-template-columns: 340px 1fr; gap: 16px; margin-bottom: 16px; }
.overview-hero { min-height: 172px; padding: 24px; display: flex; flex-direction: column; justify-content: space-between; color: #fff; border-radius: 18px; background: linear-gradient(135deg, #111827 0%, #1d4ed8 58%, #06b6d4 100%); box-shadow: 0 16px 36px rgba(17, 24, 39, 0.18); box-sizing: border-box; }
.hero-label { font-size: 13px; opacity: 0.82; }
.hero-value { margin-top: 16px; font-size: 42px; font-weight: 800; line-height: 1; letter-spacing: -1px; }
.hero-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 18px; font-size: 12px; opacity: 0.9; }
.overview-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.overview-item { min-height: 172px; padding: 18px; display: flex; flex-direction: column; justify-content: center; border-radius: 16px; background: #fff; box-shadow: 0 10px 26px rgba(15, 23, 42, 0.04); box-sizing: border-box; }
.overview-item-title { color: #64748b; font-size: 13px; }
.overview-item-value { margin-top: 10px; color: #111827; font-size: 28px; font-weight: 760; line-height: 1.1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.overview-item-desc { margin-top: 8px; color: #94a3b8; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.chart-row { margin-top: 16px; row-gap: 16px; }
.chart-card { border: none; border-radius: 16px; }
.chart-card-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-weight: 600; }
.trend-header { align-items: flex-start; }
.trend-controls { display: flex; align-items: center; justify-content: flex-end; gap: 10px; flex-wrap: wrap; }
.chart { width: 100%; height: 360px; }
.message-trend-chart { height: 340px; }
.plugin-trigger-chart { height: 380px; }
.storage-chart { height: 300px; }
@media (max-width: 1200px) {
  .business-overview { grid-template-columns: 1fr; }
  .overview-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 768px) {
  .statistics-page { height: calc(100dvh - 52px - 76px - 24px); overflow-y: auto; overflow-x: hidden; padding-right: 2px; }
  .statistics-toolbar { flex-direction: column; }
  .statistics-toolbar .el-button { width: 100%; }
  .overview-strip { grid-template-columns: 1fr; }
  .overview-hero, .overview-item { min-height: 138px; }
  .hero-value { font-size: 34px; }
  .hero-footer { flex-direction: column; align-items: flex-start; }
  .trend-header { flex-direction: column; }
  .trend-controls, .trend-controls :deep(.el-date-editor) { width: 100%; }
  .chart, .message-trend-chart, .plugin-trigger-chart, .storage-chart { height: 320px; }
}
</style>
