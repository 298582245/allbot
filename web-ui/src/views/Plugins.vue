<template>
  <div class="plugins">
    <div class="plugins-header">
      <span class="title">插件列表</span>
      <el-button type="primary" size="small">
        <el-icon><Plus /></el-icon>
        安装插件
      </el-button>
    </div>

    <div class="plugins-content" v-loading="loading">
      <div class="plugin-grid" v-if="paginatedPlugins.length > 0">
        <div
          class="plugin-card"
          v-for="plugin in paginatedPlugins"
          :key="plugin.id"
        >
          <div class="plugin-card-header">
            <span class="plugin-name">{{ plugin.name }}</span>
            <el-tag
              :type="plugin.enabled ? 'success' : 'info'"
              size="small"
            >
              {{ plugin.enabled ? '已启用' : '已禁用' }}
            </el-tag>
          </div>

          <div class="plugin-card-body">
            <div class="plugin-info-row">
              <span class="label">版本：</span>
              <span>{{ plugin.version }}</span>
            </div>
            <div class="plugin-info-row">
              <span class="label">运行时：</span>
              <el-tag size="small">{{ plugin.runtime }}</el-tag>
            </div>
            <div class="plugin-info-row">
              <span class="label">指令：</span>
              <code class="trigger-text">{{ plugin.trigger || '无' }}</code>
            </div>
            <div class="plugin-info-row">
              <span class="label">平台：</span>
              <span class="platforms">
                <el-tag
                  v-for="platform in plugin.platforms"
                  :key="platform"
                  size="small"
                  type="info"
                >
                  {{ getPlatformName(platform) }}
                </el-tag>
                <span v-if="!plugin.platforms || plugin.platforms.length === 0" style="color: #999">无</span>
              </span>
            </div>
            <div class="plugin-info-row" v-if="plugin.error">
              <span class="label">错误：</span>
              <span style="color: #f56c6c">{{ plugin.error }}</span>
            </div>
          </div>

          <div class="plugin-card-footer">
            <el-button
              v-if="plugin.enabled"
              type="warning"
              size="small"
              @click="handleDisable(plugin)"
            >
              禁用
            </el-button>
            <el-button
              v-else
              type="success"
              size="small"
              @click="handleEnable(plugin)"
            >
              启用
            </el-button>
            <el-button type="primary" size="small" @click="handleReload(plugin)">
              重载
            </el-button>
            <el-button type="info" size="small" @click="handleConfig(plugin)">
              配置
            </el-button>
            <el-button size="small" @click="handleEditCode(plugin)">
              代码
            </el-button>
            <el-button type="danger" size="small" @click="handleDelete(plugin)">
              删除
            </el-button>
          </div>
        </div>
      </div>

      <el-empty v-if="!loading && plugins.length === 0" description="暂无插件" />
    </div>

    <div class="plugins-pagination">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="plugins.length"
        layout="total, prev, pager, next"
        background
      />
    </div>

    <!-- 配置编辑对话框 -->
    <el-dialog
      v-model="configDialogVisible"
      title="插件配置"
      width="800px"
      @close="handleConfigDialogClose"
    >
      <el-form :model="currentConfig" label-width="120px">
        <el-form-item label="插件名称">
          <el-input v-model="currentConfig.name" />
        </el-form-item>
        <el-form-item label="版本">
          <el-input v-model="currentConfig.version" />
        </el-form-item>
        <el-form-item label="运行时">
          <el-select v-model="currentConfig.runtime">
            <el-option label="Python" value="python" />
            <el-option label="Node.js" value="nodejs" />
          </el-select>
        </el-form-item>
        <el-form-item label="入口文件">
          <el-input v-model="currentConfig.entry" />
        </el-form-item>
        <el-form-item label="触发规则">
          <el-input v-model="currentConfig.trigger" placeholder="正则表达式" />
        </el-form-item>
        <el-form-item label="支持平台">
          <el-checkbox-group v-model="currentConfig.platforms">
            <el-checkbox label="qq">QQ</el-checkbox>
            <el-checkbox label="wechat">微信</el-checkbox>
            <el-checkbox label="telegram">Telegram</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="currentConfig.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="configDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveConfig">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getPlugins, controlPlugin, deletePlugin } from '@/api'
import request from '@/utils/request'

const router = useRouter()

const loading = ref(false)
const plugins = ref([])
const currentPage = ref(1)
const pageSize = 8
const configDialogVisible = ref(false)
const currentPluginId = ref('')
const currentConfig = ref({
  name: '',
  version: '',
  runtime: 'python',
  entry: '',
  trigger: '',
  platforms: [],
  enabled: true
})

const paginatedPlugins = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return plugins.value.slice(start, start + pageSize)
})

const loadPlugins = async () => {
  loading.value = true
  try {
    plugins.value = await getPlugins()
  } catch (error) {
    console.error('加载插件失败:', error)
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

const handleEnable = async (plugin) => {
  try {
    await controlPlugin(plugin.id, 'enable')
    ElMessage.success(`插件 ${plugin.name} 已启用`)
    await loadPlugins()
  } catch (error) {
    console.error('启用插件失败:', error)
    ElMessage.error('启用插件失败')
  }
}

const handleDisable = async (plugin) => {
  try {
    await controlPlugin(plugin.id, 'disable')
    ElMessage.success(`插件 ${plugin.name} 已禁用`)
    await loadPlugins()
  } catch (error) {
    console.error('禁用插件失败:', error)
    ElMessage.error('禁用插件失败')
  }
}

const handleReload = async (plugin) => {
  try {
    await controlPlugin(plugin.id, 'reload')
    ElMessage.success(`插件 ${plugin.name} 已重新加载`)
    await loadPlugins()
  } catch (error) {
    console.error('重新加载插件失败:', error)
    ElMessage.error('重新加载插件失败')
  }
}

const handleConfig = async (plugin) => {
  try {
    currentPluginId.value = plugin.id
    const config = await request.get(`/plugins/config/${plugin.id}`)
    currentConfig.value = config
    configDialogVisible.value = true
  } catch (error) {
    console.error('获取插件配置失败:', error)
    ElMessage.error('获取插件配置失败')
  }
}

const handleEditCode = (plugin) => {
  router.push(`/plugins/${plugin.id}/edit`)
}

const saveConfig = async () => {
  try {
    await request.put(`/plugins/config/${currentPluginId.value}`, currentConfig.value)
    ElMessage.success('配置已保存并生效')
    configDialogVisible.value = false
    await loadPlugins()
  } catch (error) {
    console.error('保存配置失败:', error)
    ElMessage.error('保存配置失败')
  }
}

const handleConfigDialogClose = () => {
  currentConfig.value = {
    name: '',
    version: '',
    runtime: 'python',
    entry: '',
    trigger: '',
    platforms: [],
    enabled: true
  }
}

const handleDelete = async (plugin) => {
  await ElMessageBox.confirm(
    `确定要删除插件 "${plugin.name}" 吗？`,
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )

  try {
    await deletePlugin(plugin.id)
    ElMessage.success(`插件 ${plugin.name} 已删除`)
    await loadPlugins()
  } catch (error) {
    console.error('删除插件失败:', error)
    ElMessage.error('删除插件失败')
  }
}

onMounted(() => {
  loadPlugins()
})
</script>

<style scoped>
.plugins {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.plugins-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.plugins-header .title {
  font-size: 18px;
  font-weight: bold;
}

.plugins-content {
  flex: 1;
  overflow: auto;
}

.plugin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.plugin-card {
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  background: #fff;
  transition: box-shadow 0.2s;
  min-height: 220px;
}

.plugin-card:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

.plugin-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid #f0f0f0;
}

.plugin-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.plugin-card-body {
  flex: 1;
  margin-bottom: 12px;
}

.plugin-info-row {
  display: flex;
  align-items: center;
  margin-bottom: 6px;
  font-size: 13px;
  color: #606266;
}

.plugin-info-row .label {
  color: #909399;
  min-width: 50px;
  flex-shrink: 0;
}

.trigger-text {
  background: #f5f7fa;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 12px;
  color: #606266;
  word-break: break-all;
}

.platforms .el-tag {
  margin-right: 4px;
}

.plugin-card-footer {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 10px;
  border-top: 1px solid #f0f0f0;
}

.plugins-pagination {
  padding-top: 16px;
  display: flex;
  justify-content: center;
  border-top: 1px solid #ebeef5;
  margin-top: 16px;
}
</style>
