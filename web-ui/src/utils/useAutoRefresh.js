import { onBeforeUnmount, watch } from 'vue'
import { useUiSettingsStore } from '@/stores/uiSettings'

export const useAutoRefresh = (callback) => {
  const settings = useUiSettingsStore()
  let timer = null

  const stopWatch = watch(
    () => [settings.loaded, settings.autoRefresh, settings.refreshInterval],
    ([loaded, enabled, interval]) => {
      if (timer) clearInterval(timer)
      timer = loaded && enabled ? window.setInterval(callback, interval * 1000) : null
    },
    { immediate: true }
  )

  settings.loadSettings()

  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
    stopWatch()
  })
}
