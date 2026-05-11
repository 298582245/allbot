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

const logs = ref([])
let logInterval = null

const mockLogs = () => {
  const levels = ['info', 'warn', 'error', 'debug']
  const messages = [
    'AllBot 启动成功',
    '加载插件: 天气插件',
    '启动 QQ 适配器',
    '收到消息: 你好',
    '插件处理完成',
    '发送消息成功',
    'Web UI 启动: http://localhost:3000',
    '适配器配置已更新',
    '插件依赖安装完成'
  ]

  return {
    time: new Date().toLocaleTimeString(),
    level: levels[Math.floor(Math.random() * levels.length)],
    message: messages[Math.floor(Math.random() * messages.length)]
  }
}

const loadLogs = () => {
  // TODO: 实现真实的日志 API
  // 这里使用模拟数据
  if (logs.value.length < 50) {
    logs.value.unshift(mockLogs())
  } else {
    logs.value.pop()
    logs.value.unshift(mockLogs())
  }
}

const handleRefresh = () => {
  ElMessage.info('日志已刷新')
  loadLogs()
}

const handleClear = () => {
  logs.value = []
  ElMessage.success('日志已清空')
}

onMounted(() => {
  // 初始加载一些日志
  for (let i = 0; i < 10; i++) {
    logs.value.push(mockLogs())
  }

  // 每 3 秒添加新日志
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
