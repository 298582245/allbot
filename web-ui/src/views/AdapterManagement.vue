<template>
  <div class="adapter-management page-shell">
    <el-card class="page-card">
      <template #header>
        <div class="page-header">
          <div>
            <div class="title">适配器管理</div>
            <div class="subtitle">{{ platforms.length }} 个适配器定义</div>
          </div>
          <div class="header-actions">
            <el-input v-model="keyword" class="search-input" clearable placeholder="搜索名称、ID、运行时或作者" />
            <el-button @click="importVisible = true"><el-icon><Upload /></el-icon>导入适配器</el-button>
            <el-button type="primary" @click="openCreate">
              <el-icon><Plus /></el-icon>新增适配器
            </el-button>
          </div>
        </div>
      </template>

      <div v-loading="loading" class="adapter-content">
        <div v-if="pageItems.length" class="adapter-grid">
          <el-card v-for="item in pageItems" :key="item.platform" class="adapter-card" :class="{ pinned: item.pinned }" shadow="hover">
            <div class="card-heading">
              <div class="identity">
                <div class="adapter-name">{{ item.display_name || item.platform }}</div>
                <div class="adapter-id">{{ item.platform }}</div>
              </div>
              <el-tag :type="item.custom ? 'success' : 'info'" size="small" effect="plain">
                {{ item.custom ? '自定义' : 'Go 内置' }}
              </el-tag>
            </div>

            <div class="metadata-grid">
              <div><span class="meta-label">版本</span><span>{{ item.version || '1.0.0' }}</span></div>
              <div><span class="meta-label">运行时</span><span>{{ runtimeLabel(item) }}</span></div>
              <div><span class="meta-label">作者</span><span class="meta-value">{{ item.author || (item.custom ? '—' : 'AllBot') }}</span></div>
            </div>
            <div class="description">{{ item.description || '暂无介绍' }}</div>
            <div class="capabilities-block">
              <span class="meta-label">支持功能</span>
              <div class="capability-tags">
                <el-tag v-for="label in capabilityLabels(item)" :key="label" size="small" effect="plain">{{ label }}</el-tag>
                <span v-if="capabilityLabels(item).length === 0" class="empty-capabilities">—</span>
              </div>
            </div>
            <div class="card-actions">
              <el-button text size="small" :type="item.pinned ? 'warning' : 'info'" @click="togglePinned(item)">
                <el-icon><StarFilled v-if="item.pinned" /><Star v-else /></el-icon>{{ item.pinned ? '取消置顶' : '置顶' }}
              </el-button>
              <template v-if="item.custom">
                <el-button size="small" @click="reloadDefinition(item)"><el-icon><Refresh /></el-icon>重载</el-button>
                <el-button size="small" @click="openConfig(item)"><el-icon><Setting /></el-icon>配置</el-button>
                <el-button size="small" @click="openCode(item)"><el-icon><Document /></el-icon>代码</el-button>
                <el-button size="small" type="danger" plain @click="removeDefinition(item)"><el-icon><Delete /></el-icon>删除</el-button>
              </template>
              <el-tag v-else class="readonly-tag" size="small" type="info" effect="plain">只读</el-tag>
            </div>
          </el-card>
        </div>
        <el-empty v-else-if="!loading && platforms.length === 0" description="没有可用的适配器" />
        <el-empty v-else-if="!loading" description="没有匹配的适配器" />
      </div>

      <StdPagination v-model:current-page="currentPage" :page-size="pageSize" :total="filteredPlatforms.length" />
    </el-card>

    <el-dialog v-model="configVisible" :title="editing ? '配置适配器' : '新增适配器'" width="620px" @closed="clearValidation">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="110px">
        <el-form-item label="适配器 ID" prop="id"><el-input v-model="form.id" :disabled="editing" placeholder="例如 discord" /></el-form-item>
        <el-form-item label="适配器名称" prop="name"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="版本" prop="version"><el-input v-model="form.version" /></el-form-item>
        <el-form-item label="运行时" prop="runtime">
          <el-radio-group v-model="form.runtime">
            <el-radio-button value="nodejs">Node.js</el-radio-button>
            <el-radio-button value="python">Python</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="运行环境">
          <el-select v-model="form.runtime_profile" clearable placeholder="使用默认运行环境" style="width: 100%">
            <el-option
              v-for="profile in runtimeProfilesBy(form.runtime)"
              :key="profile.id"
              :label="runtimeProfileLabel(profile)"
              :value="profile.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="入口文件" prop="entry"><el-input v-model="form.entry" /></el-form-item>
        <el-form-item label="作者"><el-input v-model="form.author" /></el-form-item>
        <el-form-item label="介绍"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="配置参数" prop="schemaText">
          <el-input v-model="form.schemaText" type="textarea" :rows="7" spellcheck="false" placeholder="[]" />
          <div class="field-help">使用 JSON 数组描述实例配置字段。</div>
        </el-form-item>
        <el-form-item label="支持函数">
          <div class="capability-editor">
            <el-checkbox-group v-model="form.capabilities">
              <el-checkbox v-for="option in capabilityOptions" :key="option.value" :value="option.value">{{ option.label }}</el-checkbox>
            </el-checkbox-group>
            <div class="field-help">基础接口包含文本、图片、文件、用户/群信息查询和 @ 用户；列表显示 adapter.json 中声明的能力。</div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="configVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveDefinition">保存</el-button>
      </template>
    </el-dialog>

    <AdapterCodeEditor v-if="codeVisible" v-model="codeVisible" :platform="codePlatform" :title="codeTitle" @saved="loadPlatforms" />

    <el-dialog v-model="importVisible" title="导入适配器" width="720px" @closed="clearImportSelection">
      <input ref="importDirectoryInput" type="file" multiple webkitdirectory directory hidden @change="handleDirectorySelected" />
      <input ref="importArchiveInput" type="file" accept=".zip,application/zip" hidden @change="handleArchiveSelected" />
      <div class="import-actions">
        <el-button type="primary" @click="importDirectoryInput?.click()">选择适配器文件夹</el-button>
        <el-button @click="importArchiveInput?.click()">选择 ZIP 压缩包</el-button>
      </div>
      <div class="field-help">导入内容需包含 adapter.json 和其中声明的入口文件。</div>
      <div v-if="importState.sourceName" class="import-summary">
        <div><span class="meta-label">来源</span>{{ importState.sourceName }}</div>
        <div v-if="importState.manifest?.id"><span class="meta-label">适配器 ID</span>{{ importState.manifest.id }}</div>
        <div v-if="importState.files.length"><span class="meta-label">文件数</span>{{ importState.files.length }}　<span class="meta-label">总大小</span>{{ formatSize(importState.totalSize) }}</div>
      </div>
      <div v-if="importState.diagnostics.length" class="import-diagnostics">
        <div v-for="(item, index) in importState.diagnostics" :key="index" class="import-diagnostic">
          <el-tag size="small" :type="item.level === 'error' ? 'danger' : item.level === 'success' ? 'success' : 'info'">{{ item.level === 'error' ? '错误' : item.level === 'success' ? '通过' : '提示' }}</el-tag>
          <span>{{ item.message }}</span><code v-if="item.path">{{ item.path }}</code>
        </div>
      </div>
      <el-table v-if="importState.files.length" :data="importState.files" size="small" border max-height="260">
        <el-table-column prop="path" label="文件路径" min-width="260" show-overflow-tooltip />
        <el-table-column label="大小" width="110"><template #default="{ row }">{{ formatSize(row.size) }}</template></el-table-column>
      </el-table>
      <el-empty v-else-if="!importState.sourceName" description="请选择适配器文件夹或 ZIP 压缩包" />
      <template #footer>
        <el-button @click="importVisible = false">取消</el-button>
        <el-button type="primary" :loading="importSaving" :disabled="!importState.valid" @click="submitAdapterImport">导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
defineOptions({ name: 'AdapterManagement' })
import { computed, defineAsyncComponent, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Document, Plus, Refresh, Setting, Star, StarFilled, Upload } from '@element-plus/icons-vue'
import { createAdapterDefinition, deleteAdapterDefinition, getAdapterPlatforms, getRuntimeProfiles, importAdapterDefinition, reloadAdapterDefinition, saveAdapterDefinition, setAdapterPlatformPinned } from '@/api'
import StdPagination from '@/components/StdPagination.vue'
import { inspectAdapterArchive, inspectAdapterDirectory } from '@/utils/adapterImport'

const AdapterCodeEditor = defineAsyncComponent(() => import('@/components/AdapterCodeEditor.vue'))

const loading = ref(false)
const saving = ref(false)
const importSaving = ref(false)
const keyword = ref('')
const currentPage = ref(1)
const pageSize = 9
const platforms = ref([])
const runtimeProfiles = ref([])
const importVisible = ref(false)
const importDirectoryInput = ref(null)
const importArchiveInput = ref(null)
const importState = ref(createEmptyImportState())
const capabilityOptions = [
  { value: 'text', label: '发送文本' },
  { value: 'image', label: '发送图片' },
  { value: 'file', label: '发送文件' },
  { value: 'audio', label: '发送音频' },
  { value: 'video', label: '发送视频' },
  { value: 'markdown', label: '发送 Markdown' },
  { value: 'rich', label: '发送富文本' },
  { value: 'buttons', label: '发送按钮/混合内容' },
  { value: 'sequence', label: '消息序号' },
  { value: 'delete_message', label: '撤回消息' },
  { value: 'mute', label: '禁言成员' },
  { value: 'webhook', label: '接收 Webhook' },
  { value: 'private', label: '接收私聊消息' },
  { value: 'group', label: '接收群聊消息' },
  { value: 'mention', label: '@ 用户' }
]
const capabilityAliases = {
  send_text: 'text', send_image: 'image', send_audio: 'audio', send_video: 'video',
  send_file: 'file', send_markdown: 'markdown', send_rich: 'rich',
  send_buttons: 'buttons', send_mixed_content: 'buttons', send_text_sequence: 'sequence',
  send_message_sequence: 'sequence', private_message: 'private', group_message: 'group', at_user: 'mention'
}
const capabilityLabelsMap = {
  text: '发送文本', image: '发送图片', audio: '发送音频', video: '发送视频', file: '发送文件',
  markdown: '发送 Markdown', rich: '发送富文本', buttons: '发送按钮/混合内容', sequence: '消息序号',
  delete_message: '撤回消息', mute: '禁言成员', webhook: '接收 Webhook',
  private: '接收私聊消息', group: '接收群聊消息', mention: '@ 用户'
}
const configVisible = ref(false)
const codeVisible = ref(false)
const editing = ref(false)
const formRef = ref(null)
const form = reactive({ id: '', name: '', version: '1.0.0', runtime: 'nodejs', runtime_profile: '', entry: 'main.js', author: '', description: '', schemaText: '[]', capabilities: [] })
const codePlatform = ref('')
const codeTitle = ref('适配器代码')

const rules = {
  id: [{ required: true, message: '请输入适配器 ID', trigger: 'blur' }, { pattern: /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$/, message: 'ID 仅支持字母、数字、点、下划线和连字符', trigger: 'blur' }],
  name: [{ required: true, message: '请输入适配器名称', trigger: 'blur' }],
  version: [{ required: true, message: '请输入版本', trigger: 'blur' }],
  runtime: [{ required: true, message: '请选择运行时', trigger: 'change' }],
  entry: [{ required: true, message: '请输入入口文件', trigger: 'blur' }],
  schemaText: [{ validator: (_rule, value, done) => { try { if (!Array.isArray(JSON.parse(value))) throw new Error(); done() } catch { done(new Error('配置参数必须是 JSON 数组')) } }, trigger: 'blur' }]
}

const filteredPlatforms = computed(() => {
  const query = keyword.value.trim().toLowerCase()
  const items = platforms.value.filter(item => !query || [item.platform, item.display_name, item.version, item.runtime, item.author, item.description].filter(Boolean).join(' ').toLowerCase().includes(query))
  return items.sort((a, b) => Number(Boolean(b.pinned)) - Number(Boolean(a.pinned)) || String(a.display_name || a.platform).localeCompare(String(b.display_name || b.platform)))
})
const pageItems = computed(() => filteredPlatforms.value.slice((currentPage.value - 1) * pageSize, currentPage.value * pageSize))
watch(keyword, () => { currentPage.value = 1 })
watch(filteredPlatforms, () => { if (currentPage.value > Math.max(1, Math.ceil(filteredPlatforms.value.length / pageSize))) currentPage.value = 1 })
watch(() => form.runtime, runtime => {
  if (!form.runtime_profile) return
  const matched = runtimeProfiles.value.some(profile => profile.id === form.runtime_profile && profile.runtime === runtime)
  if (!matched) form.runtime_profile = ''
})

async function loadPlatforms() {
  loading.value = true
  try { const result = await getAdapterPlatforms(); platforms.value = Array.isArray(result) ? result : [] }
  finally { loading.value = false }
}

async function loadRuntimeProfiles() {
  try {
    const result = await getRuntimeProfiles()
    runtimeProfiles.value = Array.isArray(result) ? result : []
  } catch {
    runtimeProfiles.value = []
  }
}

function runtimeProfilesBy(runtime) {
  return runtimeProfiles.value.filter(profile => profile.runtime === runtime && profile.enabled)
}

function runtimeProfileLabel(profile) {
  const version = profile.requested_version || profile.version
  const versionLabel = version ? ` · ${version}` : ''
  return `${profile.name || profile.id}${versionLabel}${profile.default ? '（默认）' : ''}`
}

function runtimeLabel(item) {
  if (!item.runtime) return 'Go'
  const profile = runtimeProfiles.value.find(value => value.id === item.runtime_profile)
  return `${item.runtime === 'python' ? 'Python' : 'Node.js'}${profile ? ` · ${runtimeProfileLabel(profile)}` : ''}`
}

function createEmptyImportState() {
  return { sourceType: '', sourceName: '', files: [], archive: null, totalSize: 0, manifest: null, diagnostics: [], valid: false }
}

function formatSize(size) {
  const value = Number(size || 0)
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / (1024 * 1024)).toFixed(1)} MB`
}

async function handleDirectorySelected(event) {
  const files = Array.from(event.target?.files || [])
  importState.value = { ...createEmptyImportState(), sourceType: 'directory' }
  try {
    importState.value = { ...await inspectAdapterDirectory(files), sourceType: 'directory', archive: null }
  } catch (error) {
    importState.value.diagnostics = [{ level: 'error', message: error.message || '读取适配器文件夹失败' }]
  } finally {
    if (event.target) event.target.value = ''
  }
}

function handleArchiveSelected(event) {
  const archive = event.target?.files?.[0] || null
  importState.value = { ...inspectAdapterArchive(archive), sourceType: 'archive', archive }
  if (event.target) event.target.value = ''
}

function clearImportSelection() {
  importState.value = createEmptyImportState()
}

async function submitAdapterImport() {
  if (!importState.value.valid || importSaving.value) return
  const state = importState.value
  const formData = new FormData()
  formData.append('source_type', state.sourceType)
  if (state.manifest?.id) formData.append('adapter_id', state.manifest.id)
  if (state.manifest) formData.append('adapter_manifest', JSON.stringify(state.manifest))
  try {
    if (state.sourceType === 'directory') {
      formData.append('paths', JSON.stringify(state.files.map(item => item.path)))
      state.files.forEach(item => formData.append('files', item.file, item.file.name))
    } else if (state.sourceType === 'archive' && state.archive) {
      formData.append('archive', state.archive, state.archive.name)
    } else return
    importSaving.value = true
    await importAdapterDefinition(formData)
    ElMessage.success('适配器导入成功')
    importVisible.value = false
    await loadPlatforms()
  } catch (error) {
    const data = error?.response?.data
    const message = data?.msg || data?.error || data?.message || error?.message || '适配器导入失败'
    importState.value.diagnostics = [...importState.value.diagnostics.filter(item => item.level !== 'error'), { level: 'error', message }]
    importState.value.valid = false
    ElMessage.error(message)
  } finally {
    importSaving.value = false
  }
}

function openCreate() {
  editing.value = false
  Object.assign(form, { id: '', name: '', version: '1.0.0', runtime: 'nodejs', runtime_profile: '', entry: 'main.js', author: '', description: '', schemaText: '[]', capabilities: [] })
  configVisible.value = true
}

function openConfig(item) {
  editing.value = true
  Object.assign(form, { id: item.platform, name: item.display_name, version: item.version, runtime: item.runtime, runtime_profile: item.runtime_profile || '', entry: item.entry, author: item.author || '', description: item.description || '', schemaText: JSON.stringify(item.config_schema || [], null, 2), capabilities: readDeclaredCapabilities(item) })
  configVisible.value = true
}

function normalizeCapability(value) {
  const key = String(value || '').trim().toLowerCase()
  return capabilityAliases[key] || key
}

function readDeclaredCapabilities(item) {
  const declared = Array.isArray(item.declared_capabilities)
    ? item.declared_capabilities
    : Object.entries(item.capabilities || {}).filter(([, enabled]) => enabled).map(([key]) => key)
  const editable = new Set(capabilityOptions.map(option => option.value))
  return [...new Set(declared.map(normalizeCapability).filter(value => editable.has(value)))]
}

function capabilityLabels(item) {
  const labels = []
  if (item.custom) labels.push('查询用户信息', '查询群信息', '@ 用户')
  for (const [key, enabled] of Object.entries(item.capabilities || {})) {
    const label = capabilityLabelsMap[normalizeCapability(key)]
    if (enabled && label) labels.push(label)
  }
  const declared = Array.isArray(item.declared_capabilities) ? item.declared_capabilities : []
  for (const value of declared) {
    const label = capabilityLabelsMap[normalizeCapability(value)]
    if (label) labels.push(label)
  }
  return [...new Set(labels)]
}

function manifestFromForm() {
  return { id: form.id.trim(), name: form.name.trim(), version: form.version.trim(), runtime: form.runtime, entry: form.entry.trim(), author: form.author.trim(), description: form.description.trim(), protocol_version: '1', config_schema: JSON.parse(form.schemaText), capabilities: form.capabilities }
}

function defaultCode() {
  if (form.runtime === 'python') return "import os, sys\nsys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../../sdk/python')))\nfrom allbot_adapter import run_adapter\n\nclass Adapter:\n    def start(self, config, emit, context):\n        self.emit = emit\n        print('Adapter started', file=sys.stderr)\n    def stop(self):\n        pass\n    def send_text(self, data):\n        raise NotImplementedError('Connect this method to your platform')\n\nrun_adapter(Adapter())\n"
  return "const path = require('node:path');\nconst { runAdapter } = require(path.resolve(__dirname, '../../sdk/nodejs/allbot_adapter'));\n\nconst adapter = {\n  async start(config, emit, context) {\n    this.emit = emit;\n    console.error('Adapter started');\n  },\n  async stop() {},\n  async sendText(data) {\n    throw new Error('Connect this method to your platform');\n  }\n};\n\nrunAdapter(adapter);\n"
}

async function saveDefinition() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const manifest = manifestFromForm()
    const runtimeProfile = String(form.runtime_profile || '').trim()
    if (editing.value) await saveAdapterDefinition(form.id, manifest, runtimeProfile)
    else await createAdapterDefinition(manifest, defaultCode(), runtimeProfile)
    ElMessage.success(editing.value ? '适配器配置已保存' : '适配器定义已创建')
    configVisible.value = false
    await loadPlatforms()
  } finally { saving.value = false }
}

async function togglePinned(item) {
  await setAdapterPlatformPinned(item.platform, !item.pinned)
  ElMessage.success(item.pinned ? '已取消置顶' : '已置顶')
  await loadPlatforms()
}

async function reloadDefinition(item) {
  await reloadAdapterDefinition(item.platform)
  ElMessage.success('适配器定义已重载')
  await loadPlatforms()
}

async function removeDefinition(item) {
  await ElMessageBox.confirm(`删除「${item.display_name}」会同时删除该适配器的实例配置和 adapter.data，确定继续吗？`, '删除自定义适配器', { type: 'warning' })
  await deleteAdapterDefinition(item.platform)
  ElMessage.success('自定义适配器已删除')
  await loadPlatforms()
}

function openCode(item) {
  codePlatform.value = item.platform
  codeTitle.value = `${item.display_name} · 适配器文件`
  codeVisible.value = true
}

function clearValidation() { formRef.value?.clearValidate() }
onMounted(() => { loadPlatforms(); loadRuntimeProfiles() })
</script>

<style scoped>
.adapter-management,.page-shell{width:100%;height:100%;min-height:0;display:flex;flex-direction:column}.page-card{flex:1;min-height:0;display:flex;flex-direction:column;overflow:hidden}.page-card :deep(.el-card__body){flex:1;min-height:0;display:flex;flex-direction:column;gap:12px;overflow:hidden}.page-header{display:flex;justify-content:space-between;align-items:center;gap:16px}.title{font-size:18px;font-weight:600}.subtitle{margin-top:6px;color:#909399;font-size:13px}.header-actions{display:flex;align-items:center;gap:10px}.search-input{width:280px}.adapter-content{flex:1;min-height:0;overflow:auto;padding-right:2px}.adapter-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:14px}.adapter-card{min-height:204px}.adapter-card.pinned{border-color:#e6a23c}.card-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.identity{min-width:0}.adapter-name{overflow-wrap:anywhere;font-size:16px;font-weight:600;color:#303133}.adapter-id{margin-top:5px;color:#909399;font-size:12px;overflow-wrap:anywhere}.metadata-grid{display:grid;grid-template-columns:1fr 1fr 1fr;gap:8px;margin-top:18px}.metadata-grid>div{min-width:0;display:flex;flex-direction:column;gap:4px;font-size:13px;overflow-wrap:anywhere}.meta-label{color:#909399;font-size:12px}.meta-value{overflow-wrap:anywhere}.description{min-height:20px;margin-top:12px;color:#606266;font-size:13px;line-height:1.45;overflow-wrap:anywhere}.capabilities-block{display:flex;flex-direction:column;gap:6px;margin-top:12px}.capability-tags{display:flex;flex-wrap:wrap;gap:6px}.empty-capabilities{color:#909399;font-size:13px}.capability-editor{width:100%;min-width:0}.capability-editor :deep(.el-checkbox-group){display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:2px 8px}.card-actions{display:flex;flex-wrap:wrap;gap:6px;margin-top:16px;padding-top:10px;border-top:1px solid #ebeef5}.card-actions .el-button{margin-left:0}.readonly-tag{margin-left:auto}.field-help{margin-top:4px;color:#909399;font-size:12px;line-height:1.45}.import-actions{display:flex;flex-wrap:wrap;gap:8px}.import-summary{display:grid;gap:8px;margin:14px 0}.import-summary>div{display:flex;gap:12px;align-items:center;overflow-wrap:anywhere}.import-diagnostics{display:grid;gap:8px;margin:12px 0}.import-diagnostic{display:flex;align-items:center;gap:8px;flex-wrap:wrap;font-size:13px}.import-diagnostic code{color:#909399;overflow-wrap:anywhere}.code-editor{display:flex;flex-direction:column;gap:8px}.code-file{color:#606266;font-size:13px}.code-editor :deep(textarea){min-height:min(62vh,720px)!important;font-family:Consolas,'Courier New',monospace;font-size:13px;line-height:1.55;tab-size:2}@media(max-width:768px){.page-shell{height:calc(100dvh - 52px - 76px - 24px);overflow:hidden}.page-card{height:100%;border-radius:10px}.page-header{align-items:stretch;flex-direction:column;gap:10px}.title{font-size:16px}.subtitle{display:none}.header-actions{width:100%;align-items:stretch;flex-direction:column}.search-input,.header-actions .el-button{width:100%;margin-left:0}.adapter-grid{grid-template-columns:minmax(0,1fr);gap:12px}.adapter-card{min-height:auto}.metadata-grid{margin-top:14px}.capability-editor :deep(.el-checkbox-group){grid-template-columns:minmax(0,1fr)}}
</style>
