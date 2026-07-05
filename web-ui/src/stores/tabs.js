import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const AFFIX_PATHS = ['/dashboard']

export const useTabsStore = defineStore('tabs', () => {
  const visitedTabs = ref([])
  const activeTabPath = ref('')

  const cachedNames = computed(() =>
    visitedTabs.value.map(tab => tab.name).filter(Boolean)
  )

  function isAffix(path) {
    return AFFIX_PATHS.includes(path)
  }

  function addTab(tab) {
    if (!tab?.path) return
    const exists = visitedTabs.value.find(t => t.path === tab.path)
    if (!exists) {
      visitedTabs.value.push({
        path: tab.path,
        title: tab.title || '未命名',
        name: tab.name || ''
      })
    }
    activeTabPath.value = tab.path
  }

  function removeTab(path) {
    if (isAffix(path)) return null
    const index = visitedTabs.value.findIndex(t => t.path === path)
    if (index === -1) return null
    visitedTabs.value.splice(index, 1)
    if (activeTabPath.value === path) {
      const next = visitedTabs.value[index] || visitedTabs.value[index - 1]
      return next || null
    }
    return null
  }

  function removeOthers(path) {
    visitedTabs.value = visitedTabs.value.filter(t => t.path === path || isAffix(t.path))
    activeTabPath.value = path
  }

  function removeAll() {
    visitedTabs.value = visitedTabs.value.filter(t => isAffix(t.path))
    const last = visitedTabs.value[visitedTabs.value.length - 1]
    activeTabPath.value = last ? last.path : ''
    return last || null
  }

  function removeLeft(path) {
    const index = visitedTabs.value.findIndex(t => t.path === path)
    if (index === -1) return
    visitedTabs.value = visitedTabs.value.filter((t, i) => i >= index || isAffix(t.path))
  }

  function removeRight(path) {
    const index = visitedTabs.value.findIndex(t => t.path === path)
    if (index === -1) return
    visitedTabs.value = visitedTabs.value.filter((t, i) => i <= index || isAffix(t.path))
  }

  function moveTab(fromIndex, toIndex) {
    if (fromIndex === toIndex || fromIndex < 0 || toIndex < 0) return
    if (fromIndex >= visitedTabs.value.length || toIndex >= visitedTabs.value.length) return
    const tabs = [...visitedTabs.value]
    const [moved] = tabs.splice(fromIndex, 1)
    tabs.splice(toIndex, 0, moved)
    visitedTabs.value = tabs
  }

  function initAffixTabs() {
    AFFIX_PATHS.forEach(path => {
      const exists = visitedTabs.value.find(t => t.path === path)
      if (!exists) {
        const title = path === '/dashboard' ? '仪表盘' : '首页'
        visitedTabs.value.push({ path, title, name: 'Dashboard' })
      }
    })
  }

  return {
    visitedTabs,
    activeTabPath,
    cachedNames,
    isAffix,
    addTab,
    removeTab,
    removeOthers,
    removeAll,
    removeLeft,
    removeRight,
    moveTab,
    initAffixTabs
  }
})
