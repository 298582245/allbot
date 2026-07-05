<template>
  <div class="plugin-panels-page">
    <el-card shadow="never">
      <template #header>
        <div class="page-header">
          <div>
            <div class="page-title">插件面板</div>
            <div class="page-subtitle">集中进入已启用的插件 WebUI，避免侧栏展示过长</div>
          </div>
          <el-button :loading="panelsStore.loading" @click="panelsStore.loadPanels(true)">刷新</el-button>
        </div>
      </template>

      <div class="toolbar">
        <el-input
          v-model="keyword"
          clearable
          placeholder="搜索插件名称或插件 ID"
          class="search-input"
        />
      </div>

      <el-empty v-if="!panelsStore.panels.length" description="暂无已启用的插件面板" />
      <el-empty v-else-if="!filteredPanels.length" description="没有匹配的插件面板" />
      <div v-else class="panel-grid">
        <router-link
          v-for="panel in filteredPanels"
          :key="panel.plugin_id"
          :to="`/plugin-panels/${encodeURIComponent(panel.plugin_id)}`"
          class="panel-card"
        >
          <div class="panel-icon">{{ panel.title.slice(0, 1) }}</div>
          <div class="panel-info">
            <div class="panel-title">{{ panel.title }}</div>
            <div class="panel-id">{{ panel.plugin_id }}</div>
          </div>
          <el-button type="primary" link>打开</el-button>
        </router-link>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { usePluginWebPanelsStore } from '@/stores/pluginWebPanels'

const panelsStore = usePluginWebPanelsStore()
const keyword = ref('')

const filteredPanels = computed(() => {
  const text = keyword.value.trim().toLowerCase()
  if (!text) return panelsStore.panels
  return panelsStore.panels.filter((panel) => `${panel.title} ${panel.plugin_id}`.toLowerCase().includes(text))
})

panelsStore.loadPanels()
</script>

<style scoped>
.plugin-panels-page {
  min-height: 100%;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.page-title {
  font-weight: 700;
  color: var(--text-primary);
}

.page-subtitle {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

.toolbar {
  margin-bottom: 16px;
}

.search-input {
  max-width: 360px;
}

.panel-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 14px;
}

.panel-card {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 16px;
  color: inherit;
  text-decoration: none;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-surface);
  transition: all var(--transition-normal);
}

.panel-card:hover {
  border-color: var(--border-brand);
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.panel-icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-weight: 700;
  color: #fff;
  background: var(--brand-500);
}

.panel-info {
  flex: 1;
  min-width: 0;
}

.panel-title {
  font-weight: 700;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.panel-id {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .panel-grid {
    grid-template-columns: 1fr;
  }
}
</style>
