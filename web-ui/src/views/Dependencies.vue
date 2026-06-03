<template>
  <div class="dependencies">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>全局依赖管理</span>
        </div>
      </template>

      <div class="toolbar">
        <el-tabs v-model="activeTab" class="tabs">
          <el-tab-pane label="Python 依赖" name="python" />
          <el-tab-pane label="Node.js 依赖" name="nodejs" />
        </el-tabs>
        <el-button type="primary" @click="showAddDialog(activeTab)">
          <el-icon><Plus /></el-icon>
          安装新依赖
        </el-button>
      </div>

      <el-alert
        class="version-tip"
        title="版本留空会安装并升级到当前最新版，安装完成后列表会显示实际版本号，不再显示 latest。"
        type="info"
        show-icon
        :closable="false"
      />

      <div v-if="activeTab === 'python'" class="deps-content" v-loading="loading">
        <div class="dep-table desktop-dep-table">
          <el-table :data="paginatedPythonDeps" style="width: 100%" height="100%">
            <el-table-column prop="name" label="包名" min-width="220" />
            <el-table-column prop="version" label="已安装版本" min-width="180" />
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button type="danger" size="small" @click="handleUninstall('python', row.name)">卸载</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div v-if="paginatedPythonDeps.length > 0" class="dep-grid mobile-dep-grid">
          <div v-for="dep in paginatedPythonDeps" :key="dep.name" class="dep-card">
            <div class="dep-card-header">
              <span class="dep-name">{{ dep.name }}</span>
              <el-tag size="small" type="success">Python</el-tag>
            </div>
            <div class="dep-card-body">
              <div class="dep-info-row">
                <span class="label">版本：</span>
                <code class="version-text">{{ dep.version || 'unknown' }}</code>
              </div>
            </div>
            <div class="dep-card-footer">
              <el-button type="danger" size="small" @click="handleUninstall('python', dep.name)">卸载</el-button>
            </div>
          </div>
        </div>
        <el-empty v-if="!loading && pythonDeps.length === 0" description="暂无 Python 依赖" />
      </div>

      <div v-if="activeTab === 'nodejs'" class="deps-content" v-loading="loading">
        <div class="dep-table desktop-dep-table">
          <el-table :data="paginatedNodejsDeps" style="width: 100%" height="100%">
            <el-table-column prop="name" label="包名" min-width="220" />
            <el-table-column prop="version" label="已安装版本" min-width="180" />
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button type="danger" size="small" @click="handleUninstall('nodejs', row.name)">卸载</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div v-if="paginatedNodejsDeps.length > 0" class="dep-grid mobile-dep-grid">
          <div v-for="dep in paginatedNodejsDeps" :key="dep.name" class="dep-card">
            <div class="dep-card-header">
              <span class="dep-name">{{ dep.name }}</span>
              <el-tag size="small">Node.js</el-tag>
            </div>
            <div class="dep-card-body">
              <div class="dep-info-row">
                <span class="label">版本：</span>
                <code class="version-text">{{ dep.version || 'unknown' }}</code>
              </div>
            </div>
            <div class="dep-card-footer">
              <el-button type="danger" size="small" @click="handleUninstall('nodejs', dep.name)">卸载</el-button>
            </div>
          </div>
        </div>
        <el-empty v-if="!loading && nodejsDeps.length === 0" description="暂无 Node.js 依赖" />
      </div>

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

    <el-dialog
      v-model="addDialogVisible"
      :title="`安装 ${currentRuntime === 'python' ? 'Python' : 'Node.js'} 依赖`"
      width="500px"
    >
      <el-form :model="newDep" label-width="80px">
        <el-form-item label="包名">
          <el-input v-model.trim="newDep.name" placeholder="例如: requests" />
        </el-form-item>
        <el-form-item label="版本">
          <el-input v-model.trim="newDep.version" placeholder="例如: 2.28.0，留空安装并更新到最新版" />
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

const toDepRows = (deps) => Object.entries(deps || {})
  .map(([name, version]) => ({ name, version }))
  .sort((a, b) => a.name.localeCompare(b.name))

const loadDependencies = async () => {
  loading.value = true
  try {
    const data = await request.get('/dependencies')
    pythonDeps.value = toDepRows(data.python)
    nodejsDeps.value = toDepRows(data.nodejs)
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
      version: newDep.value.version
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
  height: 100%;
  display: flex;
  flex-direction: column;
}

.dependencies > .el-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.dependencies > .el-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 12px;
}

.tabs {
  flex: 1;
}

.version-tip {
  margin-bottom: 16px;
}

.deps-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-bottom: 12px;
}

.dep-table {
  height: 100%;
  min-height: 0;
}

.dep-grid.mobile-dep-grid {
  display: none;
}

.dep-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.dep-card {
  min-height: 150px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  background: #fff;
  transition: box-shadow 0.2s;
}

.dep-card:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

.dep-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid #f0f0f0;
}

.dep-name {
  min-width: 0;
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  word-break: break-all;
}

.dep-card-body {
  flex: 1;
}

.dep-info-row {
  display: flex;
  align-items: center;
  margin-bottom: 6px;
  font-size: 13px;
  color: #606266;
}

.dep-info-row .label {
  color: #909399;
  min-width: 50px;
  flex-shrink: 0;
}

.version-text {
  padding: 1px 6px;
  border-radius: 3px;
  background: #f5f7fa;
  color: #606266;
  font-size: 12px;
  word-break: break-all;
}

.dep-card-footer {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  padding-top: 10px;
  border-top: 1px solid #f0f0f0;
}

.pagination-wrapper {
  position: relative;
  z-index: 2;
  flex-shrink: 0;
  min-height: 49px;
  margin-top: 16px;
  padding-top: 16px;
  display: flex;
  justify-content: center;
  border-top: 1px solid #ebeef5;
  background: #fff;
}

@media (max-width: 768px) {
  .dependencies {
    height: calc(100dvh - 52px - 76px - 24px);
    min-height: 0;
    overflow: hidden;
  }

  .dependencies > .el-card {
    height: 100%;
    min-height: 100%;
  }

  .dependencies > .el-card :deep(.el-card__body) {
    overflow: hidden;
  }

  .card-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }

  .toolbar {
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
  }

  .tabs {
    width: 100%;
  }

  .toolbar > .el-button {
    width: 100%;
    margin-left: 0;
  }

  .version-tip :deep(.el-alert__title) {
    line-height: 1.5;
  }

  .deps-content {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
    padding-bottom: 12px;
  }

  .desktop-dep-table {
    display: none;
  }

  .dep-grid.mobile-dep-grid {
    display: grid;
  }

  .dep-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
  }

  .dep-card {
    min-height: auto;
    padding: 14px;
  }

  .dep-card-header {
    align-items: flex-start;
  }

  .dep-card-footer .el-button {
    margin-left: 0;
  }

  .pagination-wrapper {
    flex-shrink: 0;
    justify-content: flex-start;
    overflow-x: auto;
    margin-top: 12px;
    padding-top: 12px;
    min-height: 45px;
  }

  .deps-content::-webkit-scrollbar,
  .pagination-wrapper::-webkit-scrollbar {
    display: none;
  }
}
</style>
