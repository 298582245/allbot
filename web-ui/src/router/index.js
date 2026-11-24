import { createRouter, createWebHistory } from 'vue-router'
import { defineAsyncComponent, defineComponent, h } from 'vue'
import { useAuthStore } from '@/stores/auth'
import RouteChunkState from '@/components/RouteChunkState.vue'

function lazyRoute(name, loader) {
  const page = defineAsyncComponent({
    loader,
    loadingComponent: RouteChunkState,
    errorComponent: RouteChunkState,
    delay: 0,
    timeout: 30000,
    onError(error, retry, fail, attempts) {
      if (attempts < 2) retry()
      else fail()
    }
  })

  return defineComponent({
    name,
    setup() {
      return () => h(page)
    }
  })
}

const routes = [
  {
    path: '/chat',
    name: 'WebChat',
    component: () => import('@/views/WebChat.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', name: 'Dashboard', component: lazyRoute('Dashboard', () => import('@/views/Dashboard.vue')), meta: { title: '仪表盘' } },
      { path: 'plugins', name: 'Plugins', component: lazyRoute('Plugins', () => import('@/views/Plugins.vue')), meta: { title: '插件管理' } },
      { path: 'open-apis', name: 'OpenApis', component: lazyRoute('OpenApis', () => import('@/views/OpenApis.vue')), meta: { title: '开放接口' } },
      { path: 'open-apis/:id/edit', name: 'OpenApiEditor', component: lazyRoute('OpenApiEditor', () => import('@/views/OpenApiEditor.vue')), meta: { title: '编辑开放接口' } },
      { path: 'sdk', name: 'SdkManager', component: lazyRoute('SdkManager', () => import('@/views/SdkManager.vue')), meta: { title: 'SDK管理' } },
      { path: 'data', name: 'DataManager', component: lazyRoute('DataManager', () => import('@/views/DataManager.vue')), meta: { title: '数据管理' } },
      { path: 'statistics', name: 'Statistics', component: lazyRoute('Statistics', () => import('@/views/Statistics.vue')), meta: { title: '数据统计' } },
      { path: 'images', name: 'ImageManager', component: lazyRoute('ImageManager', () => import('@/views/ImageManager.vue')), meta: { title: '图床管理' } },
      { path: 'backups', name: 'Backups', component: lazyRoute('Backups', () => import('@/views/Backups.vue')), meta: { title: '备份中心' } },
      { path: 'dependencies', name: 'Dependencies', component: lazyRoute('Dependencies', () => import('@/views/Dependencies.vue')), meta: { title: '依赖管理' } },
      { path: 'runtime-profiles', name: 'RuntimeProfiles', component: lazyRoute('RuntimeProfiles', () => import('@/views/RuntimeProfiles.vue')), meta: { title: '运行环境' } },
      { path: 'scheduled-tasks', name: 'ScheduledTasks', component: lazyRoute('ScheduledTasks', () => import('@/views/ScheduledTasks.vue')), meta: { title: '定时任务' } },
      { path: 'script-tasks', name: 'ScriptTasks', component: lazyRoute('ScriptTasks', () => import('@/views/ScriptTasks.vue')), meta: { title: '脚本任务' } },
      { path: 'plugin-tasks', name: 'PluginTasks', component: lazyRoute('PluginTasks', () => import('@/views/PluginTasks.vue')), meta: { title: '插件任务' } },
      { path: 'script-envs', name: 'ScriptEnvs', component: lazyRoute('ScriptEnvs', () => import('@/views/ScriptEnvs.vue')), meta: { title: '脚本环境变量' } },
      { path: 'replies/keywords', name: 'KeywordReplies', component: lazyRoute('KeywordReplies', () => import('@/views/KeywordReplies.vue')), meta: { title: '关键字回复' } },
      { path: 'adapters', name: 'Adapters', component: lazyRoute('Adapters', () => import('@/views/Adapters.vue')), meta: { title: '对接管理' } },
      { path: 'adapter-management', name: 'AdapterManagement', component: lazyRoute('AdapterManagement', () => import('@/views/AdapterManagement.vue')), meta: { title: '适配器管理' } },
      { path: 'logs', name: 'Logs', component: lazyRoute('Logs', () => import('@/views/Logs.vue')), meta: { title: '日志查看' } },
      { path: 'permissions', name: 'PermissionControl', component: lazyRoute('PermissionControl', () => import('@/views/PermissionControl.vue')), meta: { title: '权限控制' } },
      { path: 'users', name: 'Users', component: lazyRoute('Users', () => import('@/views/Users.vue')), meta: { title: '用户管理' } },
      { path: 'payments', redirect: '/payments/config' },
      { path: 'payments/config', name: 'PaymentConfig', component: lazyRoute('PaymentConfig', () => import('@/views/PaymentConfig.vue')), meta: { title: '支付配置' } },
      { path: 'payments/orders', name: 'PaymentOrders', component: lazyRoute('PaymentOrders', () => import('@/views/PaymentOrders.vue')), meta: { title: '订单管理' } },
      { path: 'settings', name: 'Settings', component: lazyRoute('Settings', () => import('@/views/Settings.vue')), meta: { title: '系统设置' } },
      { path: 'developer-apps', name: 'DeveloperApps', component: lazyRoute('DeveloperApps', () => import('@/views/DeveloperApps.vue')), meta: { title: '开发者应用' } },
      { path: 'plugin-panels', name: 'PluginPanels', component: lazyRoute('PluginPanels', () => import('@/views/PluginPanels.vue')), meta: { title: '插件面板' } },
      { path: 'plugin-panels/:pluginId', name: 'PluginPanel', component: lazyRoute('PluginPanel', () => import('@/views/PluginPanel.vue')), meta: { title: '插件面板' } },
      { path: 'plugins/:id/edit', name: 'PluginEditor', component: lazyRoute('PluginEditor', () => import('@/views/PluginEditor.vue')), meta: { title: '编辑插件代码' } }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes
})

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()

  if (to.meta.requiresAuth && !authStore.isAuthenticated) next('/login')
  else if (to.path === '/login' && authStore.isAuthenticated) next('/')
  else next()
})

export default router
