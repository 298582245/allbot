<template>
  <div class="dependencies">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>全局依赖管理</span>
        </div>
      </template>

      <div style="display: flex; align-items: center; margin-bottom: 20px;">
        <el-tabs v-model="activeTab" style="flex: 1;">
          <el-tab-pane label="Python 依赖" name="python" />
          <el-tab-pane label="Node.js 依赖" name="nodejs" />
        </el-tabs>
        <el-button type="primary" @click="showAddDialog(activeTab)" style="margin-left: 20px;">
          <el-icon><Plus /></el-icon>
          安装新依赖
        </el-button>
      </div>

      <!-- Python 依赖表格 -->
      <div v-if="activeTab === 'python'" class="table-wrapper">
        <el-table :data="paginatedPythonDeps" v-loading="loading" style="width: 100%" max-height="400">
          <el-table-column prop="name" label="包名" min-width="200" />
          <el-table-column prop="version" label="版本" width="150" />
          <el-table-column label="操作" width="120">
            <template #default="{ row }">
              <el-button
                type="danger"
                size="small"
                @click="handleUninstall('python', row.name)"
              >
                卸载
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && pythonDeps.length === 0" description="暂无 Python 依赖" />
      </div>

      <!-- Node.js 依赖表格 -->
      <div v-if="activeTab === 'nodejs'" class="table-wrapper">
        <el-table :data="paginatedNodejsDeps" v-loading="loading" style="width: 100%" max-height="400">
          <el-table-column prop="name" label="包名" min-width="200" />
          <el-table-column prop="version" label="版本" width="150" />
          <el-table-column label="操作" width="120">
            <template #default="{ row }">
              <el-button
                type="danger"
                size="small"
                @click="handleUninstall('nodejs', row.name)"
              >
                卸载
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && nodejsDeps.length === 0" description="暂无 Node.js 依赖" />
      </div>

      <!-- 分页器 -->
      <div class="pagination-wrapper">
        <el-pagination
          v-if="activeTab === 'python' && pythonDeps.length > 0"
          v-model:current-page="pythonCurrentPage"
          :page-size="pageSize"
          :total="pythonDeps.length"
          layout="total, prev, pager, next"
        />
        <el-pagination
          v-if="activeTab === 'nodejs' && nodejsDeps.length > 0"
          v-model:current-page="nodejsCurrentPage"
          :page-size="pageSize"
          :total="nodejsDeps.length"
          layout="total, prev, pager, next"
        />
      </div>
    </el-card>

    <!-- 安装依赖对话框 -->
    <el-dialog
      v-model="addDialogVisible"
      :title="`安装 ${currentRuntime === 'python' ? 'Python' : 'Node.js'} 依赖`"
      width="500px"
    >
      <el-form :model="newDep" label-width="80px">
        <el-form-item label="包名">
          <el-input v-model="newDep.name" placeholder="例如: requests" />
        </el-form-item>
        <el-form-item label="版本">
          <el-input v-model="newDep.version" placeholder="例如: 2.28.0 (留空安装最新版)" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleInstall" :loading="installing">
          安装
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import request from '@/utils/request'

const activeTab = ref('python')
const loading = ref(false)
const pythonDeps = ref([])
const nodejsDeps = ref([])
const addDialogVisible = ref(false)
const installing = ref(false)
const currentRuntime = ref('python')
const newDep = ref({
  name: '',
  version: ''
})

const pageSize = 10
const pythonCurrentPage = ref(1)
const nodejsCurrentPage = ref(1)

const paginatedPythonDeps = computed(() => {
  const start = (pythonCurrentPage.value - 1) * pageSize
  const end = start + pageSize
  return pythonDeps.value.slice(start, end)
})

const paginatedNodejsDeps = computed(() => {
  const start = (nodejsCurrentPage.value - 1) * pageSize
  const end = start + pageSize
  return nodejsDeps.value.slice(start, end)
})

const loadDependencies = async () => {
  loading.value = true
  try {
    const data = await request.get('/dependencies')
    pythonDeps.value = Object.entries(data.python || {}).map(([name, version]) => ({
      name,
      version
    }))
    nodejsDeps.value = Object.entries(data.nodejs || {}).map(([name, version]) => ({
      name,
      version
    }))
  } catch (error) {
    console.error('加载依赖失败:', error)
    ElMessage.error('加载依赖失败')
  } finally {
    loading.value = false
  }
}

const showAddDialog = (runtime) => {
  currentRuntime.value = runtime
  newDep.value = { name: '', version: '' }
  addDialogVisible.value = true
}

const handleInstall = async () => {
  if (!newDep.value.name) {
    ElMessage.warning('请输入包名')
    return
  }

  if (!/^[a-zA-Z0-9_\-\.]+$/.test(newDep.value.name)) {
    ElMessage.error('包名只能包含字母、数字、下划线、连字符和点')
    return
  }

  if (newDep.value.version && !/^[a-zA-Z0-9_\-\.]+$/.test(newDep.value.version)) {
    ElMessage.error('版本号只能包含字母、数字、下划线、连字符和点')
    return
  }

  installing.value = true
  try {
    await request.post('/dependencies', {
      runtime: currentRuntime.value,
      name: newDep.value.name,
      version: newDep.value.version || 'latest'
    })
    ElMessage.success('依赖安装成功')
    addDialogVisible.value = false
    await loadDependencies()
  } catch (error) {
    console.error('安装依赖失败:', error)
    ElMessage.error('安装依赖失败: ' + (error.response?.data?.error || error.message))
  } finally {
    installing.value = false
  }
}

const handleUninstall = async (runtime, name) => {
  await ElMessageBox.confirm(
    `确定要卸载 "${name}" 吗？`,
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )

  try {
    await request.delete(`/dependencies/${runtime}/${name}`)
    ElMessage.success('依赖卸载成功')
    await loadDependencies()
  } catch (error) {
    console.error('卸载依赖失败:', error)
    ElMessage.error('卸载依赖失败')
  }
}

onMounted(() => {
  loadDependencies()
})
</script>

<style scoped>
.dependencies {
  width: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.table-wrapper {
  min-height: 400px;
}

.pagination-wrapper {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}
</style>
