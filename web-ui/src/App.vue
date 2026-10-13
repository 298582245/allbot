<template>
  <router-view />
  <div v-if="isDemoMode" class="demo-banner">AllBot 在线演示 · 使用浏览器模拟数据，刷新后恢复初始状态</div>
</template>

<script setup>
import { isDemoMode } from '@/demo/mockApi'
</script>

<style>
.demo-banner {
  position: fixed;
  right: 18px;
  bottom: 18px;
  z-index: 3000;
  max-width: calc(100vw - 36px);
  padding: 9px 13px;
  border: 1px solid rgba(37, 99, 235, 0.22);
  border-radius: 8px;
  background: rgba(239, 246, 255, 0.96);
  color: #1d4ed8;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
  font-size: 12px;
  line-height: 1.4;
  pointer-events: none;
}

@media (max-width: 600px) {
  .demo-banner {
    right: 10px;
    bottom: 10px;
    max-width: calc(100vw - 20px);
  }
}
</style>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: var(--font-body);
  color: var(--text-primary);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

h1, h2, h3, h4, h5, h6 {
  font-family: var(--font-heading);
}

code, pre, .mono-text {
  font-family: var(--font-mono);
}

html,
body {
  width: 100%;
  height: 100%;
  overflow: hidden;
}

#app {
  width: 100%;
  height: 100vh;
}

/* ---- Scrollbar Polish ---- */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: var(--border-strong);
  border-radius: var(--radius-full);
}
::-webkit-scrollbar-thumb:hover {
  background: var(--text-tertiary);
}

@media (max-width: 768px) {
  #app {
    height: 100dvh;
  }

  .el-dialog {
    width: calc(100vw - 24px) !important;
    max-width: calc(100vw - 24px);
    margin-top: 8vh !important;
  }

  .el-dialog__body {
    max-height: 68vh;
    overflow: auto;
  }

  .el-pagination {
    flex-wrap: nowrap;
    min-width: max-content;
  }

  .el-table {
    font-size: 12px;
  }

  .el-card__header {
    padding: 14px;
  }

  .el-card__body {
    padding: 14px;
  }
}
</style>
