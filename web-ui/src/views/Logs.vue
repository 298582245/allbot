<template>
  <div class="logs page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="card-header">
          <span>系统日志</span>
          <div class="header-actions">
            <el-button size="small" @click="handleRefresh">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
            <el-button size="small" @click="handleClear">
              <el-icon><Delete /></el-icon>
              清空
            </el-button>
          </div>
        </div>
      </template>

      <div class="log-toolbar">
        <el-input
          v-model="searchKeyword"
          clearable
          placeholder="搜索日志内容、时间或等级"
        />
        <div class="toolbar-switches">
          <el-switch
            v-model="mergeRepeatLogs"
            size="small"
            active-text="合并重复日志"
          />
          <el-switch
            v-model="pauseScroll"
            size="small"
            active-text="暂停滚动"
            inactive-text="自动定位最新"
          />
        </div>
      </div>

      <div class="log-container" ref="logContainerRef">
        <div
          v-for="(log, index) in filteredLogs"
          :key="index"
          :class="['log-item', `log-${log.level}`]"
        >
          <div class="log-meta">
            <span class="log-time">{{ formatLogTime(log) }}</span>
            <span class="log-level">{{ log.level.toUpperCase() }}</span>
            <span v-if="shouldShowRepeatBadge(log)" class="log-repeat">×{{ log.repeat }}</span>
          </div>
          <span class="log-message">{{ log.message }}</span>
        </div>

        <el-empty v-if="filteredLogs.length === 0" :description="searchKeyword ? '没有匹配的日志' : '暂无日志'" />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Delete } from '@element-plus/icons-vue'
import { getLogs, clearLogs } from '@/api'

const logs = ref([])
const logContainerRef = ref(null)
const searchKeyword = ref('')
const pauseScroll = ref(false)
const mergeRepeatLogs = ref(true)
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

const filteredLogs = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  if (!keyword) return visibleLogs.value
  return visibleLogs.value.filter((log) => {
    return [log.time, log.lastTime, log.level, log.message, log.repeat, log.rawRepeat]
      .some((value) => String(value || '').toLowerCase().includes(keyword))
  })
})

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

const loadLogs = async () => {
  try {
    logs.value = await getLogs()
    if (!pauseScroll.value && !searchKeyword.value.trim()) scrollToLatest()
  } catch (error) {
    console.error('加载日志失败:', error)
  }
}

const scrollToLatest = () => {
  requestAnimationFrame(() => {
    const container = logContainerRef.value
    if (container) container.scrollTop = 0
  })
}

const handleRefresh = async () => {
  await loadLogs()
  ElMessage.success('日志已刷新')
}

const handleClear = async () => {
  try {
    await clearLogs()
    logs.value = []
    ElMessage.success('日志已清空')
  } catch (error) {
    console.error('清空日志失败:', error)
    ElMessage.error('清空日志失败')
  }
}

onMounted(() => {
  loadLogs()

  // 每 3 秒自动刷新
  logInterval = setInterval(() => {
    loadLogs()
  }, 3000)
})

onUnmounted(() => {
  if (logInterval) {
    clearInterval(logInterval)
  }
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
  grid-template-columns: minmax(240px, 420px) minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.toolbar-switches {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 16px;
  min-width: 0;
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

.log-item {
  padding: 7px 0;
  display: flex;
  gap: 10px;
  border-bottom: 1px solid #333;
}

.log-item:last-child {
  border-bottom: none;
}

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

.log-info .log-level {
  color: #52c41a;
}

.log-warn .log-level {
  color: #faad14;
}

.log-error .log-level {
  color: #f5222d;
}

.log-debug .log-level {
  color: #1890ff;
}

.log-container::-webkit-scrollbar {
  width: 8px;
}

.log-container::-webkit-scrollbar-track {
  background: #2a2a2a;
}

.log-container::-webkit-scrollbar-thumb {
  background: #555;
  border-radius: 4px;
}

.log-container::-webkit-scrollbar-thumb:hover {
  background: #666;
}

@media (max-width: 768px) {
  .page-shell {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow: hidden;
  }

  .card-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }

  .header-actions {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .header-actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .log-toolbar {
    grid-template-columns: 1fr;
    gap: 8px;
    margin-bottom: 10px;
  }

  .toolbar-switches {
    display: grid;
    grid-template-columns: 1fr;
    gap: 6px;
  }

  .toolbar-switches :deep(.el-switch) {
    justify-content: space-between;
  }

  .log-container {
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

