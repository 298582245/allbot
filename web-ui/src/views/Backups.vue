<template>
  <div class="backups page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <h2>备份中心</h2>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看备份中心说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <p>{{ pageDescription }}</p>
          </div>
          <div class="header-actions">
            <el-button :loading="loading" @click="loadOverview">刷新</el-button>
            <el-button type="primary" :loading="creating" @click="handleCreateBackup">立即备份</el-button>
          </div>
        </div>
      </template>

      <div class="backup-content" v-loading="loading">
        <section class="form-section">
          <div class="section-header">
            <div>
              <div class="section-title">备份配置</div>
              <div class="section-desc">定时表达式支持 5 位或 6 位 cron，路径按当前系统自动处理。</div>
            </div>
            <el-button type="primary" :loading="saving" @click="handleSaveSettings">保存配置</el-button>
          </div>

          <el-form class="backup-form" :model="form" label-width="120px">
            <el-form-item label="定时备份">
              <el-switch v-model="form.enabled" />
              <span class="hint">{{ form.enabled ? '按 cron 自动备份' : '仅允许手动备份' }}</span>
            </el-form-item>
            <el-form-item label="Cron 表达式">
              <el-input v-model="form.cron" placeholder="0 3 * * *" />
            </el-form-item>
            <el-form-item label="备份目录">
              <el-input v-model="form.backup_dir" placeholder="./backups" />
            </el-form-item>
            <el-form-item label="保留最新">
              <el-input-number v-model="form.retention" :min="1" :max="365" />
              <span class="hint">超过数量后自动删除最旧备份</span>
            </el-form-item>
            <el-form-item label="备份内容">
              <el-checkbox v-model="form.include_plugins">插件目录</el-checkbox>
              <el-checkbox v-model="form.include_data">数据与 OpenAPI</el-checkbox>
            </el-form-item>
          </el-form>
        </section>

        <section class="form-section">
          <div class="section-title">运行状态</div>
          <div class="info-grid">
            <div class="info-item">
              <span>定时状态</span>
              <strong>{{ form.enabled ? '已启用' : '未启用' }}</strong>
            </div>
            <div class="info-item">
              <span>下次执行</span>
              <strong>{{ formatTime(status.next_run_at) }}</strong>
            </div>
            <div class="info-item">
              <span>保留数量</span>
              <strong>{{ form.retention }} 份</strong>
            </div>
            <div class="info-item">
              <span>本地备份</span>
              <strong>{{ files.length }} 份</strong>
            </div>
            <div v-if="status.last_error" class="info-item wide">
              <span>最近错误</span>
              <strong class="error-text">{{ status.last_error }}</strong>
            </div>
          </div>
        </section>

        <section class="form-section file-section">
          <div class="section-title">备份文件</div>
          <el-table :data="files" empty-text="暂无备份文件">
            <el-table-column prop="name" label="文件名" min-width="260" />
            <el-table-column label="大小" width="120">
              <template #default="{ row }">{{ formatSize(row.size) }}</template>
            </el-table-column>
            <el-table-column label="创建时间" width="190">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="180" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" link @click="downloadBackup(row)">下载</el-button>
                <el-button type="danger" link @click="handleDeleteBackup(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </section>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createBackup, deleteBackup, getBackups, saveBackupSettings } from '@/api'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const loading = ref(false)
const saving = ref(false)
const creating = ref(false)
const files = ref([])
const status = reactive({ running: false, next_run_at: '', last_error: '' })
const form = reactive(createDefaultSettings())
const pageDescription = '备份插件目录、OpenAPI 文件和配置数据，支持定时执行与自动清理旧备份。'

onMounted(() => {
  loadOverview()
})

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '备份中心说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

const loadOverview = async () => {
  loading.value = true
  try {
    const data = await getBackups()
    Object.assign(form, normalizeSettings(data.settings))
    Object.assign(status, data.status || {})
    files.value = Array.isArray(data.files) ? data.files : []
  } finally {
    loading.value = false
  }
}

const handleSaveSettings = async () => {
  if (!form.include_plugins && !form.include_data) {
    ElMessage.error('至少需要选择插件目录或数据')
    return
  }
  saving.value = true
  try {
    const data = await saveBackupSettings(normalizeSettings(form))
    Object.assign(form, normalizeSettings(data.settings))
    Object.assign(status, data.status || {})
    ElMessage.success('备份配置已保存')
  } finally {
    saving.value = false
  }
}

const handleCreateBackup = async () => {
  creating.value = true
  try {
    await createBackup()
    ElMessage.success('备份创建成功')
    await loadOverview()
  } finally {
    creating.value = false
  }
}

const handleDeleteBackup = async (row) => {
  await ElMessageBox.confirm(`确定删除备份 ${row.name} 吗？`, '删除备份', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'warning'
  })
  await deleteBackup(row.name)
  ElMessage.success('备份已删除')
  await loadOverview()
}

const downloadBackup = async (row) => {
  const response = await fetch(`/api/backups/${encodeURIComponent(row.name)}/download`, {
    headers: { Authorization: `Bearer ${authStore.token}` }
  })
  if (!response.ok) {
    ElMessage.error('下载失败')
    return
  }
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = row.name
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function createDefaultSettings() {
  return {
    enabled: false,
    cron: '0 3 * * *',
    retention: 7,
    backup_dir: './backups',
    include_plugins: true,
    include_data: true,
    oss: { enabled: false, provider: '', bucket: '', endpoint: '', prefix: 'allbot/' }
  }
}

function normalizeSettings(value = {}) {
  const defaults = createDefaultSettings()
  return {
    ...defaults,
    ...value,
    oss: { ...defaults.oss, ...(value.oss || {}) }
  }
}

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

function formatSize(size) {
  const value = Number(size) || 0
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
}
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.title-row { display: flex; align-items: center; gap: 6px; }
.page-header h2 { margin: 0 0 6px; }
.title-row h2 { margin: 0 0 6px; }
.mobile-info-button { display: none; padding: 0; font-size: 16px; }
.page-header p { margin: 0; color: #909399; }
.header-actions { display: flex; align-items: center; gap: 10px; }

.backup-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
}

.form-section {
  padding: 14px 16px 16px;
  margin-bottom: 14px;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  background: #fff;
}

.section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.section-title {
  margin-bottom: 14px;
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.section-header .section-title { margin-bottom: 4px; }
.section-desc { color: #909399; font-size: 13px; }
.backup-form { max-width: 820px; }
.hint { margin-left: 10px; color: #999; }

.info-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.info-item {
  padding: 12px;
  border-radius: 8px;
  background: #f8fafc;
}

.info-item.wide { grid-column: 1 / -1; }
.info-item span { display: block; margin-bottom: 6px; color: #909399; font-size: 12px; }
.info-item strong { color: #303133; font-weight: 600; word-break: break-all; }
.error-text { color: #f56c6c !important; }
.file-section { padding-bottom: 8px; }

@media (max-width: 768px) {
  .page-shell {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow: hidden;
  }

  .page-card :deep(.el-card__body) { padding: 12px; }
  .page-header { align-items: flex-start; flex-direction: column; }
  .mobile-info-button { display: inline-flex; }
  .page-header p { display: none; font-size: 12px; line-height: 1.5; }
  .header-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    width: 100%;
  }
  .header-actions .el-button { width: 100%; margin-left: 0; }
  .backup-content { padding-right: 0; }
  .form-section { padding: 12px; border-radius: 12px; }
  .section-header { display: block; }
  .section-header .el-button { width: 100%; margin-top: 12px; }
  .backups :deep(.el-form-item) { display: block; margin-bottom: 16px; }
  .backups :deep(.el-form-item__label) {
    width: 100% !important;
    justify-content: flex-start;
    padding: 0 0 6px;
    font-weight: 600;
  }
  .backups :deep(.el-form-item__content) { margin-left: 0 !important; }
  .backups :deep(.el-input-number) { width: 100%; }
  .hint { display: block; margin: 6px 0 0; }
  .info-grid { grid-template-columns: 1fr; }
}
</style>
