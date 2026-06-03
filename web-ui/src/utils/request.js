import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

let isRefreshing = false

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
  response => response.data,
  error => {
    if (error.response) {
      const { status, data } = error.response

      if (status === 401) {
        if (!isRefreshing) {
          isRefreshing = true
          const authStore = useAuthStore()
          authStore.logout()
          ElMessage.error('登录已过期，请重新登录')

          setTimeout(() => {
            isRefreshing = false
          }, 1000)
        }
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
