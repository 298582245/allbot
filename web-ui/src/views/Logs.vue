<template>
  <div class="logs">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>系统日志</span>
          <div>
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

      <div class="log-container">
        <div
          v-for="(log, index) in logs"
          :key="index"
          :class="['log-item', `log-${log.level}`]"
        >
          <span class="log-time">{{ log.time }}</span>
          <span class="log-level">{{ log.level.toUpperCase() }}</span>
          <span class="log-message">{{ log.message }}</span>
        </div>

        <el-empty v-if="logs.length === 0" description="暂无日志" />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Delete } from '@element-plus/icons-vue'
import { getLogs, clearLogs } from '@/api'

const logs = ref([])
let logInterval = null

const loadLogs = async () => {
  try {
    logs.value = await getLogs()
  } catch (error) {
    console.error('加载日志失败:', error)
  }
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
.logs {
  width: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.log-container {
  background: #1e1e1e;
  border-radius: 4px;
  padding: 15px;
  max-height: 600px;
  overflow-y: auto;
  font-family: 'Courier New', monospace;
  font-size: 13px;
}

.log-item {
  padding: 5px 0;
  display: flex;
  gap: 10px;
  border-bottom: 1px solid #333;
}

.log-item:last-child {
  border-bottom: none;
}

.log-time {
  color: #888;
  min-width: 80px;
}

.log-level {
  min-width: 60px;
  font-weight: bold;
}

.log-message {
  flex: 1;
  color: #ddd;
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
</style>
