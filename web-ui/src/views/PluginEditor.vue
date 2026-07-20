<template>
  <div class="plugin-editor">
    <el-card class="editor-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <el-button size="small" @click="goBack"><el-icon><ArrowLeft /></el-icon>返回</el-button>
            <div>
              <span class="title">编辑插件：{{ pluginName || pluginId }}</span>
              <div v-if="hybridMode" class="subtitle">当前使用分类编辑；高级文件页可直接编辑插件目录中的全部文件。</div>
              <div v-else-if="templateFormActive" class="subtitle">分类保存会由模板重新生成插件入口文件和 plugin.json。</div>
              <div v-else-if="requiresConversion" class="subtitle">检测到旧账号模板，可转换为分类编辑，也可继续使用文件编辑。</div>
              <div v-else class="subtitle">当前插件使用文件编辑模式。</div>
            </div>
          </div>
          <div class="header-actions">
            <el-tag v-if="conversionMode" type="warning" effect="plain">待转换</el-tag>
            <el-tag v-else-if="templateEditable" type="success" effect="plain">模板编辑</el-tag>
            <span v-if="showFileEditor" class="current-file">{{ selectedPath || '请选择文件' }}</span>
            <el-button v-if="requiresConversion && !conversionMode" type="warning" @click="startConversion">转换为分类编辑</el-button>
            <el-button v-if="conversionMode" plain @click="useFileEditor">返回文件编辑</el-button>
            <el-button v-if="showFileEditor" type="danger" plain :loading="deleting" :disabled="!selectedPath" @click="deleteSelected">删除</el-button>
            <el-button v-if="showFileEditor" type="primary" :loading="saving" :disabled="!canEdit" @click="saveCode">保存文件</el-button>
            <el-button v-else type="primary" :loading="saving" @click="saveTemplate">{{ conversionMode ? '确认转换并保存' : '保存模板' }}</el-button>
          </div>
        </div>
      </template>

      <div v-loading="loading" class="editor-body">
        <el-alert v-if="requiresConversion && !conversionMode" class="mode-alert" type="warning" :closable="false" show-icon title="该旧账号模板尚未纳入分类编辑管理">
          <template #default>
            <div>{{ templateProbeMessage || '可先预填现有基础与青龙配置，再手动补齐所有必需代码后转换。' }}</div>
            <div class="alert-actions"><el-button type="warning" size="small" @click="startConversion">转换为分类编辑</el-button><span>暂不转换时仍可继续编辑原始文件。</span></div>
          </template>
        </el-alert>
        <el-alert v-else-if="templateProbeMessage && !templateFormActive" class="mode-alert" type="info" :closable="false" show-icon :title="templateProbeMessage" />
        <el-alert v-if="templateFormActive && !hybridMode && activeTab !== 'files'" class="generated-alert" :type="conversionMode ? 'warning' : 'info'" :closable="false" show-icon title="模板生成范围">
          <template #default>保存后，插件入口文件和 plugin.json 将由当前分类表单重新生成。高级文件编辑对这些生成文件的修改可能被覆盖。</template>
        </el-alert>

        <el-tabs v-if="hybridMode" v-model="activeTab" class="template-tabs" @tab-change="handleTabChange">
          <el-tab-pane label="基础" name="base">
            <div class="form-scroll"><el-form :model="hybridPluginForm" label-width="140px" class="template-form">
              <el-form-item label="插件 ID"><el-input :model-value="hybridPluginFixed.id" disabled /></el-form-item>
              <el-form-item label="模板"><el-input :model-value="hybridPluginFixed.template" disabled /></el-form-item>
              <el-form-item label="运行时"><el-input :model-value="hybridPluginFixed.runtime" disabled /></el-form-item>
              <el-form-item label="插件名称"><el-input v-model="hybridPluginForm.name" /></el-form-item>
              <el-form-item label="版本"><el-input v-model="hybridPluginForm.version" /></el-form-item>
              <el-form-item label="运行环境"><el-input v-model="hybridPluginForm.runtime_profile" placeholder="留空使用默认运行环境" /></el-form-item>
              <el-form-item label="优先级"><el-input-number v-model="hybridPluginForm.priority" :step="1" /></el-form-item>
              <el-form-item label="支持平台"><el-select v-model="hybridPluginForm.platforms" multiple allow-create filterable style="width:100%"><el-option v-for="platform in platformOptions" :key="platform.value" :label="platform.label" :value="platform.value" /></el-select></el-form-item>
              <el-form-item label="读取脚本变量"><el-switch v-model="hybridPluginForm.script_env.enabled" /></el-form-item>
              <el-form-item label="脚本变量名"><el-select v-model="hybridPluginForm.script_env.names" multiple allow-create filterable clearable :disabled="!hybridPluginForm.script_env.enabled" style="width:100%" placeholder="为空时读取全部变量" /></el-form-item>
              <el-form-item label="启用状态"><el-switch v-model="hybridPluginForm.enabled" /></el-form-item>
              <el-divider content-position="left">模板兼容信息（只读）</el-divider>
              <el-form-item label="授权类型"><el-input :model-value="hybridDisplay.authorizationType" disabled /></el-form-item>
              <el-form-item label="override_builtin"><el-input :model-value="hybridDisplay.overrideBuiltin" disabled /></el-form-item>
              <el-form-item label="自定义指令"><el-input :model-value="hybridDisplay.customCommands" type="textarea" :rows="4" disabled /></el-form-item>
              <el-form-item label="task_scripts"><el-input :model-value="hybridDisplay.taskScripts" type="textarea" :rows="5" disabled /><div class="field-tip">reference_existing 脚本只引用已有文件，不由模板编辑器覆盖。</div></el-form-item>
            </el-form></div>
          </el-tab-pane>
          <el-tab-pane label="分类代码" name="sections">
            <div class="hybrid-sections">
              <nav v-if="hybridSectionGroups.length" class="hybrid-category-nav">
                <button class="category-arrow" :disabled="hybridCategoryIndex <= 0" @click="moveHybridCategory(-1)"><el-icon><ArrowLeft /></el-icon></button>
                <div ref="hybridCategoryScroll" class="hybrid-category-scroll">
                  <button v-for="(group, index) in hybridSectionGroups" :key="group.category" class="category-tab" :class="{ active: index === hybridCategoryIndex }" @click="selectHybridCategory(index)">{{ hybridCategoryLabel(group.category) }}</button>
                </div>
                <button class="category-arrow" :disabled="hybridCategoryIndex >= hybridSectionGroups.length - 1" @click="moveHybridCategory(1)"><el-icon><ArrowRight /></el-icon></button>
              </nav>
              <div class="form-scroll hybrid-section-scroll">
                <section v-if="currentHybridSectionGroup" :key="currentHybridSectionGroup.category" class="hybrid-section-group">
                  <div v-for="section in currentHybridSectionGroup.sections" :key="section.id" class="hybrid-section-card">
                    <div class="hybrid-section-heading">
                      <div><strong>{{ section.label }}</strong><span class="section-path">{{ section.path }}</span></div>
                      <el-tag v-if="isHybridSectionReadOnly(section)" type="warning" effect="plain">只读</el-tag>
                      <el-tag v-else type="success" effect="plain">可编辑</el-tag>
                    </div>
                    <el-alert v-if="hybridSectionNote(section)" :type="isHybridSectionReadOnly(section) ? 'warning' : 'info'" :closable="false" :title="hybridSectionNote(section)" />
                    <div :ref="element => setHybridSectionEditorContainer(section.id, element)" class="hybrid-section-editor"></div>
                  </div>
                </section>
                <el-empty v-else description="暂无分类代码" />
              </div>
            </div>
          </el-tab-pane>
          <el-tab-pane label="文件 / 高级编辑" name="files" />
        </el-tabs>

        <el-tabs v-if="templateFormActive && !hybridMode" v-model="activeTab" class="template-tabs" @tab-change="handleTabChange">
          <el-tab-pane label="基础" name="base">
            <div class="form-scroll"><el-form :model="templateForm" label-width="130px" class="template-form">
              <el-form-item label="插件名称" required><el-input v-model="templateForm.name" /></el-form-item>
              <el-form-item label="版本"><el-input v-model="templateForm.version" /></el-form-item>
              <el-form-item label="插件运行时"><el-input :model-value="runtimeLabel" disabled /></el-form-item>
              <el-form-item label="运行环境"><el-input v-model="templateForm.runtime_profile" placeholder="留空使用默认运行环境" /></el-form-item>
              <el-form-item label="优先级"><el-input-number v-model="templateForm.priority" :step="1" /></el-form-item>
              <el-form-item label="支持平台"><el-select v-model="templateForm.platforms" multiple allow-create filterable style="width:100%"><el-option v-for="platform in platformOptions" :key="platform.value" :label="platform.label" :value="platform.value" /></el-select></el-form-item>
              <el-form-item label="读取脚本变量"><el-switch v-model="templateForm.script_env.enabled" /></el-form-item>
              <el-form-item label="脚本变量名"><el-select v-model="templateForm.script_env.names" multiple allow-create filterable clearable :disabled="!templateForm.script_env.enabled" style="width:100%" placeholder="为空时读取全部变量" /></el-form-item>
              <el-form-item label="启用状态"><el-switch v-model="templateForm.enabled" /></el-form-item>
            </el-form></div>
          </el-tab-pane>

          <el-tab-pane label="青龙" name="account">
            <div class="form-scroll"><el-form :model="templateForm.account_ql" label-width="150px" class="template-form">
              <el-form-item label="指令前缀" required><el-input v-model="templateForm.account_ql.prefix" /></el-form-item>
              <el-form-item label="触发规则"><el-input :model-value="accountQLTriggerPreview(templateForm.account_ql)" readonly /><div class="field-tip">由指令前缀、内置指令和自定义指令自动生成。</div></el-form-item>
              <el-form-item label="账号表名" required><el-input v-model="templateForm.account_ql.table_name" /><div class="field-tip">只能使用英文、数字和下划线，且不能以数字开头。</div></el-form-item>
              <el-form-item label="青龙变量名" required><el-input v-model="templateForm.account_ql.env_name" /></el-form-item>
              <el-form-item label="青龙脚本语言"><el-select v-model="templateForm.account_ql.script_runtime"><el-option label="Node.js 脚本" value="nodejs" /><el-option label="Python 脚本" value="python" /></el-select></el-form-item>
              <el-form-item label="青龙脚本路径" required><el-input v-model="templateForm.account_ql.task_script" /></el-form-item>
              <el-form-item label="授权价格"><el-input-number v-model="templateForm.account_ql.auth_price_per_month" :min="0" /></el-form-item>
              <el-form-item label="运行定时 cron"><el-input v-model="templateForm.account_ql.cron" /></el-form-item>
              <el-form-item label="运行超时秒数"><el-input-number v-model="templateForm.account_ql.run_wait_timeout" :min="1" :step="60" /></el-form-item>
              <el-form-item label="定时等待完成"><el-switch v-model="templateForm.account_ql.wait_scheduled" /></el-form-item>
              <el-form-item label="启用完成钩子"><el-switch v-model="templateForm.account_ql.enable_after_run" /></el-form-item>
              <el-form-item label="启用 CK 检测"><el-switch v-model="templateForm.account_ql.enable_ck_check" /></el-form-item>
              <el-form-item v-if="templateForm.account_ql.enable_ck_check" label="CK 检测 cron"><el-input v-model="templateForm.account_ql.ck_check_cron" /></el-form-item>
              <el-form-item label="启用过期检测"><el-switch v-model="templateForm.account_ql.enable_expire_check" /></el-form-item>
              <template v-if="templateForm.account_ql.enable_expire_check">
                <el-form-item label="过期检测 cron"><el-input v-model="templateForm.account_ql.expire_check_cron" /></el-form-item>
                <el-form-item label="提醒天数"><el-input v-model="templateForm.account_ql.expire_notify_days" placeholder="7,3,1,0" /></el-form-item>
                <el-form-item label="删除天数"><el-input-number v-model="templateForm.account_ql.expire_delete_after_days" :min="-1" /></el-form-item>
              </template>
            </el-form></div>
          </el-tab-pane>

          <el-tab-pane label="自定义代码" name="code">
            <div class="code-grid form-scroll">
              <section><div class="section-title">登录解析代码</div><div ref="parseInputEditorContainer" class="template-code-editor"></div></section>
              <section><div class="section-title">查询代码</div><div ref="queryEditorContainer" class="template-code-editor"></div></section>
              <section v-if="templateForm.account_ql.enable_after_run"><div class="section-title">一键运行完成钩子</div><div ref="afterRunEditorContainer" class="template-code-editor"></div></section>
              <section v-if="templateForm.account_ql.enable_ck_check"><div class="section-title">CK 检测代码</div><div ref="checkCkEditorContainer" class="template-code-editor"></div></section>
            </div>
          </el-tab-pane>

          <el-tab-pane label="自定义指令" name="routes">
            <div class="form-scroll route-panel">
              <div class="route-toolbar"><span>自定义指令会同步更新触发规则和模板 routes。</span><el-button type="primary" size="small" @click="addRoute">添加指令</el-button></div>
              <div v-for="(route, index) in templateForm.account_ql.routes" :key="route.id || index" class="route-item">
                <div class="route-row"><el-input v-model="route.command" placeholder="指令" /><el-input v-model="route.function_name" placeholder="函数名" /><el-input v-model="route.description" placeholder="描述" /><el-button type="danger" plain @click="removeRoute(index)">删除</el-button></div>
                <el-input v-model="route.code" type="textarea" :rows="9" placeholder="自定义指令函数代码" />
              </div>
              <el-empty v-if="!templateForm.account_ql.routes.length" description="暂无自定义指令" />
            </div>
          </el-tab-pane>

          <el-tab-pane label="文件 / 高级编辑" name="files" />
        </el-tabs>

        <div v-if="showFileEditor" class="editor-layout" :class="{ 'file-panel-collapsed': isFilePanelCollapsed, 'inside-template-tabs': templateFormActive }">
          <aside class="file-panel">
            <div class="file-panel-header">
              <div class="file-panel-title">插件目录</div>
              <el-button size="small" text class="mobile-collapse-button" @click="toggleFilePanel"><el-icon><component :is="isFilePanelCollapsed ? ArrowDown : ArrowUp" /></el-icon>{{ isFilePanelCollapsed ? '展开' : '收起' }}</el-button>
              <div class="file-panel-actions">
                <el-button size="small" plain :loading="exporting" @click="exportPluginDirectory">导出</el-button>
                <el-dropdown trigger="click" @command="openCreateDialog"><el-button size="small" type="primary" plain>新建</el-button><template #dropdown><el-dropdown-menu><el-dropdown-item command="file">新建文件</el-dropdown-item><el-dropdown-item command="directory">新建文件夹</el-dropdown-item></el-dropdown-menu></template></el-dropdown>
              </div>
            </div>
            <el-tree ref="fileTreeRef" :data="fileTree" node-key="path" :props="treeProps" :default-expanded-keys="expandedKeys" :current-node-key="selectedPath" highlight-current @node-click="handleNodeClick">
              <template #default="{ data }">
                <span class="tree-node" :class="{ muted: data.type === 'file' && !data.text }">
                  <el-icon v-if="data.type === 'directory'"><Folder /></el-icon><el-icon v-else><Document /></el-icon><span>{{ data.name }}</span>
                </span>
              </template>
            </el-tree>
          </aside>
          <main class="editor-main">
            <el-empty v-if="!selectedPath" description="请选择左侧文件" />
            <el-result v-else-if="!canEdit" icon="warning" title="该文件无法在线读取" :sub-title="selectedPath" />
            <div v-show="selectedPath && canEdit" ref="editorContainer" class="code-editor-container"></div>
          </main>
        </div>
      </div>
    </el-card>

    <el-dialog v-model="createDialogVisible" :title="createForm.type === 'directory' ? '新建文件夹' : '新建文件'" width="480px">
      <el-form :model="createForm" label-width="90px"><el-form-item label="类型"><el-radio-group v-model="createForm.type"><el-radio-button label="file">文件</el-radio-button><el-radio-button label="directory">文件夹</el-radio-button></el-radio-group></el-form-item><el-form-item label="路径"><el-input v-model="createForm.path" :placeholder="createForm.type === 'directory' ? '例如：lib/utils' : '例如：lib/helper.js'" /><div class="field-tip">相对当前插件目录，支持输入多级目录。</div></el-form-item></el-form>
      <template #footer><el-button @click="createDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="createEntry">创建</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
defineOptions({ name: 'PluginEditor' })
import { computed, h, ref, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, ArrowLeft, ArrowRight, ArrowUp, Document, Folder } from '@element-plus/icons-vue'
import request from '@/utils/request'
import { convertPluginTemplateEditor, getPluginTemplateEditor, updatePluginTemplateEditor } from '@/api'
import { useAuthStore } from '@/stores/auth'
import {
  accountQLTriggerPreview,
  createEmptyAccountQLConfig,
  defaultRouteCode,
  defaultRouteFunctionName,
  buildHybridTemplateSource,
  cloneTemplateValue,
  hybridCategoryLabel,
  hybridFileDiffs,
  hybridOwnershipLabel,
  hybridPluginDiffs,
  hybridSectionDiffs,
  hybridSectionIsReadOnly,
  isAccountQLTemplate,
  isHybridTemplateSource,
  normalizeAccountQLConfig,
  normalizeHybridTemplateSource
} from '@/utils/accountQlTemplate'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { python } from '@codemirror/lang-python'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const pluginId = ref(route.params.id)
const pluginName = ref('')
const entryFile = ref('')
const selectedPath = ref('')
const selectedDirectory = ref('')
const fileTree = ref([])
const expandedKeys = ref([])
const loading = ref(false)
const saving = ref(false)
const creating = ref(false)
const deleting = ref(false)
const exporting = ref(false)
const canEdit = ref(false)
const isFilePanelCollapsed = ref(false)
const editorContainer = ref(null)
const fileTreeRef = ref(null)
const createDialogVisible = ref(false)
const createForm = ref({ type: 'file', path: '' })
const treeProps = { children: 'children', label: 'name' }
const templateEditable = ref(false)
const requiresConversion = ref(false)
const conversionMode = ref(false)
const conversionSource = ref(null)
const templateProbeMessage = ref('')
const templateSourceChanged = ref(false)
const templateGeneratedFiles = ref([])
const hybridMode = ref(false)
const hybridSourceInitial = ref(null)
const hybridSource = ref(null)
const hybridPluginForm = ref({})
const hybridSections = ref([])
const hybridExternalDriftPaths = ref([])
const hybridSectionEditorContainers = new Map()
const hybridSectionEditorViews = new Map()
const hybridCategoryIndex = ref(0)
const hybridCategoryScroll = ref(null)
const selectedFileSHA256 = ref('')
const activeTab = ref('base')
const templateForm = ref(createTemplateForm())
const parseInputEditorContainer = ref(null)
const queryEditorContainer = ref(null)
const afterRunEditorContainer = ref(null)
const checkCkEditorContainer = ref(null)
const hybridPluginFieldLabels = {
  name: '插件名称',
  version: '版本',
  runtime_profile: '运行环境',
  priority: '优先级',
  platforms: '支持平台',
  enabled: '启用状态',
  script_env: '脚本变量'
}
const platformOptions = [
  { label: 'QQ', value: 'qq' },
  { label: 'QQ 官方机器人', value: 'qq_office' },
  { label: '微信', value: 'wechat' },
  { label: 'Telegram', value: 'telegram' },
  { label: '钉钉', value: 'dingtalk' },
  { label: '飞书', value: 'feishu' },
  { label: '微信公众号', value: 'wechat_official' }
]
const templateFormActive = computed(() => templateEditable.value || conversionMode.value || hybridMode.value)
const showFileEditor = computed(() => !templateFormActive.value || activeTab.value === 'files')
const runtimeLabel = computed(() => templateForm.value.runtime === 'python' ? 'Python' : 'Node.js')
const hybridPluginFixed = computed(() => {
  const plugin = hybridSource.value?.plugin || {}
  return { id: String(plugin.id || pluginId.value), runtime: String(plugin.runtime || ''), template: String(plugin.template || hybridSource.value?.template || '') }
})
const hybridSectionGroups = computed(() => {
  const groups = new Map()
  for (const section of hybridSections.value) {
    const category = String(section.category || 'other')
    if (!groups.has(category)) groups.set(category, { category, sections: [] })
    groups.get(category).sections.push(section)
  }
  return [...groups.values()]
})
const currentHybridSectionGroup = computed(() => hybridSectionGroups.value[hybridCategoryIndex.value] || null)
const hybridDisplay = computed(() => {
  const compatibility = hybridSource.value?.compatibility || {}
  const migration = hybridSource.value?.migration || {}
  const taskScripts = hybridSource.value?.task_scripts || {}
  const commands = compatibility.custom_commands ?? migration.custom_commands ?? compatibility.commands ?? []
  return {
    authorizationType: String(compatibility.authorization_type || compatibility.auth_type || migration.authorization_type || '未提供'),
    overrideBuiltin: compatibility.override_builtin === undefined ? '未提供' : (compatibility.override_builtin ? '是' : '否'),
    customCommands: Array.isArray(commands) ? commands.join('、') : JSON.stringify(commands, null, 2),
    taskScripts: JSON.stringify(taskScripts, null, 2)
  }
})
function isHybridSectionReadOnly(section) { return hybridSectionIsReadOnly(section) }
function hybridSectionNote(section) {
  if (section?.read_only_reason) return String(section.read_only_reason)
  return isHybridSectionReadOnly(section) ? `文件归属：${hybridOwnershipLabel(section?.ownership)}` : ''
}
let editorView = null
let parseInputEditorView = null
let queryEditorView = null
let afterRunEditorView = null
let checkCkEditorView = null

function createTemplateForm() {
  return {
    id: String(pluginId.value || ''),
    name: '',
    version: '1.0.0',
    runtime: 'nodejs',
    runtime_profile: '',
    priority: 0,
    platforms: [],
    enabled: true,
    template: 'nodejs_account_ql',
    script_env: { enabled: false, names: [] },
    account_ql: createEmptyAccountQLConfig('nodejs')
  }
}

const loadFiles = async (preferredPath = '') => {
  loading.value = true
  try {
    const data = await request.get(`/plugins/files/${pluginId.value}`)
    pluginName.value = data.plugin_name || pluginId.value
    entryFile.value = data.entry || ''
    fileTree.value = data.tree || []
    expandedKeys.value = collectDirectories(fileTree.value)

    const firstPath = findPath(fileTree.value, preferredPath) || findPath(fileTree.value, entryFile.value) || findFirstTextFile(fileTree.value)
    if (firstPath) await openFile(firstPath)
  } catch (error) {
    console.error('加载插件目录失败:', error)
    ElMessage.error('加载插件目录失败: ' + errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function probeTemplateEditor() {
  try {
    const result = await getPluginTemplateEditor(pluginId.value)
    const model = unwrapTemplateModel(result)
    const source = model?.template_source && typeof model.template_source === 'object' ? model.template_source : model
    hybridMode.value = isHybridTemplateSource(source)
    if (hybridMode.value) initializeHybridModel(source, model, result)
    templateEditable.value = !hybridMode.value && Boolean(result?.editable ?? model?.editable)
    requiresConversion.value = !templateEditable.value && !hybridMode.value && Boolean(result?.requires_conversion ?? model?.requires_conversion)
    conversionSource.value = result?.conversion_source || model?.conversion_source || null
    conversionMode.value = false
    templateProbeMessage.value = String(result?.reason || result?.message || '')
    templateSourceChanged.value = Boolean(result?.source_changed ?? model?.source_changed ?? (Array.isArray(result?.modified_files) && result.modified_files.length))
    templateGeneratedFiles.value = Array.isArray(result?.files) ? result.files : (Array.isArray(source?.files) ? source.files : (Array.isArray(model?.files) ? model.files : []))
    if (hybridMode.value || templateEditable.value) {
      destroyEditor()
      if (hybridMode.value) {
        pluginName.value = String(source?.plugin?.name || pluginName.value || pluginId.value)
      } else {
        templateForm.value = normalizeTemplateForm(model)
        pluginName.value = templateForm.value.name || pluginName.value
      }
      activeTab.value = 'base'
    }
  } catch (error) {
    templateEditable.value = false
    requiresConversion.value = false
    conversionMode.value = false
    conversionSource.value = null
    const status = error?.response?.status
    templateProbeMessage.value = status === 404
      ? '该插件暂不支持模板分类编辑，已保留文件编辑模式。'
      : `模板编辑模型探测失败，已使用文件编辑模式：${errorMessage(error)}`
  }
}

function unwrapTemplateModel(result = {}) {
  return result.model || result.data || result.source || result.template_source || result
}

function initializeHybridModel(source, model = {}, result = {}) {
  destroyHybridSectionEditors(true)
  hybridCategoryIndex.value = 0
  const normalized = normalizeHybridTemplateSource(source, pluginId.value)
  hybridSourceInitial.value = cloneTemplateValue(source)
  hybridSource.value = normalized
  hybridPluginForm.value = {
    ...cloneTemplateValue(normalized.plugin),
    id: String(normalized.plugin.id || pluginId.value),
    runtime: String(normalized.plugin.runtime || ''),
    template: String(normalized.plugin.template || normalized.template || '')
  }
  hybridSections.value = normalized.sections.map(section => (section && typeof section === 'object' && !Array.isArray(section) ? { ...section } : section))
  hybridExternalDriftPaths.value = Array.isArray(result?.modified_files) ? result.modified_files : (Array.isArray(model?.modified_files) ? model.modified_files : [])
  templateGeneratedFiles.value = normalized.files
  templateSourceChanged.value = hybridExternalDriftPaths.value.length > 0
}

function normalizeTemplateForm(source = {}) {
  const templateSource = source.template_source && typeof source.template_source === 'object' ? source.template_source : source
  const config = source.config && typeof source.config === 'object' ? source.config : {}
  const plugin = templateSource.plugin && typeof templateSource.plugin === 'object' ? templateSource.plugin : config
  const accountQL = templateSource.account_ql || templateSource.accountQL || config.account_ql || {}
  const template = String(templateSource.template || config.template || '')
  const runtime = plugin.runtime || templateSource.runtime || config.runtime || (template === 'python_account_ql' ? 'python' : 'nodejs')
  return {
    id: String(source.id || source.plugin_id || config.id || pluginId.value),
    name: String(plugin.name || source.name || pluginName.value || pluginId.value),
    version: String(plugin.version || source.version || '1.0.0'),
    runtime: runtime === 'python' ? 'python' : 'nodejs',
    runtime_profile: String(plugin.runtime_profile || source.runtime_profile || ''),
    priority: Number(plugin.priority ?? source.priority ?? 0),
    platforms: Array.isArray(plugin.platforms) ? plugin.platforms : [],
    enabled: plugin.enabled ?? source.enabled ?? true,
    template: isAccountQLTemplate(template) ? template : (runtime === 'python' ? 'python_account_ql' : 'nodejs_account_ql'),
    script_env: normalizeScriptEnv(plugin.script_env || source.script_env),
    account_ql: normalizeAccountQLConfig(accountQL, runtime)
  }
}

function normalizeScriptEnv(source = {}) {
  const names = Array.isArray(source?.names) ? source.names : []
  return { enabled: Boolean(source?.enabled), names: names.map(name => String(name || '').trim()).filter(Boolean) }
}

function startConversion() {
  if (!conversionSource.value) {
    ElMessage.error('后端未返回旧模板转换预填数据，暂时无法开始转换')
    return
  }
  destroyEditor()
  destroyTemplateEditors()
  templateEditable.value = false
  templateForm.value = normalizeTemplateForm(conversionSource.value)
  clearConversionCode()
  pluginName.value = templateForm.value.name || pluginName.value
  conversionMode.value = true
  activeTab.value = 'base'
  templateProbeMessage.value = '请补齐所有启用的自定义代码和每个自定义指令代码后再提交转换。'
}

function clearConversionCode() {
  const accountQL = templateForm.value.account_ql
  accountQL.parse_input_code = ''
  accountQL.query_code = ''
  accountQL.after_run_code = ''
  accountQL.check_ck_code = ''
  accountQL.routes = (accountQL.routes || []).map(route => ({ ...route, code: '' }))
}

async function useFileEditor() {
  destroyTemplateEditors()
  conversionMode.value = false
  templateEditable.value = false
  activeTab.value = 'files'
  templateProbeMessage.value = '仍使用文件编辑模式；未转换不会生成或覆盖模板文件。'
  await nextTick()
  if (selectedPath.value && canEdit.value) await openFile(selectedPath.value)
}

const openCreateDialog = (type) => {
  createForm.value = { type, path: defaultCreatePath(type) }
  createDialogVisible.value = true
}

const toggleFilePanel = async () => {
  isFilePanelCollapsed.value = !isFilePanelCollapsed.value
  await nextTick()
  editorView?.requestMeasure?.()
}

const createEntry = async () => {
  const path = normalizeCreatePath(createForm.value.path)
  if (!path) {
    ElMessage.warning('请输入路径')
    return
  }
  creating.value = true
  try {
    const payload = { type: createForm.value.type, path }
    await request.post(`/plugins/files/${pluginId.value}`, payload)
    ElMessage.success(createForm.value.type === 'directory' ? '文件夹已创建' : '文件已创建')
    createDialogVisible.value = false
    await loadFiles(createForm.value.type === 'file' ? path : '')
  } catch (error) {
    console.error('创建失败:', error)
    ElMessage.error('创建失败: ' + errorMessage(error))
  } finally {
    creating.value = false
  }
}

const openFile = async (path) => {
  if (!path) return
  loading.value = true
  try {
    const data = await request.get(`/plugins/files/${pluginId.value}`, { params: { path } })
    selectedPath.value = data.path || path
    fileTreeRef.value?.setCurrentKey(selectedPath.value)
    selectedDirectory.value = parentPath(selectedPath.value)
    selectedFileSHA256.value = String(data.sha256 || '')
    canEdit.value = Boolean(data.editable)
    destroyEditor()
    if (canEdit.value) {
      await nextTick()
      createEditor(data.code || '', selectedPath.value)
    }
  } catch (error) {
    console.error('加载文件失败:', error)
    ElMessage.error('加载文件失败: ' + errorMessage(error))
  } finally {
    loading.value = false
  }
}

const createEditor = (code, path) => {
  if (!editorContainer.value) return
  const extensions = [basicSetup, languageFor(path), oneDark, EditorView.lineWrapping]
  editorView = new EditorView({ doc: code, extensions, parent: editorContainer.value })
}

function createTemplateCodeEditor(container, code, update) {
  if (!container) return null
  return new EditorView({
    doc: String(code || ''),
    extensions: [
      basicSetup,
      templateForm.value.runtime === 'python' ? python() : javascript(),
      oneDark,
      EditorView.lineWrapping,
      EditorView.updateListener.of((event) => {
        if (event.docChanged) update(event.state.doc.toString())
      })
    ],
    parent: container
  })
}

function setHybridSectionEditorContainer(sectionId, element) {
  const key = String(sectionId || '')
  if (!key) return
  if (element) hybridSectionEditorContainers.set(key, element)
  else hybridSectionEditorContainers.delete(key)
}

function hybridEditorRuntime(section = {}) {
  const declared = String(section.language || section.runtime || '').trim().toLowerCase()
  if (declared === 'python' || declared === 'py') return 'python'
  if (declared === 'nodejs' || declared === 'node' || declared === 'javascript' || declared === 'js') return 'nodejs'
  const path = String(section.path || '').trim().toLowerCase()
  if (path.endsWith('.py')) return 'python'
  if (path.endsWith('.js') || path.endsWith('.mjs') || path.endsWith('.cjs')) return 'nodejs'
  return String(hybridPluginFixed.value.runtime || '').toLowerCase() === 'python' ? 'python' : 'nodejs'
}

function createHybridSectionEditor(section, container) {
  const state = EditorState.create({
    doc: String(section.content ?? ''),
    extensions: [
      basicSetup,
      hybridEditorRuntime(section) === 'python' ? python() : javascript(),
      oneDark,
      EditorView.lineWrapping,
      EditorState.readOnly.of(isHybridSectionReadOnly(section)),
      EditorView.editable.of(!isHybridSectionReadOnly(section)),
      EditorView.updateListener.of((event) => {
        if (event.docChanged) section.content = event.state.doc.toString()
      })
    ]
  })
  return new EditorView({ state, parent: container })
}

async function ensureHybridSectionEditors() {
  if (!hybridMode.value || activeTab.value !== 'sections') return
  await nextTick()
  const sections = currentHybridSectionGroup.value?.sections || []
  const activeIds = new Set(sections.map(section => String(section.id || '')))
  for (const [key, view] of hybridSectionEditorViews) {
    if (!activeIds.has(key) || !hybridSectionEditorContainers.has(key)) {
      view.destroy()
      hybridSectionEditorViews.delete(key)
    }
  }
  for (const section of sections) {
    const key = String(section.id || '')
    const container = hybridSectionEditorContainers.get(key)
    if (key && container && !hybridSectionEditorViews.has(key)) {
      hybridSectionEditorViews.set(key, createHybridSectionEditor(section, container))
    }
  }
}

function syncHybridSectionEditors() {
  const sections = new Map(hybridSections.value.map(section => [String(section.id || ''), section]))
  for (const [key, view] of hybridSectionEditorViews) {
    const section = sections.get(key)
    if (section) section.content = view.state.doc.toString()
  }
}

function destroyHybridSectionEditors(clearContainers = false) {
  syncHybridSectionEditors()
  for (const view of hybridSectionEditorViews.values()) view.destroy()
  hybridSectionEditorViews.clear()
  if (clearContainers) hybridSectionEditorContainers.clear()
}

async function selectHybridCategory(index) {
  const nextIndex = Math.max(0, Math.min(index, hybridSectionGroups.value.length - 1))
  if (nextIndex === hybridCategoryIndex.value) return
  destroyHybridSectionEditors(true)
  hybridCategoryIndex.value = nextIndex
  await nextTick()
  await ensureHybridSectionEditors()
  const active = hybridCategoryScroll.value?.querySelector('.category-tab.active')
  active?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest', inline: 'nearest' })
}

function moveHybridCategory(direction) {
  selectHybridCategory(hybridCategoryIndex.value + direction)
}

async function ensureTemplateEditors() {
  if (!templateFormActive.value || activeTab.value !== 'code') return
  const accountQL = templateForm.value.account_ql
  if (!accountQL.enable_after_run && afterRunEditorView) {
    afterRunEditorView.destroy()
    afterRunEditorView = null
  }
  if (!accountQL.enable_ck_check && checkCkEditorView) {
    checkCkEditorView.destroy()
    checkCkEditorView = null
  }
  await nextTick()
  if (!parseInputEditorView) parseInputEditorView = createTemplateCodeEditor(parseInputEditorContainer.value, accountQL.parse_input_code, code => { accountQL.parse_input_code = code })
  if (!queryEditorView) queryEditorView = createTemplateCodeEditor(queryEditorContainer.value, accountQL.query_code, code => { accountQL.query_code = code })
  if (accountQL.enable_after_run && !afterRunEditorView) afterRunEditorView = createTemplateCodeEditor(afterRunEditorContainer.value, accountQL.after_run_code, code => { accountQL.after_run_code = code })
  if (accountQL.enable_ck_check && !checkCkEditorView) checkCkEditorView = createTemplateCodeEditor(checkCkEditorContainer.value, accountQL.check_ck_code, code => { accountQL.check_ck_code = code })
}

function syncTemplateEditors() {
  const accountQL = templateForm.value.account_ql
  if (parseInputEditorView) accountQL.parse_input_code = parseInputEditorView.state.doc.toString()
  if (queryEditorView) accountQL.query_code = queryEditorView.state.doc.toString()
  if (afterRunEditorView) accountQL.after_run_code = afterRunEditorView.state.doc.toString()
  if (checkCkEditorView) accountQL.check_ck_code = checkCkEditorView.state.doc.toString()
}

function destroyTemplateEditors() {
  syncTemplateEditors()
  for (const view of [parseInputEditorView, queryEditorView, afterRunEditorView, checkCkEditorView]) view?.destroy()
  parseInputEditorView = null
  queryEditorView = null
  afterRunEditorView = null
  checkCkEditorView = null
}

const saveCode = async () => {
  if (!editorView || !selectedPath.value || !canEdit.value) return
  saving.value = true
  try {
    const code = editorView.state.doc.toString()
    const payload = { path: selectedPath.value, code, expected_sha256: selectedFileSHA256.value }
    const result = await request.put(`/plugins/files/${pluginId.value}`, payload)
    selectedFileSHA256.value = String(result?.sha256 || selectedFileSHA256.value)
    if (hybridMode.value && result?.template_source) {
      const nextSource = normalizeHybridTemplateSource(result.template_source, pluginId.value)
      hybridSource.value = nextSource
      hybridSourceInitial.value = cloneTemplateValue(result.template_source)
      hybridSections.value = nextSource.sections.map(section => ({ ...section }))
      templateGeneratedFiles.value = nextSource.files
      hybridExternalDriftPaths.value = []
      templateSourceChanged.value = false
    } else {
      markGeneratedSourceChanged(selectedPath.value)
    }
    ElMessage.success('文件已保存并生效')
  } catch (error) {
    console.error('保存文件失败:', error)
    ElMessage.error('保存文件失败: ' + errorMessage(error))
  } finally {
    saving.value = false
  }
}

async function saveTemplate() {
  if (hybridMode.value) {
    await saveHybridTemplate()
    return
  }
  syncTemplateEditors()
  const validationIssue = validateTemplateForm()
  if (validationIssue) {
    activeTab.value = validationIssue.tab
    ElMessage.error(validationIssue.message)
    if (validationIssue.tab === 'code') await ensureTemplateEditors()
    return
  }
  let overwriteGeneratedFiles = Boolean(conversionMode.value)
  if (templateSourceChanged.value && !overwriteGeneratedFiles) {
    try {
      await ElMessageBox.confirm('检测到模板生成文件已在高级编辑中修改。继续保存会用分类表单重新生成并覆盖这些文件，是否继续？', '覆盖确认', { type: 'warning', confirmButtonText: '覆盖并保存' })
      overwriteGeneratedFiles = true
    } catch {
      return
    }
  }
  if (conversionMode.value) {
    try {
      await ElMessageBox.confirm('转换后插件入口文件和 plugin.json 将由模板生成并可能覆盖现有文件。确认已补齐所有代码并继续转换吗？', '确认转换', { type: 'warning', confirmButtonText: '确认转换' })
    } catch {
      return
    }
  }
  saving.value = true
  try {
    const payload = {
      template_source: buildTemplateSource(),
      overwrite_generated_files: overwriteGeneratedFiles
    }
    const result = conversionMode.value
      ? await convertPluginTemplateEditor(pluginId.value, payload)
      : await updatePluginTemplateEditor(pluginId.value, payload)
    applySavedTemplateResult(result)
    conversionMode.value = false
    requiresConversion.value = false
    templateEditable.value = true
    fileTree.value = []
    ElMessage.success('模板配置已保存并生效')
  } catch (error) {
    const response = error?.response?.data || {}
    const issue = firstValidationIssue(response)
    const issueMessage = issueText(issue?.message || issue)
    const issueTab = issue?.tab
      ? normalizeIssueTab(issue.tab)
      : inferIssueTab(issueMessage || response.msg || response.error || response.message, issue?.field || response.field)
    if (issue || response.msg || response.error || response.message || response.field) {
      activeTab.value = issueTab
      if (issueTab === 'code') await ensureTemplateEditors()
    }
    if (error?.response?.status === 409 || response.source_changed) {
      templateSourceChanged.value = true
      try {
        await ElMessageBox.confirm('保存时发现生成文件已变化。是否覆盖外部修改并重新保存？', '源文件已变化', { type: 'warning', confirmButtonText: '覆盖并保存' })
        const retryPayload = { template_source: buildTemplateSource(), overwrite_generated_files: true }
        const result = conversionMode.value
          ? await convertPluginTemplateEditor(pluginId.value, retryPayload)
          : await updatePluginTemplateEditor(pluginId.value, retryPayload)
        applySavedTemplateResult(result)
        conversionMode.value = false
        requiresConversion.value = false
        templateEditable.value = true
        fileTree.value = []
        ElMessage.success('模板配置已覆盖保存并生效')
      } catch (confirmError) {
        if (confirmError?.response) ElMessage.error('保存模板失败: ' + errorMessage(confirmError))
      }
      return
    }
    ElMessage.error('保存模板失败: ' + (issueMessage || errorMessage(error)))
  } finally {
    saving.value = false
  }
}

async function saveHybridTemplate() {
  syncHybridSectionEditors()
  const source = buildHybridTemplateSource(hybridSourceInitial.value, hybridPluginForm.value, hybridSections.value)
  const pluginDiffs = hybridPluginDiffs(hybridSourceInitial.value, source)
  const sectionDiffs = hybridSectionDiffs(hybridSourceInitial.value, source)
  const manifestFileDiffs = hybridFileDiffs(hybridSourceInitial.value, source)
  const externalDriftPaths = uniqueHybridPaths(hybridExternalDriftPaths.value)
  if (manifestFileDiffs.length) {
    ElMessage.error('检测到 hybrid files 清单发生变化，已阻止保存；请重新加载模板编辑模型')
    return
  }
  if (!pluginDiffs.length && !sectionDiffs.length) {
    if (externalDriftPaths.length) {
      ElMessage.warning(`未修改基础字段或 section；仅检测到后端报告的外部漂移：${externalDriftPaths.join('、')}。当前不会发送保存请求。`)
    } else {
      ElMessage.info('没有检测到基础字段或 section 变化')
    }
    return
  }
  const pluginText = pluginDiffs.length
    ? pluginDiffs.map(item => `- ${hybridPluginFieldLabels[item.key] || item.key}：${formatHybridDiffValue(item.initial)} → ${formatHybridDiffValue(item.current)}`).join('\n')
    : '- 无'
  const sectionText = sectionDiffs.length
    ? sectionDiffs.map(item => `- ${hybridCategoryLabel(item.category)} / ${item.label}${item.path ? `（写入 ${item.path}）` : ''}`).join('\n')
    : '- 无'
  const writtenFiles = uniqueHybridPaths(sectionDiffs.map(item => item.path))
  const writtenFileText = writtenFiles.length ? writtenFiles.map(path => `- ${path}`).join('\n') : '- 仅更新 plugin.json 中的基础字段与 template_source'
  const externalText = externalDriftPaths.length
    ? externalDriftPaths.map(path => `- ${path}`).join('\n')
    : '- 无'
  const message = [
    '本次修改的基础字段：',
    pluginText,
    '',
    '本次修改的 section：',
    sectionText,
    '',
    '本次保存将写入的文件：',
    writtenFileText,
    '',
    '后端报告的外部漂移文件（不是本次修改）：',
    externalText
  ].join('\n')
  try {
    await ElMessageBox.confirm(h('pre', { class: 'hybrid-change-preview' }, message), '确认保存分类编辑内容', { type: externalDriftPaths.length ? 'warning' : 'info', confirmButtonText: '保存变化', cancelButtonText: '取消' })
  } catch {
    return
  }
  saving.value = true
  try {
    const result = await updatePluginTemplateEditor(pluginId.value, { template_source: source })
    applySavedHybridResult(result)
    ElMessage.success('分类编辑内容已保存并生效')
  } catch (error) {
    const response = error?.response?.data || {}
    if (error?.response?.status === 409) {
      templateSourceChanged.value = true
      ElMessage.error('文件内容已被外部修改，请重新加载后再保存；分类编辑不会覆盖外部变化。')
      return
    }
    ElMessage.error('保存分类编辑内容失败：' + (response.msg || response.error || response.message || errorMessage(error)))
  } finally {
    saving.value = false
  }
}

function applySavedHybridResult(result) {
  const model = unwrapTemplateModel(result)
  const source = model?.template_source && typeof model.template_source === 'object' ? model.template_source : model
  if (!isHybridTemplateSource(source)) return
  initializeHybridModel(source, model, result)
  activeTab.value = 'base'
  fileTree.value = []
}

function buildTemplateSource() {
  const form = templateForm.value
  const accountQL = normalizeAccountQLConfig(form.account_ql, form.runtime)
  accountQL.routes = accountQL.routes.map((route, index) => ({
    command: String(route.command || '').trim(),
    function_name: String(route.function_name || defaultRouteFunctionName(index, form.runtime)).trim(),
    description: String(route.description || '').trim(),
    code: String(route.code || '').trim()
  })).filter(route => route.command || route.code)
  return {
    version: 1,
    template: form.runtime === 'python' ? 'python_account_ql' : 'nodejs_account_ql',
    plugin: {
      name: String(form.name || '').trim(),
      version: String(form.version || '1.0.0').trim(),
      runtime: form.runtime,
      runtime_profile: String(form.runtime_profile || '').trim(),
      priority: Number(form.priority || 0),
      platforms: Array.isArray(form.platforms) ? form.platforms : [],
      enabled: Boolean(form.enabled),
      script_env: normalizeScriptEnv(form.script_env)
    },
    account_ql: accountQL,
    files: []
  }
}

function validateTemplateForm() {
  const form = templateForm.value
  const accountQL = form.account_ql
  if (!String(form.name || '').trim()) return { tab: 'base', message: '插件名称不能为空' }
  if (!Array.isArray(form.platforms) || !form.platforms.length) return { tab: 'base', message: '请至少选择一个支持平台' }
  if (!String(accountQL.prefix || '').trim()) return { tab: 'account', message: '指令前缀不能为空' }
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(String(accountQL.table_name || '').trim())) return { tab: 'account', message: '账号表名格式无效' }
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(String(accountQL.env_name || '').trim())) return { tab: 'account', message: '青龙变量名格式无效' }
  if (!String(accountQL.task_script || '').trim()) return { tab: 'account', message: '青龙脚本路径不能为空' }
  if (!String(accountQL.parse_input_code || '').trim()) return { tab: 'code', message: '登录解析代码不能为空' }
  if (!String(accountQL.query_code || '').trim()) return { tab: 'code', message: '查询代码不能为空' }
  if (accountQL.enable_after_run && !String(accountQL.after_run_code || '').trim()) return { tab: 'code', message: '一键运行完成钩子不能为空' }
  if (accountQL.enable_ck_check && !String(accountQL.check_ck_code || '').trim()) return { tab: 'code', message: 'CK 检测代码不能为空' }
  const invalidRoute = (accountQL.routes || []).find(route => !String(route.command || '').trim() || !String(route.function_name || '').trim() || !String(route.code || '').trim())
  if (invalidRoute) return { tab: 'routes', message: '自定义指令的指令名、函数名和代码均不能为空' }
  return null
}

function applySavedTemplateResult(result) {
  const model = unwrapTemplateModel(result)
  destroyTemplateEditors()
  templateForm.value = normalizeTemplateForm(model)
  templateSourceChanged.value = Boolean(result?.source_changed || (Array.isArray(result?.modified_files) && result.modified_files.length))
  templateGeneratedFiles.value = Array.isArray(result?.files) ? result.files : (Array.isArray(model?.files) ? model.files : [])
  templateProbeMessage.value = String(result?.reason || '')
}

function firstValidationIssue(response = {}) {
  const issues = response.errors || response.validation_errors || response.issues
  if (Array.isArray(issues)) return issues[0] || null
  if (issues && typeof issues === 'object') return issues
  if (response.error && typeof response.error === 'object') return response.error
  return null
}

function issueText(value) {
  if (typeof value === 'string') return value
  if (value && typeof value === 'object') return value.message || value.error || value.detail || JSON.stringify(value)
  return String(value || '')
}

function inferIssueTab(message, field = '') {
  const value = `${field || ''} ${issueText(message)}`.toLowerCase()
  if (/route|custom.?command|function.?name|自定义指令|函数名|指令代码/.test(value)) return 'routes'
  if (/parse.?input|query.?code|after.?run|check.?ck|custom.?code|登录解析|查询代码|完成钩子|ck.?检测|代码/.test(value)) return 'code'
  if (/account.?ql|prefix|table.?name|env.?name|task.?script|cron|青龙|前缀|表名|变量名|脚本/.test(value)) return 'account'
  return 'base'
}

function normalizeIssueTab(tab) {
  const value = String(tab || '').toLowerCase()
  if (value === 'account_ql' || value === 'account' || value === 'ql') return 'account'
  if (value === 'code' || value === 'custom_code') return 'code'
  if (value === 'routes' || value === 'custom_routes') return 'routes'
  return 'base'
}

function addRoute() {
  const index = templateForm.value.account_ql.routes.length
  const functionName = defaultRouteFunctionName(index, templateForm.value.runtime)
  templateForm.value.account_ql.routes.push({ id: `${Date.now()}_${index}`, command: '', function_name: functionName, description: '', code: conversionMode.value ? '' : defaultRouteCode(functionName, templateForm.value.runtime) })
}

function removeRoute(index) {
  templateForm.value.account_ql.routes.splice(index, 1)
}

async function handleTabChange(tab) {
  if (tab === 'sections') await ensureHybridSectionEditors()
  else destroyHybridSectionEditors()
  if (tab === 'code') await ensureTemplateEditors()
  else destroyTemplateEditors()
  if (tab === 'files') {
    await nextTick()
    if (!fileTree.value.length) await loadFiles()
    else if (selectedPath.value && canEdit.value) await openFile(selectedPath.value)
  }
}

function markGeneratedSourceChanged(path) {
  if (!templateEditable.value) return
  const normalizedPath = String(path || '').replaceAll('\\', '/')
  if (templateGeneratedFiles.value.some(file => String(file?.path || '').replaceAll('\\', '/') === normalizedPath)) {
    templateSourceChanged.value = true
  }
}

function normalizeHybridPath(path) {
  return String(path || '').replaceAll('\\', '/').trim()
}

function uniqueHybridPaths(paths = []) {
  return [...new Set(paths.map(normalizeHybridPath).filter(Boolean))].sort()
}

function formatHybridDiffValue(value) {
  if (value === undefined) return '未设置'
  if (value === null) return 'null'
  if (typeof value === 'string') return value || '空字符串'
  if (typeof value === 'boolean') return value ? '是' : '否'
  return JSON.stringify(value)
}

function errorMessage(error) {
  return issueText(error?.response?.data?.msg) || issueText(error?.response?.data?.error) || issueText(error?.response?.data?.message) || error?.message || '未知错误'
}

const exportPluginDirectory = async () => {
  exporting.value = true
  try {
    const fileName = `${pluginId.value}.zip`
    const response = await fetch(`/api/plugins/export/${encodeURIComponent(pluginId.value)}`, {
      headers: { Authorization: `Bearer ${authStore.token}` }
    })
    if (!response.ok) {
      ElMessage.error('导出失败')
      return
    }
    const blob = await response.blob()
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  } catch (error) {
    console.error('导出失败:', error)
    ElMessage.error('导出失败: ' + (error.message || '未知错误'))
  } finally {
    exporting.value = false
  }
}

const deleteSelected = async () => {
  if (!selectedPath.value) return
  try {
    await ElMessageBox.confirm(`确定删除「${selectedPath.value}」吗？删除文件夹会同时删除其中所有内容。`, '删除确认', { type: 'warning' })
  } catch {
    return
  }
  deleting.value = true
  try {
    await request.delete(`/plugins/files/${pluginId.value}`, { params: { path: selectedPath.value } })
    ElMessage.success('已删除')
    selectedPath.value = ''
    selectedDirectory.value = ''
    canEdit.value = false
    destroyEditor()
    await loadFiles()
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败: ' + errorMessage(error))
  } finally {
    deleting.value = false
  }
}

const handleNodeClick = (node) => {
  selectedDirectory.value = node.type === 'directory' ? node.path : parentPath(node.path)
  if (node.type === 'directory') {
    selectedPath.value = node.path
    canEdit.value = false
    destroyEditor()
    return
  }
  if (!node.text) {
    selectedPath.value = node.path
    canEdit.value = false
    destroyEditor()
    return
  }
  openFile(node.path)
}

const defaultCreatePath = (type) => {
  const base = selectedDirectory.value ? `${selectedDirectory.value}/` : ''
  return base + (type === 'directory' ? 'new-folder' : 'new-file.js')
}

const parentPath = (path) => {
  const value = String(path || '')
  return value.includes('/') ? value.slice(0, value.lastIndexOf('/')) : ''
}

const normalizeCreatePath = (path) => {
  return String(path || '').replace(/\\/g, '/').replace(/^\/+/, '').trim()
}

const languageFor = (path) => {
  if (path.endsWith('.py')) return python()
  return javascript()
}

const collectDirectories = (nodes) => {
  const result = []
  for (const node of nodes) {
    if (node.type === 'directory') {
      result.push(node.path)
      result.push(...collectDirectories(node.children || []))
    }
  }
  return result
}

const findPath = (nodes, path) => {
  for (const node of nodes) {
    if (node.path === path && node.text) return node.path
    const child = findPath(node.children || [], path)
    if (child) return child
  }
  return ''
}

const findFirstTextFile = (nodes) => {
  for (const node of nodes) {
    if (node.type === 'file' && node.text) return node.path
    const child = findFirstTextFile(node.children || [])
    if (child) return child
  }
  return ''
}

const destroyEditor = () => {
  if (editorView) {
    editorView.destroy()
    editorView = null
  }
}

const goBack = () => {
  router.push('/plugins')
}

watch(() => templateForm.value.account_ql.enable_after_run, async () => {
  if (activeTab.value === 'code') await ensureTemplateEditors()
})
watch(() => templateForm.value.account_ql.enable_ck_check, async () => {
  if (activeTab.value === 'code') await ensureTemplateEditors()
})

onMounted(async () => {
  await probeTemplateEditor()
  if (!templateEditable.value) await loadFiles()
})
onBeforeUnmount(() => {
  destroyEditor()
  destroyTemplateEditors()
  destroyHybridSectionEditors(true)
})
</script>

<style scoped>
.plugin-editor {
  width: 100%;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: visible;
}

.editor-card {
  height: 100%;
  max-height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.editor-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.header-left,
.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title {
  font-size: 16px;
  font-weight: bold;
}

.current-file {
  color: #666;
  max-width: 420px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.subtitle {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
}

.editor-body {
  position: relative;
  flex: 1 1 0;
  height: 0;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.mode-alert,
.generated-alert {
  flex: none;
  margin: 12px 14px 0;
  width: auto;
}

.alert-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
}

.template-tabs {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 0 14px;
}

.template-tabs,
.template-tabs :deep(.el-tabs__content),
.template-tabs :deep(.el-tab-pane),
.template-tabs :deep(.el-tabs__header) {
  min-width: 0;
}

.template-tabs :deep(.el-tabs__content) {
  flex: 1;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.template-tabs :deep(.el-tab-pane) {
  flex: 1;
  height: 100%;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.form-scroll {
  flex: 1;
  min-height: 0;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 8px 14px 24px 0;
}

.template-form {
  max-width: 860px;
}

.code-grid {
  display: grid;
  gap: 18px;
}

.section-title {
  margin-bottom: 8px;
  color: #303133;
  font-weight: 600;
}

.template-code-editor {
  height: 260px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
}

.template-code-editor :deep(.cm-editor) {
  height: 100%;
  font-size: 13px;
}

.template-code-editor :deep(.cm-scroller) {
  overflow: auto;
}

.hybrid-sections {
  height: 100%;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.hybrid-category-nav {
  flex: none;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 0 10px;
  border-bottom: 1px solid #ebeef5;
}

.hybrid-category-scroll {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
}

.hybrid-category-scroll::-webkit-scrollbar {
  display: none;
}

.category-arrow,
.category-tab {
  flex: 0 0 auto;
  border: 1px solid #dcdfe6;
  background: #fff;
  color: #606266;
  cursor: pointer;
  transition: border-color 0.18s ease, color 0.18s ease, background-color 0.18s ease;
}

.category-arrow {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border-radius: 4px;
}

.category-tab {
  height: 28px;
  padding: 0 12px;
  border-radius: 14px;
  white-space: nowrap;
  font-size: 13px;
}

.category-arrow:hover:not(:disabled),
.category-tab:hover,
.category-tab.active {
  border-color: #409eff;
  color: #409eff;
  background: #ecf5ff;
}

.category-arrow:disabled {
  color: #c0c4cc;
  background: #f5f7fa;
  cursor: not-allowed;
}

.hybrid-section-scroll {
  flex: 1 1 auto;
  min-height: 0;
  min-width: 0;
  display: flex;
  padding-top: 12px;
}

.hybrid-section-group,
.hybrid-section-card {
  min-width: 0;
  display: grid;
  gap: 10px;
}

.hybrid-section-group {
  flex: 1;
  min-height: 0;
  grid-auto-rows: minmax(0, 1fr);
}

.hybrid-section-card {
  min-height: 0;
  grid-template-rows: auto auto minmax(0, 1fr);
  padding: 14px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  background: #fff;
}

.hybrid-section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.section-path {
  margin-left: 10px;
  color: #909399;
  font-size: 12px;
  font-weight: 400;
}

.hybrid-section-editor {
  height: 100%;
  min-height: 420px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
}

.hybrid-section-editor :deep(.cm-editor) {
  height: 100%;
  font-size: 13px;
}

.hybrid-section-editor :deep(.cm-scroller) {
  overflow: auto;
}

.hybrid-section-editor :deep(.cm-editor.cm-focused) {
  outline: 1px solid #409eff;
}

.route-panel {
  padding-top: 4px;
}

.route-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  color: #606266;
}

.route-item {
  padding: 14px;
  margin-bottom: 14px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
}

.route-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr auto;
  gap: 10px;
  margin-bottom: 10px;
}

.editor-layout {
  flex: 1 1 auto;
  height: 100%;
  min-height: 0;
  min-width: 0;
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
}

.editor-layout.inside-template-tabs {
  position: absolute;
  inset: 54px 14px 0;
  height: auto;
  border-top: 1px solid #ebeef5;
}

.file-panel {
  min-width: 0;
  overflow: auto;
  padding: 14px;
  border-right: 1px solid #ebeef5;
  background: #fafafa;
}

.file-panel-title {
  font-weight: 600;
}

.file-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 12px;
}

.mobile-collapse-button {
  display: none;
}

.file-panel-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.tree-node {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.tree-node.muted {
  color: #909399;
}

.field-tip {
  width: 100%;
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
  line-height: 1.5;
}

.editor-main {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  padding: 14px;
}

.code-editor-container {
  height: 100%;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
}

.code-editor-container :deep(.cm-editor) {
  height: 100%;
  font-size: 14px;
}

.code-editor-container :deep(.cm-scroller) {
  overflow: auto;
}

@media (max-width: 768px) {
  .plugin-editor {
    height: 100%;
  }

  .editor-card {
    height: 100%;
  }

  .card-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .header-left,
  .header-actions {
    width: 100%;
    flex-wrap: wrap;
  }

  .mobile-collapse-button {
    display: inline-flex;
  }

  .current-file {
    max-width: 100%;
  }

  .editor-layout {
    grid-template-columns: 1fr;
    grid-template-rows: auto minmax(0, 1fr);
  }

  .editor-layout.file-panel-collapsed {
    grid-template-rows: auto minmax(0, 1fr);
  }

  .file-panel {
    border-right: none;
    border-bottom: 1px solid #ebeef5;
    max-height: 42vh;
  }

  .editor-layout.file-panel-collapsed .file-panel {
    max-height: 52px;
    overflow: hidden;
  }

  .editor-layout.file-panel-collapsed .file-panel :deep(.el-tree),
  .editor-layout.file-panel-collapsed .file-panel .el-tree {
    display: none;
  }
}
</style>
