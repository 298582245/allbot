<template>
  <div class="dashboard">
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="12" :sm="12" :md="6" v-for="stat in stats" :key="stat.title">
        <el-card class="stat-card">
          <div class="stat-icon" :style="{ '--stat-gradient': stat.color }">
            <component :is="stat.icon" />
          </div>
          <div class="stat-content">
            <el-tooltip :disabled="!stat.tooltip" :content="stat.tooltip" placement="top">
              <div class="stat-value">{{ stat.value }}</div>
            </el-tooltip>
            <div class="stat-title">{{ stat.title }}</div>
            <el-tooltip :disabled="!stat.subtextTooltip" :content="stat.subtextTooltip" placement="top">
              <div v-if="stat.subtext" class="stat-subtext">{{ stat.subtext }}</div>
            </el-tooltip>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="chart-row">
      <el-col :xs="24" :lg="12">
        <el-card class="dashboard-card">
          <template #header>
            <div class="card-header chart-header">
              <span>消息分布图</span>
              <div class="chart-filters">
                <el-radio-group v-model="chartDimension" size="small" @change="renderMessageChart">
                  <el-radio-button label="platform">不同平台</el-radio-button>
                  <el-radio-button label="adapter">不同机器人</el-radio-button>
                </el-radio-group>
                <el-select v-model="selectedSeriesName" size="small" class="series-select" @change="renderMessageChart">
                  <el-option label="全部显示" value="" />
                  <el-option
                    v-for="item in currentSeries"
                    :key="item.name"
                    :label="item.name"
                    :value="item.name"
                  />
                </el-select>
                <el-radio-group v-model="statsMode" size="small" @change="handleStatsModeChange">
                  <el-radio-button label="date">按日期</el-radio-button>
                  <el-radio-button label="total">总计</el-radio-button>
                </el-radio-group>
                <el-date-picker
                  v-model="statsDate"
                  type="date"
                  size="small"
                  value-format="YYYY-MM-DD"
                  :disabled="statsMode === 'total'"
                  @change="loadMessageStats"
                />
              </div>
            </div>
          </template>
          <div v-loading="messageStatsLoading" class="chart-wrap">
            <div ref="messageChartRef" class="message-chart"></div>
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="12">
        <el-card class="dashboard-card">
          <template #header>
            <div class="card-header">
              <span>系统资源占用</span>
              <span class="resource-hint">CPU / 内存实时概览</span>
            </div>
          </template>
          <div class="resource-grid">
            <div class="resource-panel">
              <div class="resource-title">CPU 占用</div>
              <div ref="cpuChartRef" class="resource-chart"></div>
              <div class="resource-value">{{ formatPercent(resourceStatus.allBotCpuUsagePercent) }}</div>
              <div class="resource-detail">平均占用：{{ formatPercent(resourceStatus.allBotAverageCpuUsagePercent) }}</div>
              <div class="resource-detail">本次运行最高：{{ formatPercent(resourceStatus.allBotPeakCpuUsagePercent) }}</div>
              <div class="resource-detail">历史最高：{{ formatPercent(resourceStatus.allBotHistoricalPeakCpuUsagePercent) }}</div>
              <div class="resource-detail">系统 CPU：{{ formatPercent(resourceStatus.cpuUsagePercent) }}</div>
            </div>
            <div class="resource-panel">
              <div class="resource-title">内存占用</div>
              <div ref="memoryChartRef" class="resource-chart"></div>
              <div class="resource-value">{{ formatUsageWithPercent(resourceStatus.allBotMemoryUsedBytes, resourceStatus.allBotMemoryTotalBytes) }}</div>
              <div class="resource-detail">平均占用：{{ formatUsageWithPercent(resourceStatus.allBotAverageMemoryUsedBytes, resourceStatus.allBotMemoryTotalBytes) }}</div>
              <div class="resource-detail">本次运行最高：{{ formatUsageWithPercent(resourceStatus.allBotPeakMemoryUsedBytes, resourceStatus.allBotMemoryTotalBytes) }}</div>
              <div class="resource-detail">历史最高：{{ formatHistoricalUsage(resourceStatus.allBotHistoricalPeakMemoryUsedBytes, resourceStatus.allBotHistoricalPeakMemoryUsagePercent) }}</div>
              <div class="resource-detail">系统内存：{{ formatBytes(resourceStatus.memoryUsedBytes) }} / {{ formatBytes(resourceStatus.memoryTotalBytes) }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

  </div>
</template>

<script setup>
defineOptions({ name: 'Dashboard' })
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import {
  TrendCharts,
  Grid as GridIcon,
  Connection as ConnectionIcon,
  ChatLineRound
} from '@element-plus/icons-vue'
import { getSystemStatus, getMessageStats } from '@/api'

echarts.use([LineChart, PieChart, GridComponent, LegendComponent, TitleComponent, TooltipComponent, CanvasRenderer])

const stats = ref([
  { title: '当前运行', value: '--', subtext: '', icon: TrendCharts, color: 'var(--brand-500)' },
  { title: '运行插件', value: 0, subtext: '共 0 个插件', icon: GridIcon, color: 'var(--color-success)' },
  { title: '在线机器人', value: 0, subtext: '共 0 个机器人', icon: ConnectionIcon, color: 'var(--brand-400)' },
  { title: '今日消息', value: 0, subtext: '累计 0 条', icon: ChatLineRound, color: 'var(--color-warning)' }
])

const statsMode = ref('date')
const statsDate = ref(formatDate(new Date()))
const chartDimension = ref('platform')
const selectedSeriesName = ref('')
const messageStatsLoading = ref(false)
const messageStats = ref({ hours: [], by_platform: [], by_adapter: [] })
const resourceStatus = ref({
  cpuUsagePercent: 0,
  memoryUsagePercent: 0,
  memoryUsedBytes: 0,
  memoryTotalBytes: 0,
  allBotCpuUsagePercent: 0,
  allBotMemoryUsagePercent: 0,
  allBotMemoryUsedBytes: 0,
  allBotMemoryTotalBytes: 0,
  allBotPeakCpuUsagePercent: 0,
  allBotPeakMemoryUsedBytes: 0,
  allBotPeakMemoryUsagePercent: 0,
  allBotHistoricalPeakCpuUsagePercent: 0,
  allBotHistoricalPeakMemoryUsedBytes: 0,
  allBotHistoricalPeakMemoryUsagePercent: 0,
  allBotAverageCpuUsagePercent: 0,
  allBotAverageMemoryUsedBytes: 0,
  allBotAverageMemoryUsagePercent: 0
})
const messageChartRef = ref(null)
const cpuChartRef = ref(null)
const memoryChartRef = ref(null)
let messageChart = null
let cpuChart = null
let memoryChart = null
let refreshTimer = null
let uptimeTimer = null
let totalUptimeBaseSeconds = null
let currentUptimeBaseSeconds = null
let uptimeBaseAt = 0
let uptimeLastSyncedAt = 0
const uptimeSyncInterval = 60 * 1000

const platformSeries = computed(() => normalizeSeries(messageStats.value.by_platform))
const adapterSeries = computed(() => normalizeSeries(messageStats.value.by_adapter))
const currentSeries = computed(() => chartDimension.value === 'platform' ? platformSeries.value : adapterSeries.value)

const loadData = async () => {
  try {
    const status = await getSystemStatus()
    syncUptime(status)
    setPluginCountStat(status.pluginCount || 0, status.enabledPluginCount || 0)
    setAdapterCountStat(status.adapterCount || 0, status.runningAdapterCount || 0)
    setMessageCountStat(status.messageCount || 0, status.todayMessageCount || 0)
    setResourceStatus(status)
    await nextTick()
    renderResourceCharts()
    await loadMessageStats()
  } catch (error) {
    console.error('加载数据失败:', error)
  }
}

const loadMessageStats = async () => {
  messageStatsLoading.value = true
  try {
    messageStats.value = await getMessageStats({ mode: statsMode.value, date: statsDate.value })
    ensureSelectedSeries()
    await nextTick()
    renderMessageChart()
  } catch (error) {
    console.error('加载消息统计失败:', error)
  } finally {
    messageStatsLoading.value = false
  }
}

const handleStatsModeChange = async () => {
  await loadMessageStats()
}

const renderMessageChart = () => {
  if (!messageChartRef.value) return
  if (!messageChart) {
    messageChart = echarts.init(messageChartRef.value)
    messageChart.on('legendselectchanged', () => {
      renderMessageChart()
    })
  }
  ensureSelectedSeries()
  const hours = Array.isArray(messageStats.value.hours) && messageStats.value.hours.length > 0
    ? messageStats.value.hours
    : [0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22]
  const seriesItems = selectedSeriesName.value
    ? currentSeries.value.filter(item => item.name === selectedSeriesName.value)
    : currentSeries.value
  const title = chartDimension.value === 'platform' ? '不同平台消息分布' : '不同机器人消息分布'
  messageChart.setOption({
    title: { text: title, left: 0, top: 0, textStyle: { fontSize: 15, fontWeight: 600 } },
    color: ['#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#64748b', '#8b5cf6', '#06b6d4', '#ec4899'],
    tooltip: { trigger: 'axis' },
    legend: {
      top: 28,
      type: 'scroll',
      selectedMode: false,
      data: seriesItems.map(item => `${item.name}（${item.total}）`)
    },
    grid: { left: 48, right: 24, top: 76, bottom: 42 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: hours.map(hour => `${String(hour).padStart(2, '0')}:00`)
    },
    yAxis: { type: 'value', minInterval: 1, name: '消息数' },
    series: seriesItems.map(item => ({
      name: `${item.name}（${item.total}）`,
      type: 'line',
      smooth: true,
      symbolSize: 7,
      data: item.counts
    }))
  }, true)
}

function renderResourceCharts() {
  renderUsagePie(cpuChartRef, cpuChart, chart => { cpuChart = chart }, 'CPU', resourceStatus.value.allBotCpuUsagePercent, resourceStatus.value.cpuUsagePercent, '#6366f1', '#f59e0b')
  renderUsagePie(memoryChartRef, memoryChart, chart => { memoryChart = chart }, '内存', resourceStatus.value.allBotMemoryUsagePercent, resourceStatus.value.memoryUsagePercent, '#22c55e', '#8b5cf6')
}

function renderUsagePie(chartRef, chartInstance, setChart, name, allBotPercent, systemPercent, allBotColor, systemColor) {
  if (!chartRef.value) return
  let chart = chartInstance
  if (!chart) {
    chart = echarts.init(chartRef.value)
    setChart(chart)
  }
  const allBotUsed = clampPercent(allBotPercent)
  const systemUsed = clampPercent(systemPercent)
  const realOtherSystemUsed = Math.max(0, systemUsed - allBotUsed)
  const visualAllBotUsed = visualResourcePercent(allBotUsed)
  const visualOtherSystemUsed = Math.min(visualResourcePercent(realOtherSystemUsed), 100 - visualAllBotUsed)
  const unused = Math.max(0, 100 - visualAllBotUsed - visualOtherSystemUsed)
  const realValues = {
    'allBot 占用': allBotUsed,
    '系统其他占用': realOtherSystemUsed,
    '未使用': Math.max(0, 100 - systemUsed)
  }
  chart.setOption({
    color: [allBotColor, systemColor, '#e5e7eb'],
    tooltip: {
      trigger: 'item',
      formatter: item => `${item.name}: ${formatPercent(realValues[item.name] ?? item.value)}`
    },
    series: [{
      name,
      type: 'pie',
      radius: ['62%', '82%'],
      center: ['50%', '50%'],
      avoidLabelOverlap: false,
      label: { show: false },
      labelLine: { show: false },
      data: [
        { name: 'allBot 占用', value: visualAllBotUsed },
        { name: '系统其他占用', value: visualOtherSystemUsed },
        { name: '未使用', value: unused }
      ]
    }]
  }, true)
}

function setResourceStatus(status) {
  resourceStatus.value = {
    cpuUsagePercent: clampPercent(status?.cpuUsagePercent),
    memoryUsagePercent: clampPercent(status?.memoryUsagePercent),
    memoryUsedBytes: normalizeBytes(status?.memoryUsedBytes),
    memoryTotalBytes: normalizeBytes(status?.memoryTotalBytes),
    allBotCpuUsagePercent: clampPercent(status?.allBotCpuUsagePercent),
    allBotMemoryUsagePercent: clampPercent(status?.allBotMemoryUsagePercent),
    allBotMemoryUsedBytes: normalizeBytes(status?.allBotMemoryUsedBytes),
    allBotMemoryTotalBytes: normalizeBytes(status?.allBotMemoryTotalBytes),
    allBotPeakCpuUsagePercent: clampPercent(status?.allBotPeakCpuUsagePercent),
    allBotPeakMemoryUsedBytes: normalizeBytes(status?.allBotPeakMemoryUsedBytes),
    allBotPeakMemoryUsagePercent: clampPercent(status?.allBotPeakMemoryUsagePercent),
    allBotHistoricalPeakCpuUsagePercent: clampPercent(status?.allBotHistoricalPeakCpuUsagePercent),
    allBotHistoricalPeakMemoryUsedBytes: normalizeBytes(status?.allBotHistoricalPeakMemoryUsedBytes),
    allBotHistoricalPeakMemoryUsagePercent: clampPercent(status?.allBotHistoricalPeakMemoryUsagePercent),
    allBotAverageCpuUsagePercent: clampPercent(status?.allBotAverageCpuUsagePercent),
    allBotAverageMemoryUsedBytes: normalizeBytes(status?.allBotAverageMemoryUsedBytes),
    allBotAverageMemoryUsagePercent: clampPercent(status?.allBotAverageMemoryUsagePercent)
  }
}

function clampPercent(value) {
  const number = Number(value)
  if (!Number.isFinite(number) || number < 0) return 0
  return Math.min(100, number)
}

function visualResourcePercent(value) {
  const percent = clampPercent(value)
  if (percent < 10) return 10
  return percent
}

function normalizeBytes(value) {
  const number = Number(value)
  return Number.isFinite(number) && number > 0 ? number : 0
}

function formatPercent(value) {
  return `${clampPercent(value).toFixed(2)}%`
}

function formatBytes(value) {
  const bytes = normalizeBytes(value)
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${size.toFixed(unitIndex === 0 ? 0 : 2)} ${units[unitIndex]}`
}

function formatCompactBytes(value) {
  const bytes = normalizeBytes(value)
  if (bytes === 0) return '0B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${size.toFixed(unitIndex === 0 ? 0 : 1)}${units[unitIndex]}`
}

function formatUsageWithPercent(used, total) {
  const totalBytes = normalizeBytes(total)
  if (totalBytes === 0) return formatCompactBytes(used)
  return `${formatCompactBytes(used)}(${formatPercent(percentOfBytes(used, totalBytes))})`
}

function formatHistoricalUsage(used, percent) {
  return `${formatCompactBytes(used)}(${formatPercent(percent)})`
}

function percentOfBytes(used, total) {
  const usedBytes = normalizeBytes(used)
  const totalBytes = normalizeBytes(total)
  if (totalBytes === 0 || usedBytes > totalBytes) return 0
  return (usedBytes / totalBytes) * 100
}

function resizeCharts() {
  renderMessageChart()
  renderResourceCharts()
}

function ensureSelectedSeries() {
  if (selectedSeriesName.value && !currentSeries.value.some(item => item.name === selectedSeriesName.value)) {
    selectedSeriesName.value = ''
  }
}

function normalizeSeries(items) {
  return Array.isArray(items) ? items.filter(item => item && item.name).map(item => ({
    name: item.name,
    counts: Array.isArray(item.counts) ? item.counts : [],
    total: Number(item.total || 0)
  })) : []
}

function syncUptime(status) {
  const now = Date.now()
  if (totalUptimeBaseSeconds !== null && now - uptimeLastSyncedAt < uptimeSyncInterval) return

  const totalSeconds = normalizeSeconds(status?.totalUptimeSeconds)
  const currentSeconds = normalizeSeconds(status?.currentUptimeSeconds)
  if (totalSeconds === null || currentSeconds === null) {
    syncLegacyUptime(status?.uptime)
    return
  }
  totalUptimeBaseSeconds = totalSeconds
  currentUptimeBaseSeconds = currentSeconds
  uptimeBaseAt = now
  uptimeLastSyncedAt = now
  updateUptimeDisplay()
}

function syncLegacyUptime(uptime) {
  const seconds = parseUptimeSeconds(uptime)
  if (seconds === null) {
    if (totalUptimeBaseSeconds === null) {
      stats.value[0].value = uptime || '--'
      stats.value[0].subtext = uptime ? `累计 ${uptime}` : ''
    }
    return
  }
  totalUptimeBaseSeconds = seconds
  currentUptimeBaseSeconds = seconds
  uptimeBaseAt = Date.now()
  uptimeLastSyncedAt = uptimeBaseAt
  updateUptimeDisplay()
}

function updateUptimeDisplay() {
  if (totalUptimeBaseSeconds === null || currentUptimeBaseSeconds === null) return
  const elapsed = Math.floor((Date.now() - uptimeBaseAt) / 1000)
  const totalSeconds = totalUptimeBaseSeconds + Math.max(0, elapsed)
  const currentSeconds = currentUptimeBaseSeconds + Math.max(0, elapsed)
  const totalText = formatDurationSeconds(totalSeconds, 3, true)
  const currentText = formatDurationSeconds(currentSeconds, Infinity, true)
  const totalFullText = formatDurationSeconds(totalSeconds, 3)
  const currentFullText = formatDurationSeconds(currentSeconds)
  stats.value[0].value = currentText
  stats.value[0].tooltip = `当前运行时间: ${currentFullText}`
  stats.value[0].subtext = `累计 ${totalText}`
  stats.value[0].subtextTooltip = `总运行时间: ${totalFullText}`
}

function normalizeSeconds(value) {
  const seconds = Number(value)
  return Number.isFinite(seconds) && seconds >= 0 ? Math.floor(seconds) : null
}

function parseUptimeSeconds(uptime) {
  if (typeof uptime !== 'string') return null
  const units = { y: 365 * 24 * 3600, M: 30 * 24 * 3600, d: 24 * 3600, h: 3600, m: 60, s: 1 }
  const matches = [...uptime.matchAll(/(\d+)\s*([yMdhms])/g)]
  if (matches.length === 0) return null
  return matches.reduce((total, match) => total + Number(match[1]) * units[match[2]], 0)
}

function formatDurationSeconds(totalSeconds, maxParts = Infinity, compact = false) {
  const units = [
    { label: '年', shortLabel: 'Y', seconds: 365 * 24 * 3600 },
    { label: '月', shortLabel: 'M', seconds: 30 * 24 * 3600 },
    { label: '天', shortLabel: 'd', seconds: 24 * 3600 },
    { label: '小时', shortLabel: 'h', seconds: 3600 },
    { label: '分钟', shortLabel: 'm', seconds: 60 },
    { label: '秒', shortLabel: 's', seconds: 1 }
  ]
  let remaining = Math.max(0, Math.floor(totalSeconds))
  const parts = []
  for (const unit of units) {
    const value = Math.floor(remaining / unit.seconds)
    if (value <= 0 && parts.length === 0 && unit.label !== '秒') continue
    if (value > 0 || unit.label === '秒') {
      parts.push(`${value}${compact ? unit.shortLabel : unit.label}`)
      remaining -= value * unit.seconds
    }
    if (parts.length >= maxParts) break
  }
  return parts.join(' ')
}

function setPluginCountStat(total, enabled) {
  const totalCount = Number(total || 0)
  const enabledCount = Number(enabled || 0)
  stats.value[1].value = formatCompactCount(enabledCount)
  stats.value[1].tooltip = `运行中插件: ${formatExactCount(enabledCount)}`
  stats.value[1].subtext = `共 ${formatCompactCount(totalCount)} 个插件`
  stats.value[1].subtextTooltip = `插件总数: ${formatExactCount(totalCount)}`
}

function setAdapterCountStat(total, running) {
  const totalCount = Number(total || 0)
  const runningCount = Number(running || 0)
  stats.value[2].value = formatCompactCount(runningCount)
  stats.value[2].tooltip = `运行中机器人: ${formatExactCount(runningCount)}`
  stats.value[2].subtext = `共 ${formatCompactCount(totalCount)} 个机器人`
  stats.value[2].subtextTooltip = `机器人总数: ${formatExactCount(totalCount)}`
}

function setMessageCountStat(total, today) {
  const totalCount = Number(total || 0)
  const todayCount = Number(today || 0)
  stats.value[3].value = formatCompactCount(todayCount)
  stats.value[3].tooltip = `今日消息数: ${formatExactCount(todayCount)}`
  stats.value[3].subtext = `累计 ${formatCompactCount(totalCount)} 条`
  stats.value[3].subtextTooltip = `总消息数: ${formatExactCount(totalCount)}`
}

function formatCompactCount(value) {
  const count = Number(value || 0)
  if (count < 1000) return String(count)
  if (count < 10000) return `${(count / 1000).toFixed(2)}k`
  return `${(count / 10000).toFixed(2)}w`
}

function formatExactCount(value) {
  return Number(value || 0).toLocaleString('zh-CN')
}

function formatDate(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

watch(chartDimension, () => {
  selectedSeriesName.value = ''
  renderMessageChart()
})

onMounted(() => {
  loadData()
  refreshTimer = setInterval(loadData, 5000)
  uptimeTimer = setInterval(updateUptimeDisplay, 1000)
  window.addEventListener('resize', resizeCharts)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  if (uptimeTimer) {
    clearInterval(uptimeTimer)
  }
  window.removeEventListener('resize', resizeCharts)
  if (messageChart) {
    messageChart.dispose()
    messageChart = null
  }
  if (cpuChart) {
    cpuChart.dispose()
    cpuChart = null
  }
  if (memoryChart) {
    memoryChart.dispose()
    memoryChart = null
  }
})
</script>

<style scoped>
.dashboard {
  width: 100%;
}

.stat-card {
  cursor: pointer;
  transition: transform var(--transition-normal), box-shadow var(--transition-normal);
  height: 100%;
  overflow: hidden;
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-md);
}

.stat-card :deep(.el-card__body) {
  display: flex;
  align-items: center;
  padding: 20px;
  min-height: 104px;
  box-sizing: border-box;
  overflow: hidden;
}

.stat-icon {
  position: relative;
  width: 46px;
  height: 46px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: white;
  font-size: 20px;
  margin-right: 16px;
  background: color-mix(in srgb, white 88%, transparent);
  box-shadow:
    inset 0 0 0 1px rgba(255, 255, 255, 0.72),
    0 4px 14px rgba(31, 41, 55, 0.08);
  overflow: hidden;
}

.stat-icon::before {
  content: '';
  position: absolute;
  inset: 4px;
  border-radius: var(--radius-sm);
  background: var(--stat-gradient);
  opacity: 0.9;
}

.stat-icon::after {
  content: '';
  position: absolute;
  top: 7px;
  left: 9px;
  width: 18px;
  height: 10px;
  border-radius: var(--radius-full);
  background: rgba(255, 255, 255, 0.25);
  filter: blur(1px);
}

.stat-icon :deep(svg) {
  position: relative;
  z-index: 1;
  width: 18px;
  height: 18px;
  stroke-width: 1.8;
}

.stat-content {
  flex: 1;
  min-width: 0;
}

.stat-value {
  font-size: 28px;
  font-weight: 800;
  font-family: var(--font-heading);
  color: var(--text-primary);
  margin-bottom: 5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: -0.02em;
}

.stat-title {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 18px;
}

.stat-subtext {
  margin-top: 0;
  font-size: 12px;
  line-height: 16px;
  color: var(--color-success);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chart-header {
  gap: 12px;
}

.chart-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.series-select {
  width: 160px;
}

.chart-row {
  margin-top: 20px;
  row-gap: 20px;
}

.dashboard-card {
  height: 100%;
}

.chart-wrap {
  min-height: 420px;
}

.message-chart {
  width: 100%;
  height: 420px;
}

.resource-hint {
  color: var(--text-tertiary);
  font-size: 13px;
}

.resource-grid {
  min-height: 420px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.resource-panel {
  position: relative;
  border-radius: var(--radius-xl);
  padding: 20px 14px 16px;
  background:
    radial-gradient(circle at 22% 18%, rgba(99, 102, 241, 0.04), transparent 40%),
    linear-gradient(145deg, var(--bg-surface) 0%, var(--bg-surface-hover) 100%);
  box-shadow: inset 0 0 0 1px var(--border-subtle);
  overflow: hidden;
}

.resource-panel::after {
  content: '';
  position: absolute;
  right: -36px;
  bottom: -42px;
  width: 120px;
  height: 120px;
  border-radius: 50%;
  background: rgba(99, 102, 241, 0.05);
}

.resource-title {
  position: relative;
  z-index: 1;
  color: var(--text-secondary);
  font-size: 15px;
  font-weight: 600;
  text-align: center;
}

.resource-chart {
  position: relative;
  z-index: 1;
  width: 100%;
  height: 260px;
}

.resource-value {
  position: relative;
  z-index: 1;
  margin-top: -18px;
  color: var(--text-primary);
  font-size: 28px;
  font-weight: 700;
  font-family: var(--font-heading);
  text-align: center;
  letter-spacing: -0.02em;
}

.resource-detail {
  position: relative;
  z-index: 1;
  margin-top: 8px;
  color: var(--text-secondary);
  font-size: 13px;
  text-align: center;
  word-break: break-all;
}

@media (max-width: 768px) {
  .dashboard {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow-y: auto;
    overflow-x: hidden;
    padding-right: 2px;
  }

  .stats-row {
    row-gap: 12px;
  }

  .stat-card:hover {
    transform: none;
  }

  .stat-card :deep(.el-card__body) {
    min-height: 92px;
    padding: 14px;
  }

  .stat-icon {
    width: 42px;
    height: 42px;
    border-radius: 13px;
    font-size: 18px;
    margin-right: 12px;
  }

  .stat-icon::before {
    inset: 4px;
    border-radius: 10px;
  }

  .stat-icon :deep(svg) {
    width: 17px;
    height: 17px;
  }

  .stat-value {
    font-size: 22px;
    margin-bottom: 2px;
  }

  .stat-title {
    font-size: 12px;
    line-height: 16px;
  }

  .stat-subtext {
    font-size: 11px;
    line-height: 14px;
    white-space: nowrap;
  }

  .chart-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .message-chart {
    height: 360px;
  }

  .resource-grid {
    min-height: auto;
    grid-template-columns: 1fr;
  }

  .resource-chart {
    height: 220px;
  }

}

@media (max-width: 420px) {
  .stat-card :deep(.el-card__body) {
    min-height: 86px;
    padding: 12px;
  }

  .stat-icon {
    width: 38px;
    height: 38px;
    font-size: 17px;
    margin-right: 10px;
    border-radius: 12px;
  }

  .stat-icon :deep(svg) {
    width: 16px;
    height: 16px;
  }

  .stat-value {
    font-size: 20px;
  }
}
</style>
