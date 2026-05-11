<template>
  <div class="plugins">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>插件列表</span>
          <el-button type="primary" size="small">
            <el-icon><Plus /></el-icon>
            安装插件
          </el-button>
        </div>
      </template>

      <el-table :data="plugins" v-loading="loading" style="width: 100%">
        <el-table-column prop="name" label="插件名称" min-width="150" />
        <el-table-column prop="version" label="版本" width="100" />
        <el-table-column prop="runtime" label="运行时" width="100">
          <template #default="{ row }">
            <el-tag size="small">{{ row.runtime }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '已启用' : '已禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="平台" min-width="150">
          <template #default="{ row }">
            <el-tag
              v-for="platform in row.platforms"
              :key="platform"
              size="small"
              style="margin-right: 5px"
            >
              {{ getPlatformName(platform) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.enabled"
              type="warning"
              size="small"
              @click="handleDisable(row)"
            >
              禁用
            </el-button>
            <el-button
              v-else
              type="success"
              size="small"
              @click="handleEnable(row)"
            >
              启用
            </el-button>
            <el-button type="danger" size="small" @click="handleDelete(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && plugins.length === 0" description="暂无插件" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getPlugins, controlPlugin, deletePlugin } from '@/api'

const loading = ref(false)
const plugins = ref([])

const loadPlugins = async () => {
  loading.value = true
  try {
    plugins.value = await getPlugins()
  } catch (error) {
    console.error('加载插件失败:', error)
  } finally {
    loading.value = false
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

const handleEnable = async (plugin) => {
  try {
    await controlPlugin(plugin.id, 'enable')
    ElMessage.success(`插件 ${plugin.name} 已启用`)
    await loadPlugins()
  } catch (error) {
    console.error('启用插件失败:', error)
    ElMessage.error('启用插件失败')
  }
}

const handleDisable = async (plugin) => {
  try {
    await controlPlugin(plugin.id, 'disable')
    ElMessage.success(`插件 ${plugin.name} 已禁用`)
    await loadPlugins()
  } catch (error) {
    console.error('禁用插件失败:', error)
    ElMessage.error('禁用插件失败')
  }
}

const handleDelete = async (plugin) => {
  await ElMessageBox.confirm(
    `确定要删除插件 "${plugin.name}" 吗？`,
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )

  try {
    await deletePlugin(plugin.id)
    ElMessage.success(`插件 ${plugin.name} 已删除`)
    await loadPlugins()
  } catch (error) {
    console.error('删除插件失败:', error)
    ElMessage.error('删除插件失败')
  }
}

onMounted(() => {
  loadPlugins()
})
</script>

<style scoped>
.plugins {
  width: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
