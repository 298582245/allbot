<template>
  <div class="adapters-page page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <h2>平台机器人</h2>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看平台机器人说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <p>{{ pageDescription }}</p>
          </div>
          <el-button type="primary" @click="showAddDialog">
            <el-icon><Plus /></el-icon>
            添加机器人
          </el-button>
        </div>
      </template>

      <div v-loading="loading" class="adapters-content">
      <div class="adapter-grid">
        <el-card v-for="adapter in paginatedAdapters" :key="adapter.id" class="adapter-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <div>
              <div class="card-title">{{ adapter.remark || getPlatformName(adapter.platform) + ' #' + adapter.id }}</div>
              <div class="card-subtitle">{{ getPlatformName(adapter.platform) }} · ID {{ adapter.id }}</div>
            </div>
            <el-tag :type="adapter.running ? 'success' : 'danger'" size="small">
              {{ adapter.running ? '运行中' : '已停止' }}
            </el-tag>
          </div>
        </template>

        <div class="card-body">
          <div class="description">{{ adapter.description || '暂无描述' }}</div>
          <div class="config-text">{{ getConfigText(adapter) }}</div>
        </div>

        <div class="card-actions">
          <el-switch
            :model-value="adapter.running"
            :loading="adapter.switching"
            @change="value => handleToggleEnabled(adapter, value)"
          />
          <div class="buttons">
            <el-button size="small" type="primary" @click="handleEdit(adapter)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDelete(adapter)">删除</el-button>
          </div>
        </div>
        </el-card>
      </div>

      <el-empty v-if="!loading && adapters.length === 0" description="暂无机器人配置" />
    </div>

      <div class="adapters-pagination">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="adapters.length"
          layout="total, prev, pager, next"
          background
        />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560px" @close="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item label="平台" prop="platform">
          <el-select v-model="form.platform" style="width: 100%" @change="handlePlatformChange">
            <el-option
              v-for="platform in adapterPlatformOptions"
              :key="platform.platform"
              :label="platform.display_name || platform.platform"
              :value="platform.platform"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="备注" prop="remark">
          <el-input v-model="form.remark" placeholder="例如：主 TG 机器人、测试 QQ 号" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="机器人用途说明" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.enabled" />
          <span class="switch-text">{{ form.enabled ? '启用' : '禁用' }}</span>
        </el-form-item>

        <template v-for="field in currentConfigSchema" :key="field.key">
          <el-form-item :label="field.label || field.key" :prop="`config.${field.key}`">
            <el-switch v-if="field.type === 'boolean'" v-model="form.config[field.key]" />
            <el-input-number v-else-if="field.type === 'number'" v-model="form.config[field.key]" style="width: 100%" />
            <el-input v-else-if="field.type === 'textarea'" v-model="form.config[field.key]" type="textarea" :rows="3" :placeholder="field.placeholder || ''" />
            <el-select v-else-if="field.type === 'select'" v-model="form.config[field.key]" style="width: 100%" :placeholder="field.placeholder || '请选择'">
              <el-option
                v-for="option in configFieldOptions(field)"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              />
            </el-select>
            <el-input v-else v-model="form.config[field.key]" :type="field.type === 'password' ? 'password' : 'text'" :show-password="field.type === 'password'" :placeholder="field.placeholder || ''" />
            <div v-if="field.help" class="field-help">{{ field.help }}</div>
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, Plus } from '@element-plus/icons-vue'
import { deleteAdapter, getAdapterPlatforms, getAdapters, saveAdapter } from '@/api'

const loading = ref(false)
const saving = ref(false)
const adapters = ref([])
const currentPage = ref(1)
const pageSize = 8
const dialogVisible = ref(false)
const dialogTitle = ref('添加机器人')
const isEdit = ref(false)
const formRef = ref(null)
const pageDescription = '同一平台可添加多个机器人账号，插件默认可被全部机器人触发。'
const adapterPlatformFallback = [
  {
    platform: 'qq',
    display_name: 'QQ',
    config_schema: [
      { key: 'server_url', label: '服务地址', type: 'text', required: true, placeholder: 'ws://127.0.0.1:3001' },
      { key: 'access_token', label: '访问令牌', type: 'password', required: false, placeholder: 'NapCat 未设置 token 可留空' }
    ]
  },
  {
    platform: 'qq_office',
    display_name: 'QQ 官方机器人',
    config_schema: [
      { key: 'app_id', label: 'App ID', type: 'text', required: true },
      { key: 'client_secret', label: 'Client Secret', type: 'password', required: true },
      { key: 'api_base_url', label: 'API 地址', type: 'text', required: false, placeholder: 'https://api.sgroup.qq.com' },
      { key: 'token_url', label: 'Token 地址', type: 'text', required: false, placeholder: 'https://bots.qq.com/app/getAppAccessToken' }
    ]
  },
  {
    platform: 'telegram',
    display_name: 'Telegram',
    config_schema: [
      { key: 'bot_token', label: 'Bot Token', type: 'textarea', required: true, placeholder: '123456789:ABC...' },
      { key: 'proxy_url', label: '代理地址', type: 'text', required: false, placeholder: 'http://127.0.0.1:7890' }
    ]
  },
  {
    platform: 'wechat',
    display_name: '微信',
    config_schema: [
      { key: 'app_id', label: 'App ID', type: 'text', required: true },
      { key: 'app_secret', label: 'App Secret', type: 'password', required: true }
    ]
  }
]
const adapterPlatforms = ref([...adapterPlatformFallback])

const form = reactive({
  id: 0,
  platform: 'qq',
  remark: '',
  description: '',
  enabled: false,
  config: defaultConfig()
})

const adapterPlatformOptions = computed(() => adapterPlatforms.value)
const adapterPlatformMap = computed(() => Object.fromEntries(adapterPlatforms.value.map(item => [item.platform, item])))
const currentPlatform = computed(() => adapterPlatformMap.value[form.platform] || adapterPlatformFallback.find(item => item.platform === form.platform) || null)
const currentConfigSchema = computed(() => normalizeConfigSchema(currentPlatform.value?.config_schema))
const platformNames = computed(() => Object.fromEntries(adapterPlatforms.value.map(item => [item.platform, item.display_name || item.platform])))
const rules = computed(() => {
  const result = {
    platform: [{ required: true, message: '请选择平台', trigger: 'change' }],
    remark: [{ required: true, message: '请输入备注', trigger: 'blur' }]
  }
  currentConfigSchema.value.forEach(field => {
    if (field.required) {
      result[`config.${field.key}`] = [{ required: true, message: `请输入${field.label || field.key}`, trigger: field.type === 'select' || field.type === 'boolean' ? 'change' : 'blur' }]
    }
  })
  return result
})

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '平台机器人说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

const paginatedAdapters = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return adapters.value.slice(start, start + pageSize)
})

function defaultConfig() {
  const config = {}
  adapterPlatformFallback.forEach(platform => {
    normalizeConfigSchema(platform.config_schema).forEach(field => {
      if (config[field.key] === undefined) config[field.key] = defaultFieldValue(field)
    })
  })
  return config
}

function defaultFieldValue(field) {
  if (field.default !== undefined) return field.default
  if (field.type === 'boolean') return false
  if (field.type === 'number') return 0
  return ''
}

function normalizeConfigSchema(schema) {
  return Array.isArray(schema)
    ? schema.filter(field => field && field.key).map(field => ({ ...field, type: normalizeFieldType(field.type) }))
    : []
}

function normalizeFieldType(type) {
  const value = String(type || 'text').trim().toLowerCase()
  return ['text', 'password', 'number', 'boolean', 'textarea', 'select'].includes(value) ? value : 'text'
}

function mergeAdapterPlatformFallback(items) {
  const merged = [...items]
  const exists = new Set(merged.map(item => item.platform))
  adapterPlatformFallback.forEach(item => {
    if (!exists.has(item.platform)) merged.push(item)
  })
  return merged
}

function ensureConfigFields(platform = currentPlatform.value, preserve = {}) {
  const config = { ...preserve }
  normalizeConfigSchema(platform?.config_schema).forEach(field => {
    if (config[field.key] === undefined) config[field.key] = defaultFieldValue(field)
  })
  return config
}

function configFieldOptions(field) {
  if (!Array.isArray(field.options)) return []
  return field.options.map(option => {
    if (option && typeof option === 'object') return { label: option.label || option.value, value: option.value }
    return { label: String(option), value: option }
  })
}

async function loadAdapterPlatforms() {
  try {
    const items = await getAdapterPlatforms()
    adapterPlatforms.value = mergeAdapterPlatformFallback(Array.isArray(items) ? items : [])
  } catch (error) {
    adapterPlatforms.value = [...adapterPlatformFallback]
  }
}

async function loadAdapters() {
  loading.value = true
  try {
    adapters.value = await getAdapters()
    if (currentPage.value > Math.max(1, Math.ceil(adapters.value.length / pageSize))) {
      currentPage.value = 1
    }
  } finally {
    loading.value = false
  }
}

function getPlatformName(platform) {
  return platformNames.value[platform] || platform
}

function getConfig(row) {
  return typeof row.config === 'string' ? JSON.parse(row.config) : row.config
}

function getConfigText(row) {
  try {
    const config = getConfig(row)
    const schema = normalizeConfigSchema(adapterPlatformMap.value[row.platform]?.config_schema || adapterPlatformFallback.find(item => item.platform === row.platform)?.config_schema)
    const parts = schema
      .filter(field => field.type !== 'password')
      .slice(0, 2)
      .map(field => `${field.label || field.key}: ${config[field.key] || '未设置'}`)
    return parts.join(' | ')
  } catch (error) {
    return '配置解析失败'
  }
}

function showAddDialog() {
  isEdit.value = false
  dialogTitle.value = '添加机器人'
  resetForm()
  dialogVisible.value = true
}

function handleEdit(row) {
  isEdit.value = true
  dialogTitle.value = '编辑机器人'
  form.id = row.id
  form.platform = row.platform
  form.remark = row.remark || ''
  form.description = row.description || ''
  form.enabled = row.enabled
  form.config = ensureConfigFields(adapterPlatformMap.value[form.platform], getConfig(row))
  dialogVisible.value = true
}

function handlePlatformChange() {
  form.config = ensureConfigFields(currentPlatform.value)
}

async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      await saveAdapter({
        id: form.id,
        platform: form.platform,
        remark: form.remark,
        description: form.description,
        enabled: form.enabled,
        config: buildConfig()
      })
      ElMessage.success('机器人配置已保存')
      dialogVisible.value = false
      await loadAdapters()
    } finally {
      saving.value = false
    }
  })
}

function buildConfig() {
  return { ...form.config }
}

async function handleToggleEnabled(row, enabled) {
  const previousRunning = row.running
  row.running = enabled
  row.switching = true
  try {
    await saveAdapter({
      id: row.id,
      platform: row.platform,
      remark: row.remark || '',
      description: row.description || '',
      enabled,
      config: getConfig(row)
    })
    ElMessage.success(enabled ? '启动成功' : '已停止')
    await loadAdapters()
  } catch (error) {
    row.running = previousRunning
  } finally {
    row.switching = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确定删除机器人「${row.remark || row.id}」吗？`, '警告', { type: 'warning' })
  await deleteAdapter(row.id)
  ElMessage.success('机器人已删除')
  await loadAdapters()
}

function resetForm() {
  form.id = 0
  form.platform = 'qq'
  form.remark = ''
  form.description = ''
  form.enabled = false
  form.config = defaultConfig()
  formRef.value?.clearValidate()
}

onMounted(async () => {
  await loadAdapterPlatforms()
  await loadAdapters()
})
</script>

<style scoped>
.adapters-page,
.page-shell {
  width: 100%;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.page-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.page-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow: hidden;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.page-header h2 {
  margin: 0 0 6px;
}

.title-row h2 {
  margin: 0 0 6px;
}

.mobile-info-button {
  display: none;
  padding: 0;
  font-size: 16px;
}

.page-header p {
  margin: 0;
  color: #909399;
}

.adapter-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.adapters-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-right: 2px;
}

.adapter-card {
  min-height: 220px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.card-subtitle {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
}

.card-body {
  min-height: 88px;
}

.description {
  color: #606266;
  margin-bottom: 10px;
}

.config-text {
  color: #909399;
  font-size: 13px;
  line-height: 1.5;
  word-break: break-all;
}

.card-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
}

.buttons {
  display: flex;
  gap: 8px;
}

.adapters-pagination {
  flex-shrink: 0;
  padding-top: 12px;
  display: flex;
  justify-content: center;
  border-top: 1px solid #ebeef5;
}

.switch-text {
  margin-left: 10px;
  color: #909399;
}

.field-help {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
  line-height: 1.4;
}

@media (max-width: 768px) {
  .page-shell {
    height: calc(100dvh - 52px - 76px - 24px);
    overflow: hidden;
  }

  .page-card {
    height: 100%;
    border-radius: 10px;
  }

  .page-header {
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
  }

  .page-header h2 {
    font-size: 18px;
  }

  .mobile-info-button {
    display: inline-flex;
  }

  .page-header p {
    display: none;
    font-size: 13px;
  }

  .page-header > .el-button {
    width: 100%;
    margin-left: 0;
  }

  .adapter-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
  }

  .adapter-card {
    min-height: auto;
  }

  .card-actions {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }

  .buttons {
    width: 100%;
    flex-wrap: wrap;
  }

  .buttons .el-button {
    margin-left: 0;
  }

  .adapters-content {
    -webkit-overflow-scrolling: touch;
  }

  .adapters-pagination {
    overflow-x: auto;
    justify-content: flex-start;
  }

  .adapters-content::-webkit-scrollbar,
  .adapters-pagination::-webkit-scrollbar {
    display: none;
  }
}
</style>
