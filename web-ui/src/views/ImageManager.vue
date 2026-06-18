<template>
  <div class="image-manager page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title-row">
              <span>图床管理</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看图床管理说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <p>{{ pageDescription }}</p>
          </div>
          <div class="header-actions">
            <el-upload :show-file-list="false" :before-upload="handleUploadFile" :accept="uploadAccept">
              <el-button type="primary" :loading="uploading">
                <el-icon><Upload /></el-icon>
                上传图片
              </el-button>
            </el-upload>
            <el-button :loading="loading" @click="loadImages">刷新</el-button>
            <el-button @click="openSettingsDialog">
              <el-icon><Setting /></el-icon>
              配置
            </el-button>
          </div>
        </div>
      </template>

      <div class="filter-panel desktop-filter-panel">
        <el-input v-model="filters.keyword" clearable placeholder="搜索文件名、直链或类型" @keyup.enter="handleSearch" />
        <el-select v-model="filters.content_type" clearable filterable placeholder="content_type">
          <el-option v-for="type in contentTypeOptions" :key="type" :label="type" :value="type" />
        </el-select>
        <el-button type="primary" @click="handleSearch">查询</el-button>
      </div>

      <div class="mobile-filter-panel">
        <el-input v-model="filters.keyword" clearable placeholder="搜索图片" @keyup.enter="handleSearch" />
        <el-select v-model="filters.content_type" clearable filterable placeholder="content_type">
          <el-option v-for="type in contentTypeOptions" :key="type" :label="type" :value="type" />
        </el-select>
        <el-button type="primary" @click="handleSearch">筛选</el-button>
      </div>

      <div class="image-table-wrap desktop-image-table" v-loading="loading">
        <el-table :data="images" row-key="id" border height="100%" empty-text="暂无图片">
          <el-table-column label="预览" width="96">
            <template #default="{ row }">
              <el-image class="table-thumb" :src="imageUrl(row)" fit="cover" :preview-src-list="[imageUrl(row)]" preview-teleported />
            </template>
          </el-table-column>
          <el-table-column label="文件名" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ imageName(row) }}</template>
          </el-table-column>
          <el-table-column prop="content_type" label="content_type" min-width="140" show-overflow-tooltip>
            <template #default="{ row }"><el-tag effect="plain">{{ imageContentType(row) }}</el-tag></template>
          </el-table-column>
          <el-table-column label="大小" width="110">
            <template #default="{ row }">{{ formatSize(imageSize(row)) }}</template>
          </el-table-column>
          <el-table-column label="尺寸" width="110">
            <template #default="{ row }">{{ imageDimensions(row) }}</template>
          </el-table-column>
          <el-table-column label="直链" min-width="260" show-overflow-tooltip>
            <template #default="{ row }"><code>{{ imageUrl(row) }}</code></template>
          </el-table-column>
          <el-table-column label="上传时间" min-width="170">
            <template #default="{ row }">{{ formatTime(row.created_at || row.createdAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="copyDirectUrl(row)">复制直链</el-button>
              <el-button link type="success" @click="previewImage(row)">预览</el-button>
              <el-button link type="danger" :loading="deletingId === imageKey(row)" @click="deleteItem(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="mobile-image-grid" v-loading="loading">
        <article v-for="row in images" :key="imageKey(row)" class="mobile-image-card">
          <el-image class="mobile-thumb" :src="imageUrl(row)" fit="cover" :preview-src-list="[imageUrl(row)]" preview-teleported />
          <div class="mobile-image-body">
            <div class="mobile-image-title">{{ imageName(row) }}</div>
            <div class="mobile-image-meta"><span>类型</span><strong>{{ imageContentType(row) }}</strong></div>
            <div class="mobile-image-meta"><span>大小</span><strong>{{ formatSize(imageSize(row)) }}</strong></div>
            <div class="mobile-image-meta"><span>尺寸</span><strong>{{ imageDimensions(row) }}</strong></div>
            <div class="mobile-image-meta"><span>时间</span><strong>{{ formatTime(row.created_at || row.createdAt) }}</strong></div>
            <code class="mobile-url">{{ imageUrl(row) }}</code>
          </div>
          <div class="mobile-image-actions">
            <el-button size="small" type="primary" @click="copyDirectUrl(row)">复制</el-button>
            <el-button size="small" type="success" @click="previewImage(row)">预览</el-button>
            <el-button size="small" type="danger" :loading="deletingId === imageKey(row)" @click="deleteItem(row)">删除</el-button>
          </div>
        </article>
        <el-empty v-if="!loading && images.length === 0" description="暂无图片" />
      </div>

      <div class="pagination-row">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          background
          @current-change="loadImages"
        />
      </div>
    </el-card>

    <el-dialog v-model="settingsVisible" title="图床配置" width="560px" class="settings-dialog" :close-on-click-modal="false">
      <el-form :model="settingsForm" label-width="130px">
        <el-form-item label="storage_dir" required>
          <el-input v-model.trim="settingsForm.storage_dir" placeholder="例如：runtime/images" />
          <div class="field-tip">图片存储目录，留空会导致后端无法确定保存位置。</div>
        </el-form-item>
        <el-form-item label="public_base_url">
          <el-input v-model.trim="settingsForm.public_base_url" placeholder="例如：https://example.com/images" />
          <div class="field-tip">用于拼接图片直链，通常填写外部可访问的图床根地址。</div>
        </el-form-item>
        <el-form-item label="max_size" required>
          <el-input-number v-model="settingsForm.max_size" :min="1" :step="1024 * 1024" style="width: 100%" />
          <div class="field-tip">单张图片最大字节数，例如 5242880 表示 5 MB。</div>
        </el-form-item>
        <el-form-item label="allowed_types" required>
          <el-select v-model="settingsForm.allowed_types" multiple filterable allow-create default-first-option style="width: 100%" placeholder="选择或输入允许的 MIME 类型">
            <el-option v-for="type in defaultAllowedTypes" :key="type" :label="type" :value="type" />
          </el-select>
          <div class="field-tip">只允许上传这些 content_type，可手动输入 image/webp 等类型。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="settingsVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingSettings" @click="saveSettingsDialog">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="previewVisible" title="图片预览" width="72vw" class="preview-dialog">
      <img v-if="previewUrl" class="preview-image" :src="previewUrl" alt="图片预览" />
      <template #footer>
        <el-button @click="copyText(previewUrl, '图片直链已复制')">复制直链</el-button>
        <el-button type="primary" @click="previewVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, Setting, Upload } from '@element-plus/icons-vue'
import { deleteImage, getImageSettings, listImages, saveImageSettings, uploadImage } from '@/api'

const pageDescription = '管理内置图床图片、上传新图片、复制直链，并维护存储目录和上传限制。'
const defaultAllowedTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']
const loading = ref(false)
const uploading = ref(false)
const savingSettings = ref(false)
const deletingId = ref('')
const images = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = 20
const settingsVisible = ref(false)
const previewVisible = ref(false)
const previewUrl = ref('')
const filters = reactive({ keyword: '', content_type: '' })
const settingsForm = reactive(createDefaultSettings())
const originalStorageDir = ref('')

const contentTypeOptions = computed(() => {
  const types = new Set(defaultAllowedTypes)
  images.value.forEach(item => {
    const type = imageContentType(item)
    if (type && type !== '-') types.add(type)
  })
  settingsForm.allowed_types.forEach(type => types.add(type))
  return Array.from(types).sort()
})

const uploadAccept = computed(() => settingsForm.allowed_types.join(','))

const loadImages = async () => {
  loading.value = true
  try {
    const data = await listImages({
      keyword: filters.keyword.trim(),
      content_type: filters.content_type,
      limit: pageSize,
      offset: (currentPage.value - 1) * pageSize
    })
    images.value = normalizeImageList(data)
    total.value = Number(data?.total ?? images.value.length)
  } finally {
    loading.value = false
  }
}

const loadSettings = async () => {
  const data = await getImageSettings()
  const settings = normalizeSettings(data)
  Object.assign(settingsForm, settings)
  originalStorageDir.value = settings.storage_dir
}

const handleSearch = () => {
  currentPage.value = 1
  loadImages()
}

const handleUploadFile = async (file) => {
  if (!validateUploadFile(file)) return false
  uploading.value = true
  try {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('name', file.name || '')
    await uploadImage(formData)
    ElMessage.success('图片上传成功')
    currentPage.value = 1
    await loadImages()
  } finally {
    uploading.value = false
  }
  return false
}

const openSettingsDialog = async () => {
  await loadSettings()
  settingsVisible.value = true
}

const saveSettingsDialog = async () => {
  const payload = buildSettingsPayload()
  if (!payload) return
  const action = await chooseStorageDirAction(payload.storage_dir)
  if (action === null) return
  if (action) payload.storage_dir_action = action
  savingSettings.value = true
  try {
    const result = await saveImageSettings(payload)
    const savedSettings = normalizeSettings(result)
    Object.assign(settingsForm, savedSettings)
    originalStorageDir.value = savedSettings.storage_dir
    showSettingsSavedMessage(result?.migration)
    settingsVisible.value = false
  } finally {
    savingSettings.value = false
  }
}

const deleteItem = async (row) => {
  const id = imageKey(row)
  if (!id) {
    ElMessage.warning('图片缺少 ID，无法删除')
    return
  }
  try {
    await ElMessageBox.confirm(`确定删除图片「${imageName(row)}」吗？`, '删除图片', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch {
    return
  }
  deletingId.value = id
  try {
    await deleteImage(id)
    ElMessage.success('图片已删除')
    await loadImages()
  } finally {
    deletingId.value = ''
  }
}

const copyDirectUrl = (row) => copyText(imageUrl(row), '图片直链已复制')

const previewImage = (row) => {
  previewUrl.value = imageUrl(row)
  previewVisible.value = true
}

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, '图床管理说明', { confirmButtonText: '知道了', type: 'info' })
}

function createDefaultSettings() {
  return { storage_dir: '', public_base_url: '', max_size: 5 * 1024 * 1024, allowed_types: [...defaultAllowedTypes] }
}

function normalizeSettings(data) {
  const settings = data?.settings || data || {}
  return {
    storage_dir: String(settings.storage_dir || settings.storageDir || ''),
    public_base_url: String(settings.public_base_url || settings.publicBaseUrl || ''),
    max_size: Number(settings.max_size || settings.maxSize || 5 * 1024 * 1024),
    allowed_types: normalizeAllowedTypes(settings.allowed_types || settings.allowedTypes)
  }
}

function normalizeAllowedTypes(value) {
  if (Array.isArray(value)) return value.map(item => String(item).trim()).filter(Boolean)
  const text = String(value || '').trim()
  if (!text) return [...defaultAllowedTypes]
  return text.split(/[\n,，]/).map(item => item.trim()).filter(Boolean)
}

function normalizeImageList(data) {
  if (Array.isArray(data)) return data
  if (Array.isArray(data?.items)) return data.items
  if (Array.isArray(data?.images)) return data.images
  if (Array.isArray(data?.list)) return data.list
  return []
}

function buildSettingsPayload() {
  const storageDir = settingsForm.storage_dir.trim()
  const publicBaseUrl = settingsForm.public_base_url.trim()
  const maxSize = Number(settingsForm.max_size || 0)
  const allowedTypes = settingsForm.allowed_types.map(item => String(item).trim()).filter(Boolean)
  if (!storageDir) {
    ElMessage.warning('请输入 storage_dir')
    return null
  }
  if (!Number.isFinite(maxSize) || maxSize <= 0) {
    ElMessage.warning('max_size 必须大于 0')
    return null
  }
  if (allowedTypes.length === 0) {
    ElMessage.warning('请至少设置一种 allowed_types')
    return null
  }
  return { storage_dir: storageDir, public_base_url: publicBaseUrl, max_size: maxSize, allowed_types: allowedTypes }
}

async function chooseStorageDirAction(storageDir) {
  if (normalizeStorageDir(storageDir) === normalizeStorageDir(originalStorageDir.value)) return ''
  try {
    await ElMessageBox.confirm(
      '检测到图片存储目录已修改。请选择旧目录图片处理方式：迁移旧目录图片到新目录并删除旧目录；或不迁移不删除旧目录，保存后旧图片直链将失效。',
      '存储目录变更处理',
      {
        type: 'warning',
        distinguishCancelAndClose: true,
        confirmButtonText: '迁移并删除旧目录',
        cancelButtonText: '不迁移，仅保存'
      }
    )
    return 'migrate_delete_old'
  } catch (action) {
    if (action === 'cancel') return 'keep_old'
    return null
  }
}

function showSettingsSavedMessage(migration) {
  if (!migration?.changed) {
    ElMessage.success('图床配置已保存')
    return
  }
  if (migration.warning) {
    ElMessage.warning(migration.warning)
    return
  }
  if (migration.action === 'migrate_delete_old') {
    ElMessage.success(`图床配置已保存，已迁移 ${Number(migration.migrated_files || migration.migratedFiles || 0)} 个旧图片文件`)
    return
  }
  if (migration.action === 'keep_old') {
    ElMessage.warning('图床配置已保存，旧目录未迁移，历史图片直链可能失效')
    return
  }
  ElMessage.success('图床配置已保存')
}

function normalizeStorageDir(value) {
  return String(value || '').trim().replace(/\\/g, '/').replace(/\/+$/, '')
}

function validateUploadFile(file) {
  const type = String(file?.type || '').trim()
  const maxSize = Number(settingsForm.max_size || 0)
  if (settingsForm.allowed_types.length > 0 && type && !settingsForm.allowed_types.includes(type)) {
    ElMessage.warning(`不允许上传 ${type} 类型图片`)
    return false
  }
  if (maxSize > 0 && file.size > maxSize) {
    ElMessage.warning(`图片大小不能超过 ${formatSize(maxSize)}`)
    return false
  }
  return true
}

function imageKey(row) {
  return String(row?.public_id || row?.publicId || row?.id || row?.image_id || row?.imageId || row?.filename || row?.name || '')
}

function imageSize(row) {
  return row?.size_bytes || row?.sizeBytes || row?.size || 0
}

function imageDimensions(row) {
  const width = Number(row?.width || 0)
  const height = Number(row?.height || 0)
  return width > 0 && height > 0 ? `${width}×${height}` : '-'
}

function imageName(row) {
  return row?.original_name || row?.originalName || row?.filename || row?.name || imageKey(row) || '-'
}

function imageContentType(row) {
  return row?.content_type || row?.contentType || row?.mime_type || row?.mimeType || '-'
}

function imageUrl(row) {
  const url = row?.url || row?.public_url || row?.publicUrl || row?.direct_url || row?.directUrl || row?.path || ''
  if (!url) return ''
  if (/^https?:\/\//i.test(url)) return url
  return url.startsWith('/') ? `${window.location.origin}${url}` : `${window.location.origin}/${url}`
}

function formatSize(size) {
  const value = Number(size || 0)
  if (value <= 0) return '-'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(2)} MB`
}

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

async function copyText(text, message) {
  if (!text) {
    ElMessage.warning('没有可复制的图片直链')
    return
  }
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    copyByFallback(text)
  }
  ElMessage.success(message)
}

function copyByFallback(text) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'readonly')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

onMounted(async () => {
  try {
    await loadSettings()
  } catch {
    Object.assign(settingsForm, createDefaultSettings())
  }
  await loadImages()
})
</script>

<style scoped>
.page-shell { height: 100%; min-height: 0; }
.page-card { height: 100%; display: flex; flex-direction: column; }
.page-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.title-row { display: flex; align-items: center; gap: 6px; font-size: 18px; font-weight: 600; }
.mobile-info-button { display: none; padding: 0; font-size: 16px; }
.page-header p { margin: 6px 0 0; color: #909399; font-size: 13px; }
.header-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.filter-panel { display: grid; grid-template-columns: minmax(0, 1fr) 220px auto; gap: 10px; margin-bottom: 12px; }
.mobile-filter-panel { display: none; }
.image-table-wrap { flex: 1; min-height: 0; }
.table-thumb { width: 58px; height: 58px; border-radius: 8px; background: #f5f7fa; }
.desktop-image-table code,
.mobile-url { padding: 4px 8px; border-radius: 6px; color: #1d4ed8; background: #eff6ff; font-family: "JetBrains Mono", "Cascadia Code", monospace; word-break: break-all; }
.mobile-image-grid { display: none; }
.pagination-row { display: flex; justify-content: flex-end; flex-shrink: 0; padding-top: 12px; }
.field-tip { margin-top: 6px; color: #909399; font-size: 12px; line-height: 1.5; }
.preview-dialog :deep(.el-dialog__body) { text-align: center; background: #f8fafc; }
.preview-image { max-width: 100%; max-height: 66vh; border-radius: 8px; object-fit: contain; }
@media (max-width: 768px) {
  .page-shell { height: calc(100dvh - 52px - 76px - 24px); overflow: hidden; }
  .page-card { height: 100%; min-height: 0; }
  .page-card :deep(.el-card__header) { padding: 10px 12px; flex-shrink: 0; }
  .page-card :deep(.el-card__body) { min-height: 0; padding: 10px 12px; overflow: hidden; }
  .page-header { align-items: flex-start; flex-direction: column; gap: 10px; }
  .title-row { font-size: 16px; }
  .mobile-info-button { display: inline-flex; }
  .page-header p { display: none; }
  .header-actions { width: 100%; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .header-actions .el-button,
  .header-actions :deep(.el-upload) { width: 100%; margin-left: 0; }
  .header-actions :deep(.el-upload .el-button) { width: 100%; }
  .desktop-filter-panel,
  .desktop-image-table { display: none; }
  .mobile-filter-panel { display: grid; grid-template-columns: minmax(0, 1fr); gap: 8px; flex-shrink: 0; margin-bottom: 10px; }
  .mobile-filter-panel .el-button { width: 100%; margin-left: 0; }
  .mobile-image-grid { flex: 1 1 0; min-height: 0; display: grid; align-content: start; gap: 10px; overflow-y: auto; overflow-x: hidden; padding-bottom: 8px; }
  .mobile-image-card { display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 10px; padding: 12px; border: 1px solid #ebeef5; border-radius: 12px; background: #fff; box-shadow: 0 6px 18px rgba(15, 23, 42, 0.05); }
  .mobile-thumb { width: 92px; height: 92px; border-radius: 10px; background: #f5f7fa; }
  .mobile-image-body { min-width: 0; display: grid; gap: 6px; }
  .mobile-image-title { color: #303133; font-weight: 600; word-break: break-word; overflow-wrap: anywhere; }
  .mobile-image-meta { display: flex; justify-content: space-between; gap: 10px; font-size: 12px; }
  .mobile-image-meta span { color: #909399; flex-shrink: 0; }
  .mobile-image-meta strong { min-width: 0; color: #303133; font-weight: 500; text-align: right; word-break: break-word; overflow-wrap: anywhere; }
  .mobile-url { display: block; max-height: 45px; overflow: hidden; font-size: 12px; }
  .mobile-image-actions { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; padding-top: 10px; border-top: 1px solid #f0f2f5; }
  .mobile-image-actions .el-button { width: 100%; margin-left: 0; }
  .pagination-row { justify-content: flex-start; overflow-x: auto; flex-shrink: 0; }
  .settings-dialog :deep(.el-dialog),
  .preview-dialog :deep(.el-dialog) { width: 94vw !important; }
  .settings-dialog :deep(.el-form-item) { display: block; }
  .settings-dialog :deep(.el-form-item__label) { width: 100% !important; justify-content: flex-start; padding: 0 0 6px; }
  .settings-dialog :deep(.el-form-item__content) { margin-left: 0 !important; }
  .mobile-image-grid::-webkit-scrollbar,
  .pagination-row::-webkit-scrollbar { display: none; }
}
</style>
