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
    const isEnvelope = data !== null && typeof data === 'object' && !Array.isArray(data) &&
      Object.prototype.hasOwnProperty.call(data, 'code') &&
      Object.prototype.hasOwnProperty.call(data, 'msg') &&
      Object.prototype.hasOwnProperty.call(data, 'data')
    if (data?.error === 'Unauthorized' || (isEnvelope && (Number(data.code) === 401 || data.msg === 'Unauthorized'))) {
      handleUnauthorized()
      return Promise.reject(new Error(data?.msg || data?.error || 'Unauthorized'))
    }
    const responseType = response.config?.responseType
    const disposition = response.headers?.get?.('content-disposition') || response.headers?.['content-disposition'] || ''
    if (responseType === 'blob' || responseType === 'arraybuffer' || data instanceof Blob || data instanceof ArrayBuffer || disposition.toLowerCase().includes('attachment')) return data
    return isEnvelope ? data.data : data
  },
  error => {
    if (error.response) {
      const { status, data } = error.response

      if (status === 401 || data?.error === 'Unauthorized' || Number(data?.code) === 401) {
        handleUnauthorized()
      } else if (!error.config?.silent) {
        ElMessage.error(data?.msg || data?.error || data?.message || '请求失败')
      }
    } else if (!error.config?.silent) {
      ElMessage.error('网络错误，请检查连接')
    }

    return Promise.reject(error)
  }
)

export default request
