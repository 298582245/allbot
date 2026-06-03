<template>
  <div class="keyword-replies page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <h2>关键字回复</h2>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看关键字回复说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <p>{{ pageDescription }}</p>
          </div>
          <el-button type="primary" @click="openDialog()">新增回复</el-button>
        </div>
      </template>

      <div class="table-area desktop-keyword-table">
        <el-table :data="pagedItems" border stripe height="100%">
          <el-table-column label="ID" width="90">
            <template #default="{ row }">
              <span class="id-with-pin">{{ row.id }}<span v-if="row.pinned" class="pin-icon">📌</span></span>
            </template>
          </el-table-column>
          <el-table-column label="关键字" min-width="180">
            <template #default="{ row }">
              <div class="keyword">{{ row.keyword }}</div>
              <div class="description">{{ row.description || typeName(row.reply_type) }}</div>
            </template>
          </el-table-column>
          <el-table-column label="匹配" width="100">
            <template #default="{ row }">{{ row.match_type === 'exact' ? '精确' : '正则' }}</template>
          </el-table-column>
          <el-table-column label="类型" width="100">
            <template #default="{ row }">{{ typeName(row.reply_type) }}</template>
          </el-table-column>
          <el-table-column label="权限" width="120">
            <template #default="{ row }">{{ row.admin_only ? '仅管理员' : '所有人' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="210">
            <template #default="{ row }">
              <div class="tags">
                <el-tag v-if="row.builtin" type="warning" size="small">内置</el-tag>
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <el-button size="small" @click="openDialog(row)">编辑</el-button>
              <el-button size="small" type="danger" :disabled="row.builtin" @click="deleteItem(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="mobile-keyword-table">
        <el-table :data="pagedItems" border stripe height="100%">
          <el-table-column label="回复内容" min-width="280">
            <template #default="{ row }">
              <div class="mobile-table-row-title">{{ row.keyword }}</div>
              <div class="mobile-table-row-desc">{{ row.description || typeName(row.reply_type) }}</div>
              <div class="mobile-table-fields">
                <div><span>ID</span><strong class="id-with-pin">{{ row.id }}<span v-if="row.pinned" class="pin-icon">📌</span></strong></div>
                <div><span>匹配</span><strong>{{ row.match_type === 'exact' ? '精确' : '正则' }}</strong></div>
                <div><span>类型</span><strong>{{ typeName(row.reply_type) }}</strong></div>
                <div><span>权限</span><strong>{{ row.admin_only ? '仅管理员' : '所有人' }}</strong></div>
                <div class="mobile-table-status">
                  <span>状态</span>
                  <div class="tags">
                    <el-tag v-if="row.builtin" type="warning" size="small">内置</el-tag>
                    <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
                  </div>
                </div>
              </div>
              <div class="mobile-table-actions">
                <el-button size="small" @click="openDialog(row)">编辑</el-button>
                <el-button size="small" type="danger" :disabled="row.builtin" @click="deleteItem(row)">删除</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="pagedItems.length === 0" description="暂无关键字回复" />
      </div>

      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="items.length"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
        />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑回复' : '新增回复'" width="640px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="关键字">
          <el-input v-model="form.keyword" :disabled="form.builtin" placeholder="例如：^天气.* 或 myid" />
        </el-form-item>
        <el-form-item label="匹配方式">
          <el-radio-group v-model="form.match_type" :disabled="form.builtin">
            <el-radio-button label="regex">正则</el-radio-button>
            <el-radio-button label="exact">精确</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="回复类型">
          <el-radio-group v-model="form.reply_type" :disabled="form.builtin">
            <el-radio-button label="text">文本</el-radio-button>
            <el-radio-button label="image">图片</el-radio-button>
            <el-radio-button label="audio">音频</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="回复内容">
          <el-input v-model="form.content" type="textarea" :rows="4" :disabled="form.builtin" placeholder="文本内容、图片地址或音频文件地址" />
        </el-form-item>
        <el-form-item label="开关">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="管理员指令">
          <el-switch v-model="form.admin_only" />
          <span class="hint">启用后只有平台管理员可触发</span>
        </el-form-item>
        <el-form-item label="置顶">
          <el-switch v-model="form.pinned" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveItem">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/request'

const items = ref([])
const dialogVisible = ref(false)
const page = ref(1)
const pageSize = ref(20)
const form = reactive(emptyForm())
const pageDescription = '支持正则匹配、管理员指令、置顶展示、文本/图片/音频回复。'

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '关键字回复说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

const pagedItems = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return items.value.slice(start, start + pageSize.value)
})

watch([items, pageSize], () => {
  const maxPage = Math.max(1, Math.ceil(items.value.length / pageSize.value))
  if (page.value > maxPage) page.value = maxPage
})

const loadItems = async () => {
  items.value = await request.get('/replies/keywords')
}

const openDialog = (item) => {
  Object.assign(form, emptyForm(), item || {})
  dialogVisible.value = true
}

const saveItem = async () => {
  await request.post('/replies/keywords', form)
  ElMessage.success('已保存')
  dialogVisible.value = false
  await loadItems()
}

const deleteItem = async (item) => {
  await ElMessageBox.confirm(`确定删除「${item.keyword}」吗？`, '提示', { type: 'warning' })
  await request.delete(`/replies/keywords/${item.id}`)
  ElMessage.success('已删除')
  await loadItems()
}

function emptyForm() {
  return {
    id: 0,
    keyword: '',
    match_type: 'regex',
    reply_type: 'text',
    content: '',
    enabled: true,
    admin_only: false,
    pinned: false,
    builtin: false,
    schedule_enabled: false,
    schedule_cron: '',
    description: ''
  }
}

function typeName(type) {
  if (type === 'image') return '图片'
  if (type === 'audio') return '音频'
  if (type === 'builtin') return '内置指令'
  return '文本'
}

onMounted(loadItems)
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 12px; overflow: hidden; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.title-row { display: flex; align-items: center; gap: 6px; }
.page-header h2 { margin: 0 0 6px; }
.title-row h2 { margin: 0 0 6px; }
.mobile-info-button { display: none; padding: 0; font-size: 16px; }
.page-header p { margin: 0; color: #909399; }
.table-area { flex: 1; min-height: 0; overflow: hidden; }
.mobile-keyword-table { display: none; }
.pagination-bar { display: flex; justify-content: flex-end; flex-shrink: 0; }
.keyword { font-weight: 600; }
.id-with-pin { display: inline-flex; align-items: center; gap: 2px; }
.pin-icon { font-size: 14px; line-height: 1; }
.description { margin-top: 4px; color: #909399; font-size: 12px; }
.tags { display: flex; gap: 6px; flex-wrap: wrap; }
.hint { margin-left: 12px; color: #909399; font-size: 12px; }

@media (max-width: 768px) {
  .page-shell { height: calc(100dvh - 52px - 76px - 24px); overflow: hidden; }
  .page-card { height: 100%; }
  .page-header { align-items: flex-start; flex-direction: column; }
  .mobile-info-button { display: inline-flex; }
  .page-header p { display: none; }
  .page-header > .el-button { width: 100%; margin-left: 0; }
  .desktop-keyword-table { display: none; }
  .mobile-keyword-table {
    flex: 1;
    min-height: 0;
    display: block;
    overflow: hidden;
    padding-bottom: 8px;
  }
  .mobile-keyword-table :deep(.el-table__inner-wrapper::before) { display: none; }
  .mobile-keyword-table :deep(.el-table__cell) { padding: 10px 8px; }
  .mobile-table-row-title { font-weight: 600; word-break: break-all; }
  .mobile-table-row-desc { margin-top: 4px; color: #909399; font-size: 12px; word-break: break-all; }
  .mobile-table-fields { margin-top: 8px; display: grid; gap: 6px; font-size: 12px; }
  .mobile-table-fields > div { display: flex; justify-content: space-between; gap: 10px; }
  .mobile-table-fields span { color: #909399; flex-shrink: 0; }
  .mobile-table-fields strong { color: #303133; font-weight: 500; text-align: right; }
  .mobile-table-status { align-items: center; }
  .mobile-table-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    margin-top: 10px;
    padding-top: 10px;
    border-top: 1px solid #f0f2f5;
  }
  .mobile-table-actions .el-button { width: 100%; margin-left: 0; }
  .keyword-replies :deep(.el-dialog) { width: 94vw !important; }
  .keyword-replies :deep(.el-form-item) { display: block; }
  .keyword-replies :deep(.el-form-item__label) { width: 100% !important; justify-content: flex-start; padding: 0 0 6px; }
  .keyword-replies :deep(.el-form-item__content) { margin-left: 0 !important; }
  .pagination-bar { justify-content: flex-start; overflow-x: auto; flex-shrink: 0; }
  .mobile-keyword-table::-webkit-scrollbar,
  .pagination-bar::-webkit-scrollbar { display: none; }
}
</style>
