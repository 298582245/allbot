<template>
  <div class="std-pagination">
    <el-pagination
      :current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      :page-sizes="pageSizes || undefined"
      :layout="resolvedLayout"
      background
      @update:current-page="val => emit('update:currentPage', val)"
      @update:page-size="val => emit('update:pageSize', val)"
      @current-change="val => emit('current-change', val)"
      @size-change="val => emit('size-change', val)"
    />
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  currentPage: { type: Number, default: 1 },
  pageSize: { type: Number, default: 20 },
  total: { type: Number, required: true },
  pageSizes: { type: Array, default: null },
  layout: { type: String, default: '' }
})

const emit = defineEmits(['update:currentPage', 'update:pageSize', 'current-change', 'size-change'])

const resolvedLayout = computed(() => {
  if (props.layout) return props.layout
  return props.pageSizes && props.pageSizes.length
    ? 'total, sizes, prev, pager, next, jumper'
    : 'total, prev, pager, next, jumper'
})
</script>

<style scoped>
.std-pagination {
  flex-shrink: 0;
  padding-top: 12px;
  display: flex;
  justify-content: center;
  border-top: 1px solid #ebeef5;
}

.std-pagination :deep(.el-pagination) {
  max-width: 100%;
}

@media (max-width: 768px) {
  .std-pagination {
    overflow-x: auto;
    justify-content: flex-start;
  }

  .std-pagination :deep(.el-pagination) {
    flex-wrap: nowrap;
    min-width: max-content;
  }

  .std-pagination::-webkit-scrollbar {
    display: none;
  }
}
</style>
