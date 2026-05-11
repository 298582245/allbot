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

  const logout = () => {
    token.value = ''
    username.value = ''
    isAuthenticated.value = false

    localStorage.removeItem('token')
    localStorage.removeItem('username')

    router.push('/login')
  }

  return {
    token,
    username,
    isAuthenticated,
    setAuth,
    logout
  }
})
