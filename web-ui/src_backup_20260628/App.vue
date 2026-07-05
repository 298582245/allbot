<template>
  <router-view />
</template>

<script setup>
</script>

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
