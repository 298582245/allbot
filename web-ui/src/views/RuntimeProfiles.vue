<template>
  <div class="runtime-profiles page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span class="title">运行环境</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看运行环境说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
          <div class="header-actions">
            <el-button @click="openDownloadSettings">下载设置</el-button>
            <el-button type="primary" @click="openDialog()">新增环境</el-button>
          </div>
        </div>
      </template>

      <div class="table-area desktop-table" v-loading="loading">
        <el-table :data="paginatedProfiles" border height="100%" empty-text="暂无运行环境">
          <el-table-column prop="id" label="ID" width="150" />
          <el-table-column prop="name" label="名称" min-width="150" />
          <el-table-column label="运行时" width="100">
            <template #default="{ row }">
              <el-tag>{{ runtimeLabel(row.runtime) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="来源" width="110">
            <template #default="{ row }">
              <el-tag :type="row.source === 'managed' ? 'warning' : 'info'">{{ sourceLabel(row.source) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="版本" width="130">
            <template #default="{ row }">{{ row.requested_version || row.version || '-' }}</template>
          </el-table-column>
          <el-table-column prop="architecture" label="架构" width="110" />
          <el-table-column prop="executable" label="解释器路径" min-width="260" show-overflow-tooltip />
          <el-table-column label="初始化" width="210">
            <template #default="{ row }">
              <div v-if="runningJobByProfile(row.id)" class="init-progress">
                <div class="init-progress-text">{{ runningJobByProfile(row.id).message || '正在初始化' }}</div>
                <el-progress :percentage="runningJobByProfile(row.id).progress || 1" :stroke-width="8" />
                <div v-if="downloadProgressText(runningJobByProfile(row.id))" class="field-tip">{{ downloadProgressText(runningJobByProfile(row.id)) }}</div>
              </div>
              <template v-else>
                <el-tooltip v-if="statusById(row.id)?.error" :content="statusById(row.id).error" placement="top">
                  <el-tag :type="initStatusType(row)">{{ initStatusLabel(row) }}</el-tag>
                </el-tooltip>
                <el-tag v-else :type="initStatusType(row)">{{ initStatusLabel(row) }}</el-tag>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="145">
            <template #default="{ row }">
              <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
              <el-tag v-if="row.default" type="warning" class="default-tag">默认</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="320" fixed="right">
            <template #default="{ row, $index }">
              <el-button size="small" type="primary" @click="openDialog(row, $index)">编辑</el-button>
              <el-button size="small" type="warning" :loading="Boolean(runningJobByProfile(row.id))" :disabled="Boolean(runningJobByProfile(row.id))" @click="initializeProfile(row, false, shouldForceInitialize(row))">{{ initButtonLabel(row) }}</el-button>
              <el-button size="small" type="success" :loading="testingId === row.id" @click="testProfile(row)">测试</el-button>
              <el-button size="small" type="danger" @click="deleteProfile(row, $index)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="mobile-profile-list" v-loading="loading">
        <el-empty v-if="profiles.length === 0" description="暂无运行环境" />
        <div v-for="(row, index) in paginatedProfiles" :key="row.id" class="mobile-profile-card">
          <div class="mobile-card-header">
            <div class="mobile-title">
              <span>{{ row.name || row.id }}</span>
              <el-tag size="small">{{ runtimeLabel(row.runtime) }}</el-tag>
            </div>
            <div class="mobile-tags">
              <el-tag size="small" :type="row.source === 'managed' ? 'warning' : 'info'">{{ sourceLabel(row.source) }}</el-tag>
              <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
              <el-tag v-if="row.default" size="small" type="warning">默认</el-tag>
            </div>
          </div>

          <div class="mobile-meta-grid">
            <div class="mobile-meta-item">
              <span>ID</span>
              <strong>{{ row.id }}</strong>
            </div>
            <div class="mobile-meta-item">
              <span>版本</span>
              <strong>{{ row.requested_version || row.version || '-' }}</strong>
            </div>
            <div class="mobile-meta-item">
              <span>架构</span>
              <strong>{{ row.architecture || '-' }}</strong>
            </div>
            <div class="mobile-meta-item">
              <span>初始化</span>
              <strong>{{ runningJobByProfile(row.id) ? '初始化中' : initStatusLabel(row) }}</strong>
            </div>
            <div class="mobile-meta-item wide">
              <span>解释器路径</span>
              <strong>{{ row.executable || '-' }}</strong>
            </div>
          </div>

          <div v-if="runningJobByProfile(row.id)" class="init-progress mobile-init-progress">
            <div class="init-progress-text">{{ runningJobByProfile(row.id).message || '正在初始化' }}</div>
            <el-progress :percentage="runningJobByProfile(row.id).progress || 1" :stroke-width="8" />
            <div v-if="downloadProgressText(runningJobByProfile(row.id))" class="field-tip">{{ downloadProgressText(runningJobByProfile(row.id)) }}</div>
          </div>

          <div class="mobile-actions">
            <el-button size="small" type="primary" @click="openDialog(row, index)">编辑</el-button>
            <el-button size="small" type="warning" :loading="Boolean(runningJobByProfile(row.id))" :disabled="Boolean(runningJobByProfile(row.id))" @click="initializeProfile(row, false, shouldForceInitialize(row))">{{ initButtonLabel(row) }}</el-button>
            <el-button size="small" type="success" :loading="testingId === row.id" @click="testProfile(row)">测试</el-button>
            <el-button size="small" type="danger" @click="deleteProfile(row, index)">删除</el-button>
          </div>
        </div>
      </div>

      <StdPagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="filteredProfiles.length"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingIndex >= 0 ? '编辑运行环境' : '新增运行环境'" width="680px">
      <el-form :model="form" label-width="130px">
        <el-form-item label="ID" required>
          <el-input v-model="form.id" :disabled="editingIndex >= 0" placeholder="例如 node18、python310" />
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="例如 Node.js 18" />
        </el-form-item>
        <el-form-item label="运行时" required>
          <el-select v-model="form.runtime" style="width: 100%">
            <el-option label="Node.js" value="nodejs" />
            <el-option label="Python" value="python" />
          </el-select>
        </el-form-item>
        <el-form-item label="来源" required>
          <el-radio-group v-model="form.source">
            <el-radio-button label="manual">手动路径</el-radio-button>
            <el-radio-button label="managed">自动下载</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="版本说明">
          <el-input v-model="form.version" placeholder="例如 18.20.4、3.10.11" />
        </el-form-item>
        <el-form-item v-if="form.source === 'managed'" label="目标版本" required>
          <el-input v-model="form.requested_version" placeholder="例如 18.20.4、3.10.11" />
          <div class="field-tip">只填写版本号，下载源可在下载设置中配置，安装目录限制在项目 runtime/interpreters。</div>
        </el-form-item>
        <el-form-item label="架构">
          <el-select v-model="form.architecture" style="width: 100%">
            <el-option label="Windows x64" value="win-x64" />
            <el-option v-if="form.runtime !== 'python' || form.source !== 'managed'" label="Windows ARM64" value="win-arm64" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.source === 'manual'" label="解释器路径" required>
          <el-input v-model="form.executable" :placeholder="form.runtime === 'python' ? '例如 D:/Python310/python.exe' : '例如 D:/node-v18/node.exe，或 node'" />
          <div class="field-tip">可填写绝对路径；Node.js 也可以填写 PATH 中的命令名 node。</div>
        </el-form-item>
        <el-form-item v-else label="解释器路径">
          <el-input v-model="form.executable" disabled placeholder="保存并初始化后由后端写入" />
        </el-form-item>
        <el-form-item label="保存后初始化">
          <el-switch v-model="autoInitialize" active-text="开启" inactive-text="关闭" />
          <div class="field-tip">新增环境默认开启；编辑环境默认关闭，避免无意重建。</div>
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>
        <el-form-item label="设为默认">
          <el-switch v-model="form.default" />
          <div class="field-tip">每种运行时只能有一个默认环境；旧插件未指定 Profile 时使用默认环境。</div>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" maxlength="200" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="success" :loading="testingId === form.id" @click="testProfile(form)">测试</el-button>
        <el-button type="primary" :loading="saving" @click="saveDialog">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="downloadSettingsVisible" title="运行环境下载设置" width="720px">
      <el-form :model="downloadSettings" label-width="170px">
        <el-form-item label="代理地址">
          <el-input v-model="downloadSettings.proxy_url" placeholder="例如 http://127.0.0.1:7890，留空表示直连或使用系统环境" />
          <div class="field-tip">如包含用户名密码会明文保存在本地数据库，请仅在可信环境使用。</div>
        </el-form-item>
        <el-form-item label="Node.js 镜像">
          <el-input v-model="downloadSettings.node_mirror_url" placeholder="https://nodejs.org/dist" />
          <div class="field-tip">示例：https://npmmirror.com/mirrors/node</div>
        </el-form-item>
        <el-form-item label="Python 包镜像">
          <el-input v-model="downloadSettings.python_package_mirror_url" placeholder="https://www.nuget.org/api/v2/package/python" />
        </el-form-item>
        <el-form-item label="Python 元数据地址">
          <el-input v-model="downloadSettings.python_metadata_url" placeholder="https://api.nuget.org/v3/registration5-gz-semver2/python/index.json" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="downloadSettingsVisible = false">取消</el-button>
        <el-button @click="resetDownloadSettings">恢复默认</el-button>
        <el-button type="primary" :loading="downloadSettingsSaving" @click="saveDownloadSettingsDialog">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
defineOptions({ name: 'RuntimeProfiles' })
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled } from '@element-plus/icons-vue'
import { getLatestRuntimeProfileInitJob, getRuntimeDownloadSettings, getRuntimeProfileInitJob, getRuntimeProfiles, getRuntimeProfileStatus, initRuntimeProfile, saveRuntimeDownloadSettings, saveRuntimeProfiles, testRuntimeProfile } from '@/api'
import StdPagination from '@/components/StdPagination.vue'

const loading = ref(false)
const saving = ref(false)
const testingId = ref('')
const initializingId = ref('')
const profiles = ref([])
const currentPage = ref(1)
const pageSize = 10
const statuses = ref([])
const initJobs = ref({})
const pollTimers = new Map()
const dialogVisible = ref(false)
const editingIndex = ref(-1)
const autoInitialize = ref(true)
const form = reactive(emptyProfile())
const downloadSettingsVisible = ref(false)
const downloadSettingsSaving = ref(false)
const downloadSettings = reactive(defaultDownloadSettings())

const pageDescription = '维护多个 Node.js/Python 解释器，支持手动路径、项目内自动下载托管，并可配置代理/镜像。'
const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '运行环境说明', { confirmButtonText: '知道了', type: 'info' })
}

const filteredProfiles = computed(() => profiles.value)

const paginatedProfiles = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredProfiles.value.slice(start, start + pageSize)
})

function emptyProfile() {
  return { id: '', name: '', runtime: 'nodejs', version: '', executable: '', enabled: true, default: false, description: '', source: 'manual', requested_version: '', architecture: defaultArchitecture() }
}

function defaultDownloadSettings() {
  return {
    proxy_url: '',
    node_mirror_url: 'https://nodejs.org/dist',
    python_package_mirror_url: 'https://www.nuget.org/api/v2/package/python',
    python_metadata_url: 'https://api.nuget.org/v3/registration5-gz-semver2/python/index.json'
  }
}

function assignDownloadSettings(settings) {
  Object.assign(downloadSettings, defaultDownloadSettings(), settings || {})
}

function defaultArchitecture() {
  const platform = String(navigator.platform || '').toLowerCase()
  return platform.includes('arm') ? 'win-arm64' : 'win-x64'
}

function normalizeArchitectureForRuntime() {
  if (form.source === 'managed' && form.runtime === 'python') form.architecture = 'win-x64'
}

watch(() => [form.runtime, form.source], normalizeArchitectureForRuntime)

watch(filteredProfiles, () => {
  const maxPage = Math.max(1, Math.ceil(filteredProfiles.value.length / pageSize))
  if (currentPage.value > maxPage) currentPage.value = maxPage
})

async function loadProfiles() {
  loading.value = true
  try {
    const [profileData, statusData] = await Promise.all([getRuntimeProfiles(), getRuntimeProfileStatus()])
    profiles.value = Array.isArray(profileData) ? profileData : []
    statuses.value = Array.isArray(statusData) ? statusData : []
    await resumeRunningJobs()
  } finally {
    loading.value = false
  }
}

async function loadStatuses() {
  const data = await getRuntimeProfileStatus()
  statuses.value = Array.isArray(data) ? data : []
}

function openDialog(row = null, index = -1) {
  Object.assign(form, emptyProfile(), row || {})
  form.source = form.source || 'manual'
  form.architecture = form.architecture || defaultArchitecture()
  form.requested_version = form.requested_version || ''
  editingIndex.value = index
  autoInitialize.value = index < 0
  dialogVisible.value = true
}

async function openDownloadSettings() {
  const settings = await getRuntimeDownloadSettings()
  assignDownloadSettings(settings)
  downloadSettingsVisible.value = true
}

function resetDownloadSettings() {
  assignDownloadSettings(defaultDownloadSettings())
}

async function saveDownloadSettingsDialog() {
  downloadSettingsSaving.value = true
  try {
    const response = await saveRuntimeDownloadSettings({ ...downloadSettings })
    assignDownloadSettings(response?.settings)
    ElMessage.success(response?.message || '保存成功')
  } finally {
    downloadSettingsSaving.value = false
  }
}

async function saveDialog() {
  const item = normalizeFormProfile()
  const next = normalizeProfilesForSave(item)
  if (!next) return
  const shouldInitialize = autoInitialize.value
  saving.value = true
  try {
    profiles.value = await saveRuntimeProfiles(next)
    dialogVisible.value = false
    ElMessage.success('运行环境已保存')
    if (shouldInitialize) {
      await initializeProfile(item, true)
    } else {
      await loadStatuses()
    }
  } finally {
    saving.value = false
  }
}

function normalizeFormProfile() {
  const item = { ...form }
  item.id = String(item.id || '').trim()
  item.name = String(item.name || '').trim()
  item.runtime = String(item.runtime || '').trim()
  item.source = String(item.source || 'manual').trim()
  item.executable = item.source === 'managed' ? String(item.executable || '').trim() : String(item.executable || '').trim()
  item.version = String(item.version || '').trim()
  item.requested_version = String(item.requested_version || '').trim()
  item.architecture = String(item.architecture || defaultArchitecture()).trim()
  item.description = String(item.description || '').trim()
  if (item.source === 'managed' && !item.requested_version && item.version) item.requested_version = item.version
  return item
}

function normalizeProfilesForSave(item) {
  if (!item.id || !item.name || !item.runtime) {
    ElMessage.warning('请填写 ID、名称和运行时')
    return null
  }
  if (item.source === 'manual' && !item.executable) {
    ElMessage.warning('手动路径模式请填写解释器路径')
    return null
  }
  if (item.source === 'managed' && !item.requested_version) {
    ElMessage.warning('自动下载模式请填写目标版本')
    return null
  }
  const next = profiles.value.map(profile => ({ ...profile }))
  if (editingIndex.value >= 0) next.splice(editingIndex.value, 1, item)
  else next.push(item)
  if (item.default) {
    next.forEach(profile => {
      if (profile.runtime === item.runtime && profile.id !== item.id) profile.default = false
    })
  }
  return next
}

async function deleteProfile(row, index) {
  await ElMessageBox.confirm(`确定删除运行环境「${row.name || row.id}」吗？`, '删除确认', { type: 'warning' })
  const next = profiles.value.filter((_, i) => i !== index)
  profiles.value = await saveRuntimeProfiles(next)
  await loadStatuses()
  ElMessage.success('运行环境已删除')
}

async function initializeProfile(row, silent = false, force = false) {
  const item = { ...row, id: String(row.id || '').trim(), source: String(row.source || 'manual').trim() }
  if (!item.id) {
    ElMessage.warning('请先填写 ID')
    return
  }
  if (force && !silent) {
    await ElMessageBox.confirm(`确定重新初始化运行环境「${item.name || item.id}」吗？`, '重新初始化确认', { type: 'warning' })
  }
  initializingId.value = item.id
  try {
    const job = await initRuntimeProfile({ profile_id: item.id, auto_download: item.source === 'managed', force })
    trackInitJob(job)
    if (!silent) ElMessage.success('初始化已在后台开始')
  } catch (error) {
    await loadStatuses().catch(() => {})
    if (silent) ElMessage.warning('配置已保存，可稍后重试初始化')
    else throw error
  } finally {
    initializingId.value = ''
  }
}

async function testProfile(profile) {
  const item = normalizeTestProfile(profile)
  if (!item.id || (item.source !== 'managed' && !item.executable)) {
    ElMessage.warning('请先填写 ID 和解释器路径')
    return
  }
  testingId.value = item.id
  try {
    const result = await testRuntimeProfile(item)
    ElMessage.success(`测试通过：${result.version_output || '可执行'}`)
  } finally {
    testingId.value = ''
  }
}

function normalizeTestProfile(profile) {
  const item = { ...profile, id: String(profile.id || '').trim(), executable: String(profile.executable || '').trim(), source: String(profile.source || 'manual').trim() }
  item.requested_version = String(profile.requested_version || '').trim()
  item.architecture = String(profile.architecture || defaultArchitecture()).trim()
  return item
}

async function resumeRunningJobs() {
  await Promise.all(profiles.value.map(async profile => {
    try {
      const job = await getLatestRuntimeProfileInitJob(profile.id)
      if (job?.status === 'running') trackInitJob(job)
    } catch (_) {}
  }))
}

function trackInitJob(job) {
  if (!job?.id || !job.profile_id) return
  initJobs.value = { ...initJobs.value, [job.profile_id]: job }
  if (job.status === 'running') startPollingJob(job)
}

function startPollingJob(job) {
  if (pollTimers.has(job.id)) return
  const poll = async () => {
    try {
      const latest = await getRuntimeProfileInitJob(job.id)
      initJobs.value = { ...initJobs.value, [latest.profile_id]: latest }
      if (latest.status === 'completed') {
        stopPollingJob(latest.id)
        ElMessage.success(latest.result?.message || '初始化成功')
        await loadProfiles()
        return
      }
      if (latest.status === 'failed') {
        stopPollingJob(latest.id)
        ElMessage.error(latest.error || '初始化失败')
        await loadStatuses().catch(() => {})
      }
    } catch (_) {
      stopPollingJob(job.id)
    }
  }
  pollTimers.set(job.id, window.setInterval(poll, 1000))
  poll()
}

function stopPollingJob(jobId) {
  const timer = pollTimers.get(jobId)
  if (timer) window.clearInterval(timer)
  pollTimers.delete(jobId)
}

function runningJobByProfile(profileId) {
  const job = initJobs.value[profileId]
  return job?.status === 'running' ? job : null
}

function downloadProgressText(job) {
  if (!job?.downloaded_bytes) return ''
  const downloaded = formatBytes(job.downloaded_bytes)
  if (!job.total_bytes || job.total_bytes < 0) return downloaded
  return `${downloaded} / ${formatBytes(job.total_bytes)}`
}

function formatBytes(value) {
  const size = Number(value || 0)
  if (size >= 1024 * 1024 * 1024) return `${(size / 1024 / 1024 / 1024).toFixed(2)} GB`
  if (size >= 1024 * 1024) return `${(size / 1024 / 1024).toFixed(2)} MB`
  if (size >= 1024) return `${(size / 1024).toFixed(2)} KB`
  return `${size} B`
}

function runtimeLabel(runtime) {
  return runtime === 'python' ? 'Python' : 'Node.js'
}

function sourceLabel(source) {
  return source === 'managed' ? '自动下载' : '手动路径'
}

function statusById(profileId) {
  return statuses.value.find(item => item.profile_id === profileId)
}

function initStatusLabel(row) {
  const status = statusById(row.id)
  if (!status) return '未初始化'
  if (status.initialized) return '已初始化'
  if (status.error) return status.error.includes('解释器') ? '解释器不可用' : '初始化失败'
  return '未初始化'
}

function initStatusType(row) {
  const status = statusById(row.id)
  if (status?.initialized) return 'success'
  if (status?.error) return 'danger'
  return 'info'
}

function shouldForceInitialize(row) {
  const status = statusById(row.id)
  return status?.initialized || Boolean(status?.error)
}

function initButtonLabel(row) {
  if (runningJobByProfile(row.id)) return '初始化中'
  return statusById(row.id)?.initialized ? '重新初始化' : '初始化'
}

onMounted(loadProfiles)
onBeforeUnmount(() => {
  pollTimers.forEach(timer => window.clearInterval(timer))
  pollTimers.clear()
})
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.title-row { display: flex; align-items: center; gap: 6px; }
.title { font-size: 18px; font-weight: 600; }
.mobile-info-button { display: none; padding: 0; font-size: 16px; }
.subtitle { margin-top: 6px; color: #909399; font-size: 13px; line-height: 1.5; }
.table-area { flex: 1; min-height: 0; overflow: hidden; }
.mobile-profile-list { display: none; }
.default-tag { margin-left: 6px; }
.field-tip { margin-top: 4px; color: #909399; font-size: 12px; line-height: 1.4; }
.init-progress { display: flex; flex-direction: column; gap: 4px; }
.init-progress-text { color: #606266; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 768px) {
  .page-shell { height: calc(100dvh - 52px - 76px - 24px); overflow: hidden; }
  .page-card { height: 100%; min-height: 0; }
  .page-card :deep(.el-card__body) { overflow: hidden; }
  .page-header { align-items: flex-start; flex-direction: column; }
  .title { font-size: 16px; }
  .mobile-info-button { display: inline-flex; }
  .subtitle { display: none; }
  .header-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; width: 100%; }
  .header-actions > .el-button { width: 100%; margin-left: 0; }
  .desktop-table { display: none; }
  .mobile-profile-list { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 12px; overflow-y: auto; padding-right: 2px; }
  .mobile-profile-card { padding: 14px; border: 1px solid #ebeef5; border-radius: 12px; background: #fff; }
  .mobile-card-header { display: flex; flex-direction: column; gap: 10px; margin-bottom: 12px; }
  .mobile-title { display: flex; align-items: center; justify-content: space-between; gap: 8px; font-weight: 600; color: #303133; }
  .mobile-title span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .mobile-tags { display: flex; flex-wrap: wrap; gap: 6px; }
  .mobile-meta-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
  .mobile-meta-item { min-width: 0; }
  .mobile-meta-item.wide { grid-column: 1 / -1; }
  .mobile-meta-item span { display: block; margin-bottom: 4px; color: #909399; font-size: 12px; }
  .mobile-meta-item strong { display: block; overflow: hidden; color: #303133; font-size: 13px; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
  .mobile-init-progress { margin-top: 12px; }
  .mobile-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-top: 12px; }
  .mobile-actions .el-button { width: 100%; margin-left: 0; }
  .runtime-profiles :deep(.el-dialog) { width: 94vw !important; }
  .runtime-profiles :deep(.el-form-item) { display: block; margin-bottom: 16px; }
  .runtime-profiles :deep(.el-form-item__label) { width: 100% !important; justify-content: flex-start; padding: 0 0 6px; font-weight: 600; }
  .runtime-profiles :deep(.el-form-item__content) { margin-left: 0 !important; }
  .runtime-profiles :deep(.el-radio-group),
  .runtime-profiles :deep(.el-select),
  .runtime-profiles :deep(.el-input) { width: 100%; }
  .runtime-profiles :deep(.el-radio-button) { flex: 1; }
  .runtime-profiles :deep(.el-radio-button__inner) { width: 100%; }
  .runtime-profiles :deep(.el-dialog__footer) { display: grid; grid-template-columns: 1fr; gap: 8px; }
  .runtime-profiles :deep(.el-dialog__footer .el-button) { width: 100%; margin-left: 0; }
}
</style>
