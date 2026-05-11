<template>
  <div class="adapters">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>平台适配器</span>
          <el-button type="primary" size="small" @click="showAddDialog">
            <el-icon><Plus /></el-icon>
            添加平台
          </el-button>
        </div>
      </template>

      <el-table :data="adapters" v-loading="loading" style="width: 100%">
        <el-table-column label="平台" width="120">
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 8px">
              <el-icon v-if="row.platform === 'qq'" color="#12B7F5"><ChatDotRound /></el-icon>
              <el-icon v-else-if="row.platform === 'telegram'" color="#0088cc"><ChatDotSquare /></el-icon>
              <el-icon v-else-if="row.platform === 'wechat'" color="#07C160"><ChatLineRound /></el-icon>
              <span>{{ getPlatformName(row.platform) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="配置信息" min-width="300">
          <template #default="{ row }">
            <div class="config-info">
              {{ getConfigText(row) }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="启用状态" width="100">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              @change="handleToggleEnabled(row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="运行状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.running ? 'success' : 'danger'" size="small">
              {{ row.running ? '运行中' : '已停止' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="handleEdit(row)">
              编辑
            </el-button>
            <el-button type="danger" size="small" @click="handleDelete(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && adapters.length === 0" description="暂无平台配置，点击"添加平台"开始配置" />
    </el-card>

    <!-- 添加/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="500px"
      @close="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item label="平台" prop="platform">
          <el-select
            v-model="form.platform"
            placeholder="请选择平台"
            :disabled="isEdit"
            @change="handlePlatformChange"
            style="width: 100%"
          >
            <el-option label="QQ" value="qq" />
            <el-option label="Telegram" value="telegram" />
            <el-option label="微信" value="wechat" />
          </el-select>
        </el-form-item>

        <el-form-item label="状态" prop="enabled">
          <el-switch v-model="form.enabled" />
          <span style="margin-left: 10px; color: #999">
            {{ form.enabled ? '启用' : '禁用' }}
          </span>
        </el-form-item>

        <!-- QQ 配置 -->
        <template v-if="form.platform === 'qq'">
          <el-form-item label="API 地址" prop="config.api_url">
            <el-input
              v-model="form.config.api_url"
              placeholder="http://localhost:5700"
            />
          </el-form-item>
          <el-form-item label="监听地址" prop="config.listen_addr">
            <el-input
              v-model="form.config.listen_addr"
              placeholder=":8080"
            />
          </el-form-item>
        </template>

        <!-- Telegram 配置 -->
        <template v-if="form.platform === 'telegram'">
          <el-form-item label="Bot Token" prop="config.bot_token">
            <el-input
              v-model="form.config.bot_token"
              type="textarea"
              :rows="3"
              placeholder="123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
            />
          </el-form-item>
        </template>

        <!-- 微信配置 -->
        <template v-if="form.platform === 'wechat'">
          <el-form-item label="App ID" prop="config.app_id">
            <el-input
              v-model="form.config.app_id"
              placeholder="wx..."
            />
          </el-form-item>
          <el-form-item label="App Secret" prop="config.app_secret">
            <el-input
              v-model="form.config.app_secret"
              type="password"
              placeholder="..."
            />
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus,
  ChatDotRound,
  ChatDotSquare,
  ChatLineRound
} from '@element-plus/icons-vue'
import { getAdapters, saveAdapter, deleteAdapter } from '@/api'

const loading = ref(false)
const adapters = ref([])
const dialogVisible = ref(false)
const dialogTitle = ref('添加平台')
const isEdit = ref(false)
const saving = ref(false)
const formRef = ref(null)

const form = reactive({
  platform: 'qq',
  enabled: false,
  config: {
    api_url: 'http://localhost:5700',
    listen_addr: ':8080',
    bot_token: '',
    app_id: '',
    app_secret: ''
  }
})

const rules = {
  platform: [
    { required: true, message: '请选择平台', trigger: 'change' }
  ]
}

const loadAdapters = async () => {
  loading.value = true
  try {
    adapters.value = await getAdapters()
  } catch (error) {
    console.error('加载适配器失败:', error)
  } finally {
    loading.value = false
  }
}

const getPlatformName = (platform) => {
  const names = {
    'qq': 'QQ',
    'wechat': '微信',
    'telegram': 'Telegram'
  }
  return names[platform] || platform
}

const getConfigText = (row) => {
  try {
    const config = typeof row.config === 'string' ? JSON.parse(row.config) : row.config

    if (row.platform === 'qq') {
      return `API: ${config.api_url} | 监听: ${config.listen_addr}`
    } else if (row.platform === 'telegram') {
      return `Token: ${config.bot_token?.substring(0, 20)}...`
    } else if (row.platform === 'wechat') {
      return `AppID: ${config.app_id}`
    }
  } catch (e) {
    return '配置解析失败'
  }
}

const showAddDialog = () => {
  isEdit.value = false
  dialogTitle.value = '添加平台'
  resetForm()
  dialogVisible.value = true
}

const handleEdit = (row) => {
  isEdit.value = true
  dialogTitle.value = '编辑平台'

  form.platform = row.platform
  form.enabled = row.enabled

  try {
    const config = typeof row.config === 'string' ? JSON.parse(row.config) : row.config
    form.config = { ...form.config, ...config }
  } catch (e) {
    ElMessage.error('配置解析失败')
  }

  dialogVisible.value = true
}

const handlePlatformChange = () => {
  // 切换平台时重置配置
  form.config = {
    api_url: 'http://localhost:5700',
    listen_addr: ':8080',
    bot_token: '',
    app_id: '',
    app_secret: ''
  }
}

const handleSave = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    saving.value = true

    try {
      let config = {}

      if (form.platform === 'qq') {
        config = {
          api_url: form.config.api_url,
          listen_addr: form.config.listen_addr
        }
      } else if (form.platform === 'telegram') {
        config = {
          bot_token: form.config.bot_token
        }
      } else if (form.platform === 'wechat') {
        config = {
          app_id: form.config.app_id,
          app_secret: form.config.app_secret
        }
      }

      await saveAdapter({
        platform: form.platform,
        enabled: form.enabled,
        config
      })

      ElMessage.success('配置已保存并生效！')
      dialogVisible.value = false
      await loadAdapters()
    } catch (error) {
      console.error('保存失败:', error)
    } finally {
      saving.value = false
    }
  })
}

const handleToggleEnabled = async (row) => {
  try {
    const config = typeof row.config === 'string' ? JSON.parse(row.config) : row.config

    await saveAdapter({
      platform: row.platform,
      enabled: row.enabled,
      config
    })

    ElMessage.success(row.enabled ? '已启用' : '已禁用')
    await loadAdapters()
  } catch (error) {
    console.error('切换状态失败:', error)
    row.enabled = !row.enabled
  }
}

const handleDelete = async (row) => {
  await ElMessageBox.confirm(
    `确定要删除 ${getPlatformName(row.platform)} 的配置吗？`,
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )

  try {
    await deleteAdapter(row.platform)
    ElMessage.success('配置已删除')
    await loadAdapters()
  } catch (error) {
    console.error('删除失败:', error)
  }
}

const resetForm = () => {
  form.platform = 'qq'
  form.enabled = false
  form.config = {
    api_url: 'http://localhost:5700',
    listen_addr: ':8080',
    bot_token: '',
    app_id: '',
    app_secret: ''
  }

  if (formRef.value) {
    formRef.value.clearValidate()
  }
}

onMounted(() => {
  loadAdapters()
})
</script>

<style scoped>
.adapters {
  width: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.config-info {
  font-size: 13px;
  color: #666;
}
</style>
