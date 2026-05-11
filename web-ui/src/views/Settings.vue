<template>
  <div class="settings">
    <el-card>
      <template #header>
        <span>系统设置</span>
      </template>

      <el-form :model="form" label-width="120px">
        <el-divider content-position="left">管理员账号</el-divider>

        <el-form-item label="用户名">
          <el-input v-model="form.username" disabled />
        </el-form-item>

        <el-form-item label="修改密码">
          <el-button type="primary" @click="showPasswordDialog">
            修改密码
          </el-button>
        </el-form-item>

        <el-divider content-position="left">Web UI 配置</el-divider>

        <el-form-item label="端口">
          <el-input-number v-model="form.webPort" :min="1000" :max="65535" />
        </el-form-item>

        <el-form-item label="自动刷新">
          <el-switch v-model="form.autoRefresh" />
          <span style="margin-left: 10px; color: #999">
            {{ form.autoRefresh ? '启用' : '禁用' }}
          </span>
        </el-form-item>

        <el-form-item label="刷新间隔">
          <el-input-number
            v-model="form.refreshInterval"
            :min="1"
            :max="60"
            :disabled="!form.autoRefresh"
          />
          <span style="margin-left: 10px; color: #999">秒</span>
        </el-form-item>

        <el-divider content-position="left">插件配置</el-divider>

        <el-form-item label="插件目录">
          <el-input v-model="form.pluginDir" />
        </el-form-item>

        <el-form-item label="自动加载">
          <el-switch v-model="form.autoLoadPlugins" />
          <span style="margin-left: 10px; color: #999">
            启动时自动加载所有插件
          </span>
        </el-form-item>

        <el-divider content-position="left">系统信息</el-divider>

        <el-form-item label="版本">
          <span>AllBot v1.0.0</span>
        </el-form-item>

        <el-form-item label="Go 版本">
          <span>1.21+</span>
        </el-form-item>

        <el-form-item label="数据库">
          <span>SQLite 3</span>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="handleSave">
            保存设置
          </el-button>
          <el-button @click="handleReset">
            重置
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 修改密码对话框 -->
    <el-dialog v-model="passwordDialogVisible" title="修改密码" width="400px">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="100px">
        <el-form-item label="当前密码" prop="oldPassword">
          <el-input v-model="passwordForm.oldPassword" type="password" />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input v-model="passwordForm.newPassword" type="password" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input v-model="passwordForm.confirmPassword" type="password" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleChangePassword">
          确定
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const form = reactive({
  username: authStore.username,
  webPort: 3000,
  autoRefresh: true,
  refreshInterval: 5,
  pluginDir: './plugins',
  autoLoadPlugins: true
})

const passwordDialogVisible = ref(false)
const passwordFormRef = ref(null)

const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const passwordRules = {
  oldPassword: [
    { required: true, message: '请输入当前密码', trigger: 'blur' }
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于 6 位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== passwordForm.newPassword) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const showPasswordDialog = () => {
  passwordDialogVisible.value = true
}

const handleChangePassword = async () => {
  if (!passwordFormRef.value) return

  await passwordFormRef.value.validate(async (valid) => {
    if (!valid) return

    // TODO: 实现修改密码 API
    ElMessage.success('密码修改成功，请重新登录')
    passwordDialogVisible.value = false

    // 重置表单
    passwordForm.oldPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''

    // 退出登录
    setTimeout(() => {
      authStore.logout()
    }, 1500)
  })
}

const handleSave = () => {
  // TODO: 实现保存设置 API
  ElMessage.success('设置已保存')
}

const handleReset = () => {
  form.webPort = 3000
  form.autoRefresh = true
  form.refreshInterval = 5
  form.pluginDir = './plugins'
  form.autoLoadPlugins = true

  ElMessage.info('设置已重置')
}
</script>

<style scoped>
.settings {
  width: 100%;
}
</style>
