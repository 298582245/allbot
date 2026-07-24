import { defineStore } from 'pinia'
import { ref } from 'vue'
import router from '@/router'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const csrfToken = ref(localStorage.getItem('csrfToken') || '')
  const username = ref(localStorage.getItem('username') || '')

  const isAuthenticated = ref(!!token.value)

  const setAuth = (newToken, newUsername, newCsrfToken) => {
    token.value = newToken
    csrfToken.value = newCsrfToken
    username.value = newUsername
    isAuthenticated.value = true

    localStorage.setItem('token', newToken)
    localStorage.setItem('csrfToken', newCsrfToken)
    localStorage.setItem('username', newUsername)
  }

  const logout = async ({ callApi = true, redirect = true } = {}) => {
    if (callApi) {
      try {
        await fetch('/api/logout', {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token.value}`,
            'X-AllBot-CSRF': csrfToken.value
          }
        })
      } catch (error) {
        console.warn('退出登录接口调用失败', error)
      }
    }

    token.value = ''
    csrfToken.value = ''
    username.value = ''
    isAuthenticated.value = false

    localStorage.removeItem('token')
    localStorage.removeItem('csrfToken')
    localStorage.removeItem('username')

    if (redirect) {
      router.push('/login')
    }
  }

  return {
    token,
    csrfToken,
    username,
    isAuthenticated,
    setAuth,
    logout
  }
})
