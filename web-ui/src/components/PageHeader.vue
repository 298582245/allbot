<template>
  <div class="page-header">
    <div class="page-header-left">
      <h1 v-if="title" class="page-title">{{ title }}</h1>
      <p v-if="description" class="page-description">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="page-header-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, default: '' },
  description: { type: String, default: '' }
})
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  font-family: var(--font-heading);
  color: var(--text-primary);
  letter-spacing: -0.01em;
}
.page-description {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--text-tertiary);
}
.page-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
  }
  .page-header-actions {
    width: 100%;
  }
}
</style>
