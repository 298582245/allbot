<template>
  <div class="plugin-panel-page">
    <el-card v-if="panel" class="panel-card" shadow="never">
      <template #header>
        <div class="panel-header">
          <div>
            <div class="panel-title">{{ panel.title }}</div>
            <div class="panel-subtitle">插件 ID：{{ panel.plugin_id }}</div>
          </div>
          <el-button size="small" @click="reloadPanel">刷新</el-button>
        </div>
      </template>
      <iframe :key="iframeKey" class="plugin-frame" :src="panel.entry_url" :title="panel.title"></iframe>
    </el-card>
    <el-empty v-else description="插件面板不存在或未启用" />
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { usePluginWebPanelsStore } from '@/stores/pluginWebPanels'

const route = useRoute()
const panelsStore = usePluginWebPanelsStore()
const iframeKey = ref(0)

panelsStore.loadPanels()

const panel = computed(() => panelsStore.findPanel(route.params.pluginId))

function reloadPanel() {
  iframeKey.value += 1
}
</script>

<style scoped>
.plugin-panel-page {
  height: 100%;
  min-height: calc(100vh - 140px);
}

.panel-card {
  height: 100%;
  min-height: calc(100vh - 140px);
  display: flex;
  flex-direction: column;
}

.panel-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  padding: 0;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.panel-title {
  font-weight: 700;
  color: var(--text-primary);
}

.panel-subtitle {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

.plugin-frame {
  display: block;
  width: 100%;
  height: calc(100vh - 220px);
  min-height: 520px;
  border: 0;
  background: #fff;
}

@media (max-width: 768px) {
  .plugin-panel-page,
  .panel-card {
    min-height: calc(100dvh - 140px);
  }

  .plugin-frame {
    height: calc(100dvh - 210px);
    min-height: 420px;
  }
}
</style>
