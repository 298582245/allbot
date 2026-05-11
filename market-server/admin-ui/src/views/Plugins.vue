<template>
  <div class="plugins">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>插件管理</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            创建插件
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
        <el-table-column prop="price" label="价格" width="120">
          <template #default="{ row }">
            <span v-if="row.price === 0">免费</span>
            <span v-else>¥{{ (row.price / 100).toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="downloads" label="下载量" width="100" />
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="handleEdit(row)">
              编辑
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
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getPlugins, deletePlugin } from '@/api'

const router = useRouter()
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

const getStatusType = (status) => {
  const typeMap = {
    pending: 'warning',
    approved: 'success',
    rejected: 'danger',
    archived: 'info'
  }
  return typeMap[status] || 'info'
}

const getStatusText = (status) => {
  const textMap = {
    pending: '待审核',
    approved: '已上架',
    rejected: '已拒绝',
    archived: '已下架'
  }
  return textMap[status] || status
}

const handleCreate = () => {
  router.push('/plugins/create')
}

const handleEdit = (plugin) => {
  router.push(`/plugins/${plugin.id}/edit`)
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
