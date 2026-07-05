<template>
  <div class="logs page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="card-header">
          <span>系统日志</span>
          <div class="header-actions">
            <el-button size="small" @click="loadLogs(true)">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
            <el-button size="small" type="warning" :disabled="!selectedDate" @click="handleDeleteDate">
              <el-icon><Delete /></el-icon>
              删除当前日期
            </el-button>
            <el-button size="small" type="danger" @click="handleClearAll">
              <el-icon><Delete /></el-icon>
              清空全部
            </el-button>
          </div>
        </div>
      </template>

      <div class="log-toolbar">
        <el-select v-model="selectedDate" placeholder="选择日期" filterable @change="handleFilterChange">
          <el-option
            v-for="item in logDates"
            :key="item.date"
            :label="`${item.date} (${formatFileSize(item.size)})`"
            :value="item.date"
          />
        </el-select>
        <el-select v-model="level" placeholder="日志等级" @change="handleFilterChange">
          <el-option label="全部等级" value="" />
          <el-option label="INFO" value="info" />
          <el-option label="WARN" value="warn" />
          <el-option label="ERROR" value="error" />
          <el-option label="DEBUG" value="debug" />
        </el-select>
        <el-input
          v-model="keyword"
          clearable
          placeholder="搜索日志内容、时间或等级"
          @clear="handleFilterChange"
          @keyup.enter="handleFilterChange"
        />
        <el-button @click="handleFilterChange">搜索</el-button>
      </div>

      <div class="log-options">
        <div class="toolbar-switches">
          <el-switch v-model="mergeRepeatLogs" size="small" active-text="合并重复日志" />
          <el-switch
            v-model="pauseScroll"
            size="small"
            active-text="暂停滚动"
            inactive-text="自动定位最新"
          />
        </div>
        <div class="retention-settings">
          <span>保留天数</span>
          <el-input-number v-model="retentionDays" :min="0" :max="3650" size="small" controls-position="right" />
          <span class="settings-tip">0 表示禁用自动清理</span>
          <el-button size="small" type="primary" @click="handleSaveSettings">保存设置</el-button>
          <el-button size="small" @click="handleCleanupNow">立即清理</el-button>
        </div>
      </div>

      <div v-loading="loading" class="log-container" ref="logContainerRef">
        <div
          v-for="(log, index) in visibleLogs"
          :key="`${pagination.page}-${index}`"
          :class="['log-item', `log-${log.level}`]"
        >
          <div class="log-meta">
            <span class="log-time">{{ formatLogTime(log) }}</span>
            <span class="log-level">{{ String(log.level || 'info').toUpperCase() }}</span>
            <span v-if="shouldShowRepeatBadge(log)" class="log-repeat">×{{ log.repeat }}</span>
          </div>
          <span class="log-message">{{ log.message }}</span>
        </div>

        <el-empty v-if="!loading && visibleLogs.length === 0" :description="selectedDate ? '暂无日志' : '暂无日志文件'" />
      </div>

      <StdPagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[50, 100, 200, 500, 1000]"
        @current-change="loadLogs"
        @size-change="handlePageSizeChange"
      />
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Refresh } from '@element-plus/icons-vue'
import { cleanupLogs, clearLogs, getLogSettings, getLogs, saveLogSettings } from '@/api'
import StdPagination from '@/components/StdPagination.vue'

const logs = ref([])
const logDates = ref([])
const selectedDate = ref('')
const keyword = ref('')
const level = ref('')
const retentionDays = ref(0)
const loading = ref(false)
const logContainerRef = ref(null)
const pauseScroll = ref(false)
const mergeRepeatLogs = ref(true)
const pagination = ref({ page: 1, pageSize: 100, total: 0 })
let logInterval = null

const visibleLogs = computed(() => {
  if (mergeRepeatLogs.value) return logs.value
  return logs.value.map((log) => ({
    ...log,
    rawRepeat: log.repeat,
    repeat: 1,
    lastTime: ''
  }))
})

const todayText = () => {
  const now = new Date()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${now.getFullYear()}-${month}-${day}`
}

const isTodaySelected = computed(() => selectedDate.value === todayText())

const normalizeRepeat = (repeat) => {
  const value = Number(repeat)
  return Number.isFinite(value) && value > 0 ? Math.floor(value) : 1
}

const shouldShowRepeatBadge = (log) => {
  return mergeRepeatLogs.value && normalizeRepeat(log.repeat) > 1
}

const formatLogTime = (log) => {
  if (!shouldShowRepeatBadge(log) || !log.lastTime || log.lastTime === log.time) {
    return log.time
  }
  return `${log.time} - ${log.lastTime}`
}

const formatFileSize = (size) => {
  const value = Number(size) || 0
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

const loadSettings = async () => {
  try {
    const data = await getLogSettings()
    retentionDays.value = Number(data.retention_days) || 0
  } catch (error) {
    console.error('加载日志设置失败:', error)
  }
}

const loadLogs = async (showMessage = false) => {
  loading.value = true
  try {
    const params = {
      date: selectedDate.value,
      page: pagination.value.page,
      page_size: pagination.value.pageSize,
      keyword: keyword.value.trim(),
      level: level.value
    }
    const data = await getLogs(params)
    logs.value = Array.isArray(data.items) ? data.items : []
    logDates.value = Array.isArray(data.dates) ? data.dates : []
    pagination.value.total = Number(data.total) || 0
    pagination.value.page = Number(data.page) || 1
    pagination.value.pageSize = Number(data.page_size) || pagination.value.pageSize
    retentionDays.value = Number(data.retention_days) || retentionDays.value || 0
    if (data.date && data.date !== selectedDate.value) selectedDate.value = data.date
    if (!pauseScroll.value && pagination.value.page === 1) scrollToLatest()
    if (showMessage) ElMessage.success('日志已刷新')
  } catch (error) {
    console.error('加载日志失败:', error)
    ElMessage.error('加载日志失败')
  } finally {
    loading.value = false
  }
}

const scrollToLatest = () => {
  requestAnimationFrame(() => {
    const container = logContainerRef.value
    if (container) container.scrollTop = 0
  })
}

const handleFilterChange = () => {
  pagination.value.page = 1
  loadLogs()
}

const handlePageSizeChange = () => {
  pagination.value.page = 1
  loadLogs()
}

const reloadAfterDelete = async () => {
  pagination.value.page = 1
  selectedDate.value = ''
  await loadLogs()
}

const handleDeleteDate = async () => {
  if (!selectedDate.value) return
  try {
    await ElMessageBox.confirm(`确定删除 ${selectedDate.value} 的日志文件吗？`, '删除日志', { type: 'warning' })
    await clearLogs({ date: selectedDate.value })
    ElMessage.success('当前日期日志已删除')
    await reloadAfterDelete()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除日志失败:', error)
      ElMessage.error('删除日志失败')
    }
  }
}

const handleClearAll = async () => {
  try {
    await ElMessageBox.confirm('确定删除全部日志文件吗？该操作不会删除非日志文件。', '清空全部日志', { type: 'warning' })
    await clearLogs()
    logs.value = []
    ElMessage.success('全部日志已清空')
    await reloadAfterDelete()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('清空日志失败:', error)
      ElMessage.error('清空日志失败')
    }
  }
}

const handleSaveSettings = async () => {
  try {
    await saveLogSettings({ retention_days: Number(retentionDays.value) || 0 })
    ElMessage.success('日志设置已保存')
  } catch (error) {
    console.error('保存日志设置失败:', error)
    ElMessage.error('保存日志设置失败')
  }
}

const handleCleanupNow = async () => {
  try {
    const data = await cleanupLogs({ retention_days: Number(retentionDays.value) || 0 })
    ElMessage.success(`清理完成，删除 ${Number(data.deleted) || 0} 个日志文件`)
    await reloadAfterDelete()
  } catch (error) {
    console.error('立即清理日志失败:', error)
    ElMessage.error('立即清理日志失败')
  }
}

onMounted(async () => {
  await loadSettings()
  await loadLogs()
  logInterval = setInterval(() => {
    if (isTodaySelected.value && pagination.value.page === 1) loadLogs()
  }, 3000)
})

onUnmounted(() => {
  if (logInterval) clearInterval(logInterval)
})
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.log-toolbar {
  display: grid;
  grid-template-columns: 180px 130px minmax(220px, 1fr) auto;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.log-options {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.toolbar-switches,
.retention-settings {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  min-width: 0;
}

.settings-tip {
  color: #909399;
  font-size: 12px;
}

.log-container {
  flex: 1;
  min-height: 0;
  background: #1e1e1e;
  border-radius: 8px;
  padding: 15px;
  overflow-y: auto;
  font-family: 'Courier New', monospace;
  font-size: 13px;
}

.log-container :deep(.el-empty__description p) {
  color: #909399;
}

.log-item {
  padding: 7px 0;
  display: flex;
  gap: 10px;
  border-bottom: 1px solid #333;
}

.log-item:last-child { border-bottom: none; }

.log-meta {
  display: flex;
  gap: 10px;
  flex-shrink: 0;
}

.log-time {
  color: #888;
  min-width: 80px;
}

.log-level {
  min-width: 60px;
  font-weight: bold;
}

.log-repeat {
  flex-shrink: 0;
  padding: 0 6px;
  border-radius: 999px;
  background: #3a3a3a;
  color: #f5f5f5;
  font-size: 12px;
  font-weight: bold;
  line-height: 18px;
}

.log-message {
  flex: 1;
  color: #ddd;
  min-width: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

.log-info .log-level { color: #52c41a; }
.log-warn .log-level { color: #faad14; }
.log-error .log-level { color: #f5222d; }
.log-debug .log-level { color: #1890ff; }

.log-container::-webkit-scrollbar { width: 8px; }
.log-container::-webkit-scrollbar-track { background: #2a2a2a; }
.log-container::-webkit-scrollbar-thumb { background: #555; border-radius: 4px; }
.log-container::-webkit-scrollbar-thumb:hover { background: #666; }

@media (max-width: 768px) {
  .page-shell {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow: hidden;
  }

  .page-card :deep(.el-card__body) {
    display: block;
    overflow-y: auto;
  }

  .card-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }

  .header-actions {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
  }

  .header-actions .el-button {
    width: 100%;
    margin-left: 0;
    font-size: 12px;
  }

  .log-toolbar {
    grid-template-columns: 1fr 1fr;
    gap: 8px;
    margin-bottom: 10px;
  }

  .log-options {
    flex-direction: column;
    gap: 8px;
    margin-bottom: 10px;
  }

  .toolbar-switches {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .retention-settings {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
  }

  .retention-settings .settings-tip {
    display: none;
  }

  .toolbar-switches :deep(.el-switch) {
    justify-content: space-between;
  }

  .log-container {
    flex: none;
    height: 300px;
    padding: 10px;
    font-size: 12px;
    border-radius: 10px;
  }

  .log-item {
    display: block;
    padding: 10px 0;
  }

  .log-meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: flex-start;
    gap: 8px;
    margin-bottom: 6px;
  }

  .log-time {
    min-width: 0;
    color: #9ca3af;
  }

  .log-level {
    min-width: 0;
    padding: 1px 6px;
    border-radius: 999px;
    background: #2a2a2a;
    font-size: 11px;
  }

  .log-message {
    display: block;
    line-height: 1.55;
    word-break: break-word;
  }
}
</style>
