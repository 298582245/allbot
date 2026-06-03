import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
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
      { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '仪表盘' } },
      { path: 'plugins', name: 'Plugins', component: () => import('@/views/Plugins.vue'), meta: { title: '插件管理' } },
      { path: 'open-apis', name: 'OpenApis', component: () => import('@/views/OpenApis.vue'), meta: { title: '开放接口' } },
      { path: 'open-apis/:id/edit', name: 'OpenApiEditor', component: () => import('@/views/OpenApiEditor.vue'), meta: { title: '编辑开放接口' } },
      { path: 'sdk', name: 'SdkManager', component: () => import('@/views/SdkManager.vue'), meta: { title: 'SDK管理' } },
      { path: 'data', name: 'DataManager', component: () => import('@/views/DataManager.vue'), meta: { title: '数据管理' } },
      { path: 'dependencies', name: 'Dependencies', component: () => import('@/views/Dependencies.vue'), meta: { title: '依赖管理' } },
      { path: 'scheduled-tasks', name: 'ScheduledTasks', component: () => import('@/views/ScheduledTasks.vue'), meta: { title: '定时任务' } },
      { path: 'script-tasks', name: 'ScriptTasks', component: () => import('@/views/ScriptTasks.vue'), meta: { title: '脚本任务' } },
      { path: 'replies/keywords', name: 'KeywordReplies', component: () => import('@/views/KeywordReplies.vue'), meta: { title: '关键字回复' } },
      { path: 'adapters', name: 'Adapters', component: () => import('@/views/Adapters.vue'), meta: { title: '平台配置' } },
      { path: 'logs', name: 'Logs', component: () => import('@/views/Logs.vue'), meta: { title: '日志查看' } },
      { path: 'permissions', name: 'PermissionControl', component: () => import('@/views/PermissionControl.vue'), meta: { title: '权限控制' } },
      { path: 'settings', name: 'Settings', component: () => import('@/views/Settings.vue'), meta: { title: '系统设置' } },
      { path: 'plugins/:id/edit', name: 'PluginEditor', component: () => import('@/views/PluginEditor.vue'), meta: { title: '编辑插件代码' } }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()

  if (to.meta.requiresAuth && !authStore.isAuthenticated) next('/login')
  else if (to.path === '/login' && authStore.isAuthenticated) next('/')
  else next()
})

export default router
