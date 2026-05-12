<template>
  <div class="plugin-editor">
    <el-card>
      <template #header>
        <div class="card-header">
          <div>
            <el-button @click="goBack" size="small">
              <el-icon><ArrowLeft /></el-icon>
              返回
            </el-button>
            <span style="margin-left: 15px; font-size: 16px; font-weight: bold">
              编辑插件代码: {{ pluginName }}
            </span>
          </div>
          <div>
            <span style="color: #666; margin-right: 15px">{{ filename }}</span>
            <el-button type="primary" @click="saveCode" :loading="saving">
              保存
            </el-button>
          </div>
        </div>
      </template>

      <div v-loading="loading" style="min-height: 400px">
        <div ref="editorContainer" class="code-editor-container"></div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import request from '@/utils/request'
import { EditorView, basicSetup } from 'codemirror'
import { python } from '@codemirror/lang-python'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'

const route = useRoute()
const router = useRouter()

const pluginId = ref(route.params.id)
const pluginName = ref('')
const filename = ref('')
const loading = ref(false)
const saving = ref(false)
const editorContainer = ref(null)
let editorView = null

const loadCode = async () => {
  loading.value = true
  try {
    const data = await request.get(`/plugins/code/${pluginId.value}`)
    filename.value = data.filename
    pluginName.value = data.plugin_name || pluginId.value

    // 根据文件扩展名选择语言
    const isPython = filename.value.endsWith('.py')
    const language = isPython ? python() : javascript()

    // 创建编辑器
    if (editorContainer.value) {
      editorView = new EditorView({
        doc: data.code,
        extensions: [
          basicSetup,
          language,
          oneDark,
          EditorView.lineWrapping
        ],
        parent: editorContainer.value
      })
    }
  } catch (error) {
    console.error('加载代码失败:', error)
    ElMessage.error('加载代码失败: ' + (error.response?.data?.error || error.message))
  } finally {
    loading.value = false
  }
}

const saveCode = async () => {
  if (!editorView) return

  saving.value = true
  try {
    const code = editorView.state.doc.toString()
    await request.put(`/plugins/code/${pluginId.value}`, { code })

    ElMessage.success('代码已保存并生效')
  } catch (error) {
    console.error('保存代码失败:', error)
    ElMessage.error('保存代码失败: ' + (error.response?.data?.error || error.message))
  } finally {
    saving.value = false
  }
}

const goBack = () => {
  router.push('/plugins')
}

onMounted(() => {
  loadCode()
})

onBeforeUnmount(() => {
  if (editorView) {
    editorView.destroy()
  }
})
</script>

<style scoped>
.plugin-editor {
  width: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.code-editor-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
}

.code-editor-container :deep(.cm-editor) {
  height: 600px;
  font-size: 14px;
}

.code-editor-container :deep(.cm-scroller) {
  overflow: auto;
}
</style>
