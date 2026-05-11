<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :span="6" v-for="stat in stats" :key="stat.title">
        <el-card class="stat-card">
          <div class="stat-icon" :style="{ background: stat.color }">
            <component :is="stat.icon" />
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stat.value }}</div>
            <div class="stat-title">{{ stat.title }}</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="12">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>插件状态</span>
            </div>
          </template>
          <el-table :data="plugins" style="width: 100%">
            <el-table-column prop="name" label="插件名称" />
            <el-table-column prop="version" label="版本" width="100" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.status === 'running' ? 'success' : 'danger'" size="small">
                  {{ row.status === 'running' ? '运行中' : '已停止' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>平台状态</span>
            </div>
          </template>
          <el-table :data="adapters" style="width: 100%">
            <el-table-column prop="platform" label="平台" width="120">
              <template #default="{ row }">
                {{ getPlatformName(row.platform) }}
              </template>
            </el-table-column>
            <el-table-column label="启用" width="80">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
                  {{ row.enabled ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="运行" width="80">
              <template #default="{ row }">
                <el-tag :type="row.running ? 'success' : 'danger'" size="small">
                  {{ row.running ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-row style="margin-top: 20px">
      <el-col :span="24">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>快速操作</span>
            </div>
          </template>
          <div class="quick-actions">
            <el-button type="primary" @click="$router.push('/plugins')">
              <el-icon><Grid /></el-icon>
              管理插件
            </el-button>
            <el-button type="success" @click="$router.push('/adapters')">
              <el-icon><Connection /></el-icon>
              配置平台
            </el-button>
            <el-button @click="$router.push('/logs')">
              <el-icon><Document /></el-icon>
              查看日志
            </el-button>
            <el-button @click="refreshData">
              <el-icon><Refresh /></el-icon>
              刷新数据
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Timer,
  Grid as GridIcon,
  Connection as ConnectionIcon,
  ChatDotRound,
  Grid,
  Connection,
  Document,
  Refresh
} from '@element-plus/icons-vue'
import { getSystemStatus, getPlugins, getAdapters } from '@/api'

const stats = ref([
  { title: '运行时间', value: '--', icon: Timer, color: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)' },
  { title: '插件总数', value: 0, icon: GridIcon, color: 'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)' },
  { title: '运行中', value: 0, icon: ConnectionIcon, color: 'linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)' },
  { title: '消息数', value: 0, icon: ChatDotRound, color: 'linear-gradient(135deg, #43e97b 0%, #38f9d7 100%)' }
])

const plugins = ref([])
const adapters = ref([])

const loadData = async () => {
  try {
    // 加载系统状态
    const status = await getSystemStatus()
    stats.value[0].value = status.uptime || '--'
    stats.value[1].value = status.pluginCount || 0
    stats.value[2].value = status.runningCount || 0
    stats.value[3].value = status.messageCount || 0

    // 加载插件列表
    plugins.value = await getPlugins()

    // 加载适配器列表
    adapters.value = await getAdapters()
  } catch (error) {
    console.error('加载数据失败:', error)
  }
}

const getPlatformName = (platform) => {
  const names = {
    'qq': 'QQ',
    'wechat': '微信',
    'telegram': 'Telegram'
  }
  return names[platform] || platform
}

const refreshData = async () => {
  ElMessage.info('正在刷新数据...')
  await loadData()
  ElMessage.success('数据已刷新')
}

onMounted(() => {
  loadData()

  // 每 5 秒自动刷新
  setInterval(loadData, 5000)
})
</script>

<style scoped>
.dashboard {
  width: 100%;
}

.stat-card {
  cursor: pointer;
  transition: transform 0.3s, box-shadow 0.3s;
}

.stat-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
}

.stat-card :deep(.el-card__body) {
  display: flex;
  align-items: center;
  padding: 20px;
}

.stat-icon {
  width: 60px;
  height: 60px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 28px;
  margin-right: 20px;
}

.stat-content {
  flex: 1;
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
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.quick-actions {
  display: flex;
  gap: 10px;
}
</style>
