<template>
  <div class="route-chunk-state" :role="error ? 'alert' : 'status'">
    <el-icon v-if="!error" class="route-loading-icon is-loading"><Loading /></el-icon>
    <template v-else>
      <el-icon class="route-error-icon"><WarningFilled /></el-icon>
      <span>页面加载失败，请检查网络后重试。</span>
      <el-button size="small" @click="reloadPage">重新加载</el-button>
    </template>
  </div>
</template>

<script setup>
import { Loading, WarningFilled } from '@element-plus/icons-vue'

defineProps({
  error: { type: Error, default: null }
})

function reloadPage() {
  window.location.reload()
}
</script>

<style scoped>
.route-chunk-state {
  width: 100%;
  min-height: 220px;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--text-secondary);
  font-size: 14px;
}

.route-loading-icon,
.route-error-icon {
  font-size: 28px;
}

.route-loading-icon {
  color: var(--brand-500);
}

.route-error-icon {
  color: var(--color-danger, #f56c6c);
}
</style>
