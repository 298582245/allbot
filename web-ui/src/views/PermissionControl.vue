<template>
  <div class="permission-control page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <h2>权限控制</h2>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看权限控制说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <p>{{ pageDescription }}</p>
          </div>
        </div>
      </template>

      <el-form class="permission-form" :model="form" label-width="120px" v-loading="loading">
        <section class="form-section">
          <div class="section-title">平台管理员</div>
          <el-empty v-if="adapterOptions.length === 0" description="暂无适配器，请先在适配器页面添加平台" />
          <el-form-item v-for="adapter in adapterOptions" :key="adapter.platform" :label="adapter.label">
            <el-select
              v-model="platformAdminMap[adapter.platform]"
              multiple
              filterable
              allow-create
              default-first-option
              placeholder="用户 ID，可输入多个"
              style="width: 100%"
            />
            <div class="field-tip">{{ adapter.remark || adapter.platform }} 平台管理员用于系统级管理权限判断。</div>
          </el-form-item>
        </section>

        <section class="form-section">
          <div class="section-title">访问控制</div>
          <el-form-item label="白名单群">
            <el-select v-model="form.access_control.whitelist_groups" multiple filterable allow-create default-first-option placeholder="群 ID，可输入多个" style="width: 100%" />
            <div class="field-tip">设置后插件只在这些群触发；私聊正常。</div>
          </el-form-item>

          <el-form-item label="屏蔽群消息">
            <el-select v-model="form.access_control.blocked_groups" multiple filterable allow-create default-first-option placeholder="群 ID，可输入多个" style="width: 100%" />
            <div class="field-tip">这些群的消息完全不处理，系统功能和插件都不会响应。</div>
          </el-form-item>

          <el-form-item label="白名单 ID">
            <el-select v-model="form.access_control.whitelist_user_ids" multiple filterable allow-create default-first-option placeholder="用户 ID，可输入多个" style="width: 100%" />
            <div class="field-tip">设置后只有这些用户能触发插件。</div>
          </el-form-item>

          <el-form-item label="黑名单 ID">
            <el-select v-model="form.access_control.blocked_user_ids" multiple filterable allow-create default-first-option placeholder="用户 ID，可输入多个" style="width: 100%" />
            <div class="field-tip">这些用户不会触发任何系统功能或插件。</div>
          </el-form-item>
        </section>

        <div class="form-actions">
          <el-button type="primary" :loading="saving" @click="handleSave">保存权限配置</el-button>
          <el-button @click="loadSettings">重置</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/request'

const loading = ref(false)
const saving = ref(false)
const adapters = ref([])
const platformAdminMap = reactive({})
const pageDescription = '统一管理平台管理员、群聊和用户访问规则。'

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '权限控制说明', {
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
  access_control: createAccessControl()
})

const adapterOptions = computed(() => {
  const platformMap = new Map()
  adapters.value.forEach((adapter) => {
    const platform = String(adapter.platform || '').trim()
    if (!platform || platformMap.has(platform)) return
    platformMap.set(platform, {
      platform,
      remark: String(adapter.remark || '').trim(),
      label: adapterLabel(adapter)
    })
  })
  return Array.from(platformMap.values())
})

const loadSettings = async () => {
  loading.value = true
  try {
    const [data, adapterItems] = await Promise.all([
      request.get('/settings'),
      request.get('/adapters')
    ])
    adapters.value = Array.isArray(adapterItems) ? adapterItems : []
    Object.assign(form, {
      admin_username: data.admin_username,
      platform_admins: Array.isArray(data.platform_admins) ? data.platform_admins : [],
      auto_refresh: data.auto_refresh,
      refresh_interval: data.refresh_interval,
      plugin_dir: data.plugin_dir,
      auto_load_plugins: data.auto_load_plugins,
      access_control: normalizeAccessControl(data.access_control)
    })
    syncPlatformAdminMap(form.platform_admins)
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  saving.value = true
  try {
    form.platform_admins = collectPlatformAdmins()
    await request.put('/settings', {
      ...form,
      platform_admins: form.platform_admins,
      access_control: normalizeAccessControl(form.access_control)
    })
    ElMessage.success('权限配置已保存')
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)

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
    inherit_system: false,
    whitelist_groups: list(source.whitelist_groups),
    blocked_groups: list(source.blocked_groups),
    whitelist_user_ids: list(source.whitelist_user_ids),
    blocked_user_ids: list(source.blocked_user_ids)
  }
}

function adapterLabel(adapter) {
  const platform = String(adapter.platform || '').trim()
  const remark = String(adapter.remark || '').trim()
  if (!remark) return platform
  return `${platform}（${remark}）`
}

function syncPlatformAdminMap(admins) {
  Object.keys(platformAdminMap).forEach((platform) => delete platformAdminMap[platform])
  adapterOptions.value.forEach((adapter) => {
    platformAdminMap[adapter.platform] = []
  })
  ;(Array.isArray(admins) ? admins : []).forEach((admin) => {
    const platform = String(admin.platform || '').trim()
    const userID = String(admin.user_id || '').trim()
    if (!platform || !userID || !Array.isArray(platformAdminMap[platform])) return
    if (!platformAdminMap[platform].includes(userID)) platformAdminMap[platform].push(userID)
  })
}

function collectPlatformAdmins() {
  return Object.entries(platformAdminMap)
    .flatMap(([platform, userIDs]) => {
      if (!Array.isArray(userIDs)) return []
      return userIDs
        .map(userID => String(userID).trim())
        .filter(Boolean)
        .map(userID => ({ platform, user_id: userID }))
    })
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

.permission-form {
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

.platform-admins {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.platform-admin-row {
  display: grid;
  grid-template-columns: 140px minmax(180px, 1fr) auto;
  gap: 10px;
  align-items: center;
}

.platform-select { width: 100%; }

.field-tip {
  width: 100%;
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
  line-height: 1.5;
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
  .permission-form { padding-right: 0; }
  .form-section { padding: 12px; border-radius: 12px; }
  .permission-control :deep(.el-form-item) { display: block; margin-bottom: 16px; }
  .permission-control :deep(.el-form-item__label) {
    width: 100% !important;
    justify-content: flex-start;
    padding: 0 0 6px;
    font-weight: 600;
  }
  .permission-control :deep(.el-form-item__content) { margin-left: 0 !important; }
  .platform-admin-row { grid-template-columns: 1fr; padding: 10px; border: 1px solid #ebeef5; border-radius: 10px; }
  .platform-admin-row .el-button { width: 100%; margin-left: 0; }
  .platform-admins > .el-button { width: 100%; margin-left: 0; }
  .form-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    padding-top: 10px;
  }
  .form-actions .el-button { width: 100%; margin-left: 0; }
}
</style>
