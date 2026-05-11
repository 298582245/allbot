<template>
  <div class="plugins">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>插件列表</span>
          <el-button type="primary" size="small">
            <el-icon><Plus /></el-icon>
            安装插件
          </el-button>
        </div>
      </template>

      <el-table :data="plugins" v-loading="loading" style="width: 100%">
        <el-table-column prop="name" label="插件名称" min-width="150" />
        <el-table-column prop="version" label="版本" width="100" />
        <el-table-column prop="runtime" label="运行时" width="100">
          <template #default="{ row }">
            <el-tag size="small">{{ row.runtime }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '已启用' : '已禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="平台" min-width="150">
          <template #default="{ row }">
            <el-tag
              v-for="platform in row.platforms"
              :key="platform"
              size="small"
              style="margin-right: 5px"
            >
              {{ getPlatformName(platform) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="依赖" min-width="150">
          <template #default="{ row }">
            <el-tag
              v-if="row.dependencies && Object.keys(row.dependencies).length > 0"
              size="small"
              type="info"
            >
              {{ Object.keys(row.dependencies).length }} 个依赖
            </el-tag>
            <span v-else style="color: #999">无依赖</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.enabled"
              type="warning"
              size="small"
              @click="handleDisable(row)"
            >
              禁用
            </el-button>
            <el-button
              v-else
              type="success"
              size="small"
              @click="handleEnable(row)"
            >
              启用
            </el-button>
            <el-button type="primary" size="small" @click="handleReload(row)">
              重新加载
            </el-button>
            <el-button type="info" size="small" @click="handleConfig(row)">
              配置
            </el-button>
            <el-button type="danger" size="small" @click="handleDelete(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && plugins.length === 0" description="暂无插件" />
    </el-card>

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
        <el-form-item label="依赖管理">
          <el-table :data="dependenciesArray" style="width: 100%">
            <el-table-column prop="name" label="包名" width="200">
              <template #default="{ row, $index }">
                <el-input v-model="dependenciesArray[$index].name" size="small" />
              </template>
            </el-table-column>
            <el-table-column prop="version" label="版本" width="150">
              <template #default="{ row, $index }">
                <el-input v-model="dependenciesArray[$index].version" size="small" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100">
              <template #default="{ $index }">
                <el-button
                  type="danger"
                  size="small"
                  @click="removeDependency($index)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-button
            type="primary"
            size="small"
            style="margin-top: 10px"
            @click="addDependency"
          >
            添加依赖
          </el-button>
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
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { getPlugins, controlPlugin, deletePlugin } from '@/api'
import request from '@/utils/request'

const loading = ref(false)
const plugins = ref([])
const configDialogVisible = ref(false)
const currentPluginId = ref('')
const currentConfig = ref({
  name: '',
  version: '',
  runtime: 'python',
  entry: '',
  trigger: '',
  platforms: [],
  enabled: true,
  dependencies: {}
})
const dependenciesArray = ref([])

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
    // 获取插件配置
    const config = await request.get(`/api/plugins/config/${plugin.id}`)
    currentConfig.value = config

    // 转换 dependencies 对象为数组
    dependenciesArray.value = Object.entries(config.dependencies || {}).map(([name, version]) => ({
      name,
      version
    }))

    configDialogVisible.value = true
  } catch (error) {
    console.error('获取插件配置失败:', error)
    ElMessage.error('获取插件配置失败')
  }
}

const addDependency = () => {
  dependenciesArray.value.push({ name: '', version: '' })
}

const removeDependency = (index) => {
  dependenciesArray.value.splice(index, 1)
}

const saveConfig = async () => {
  try {
    // 转换 dependencies 数组为对象
    const dependencies = {}
    dependenciesArray.value.forEach(dep => {
      if (dep.name && dep.version) {
        dependencies[dep.name] = dep.version
      }
    })

    const configToSave = {
      ...currentConfig.value,
      dependencies
    }

    // 保存配置
    await request.put(`/api/plugins/config/${currentPluginId.value}`, configToSave)

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
    enabled: true,
    dependencies: {}
  }
  dependenciesArray.value = []
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
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
