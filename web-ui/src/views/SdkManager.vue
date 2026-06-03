<template>
  <div class="sdk-manager-page">
    <el-card class="sdk-card">
      <template #header>
        <div class="card-header">
          <div>
            <div class="title-row">
              <span class="title">SDK 管理</span>
              <el-button class="mobile-info-button" type="primary" link aria-label="查看 SDK 管理说明" @click="showPageDescription">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </div>
            <div class="subtitle">{{ pageDescription }}</div>
          </div>
          <div class="header-actions">
            <span class="current-file">{{ activeMenu === 'editor' ? (selectedPath || '请选择 SDK 文件') : '参考示例和函数说明' }}</span>
            <el-button v-if="activeMenu === 'editor'" type="primary" :loading="saving" :disabled="!canEdit" @click="saveCode">保存</el-button>
          </div>
        </div>
      </template>

      <div class="sdk-layout">
        <aside class="sdk-sidebar">
          <el-menu class="sdk-menu" :default-active="activeMenu" @select="selectMenu">
            <el-menu-item index="editor">SDK编辑</el-menu-item>
            <el-menu-item index="reference">参考示例和函数说明</el-menu-item>
          </el-menu>
          <div v-if="activeMenu === 'editor'" class="file-tree">
            <el-tree
              :data="fileTree"
              node-key="path"
              :props="treeProps"
              :default-expanded-keys="expandedKeys"
              highlight-current
              @node-click="handleNodeClick"
            >
              <template #default="{ data }">
                <span class="tree-node" :class="{ muted: data.type === 'file' && !data.text }">
                  <el-icon v-if="data.type === 'directory'"><Folder /></el-icon>
                  <el-icon v-else><Document /></el-icon>
                  <span>{{ data.name }}</span>
                </span>
              </template>
            </el-tree>
          </div>
        </aside>

        <main v-loading="loading" class="sdk-main">
          <template v-if="activeMenu === 'editor'">
            <el-empty v-if="!selectedPath" description="请选择左侧 SDK 文件" />
            <el-result v-else-if="!canEdit" icon="warning" title="该文件不支持在线预览" :sub-title="selectedPath" />
            <div v-show="selectedPath && canEdit" ref="editorContainer" class="code-editor-container"></div>
          </template>
          <template v-else>
            <pre class="reference-content">{{ referenceContent }}</pre>
          </template>
        </main>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Document, Folder, InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '@/utils/request'
import { EditorView, basicSetup } from 'codemirror'
import { python } from '@codemirror/lang-python'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'

const activeMenu = ref('editor')
const fileTree = ref([])
const expandedKeys = ref([])
const selectedPath = ref('')
const canEdit = ref(false)
const loading = ref(false)
const saving = ref(false)
const editorContainer = ref(null)
const referenceContent = ref('')
const treeProps = { children: 'children', label: 'name' }
const pageDescription = '查看和编辑 Node.js / Python SDK，阅读账号青龙插件封装示例。'
let editorView = null

const showPageDescription = () => {
  ElMessageBox.alert(pageDescription, 'SDK 管理说明', {
    confirmButtonText: '知道了',
    type: 'info'
  })
}

const selectMenu = async (key) => {
  activeMenu.value = key
  if (key === 'editor') await loadFiles(selectedPath.value)
  else await loadReference()
}

const loadFiles = async (preferredPath = '') => {
  loading.value = true
  try {
    const data = await request.get('/sdk/files')
    fileTree.value = data.tree || []
    expandedKeys.value = collectDirectories(fileTree.value)
    const firstPath = findPath(fileTree.value, preferredPath) || findPath(fileTree.value, 'nodejs/account_ql_plugin.js') || findFirstTextFile(fileTree.value)
    if (firstPath) await openFile(firstPath)
  } finally {
    loading.value = false
  }
}

const loadReference = async () => {
  loading.value = true
  try {
    const data = await request.get('/sdk/reference')
    referenceContent.value = data.content || ''
    destroyEditor()
  } finally {
    loading.value = false
  }
}

const openFile = async (path) => {
  if (!path) return
  const data = await request.get('/sdk/files', { params: { path } })
  selectedPath.value = data.path || path
  canEdit.value = Boolean(data.editable)
  destroyEditor()
  if (canEdit.value) {
    await nextTick()
    createEditor(data.code || '', selectedPath.value)
  }
}

const handleNodeClick = (node) => {
  if (node.type === 'file' && node.text) openFile(node.path)
}

const createEditor = (code, path) => {
  if (!editorContainer.value) return
  editorView = new EditorView({
    doc: code,
    extensions: [basicSetup, languageFor(path), oneDark, EditorView.lineWrapping],
    parent: editorContainer.value
  })
}

const saveCode = async () => {
  if (!editorView || !selectedPath.value || !canEdit.value) return
  saving.value = true
  try {
    await request.put('/sdk/files', { path: selectedPath.value, code: editorView.state.doc.toString() })
    ElMessage.success('SDK 文件已保存')
  } finally {
    saving.value = false
  }
}

const languageFor = (path) => path.endsWith('.py') ? python() : javascript()

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

onMounted(loadFiles)
onBeforeUnmount(destroyEditor)
</script>

<style scoped>
.sdk-manager-page { width: 100%; height: 100%; padding: 0; }
.sdk-card { height: 100%; display: flex; flex-direction: column; }
.sdk-card :deep(.el-card__body) { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.card-header { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
.title-row { display: flex; align-items: center; gap: 6px; }
.title { font-size: 18px; font-weight: 600; }
.mobile-info-button { display: none; padding: 0; font-size: 16px; }
.subtitle { margin-top: 6px; color: #909399; font-size: 13px; line-height: 1.5; }
.header-actions { display: flex; align-items: center; gap: 12px; min-width: 0; }
.current-file { max-width: 360px; overflow: hidden; color: #606266; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.sdk-layout { height: 100%; min-height: 0; display: grid; grid-template-columns: 280px minmax(0, 1fr); gap: 14px; }
.sdk-sidebar { min-height: 0; border-right: 1px solid #ebeef5; padding-right: 12px; overflow: auto; }
.sdk-menu { border-right: none; }
.file-tree { margin-top: 12px; overflow-x: auto; }
.tree-node { max-width: 100%; display: inline-flex; align-items: center; gap: 6px; }
.tree-node span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tree-node.muted { color: #a8abb2; }
.sdk-main { min-width: 0; min-height: 0; overflow: hidden; }
.sdk-main :deep(.el-empty),
.sdk-main :deep(.el-result) { height: 100%; display: flex; flex-direction: column; justify-content: center; }
.code-editor-container { height: 100%; border: 1px solid #dcdfe6; border-radius: 4px; overflow: hidden; }
.code-editor-container :deep(.cm-editor) { height: 100%; font-size: 14px; }
.code-editor-container :deep(.cm-scroller) { overflow: auto; }
.reference-content { height: 100%; overflow: auto; padding: 16px; margin: 0; white-space: pre-wrap; word-break: break-word; border-radius: 4px; background: #1f2937; color: #f9fafb; font-family: Consolas, Monaco, monospace; }
@media (max-width: 768px) {
  .sdk-manager-page {
    height: calc(100dvh - 52px - 76px - 24px);
    min-height: 0;
    overflow: hidden;
  }

  .sdk-card {
    height: 100%;
    min-height: 100%;
  }

  .sdk-card :deep(.el-card__header) {
    padding: 14px;
  }

  .sdk-card :deep(.el-card__body) {
    padding: 12px;
    overflow: hidden;
  }

  .card-header {
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
  }

  .title {
    font-size: 16px;
  }

  .mobile-info-button {
    display: inline-flex;
  }

  .subtitle {
    display: none;
    font-size: 12px;
  }

  .header-actions {
    width: 100%;
    align-items: stretch;
    flex-direction: column;
    gap: 8px;
  }

  .current-file {
    max-width: 100%;
    white-space: normal;
    word-break: break-all;
  }

  .header-actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .sdk-layout {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr);
    gap: 12px;
    overflow: hidden;
  }

  .sdk-sidebar {
    max-height: 236px;
    border-right: none;
    border-bottom: 1px solid #ebeef5;
    padding-right: 0;
    padding-bottom: 10px;
    overflow-y: auto;
    overflow-x: hidden;
  }

  .sdk-menu :deep(.el-menu-item) {
    height: auto;
    min-height: 40px;
    padding: 0 12px !important;
    line-height: 1.4;
    white-space: normal;
  }

  .file-tree {
    margin-top: 10px;
    overflow-x: auto;
  }

  .file-tree :deep(.el-tree-node__content) {
    min-width: max-content;
  }

  .sdk-main {
    min-height: 0;
    overflow: hidden;
  }

  .code-editor-container {
    border-radius: 8px;
  }

  .code-editor-container :deep(.cm-editor) {
    font-size: 13px;
  }

  .code-editor-container :deep(.cm-gutters) {
    min-width: 34px;
  }

  .reference-content {
    padding: 12px;
    font-size: 12px;
  }

  .sdk-sidebar::-webkit-scrollbar,
  .file-tree::-webkit-scrollbar,
  .reference-content::-webkit-scrollbar {
    display: none;
  }
}
</style>
