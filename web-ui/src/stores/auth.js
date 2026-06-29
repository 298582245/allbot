import { defineStore } from 'pinia'
import { ref } from 'vue'
import router from '@/router'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const username = ref(localStorage.getItem('username') || '')

  const isAuthenticated = ref(!!token.value)

  const setAuth = (newToken, newUsername) => {
    token.value = newToken
    username.value = newUsername
    isAuthenticated.value = true

    localStorage.setItem('token', newToken)
    localStorage.setItem('username', newUsername)
  }

  const logout = async ({ callApi = true, redirect = true } = {}) => {
    if (callApi) {
      try {
        await fetch('/api/logout', { method: 'POST' })
      } catch (error) {
        console.warn('退出登录接口调用失败', error)
      }
    }

    token.value = ''
    username.value = ''
    isAuthenticated.value = false

    localStorage.removeItem('token')
    localStorage.removeItem('username')

    if (redirect) {
      router.push('/login')
    }
  }

  return {
    token,
    username,
    isAuthenticated,
    setAuth,
    logout
  }
})
