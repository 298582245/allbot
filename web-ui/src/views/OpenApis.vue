<template>
  <div class="open-apis-page">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span class="title">开放接口</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看开放接口说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
          <div class="header-actions">
            <el-button :loading="loading" @click="loadItems">刷新</el-button>
            <el-button type="primary" @click="createItem">
              <el-icon><Plus /></el-icon>
              新增接口
            </el-button>
          </div>
        </div>
      </template>

      <div class="api-content" v-loading="loading">
        <div class="api-table-area desktop-api-table">
          <el-table :data="paginatedItems" row-key="id" stripe border height="100%" class="api-table">
            <el-table-column prop="name" label="接口名称" min-width="150" show-overflow-tooltip>
              <template #default="{ row }">{{ row.name || row.id || '-' }}</template>
            </el-table-column>
            <el-table-column label="启用状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="方法" width="96">
              <template #default="{ row }">
                <el-tag effect="plain">{{ normalizeMethod(row.method) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="路径" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">
                <code>{{ displayPath(row) }}</code>
              </template>
            </el-table-column>
            <el-table-column label="Runtime" width="120">
              <template #default="{ row }">
                <el-tag type="warning" effect="plain">{{ runtimeLabel(row.runtime) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="描述" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">{{ row.description || '-' }}</template>
            </el-table-column>
            <el-table-column label="Token" width="110">
              <template #default="{ row }">
                <el-tag :type="hasToken(row) ? 'success' : 'info'">{{ hasToken(row) ? '已设置' : '未设置' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="340" fixed="right">
              <template #default="{ row }">
                <el-button size="small" type="primary" @click="editItem(row)">编辑</el-button>
                <el-button size="small" type="warning" @click="openFile(row)">文件</el-button>
                <el-button size="small" type="success" @click="copyAddress(row)">复制地址</el-button>
                <el-button size="small" type="danger" :loading="deletingId === itemId(row)" @click="deleteItem(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-if="paginatedItems.length > 0" class="api-grid mobile-api-grid">
          <div v-for="row in paginatedItems" :key="itemId(row)" class="api-card-item">
            <div class="api-card-header">
              <span class="api-name">{{ row.name || row.id || '-' }}</span>
              <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
            </div>
            <div class="api-card-body">
              <div class="api-info-row">
                <span class="label">方法：</span>
                <el-tag effect="plain" size="small">{{ normalizeMethod(row.method) }}</el-tag>
              </div>
              <div class="api-info-row">
                <span class="label">路径：</span>
                <code class="path-text">{{ displayPath(row) }}</code>
              </div>
              <div class="api-info-row">
                <span class="label">Runtime：</span>
                <el-tag type="warning" effect="plain" size="small">{{ runtimeLabel(row.runtime) }}</el-tag>
              </div>
              <div class="api-info-row">
                <span class="label">描述：</span>
                <span class="value-text">{{ row.description || '-' }}</span>
              </div>
              <div class="api-info-row">
                <span class="label">Token：</span>
                <el-tag :type="hasToken(row) ? 'success' : 'info'" size="small">{{ hasToken(row) ? '已设置' : '未设置' }}</el-tag>
              </div>
            </div>
            <div class="api-card-footer">
              <el-button size="small" type="primary" @click="editItem(row)">编辑</el-button>
              <el-button size="small" type="warning" @click="openFile(row)">文件</el-button>
              <el-button size="small" type="success" @click="copyAddress(row)">复制地址</el-button>
              <el-button size="small" type="danger" :loading="deletingId === itemId(row)" @click="deleteItem(row)">删除</el-button>
            </div>
          </div>
        </div>

        <el-empty v-if="!loading && items.length === 0" description="暂无开放接口" />
      </div>

      <div v-if="items.length > 0" class="pagination-wrapper">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="items.length"
          layout="total, prev, pager, next"
          background
        />
      </div>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogMode === 'create' ? '新增开放接口' : '编辑开放接口'"
      width="560px"
      class="api-dialog"
      :close-on-click-modal="false"
    >
      <el-form :model="form" label-width="96px" class="dialog-form">
        <el-form-item label="接口名称" required>
          <el-input v-model="form.name" maxlength="60" show-word-limit placeholder="例如：自定义回复接口" />
        </el-form-item>
        <el-form-item label="接口路径" required>
          <el-input v-model="form.path" :disabled="dialogMode === 'edit'" placeholder="只输入单个词，例如 a">
            <template #prepend>/api/open/</template>
          </el-input>
          <div class="field-tip">只允许字母、数字、横线和下划线，不能输入 a/b、/a 或 /api/open/a。</div>
        </el-form-item>
        <el-form-item label="请求方法" required>
          <el-select v-model="form.method" style="width: 100%">
            <el-option v-for="method in httpMethods" :key="method" :label="method" :value="method" />
          </el-select>
        </el-form-item>
        <el-form-item label="是否开启">
          <el-switch v-model="form.enabled" active-text="开启" inactive-text="停用" />
        </el-form-item>
        <el-form-item label="运行语言" required>
          <el-radio-group v-model="form.runtime">
            <el-radio-button label="nodejs">Node.js</el-radio-button>
            <el-radio-button label="python">Python</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="Token" required>
          <el-input
            v-model="form.token"
            type="password"
            show-password
            :placeholder="dialogMode === 'create' ? '必填，调用接口时需要 Token' : '留空则保留原 Token，填写则更新'"
          />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            maxlength="240"
            show-word-limit
            :rows="3"
            placeholder="说明这个接口的用途、入参或调用场景"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveDialog">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, Plus } from '@element-plus/icons-vue'
import { createOpenApi, deleteOpenApi, getOpenApis, updateOpenApi } from '@/api'

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const deletingId = ref('')
const items = ref([])
const currentPage = ref(1)
const pageSize = 10
const dialogVisible = ref(false)
const dialogMode = ref('create')
const editingId = ref('')
const form = reactive(createEmptyForm())
const singlePathPattern = /^[A-Za-z0-9_-]+$/
const httpMethods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']
const pageDescription = '独立管理对外 HTTP 接口、运行时和调用代码。'

const paginatedItems = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return items.value.slice(start, start + pageSize)
})

watch(items, () => {
  const maxPage = Math.max(1, Math.ceil(items.value.length / pageSize))
  if (currentPage.value > maxPage) currentPage.value = maxPage
})

const loadItems = async () => {
  loading.value = true
  try {
    const result = await getOpenApis()
    items.value = normalizeListResult(result)
  } finally {
    loading.value = false
  }
}

const normalizeListResult = (result) => {
  if (Array.isArray(result)) return result
  if (Array.isArray(result?.items)) return result.items
  if (Array.isArray(result?.apis)) return result.apis
  if (Array.isArray(result?.open_apis)) return result.open_apis
  if (Array.isArray(result?.openApis)) return result.openApis
  return []
}

const normalizeMethod = (method) => String(method || 'POST').toUpperCase()

const normalizePath = (path) => String(path || '').replace(/\\/g, '/').replace(/^\/api\/open\//, '').replace(/^\/+|\/+$/g, '').trim()

const normalizeRuntime = (runtime) => runtime === 'python' ? 'python' : 'nodejs'

const runtimeLabel = (runtime) => normalizeRuntime(runtime) === 'python' ? 'Python' : 'Node.js'

const resolvePath = (row) => row.path || row.url_path || row.urlPath || row.route || ''

const displayPath = (row) => {
  const normalized = normalizePath(resolvePath(row))
  return normalized ? `/api/open/${normalized}` : '/api/open'
}

const hasToken = (row) => {
  if (typeof row.has_token === 'boolean') return row.has_token
  if (typeof row.hasToken === 'boolean') return row.hasToken
  if (typeof row.token_set === 'boolean') return row.token_set
  if (typeof row.tokenSet === 'boolean') return row.tokenSet
  return Boolean(String(row.token || '').trim())
}

const itemId = (row) => String(row.id || row.api_id || row.apiId || normalizePath(resolvePath(row)) || '')

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '开放接口说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

const createItem = () => {
  dialogMode.value = 'create'
  editingId.value = ''
  Object.assign(form, createEmptyForm())
  dialogVisible.value = true
}

const editItem = (row) => {
  const id = itemId(row)
  if (!id) {
    ElMessage.warning('接口缺少 ID，无法编辑')
    return
  }
  dialogMode.value = 'edit'
  editingId.value = id
  Object.assign(form, {
    name: row.name || id,
    path: normalizePath(resolvePath(row)) || id,
    enabled: Boolean(row.enabled),
    runtime: normalizeRuntime(row.runtime),
    token: '',
    description: row.description || '',
    method: normalizeMethod(row.method)
  })
  dialogVisible.value = true
}

const openFile = (row) => {
  const id = itemId(row)
  if (!id) {
    ElMessage.warning('接口缺少 ID，无法打开文件')
    return
  }
  router.push(`/open-apis/${encodeURIComponent(id)}/edit`)
}

const saveDialog = async () => {
  const payload = buildPayload()
  if (!payload) return

  saving.value = true
  try {
    if (dialogMode.value === 'create') {
      await createOpenApi(payload)
      ElMessage.success('开放接口已创建')
    } else {
      await updateOpenApi(editingId.value, payload)
      ElMessage.success('开放接口已保存')
    }
    dialogVisible.value = false
    await loadItems()
  } finally {
    saving.value = false
  }
}

const buildPayload = () => {
  const name = String(form.name || '').trim()
  const validation = validatePath(form.path)
  if (!name) {
    ElMessage.warning('请输入接口名称')
    return null
  }
  if (!validation.ok) {
    ElMessage.warning(validation.message)
    return null
  }
  if (isPathDuplicated(validation.path)) {
    ElMessage.warning('接口路径已存在，请换一个路径名')
    return null
  }

  const runtime = normalizeRuntime(form.runtime)
  const token = String(form.token || '').trim()
  const currentItem = dialogMode.value === 'edit' ? items.value.find((row) => itemId(row) === editingId.value) : null
  if (!token && (dialogMode.value === 'create' || !hasToken(currentItem || {}) || Boolean(form.enabled))) {
    ElMessage.warning('请输入 Open API token')
    return null
  }
  const payload = {
    id: dialogMode.value === 'create' ? validation.path : editingId.value,
    name,
    path: validation.path,
    method: normalizeMethod(form.method),
    enabled: Boolean(form.enabled),
    runtime,
    description: String(form.description || '').trim(),
    entry: codeFileName(validation.path, runtime)
  }
  if (dialogMode.value === 'create' || token) payload.token = token
  return payload
}

const validatePath = (value) => {
  const path = String(value || '').trim()
  if (!path) return { ok: false, message: '请输入接口路径' }
  if (path.includes('/') || path.includes('\\')) {
    return { ok: false, message: '接口路径只支持单个词，例如 a，不能输入 a/b、/a 或 /api/open/a' }
  }
  if (!singlePathPattern.test(path)) {
    return { ok: false, message: '接口路径只能包含字母、数字、横线和下划线' }
  }
  return { ok: true, path }
}

const isPathDuplicated = (path) => {
  const currentId = editingId.value
  const normalized = path.toLowerCase()
  return items.value.some((row) => {
    if (dialogMode.value === 'edit' && itemId(row) === currentId) return false
    return normalizePath(resolvePath(row)).toLowerCase() === normalized
  })
}

const codeFileName = (path, runtime) => `${path}.${normalizeRuntime(runtime) === 'python' ? 'py' : 'js'}`

function createEmptyForm() {
  return {
    name: '',
    path: '',
    enabled: true,
    runtime: 'nodejs',
    token: '',
    description: '',
    method: 'POST'
  }
}

const deleteItem = async (row) => {
  const id = itemId(row)
  if (!id) {
    ElMessage.warning('接口缺少 ID，无法删除')
    return
  }
  try {
    await ElMessageBox.confirm(`确定删除开放接口「${row.name || id}」吗？`, '删除确认', { type: 'warning' })
  } catch {
    return
  }
  deletingId.value = id
  try {
    await deleteOpenApi(id)
    ElMessage.success('开放接口已删除')
    await loadItems()
  } finally {
    deletingId.value = ''
  }
}

const copyAddress = async (row) => {
  const address = displayPath(row)
  const url = `${window.location.origin}${address}`
  try {
    await navigator.clipboard.writeText(url)
  } catch {
    copyByFallback(url)
  }
  ElMessage.success('接口地址已复制')
}

const copyByFallback = (text) => {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'readonly')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

onMounted(loadItems)
</script>

<style scoped>
.open-apis-page {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.page-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.page-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.title {
  font-size: 18px;
  font-weight: 600;
}

.mobile-info-button {
  display: none;
  padding: 0;
  font-size: 16px;
}

.subtitle {
  margin-top: 6px;
  color: #909399;
  font-size: 13px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.api-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-bottom: 12px;
}

.api-table-area {
  height: 100%;
  min-height: 0;
}

.api-grid.mobile-api-grid {
  display: none;
}

.api-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.api-card-item {
  min-width: 0;
  min-height: 260px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  background: #fff;
  transition: box-shadow 0.2s;
}

.api-card-item:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

.api-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid #f0f0f0;
}

.api-name {
  min-width: 0;
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  word-break: break-all;
}

.api-card-body {
  flex: 1;
}

.api-info-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 13px;
  color: #606266;
}

.api-info-row .label {
  min-width: 70px;
  flex-shrink: 0;
  color: #909399;
}

.value-text {
  min-width: 0;
  word-break: break-all;
}

.api-table code,
.path-text {
  max-width: 100%;
  padding: 4px 8px;
  border-radius: 6px;
  color: #1d4ed8;
  background: #eff6ff;
  font-family: "JetBrains Mono", "Cascadia Code", monospace;
  word-break: break-all;
  white-space: normal;
}

.api-card-footer {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid #f0f0f0;
}

.api-card-footer .el-button {
  width: 100%;
  margin-left: 0;
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

.dialog-form {
  padding: 2px 4px 0;
}

.field-tip {
  margin-top: 6px;
  color: #909399;
  font-size: 12px;
  line-height: 1.5;
}

.api-dialog :deep(.el-dialog__header) {
  padding-bottom: 12px;
  border-bottom: 1px solid #edf0f5;
}

.api-dialog :deep(.el-dialog__footer) {
  padding-top: 12px;
  border-top: 1px solid #edf0f5;
}

@media (max-width: 768px) {
  .open-apis-page {
    height: calc(100dvh - 52px - 76px - 24px);
    min-height: 0;
    overflow: hidden;
  }

  .page-card {
    height: 100%;
    min-height: 100%;
  }

  .page-card :deep(.el-card__body) {
    overflow: hidden;
  }

  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .title {
    font-size: 16px;
  }

  .mobile-info-button {
    display: inline-flex;
  }

  .subtitle {
    display: none;
  }

  .header-actions {
    width: 100%;
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
  }

  .header-actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .api-content {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
    padding-bottom: 12px;
  }

  .desktop-api-table {
    display: none;
  }

  .api-grid.mobile-api-grid {
    display: grid;
  }

  .api-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
  }

  .api-card-item {
    min-height: auto;
    padding: 14px;
  }

  .api-info-row {
    gap: 6px;
  }

  .api-info-row .label {
    min-width: 66px;
  }

  .api-card-footer {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .pagination-wrapper {
    flex-shrink: 0;
    justify-content: flex-start;
    overflow-x: auto;
    margin-top: 12px;
    padding-top: 12px;
    min-height: 45px;
  }

  .api-dialog :deep(.el-dialog) {
    width: 94vw !important;
  }

  .api-dialog :deep(.el-form-item) {
    display: block;
  }

  .api-dialog :deep(.el-form-item__label) {
    width: 100% !important;
    justify-content: flex-start;
    padding: 0 0 6px;
  }

  .api-dialog :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }

  .api-dialog :deep(.el-input-group__prepend) {
    padding: 0 10px;
  }

  .api-content::-webkit-scrollbar,
  .pagination-wrapper::-webkit-scrollbar {
    display: none;
  }
}
</style>
