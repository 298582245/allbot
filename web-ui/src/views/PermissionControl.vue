<template>
  <div class="permission-control page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span class="title">权限控制</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看权限控制说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
        </div>
      </template>

      <el-form class="permission-form" :model="form" label-width="120px" v-loading="loading">
        <section class="form-section">
          <div class="section-title">平台管理员</div>
          <el-form-item label="管理员 union_id">
            <el-select
              v-model="platformAdminUnionIDs"
              multiple
              filterable
              allow-create
              default-first-option
              placeholder="union_id，可输入多个"
              style="width: 100%"
            />
            <div class="field-tip">按统一用户 union_id 授予管理员权限，用户绑定的不同平台账号都会生效。</div>
          </el-form-item>
          <el-empty v-if="adapterOptions.length === 0" description="暂无适配器，请先在适配器页面添加平台" />
          <template v-else>
            <el-form-item label="选择平台">
              <el-select v-model="selectedPlatform" filterable placeholder="搜索或选择平台" style="width: 100%" @change="onPlatformChange">
                <el-option v-for="adapter in adapterOptions" :key="adapter.platform" :label="adapter.label" :value="adapter.platform" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="selectedPlatform" :label="selectedPlatformLabel">
              <el-select
                v-model="platformAdminMap[selectedPlatform]"
                multiple
                filterable
                allow-create
                default-first-option
                placeholder="用户 ID，可输入多个"
                style="width: 100%"
                @visible-change="onAdminSelectVisibleChange"
              />
              <div class="field-tip">{{ selectedPlatformRemark }} 平台管理员用于系统级管理权限判断。修改后失焦自动保存。</div>
            </el-form-item>
          </template>
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
            <el-select v-model="form.access_control.whitelist_user_ids" multiple filterable allow-create default-first-option placeholder="平台用户 ID，可输入多个" style="width: 100%" />
            <div class="field-tip">按当前平台的用户 ID 放行，适合只限制某个平台账号。</div>
          </el-form-item>

          <el-form-item label="黑名单 ID">
            <el-select v-model="form.access_control.blocked_user_ids" multiple filterable allow-create default-first-option placeholder="平台用户 ID，可输入多个" style="width: 100%" />
            <div class="field-tip">按当前平台的用户 ID 屏蔽，优先级高于白名单。</div>
          </el-form-item>

          <el-form-item label="白名单 union_id">
            <el-select v-model="form.access_control.whitelist_union_ids" multiple filterable allow-create default-first-option placeholder="union_id，可输入多个" style="width: 100%" />
            <div class="field-tip">按统一用户 union_id 放行，同一个用户绑定的不同平台账号都会生效。</div>
          </el-form-item>

          <el-form-item label="黑名单 union_id">
            <el-select v-model="form.access_control.blocked_union_ids" multiple filterable allow-create default-first-option placeholder="union_id，可输入多个" style="width: 100%" />
            <div class="field-tip">按统一用户 union_id 屏蔽，同一个用户绑定的不同平台账号都会被拦截。</div>
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
defineOptions({ name: 'PermissionControl' })
import { computed, onMounted, reactive, ref } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/request'

const loading = ref(false)
const saving = ref(false)
const adapters = ref([])
const platformAdminMap = reactive({})
const platformAdminUnionIDs = ref([])
const selectedPlatform = ref('')
const lastSavedAdmins = ref({})
let autoSaving = false
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

const selectedPlatformLabel = computed(() => {
  const adapter = adapterOptions.value.find(a => a.platform === selectedPlatform.value)
  return adapter ? adapter.label : selectedPlatform.value
})

const selectedPlatformRemark = computed(() => {
  const adapter = adapterOptions.value.find(a => a.platform === selectedPlatform.value)
  return adapter ? (adapter.remark || adapter.platform) : selectedPlatform.value
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
    snapshotPlatformAdmins()
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
    snapshotPlatformAdmins()
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
    blocked_user_ids: [],
    whitelist_union_ids: [],
    blocked_union_ids: []
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
    blocked_user_ids: list(source.blocked_user_ids),
    whitelist_union_ids: list(source.whitelist_union_ids),
    blocked_union_ids: list(source.blocked_union_ids)
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
  platformAdminUnionIDs.value = []
  ;(Array.isArray(admins) ? admins : []).forEach((admin) => {
    const unionID = String(admin.union_id || '').trim()
    if (unionID) {
      if (!platformAdminUnionIDs.value.includes(unionID)) platformAdminUnionIDs.value.push(unionID)
      return
    }
    const platform = String(admin.platform || '').trim()
    const userID = String(admin.user_id || '').trim()
    if (!platform || !userID || !Array.isArray(platformAdminMap[platform])) return
    if (!platformAdminMap[platform].includes(userID)) platformAdminMap[platform].push(userID)
  })
}

function collectPlatformAdmins() {
  const unionAdmins = platformAdminUnionIDs.value
    .map(unionID => String(unionID).trim())
    .filter(Boolean)
    .map(unionID => ({ union_id: unionID }))
  const platformAdmins = Object.entries(platformAdminMap)
    .flatMap(([platform, userIDs]) => {
      if (!Array.isArray(userIDs)) return []
      return userIDs
        .map(userID => String(userID).trim())
        .filter(Boolean)
        .map(userID => ({ platform, user_id: userID }))
    })
  return [...unionAdmins, ...platformAdmins]
}

function snapshotPlatformAdmins() {
  const snapshot = {}
  Object.keys(platformAdminMap).forEach((platform) => {
    snapshot[platform] = [...(platformAdminMap[platform] || [])]
  })
  lastSavedAdmins.value = snapshot
}

function onPlatformChange(platform) {
  if (platform && !Array.isArray(platformAdminMap[platform])) {
    platformAdminMap[platform] = []
  }
}

function onAdminSelectVisibleChange(visible) {
  if (visible) return
  if (!selectedPlatform.value) return
  const platform = selectedPlatform.value
  const current = [...(platformAdminMap[platform] || [])].sort().join(',')
  const saved = [...(lastSavedAdmins.value[platform] || [])].sort().join(',')
  if (current === saved) return
  autoSavePlatformAdmin(platform)
}

async function autoSavePlatformAdmin(platform) {
  if (autoSaving) return
  autoSaving = true
  try {
    form.platform_admins = collectPlatformAdmins()
    await request.put('/settings', {
      ...form,
      platform_admins: form.platform_admins,
      access_control: normalizeAccessControl(form.access_control)
    })
    lastSavedAdmins.value[platform] = [...(platformAdminMap[platform] || [])]
    const label = adapterOptions.value.find(a => a.platform === platform)?.label || platform
    ElMessage.success(`${label} 管理员已自动保存`)
  } catch {
    ElMessage.error('自动保存失败，请点击「保存权限配置」手动保存')
  } finally {
    autoSaving = false
  }
}
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
  .subtitle { display: none; }
  .title { font-size: 16px; }
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
  .form-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    padding-top: 10px;
  }
  .form-actions .el-button { width: 100%; margin-left: 0; }
}
</style>
