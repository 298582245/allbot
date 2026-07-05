<template>
  <div class="script-envs-page">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span class="title">脚本环境变量</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看脚本环境变量说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
          <div class="header-actions">
            <el-input v-model="searchKeyword" class="header-search" clearable placeholder="搜索变量名、备注或置顶状态" />
            <el-button :loading="loading" @click="loadItems">刷新</el-button>
            <el-upload :show-file-list="false" accept="application/json,.json" :before-upload="handleImportFile">
              <el-button type="warning">
                <el-icon><Upload /></el-icon>
                导入变量
              </el-button>
            </el-upload>
            <el-button type="primary" @click="createItem">
              <el-icon><Plus /></el-icon>
              新增变量
            </el-button>
          </div>
        </div>
      </template>

      <div v-if="selectedRows.length > 0" class="batch-toolbar">
        <span>已选 {{ selectedRows.length }} 项</span>
        <el-button size="small" type="danger" :loading="batching" @click="runBatchAction('delete')">批量删除</el-button>
        <el-button size="small" :loading="batching" @click="runBatchAction('disable')">批量停用</el-button>
        <el-button size="small" type="success" :loading="batching" @click="runBatchAction('enable')">批量启动</el-button>
        <el-button size="small" type="primary" :loading="batching" @click="runBatchAction('pin')">批量置顶</el-button>
        <el-button size="small" :loading="batching" @click="runBatchAction('unpin')">批量取消置顶</el-button>
        <el-button size="small" @click="exportSelectedItems">批量导出</el-button>
      </div>

      <div v-loading="loading" class="env-table-wrap desktop-env-table">
        <el-table ref="tableRef" :data="pagedItems" row-key="id" stripe border height="100%" class="env-table" @selection-change="handleSelectionChange">
          <el-table-column type="selection" width="48" />
          <el-table-column prop="id" label="序号" width="72" />
          <el-table-column prop="name" label="变量名" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="name-with-pin"><code>{{ row.name }}</code><svg v-if="row.pinned" class="pin-icon" viewBox="0 0 24 24" width="14" height="14"><path d="M16 9V4l1-1V2H7v1l1 1v5l-2 2v2h5v6l1 1 1-1v-6h5v-2l-2-2z" fill="currentColor"/></svg></span>
            </template>
          </el-table-column>
          <el-table-column label="变量值" min-width="220">
            <template #default="{ row }">
              <div class="value-cell">
                <el-tooltip :content="String(row.value || '')" placement="top" :disabled="!visibleValueIds[row.id]">
                  <span class="value-text">{{ visibleValueIds[row.id] ? row.value : maskValue(row.value) }}</span>
                </el-tooltip>
                <el-icon class="eye-toggle" @click="toggleValueVisible(row)">
                  <View v-if="!visibleValueIds[row.id]" />
                  <Hide v-else />
                </el-icon>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="启用" width="100">
            <template #default="{ row }">
              <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="remark" label="备注" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.remark || '-' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" @click="editItem(row)">编辑</el-button>
              <el-button size="small" type="danger" :loading="deletingId === row.id" @click="deleteItem(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && items.length === 0" description="暂无脚本环境变量" />
        <el-empty v-else-if="!loading && filteredItems.length === 0" description="没有匹配的环境变量" />
      </div>

      <div v-loading="loading" class="mobile-env-list">
        <div v-for="row in pagedItems" :key="row.id" class="mobile-env-card">
          <div class="mobile-env-title">
            <div class="mobile-title-left">
              <el-checkbox :model-value="isSelected(row)" @change="toggleMobileSelection(row, $event)" />
              <span class="mobile-index">#{{ row.id }}</span>
              <span class="name-with-pin"><code>{{ row.name }}</code><svg v-if="row.pinned" class="pin-icon" viewBox="0 0 24 24" width="14" height="14"><path d="M16 9V4l1-1V2H7v1l1 1v5l-2 2v2h5v6l1 1 1-1v-6h5v-2l-2-2z" fill="currentColor"/></svg></span>
            </div>
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
          </div>
          <div class="mobile-field-grid">
            <div><span>变量值</span><div class="mobile-value-wrap"><strong>{{ visibleValueIds[row.id] ? row.value : maskValue(row.value) }}</strong><el-icon class="eye-toggle" @click="toggleValueVisible(row)"><View v-if="!visibleValueIds[row.id]" /><Hide v-else /></el-icon></div></div>
            <div><span>备注</span><strong>{{ row.remark || '-' }}</strong></div>
          </div>
          <div class="mobile-card-actions">
            <el-button size="small" type="primary" @click="editItem(row)">编辑</el-button>
            <el-button size="small" type="danger" :loading="deletingId === row.id" @click="deleteItem(row)">删除</el-button>
          </div>
        </div>
        <el-empty v-if="!loading && items.length === 0" description="暂无脚本环境变量" />
        <el-empty v-else-if="!loading && filteredItems.length === 0" description="没有匹配的环境变量" />
      </div>

      <StdPagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="filteredItems.length"
        :page-sizes="[10, 20, 50, 100]"
        @size-change="handlePageSizeChange"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogMode === 'create' ? '新增脚本环境变量' : '编辑脚本环境变量'" width="560px">
      <el-form :model="form" label-width="96px">
        <el-form-item label="变量名" required>
          <el-input v-model="form.name" placeholder="例如 API_TOKEN" />
          <div class="field-tip">脚本中通过 process.env.API_TOKEN 或 os.getenv('API_TOKEN') 读取。同名变量允许多个值，脚本读取时用 & 拼接。</div>
        </el-form-item>
        <el-form-item label="变量值">
          <el-input v-model="form.value" type="textarea" :rows="4" show-word-limit maxlength="10000" placeholder="变量值，支持多行" />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>
        <el-form-item label="置顶">
          <el-switch v-model="form.pinned" active-text="置顶" inactive-text="普通" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="3" maxlength="160" show-word-limit placeholder="说明用途" />
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
defineOptions({ name: 'ScriptEnvs' })
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, Plus, Upload, View, Hide } from '@element-plus/icons-vue'
import { batchScriptEnvs, createScriptEnv, deleteScriptEnv, getScriptEnvs, importScriptEnvs, updateScriptEnv } from '@/api'
import StdPagination from '@/components/StdPagination.vue'

const pageDescription = '集中维护脚本运行时可注入的额外环境变量，支持置顶常用变量；插件需在配置中开启后才会读取。'
const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '脚本环境变量说明', { confirmButtonText: '知道了', type: 'info' })
}

const loading = ref(false)
const saving = ref(false)
const batching = ref(false)
const deletingId = ref(0)
const items = ref([])
const selectedRows = ref([])
const tableRef = ref(null)
const syncingSelection = ref(false)
const searchKeyword = ref('')
const dialogVisible = ref(false)
const dialogMode = ref('create')
const editingId = ref(0)
const page = ref(1)
const pageSize = ref(20)
const form = reactive(createEmptyForm())
const visibleValueIds = reactive({})

function toggleValueVisible(row) {
  if (visibleValueIds[row.id]) {
    delete visibleValueIds[row.id]
  } else {
    visibleValueIds[row.id] = true
  }
}

const filteredItems = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  if (!keyword) return items.value
  return items.value.filter((row) => getScriptEnvSearchText(row).includes(keyword))
})

const pagedItems = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredItems.value.slice(start, start + pageSize.value)
})

watch(searchKeyword, () => {
  page.value = 1
})

watch([filteredItems, pageSize], () => {
  const maxPage = Math.max(1, Math.ceil(filteredItems.value.length / pageSize.value))
  if (page.value > maxPage) page.value = maxPage
})

const loadItems = async () => {
  loading.value = true
  try {
    const result = await getScriptEnvs()
    items.value = Array.isArray(result?.items) ? result.items : []
    syncSelectionAfterLoad()
  } finally {
    loading.value = false
  }
}

const createItem = () => {
  Object.assign(form, createEmptyForm())
  editingId.value = 0
  dialogMode.value = 'create'
  dialogVisible.value = true
}

const editItem = (row) => {
  Object.assign(form, createEmptyForm(), row)
  editingId.value = row.id
  dialogMode.value = 'edit'
  dialogVisible.value = true
}

const handlePageSizeChange = () => {
  page.value = 1
}

const handleSelectionChange = (selection) => {
  if (syncingSelection.value) return
  const pagedIds = new Set(pagedItems.value.map((row) => row.id))
  const selectedIds = new Set(selection.map((row) => row.id))
  selectedRows.value = [
    ...selectedRows.value.filter((row) => !pagedIds.has(row.id)),
    ...selection.filter((row) => selectedIds.has(row.id))
  ]
}

const isSelected = (row) => selectedRows.value.some((item) => item.id === row.id)

const toggleMobileSelection = (row, checked) => {
  if (checked) {
    if (!isSelected(row)) selectedRows.value = [...selectedRows.value, row]
  } else {
    selectedRows.value = selectedRows.value.filter((item) => item.id !== row.id)
  }
  syncTableSelection()
}

const clearSelection = () => {
  selectedRows.value = []
  tableRef.value?.clearSelection?.()
}

const syncSelectionAfterLoad = () => {
  const selectedIds = new Set(selectedRows.value.map((row) => row.id))
  selectedRows.value = items.value.filter((row) => selectedIds.has(row.id))
  nextTick(syncTableSelection)
}

const syncTableSelection = () => {
  if (!tableRef.value) return
  syncingSelection.value = true
  tableRef.value.clearSelection()
  const selectedIds = new Set(selectedRows.value.map((row) => row.id))
  pagedItems.value.forEach((row) => {
    if (selectedIds.has(row.id)) tableRef.value.toggleRowSelection(row, true)
  })
  nextTick(() => {
    syncingSelection.value = false
  })
}

watch(pagedItems, () => nextTick(syncTableSelection))

const saveDialog = async () => {
  const payload = buildPayload()
  if (!payload.name) {
    ElMessage.warning('变量名不能为空')
    return
  }
  saving.value = true
  try {
    if (dialogMode.value === 'create') {
      await createScriptEnv(payload)
    } else {
      await updateScriptEnv(editingId.value, payload)
    }
    ElMessage.success('脚本环境变量已保存')
    dialogVisible.value = false
    await loadItems()
  } finally {
    saving.value = false
  }
}

const deleteItem = async (row) => {
  await ElMessageBox.confirm(`确定要删除脚本环境变量 "${row.name}" 吗？`, '删除变量', { type: 'warning' })
  deletingId.value = row.id
  try {
    await deleteScriptEnv(row.id)
    ElMessage.success('脚本环境变量已删除')
    await loadItems()
  } finally {
    deletingId.value = 0
  }
}

const runBatchAction = async (action) => {
  const ids = selectedRows.value.map((row) => row.id)
  if (ids.length === 0) {
    ElMessage.warning('请先选择变量')
    return
  }
  if (action === 'delete') {
    await ElMessageBox.confirm(`确定要删除选中的 ${ids.length} 个脚本环境变量吗？`, '批量删除变量', { type: 'warning' })
  }
  batching.value = true
  try {
    const result = await batchScriptEnvs(action, ids)
    ElMessage.success(result?.message || '批量操作已完成')
    clearSelection()
    await loadItems()
  } finally {
    batching.value = false
  }
}

const exportSelectedItems = () => {
  if (selectedRows.value.length === 0) {
    ElMessage.warning('请先选择变量')
    return
  }
  const data = selectedRows.value.map(({ name, value, remark }) => ({
    name: String(name || ''),
    value: String(value || ''),
    remark: String(remark || '')
  }))
  downloadJSON(data, `script-envs-${formatDateForFile(new Date())}.json`)
}

const handleImportFile = (file) => {
  const reader = new FileReader()
  reader.onload = async () => {
    try {
      const data = JSON.parse(String(reader.result || ''))
      const items = validateImportItems(data)
      const result = await importScriptEnvs(items)
      ElMessage.success(`已导入 ${result?.affected ?? items.length} 个脚本环境变量`)
      clearSelection()
      await loadItems()
    } catch (error) {
      ElMessage.error(error?.message || '导入失败')
    }
  }
  reader.onerror = () => ElMessage.error('读取导入文件失败')
  reader.readAsText(file)
  return false
}

function buildPayload() {
  return {
    name: String(form.name || '').trim(),
    value: String(form.value || ''),
    remark: String(form.remark || '').trim(),
    enabled: Boolean(form.enabled),
    pinned: Boolean(form.pinned)
  }
}

function createEmptyForm() {
  return { name: '', value: '', remark: '', enabled: true, pinned: false }
}

function getScriptEnvSearchText(row) {
  return [
    row.name,
    row.remark,
    row.enabled ? '启用' : '停用',
    row.pinned ? '置顶' : '普通 未置顶'
  ].filter(value => value !== undefined && value !== null).join(' ').toLowerCase()
}

function validateImportItems(data) {
  if (!Array.isArray(data)) throw new Error('导入文件根节点必须是数组')
  const seen = new Set()
  return data.map((item, index) => {
    const name = String(item?.name || '').trim()
    const value = String(item?.value || '')
    const remark = String(item?.remark || '').trim()
    if (!name) throw new Error(`第 ${index + 1} 个变量名不能为空`)
    const key = `${name}\u0000${value}`
    if (seen.has(key)) throw new Error(`导入文件存在重复变量名和值：${name}`)
    seen.add(key)
    return { name, value, remark }
  })
}

function downloadJSON(data, filename) {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function formatDateForFile(date) {
  const pad = (value) => String(value).padStart(2, '0')
  return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}-${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`
}

function maskValue(value) {
  const text = String(value || '')
  if (!text) return '-'
  if (text.length <= 6) return '******'
  return `${text.slice(0, 2)}******${text.slice(-2)}`
}

onMounted(loadItems)
</script>

<style scoped>
.script-envs-page { height: 100%; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 12px; overflow: hidden; }
.page-header { display: flex; justify-content: space-between; gap: 16px; align-items: flex-start; }
.title { font-size: 18px; font-weight: 600; }
.title-row { display: flex; align-items: center; gap: 6px; }
.mobile-info-button { display: none; padding: 0; font-size: 16px; }
.subtitle { margin-top: 6px; color: #909399; font-size: 13px; line-height: 1.5; }
.header-actions { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; justify-content: flex-end; }
.header-search { width: 240px; }
.batch-toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; padding: 10px 12px; border: 1px solid #dbeafe; border-radius: 8px; background: #eff6ff; color: #1f4f8f; }
.env-table-wrap { flex: 1; min-height: 0; overflow: hidden; }
.env-table { width: 100%; }
.mobile-env-list { display: none; }
.field-tip { color: #909399; font-size: 12px; line-height: 1.6; margin-top: 6px; }
.name-with-pin { display: inline-flex; align-items: center; gap: 4px; min-width: 0; max-width: 100%; }
.name-with-pin code { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.pin-icon { color: #6366f1; flex-shrink: 0; }
.value-cell { display: flex; align-items: center; gap: 6px; min-width: 0; }
.value-text { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.eye-toggle { cursor: pointer; color: #909399; flex-shrink: 0; font-size: 15px; }
.eye-toggle:hover { color: #6366f1; }
.mobile-value-wrap { display: inline-flex; align-items: center; gap: 4px; min-width: 0; }
code { font-family: Consolas, Monaco, monospace; }

@media (max-width: 768px) {
  .script-envs-page { height: auto; min-height: 100%; }
  .page-card { height: auto; min-height: 100%; }
  .page-card :deep(.el-card__body) { overflow: visible; }
  .page-header { flex-direction: column; }
  .title { font-size: 16px; }
  .mobile-info-button { display: inline-flex; }
  .subtitle { display: none; }
  .header-actions { width: 100%; display: grid; grid-template-columns: 1fr 1fr; justify-content: stretch; }
  .header-search { width: 100%; grid-column: 1 / -1; }
  .header-actions :deep(.el-button) { width: 100%; margin-left: 0; }
  .header-actions :deep(.el-upload) { width: 100%; }
  .batch-toolbar { align-items: stretch; }
  .batch-toolbar .el-button { flex: 1 1 calc(50% - 8px); margin-left: 0; }
  .desktop-env-table { display: none; }
  .mobile-env-list { display: flex; flex-direction: column; gap: 10px; min-height: 220px; }
  .mobile-env-card { padding: 12px; border: 1px solid #ebeef5; border-radius: 8px; background: #fff; box-shadow: 0 2px 8px rgba(31, 41, 55, 0.04); }
  .mobile-env-title { display: flex; justify-content: space-between; align-items: center; gap: 10px; font-weight: 600; }
  .mobile-title-left { display: flex; align-items: center; gap: 8px; min-width: 0; flex: 1; }
  .mobile-index { flex-shrink: 0; color: #909399; font-size: 12px; }
  .mobile-env-title .name-with-pin { flex: 1; }
  .mobile-env-title code { word-break: break-word; overflow-wrap: anywhere; }
  .mobile-field-grid { margin-top: 10px; display: grid; gap: 8px; font-size: 12px; }
  .mobile-field-grid > div { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; min-width: 0; }
  .mobile-field-grid span { color: #909399; flex-shrink: 0; }
  .mobile-field-grid strong { min-width: 0; color: #303133; font-weight: 500; text-align: right; word-break: break-word; overflow-wrap: anywhere; }
  .mobile-card-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-top: 12px; padding-top: 10px; border-top: 1px solid #f0f2f5; }
  .mobile-card-actions .el-button { width: 100%; min-width: 0; margin-left: 0; }
  .script-envs-page :deep(.el-dialog) { width: 94vw !important; }
  .script-envs-page :deep(.el-form-item) { display: block; }
  .script-envs-page :deep(.el-form-item__label) { width: 100% !important; justify-content: flex-start; padding: 0 0 6px; }
  .script-envs-page :deep(.el-form-item__content) { margin-left: 0 !important; }
}
</style>
