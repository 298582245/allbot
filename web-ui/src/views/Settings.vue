<template>
  <div class="settings page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <h2>系统设置</h2>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看系统设置说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <p>{{ pageDescription }}</p>
          </div>
        </div>
      </template>

      <el-form class="settings-form" :model="form" label-width="120px" v-loading="loading">
        <section class="form-section">
          <div class="section-title">Web UI 配置</div>
          <el-form-item label="用户名">
            <el-input v-model="form.admin_username" />
          </el-form-item>
          <el-form-item label="修改密码">
            <el-button type="primary" @click="showPasswordDialog">修改密码</el-button>
          </el-form-item>
          <el-form-item label="管理端口">
            <el-alert title="管理后台端口由启动环境变量 ALLBOT_WEB_PORT 控制，修改后需要重启服务。" type="info" :closable="false" show-icon />
          </el-form-item>
          <el-form-item label="自动刷新">
            <el-switch v-model="form.auto_refresh" />
            <span class="hint">{{ form.auto_refresh ? '启用' : '禁用' }}</span>
          </el-form-item>
          <el-form-item label="刷新间隔">
            <el-input-number v-model="form.refresh_interval" :min="1" :max="60" :disabled="!form.auto_refresh" />
            <span class="hint">秒</span>
          </el-form-item>
        </section>

        <section class="form-section">
          <div class="section-title">插件配置</div>
          <el-form-item label="插件目录">
            <el-input v-model="form.plugin_dir" />
          </el-form-item>
          <el-form-item label="自动加载">
            <el-switch v-model="form.auto_load_plugins" />
            <span class="hint">启动时自动加载所有插件</span>
          </el-form-item>
          <el-form-item label="积分单位">
            <el-input v-model="form.points_unit" placeholder="积分" />
          </el-form-item>
        </section>

        <section class="form-section">
          <div class="section-header">
            <div class="section-title">系统信息</div>
            <div class="section-actions">
              <el-button size="small" :loading="checkingUpdate" @click="loadUpdateInfo">检查更新</el-button>
              <el-tooltip :content="upgradeButtonTip" placement="top">
                <span class="disabled-button-wrap">
                  <el-button size="small" type="primary" disabled>升级</el-button>
                </span>
              </el-tooltip>
            </div>
          </div>
          <div class="info-grid" v-loading="checkingUpdate">
            <div class="info-item">
              <span>版本</span>
              <strong>{{ displayValue(systemInfo.version) }}</strong>
            </div>
            <div class="info-item">
              <span>Commit</span>
              <strong>{{ displayValue(systemInfo.commit) }}</strong>
            </div>
            <div class="info-item">
              <span>构建时间</span>
              <strong>{{ displayValue(systemInfo.buildTime) }}</strong>
            </div>
            <div class="info-item">
              <span>Go 版本</span>
              <strong>{{ displayValue(systemInfo.goVersion) }}</strong>
            </div>
            <div class="info-item">
              <span>最新版本</span>
              <strong>{{ displayValue(systemInfo.latestVersion) }}</strong>
            </div>
            <div class="info-item">
              <span>更新状态</span>
              <el-tag :type="updateStatusType" effect="plain">{{ updateStatusText }}</el-tag>
              <p v-if="updateInfo.error" class="info-tip error">{{ updateInfo.error }}</p>
              <p v-if="systemInfo.upgradeMessage" class="info-tip">{{ systemInfo.upgradeMessage }}</p>
            </div>
            <div class="info-item wide">
              <span>Release 内容</span>
              <pre class="release-body">{{ displayValue(systemInfo.releaseBody, '暂无 Release 内容') }}</pre>
            </div>
            <div class="info-item wide">
              <span>Release 链接</span>
              <el-link v-if="systemInfo.releaseUrl" :href="systemInfo.releaseUrl" target="_blank" type="primary">
                {{ systemInfo.releaseUrl }}
              </el-link>
              <strong v-else>{{ displayValue(systemInfo.releaseUrl) }}</strong>
            </div>
          </div>
        </section>

        <div class="form-actions">
          <el-button type="primary" :loading="saving" @click="handleSave">保存设置</el-button>
          <el-button @click="loadSettings">重置</el-button>
        </div>
      </el-form>
    </el-card>

    <el-dialog v-model="passwordDialogVisible" title="修改密码" width="400px">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="100px">
        <el-form-item label="当前密码" prop="oldPassword">
          <el-input v-model="passwordForm.oldPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input v-model="passwordForm.newPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input v-model="passwordForm.confirmPassword" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleChangePassword">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getUpdateInfo } from '@/api'
import { useAuthStore } from '@/stores/auth'
import request from '@/utils/request'

const authStore = useAuthStore()
const loading = ref(false)
const saving = ref(false)
const checkingUpdate = ref(false)
const pageDescription = '管理 Web UI、插件加载和系统基础信息。'

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '系统设置说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

const form = reactive({
  admin_username: '',
  platform_admins: [],
  auto_refresh: true,
  refresh_interval: 5,
  plugin_dir: './plugins',
  auto_load_plugins: true,
  points_unit: '积分',
  access_control: createAccessControl()
})

const updateInfo = reactive(createEmptyUpdateInfo())

const systemInfo = computed(() => {
  const current = objectValue(updateInfo.current)
  const latest = objectValue(updateInfo.latest)
  const release = objectValue(updateInfo.release)
  return {
    version: firstText(updateInfo.displayVersion, updateInfo.display_version, updateInfo.version, updateInfo.currentVersion, updateInfo.current_version, current.displayVersion, current.display_version, current.version),
    commit: firstText(updateInfo.commit, current.commit),
    buildTime: firstText(updateInfo.buildTime, updateInfo.build_time, current.buildTime, current.build_time),
    goVersion: firstText(updateInfo.goVersion, updateInfo.go_version, current.goVersion, current.go_version),
    latestVersion: firstText(updateInfo.latestVersion, updateInfo.latest_version, latest.version, latest.tagName, latest.tag_name, release.version),
    hasUpdate: Boolean(firstDefined(updateInfo.hasUpdate, updateInfo.has_update)),
    upgradeSupported: Boolean(firstDefined(updateInfo.upgradeSupported, updateInfo.upgrade_supported)),
    upgradeMessage: firstText(updateInfo.upgradeMessage, updateInfo.upgrade_message, updateInfo.message),
    releaseBody: firstText(updateInfo.releaseBody, updateInfo.release_body, updateInfo.body, latest.body, release.body),
    releaseUrl: firstText(updateInfo.releaseUrl, updateInfo.release_url, updateInfo.url, updateInfo.htmlUrl, updateInfo.html_url, latest.url, latest.htmlUrl, latest.html_url, release.url, release.htmlUrl, release.html_url)
  }
})

const updateStatusText = computed(() => {
  if (!updateInfo.loaded) return '未检查'
  if (updateInfo.error) return '检查失败'
  if (systemInfo.value.hasUpdate) return '发现新版本'
  return '已是最新'
})

const updateStatusType = computed(() => {
  if (updateInfo.error) return 'danger'
  if (systemInfo.value.hasUpdate) return 'warning'
  if (updateInfo.loaded) return 'success'
  return 'info'
})

const upgradeButtonTip = computed(() => {
  if (!systemInfo.value.hasUpdate) return systemInfo.value.upgradeMessage || '当前没有可升级版本'
  if (!systemInfo.value.upgradeSupported) return systemInfo.value.upgradeMessage || '当前环境暂不支持在线升级'
  return systemInfo.value.upgradeMessage || '升级功能暂未开放'
})

const passwordDialogVisible = ref(false)
const passwordFormRef = ref(null)
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })

const passwordRules = {
  oldPassword: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于 6 位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== passwordForm.newPassword) callback(new Error('两次输入的密码不一致'))
        else callback()
      },
      trigger: 'blur'
    }
  ]
}

const loadSettings = async () => {
  loading.value = true
  try {
    const data = await request.get('/settings')
    Object.assign(form, {
      admin_username: data.admin_username,
      platform_admins: Array.isArray(data.platform_admins) ? data.platform_admins : [],
      auto_refresh: data.auto_refresh,
      refresh_interval: data.refresh_interval,
      plugin_dir: data.plugin_dir,
      auto_load_plugins: data.auto_load_plugins,
      points_unit: data.points_unit || '积分',
      access_control: normalizeAccessControl(data.access_control)
    })
  } finally {
    loading.value = false
  }
}

const loadUpdateInfo = async () => {
  checkingUpdate.value = true
  try {
    const data = await getUpdateInfo()
    Object.assign(updateInfo, createEmptyUpdateInfo(), normalizeUpdateInfo(data), {
      loaded: true
    })
  } catch (error) {
    Object.assign(updateInfo, createEmptyUpdateInfo(), {
      loaded: true,
      error: error?.response?.data?.error || error?.message || '检查更新失败'
    })
  } finally {
    checkingUpdate.value = false
  }
}

const loadPageData = () => {
  Promise.allSettled([loadSettings(), loadUpdateInfo()])
}

const showPasswordDialog = () => {
  passwordForm.oldPassword = ''
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  passwordDialogVisible.value = true
}

const handleChangePassword = async () => {
  await passwordFormRef.value.validate(async (valid) => {
    if (!valid) return
    await request.post('/settings/password', {
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword
    })
    ElMessage.success('设置已保存')
    passwordDialogVisible.value = false
    authStore.logout()
  })
}

const handleSave = async () => {
  saving.value = true
  try {
    await request.put('/settings', {
      ...form,
      platform_admins: form.platform_admins.filter(item => item.platform && item.user_id),
      access_control: normalizeAccessControl(form.access_control)
    })
    authStore.username = form.admin_username
    localStorage.setItem('username', form.admin_username)
    ElMessage.success('设置已保存')
  } finally {
    saving.value = false
  }
}

onMounted(loadPageData)

function createEmptyUpdateInfo() {
  return {
    loaded: false,
    error: '',
    version: '',
    displayVersion: '',
    display_version: '',
    currentVersion: '',
    current_version: '',
    commit: '',
    buildTime: '',
    build_time: '',
    goVersion: '',
    go_version: '',
    latestVersion: '',
    latest_version: '',
    hasUpdate: false,
    has_update: false,
    upgradeSupported: false,
    upgrade_supported: false,
    upgradeMessage: '',
    upgrade_message: '',
    releaseBody: '',
    release_body: '',
    releaseUrl: '',
    release_url: '',
    url: '',
    htmlUrl: '',
    html_url: '',
    body: '',
    current: null,
    latest: null,
    release: null
  }
}

function normalizeUpdateInfo(value) {
  return value && typeof value === 'object' ? value : {}
}

function objectValue(value) {
  return value && typeof value === 'object' ? value : {}
}

function firstDefined(...items) {
  return items.find(item => item !== undefined && item !== null)
}

function firstText(...items) {
  const value = firstDefined(...items)
  return value === undefined ? '' : String(value).trim()
}

function displayValue(value, fallback = '未知') {
  return String(value || '').trim() || fallback
}

function createAccessControl() {
  return {
    inherit_system: false,
    whitelist_groups: [],
    blocked_groups: [],
    whitelist_user_ids: [],
    blocked_user_ids: []
  }
}

function normalizeAccessControl(value) {
  const source = value && typeof value === 'object' ? value : {}
  const list = (items) => Array.isArray(items) ? items.map(item => String(item).trim()).filter(Boolean) : []
  return {
    inherit_system: Boolean(source.inherit_system),
    whitelist_groups: list(source.whitelist_groups),
    blocked_groups: list(source.blocked_groups),
    whitelist_user_ids: list(source.whitelist_user_ids),
    blocked_user_ids: list(source.blocked_user_ids)
  }
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

.settings-form {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
}

.form-section {
  padding: 14px 16px 6px;
  margin-bottom: 14px;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  background: #fff;
}

.section-title {
  margin-bottom: 14px;
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.section-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.disabled-button-wrap {
  display: inline-flex;
}

.hint { margin-left: 10px; color: #999; }

.info-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  padding-bottom: 10px;
}

.info-item {
  padding: 12px;
  border-radius: 8px;
  background: #f8fafc;
}

.info-item.wide { grid-column: 1 / -1; }
.info-item span { display: block; margin-bottom: 6px; color: #909399; font-size: 12px; }
.info-item strong { color: #303133; font-weight: 600; word-break: break-all; }

.info-tip {
  margin: 8px 0 0;
  color: #909399;
  font-size: 12px;
  line-height: 1.5;
}

.info-tip.error { color: #f56c6c; }

.release-body {
  max-height: 220px;
  margin: 0;
  overflow: auto;
  color: #303133;
  font-family: inherit;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.form-actions {
  position: sticky;
  bottom: 0;
  z-index: 2;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 0 0;
  background: linear-gradient(180deg, rgba(255,255,255,0), #fff 28%);
}

@media (max-width: 768px) {
  .page-shell {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow: hidden;
  }

  .page-card :deep(.el-card__body) { padding: 12px; }
  .page-header { align-items: flex-start; flex-direction: column; }
  .mobile-info-button { display: inline-flex; }
  .page-header p { display: none; font-size: 12px; line-height: 1.5; }
  .settings-form { padding-right: 0; }
  .form-section { padding: 12px; border-radius: 12px; }
  .section-header { display: block; }
  .section-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    margin-bottom: 14px;
  }
  .section-actions .el-button,
  .disabled-button-wrap { width: 100%; }
  .settings :deep(.el-form-item) { display: block; margin-bottom: 16px; }
  .settings :deep(.el-form-item__label) {
    width: 100% !important;
    justify-content: flex-start;
    padding: 0 0 6px;
    font-weight: 600;
  }
  .settings :deep(.el-form-item__content) { margin-left: 0 !important; }
  .settings :deep(.el-input-number) { width: 100%; }
  .settings :deep(.el-dialog) { width: 94vw !important; }
  .settings :deep(.el-dialog .el-form-item) { display: block; }
  .settings :deep(.el-dialog .el-form-item__label) { width: 100% !important; justify-content: flex-start; padding: 0 0 6px; }
  .settings :deep(.el-dialog .el-form-item__content) { margin-left: 0 !important; }
  .hint { display: block; margin: 6px 0 0; }
  .info-grid { grid-template-columns: 1fr; }
  .form-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    padding-top: 10px;
  }
  .form-actions .el-button { width: 100%; margin-left: 0; }
}
</style>
