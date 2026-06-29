<template>
  <div class="script-envs-page">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title">脚本环境变量</div>
            <div class="subtitle">集中维护脚本运行时可注入的额外环境变量，插件需在配置中开启后才会读取。</div>
          </div>
          <div class="header-actions">
            <el-input v-model="searchKeyword" class="header-search" clearable placeholder="搜索变量名或备注" />
            <el-button :loading="loading" @click="loadItems">刷新</el-button>
            <el-button type="primary" @click="createItem">
              <el-icon><Plus /></el-icon>
              新增变量
            </el-button>
          </div>
        </div>
      </template>

      <div v-loading="loading" class="env-table-wrap desktop-env-table">
        <el-table :data="pagedItems" row-key="id" stripe border height="100%" class="env-table">
          <el-table-column prop="name" label="变量名" min-width="180" show-overflow-tooltip>
            <template #default="{ row }"><code>{{ row.name }}</code></template>
          </el-table-column>
          <el-table-column label="变量值" min-width="220" show-overflow-tooltip>
            <template #default="{ row }">{{ maskValue(row.value) }}</template>
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
            <code>{{ row.name }}</code>
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
          </div>
          <div class="mobile-field-grid">
            <div><span>变量值</span><strong>{{ maskValue(row.value) }}</strong></div>
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

      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="filteredItems.length"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handlePageSizeChange"
        />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogMode === 'create' ? '新增脚本环境变量' : '编辑脚本环境变量'" width="560px">
      <el-form :model="form" label-width="96px">
        <el-form-item label="变量名" required>
          <el-input v-model="form.name" placeholder="例如 API_TOKEN" />
          <div class="field-tip">脚本中通过 process.env.API_TOKEN 或 os.getenv('API_TOKEN') 读取。</div>
        </el-form-item>
        <el-form-item label="变量值">
          <el-input v-model="form.value" type="textarea" :rows="4" show-word-limit maxlength="10000" placeholder="变量值，支持多行" />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" maxlength="160" show-word-limit placeholder="说明用途" />
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { createScriptEnv, deleteScriptEnv, getScriptEnvs, updateScriptEnv } from '@/api'

const loading = ref(false)
const saving = ref(false)
const deletingId = ref(0)
const items = ref([])
const searchKeyword = ref('')
const dialogVisible = ref(false)
const dialogMode = ref('create')
const editingId = ref(0)
const page = ref(1)
const pageSize = ref(20)
const form = reactive(createEmptyForm())

const filteredItems = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  if (!keyword) return items.value
  return items.value.filter((row) => [row.name, row.remark].filter(Boolean).join(' ').toLowerCase().includes(keyword))
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

function buildPayload() {
  return {
    name: String(form.name || '').trim(),
    value: String(form.value || ''),
    remark: String(form.remark || '').trim(),
    enabled: Boolean(form.enabled)
  }
}

function createEmptyForm() {
  return { name: '', value: '', remark: '', enabled: true }
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
.subtitle { margin-top: 6px; color: #666; font-size: 13px; }
.header-actions { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; justify-content: flex-end; }
.header-search { width: 240px; }
.env-table-wrap { flex: 1; min-height: 0; overflow: hidden; }
.env-table { width: 100%; }
.mobile-env-list { display: none; }
.pagination-bar { display: flex; justify-content: flex-end; flex-shrink: 0; min-width: 0; }
.pagination-bar :deep(.el-pagination) { max-width: 100%; }
.field-tip { color: #909399; font-size: 12px; line-height: 1.6; margin-top: 6px; }
code { font-family: Consolas, Monaco, monospace; }

@media (max-width: 768px) {
  .script-envs-page { height: auto; min-height: 100%; }
  .page-card { height: auto; min-height: 100%; }
  .page-card :deep(.el-card__body) { overflow: visible; }
  .page-header { flex-direction: column; }
  .subtitle { display: none; }
  .header-actions { width: 100%; display: grid; grid-template-columns: 1fr 1fr; justify-content: stretch; }
  .header-search { width: 100%; grid-column: 1 / -1; }
  .header-actions :deep(.el-button) { width: 100%; margin-left: 0; }
  .desktop-env-table { display: none; }
  .mobile-env-list { display: flex; flex-direction: column; gap: 10px; min-height: 220px; }
  .mobile-env-card { padding: 12px; border: 1px solid #ebeef5; border-radius: 8px; background: #fff; box-shadow: 0 2px 8px rgba(31, 41, 55, 0.04); }
  .mobile-env-title { display: flex; justify-content: space-between; align-items: center; gap: 10px; font-weight: 600; }
  .mobile-env-title code { min-width: 0; word-break: break-word; overflow-wrap: anywhere; }
  .mobile-field-grid { margin-top: 10px; display: grid; gap: 8px; font-size: 12px; }
  .mobile-field-grid > div { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; min-width: 0; }
  .mobile-field-grid span { color: #909399; flex-shrink: 0; }
  .mobile-field-grid strong { min-width: 0; color: #303133; font-weight: 500; text-align: right; word-break: break-word; overflow-wrap: anywhere; }
  .mobile-card-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-top: 12px; padding-top: 10px; border-top: 1px solid #f0f2f5; }
  .mobile-card-actions .el-button { width: 100%; min-width: 0; margin-left: 0; }
  .pagination-bar { justify-content: flex-start; width: 100%; overflow-x: auto; padding-bottom: 2px; }
  .pagination-bar :deep(.el-pagination) { flex-wrap: nowrap; min-width: max-content; }
  .pagination-bar::-webkit-scrollbar { display: none; }
  .script-envs-page :deep(.el-dialog) { width: 94vw !important; }
  .script-envs-page :deep(.el-form-item) { display: block; }
  .script-envs-page :deep(.el-form-item__label) { width: 100% !important; justify-content: flex-start; padding: 0 0 6px; }
  .script-envs-page :deep(.el-form-item__content) { margin-left: 0 !important; }
}
</style>
