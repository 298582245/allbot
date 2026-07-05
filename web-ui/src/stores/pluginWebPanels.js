import { defineStore } from 'pinia'
import { getPluginWebPanels } from '@/api'

export const usePluginWebPanelsStore = defineStore('pluginWebPanels', {
  state: () => ({
    panels: [],
    loaded: false,
    loading: false
  }),
  actions: {
    async loadPanels(force = false) {
      if (this.loading || (this.loaded && !force)) return this.panels
      this.loading = true
      try {
        this.panels = await getPluginWebPanels()
        this.loaded = true
        return this.panels
      } catch (error) {
        this.panels = []
        return this.panels
      } finally {
        this.loading = false
      }
    },
    findPanel(pluginId) {
      return this.panels.find(item => item.plugin_id === pluginId) || null
    }
  }
})
