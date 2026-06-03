<template>
  <div class="plugin-editor">
    <el-card class="editor-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <el-button @click="goBack" size="small">
              <el-icon><ArrowLeft /></el-icon>
              返回
            </el-button>
            <span class="title">编辑插件文件: {{ pluginName }}</span>
          </div>
          <div class="header-actions">
            <span class="current-file">{{ selectedPath || '请选择文件' }}</span>
            <el-button type="danger" plain @click="deleteSelected" :loading="deleting" :disabled="!selectedPath">
              删除
            </el-button>
            <el-button type="primary" @click="saveCode" :loading="saving" :disabled="!canEdit">
              保存文件
            </el-button>
          </div>
        </div>
      </template>

      <div v-loading="loading" class="editor-layout">
        <aside class="file-panel">
          <div class="file-panel-header">
            <div class="file-panel-title">插件目录</div>
            <el-dropdown trigger="click" @command="openCreateDialog">
              <el-button size="small" type="primary" plain>新建</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="file">新建文件</el-dropdown-item>
                  <el-dropdown-item command="directory">新建文件夹</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
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
        </aside>

        <main class="editor-main">
          <el-empty v-if="!selectedPath" description="请选择左侧文件" />
          <el-result v-else-if="!canEdit" icon="warning" title="该文件不支持在线预览" :sub-title="selectedPath" />
          <div v-show="selectedPath && canEdit" ref="editorContainer" class="code-editor-container"></div>
        </main>
      </div>
    </el-card>

    <el-dialog v-model="createDialogVisible" :title="createForm.type === 'directory' ? '新建文件夹' : '新建文件'" width="480px">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="类型">
          <el-radio-group v-model="createForm.type">
            <el-radio-button label="file">文件</el-radio-button>
            <el-radio-button label="directory">文件夹</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="路径">
          <el-input v-model="createForm.path" :placeholder="createForm.type === 'directory' ? '例如：lib/utils' : '例如：lib/helper.js'" />
          <div class="field-tip">相对当前插件目录，支持输入多级目录。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="createEntry">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Document, Folder } from '@element-plus/icons-vue'
import request from '@/utils/request'
import { EditorView, basicSetup } from 'codemirror'
import { python } from '@codemirror/lang-python'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'

const route = useRoute()
const router = useRouter()

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
const canEdit = ref(false)
const editorContainer = ref(null)
const createDialogVisible = ref(false)
const createForm = ref({ type: 'file', path: '' })
const treeProps = { children: 'children', label: 'name' }
let editorView = null

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
    ElMessage.error('加载插件目录失败: ' + (error.response?.data?.error || error.message))
  } finally {
    loading.value = false
  }
}

const openCreateDialog = (type) => {
  createForm.value = { type, path: defaultCreatePath(type) }
  createDialogVisible.value = true
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
    ElMessage.error('创建失败: ' + (error.response?.data?.error || error.message))
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
    canEdit.value = Boolean(data.editable)
    destroyEditor()
    if (canEdit.value) {
      await nextTick()
      createEditor(data.code || '', selectedPath.value)
    }
  } catch (error) {
    console.error('加载文件失败:', error)
    ElMessage.error('加载文件失败: ' + (error.response?.data?.error || error.message))
  } finally {
    loading.value = false
  }
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
    const code = editorView.state.doc.toString()
    await request.put(`/plugins/files/${pluginId.value}`, { path: selectedPath.value, code })
    ElMessage.success('文件已保存并生效')
  } catch (error) {
    console.error('保存文件失败:', error)
    ElMessage.error('保存文件失败: ' + (error.response?.data?.error || error.message))
  } finally {
    saving.value = false
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
    ElMessage.error('删除失败: ' + (error.response?.data?.error || error.message))
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

onMounted(loadFiles)
onBeforeUnmount(destroyEditor)
</script>

<style scoped>
.plugin-editor {
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

  .current-file {
    max-width: 100%;
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
