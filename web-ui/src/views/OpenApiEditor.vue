<template>
  <div class="open-api-editor-page">
    <el-card class="editor-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <el-button size="small" @click="goBack">
              <el-icon><ArrowLeft /></el-icon>
              返回
            </el-button>
            <div>
              <span class="title">接口文件: {{ form.name || apiId }}</span>
              <div class="subtitle">编辑开放接口的单文件脚本，保存后通过 /api/open 路径调用。</div>
            </div>
          </div>
          <div class="header-actions">
            <span class="current-file">{{ selectedFile }}</span>
            <el-button :loading="loading" @click="loadDetail">刷新</el-button>
            <el-button type="primary" :loading="saving" @click="saveCode">保存文件</el-button>
          </div>
        </div>
      </template>

      <div v-loading="loading" class="editor-layout">
        <aside class="file-panel">
          <div class="file-panel-header">
            <div class="file-panel-title">接口文件</div>
            <el-tag size="small" type="info">{{ runtimeLabel }}</el-tag>
          </div>
          <el-tree
            :data="fileTree"
            node-key="path"
            :props="treeProps"
            :default-expanded-keys="['root']"
            highlight-current
            :current-node-key="selectedFile"
            @node-click="handleNodeClick"
          >
            <template #default="{ data }">
              <span class="tree-node" :class="{ muted: data.type === 'directory' }">
                <el-icon v-if="data.type === 'directory'"><Folder /></el-icon>
                <el-icon v-else><Document /></el-icon>
                <span>{{ data.name }}</span>
              </span>
            </template>
          </el-tree>
          <el-alert class="file-tip" type="info" :closable="false" show-icon>
            <template #title>文件名由接口路径和运行语言生成，例如 a.js 或 a.py。</template>
          </el-alert>
        </aside>

        <main class="editor-main">
          <section class="code-panel">
            <div class="code-panel-header">
              <span>代码编辑器</span>
              <el-tag size="small" effect="plain">{{ selectedFile }}</el-tag>
            </div>
            <div ref="editorContainer" class="code-editor-container"></div>
          </section>
        </main>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Document, Folder } from '@element-plus/icons-vue'
import { getOpenApi, getOpenApiCode, updateOpenApiCode } from '@/api'
import { EditorView, basicSetup } from 'codemirror'
import { python } from '@codemirror/lang-python'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'

const route = useRoute()
const router = useRouter()
const apiId = computed(() => String(route.params.id || ''))
const loading = ref(false)
const saving = ref(false)
const editorContainer = ref(null)
const form = ref(defaultForm())
const treeProps = { children: 'children', label: 'name' }
let editorView = null

const normalizedPath = computed(() => normalizeOpenPath(form.value.path || apiId.value))
const selectedFile = computed(() => codeFileName(normalizedPath.value || apiId.value, form.value.runtime))
const runtimeLabel = computed(() => normalizeRuntime(form.value.runtime) === 'python' ? 'Python' : 'Node.js')
const fileTree = computed(() => [
  {
    name: '开放接口',
    path: 'root',
    type: 'directory',
    children: [
      {
        name: selectedFile.value,
        path: selectedFile.value,
        type: 'file',
        text: true
      }
    ]
  }
])

async function loadDetail() {
  if (apiId.value === 'new') {
    ElMessage.warning('请先在列表页新建接口')
    router.replace('/open-apis')
    return
  }

  loading.value = true
  try {
    const detail = await getOpenApi(apiId.value)
    form.value = normalizeDetail(unwrapDetail(detail))
    try {
      const codeResult = await getOpenApiCode(apiId.value)
      await resetEditor(resolveCode(codeResult, form.value.runtime))
    } catch (error) {
      ElMessage.error('加载接口代码失败，已使用默认模板: ' + errorMessage(error))
      await resetEditor(defaultCode(form.value.runtime))
    }
  } catch (error) {
    ElMessage.error('加载接口配置失败: ' + errorMessage(error))
  } finally {
    loading.value = false
  }
}

function unwrapDetail(detail = {}) {
  return detail.item || detail.api || detail.open_api || detail.openApi || detail
}

function normalizeDetail(detail = {}) {
  return {
    id: detail.id || apiId.value,
    name: detail.name || detail.title || apiId.value,
    path: normalizeOpenPath(detail.path || detail.url_path || detail.urlPath || apiId.value),
    method: String(detail.method || 'POST').toUpperCase(),
    enabled: Boolean(detail.enabled),
    runtime: normalizeRuntime(detail.runtime),
    entry: detail.entry || '',
    description: detail.description || ''
  }
}

function resolveCode(result, runtime) {
  if (typeof result === 'string') return result || defaultCode(runtime)
  return result?.code || result?.content || defaultCode(runtime)
}

function defaultForm() {
  return {
    id: '',
    name: '',
    path: '',
    method: 'POST',
    enabled: true,
    runtime: 'nodejs',
    entry: '',
    description: ''
  }
}

function defaultCode(runtime) {
  if (normalizeRuntime(runtime) === 'python') {
    return `async def action(ctx, req, res):\n    res.json({"ok": True, "message": "hello from open api"})\n`
  }
  return `module.exports.action = async function action(ctx, req, res) {\n  res.json({ ok: true, message: 'hello from open api' })\n}\n`
}

async function resetEditor(code) {
  destroyEditor()
  await nextTick()
  createEditor(code || '')
}

function createEditor(code) {
  if (!editorContainer.value) return
  editorView = new EditorView({
    doc: code,
    extensions: [basicSetup, languageFor(selectedFile.value), oneDark, EditorView.lineWrapping],
    parent: editorContainer.value
  })
}

async function saveCode() {
  if (!apiId.value || apiId.value === 'new') {
    ElMessage.warning('接口 ID 无效，无法保存文件')
    return
  }

  saving.value = true
  try {
    await updateOpenApiCode(apiId.value, {
      code: getEditorCode(),
      runtime: form.value.runtime,
      file: selectedFile.value,
      entry: selectedFile.value
    })
    form.value.entry = selectedFile.value
    ElMessage.success('接口文件已保存')
  } catch (error) {
    ElMessage.error('保存接口文件失败: ' + errorMessage(error))
  } finally {
    saving.value = false
  }
}

function getEditorCode() {
  return editorView ? editorView.state.doc.toString() : ''
}

function errorMessage(error) {
  return error?.response?.data?.error || error?.message || '未知错误'
}

function normalizeOpenPath(path) {
  return String(path || '').replace(/\\/g, '/').replace(/^\/api\/open\//, '').replace(/^\/+|\/+$/g, '').trim()
}

function normalizeRuntime(runtime) {
  return runtime === 'python' ? 'python' : 'nodejs'
}

function codeFileName(path, runtime) {
  const normalized = normalizeOpenPath(path) || 'handler'
  return `${normalized}.${normalizeRuntime(runtime) === 'python' ? 'py' : 'js'}`
}

function languageFor(path) {
  return path.endsWith('.py') ? python() : javascript()
}

function handleNodeClick(node) {
  if (node.type === 'file') return
}

function goBack() {
  router.push('/open-apis')
}

function destroyEditor() {
  if (editorView) {
    editorView.destroy()
    editorView = null
  }
}

onMounted(loadDetail)
onBeforeUnmount(destroyEditor)
</script>

<style scoped>
.open-api-editor-page {
  width: 100%;
  height: 100%;
}

.editor-card {
  height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.editor-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  padding: 0;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
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
  font-weight: 600;
}

.subtitle {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
}

.current-file {
  color: #606266;
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-family: "JetBrains Mono", "Cascadia Code", monospace;
}

.editor-layout {
  height: 100%;
  min-height: 0;
  display: grid;
  grid-template-columns: 280px 1fr;
}

.file-panel {
  min-width: 0;
  overflow: auto;
  padding: 14px;
  border-right: 1px solid #ebeef5;
  background: #fafafa;
}

.file-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 12px;
}

.file-panel-title {
  font-weight: 600;
}

.tree-node {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.tree-node.muted {
  color: #909399;
}

.file-tip {
  margin-top: 14px;
}

.editor-main {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  padding: 14px;
  display: grid;
}

.code-panel {
  min-height: 0;
  display: grid;
  grid-template-rows: auto 1fr;
  gap: 8px;
}

.code-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #606266;
  font-size: 13px;
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
  .editor-card {
    height: calc(100dvh - 140px);
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

  .editor-layout {
    grid-template-columns: 1fr;
    grid-template-rows: 220px 1fr;
  }

  .file-panel {
    border-right: none;
    border-bottom: 1px solid #ebeef5;
  }
}
</style>
