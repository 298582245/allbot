import { defineStore } from 'pinia'
import { ref } from 'vue'
import request from '@/utils/request'

export const useUiSettingsStore = defineStore('uiSettings', () => {
  const autoRefresh = ref(true)
  const refreshInterval = ref(5)
  const loaded = ref(false)
  let loadPromise = null

  const applySettings = (settings = {}) => {
    autoRefresh.value = settings.auto_refresh !== false
    const interval = Number(settings.refresh_interval)
    refreshInterval.value = Number.isFinite(interval) && interval >= 1 && interval <= 60 ? interval : 5
    loaded.value = true
  }

  const loadSettings = async () => {
    if (loaded.value) return
    if (!loadPromise) {
      loadPromise = request.get('/settings', { silent: true })
        .then(applySettings)
        .catch((error) => {
          console.error('加载自动刷新设置失败:', error)
          applySettings()
        })
        .finally(() => {
          loadPromise = null
        })
    }
    await loadPromise
  }

  return { autoRefresh, refreshInterval, loaded, applySettings, loadSettings }
})
