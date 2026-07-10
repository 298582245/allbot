<template>
  <el-container class="layout-container">
    <el-aside :width="collapsed ? '64px' : '220px'" class="sidebar">
      <div class="logo">
        <svg class="logo-icon" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="32" height="32" rx="8" fill="var(--brand-500)"/>
          <path d="M10 12.5C10 11.1193 11.1193 10 12.5 10H19.5C20.8807 10 22 11.1193 22 12.5V16C22 17.3807 20.8807 18.5 19.5 18.5H17L14 21V18.5H12.5C11.1193 18.5 10 17.3807 10 16V12.5Z" fill="white" fill-opacity="0.9"/>
          <circle cx="14" cy="14.5" r="1.2" fill="var(--brand-600)"/>
          <circle cx="18" cy="14.5" r="1.2" fill="var(--brand-600)"/>
        </svg>
        <h2 v-show="!collapsed">AllBot</h2>
      </div>

      <el-menu
        ref="menuRef"
        :default-active="sidebarActiveMenu"
        :unique-opened="!collapsed"
        :collapse="collapsed"
        :collapse-transition="true"
        class="sidebar-menu"
        @open="handleMenuOpen"
        @select="handleMenuSelect"
      >
        <el-menu-item index="/dashboard">
          <el-icon><DataAnalysis /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>

        <el-sub-menu index="pluginApis">
          <template #title>
            <el-icon><Grid /></el-icon>
            <span>插件中心</span>
          </template>
          <el-menu-item index="/plugins">
            <el-icon><Cpu /></el-icon>
            <span>插件管理</span>
          </el-menu-item>
          <el-menu-item index="/script-envs">
            <el-icon><Setting /></el-icon>
            <span>脚本变量</span>
          </el-menu-item>
          <el-menu-item index="/script-tasks">
            <el-icon><Document /></el-icon>
            <span>脚本任务</span>
          </el-menu-item>
          <el-menu-item index="/open-apis">
            <el-icon><Link /></el-icon>
            <span>开放接口</span>
          </el-menu-item>
          <el-menu-item index="/sdk">
            <el-icon><Document /></el-icon>
            <span>SDK管理</span>
          </el-menu-item>
          <el-menu-item index="/plugin-panels">
            <el-icon><Grid /></el-icon>
            <span class="menu-label-with-count">
              插件面板
              <span v-if="pluginWebPanelsStore.panels.length > 0 && !collapsed" class="menu-count-badge">
                {{ pluginWebPanelsStore.panels.length > 99 ? '99+' : pluginWebPanelsStore.panels.length }}
              </span>
            </span>
          </el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="messageTasks">
          <template #title>
            <el-icon><ChatDotRound /></el-icon>
            <span>回复配置</span>
          </template>
          <el-menu-item index="/replies/keywords">
            <el-icon><ChatLineRound /></el-icon>
            <span>关键字回复</span>
          </el-menu-item>
          <el-menu-item index="/scheduled-tasks">
            <el-icon><Timer /></el-icon>
            <span>定时任务</span>
          </el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="dataBackups">
          <template #title>
            <el-icon><Coin /></el-icon>
            <span>数据中心</span>
          </template>
          <el-menu-item index="/data">
            <el-icon><Coin /></el-icon>
            <span>数据管理</span>
          </el-menu-item>
          <el-menu-item index="/statistics">
            <el-icon><DataAnalysis /></el-icon>
            <span>数据统计</span>
          </el-menu-item>
          <el-menu-item index="/images">
            <el-icon><Picture /></el-icon>
            <span>图床管理</span>
          </el-menu-item>
          <el-menu-item index="/backups">
            <el-icon><FolderChecked /></el-icon>
            <span>备份中心</span>
          </el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="platformAuth">
          <template #title>
            <el-icon><Connection /></el-icon>
            <span>平台配置</span>
          </template>
          <el-menu-item index="/adapters">
            <el-icon><Connection /></el-icon>
            <span>对接管理</span>
          </el-menu-item>
          <el-menu-item index="/chat">
            <el-icon><ChatDotRound /></el-icon>
            <span>Web 聊天</span>
          </el-menu-item>
          <el-menu-item index="/permissions">
            <el-icon><Lock /></el-icon>
            <span>权限控制</span>
          </el-menu-item>
          <el-menu-item index="/users">
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="payments">
          <template #title>
            <el-icon><Money /></el-icon>
            <span>支付管理</span>
          </template>
          <el-menu-item index="/payments/config">
            <el-icon><Setting /></el-icon>
            <span>支付配置</span>
          </el-menu-item>
          <el-menu-item index="/payments/orders">
            <el-icon><Tickets /></el-icon>
            <span>订单管理</span>
          </el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="operations">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>系统配置</span>
          </template>
          <el-menu-item index="/runtime-profiles">
            <el-icon><Setting /></el-icon>
            <span>运行环境</span>
          </el-menu-item>
          <el-menu-item index="/dependencies">
            <el-icon><Box /></el-icon>
            <span>依赖管理</span>
          </el-menu-item>
          <el-menu-item index="/logs">
            <el-icon><Document /></el-icon>
            <span>日志查看</span>
          </el-menu-item>
          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-button class="sidebar-toggle" text @click="collapsed = !collapsed">
            <el-icon><Fold v-if="!collapsed" /><Expand v-else /></el-icon>
          </el-button>
          <h3>{{ currentTitle }}</h3>
        </div>
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

      <TagsView />

      <el-main class="main-content">
        <router-view v-slot="{ Component }">
          <transition name="page" mode="out-in">
            <keep-alive :include="tabsStore.cachedNames">
              <component :is="Component" :key="route.fullPath" />
            </keep-alive>
          </transition>
        </router-view>
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
import { computed, ref, watch, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessageBox } from "element-plus";
import {
  DataAnalysis,
  Grid,
  Connection,
  Document,
  Setting,
  User,
  SwitchButton,
  Cpu,
  Coin,
  Box,
  ChatDotRound,
  ChatLineRound,
  Lock,
  Timer,
  Link,
  Money,
  Tickets,
  FolderChecked,
  Picture,
  Fold,
  Expand,
} from "@element-plus/icons-vue";
import { useAuthStore } from "@/stores/auth";
import { useTabsStore } from "@/stores/tabs";
import { usePluginWebPanelsStore } from "@/stores/pluginWebPanels";
import TagsView from "@/components/TagsView.vue";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const tabsStore = useTabsStore();
const pluginWebPanelsStore = usePluginWebPanelsStore();
const moreDrawerVisible = ref(false);
const menuRef = ref(null);
const collapsed = ref(false);

tabsStore.initAffixTabs();
pluginWebPanelsStore.loadPanels();
watch(
  () => route.path,
  () => {
    if (route.meta.title) {
      tabsStore.addTab({
        path: route.path,
        title: route.meta.title,
        name: route.name,
      });
    }
  },
  { immediate: true }
);
const submenuKeys = [
  "pluginApis",
  "messageTasks",
  "dataBackups",
  "platformAuth",
  "payments",
  "operations",
];
const topLevelMenuIndexes = ["/dashboard"];
const activeMenu = computed(() => {
  if (route.path.startsWith("/open-apis")) return "/open-apis";
  if (route.path.startsWith("/plugin-panels")) return "/plugin-panels";
  if (route.path === "/payments") return "/payments/config";
  return route.path;
});
const pluginPanelNavItems = computed(() =>
  pluginWebPanelsStore.panels.map((panel) => ({
    ...panel,
    path: `/plugin-panels/${encodeURIComponent(panel.plugin_id)}`,
    icon: Grid,
  }))
);
const sidebarActiveMenu = computed(() => activeMenu.value);
const currentTitle = computed(() => {
  if (route.name === "PluginPanel") {
    return pluginPanelNavItems.value.find((item) => item.plugin_id === route.params.pluginId)?.title || "插件面板";
  }
  return route.meta.title || "AllBot";
});
const primaryMobileNavItems = [
  { path: "/dashboard", title: "仪表盘", icon: DataAnalysis },
  { path: "/plugins", title: "插件", icon: Cpu },
  { path: "/adapters", title: "平台", icon: Connection },
  { path: "/settings", title: "设置", icon: Setting },
];
const baseMoreMobileNavItems = [
  { path: "/open-apis", title: "开放接口", icon: Link },
  { path: "/sdk", title: "SDK管理", icon: Document },
  { path: "/replies/keywords", title: "关键字回复", icon: ChatDotRound },
  { path: "/scheduled-tasks", title: "定时任务", icon: Timer },
  { path: "/script-tasks", title: "脚本任务", icon: Document },
  { path: "/script-envs", title: "脚本变量", icon: Setting },
  { path: "/plugin-panels", title: "插件面板", icon: Grid },
  { path: "/data", title: "数据管理", icon: Coin },
  { path: "/statistics", title: "数据统计", icon: DataAnalysis },
  { path: "/images", title: "图床管理", icon: Picture },
  { path: "/backups", title: "备份中心", icon: FolderChecked },
  { path: "/permissions", title: "权限控制", icon: Lock },
  { path: "/users", title: "用户管理", icon: User },
  { path: "/payments/config", title: "支付配置", icon: Money },
  { path: "/payments/orders", title: "订单管理", icon: Tickets },
  { path: "/dependencies", title: "依赖管理", icon: Box },
  { path: "/runtime-profiles", title: "运行环境", icon: Setting },
  { path: "/logs", title: "日志查看", icon: Document },
];
const moreMobileNavItems = computed(() => baseMoreMobileNavItems);
const isMoreActive = computed(() =>
  moreMobileNavItems.value.some((item) => activeMenu.value === item.path)
);

const handleMenuOpen = (index) => {
  submenuKeys
    .filter((key) => key !== index)
    .forEach((key) => menuRef.value?.close(key));
};

const handleMenuSelect = (index) => {
  if (index === "/chat") {
    window.open(router.resolve({ path: "/chat" }).href, "_blank", "noopener");
    return;
  }
  if (topLevelMenuIndexes.includes(index)) {
    submenuKeys.forEach((key) => menuRef.value?.close(key));
  }
  if (index.startsWith("/") && index !== route.path) {
    router.push(index);
  }
};

const handleCommand = async (command) => {
  if (command === "logout") {
    await ElMessageBox.confirm("确定要退出登录吗？", "提示", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      type: "warning",
    });
    await authStore.logout();
  }
};
</script>

<style scoped>
.layout-container {
  height: 100vh;
  overflow: hidden;
  min-height: 0;
}
.sidebar {
  background: var(--bg-sidebar-gradient);
  color: var(--text-on-dark);
  border-right: 1px solid var(--border-on-dark);
  transition: width 0.3s ease;
  overflow-x: hidden;
}
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border-bottom: 1px solid var(--border-on-dark);
}
.logo-icon {
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  filter: drop-shadow(0 2px 8px rgba(99, 102, 241, 0.3));
}
.logo h2 {
  font-size: 18px;
  font-weight: 700;
  font-family: var(--font-heading);
  color: var(--text-on-dark);
  margin: 0;
  letter-spacing: -0.01em;
}
.sidebar-menu {
  border: none;
  background: transparent;
}
.sidebar-menu :deep(.el-menu),
.sidebar-menu :deep(.el-sub-menu .el-menu) {
  background: transparent;
}
.sidebar-menu :deep(.el-menu-item),
.sidebar-menu :deep(.el-sub-menu__title) {
  color: var(--text-on-dark-muted);
  background: transparent;
  transition: all var(--transition-normal);
  position: relative;
}
.sidebar-menu :deep(.el-menu-item:hover),
.sidebar-menu :deep(.el-sub-menu__title:hover) {
  color: var(--text-on-dark);
  background: var(--bg-sidebar-hover);
}
.sidebar-menu :deep(.el-menu-item.is-active) {
  color: #fff;
  background: var(--bg-sidebar-active);
  box-shadow: 0 4px 12px var(--bg-sidebar-active-glow);
}
.sidebar-menu :deep(.el-menu-item.is-active)::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 60%;
  background: var(--brand-300);
  border-radius: 0 3px 3px 0;
}
.menu-label-with-count {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.menu-count-badge {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  line-height: 18px;
  color: #fff;
  background: #f56c6c;
  box-shadow: 0 2px 6px rgba(245, 108, 108, 0.35);
}
.sidebar-menu :deep(.el-menu-item.is-active) .menu-count-badge {
  background: #ff4d4f;
  box-shadow: 0 2px 8px rgba(255, 77, 79, 0.45);
}
.sidebar-menu :deep(.el-sub-menu.is-opened > .el-sub-menu__title),
.sidebar-menu :deep(.el-sub-menu.is-active > .el-sub-menu__title) {
  color: var(--text-on-dark);
  background: var(--bg-sidebar-submenu);
}
.header {
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.header-left h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  font-family: var(--font-heading);
  color: var(--text-primary);
}
.sidebar-toggle {
  padding: 8px;
  font-size: 18px;
  color: var(--text-secondary);
  flex-shrink: 0;
}
.sidebar-toggle:hover {
  color: var(--brand-500);
  background: var(--bg-surface-hover);
}
.header-right {
  display: flex;
  align-items: center;
}
.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  transition: background var(--transition-normal);
  color: var(--text-secondary);
}
.user-info:hover {
  background: var(--bg-surface-hover);
}
.main-content {
  background:
    radial-gradient(ellipse at top, rgba(99, 102, 241, 0.025), transparent 50%),
    var(--bg-base);
  padding: 20px 20px 36px;
  overflow-x: hidden;
  overflow-y: auto;
  min-height: 0;
}
.mobile-tabbar {
  display: none;
}

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

  .sidebar-toggle {
    display: none;
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
    background: var(--bg-sidebar);
    border-top: 1px solid var(--border-on-dark);
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
    color: var(--text-on-dark-muted);
    text-decoration: none;
    border: none;
    border-radius: var(--radius-md);
    background: transparent;
    font: inherit;
    font-size: 12px;
    transition: all var(--transition-normal);
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
    background: var(--brand-500);
    box-shadow: 0 2px 8px var(--bg-sidebar-active-glow);
  }

  .mobile-more-drawer :deep(.el-drawer) {
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
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
    color: var(--text-secondary);
    text-decoration: none;
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    font-size: 12px;
    transition: all var(--transition-normal);
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
    color: var(--brand-500);
    border-color: var(--border-brand);
    background: var(--brand-50);
  }
}

@media (max-width: 380px) {
  .mobile-more-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

<style>
/* Collapsed sidebar sub-menu popup — teleported outside component, needs global CSS */
.el-popper.el-menu--popup {
  background: var(--bg-sidebar-gradient);
  border: 1px solid var(--border-on-dark);
}
.el-popper.el-menu--popup .el-menu-item,
.el-popper.el-menu--popup .el-sub-menu__title {
  color: var(--text-on-dark-muted);
  background: transparent;
}
.el-popper.el-menu--popup .el-menu-item:hover,
.el-popper.el-menu--popup .el-sub-menu__title:hover {
  color: var(--text-on-dark);
  background: var(--bg-sidebar-hover);
}
.el-popper.el-menu--popup .el-menu-item.is-active {
  color: #fff;
  background: var(--bg-sidebar-active);
}
</style>
