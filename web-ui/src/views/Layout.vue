<template>
  <el-container class="layout-container">
    <el-aside width="220px" class="sidebar">
      <div class="logo"><h2>AllBot</h2></div>

      <el-menu
        ref="menuRef"
        :default-active="sidebarActiveMenu"
        :unique-opened="true"
        router
        class="sidebar-menu"
        @open="handleMenuOpen"
        @select="handleMenuSelect"
      >
        <el-menu-item index="/dashboard">
          <el-icon><DataAnalysis /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>

        <el-sub-menu index="features">
          <template #title>
            <el-icon><Grid /></el-icon>
            <span>功能管理</span>
          </template>
          <el-menu-item index="/plugins">
            <el-icon><Cpu /></el-icon>
            <span>插件管理</span>
          </el-menu-item>
          <el-menu-item index="/open-apis">
            <el-icon><Link /></el-icon>
            <span>开放接口</span>
          </el-menu-item>
          <el-menu-item index="/sdk">
            <el-icon><Document /></el-icon>
            <span>SDK管理</span>
          </el-menu-item>
          <el-menu-item index="/data">
            <el-icon><Coin /></el-icon>
            <span>数据管理</span>
          </el-menu-item>
          <el-menu-item index="/dependencies">
            <el-icon><Box /></el-icon>
            <span>依赖管理</span>
          </el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="replies">
          <template #title>
            <el-icon><ChatDotRound /></el-icon>
            <span>回复设置</span>
          </template>
          <el-menu-item index="/replies/keywords">
            <el-icon><ChatLineRound /></el-icon>
            <span>关键字回复</span>
          </el-menu-item>
          <el-menu-item index="/scheduled-tasks">
            <el-icon><Timer /></el-icon>
            <span>定时任务</span>
          </el-menu-item>
          <el-menu-item index="/script-tasks">
            <el-icon><Document /></el-icon>
            <span>脚本任务</span>
          </el-menu-item>
        </el-sub-menu>

        <el-menu-item index="/adapters">
          <el-icon><Connection /></el-icon>
          <span>平台配置</span>
        </el-menu-item>

        <el-menu-item index="/logs">
          <el-icon><Document /></el-icon>
          <span>日志查看</span>
        </el-menu-item>

        <el-menu-item index="/permissions">
          <el-icon><Lock /></el-icon>
          <span>权限控制</span>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <span>系统设置</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-left"><h3>{{ currentTitle }}</h3></div>
        <div class="header-right">
          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-icon><User /></el-icon>
              {{ authStore.username }}
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>

    <nav class="mobile-tabbar">
      <router-link
        v-for="item in primaryMobileNavItems"
        :key="item.path"
        :to="item.path"
        class="mobile-tabbar-item"
        :class="{ active: activeMenu === item.path }"
      >
        <el-icon><component :is="item.icon" /></el-icon>
        <span>{{ item.title }}</span>
      </router-link>
      <button
        type="button"
        class="mobile-tabbar-item mobile-more-button"
        :class="{ active: isMoreActive }"
        @click="moreDrawerVisible = true"
      >
        <el-icon><Grid /></el-icon>
        <span>更多</span>
      </button>
    </nav>

    <el-drawer
      v-model="moreDrawerVisible"
      direction="btt"
      size="58%"
      class="mobile-more-drawer"
      title="更多功能"
    >
      <div class="mobile-more-grid">
        <router-link
          v-for="item in moreMobileNavItems"
          :key="item.path"
          :to="item.path"
          class="mobile-more-item"
          :class="{ active: activeMenu === item.path }"
          @click="moreDrawerVisible = false"
        >
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.title }}</span>
        </router-link>
      </div>
    </el-drawer>
  </el-container>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { DataAnalysis, Grid, Connection, Document, Setting, User, SwitchButton, Cpu, Coin, Box, ChatDotRound, ChatLineRound, Lock, Timer, Link } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const authStore = useAuthStore()
const moreDrawerVisible = ref(false)
const menuRef = ref(null)
const openedSubmenu = ref('')
const submenuKeys = ['features', 'replies']
const activeMenu = computed(() => route.path.startsWith('/open-apis') ? '/open-apis' : route.path)
const sidebarActiveMenu = computed(() => openedSubmenu.value || activeMenu.value)
const currentTitle = computed(() => route.meta.title || 'AllBot')
const primaryMobileNavItems = [
  { path: '/dashboard', title: '仪表盘', icon: DataAnalysis },
  { path: '/plugins', title: '插件', icon: Cpu },
  { path: '/adapters', title: '平台', icon: Connection },
  { path: '/settings', title: '设置', icon: Setting }
]
const moreMobileNavItems = [
  { path: '/open-apis', title: '开放接口', icon: Link },
  { path: '/sdk', title: 'SDK管理', icon: Document },
  { path: '/data', title: '数据管理', icon: Coin },
  { path: '/dependencies', title: '依赖管理', icon: Box },
  { path: '/replies/keywords', title: '关键字回复', icon: ChatDotRound },
  { path: '/scheduled-tasks', title: '定时任务', icon: Timer },
  { path: '/script-tasks', title: '脚本任务', icon: Document },
  { path: '/logs', title: '日志查看', icon: Document },
  { path: '/permissions', title: '权限控制', icon: Lock }
]
const isMoreActive = computed(() => moreMobileNavItems.some((item) => activeMenu.value === item.path))

const handleMenuOpen = (index) => {
  openedSubmenu.value = index
  submenuKeys.filter((key) => key !== index).forEach((key) => menuRef.value?.close(key))
}

const handleMenuSelect = (index) => {
  openedSubmenu.value = ''
  if (!submenuKeys.includes(index)) {
    submenuKeys.forEach((key) => menuRef.value?.close(key))
  }
}

const handleCommand = async (command) => {
  if (command === 'logout') {
    await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    authStore.logout()
  }
}
</script>

<style scoped>
.layout-container { height: 100vh; overflow: hidden; }
.sidebar { background: #001529; color: white; }
.logo { height: 60px; display: flex; align-items: center; justify-content: center; border-bottom: 1px solid rgba(255,255,255,0.1); }
.logo h2 { font-size: 20px; color: white; margin: 0; }
.sidebar-menu { border: none; background: #001529; }
.sidebar-menu :deep(.el-menu), .sidebar-menu :deep(.el-sub-menu .el-menu) { background: #001529; }
.sidebar-menu :deep(.el-menu-item), .sidebar-menu :deep(.el-sub-menu__title) { color: rgba(255,255,255,0.65); background: #001529; }
.sidebar-menu :deep(.el-menu-item:hover), .sidebar-menu :deep(.el-sub-menu__title:hover) { color: white; background: rgba(255,255,255,0.1); }
.sidebar-menu :deep(.el-menu-item.is-active) { color: white; background: #1890ff; }
.sidebar-menu :deep(.el-sub-menu.is-opened > .el-sub-menu__title),
.sidebar-menu :deep(.el-sub-menu.is-active > .el-sub-menu__title) { color: white; background: #1890ff; }
.header { background: white; border-bottom: 1px solid #f0f0f0; display: flex; align-items: center; justify-content: space-between; padding: 0 20px; }
.header-left h3 { margin: 0; font-size: 18px; color: #333; }
.header-right { display: flex; align-items: center; }
.user-info { display: flex; align-items: center; gap: 8px; cursor: pointer; padding: 8px 12px; border-radius: 4px; transition: background 0.3s; }
.user-info:hover { background: #f5f5f5; }
.main-content { background: #f0f2f5; padding: 20px 20px 36px; overflow: hidden; }
.mobile-tabbar { display: none; }

@media (max-width: 768px) {
  .layout-container {
    height: 100dvh;
  }

  .sidebar {
    display: none;
  }

  .header {
    height: 52px;
    padding: 0 12px;
  }

  .header-left h3 {
    font-size: 16px;
  }

  .user-info {
    padding: 6px 8px;
    font-size: 13px;
  }

  .main-content {
    padding: 12px;
    padding-bottom: 76px;
  }

  .mobile-tabbar {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 2000;
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 4px;
    padding: 6px 8px calc(6px + env(safe-area-inset-bottom));
    background: #001529;
    border-top: 1px solid rgba(255, 255, 255, 0.12);
    box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.16);
  }

  .mobile-tabbar-item {
    width: 100%;
    min-width: 0;
    height: 52px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3px;
    color: rgba(255, 255, 255, 0.68);
    text-decoration: none;
    border: none;
    border-radius: 10px;
    background: transparent;
    font: inherit;
    font-size: 12px;
  }

  .mobile-tabbar-item .el-icon {
    font-size: 18px;
  }

  .mobile-more-button {
    cursor: pointer;
  }

  .mobile-tabbar-item span {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-tabbar-item.active {
    color: #fff;
    background: #1890ff;
  }

  .mobile-more-drawer :deep(.el-drawer) {
    border-radius: 18px 18px 0 0;
  }

  .mobile-more-drawer :deep(.el-drawer__header) {
    margin-bottom: 0;
    padding: 16px 18px 8px;
    font-weight: 600;
  }

  .mobile-more-drawer :deep(.el-drawer__body) {
    padding: 12px 16px calc(18px + env(safe-area-inset-bottom));
  }

  .mobile-more-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
  }

  .mobile-more-item {
    min-width: 0;
    height: 74px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: #606266;
    text-decoration: none;
    border: 1px solid #ebeef5;
    border-radius: 14px;
    background: #f8fafc;
    font-size: 12px;
  }

  .mobile-more-item .el-icon {
    font-size: 20px;
  }

  .mobile-more-item span {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-more-item.active {
    color: #1890ff;
    border-color: rgba(24, 144, 255, 0.35);
    background: #ecf5ff;
  }
}

@media (max-width: 380px) {
  .mobile-more-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

</style>
