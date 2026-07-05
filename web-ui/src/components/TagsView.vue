<template>
  <div class="tags-view">
    <button
      v-show="showScrollBtn"
      class="scroll-btn"
      :class="{ disabled: atStart }"
      @click="scrollByDir(-1)"
    >
      <el-icon><ArrowLeft /></el-icon>
    </button>

    <div ref="scrollRef" class="tags-scroll" @scroll="checkScrollState">
      <div class="tags-list">
        <div
          v-for="tab in tabsStore.visitedTabs"
          :key="tab.path"
          class="tag-item"
          :class="{
            active: tab.path === tabsStore.activeTabPath,
            dragging: draggedPath === tab.path,
            'drag-over': dragOverPath === tab.path && draggedPath !== tab.path
          }"
          draggable="true"
          @click="handleClick(tab)"
          @contextmenu.prevent="handleContextMenu($event, tab)"
          @dragstart="onDragStart($event, tab)"
          @dragover.prevent="onDragOver($event, tab)"
          @dragleave="onDragLeave"
          @drop="onDrop($event, tab)"
          @dragend="onDragEnd"
        >
          <span class="tag-dot" v-if="tab.path === tabsStore.activeTabPath"></span>
          <span class="tag-title">{{ tab.title }}</span>
          <el-icon
            v-if="!tabsStore.isAffix(tab.path)"
            class="tag-close"
            @click.stop="handleClose(tab)"
          >
            <Close />
          </el-icon>
        </div>
      </div>
    </div>

    <button
      v-show="showScrollBtn"
      class="scroll-btn"
      :class="{ disabled: atEnd }"
      @click="scrollByDir(1)"
    >
      <el-icon><ArrowRight /></el-icon>
    </button>

    <ul
      v-if="contextMenu.visible"
      class="context-menu"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
    >
      <li v-if="contextMenu.tab && !tabsStore.isAffix(contextMenu.tab.path)" @click="handleContextAction('close')">
        <el-icon><Close /></el-icon>关闭
      </li>
      <li @click="handleContextAction('closeOthers')">
        <el-icon><CircleClose /></el-icon>关闭其他
      </li>
      <li @click="handleContextAction('closeLeft')">
        <el-icon><Back /></el-icon>关闭左侧
      </li>
      <li @click="handleContextAction('closeRight')">
        <el-icon><Right /></el-icon>关闭右侧
      </li>
      <li @click="handleContextAction('closeAll')">
        <el-icon><Remove /></el-icon>全部关闭
      </li>
    </ul>
  </div>
</template>

<script setup>
import { ref, reactive, nextTick, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Close, CircleClose, Back, Right, Remove, ArrowLeft, ArrowRight } from '@element-plus/icons-vue'
import { useTabsStore } from '@/stores/tabs'

const router = useRouter()
const route = useRoute()
const tabsStore = useTabsStore()

const scrollRef = ref(null)
const showScrollBtn = ref(false)
const atStart = ref(true)
const atEnd = ref(false)
const draggedPath = ref('')
const dragOverPath = ref('')

const contextMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  tab: null
})

function handleClick(tab) {
  if (tab.path !== tabsStore.activeTabPath) {
    router.push(tab.path)
  }
}

function handleClose(tab) {
  const next = tabsStore.removeTab(tab.path)
  if (next) router.push(next.path)
}

function handleContextMenu(e, tab) {
  contextMenu.visible = true
  contextMenu.x = e.clientX
  contextMenu.y = e.clientY
  contextMenu.tab = tab
}

function closeContextMenu() {
  contextMenu.visible = false
}

function handleContextAction(action) {
  const tab = contextMenu.tab
  closeContextMenu()
  if (!tab) return

  switch (action) {
    case 'close':
      handleClose(tab)
      break
    case 'closeOthers':
      tabsStore.removeOthers(tab.path)
      if (tabsStore.activeTabPath !== tab.path) router.push(tab.path)
      break
    case 'closeLeft':
      tabsStore.removeLeft(tab.path)
      if (!tabsStore.visitedTabs.find(t => t.path === tabsStore.activeTabPath)) {
        router.push(tab.path)
      }
      break
    case 'closeRight':
      tabsStore.removeRight(tab.path)
      if (!tabsStore.visitedTabs.find(t => t.path === tabsStore.activeTabPath)) {
        router.push(tab.path)
      }
      break
    case 'closeAll': {
      const next = tabsStore.removeAll()
      if (next) router.push(next.path)
      break
    }
  }
}

function scrollByDir(dir) {
  if (!scrollRef.value) return
  const firstTab = scrollRef.value.querySelector('.tag-item')
  const tabWidth = firstTab ? firstTab.offsetWidth : 120
  scrollRef.value.scrollBy({ left: dir * tabWidth * 3, behavior: 'smooth' })
}

function checkOverflow() {
  if (!scrollRef.value) return
  showScrollBtn.value = scrollRef.value.scrollWidth > scrollRef.value.clientWidth + 1
  checkScrollState()
}

function checkScrollState() {
  if (!scrollRef.value) return
  atStart.value = scrollRef.value.scrollLeft <= 0
  atEnd.value = scrollRef.value.scrollLeft >= scrollRef.value.scrollWidth - scrollRef.value.clientWidth - 1
}

function scrollToActiveTab() {
  if (!scrollRef.value) return
  const activeTab = scrollRef.value.querySelector('.tag-item.active')
  if (!activeTab) return
  const containerRect = scrollRef.value.getBoundingClientRect()
  const tabRect = activeTab.getBoundingClientRect()
  const tabLeft = tabRect.left - containerRect.left + scrollRef.value.scrollLeft
  const tabRight = tabLeft + tabRect.width
  const viewLeft = scrollRef.value.scrollLeft
  const viewRight = viewLeft + scrollRef.value.clientWidth
  if (tabLeft < viewLeft) {
    scrollRef.value.scrollTo({ left: tabLeft - 10, behavior: 'smooth' })
  } else if (tabRight > viewRight) {
    scrollRef.value.scrollTo({ left: tabRight - scrollRef.value.clientWidth + 10, behavior: 'smooth' })
  }
}

function onDragStart(e, tab) {
  draggedPath.value = tab.path
  e.dataTransfer.effectAllowed = 'move'
  e.dataTransfer.setData('text/plain', tab.path)
}

function onDragOver(e, tab) {
  if (draggedPath.value === tab.path) return
  dragOverPath.value = tab.path
  e.dataTransfer.dropEffect = 'move'
}

function onDragLeave() {
  dragOverPath.value = ''
}

function onDrop(_e, tab) {
  dragOverPath.value = ''
  if (!draggedPath.value || draggedPath.value === tab.path) return
  const fromIndex = tabsStore.visitedTabs.findIndex(t => t.path === draggedPath.value)
  const toIndex = tabsStore.visitedTabs.findIndex(t => t.path === tab.path)
  if (fromIndex !== -1 && toIndex !== -1) {
    tabsStore.moveTab(fromIndex, toIndex)
  }
  draggedPath.value = ''
}

function onDragEnd() {
  draggedPath.value = ''
  dragOverPath.value = ''
}

function onDocumentClick() {
  closeContextMenu()
}

let resizeObserver = null

onMounted(() => {
  checkOverflow()
  if (scrollRef.value) {
    resizeObserver = new ResizeObserver(checkOverflow)
    resizeObserver.observe(scrollRef.value)
  }
  document.addEventListener('click', onDocumentClick)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
  if (resizeObserver) resizeObserver.disconnect()
})

watch(() => tabsStore.visitedTabs.length, () => {
  nextTick(() => {
    checkOverflow()
    requestAnimationFrame(scrollToActiveTab)
  })
})

watch(() => route.path, () => {
  nextTick(() => {
    requestAnimationFrame(scrollToActiveTab)
  })
})
</script>

<style scoped>
.tags-view {
  display: flex;
  align-items: center;
  background: var(--bg-surface, #fff);
  border-bottom: 1px solid var(--border-subtle, #ebeef5);
  padding: 0 4px;
  flex-shrink: 0;
  height: 38px;
}

.scroll-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--text-secondary, #909399);
  cursor: pointer;
  border-radius: 4px;
  flex-shrink: 0;
  transition: all 0.15s;
}

.scroll-btn:hover {
  color: var(--brand-500, #6366f1);
  background: var(--bg-surface-hover, #f5f7fa);
}

.scroll-btn.disabled {
  opacity: 0.3;
  pointer-events: none;
}

.tags-scroll {
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.tags-scroll::-webkit-scrollbar {
  display: none;
}

.tags-list {
  display: flex;
  align-items: center;
  white-space: nowrap;
  height: 28px;
}

.tag-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 12px;
  border: none;
  border-right: 1px solid var(--border-subtle, #ebeef5);
  background: transparent;
  color: var(--text-secondary, #606266);
  font-size: 13px;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s, color 0.15s;
  flex-shrink: 0;
}

.tag-item:last-child {
  border-right: none;
}

.tag-item:hover {
  color: var(--brand-500, #6366f1);
  background: var(--bg-surface-hover, #f5f7fa);
}

.tag-item.active {
  color: #fff;
  background: var(--brand-500, #6366f1);
}

.tag-item.active:hover {
  color: #fff;
  background: var(--brand-600, #4f46e5);
}

.tag-item.dragging {
  opacity: 0.4;
}

.tag-item.drag-over {
  position: relative;
}

.tag-item.drag-over::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--brand-500, #6366f1);
  z-index: 1;
}

.tag-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.8);
  flex-shrink: 0;
}

.tag-title {
  line-height: 1;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tag-close {
  font-size: 12px;
  border-radius: 50%;
  padding: 1px;
  transition: background 0.15s;
  flex-shrink: 0;
}

.tag-close:hover {
  background: rgba(255, 255, 255, 0.25);
}

.tag-item:not(.active) .tag-close:hover {
  background: #e0e0e0;
  color: var(--text-primary, #303133);
}

.context-menu {
  position: fixed;
  z-index: 3000;
  list-style: none;
  margin: 0;
  padding: 4px 0;
  background: #fff;
  border: 1px solid var(--border-default, #dcdfe6);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  min-width: 130px;
}

.context-menu li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  font-size: 13px;
  color: var(--text-primary, #303133);
  cursor: pointer;
  transition: background 0.15s;
}

.context-menu li:hover {
  background: var(--bg-surface-hover, #f5f7fa);
  color: var(--brand-500, #6366f1);
}

.context-menu li .el-icon {
  font-size: 14px;
}

@media (max-width: 768px) {
  .tags-view {
    display: none;
  }
}
</style>
