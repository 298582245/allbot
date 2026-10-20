<template>
  <div class="developer-apps-page page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <h2>开发者应用</h2>
            <p>管理开发者 HTTP API 与 MCP 共用的应用凭据和模块授权。</p>
          </div>
          <div class="header-actions">
            <el-input v-model="keyword" class="header-search" clearable placeholder="搜索应用名称或 appKey" />
            <el-button type="primary" :icon="Plus" @click="openCreate">创建应用</el-button>
          </div>
        </div>
      </template>

      <div class="table-area">
        <el-table v-loading="loading" :data="apps" row-key="id" border stripe height="100%">
          <el-table-column prop="name" label="名称" min-width="150" />
          <el-table-column prop="appKey" label="appKey" width="270">
            <template #default="{ row }">
              <el-tooltip :content="row.appKey || row.clientId || row.client_id" placement="top">
                <code class="app-key">{{ row.appKey || row.clientId || row.client_id }}</code>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column label="权限模块" min-width="150">
            <template #default="{ row }">
              <el-tag v-if="row.permissions?.includes('*')" size="small" type="danger" effect="plain" class="module-tag">全部模块</el-tag>
              <template v-else-if="authorizedModules(row).length">
                <div class="module-grid" :style="{ '--module-columns': moduleColumnCount }">
                  <el-tag v-for="moduleName in authorizedModules(row)" :key="moduleName" size="small" effect="plain" class="module-tag">{{ moduleName }}</el-tag>
                </div>
              </template>
              <el-tag v-else size="small" type="info" effect="plain" class="module-tag">未授权</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="90">
            <template #default="{ row }"><el-tag type="success" effect="plain">{{ row.status === 'active' ? '启用' : row.status }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="createdAt" label="创建时间" min-width="175">
            <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column prop="lastUsedAt" label="最近使用" min-width="175">
            <template #default="{ row }">{{ formatTime(row.lastUsedAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" fixed="right" width="230">
            <template #default="{ row }">
              <el-button link type="primary" @click="openEdit(row)">权限</el-button>
              <el-button link type="warning" @click="resetSecret(row)">重置 Secret</el-button>
              <el-button link type="danger" @click="removeApp(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><el-empty :description="keyword ? '没有匹配的应用' : '暂无开发者应用'" /></template>
        </el-table>
      </div>

      <StdPagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        @current-change="load"
        @size-change="handlePageSizeChange"
      />
    </el-card>

    <el-dialog v-model="editorVisible" :title="editing ? '修改应用权限' : '创建开发者应用'" width="720px" top="7vh" class="developer-app-dialog" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="应用名称">
          <el-input v-model="form.name" maxlength="100" show-word-limit placeholder="输入应用名称" />
        </el-form-item>
        <el-form-item label="接口模块授权">
          <el-checkbox v-model="allAccess" @change="toggleAll">全部模块（高风险）</el-checkbox>
          <div v-if="!allAccess" class="capability-groups">
            <div v-for="(items, module) in groupedCapabilities" :key="module" class="capability-group">
              <div class="group-header">
                <div class="group-title">{{ module }}</div>
                <el-checkbox-group v-model="form.permissions">
                  <el-checkbox v-for="item in items" :key="item.code" :value="item.code">
                    <span>选择</span>
                    <el-tag v-if="item.risk === 'high'" size="small" type="warning">含敏感操作</el-tag>
                  </el-checkbox>
                </el-checkbox-group>
              </div>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editorVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveApp">{{ editing ? '保存权限' : '创建应用' }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="secretVisible" title="请立即保存应用凭据" width="560px" :close-on-click-modal="false" @closed="clearSecret">
      <el-alert type="warning" :closable="false" title="appSecret 只会显示这一次。关闭后无法再次查看。" />
      <div v-if="oneTimeAppKey" class="secret-field">
        <label>appKey</label>
        <div class="secret-line"><code>{{ oneTimeAppKey }}</code><el-button :icon="CopyDocument" aria-label="复制 appKey" @click="copyCredential(oneTimeAppKey, 'appKey')" /></div>
      </div>
      <div class="secret-field">
        <label>appSecret</label>
        <div class="secret-line"><code>{{ oneTimeSecret }}</code><el-button :icon="CopyDocument" aria-label="复制 appSecret" @click="copyCredential(oneTimeSecret, 'appSecret')" /></div>
      </div>
      <template #footer><el-button type="primary" @click="secretVisible = false">我已保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Plus } from '@element-plus/icons-vue'
import {
  createDeveloperApp,
  deleteDeveloperApp,
  getDeveloperApps,
  getDeveloperCapabilities,
  resetDeveloperAppSecret,
  updateDeveloperApp
} from '@/api'
import StdPagination from '@/components/StdPagination.vue'

const apps = ref([])
const capabilities = ref([])
const keyword = ref('')
const loading = ref(false)
const saving = ref(false)
const editorVisible = ref(false)
const secretVisible = ref(false)
const oneTimeAppKey = ref('')
const oneTimeSecret = ref('')
const editing = ref(null)
const allAccess = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const form = reactive({ name: '', permissions: [] })
let searchTimer = 0
const groupedCapabilities = computed(() => capabilities.value.reduce((groups, item) => {
  ;(groups[item.module] ||= []).push(item)
  return groups
}, {}))

const authorizedModules = (app) => {
  const permissions = new Set((app.permissions || []).map((permission) => permission.split(':')[0]))
  return [...new Set(capabilities.value
    .filter((capability) => permissions.has(capability.code.split(':')[0]))
    .map((capability) => capability.module))]
}

const moduleColumnCount = computed(() => {
  const max = apps.value.reduce((count, app) => Math.max(count, authorizedModules(app).length), 0)
  return Math.min(10, Math.max(1, max))
})

const formatTime = (value) => value ? new Date(value).toLocaleString() : '从未使用'

const load = async () => {
  loading.value = true
  try {
    const [appResult, capabilityResult] = await Promise.all([
      getDeveloperApps({ page: page.value, page_size: pageSize.value, keyword: keyword.value.trim() }),
      capabilities.value.length ? Promise.resolve(capabilities.value) : getDeveloperCapabilities()
    ])
    const pagedResult = Array.isArray(appResult) ? null : appResult
    apps.value = Array.isArray(appResult) ? appResult : (Array.isArray(pagedResult?.items) ? pagedResult.items : [])
    total.value = Array.isArray(appResult) ? appResult.length : (Number(pagedResult?.total) || 0)
    page.value = Number(pagedResult?.page) || 1
    pageSize.value = Number(pagedResult?.page_size) || pageSize.value
    capabilities.value = capabilityResult || []
  } catch (error) {
    ElMessage.error(error.message || '加载开发者应用失败')
  } finally {
    loading.value = false
  }
}

const handlePageSizeChange = () => {
  page.value = 1
  load()
}

watch(keyword, () => {
  page.value = 1
  clearTimeout(searchTimer)
  searchTimer = setTimeout(load, 300)
})

const openCreate = () => {
  editing.value = null
  form.name = ''
  form.permissions = []
  allAccess.value = false
  editorVisible.value = true
}

const openEdit = (row) => {
  editing.value = row
  form.name = row.name
  allAccess.value = row.permissions?.includes('*') || false
  form.permissions = [...new Set((row.permissions || []).filter((permission) => permission !== '*').map((permission) => permission.split(':')[0]))]
  editorVisible.value = true
}

const toggleAll = (checked) => {
  if (checked) form.permissions = []
}

const saveApp = async () => {
  if (!form.name.trim()) return ElMessage.warning('请填写应用名称')
  saving.value = true
  try {
    const permissions = allAccess.value ? ['*'] : form.permissions
    if (editing.value) {
      await updateDeveloperApp(editing.value.id, { name: form.name.trim(), permissions })
      ElMessage.success('应用权限已更新')
      editorVisible.value = false
      await load()
    } else {
      const result = await createDeveloperApp({ name: form.name.trim(), permissions })
      editorVisible.value = false
      oneTimeAppKey.value = result?.app?.appKey || result?.app?.clientId || result?.app?.client_id || ''
      oneTimeSecret.value = result?.appSecret || result?.app_secret || ''
      secretVisible.value = true
      await load()
    }
  } catch (error) {
    ElMessage.error(error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

const resetSecret = async (row) => {
  try {
    await ElMessageBox.confirm(`重置后 ${row.name} 的旧 Secret 和已签发令牌立即失效。`, '重置 appSecret', { type: 'warning', confirmButtonText: '重置' })
    const result = await resetDeveloperAppSecret(row.id)
    oneTimeAppKey.value = ''
    oneTimeSecret.value = result?.appSecret || result?.app_secret || ''
    secretVisible.value = true
    await load()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error(error.message || '重置失败')
  }
}

const removeApp = async (row) => {
  try {
    await ElMessageBox.confirm(`删除后 ${row.name} 的所有凭据立即失效，且无法恢复。`, '删除开发者应用', { type: 'error', confirmButtonText: '删除' })
    await deleteDeveloperApp(row.id)
    ElMessage.success('应用已删除')
    await load()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error(error.message || '删除失败')
  }
}

const copyCredential = async (value, label) => {
  await navigator.clipboard.writeText(value)
  ElMessage.success(`${label} 已复制`)
}

const clearSecret = () => {
  oneTimeAppKey.value = ''
  oneTimeSecret.value = ''
}

onMounted(load)
onBeforeUnmount(() => clearTimeout(searchTimer))
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 12px; overflow: hidden; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.page-header h2 { margin: 0 0 6px; font-size: 18px; font-weight: 600; }
.page-header p { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.header-actions { display: flex; align-items: center; gap: 10px; }
.header-search { width: 280px; }
.app-key { display: inline-block; max-width: 100%; white-space: nowrap; }
.module-grid { display: grid; grid-template-columns: repeat(var(--module-columns), max-content); gap: 4px; justify-content: start; }
.module-tag { height: 24px; margin: 0; padding: 2px 8px; line-height: 18px; }
.table-area { flex: 1; min-height: 0; overflow: hidden; }
.capability-groups { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); width: 100%; margin-top: 12px; border-top: 1px solid var(--el-border-color-lighter); }
.capability-group { min-width: 0; padding: 12px 10px 4px 0; border-bottom: 1px solid var(--el-border-color-lighter); }
.group-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
.group-title { color: var(--el-text-color-primary); font-size: 13px; font-weight: 600; }
.group-header :deep(.el-checkbox-group) { display: flex; flex-wrap: wrap; gap: 2px 14px; }
.capability-group :deep(.el-checkbox) { max-width: 100%; margin-right: 0; align-items: flex-start; }
.capability-group :deep(.el-checkbox__label) { min-width: 0; white-space: normal; overflow-wrap: anywhere; }
.capability-group :deep(.el-tag) { margin-left: 5px; }
:global(.developer-app-dialog.el-dialog) { height: min(680px, 84vh); display: flex; flex-direction: column; }
:global(.developer-app-dialog .el-dialog__header),
:global(.developer-app-dialog .el-dialog__footer) { flex: 0 0 auto; }
:global(.developer-app-dialog .el-dialog__body) { flex: 1 1 auto; min-height: 0; overflow-y: auto; }
.secret-field { margin-top: 16px; }
.secret-field label { color: var(--el-text-color-secondary); font-size: 13px; }
.secret-line { display: flex; align-items: center; gap: 8px; margin-top: 6px; }
.secret-line code { flex: 1; overflow-wrap: anywhere; padding: 10px; background: var(--el-fill-color-light); }
@media (max-width: 768px) {
  .page-shell { height: calc(100dvh - 52px - 76px - 24px); overflow: hidden; }
  .page-header { align-items: stretch; flex-direction: column; }
  .header-actions { align-items: stretch; flex-direction: column; }
  .header-search { width: 100%; }
  .capability-groups { grid-template-columns: 1fr; }
  :global(.developer-app-dialog.el-dialog) { height: 82dvh; margin-top: 9dvh !important; }
}
</style>
