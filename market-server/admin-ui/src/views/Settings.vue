<template>
  <div class="settings">
    <el-card>
      <template #header>
        <span>系统设置</span>
      </template>

      <el-tabs v-model="activeTab">
        <el-tab-pane label="基本信息" name="basic">
          <el-form :model="basicForm" label-width="120px" style="max-width: 600px">
            <el-form-item label="用户名">
              <el-input v-model="basicForm.username" disabled />
            </el-form-item>

            <el-form-item label="邮箱">
              <el-input v-model="basicForm.email" />
            </el-form-item>

            <el-form-item label="角色">
              <el-tag>{{ basicForm.role }}</el-tag>
            </el-form-item>

            <el-form-item>
              <el-button type="primary" @click="handleSaveBasic">保存</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="修改密码" name="password">
          <el-form :model="passwordForm" :rules="passwordRules" ref="passwordFormRef" label-width="120px" style="max-width: 600px">
            <el-form-item label="当前密码" prop="oldPassword">
              <el-input v-model="passwordForm.oldPassword" type="password" />
            </el-form-item>

            <el-form-item label="新密码" prop="newPassword">
              <el-input v-model="passwordForm.newPassword" type="password" />
            </el-form-item>

            <el-form-item label="确认密码" prop="confirmPassword">
              <el-input v-model="passwordForm.confirmPassword" type="password" />
            </el-form-item>

            <el-form-item>
              <el-button type="primary" @click="handleChangePassword">修改密码</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="支付配置" name="payment">
          <el-form :model="paymentForm" label-width="150px" style="max-width: 800px">
            <el-divider content-position="left">支付宝配置</el-divider>
            <el-form-item label="App ID">
              <el-input v-model="paymentForm.alipay.appId" />
            </el-form-item>
            <el-form-item label="私钥">
              <el-input v-model="paymentForm.alipay.privateKey" type="textarea" :rows="3" />
            </el-form-item>

            <el-divider content-position="left">微信支付配置</el-divider>
            <el-form-item label="App ID">
              <el-input v-model="paymentForm.wechat.appId" />
            </el-form-item>
            <el-form-item label="商户号">
              <el-input v-model="paymentForm.wechat.mchId" />
            </el-form-item>
            <el-form-item label="API密钥">
              <el-input v-model="paymentForm.wechat.apiKey" type="password" />
            </el-form-item>

            <el-divider content-position="left">Stripe配置</el-divider>
            <el-form-item label="API Key">
              <el-input v-model="paymentForm.stripe.apiKey" type="password" />
            </el-form-item>

            <el-form-item>
              <el-button type="primary" @click="handleSavePayment">保存配置</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const activeTab = ref('basic')
const passwordFormRef = ref(null)

const basicForm = ref({
  username: 'developer',
  email: 'developer@example.com',
  role: 'developer'
})

const passwordForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const passwordRules = {
  oldPassword: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度至少6位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== passwordForm.value.newPassword) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const paymentForm = ref({
  alipay: {
    appId: '',
    privateKey: ''
  },
  wechat: {
    appId: '',
    mchId: '',
    apiKey: ''
  },
  stripe: {
    apiKey: ''
  }
})

const handleSaveBasic = () => {
  ElMessage.success('基本信息已保存')
}

const handleChangePassword = async () => {
  if (!passwordFormRef.value) return

  await passwordFormRef.value.validate((valid) => {
    if (valid) {
      ElMessage.success('密码修改成功')
      passwordForm.value = {
        oldPassword: '',
        newPassword: '',
        confirmPassword: ''
      }
    }
  })
}

const handleSavePayment = () => {
  ElMessage.success('支付配置已保存')
}
</script>

<style scoped>
.settings {
  width: 100%;
}
</style>
