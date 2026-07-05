<template>
  <div class="login-container">
    <div class="login-box">
      <div class="login-header">
        <div class="login-logo">
          <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect width="32" height="32" rx="8" fill="var(--brand-500)"/>
            <path d="M10 12.5C10 11.1193 11.1193 10 12.5 10H19.5C20.8807 10 22 11.1193 22 12.5V16C22 17.3807 20.8807 18.5 19.5 18.5H17L14 21V18.5H12.5C11.1193 18.5 10 17.3807 10 16V12.5Z" fill="white" fill-opacity="0.9"/>
            <circle cx="14" cy="14.5" r="1.2" fill="var(--brand-600)"/>
            <circle cx="18" cy="14.5" r="1.2" fill="var(--brand-600)"/>
          </svg>
        </div>
        <h1>AllBot</h1>
        <p>去中心化多平台机器人框架</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        class="login-form"
        @submit.prevent="handleLogin"
      >
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            placeholder="用户名"
            size="large"
            :prefix-icon="User"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            size="large"
            :prefix-icon="Lock"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            @click="handleLogin"
            style="width: 100%"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <div class="login-footer">
        <p>首次启动会在控制台输出随机管理员密码</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { login } from '@/api'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref(null)
const loading = ref(false)

const form = reactive({
  username: 'admin',
  password: ''
})

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    loading.value = true

    try {
      const res = await login(form)
      authStore.setAuth(res.token, form.username)
      ElMessage.success('登录成功')
      router.push('/')
    } catch (error) {
      console.error('登录失败:', error)
    } finally {
      loading.value = false
    }
  })
}
</script>

<style scoped>
.login-container {
  width: 100%;
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-base);
  position: relative;
  overflow: hidden;
}

.login-container::before {
  content: '';
  position: absolute;
  top: -10%;
  right: -5%;
  width: 500px;
  height: 500px;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.12), transparent 70%);
  border-radius: 50%;
  filter: blur(40px);
  pointer-events: none;
}

.login-container::after {
  content: '';
  position: absolute;
  bottom: -10%;
  left: -5%;
  width: 400px;
  height: 400px;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.06), transparent 70%);
  border-radius: 50%;
  filter: blur(40px);
  pointer-events: none;
}

.login-box {
  position: relative;
  z-index: 1;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-lg), 0 0 40px rgba(99, 102, 241, 0.04);
  padding: 40px;
  width: 400px;
  animation: scale-in 0.5s ease-out backwards;
}

.login-header {
  text-align: center;
  margin-bottom: 40px;
}

.login-logo {
  display: flex;
  justify-content: center;
  margin-bottom: 16px;
}

.login-logo svg {
  width: 48px;
  height: 48px;
  filter: drop-shadow(0 4px 12px rgba(99, 102, 241, 0.25));
}

.login-header h1 {
  font-size: 28px;
  font-weight: 800;
  color: var(--text-primary);
  margin-bottom: 8px;
  letter-spacing: -0.02em;
}

.login-header p {
  font-size: 14px;
  color: var(--text-secondary);
}

.login-form {
  margin-bottom: 20px;
}

.login-footer {
  text-align: center;
}

.login-footer p {
  font-size: 12px;
  color: var(--text-tertiary);
}

@media (max-width: 768px) {
  .login-container {
    padding: 16px;
  }

  .login-box {
    width: 100%;
    max-width: 400px;
    padding: 28px 20px;
  }

  .login-header {
    margin-bottom: 28px;
  }

  .login-header h1 {
    font-size: 24px;
  }
}
</style>
