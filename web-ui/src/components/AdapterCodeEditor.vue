<template>
  <el-dialog v-model="visible" :title="title" width="min(1100px, 94vw)" top="5vh" :before-close="beforeClose">
    <div v-loading="loading" class="editor-layout" :class="{ 'file-panel-collapsed': collapsed }">
      <aside class="file-panel">
        <div class="file-panel-header">
          <span class="file-panel-title">适配器目录</span>
          <el-button size="small" text class="mobile-collapse-button" @click="collapsed = !collapsed">
            <el-icon><component :is="collapsed ? ArrowDown : ArrowUp" /></el-icon>{{ collapsed ? '展开' : '收起' }}
          </el-button>
          <el-button size="small" text :disabled="saving || loading" @click="refreshFiles">刷新</el-button>
        </div>
        <el-tree ref="treeRef" :data="tree" node-key="path" :props="treeProps" :default-expanded-keys="expandedKeys" :current-node-key="selectedPath" highlight-current @node-click="selectNode">
          <template #default="{ data }">
            <span class="tree-node"><el-icon><Folder v-if="data.type === 'directory'" /><Document v-else /></el-icon><span>{{ data.name }}</span></span>
          </template>
        </el-tree>
      </aside>
      <main class="editor-main">
        <div class="code-panel-header"><span>{{ selectedPath || '请选择文件' }}{{ dirty ? ' *' : '' }}</span></div>
        <el-empty v-if="!selectedPath" description="请选择左侧文件" />
        <el-result v-else-if="!editable" icon="info" title="该文件无法在线编辑" sub-title="仅支持 2 MiB 以内的 UTF-8 文本文件" />
        <div v-show="selectedPath && editable" ref="editorContainer" class="code-editor-container"></div>
      </main>
    </div>
    <template #footer>
      <el-button :disabled="saving" @click="close">关闭</el-button>
      <el-button type="primary" :loading="saving" :disabled="loading || !editable || !selectedPath || !dirty" @click="save">保存并重载</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, ArrowUp, Document, Folder } from '@element-plus/icons-vue'
import { EditorView, basicSetup } from 'codemirror'
import { javascript } from '@codemirror/lang-javascript'
import { python } from '@codemirror/lang-python'
import { oneDark } from '@codemirror/theme-one-dark'
import { getAdapterCode, saveAdapterCode } from '@/api'

const props = defineProps({ platform: { type: String, required: true }, title: { type: String, default: '适配器文件' } })
const visible = defineModel({ type: Boolean })
const emit = defineEmits(['saved'])
const loading = ref(false)
const saving = ref(false)
const tree = ref([])
const treeRef = ref(null)
const treeProps = { children: 'children', label: 'name' }
const expandedKeys = ref([])
const selectedPath = ref('')
const editable = ref(false)
const originalCode = ref('')
const code = ref('')
const sha256 = ref('')
const dirty = computed(() => editable.value && code.value !== originalCode.value)
const collapsed = ref(false)
const editorContainer = ref(null)
let editor = null
let disposed = false

function destroyEditor() { editor?.destroy(); editor = null }

async function canDiscard() {
  if (!dirty.value) return true
  try {
    await ElMessageBox.confirm('当前文件有未保存的修改，确定放弃吗？', '未保存的修改', { type: 'warning', confirmButtonText: '放弃修改', cancelButtonText: '继续编辑' })
    return true
  } catch { return false }
}

async function beforeClose(done) {
  if (!saving.value && await canDiscard()) done()
}

async function close() {
  if (await canDiscard()) visible.value = false
}

function directories(nodes) {
  return nodes.flatMap(node => node.type === 'directory' ? [node.path, ...directories(node.children || [])] : [])
}

function firstFile(nodes) {
  for (const node of nodes) {
    if (node.type === 'file') return node.path
    const child = firstFile(node.children || [])
    if (child) return child
  }
  return ''
}

function containsPath(nodes, path) {
  return nodes.some(node => node.type === 'file' && node.path === path || containsPath(node.children || [], path))
}

async function loadFile(path) {
  const result = await getAdapterCode(props.platform, { path })
  if (disposed) return
  destroyEditor()
  selectedPath.value = result.filename || path
  editable.value = Boolean(result.editable)
  originalCode.value = result.code || ''
  code.value = originalCode.value
  sha256.value = result.sha256 || ''
  treeRef.value?.setCurrentKey(selectedPath.value)
  if (!editable.value) return
  await nextTick()
  if (disposed || !editorContainer.value) return
  const extensions = [basicSetup, oneDark, EditorView.lineWrapping, EditorView.updateListener.of(update => {
    if (update.docChanged) code.value = update.state.doc.toString()
  })]
  if (path.endsWith('.py')) extensions.push(python())
  else if (/\.(?:js|mjs|cjs|jsx|ts|tsx)$/i.test(path)) extensions.push(javascript({ typescript: /\.tsx?$/i.test(path), jsx: /\.[jt]sx$/i.test(path) }))
  editor = new EditorView({ doc: code.value, extensions, parent: editorContainer.value })
}

async function loadFiles(preferred = '') {
  loading.value = true
  try {
    const result = await getAdapterCode(props.platform, { tree: '1' })
    if (disposed) return
    tree.value = result.tree || []
    expandedKeys.value = directories(tree.value)
    const path = containsPath(tree.value, preferred) ? preferred : containsPath(tree.value, result.entry) ? result.entry : firstFile(tree.value)
    if (path) await loadFile(path)
    else { destroyEditor(); selectedPath.value = ''; editable.value = false }
  } catch (error) { ElMessage.error(errorMessage(error)) }
  finally { loading.value = false }
}

async function refreshFiles() {
  if (await canDiscard()) await loadFiles(selectedPath.value)
}

async function selectNode(node) {
  if (node.type !== 'file' || node.path === selectedPath.value) return
  if (loading.value || saving.value || !await canDiscard()) {
    treeRef.value?.setCurrentKey(selectedPath.value)
    return
  }
  loading.value = true
  try { await loadFile(node.path) }
  catch (error) { treeRef.value?.setCurrentKey(selectedPath.value); ElMessage.error(errorMessage(error)) }
  finally { loading.value = false }
}

async function save() {
  if (!editable.value || !selectedPath.value || saving.value) return
  saving.value = true
  try {
    await saveAdapterCode(props.platform, code.value, selectedPath.value, sha256.value)
    originalCode.value = code.value
    ElMessage.success('文件已保存并重载')
    emit('saved')
    await loadFiles(selectedPath.value)
  } catch (error) { ElMessage.error(errorMessage(error)) }
  finally { saving.value = false }
}

function errorMessage(error) {
  return error?.response?.data?.msg || error?.response?.data?.error || error?.message || '适配器文件操作失败'
}

onMounted(() => loadFiles())
onBeforeUnmount(() => { disposed = true; destroyEditor() })
</script>

<style scoped>
.editor-layout { height: 65vh; min-height: 0; display: grid; grid-template-columns: 260px minmax(0, 1fr); }
.file-panel { min-width: 0; overflow: auto; padding: 14px; border-right: 1px solid #ebeef5; background: #fafafa; }
.file-panel-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 12px; }
.file-panel-title { font-weight: 600; }
.mobile-collapse-button { display: none; }
.tree-node { display: inline-flex; align-items: center; gap: 6px; }
.editor-main { min-width: 0; min-height: 0; overflow: hidden; padding: 14px; display: flex; flex-direction: column; gap: 8px; }
.code-panel-header { color: #606266; font-size: 13px; overflow-wrap: anywhere; }
.code-editor-container { flex: 1; min-height: 0; border: 1px solid #dcdfe6; border-radius: 4px; overflow: hidden; }
.code-editor-container :deep(.cm-editor) { height: 100%; font-size: 14px; }
.code-editor-container :deep(.cm-scroller) { overflow: auto; }
@media (max-width: 768px) {
  .editor-layout { grid-template-columns: minmax(0, 1fr); grid-template-rows: minmax(100px, 30%) minmax(0, 1fr); }
  .editor-layout.file-panel-collapsed { grid-template-rows: 48px minmax(0, 1fr); }
  .file-panel { border-right: 0; border-bottom: 1px solid #ebeef5; padding: 10px; }
  .mobile-collapse-button { display: inline-flex; }
  .file-panel-collapsed .file-panel :deep(.el-tree) { display: none; }
  .editor-main { padding: 10px 0; }
}
</style>
