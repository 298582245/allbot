<template>
  <div class="plugin-tasks-page page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="card-header">
          <div>
            <div class="title-row">
              <span class="title">插件任务</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看插件任务说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">记录插件进程，重复来源和消息会累计次数，可随时停止卡住的任务。</div>
          </div>
          <div class="header-actions">
            <el-button @click="settingsVisible = true">日志设置</el-button>
            <el-switch v-model="autoRefresh" active-text="自动刷新" />
            <el-button :loading="loading" @click="loadItems">刷新</el-button>
          </div>
        </div>
      </template>

      <div class="search-bar">
        <el-input v-model="keyword" clearable placeholder="搜索插件 / 用户 / 消息内容" @keyup.enter="searchItems" @clear="searchItems" />
        <el-select v-model="status" clearable placeholder="状态" @change="searchItems">
          <el-option label="运行中" value="running" />
          <el-option label="停止中" value="stopping" />
          <el-option label="已完成" value="success" />
          <el-option label="失败" value="failed" />
          <el-option label="已停止" value="stopped" />
        </el-select>
        <el-button type="primary" :loading="loading" @click="searchItems">搜索</el-button>
        <el-button @click="resetSearch">重置</el-button>
      </div>

      <div class="table-area">
        <el-table v-loading="loading" :data="items" row-key="id" stripe height="100%">
          <el-table-column prop="id" label="ID" width="74" />
          <el-table-column prop="plugin_id" label="插件" min-width="140" show-overflow-tooltip />
          <el-table-column label="触发来源" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ sourceText(row) }}</template>
          </el-table-column>
          <el-table-column prop="content" label="消息内容" min-width="220" show-overflow-tooltip />
          <el-table-column label="次数" width="80" align="center"><template #default="{ row }">{{ Number(row.run_total || 0).toLocaleString() }}</template></el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
          </el-table-column>
          <el-table-column label="开始时间" min-width="170"><template #default="{ row }">{{ formatTime(row.started_at) }}</template></el-table-column>
          <el-table-column label="结束时间" min-width="170"><template #default="{ row }">{{ isActive(row.status) ? '-' : formatTime(row.finished_at) }}</template></el-table-column>
          <el-table-column label="操作" width="235" fixed="right">
            <template #default="{ row }">
              <div class="table-actions">
                <el-button size="small" @click="openLog(row)">日志</el-button>
                <el-button size="small" type="warning" :disabled="!isActive(row.status)" :loading="stoppingId === row.id" @click="stopItem(row)">停止</el-button>
                <el-button size="small" type="danger" :loading="deletingId === row.id" @click="deleteItem(row)">删除</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loading && items.length === 0" description="暂无插件任务" />
      </div>

      <StdPagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10, 20, 50, 100]" @current-change="loadItems" @size-change="handlePageSizeChange" />
    </el-card>

    <el-dialog v-model="settingsVisible" title="插件任务日志设置" width="480px" destroy-on-close>
      <el-form label-width="120px">
        <el-form-item label="日志保留天数">
          <el-input-number v-model="retentionDays" :min="0" :max="3650" controls-position="right" />
          <div class="form-tip">0 表示不自动清理。</div>
        </el-form-item>
        <el-form-item label="运行超时秒数">
          <el-input-number v-model="runTimeoutSeconds" :min="0" :max="86400" :step="60" controls-position="right" />
          <div class="form-tip">0 表示不限制。</div>
        </el-form-item>
        <el-form-item label="超时通知管理员">
          <el-switch v-model="timeoutNotifyAdminEnabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="settingsVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingSettings" @click="saveSettings">保存设置</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="logVisible" :title="logTitle" width="78%" top="5vh" destroy-on-close>
      <div v-loading="logLoading" class="log-dialog-body">
        <div v-if="currentLog" class="detail-meta">
          <el-tag size="small" :type="statusType(currentLog.status)">{{ statusText(currentLog.status) }}</el-tag>
          <span>插件：{{ currentLog.plugin_id || '-' }}</span>
          <span>来源：{{ sourceText(currentLog) }}</span>
          <span>消息：{{ currentLog.message_id || '-' }}</span>
          <span>次数：{{ Number(currentLog.run_total || 0).toLocaleString() }}</span>
        </div>
        <div class="log-section"><div class="log-title">插件输出</div><pre>{{ currentLog?.output || '暂无输出' }}</pre></div>
        <div v-if="currentLog?.error" class="log-section"><div class="log-title">错误信息</div><pre class="error-log">{{ currentLog.error }}</pre></div>
      </div>
      <template #footer>
        <el-button @click="logVisible = false">关闭</el-button>
        <el-button :loading="logLoading" @click="refreshLog">刷新日志</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
defineOptions({ name: 'PluginTasks' })
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/request'
import StdPagination from '@/components/StdPagination.vue'

const loading = ref(false)
const logLoading = ref(false)
const savingSettings = ref(false)
const stoppingId = ref(0)
const deletingId = ref(0)
const autoRefresh = ref(true)
const items = ref([])
const keyword = ref('')
const status = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const retentionDays = ref(0)
const runTimeoutSeconds = ref(0)
const timeoutNotifyAdminEnabled = ref(false)
const settingsVisible = ref(false)
const currentLog = ref(null)
const logVisible = ref(false)
let refreshTimer = 0

const logTitle = computed(() => currentLog.value ? `插件任务日志 #${currentLog.value.id}` : '插件任务日志')
const pageDescription = '插件任务会记录来源和消息内容；相同任务在上次完成后会累计次数，未完成或手动停止的任务会分开记录。插件没有正常结束时，可以从这里停止对应进程；日志会按保留天数自动清理。'

const loadItems = async () => {
  loading.value = true
  try {
    const params = new URLSearchParams({ page: String(page.value), page_size: String(pageSize.value) })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    if (status.value) params.set('status', status.value)
    const result = await request.get(`/plugin-tasks?${params.toString()}`)
    items.value = Array.isArray(result) ? result : (result.items || [])
    total.value = Array.isArray(result) ? items.value.length : Number(result.total || 0)
    if (!settingsVisible.value && !savingSettings.value && !Array.isArray(result)) {
      retentionDays.value = Number(result.settings?.retention_days) || 0
      runTimeoutSeconds.value = Number(result.settings?.run_timeout_seconds) || 0
      timeoutNotifyAdminEnabled.value = Boolean(result.settings?.timeout_notify_admin_enabled)
    }
  } finally { loading.value = false }
}
const searchItems = async () => { page.value = 1; await loadItems() }
const resetSearch = async () => { keyword.value = ''; status.value = ''; await searchItems() }
const handlePageSizeChange = async () => { page.value = 1; await loadItems() }
const saveSettings = async () => {
  savingSettings.value = true
  try {
    const result = await request.post('/plugin-tasks?action=settings', {
      retention_days: Number(retentionDays.value) || 0,
      run_timeout_seconds: Number(runTimeoutSeconds.value) || 0,
      timeout_notify_admin_enabled: Boolean(timeoutNotifyAdminEnabled.value),
    })
    ElMessage.success(`${result.message || '插件任务设置已保存'}，已清理 ${result.removed || 0} 条`)
    settingsVisible.value = false
    await loadItems()
  } finally { savingSettings.value = false }
}
const openLog = async (item) => { currentLog.value = { ...item, output: '' }; logVisible.value = true; await refreshLog() }
const refreshLog = async () => {
  if (!currentLog.value?.id) return
  logLoading.value = true
  try { currentLog.value = await request.get(`/plugin-tasks/${currentLog.value.id}`) } finally { logLoading.value = false }
}
const stopItem = async (item) => {
  await ElMessageBox.confirm(`确定停止插件任务 #${item.id} 吗？`, '停止确认', { type: 'warning' })
  stoppingId.value = item.id
  try { const result = await request.put(`/plugin-tasks/${item.id}?action=stop`); ElMessage.success(result.message || '插件任务停止请求已发送'); await loadItems() } finally { stoppingId.value = 0 }
}
const deleteItem = async (item) => {
  await ElMessageBox.confirm(`确定删除插件任务 #${item.id} 吗？运行中的任务会先尝试停止。`, '删除确认', { type: 'warning' })
  deletingId.value = item.id
  try { const result = await request.delete(`/plugin-tasks/${item.id}`); ElMessage.success(result.message || '插件任务已删除'); await loadItems() } finally { deletingId.value = 0 }
}
const showPageDescription = () => ElMessageBox.alert(pageDescription, '插件任务说明', { confirmButtonText: '知道了', type: 'info' })
const isActive = (value) => ['running', 'stopping'].includes(value)
const statusText = (value) => ({ running: '运行中', stopping: '停止中', success: '已完成', failed: '失败', stopped: '已停止' }[value] || value || '-')
const statusType = (value) => ({ running: 'primary', stopping: 'warning', success: 'success', failed: 'danger', stopped: 'info' }[value] || 'info')
const sourceText = (row) => [row.platform, row.user_id || row.group_id].filter(Boolean).join(' / ') || '-'
const formatTime = (value) => { if (!value) return '-'; const date = new Date(value); return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString() }
watch(autoRefresh, (enabled) => { stopTimer(); if (enabled) startTimer() })
const startTimer = () => { refreshTimer = window.setInterval(async () => { if (!loading.value) await loadItems(); if (logVisible.value && isActive(currentLog.value?.status) && !logLoading.value) await refreshLog() }, 5000) }
const stopTimer = () => { if (refreshTimer) window.clearInterval(refreshTimer); refreshTimer = 0 }
onMounted(async () => { await loadItems(); if (autoRefresh.value) startTimer() })
onBeforeUnmount(stopTimer)
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 14px; overflow: hidden; }
.plugin-tasks-page { padding: 0; }
.card-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.title-row { display: flex; align-items: center; gap: 6px; }
.title { font-size: 18px; font-weight: 650; }
.subtitle { color: var(--el-text-color-secondary); font-size: 13px; margin-top: 8px; }
.header-actions, .table-actions { display: flex; align-items: center; gap: 8px; }
.search-bar { display: flex; gap: 10px; flex-wrap: wrap; }
.search-bar .el-input { width: min(360px, 100%); }
.search-bar .el-select { width: 150px; }
.table-area { flex: 1; min-height: 260px; overflow: auto; }
.detail-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; color: var(--el-text-color-secondary); font-size: 13px; margin-bottom: 16px; }
.log-section { margin-top: 14px; }
.log-title { font-weight: 600; margin-bottom: 8px; }
pre { max-height: 45vh; overflow: auto; padding: 14px; margin: 0; border-radius: 6px; background: #111827; color: #d1fae5; white-space: pre-wrap; word-break: break-word; font: 12px/1.6 ui-monospace, SFMono-Regular, Consolas, monospace; }
.error-log { color: #fecaca; }
.form-tip { color: var(--el-text-color-secondary); font-size: 12px; margin-left: 12px; }
.mobile-info-button { display: none; }
@media (max-width: 760px) {
  .card-header { flex-direction: column; }
  .header-actions { width: 100%; justify-content: space-between; }
  .subtitle { display: none; }
  .mobile-info-button { display: inline-flex; }
  .search-bar .el-input, .search-bar .el-select { width: 100%; }
  .table-area :deep(.el-table) { min-width: 900px; }
}
</style>
