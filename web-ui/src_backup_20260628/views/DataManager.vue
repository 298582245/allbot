<template>
  <div class="data-manager">
    <el-card class="data-card">
      <template #header>
        <div class="card-header">
          <div>
            <div class="title-row">
              <span>数据管理</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看数据管理说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <p>{{ pageDescription }}</p>
          </div>
          <div class="header-actions">
            <el-button @click="loadTables">刷新</el-button>
            <el-button @click="exportData()">导出全部</el-button>
            <el-upload :show-file-list="false" :before-upload="handleImportFile">
              <el-button type="warning">导入 JSON</el-button>
            </el-upload>
          </div>
        </div>
      </template>

      <el-alert class="data-tip" title="这里会直接修改数据库。删除整张表会让相关功能不可用，谨慎操作。" type="warning" show-icon :closable="false" />

      <div class="content desktop-data-view">
        <el-card class="table-list" shadow="never">
          <div class="table-list-title">数据表分类</div>
          <el-input v-model="tableSearch" class="table-search" clearable placeholder="搜索表名或视图名称" />
          <el-empty v-if="groupedTables.length === 0" description="没有匹配的数据表" />
          <el-collapse v-model="activeGroups">
            <el-collapse-item v-for="group in groupedTables" :key="group.name" :title="`${group.name}（${group.tables.length}）`" :name="group.name">
              <el-menu :default-active="selectedTable" @select="selectTable">
                <el-menu-item v-for="table in group.tables" :key="table.name" :index="table.name">
                  <span>{{ table.view_name || table.name }}</span>
                  <el-tag size="small" type="info">{{ table.count }}</el-tag>
                </el-menu-item>
              </el-menu>
            </el-collapse-item>
          </el-collapse>
        </el-card>

        <el-card class="table-content" shadow="never">
          <template #header>
            <div class="table-toolbar">
              <div>
                <strong>{{ selectedTableInfo?.view_name || selectedTable || '请选择数据表' }}</strong>
                <span v-if="selectedTable" class="muted">共 {{ total }} 行</span>
                <span v-if="selectedTableInfo?.description" class="muted">{{ selectedTableInfo.description }}</span>
              </div>
              <div v-if="selectedTable" class="row-actions">
                <el-button type="primary" @click="openEditDialog()">新增行</el-button>
                <el-button @click="exportData(selectedTable)">导出当前表</el-button>
                <el-button @click="openRenameDialog">表改名</el-button>
                <el-button type="warning" @click="clearTable">仅清空数据</el-button>
                <el-button type="danger" @click="dropTable">删除整张表</el-button>
              </div>
            </div>
          </template>

          <div class="table-content-body">
            <el-empty v-if="!selectedTable" description="请选择左侧数据表" />
            <template v-else>
              <el-input v-model="rowSearch" class="row-search" clearable placeholder="搜索表名、视图名称或表内数据" />
              <div class="table-scroll">
                <el-table :data="rows" v-loading="loadingRows" border height="100%">
              <el-table-column fixed="left" prop="__rowid__" label="行ID" width="90" />
              <el-table-column v-for="column in columns" :key="column.name" :prop="column.name" :label="column.name" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">
                  <span>{{ displayValue(row[column.name]) }}</span>
                </template>
              </el-table-column>
              <el-table-column fixed="right" label="操作" width="150">
                <template #default="{ row }">
                  <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
                  <el-button size="small" type="danger" @click="deleteRow(row)">删除</el-button>
                </template>
              </el-table-column>
                </el-table>
              </div>

              <div class="pagination-wrapper">
                <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :page-sizes="[10, 20, 50, 100]" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadRows" @size-change="handleSizeChange" />
              </div>
            </template>
          </div>
        </el-card>
      </div>

      <div class="mobile-data-view">
        <el-card class="mobile-selector-card" shadow="never">
          <div class="mobile-section-title">选择数据表</div>
          <el-input v-model="tableSearch" class="table-search" clearable placeholder="搜索表名或视图名称" />
          <el-select v-model="selectedTable" filterable placeholder="请选择数据表" style="width: 100%" @change="selectTable">
            <el-option-group v-for="group in groupedTables" :key="group.name" :label="`${group.name}（${group.tables.length}）`">
              <el-option
                v-for="table in group.tables"
                :key="table.name"
                :label="`${table.view_name || table.name}（${table.count}）`"
                :value="table.name"
              />
            </el-option-group>
          </el-select>
        </el-card>

        <el-card class="mobile-table-card" shadow="never">
          <div class="mobile-table-header">
            <div>
              <strong>{{ selectedTableInfo?.view_name || selectedTable || '请选择数据表' }}</strong>
              <span v-if="selectedTable" class="muted">共 {{ total }} 行</span>
              <span v-if="selectedTableInfo?.description" class="muted">{{ selectedTableInfo.description }}</span>
            </div>
          </div>

          <div v-if="selectedTable" class="mobile-actions">
            <el-button type="primary" @click="openEditDialog()">新增行</el-button>
            <el-button @click="exportData(selectedTable)">导出当前表</el-button>
            <el-button @click="openRenameDialog">表改名</el-button>
            <el-button type="warning" @click="clearTable">清空数据</el-button>
            <el-button type="danger" @click="dropTable">删除表</el-button>
          </div>
          <el-input v-if="selectedTable" v-model="rowSearch" class="row-search" clearable placeholder="搜索表名、视图名称或表内数据" />

          <el-empty v-if="!selectedTable" description="请选择数据表" />
          <div v-else v-loading="loadingRows" class="mobile-row-list">
            <el-empty v-if="rows.length === 0 && !loadingRows" description="暂无数据" />
            <div v-for="row in rows" :key="row.__rowid__" class="mobile-row-card">
              <div class="mobile-row-card-header">
                <span>行 ID：{{ row.__rowid__ }}</span>
                <div>
                  <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
                  <el-button size="small" type="danger" @click="deleteRow(row)">删除</el-button>
                </div>
              </div>
              <div class="mobile-field-list">
                <div v-for="column in columns" :key="column.name" class="mobile-field-item">
                  <div class="mobile-field-name">{{ column.name }}</div>
                  <div class="mobile-field-value">{{ displayValue(row[column.name]) || '空' }}</div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="selectedTable" class="mobile-pagination">
            <el-pagination
              v-model:current-page="page"
              :page-size="pageSize"
              :total="total"
              small
              layout="prev, pager, next"
              @current-change="loadRows"
            />
          </div>
        </el-card>
      </div>
    </el-card>

    <el-dialog v-model="editDialogVisible" :title="editingRow ? '编辑行' : '新增行'" width="720px">
      <el-form label-width="140px">
        <el-form-item v-for="column in columns" :key="column.name" :label="columnLabel(column)">
          <el-input v-model="editForm[column.name]" :type="isLongText(column) ? 'textarea' : 'text'" :autosize="{ minRows: 2, maxRows: 8 }" :placeholder="column.type || 'TEXT'" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveRow" :loading="saving">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="renameDialogVisible" title="表改名" width="420px">
      <el-form label-width="90px">
        <el-form-item label="当前表名">
          <el-input :model-value="selectedTable" disabled />
        </el-form-item>
        <el-form-item label="新表名">
          <el-input v-model.trim="renameForm.name" placeholder="请输入新的表名" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="renameTable">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="导入数据" width="520px">
      <el-alert title="导入文件必须是本页导出的 JSON 格式。" type="info" show-icon :closable="false" />
      <el-checkbox v-model="replaceImport" class="replace-check">导入前清空对应表</el-checkbox>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="warning" @click="confirmImport" :loading="importing">开始导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/request'

const tables = ref([])
const tableSearch = ref('')
const rowSearch = ref('')
const activeGroups = ref([])
const selectedTable = ref('')
const columns = ref([])
const rows = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loadingRows = ref(false)
const editDialogVisible = ref(false)
const editingRow = ref(null)
const editForm = ref({})
const saving = ref(false)
const importDialogVisible = ref(false)
const importContent = ref('')
const replaceImport = ref(false)
const importing = ref(false)
const renameDialogVisible = ref(false)
const renameForm = ref({ name: '' })
const pageDescription = '可查看、编辑、导入、导出数据库表，也可以重命名、清空或删除表。'

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '数据管理说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

const groupedTables = computed(() => {
  const keyword = tableSearch.value.trim().toLowerCase()
  const groups = new Map()
  for (const table of tables.value) {
    const groupName = table.group || '业务数据'
    if (keyword && !tableMatchesSearch(table, groupName, keyword)) continue
    if (!groups.has(groupName)) groups.set(groupName, [])
    groups.get(groupName).push(table)
  }
  return Array.from(groups.entries()).map(([name, groupTables]) => ({ name, tables: groupTables }))
})

const tableMatchesSearch = (table, groupName, keyword) => {
  return [table.name, table.view_name, groupName]
    .some(value => String(value || '').toLowerCase().includes(keyword))
}

watch(tableSearch, () => {
  activeGroups.value = groupedTables.value.map(group => group.name)
})

watch(rowSearch, async () => {
  page.value = 1
  await loadRows()
})

const selectedTableInfo = computed(() => tables.value.find(table => table.name === selectedTable.value))

const loadTables = async () => {
  const data = await request.get('/data/tables')
  tables.value = data
  activeGroups.value = [...new Set(data.map(table => table.group || '业务数据'))]
  if (selectedTable.value && !data.some(table => table.name === selectedTable.value)) {
    selectedTable.value = ''
  }
  if (!selectedTable.value && data.length > 0) {
    selectedTable.value = data[0].name
    await loadRows()
  }
}

const selectTable = async (table) => {
  selectedTable.value = table
  page.value = 1
  await loadRows()
}

const loadRows = async () => {
  if (!selectedTable.value) return
  loadingRows.value = true
  try {
    const params = { page: page.value, size: pageSize.value }
    const search = rowSearch.value.trim()
    if (search) params.search = search
    const data = await request.get(`/data/tables/${selectedTable.value}/rows`, { params })
    columns.value = data.columns || []
    rows.value = data.rows || []
    total.value = data.total || 0
  } finally {
    loadingRows.value = false
  }
}

const handleSizeChange = async () => {
  page.value = 1
  await loadRows()
}

const openEditDialog = (row = null) => {
  editingRow.value = row
  editForm.value = {}
  for (const column of columns.value) editForm.value[column.name] = row ? normalizeEditValue(row[column.name]) : ''
  editDialogVisible.value = true
}

const saveRow = async () => {
  saving.value = true
  try {
    const values = { ...editForm.value }
    if (editingRow.value) {
      await request.put(`/data/tables/${selectedTable.value}/rows/${editingRow.value.__rowid__}`, { values })
      ElMessage.success('更新成功')
    } else {
      await request.post(`/data/tables/${selectedTable.value}/rows`, { values })
      ElMessage.success('新增成功')
    }
    editDialogVisible.value = false
    await loadRows()
    await loadTables()
  } finally {
    saving.value = false
  }
}

const deleteRow = async (row) => {
  await ElMessageBox.confirm(`确定删除行 ${row.__rowid__} 吗？`, '警告', { type: 'warning' })
  await request.delete(`/data/tables/${selectedTable.value}/rows/${row.__rowid__}`)
  ElMessage.success('删除成功')
  await loadRows()
  await loadTables()
}

const openRenameDialog = () => {
  renameForm.value.name = selectedTable.value
  renameDialogVisible.value = true
}

const renameTable = async () => {
  await request.post(`/data/tables/${selectedTable.value}/rename`, { name: renameForm.value.name })
  ElMessage.success('表已重命名')
  selectedTable.value = renameForm.value.name
  renameDialogVisible.value = false
  await loadTables()
  await loadRows()
}

const clearTable = async () => {
  await ElMessageBox.confirm(`确定只清空表「${selectedTable.value}」的数据吗？表结构会保留。`, '警告', { type: 'warning' })
  await request.post(`/data/tables/${selectedTable.value}/clear`)
  ElMessage.success('表数据已清空')
  page.value = 1
  await loadRows()
  await loadTables()
}

const dropTable = async () => {
  await ElMessageBox.confirm(`确定删除整张表「${selectedTable.value}」吗？表结构和数据都会删除。`, '危险操作', { type: 'error' })
  await request.delete(`/data/tables/${selectedTable.value}`)
  ElMessage.success('表已删除')
  selectedTable.value = ''
  rows.value = []
  columns.value = []
  total.value = 0
  await loadTables()
}

const exportData = async (table = '') => {
  const response = await request.get('/data/export', { params: table ? { table } : {}, responseType: 'blob', transformResponse: [(data) => data] })
  const blob = new Blob([response], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${table || 'allbot'}-data.json`
  link.click()
  URL.revokeObjectURL(url)
}

const handleImportFile = (file) => {
  const reader = new FileReader()
  reader.onload = () => {
    importContent.value = String(reader.result || '')
    replaceImport.value = false
    importDialogVisible.value = true
  }
  reader.readAsText(file, 'utf-8')
  return false
}

const confirmImport = async () => {
  importing.value = true
  try {
    await request.post('/data/import', importContent.value, { params: { replace: replaceImport.value }, headers: { 'Content-Type': 'application/json' } })
    ElMessage.success('导入成功')
    importDialogVisible.value = false
    await loadTables()
    await loadRows()
  } finally {
    importing.value = false
  }
}

const displayValue = (value) => {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

const normalizeEditValue = (value) => {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

const columnLabel = (column) => `${column.name}${column.primary_key ? '（主键）' : ''}`

const isLongText = (column) => {
  const type = String(column.type || '').toUpperCase()
  return type.includes('TEXT') || type.includes('JSON') || column.name.includes('config')
}

onMounted(loadTables)
</script>

<style scoped>
.data-manager {
  width: 100%;
  height: 100%;
}

.data-card {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.data-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.card-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 18px;
  font-weight: 600;
}

.mobile-info-button {
  display: none;
  padding: 0;
  font-size: 16px;
}

.card-header p {
  margin: 6px 0 0;
  color: #666;
  font-size: 13px;
}

.header-actions,
.row-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.data-tip {
  margin-bottom: 16px;
}

.content {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 260px 1fr;
  gap: 16px;
}

.mobile-data-view {
  display: none;
}

.table-list,
.table-content {
  min-height: 0;
  overflow: hidden;
}

.table-list :deep(.el-card__body) {
  height: 100%;
  overflow: auto;
}

.table-content :deep(.el-card__body) {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.table-content-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.table-scroll {
  flex: 1;
  min-height: 0;
}

.table-list-title {
  font-weight: 600;
  margin-bottom: 12px;
}

.table-search,
.row-search {
  margin-bottom: 12px;
}

.row-search {
  flex-shrink: 0;
}

.table-list :deep(.el-menu) {
  border-right: none;
}

.table-list :deep(.el-menu-item) {
  display: flex;
  justify-content: space-between;
}

.table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.muted {
  margin-left: 10px;
  color: #909399;
  font-size: 13px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  flex-shrink: 0;
  padding-top: 16px;
}

.replace-check {
  margin-top: 16px;
}

@media (max-width: 768px) {
  .data-manager {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow-y: auto;
    overflow-x: hidden;
    padding-right: 2px;
  }

  .data-card {
    height: auto;
    min-height: 100%;
    border-radius: 10px;
  }

  .data-card :deep(.el-card__body) {
    min-height: auto;
    gap: 12px;
    overflow: visible;
  }

  .card-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
  }

  .title-row {
    font-size: 16px;
  }

  .mobile-info-button {
    display: inline-flex;
  }

  .card-header p {
    display: none;
    font-size: 12px;
    line-height: 1.5;
  }

  .header-actions,
  .row-actions {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .header-actions .el-button,
  .row-actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .header-actions :deep(.el-upload) {
    width: 100%;
  }

  .header-actions :deep(.el-upload .el-button) {
    width: 100%;
  }

  .data-tip {
    margin-bottom: 0;
  }

  .data-tip :deep(.el-alert__title) {
    line-height: 1.5;
  }

  .desktop-data-view {
    display: none;
  }

  .mobile-data-view {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .mobile-section-title {
    margin-bottom: 8px;
    font-weight: 600;
    color: #303133;
  }

  .mobile-selector-card,
  .mobile-table-card {
    border-radius: 10px;
  }

  .mobile-selector-card :deep(.el-card__body),
  .mobile-table-card :deep(.el-card__body) {
    padding: 12px;
  }

  .table-search {
    margin-bottom: 8px;
  }

  .mobile-table-header {
    margin-bottom: 10px;
  }

  .mobile-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    margin-bottom: 12px;
  }

  .mobile-actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .mobile-actions .el-button:last-child {
    grid-column: span 2;
  }

  .mobile-row-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-height: 180px;
  }

  .mobile-row-card {
    border: 1px solid #ebeef5;
    border-radius: 10px;
    background: #fff;
    overflow: hidden;
  }

  .mobile-row-card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 10px;
    background: #f7f8fa;
    color: #606266;
    font-size: 13px;
  }

  .mobile-row-card-header .el-button {
    margin-left: 4px;
  }

  .mobile-field-list {
    padding: 4px 10px 10px;
  }

  .mobile-field-item {
    padding: 8px 0;
    border-bottom: 1px solid #f0f2f5;
  }

  .mobile-field-item:last-child {
    border-bottom: none;
  }

  .mobile-field-name {
    margin-bottom: 4px;
    color: #909399;
    font-size: 12px;
    word-break: break-all;
  }

  .mobile-field-value {
    color: #303133;
    font-size: 13px;
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .mobile-pagination {
    display: flex;
    justify-content: center;
    padding-top: 12px;
    overflow-x: auto;
  }

  .mobile-pagination::-webkit-scrollbar,
  .data-manager::-webkit-scrollbar {
    display: none;
  }

  .muted {
    display: block;
    margin-left: 0;
    margin-top: 4px;
  }
}
</style>
