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
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
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
