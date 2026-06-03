<template>
  <div class="dashboard">
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="12" :sm="12" :md="6" v-for="stat in stats" :key="stat.title">
        <el-card class="stat-card">
          <div class="stat-icon" :style="{ '--stat-gradient': stat.color }">
            <component :is="stat.icon" />
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stat.value }}</div>
            <div class="stat-title">{{ stat.title }}</div>
            <div v-if="stat.subtext" class="stat-subtext">{{ stat.subtext }}</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="24">
        <el-card>
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
    </el-row>

  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import {
  TrendCharts,
  Grid as GridIcon,
  Connection as ConnectionIcon,
  ChatLineRound
} from '@element-plus/icons-vue'
import { getSystemStatus, getMessageStats } from '@/api'

echarts.use([LineChart, GridComponent, LegendComponent, TitleComponent, TooltipComponent, CanvasRenderer])

const stats = ref([
  { title: '运行时间', value: '--', icon: TrendCharts, color: 'linear-gradient(135deg, #6d7dfc 0%, #8b5cf6 100%)' },
  { title: '插件总数', value: 0, subtext: '运行中: 0', icon: GridIcon, color: 'linear-gradient(135deg, #ff8fc7 0%, #ff6b88 100%)' },
  { title: '平台机器人', value: 0, icon: ConnectionIcon, color: 'linear-gradient(135deg, #38bdf8 0%, #22d3ee 100%)' },
  { title: '消息数', value: 0, icon: ChatLineRound, color: 'linear-gradient(135deg, #34d399 0%, #2dd4bf 100%)' }
])

const statsMode = ref('date')
const statsDate = ref(formatDate(new Date()))
const chartDimension = ref('platform')
const selectedSeriesName = ref('')
const messageStatsLoading = ref(false)
const messageStats = ref({ hours: [], by_platform: [], by_adapter: [] })
const messageChartRef = ref(null)
let messageChart = null
let refreshTimer = null
let uptimeTimer = null
let uptimeBaseSeconds = null
let uptimeBaseAt = 0
let uptimeLastSyncedAt = 0
const uptimeSyncInterval = 60 * 1000

const platformSeries = computed(() => normalizeSeries(messageStats.value.by_platform))
const adapterSeries = computed(() => normalizeSeries(messageStats.value.by_adapter))
const currentSeries = computed(() => chartDimension.value === 'platform' ? platformSeries.value : adapterSeries.value)

const loadData = async () => {
  try {
    const status = await getSystemStatus()
    syncUptime(status.uptime)
    stats.value[1].value = status.pluginCount || 0
    stats.value[1].subtext = `运行中: ${status.enabledPluginCount || 0}`
    stats.value[2].value = status.adapterCount || 0
    stats.value[3].value = status.messageCount || 0
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
    color: ['#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#909399', '#9c27b0', '#00bcd4', '#795548'],
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

function syncUptime(uptime) {
  const now = Date.now()
  if (uptimeBaseSeconds !== null && now - uptimeLastSyncedAt < uptimeSyncInterval) return

  const seconds = parseUptimeSeconds(uptime)
  if (seconds === null) {
    if (uptimeBaseSeconds === null) stats.value[0].value = uptime || '--'
    return
  }
  uptimeBaseSeconds = seconds
  uptimeBaseAt = now
  uptimeLastSyncedAt = now
  updateUptimeDisplay()
}

function updateUptimeDisplay() {
  if (uptimeBaseSeconds === null) return
  const elapsed = Math.floor((Date.now() - uptimeBaseAt) / 1000)
  stats.value[0].value = formatUptimeSeconds(uptimeBaseSeconds + Math.max(0, elapsed))
}

function parseUptimeSeconds(uptime) {
  if (typeof uptime !== 'string') return null
  const matches = [...uptime.matchAll(/(\d+)\s*([hms])/g)]
  if (matches.length === 0) return null
  return matches.reduce((total, match) => {
    const value = Number(match[1])
    if (match[2] === 'h') return total + value * 3600
    if (match[2] === 'm') return total + value * 60
    return total + value
  }, 0)
}

function formatUptimeSeconds(totalSeconds) {
  const seconds = totalSeconds % 60
  const minutes = Math.floor(totalSeconds / 60) % 60
  const hours = Math.floor(totalSeconds / 3600)
  if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`
  if (minutes > 0) return `${minutes}m ${seconds}s`
  return `${seconds}s`
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
  window.addEventListener('resize', renderMessageChart)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  if (uptimeTimer) {
    clearInterval(uptimeTimer)
  }
  window.removeEventListener('resize', renderMessageChart)
  if (messageChart) {
    messageChart.dispose()
    messageChart = null
  }
})
</script>

<style scoped>
.dashboard {
  width: 100%;
}

.stat-card {
  cursor: pointer;
  transition: transform 0.3s, box-shadow 0.3s;
  height: 100%;
  overflow: hidden;
}

.stat-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
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
  border-radius: 14px;
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
    0 8px 22px rgba(31, 41, 55, 0.1);
  overflow: hidden;
}

.stat-icon::before {
  content: '';
  position: absolute;
  inset: 4px;
  border-radius: 11px;
  background: var(--stat-gradient);
  box-shadow: 0 6px 14px rgba(31, 41, 55, 0.14);
}

.stat-icon::after {
  content: '';
  position: absolute;
  top: 7px;
  left: 9px;
  width: 18px;
  height: 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.28);
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
  font-weight: bold;
  color: #333;
  margin-bottom: 5px;
}

.stat-title {
  font-size: 14px;
  color: #666;
  line-height: 18px;
}

.stat-subtext {
  margin-top: 0;
  font-size: 12px;
  line-height: 16px;
  color: #67c23a;
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

.chart-wrap {
  min-height: 420px;
}

.message-chart {
  width: 100%;
  height: 420px;
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
