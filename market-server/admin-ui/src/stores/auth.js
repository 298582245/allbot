import { defineStore } from 'pinia'
import { ref } from 'vue'
import request from '@/utils/request'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref(JSON.parse(localStorage.getItem('user') || 'null'))

  const isAuthenticated = ref(!!token.value)

  const login = async (username, password) => {
    const formData = new FormData()
    formData.append('username', username)
    formData.append('password', password)

    const response = await request.post('/token', formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })

    token.value = response.access_token
    isAuthenticated.value = true

    localStorage.setItem('token', response.access_token)

    // 获取用户信息
    await fetchUserInfo()
  }

  const fetchUserInfo = async () => {
    // 实际应用中需要实现获取用户信息的API
    user.value = { username: 'developer' }
    localStorage.setItem('user', JSON.stringify(user.value))
  }

  const logout = () => {
    token.value = ''
    user.value = null
    isAuthenticated.value = false
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  return {
    token,
    user,
    isAuthenticated,
    login,
    logout
  }
})
