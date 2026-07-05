import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

let isRefreshing = false

async function handleUnauthorized() {
  if (isRefreshing) return
  isRefreshing = true
  const authStore = useAuthStore()
  await authStore.logout({ callApi: false, redirect: false })
  if (router.currentRoute.value.path !== '/login') {
    await router.replace('/login')
  }
  ElMessage.error('登录已过期，请重新登录')

  setTimeout(() => {
    isRefreshing = false
  }, 1000)
}

request.interceptors.request.use(
  config => {
    const authStore = useAuthStore()
    if (authStore.token) {
      config.headers.Authorization = `Bearer ${authStore.token}`
    }
    return config
  },
  error => Promise.reject(error)
)

request.interceptors.response.use(
  response => {
    const data = response.data
    if (data?.error === 'Unauthorized') {
      handleUnauthorized()
      return Promise.reject(new Error('Unauthorized'))
    }
    return data
  },
  error => {
    if (error.response) {
      const { status, data } = error.response

      if (status === 401 || data?.error === 'Unauthorized') {
        handleUnauthorized()
      } else if (!error.config?.silent) {
        ElMessage.error(data.error || '请求失败')
      }
    } else {
      ElMessage.error('网络错误，请检查连接')
    }

    return Promise.reject(error)
  }
)

export default request
