const demoMode = import.meta.env.VITE_ALLBOT_DEMO === 'true' || import.meta.env.MODE === 'demo'

export const isDemoMode = demoMode

const clone = (value) => {
  if (value === undefined) return undefined
  return JSON.parse(JSON.stringify(value))
}

const now = () => new Date().toISOString()

const demoState = {
  plugins: [
    {
      id: 'echo-helper', name: '回声助手', version: '1.2.0', runtime: 'nodejs', entry: 'main.js',
      trigger: '^回声\\s+(.+)$', priority: 100, platforms: ['qq', 'telegram'], enabled: true, pinned: true,
      description: '一个用于演示消息处理链路的轻量插件。'
    },
    {
      id: 'daily-report', name: '每日播报', version: '2.0.1', runtime: 'python', entry: 'main.py',
      trigger: '^日报$', priority: 80, platforms: ['qq_office'], enabled: true, pinned: false,
      description: '按计划发送每日摘要，演示定时任务和插件配置。'
    },
    {
      id: 'weather-card', name: '天气卡片', version: '1.0.4', runtime: 'nodejs', entry: 'index.js',
      trigger: '^天气(?:\\s+(.+))?$', priority: 60, platforms: ['qq', 'feishu', 'wechat_official'], enabled: false, pinned: false,
      description: '展示多平台插件的配置和状态管理。'
    }
  ],
  adapters: [
    { id: 1, platform: 'qq_office', display_name: 'QQ 官方机器人', remark: '演示群机器人', description: '演示用适配器，不连接 QQ', enabled: true, running: true, pinned: true, config: { app_id: 'demo-app', callback_path: 'callback' } },
    { id: 2, platform: 'telegram', display_name: 'Telegram', remark: '通知机器人', description: '模拟 Telegram 适配器', enabled: true, running: true, pinned: false, config: { token: 'demo-token' } },
    { id: 3, platform: 'feishu', display_name: '飞书', remark: '已停止示例', description: '可在演示中启停和编辑', enabled: false, running: false, pinned: false, config: { app_id: 'demo-feishu' } }
  ],
  environments: [
    { id: 'node-default', name: 'Node.js 默认环境', runtime: 'nodejs', source: 'manual', executable: 'node', version: 'v22.14.0', architecture: 'amd64', enabled: true, default: true, description: '演示运行环境' },
    { id: 'python-default', name: 'Python 默认环境', runtime: 'python', source: 'manual', executable: 'python3', version: '3.12.8', architecture: 'amd64', enabled: true, default: true, description: '演示运行环境' }
  ],
  envs: [
    { id: 1, key: 'DEMO_REGION', value: 'Asia/Shanghai', runtime: 'nodejs', enabled: true, pinned: true, description: '演示环境变量' },
    { id: 2, key: 'REPORT_FORMAT', value: 'markdown', runtime: 'python', enabled: true, pinned: false, description: '日报输出格式' }
  ],
  openApis: [
    { id: 'weather', name: '天气查询', path: 'weather', method: 'GET', runtime: 'nodejs', enabled: true, has_token: true, description: '用于演示开放接口管理', call_stats: { total: 128, success: 124, rejected: 2, failed: 2, last_called_at: now() }, ip_whitelist_mode: 'allow_all', effective_ip_whitelist: ['*'] },
    { id: 'health', name: '健康检查', path: 'health', method: 'GET', runtime: 'builtin', enabled: true, has_token: false, description: '内置健康检查接口', call_stats: { total: 45, success: 45, rejected: 0, failed: 0, last_called_at: now() }, ip_whitelist_mode: 'inherit', effective_ip_whitelist: ['*'] }
  ],
  users: [
    { union_id: 'demo_user_001', points: 860, disabled: false, account_count: 2, last_seen_at: now() },
    { union_id: 'demo_user_002', points: 120, disabled: false, account_count: 1, last_seen_at: now() },
    { union_id: 'demo_blocked', points: 0, disabled: true, account_count: 1, last_seen_at: now() }
  ],
  accounts: [
    { id: 1, platform: 'qq_office', user_id: 'demo_qq_1001', union_id: 'demo_user_001', nickname: '演示用户', enabled: true },
    { id: 2, platform: 'telegram', user_id: 'demo_tg_2002', union_id: 'demo_user_001', nickname: 'Demo Telegram', enabled: true },
    { id: 3, platform: 'qq_office', user_id: 'demo_qq_1002', union_id: 'demo_user_002', nickname: '测试成员', enabled: true }
  ],
  scheduledTasks: [
    { id: 1, name: '每日播报', task_key: 'daily-report', description: '工作日早间播报', plugin_id: 'daily-report', platform: 'qq_office', adapter_id: 1, user_id: 'demo_qq_1001', cron: '0 9 * * 1-5', enabled: true, source: 'plugin' },
    { id: 2, name: '健康提醒', task_key: 'health-check', description: '每小时发送一次', plugin_id: 'echo-helper', platform: 'telegram', adapter_id: 2, user_id: 'demo_tg_2002', cron: '0 * * * *', enabled: false, source: 'user' }
  ],
  scriptTasks: [
    { id: 1, plugin_id: 'daily-report', script_path: 'scripts/report.py', run_mode: 'all_authorized', status: 'success', union_id: 'demo_user_001', runtime_profile: 'python-default', run_total: 42, failed_total: 1, started_at: now(), finished_at: now(), output: '演示任务执行完成' }
  ],
  pluginTasks: [
    { id: 1, plugin_id: 'daily-report', platform: 'qq_office', user_id: 'demo_qq_1001', message_id: 'demo-msg-001', status: 'success', run_total: 36, failed_total: 0, started_at: now(), finished_at: now(), output: '插件输出示例' }
  ],
  replies: [
    { id: 1, keyword: 'myid', match_type: 'exact', reply_type: 'builtin', content: 'myid', builtin: 'myid', enabled: true, admin_only: false, pinned: true, description: '查看当前账号身份' },
    { id: 2, keyword: '^帮助$', match_type: 'regex', reply_type: 'text', content: '可用指令：myid、version、天气', builtin: '', enabled: true, admin_only: false, pinned: false, description: '演示关键字回复' },
    { id: 3, keyword: '系统状态', match_type: 'exact', reply_type: 'text', content: 'AllBot 演示环境运行正常', builtin: '', enabled: true, admin_only: true, pinned: false, description: '管理员回复示例' }
  ],
  logs: [
    { id: 1, time: now(), level: 'INFO', service: 'router', message: 'Plugin matched: 回声助手 for message: 回声 你好', repeat: 1 },
    { id: 2, time: new Date(Date.now() - 3600 * 1000).toISOString(), level: 'INFO', service: 'adapter', message: 'QQ 官方机器人已连接', repeat: 1 },
    { id: 3, time: new Date(Date.now() - 2 * 3600 * 1000).toISOString(), level: 'WARN', service: 'plugin', message: '演示任务使用模拟执行器', repeat: 2, lastTime: now() }
  ],
  backups: [
    { name: 'allbot-demo-2026-10-13.zip', size: 184320, created_at: now(), type: 'manual' }
  ],
  images: [],
  tables: [
    { name: 'users', view_name: '用户', group: '用户数据', columns: ['union_id', 'points', 'disabled'] },
    { name: 'keyword_replies', view_name: '关键字回复', group: '回复配置', columns: ['keyword', 'reply_type', 'enabled'] }
  ],
  settings: { access_enabled: false, access_code: '', github_proxy_url: '', public_base_url: '', log_retention_days: 30 },
  logSettings: { retention_days: 30 },
  paymentSettings: { max_payment_amount_cents: 10000, points_per_rmb: 100 },
  update: { current_version: 'v1.0.5-demo', latest_version: 'v1.0.5-demo', release_url: '', url: '', html_url: '' }
}

const platformList = [
  { platform: 'qq', display_name: 'QQ', config_schema: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'app_secret', label: 'App Secret', type: 'password' }] },
  { platform: 'qq_office', display_name: 'QQ 官方机器人', config_schema: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'callback_path', label: '回调路径', type: 'text' }] },
  { platform: 'telegram', display_name: 'Telegram', config_schema: [{ key: 'token', label: 'Bot Token', type: 'password' }] },
  { platform: 'feishu', display_name: '飞书', config_schema: [{ key: 'app_id', label: 'App ID', type: 'text' }] },
  { platform: 'wechat_official', display_name: '微信公众号', config_schema: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'callback_path', label: '回调路径', type: 'text' }] }
]

const webChatPlugins = demoState.plugins.map((plugin) => ({
  id: plugin.id,
  name: plugin.name,
  title: plugin.name,
  description: plugin.description,
  quick_actions: [{ label: '查看帮助', text: '帮助', action: 'fill' }],
  placeholder: `输入 ${plugin.name} 指令`
}))

const webChatMessages = [
  { message_id: 1, direction: 'out', message_type: 'markdown', content: '欢迎进入 AllBot 在线演示。这里的消息由浏览器模拟，不会发送到真实平台。', plugin_id: '', created_at: now() },
  { message_id: 2, direction: 'out', message_type: 'text', content: '可以从左侧选择插件，发送“帮助”查看模拟回复。', plugin_id: '', created_at: now() }
]

function parseData(config) {
  if (config?.data === undefined || config.data === null || config.data === '') return {}
  if (typeof config.data === 'object') return config.data
  try { return JSON.parse(config.data) } catch { return {} }
}

function parseRequest(config) {
  const raw = String(config?.url || '').replace(/^https?:\/\/[^/]+/, '')
  const [pathname, queryString = ''] = raw.split('?')
  const params = new URLSearchParams(queryString)
  if (config?.params && typeof config.params === 'object') {
    Object.entries(config.params).forEach(([key, value]) => {
      if (value !== undefined && value !== null) params.set(key, String(value))
    })
  }
  return { path: pathname.replace(/^\/api/, '').replace(/\/$/, '') || '/', method: String(config?.method || 'get').toLowerCase(), params, data: parseData(config) }
}

function resultItems(items, params) {
  const page = Math.max(1, Number(params.get('page') || 1))
  const pageSize = Math.max(1, Number(params.get('page_size') || params.get('limit') || 20))
  const offset = Number(params.get('offset') || ((page - 1) * pageSize))
  return { items: clone(items.slice(offset, offset + pageSize)), total: items.length, page, page_size: pageSize }
}

function findByID(items, id) {
  return items.find((item) => String(item.id ?? item.union_id ?? item.order_no ?? item.path) === String(id))
}

function systemStatus() {
  return {
    uptimeSeconds: 3 * 24 * 3600 + 4 * 3600 + 18 * 60,
    pluginCount: demoState.plugins.length,
    enabledPluginCount: demoState.plugins.filter((item) => item.enabled).length,
    adapterCount: demoState.adapters.length,
    runningAdapterCount: demoState.adapters.filter((item) => item.running).length,
    messageCount: 4821,
    todayMessageCount: 326,
    cpuUsagePercent: 18.4,
    memoryUsagePercent: 43.2,
    memoryUsedBytes: 4.32 * 1024 * 1024 * 1024,
    memoryTotalBytes: 8 * 1024 * 1024 * 1024,
    allBotCpuUsagePercent: 2.8,
    allBotAverageCpuUsagePercent: 2.1,
    allBotPeakCpuUsagePercent: 7.4,
    allBotHistoricalPeakCpuUsagePercent: 14.2,
    allBotMemoryUsagePercent: 8.7,
    allBotMemoryUsedBytes: 112 * 1024 * 1024,
    allBotMemoryTotalBytes: 8 * 1024 * 1024 * 1024,
    allBotAverageMemoryUsedBytes: 106 * 1024 * 1024,
    allBotPeakMemoryUsedBytes: 138 * 1024 * 1024,
    allBotHistoricalPeakMemoryUsedBytes: 186 * 1024 * 1024,
    allBotHistoricalPeakMemoryUsagePercent: 2.3,
    allBotAverageMemoryUsagePercent: 8.3
  }
}

function messageStats() {
  const hours = [0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22]
  return {
    hours,
    by_platform: [
      { name: 'QQ 官方机器人', total: 220, counts: [2, 1, 0, 4, 12, 18, 25, 34, 41, 38, 28, 17] },
      { name: 'Telegram', total: 76, counts: [0, 0, 2, 3, 4, 8, 10, 12, 11, 9, 8, 9] },
      { name: '飞书', total: 30, counts: [0, 0, 0, 1, 2, 3, 4, 5, 4, 4, 4, 3] }
    ],
    by_adapter: [
      { name: '演示群机器人', total: 220, counts: [2, 1, 0, 4, 12, 18, 25, 34, 41, 38, 28, 17] },
      { name: '通知机器人', total: 76, counts: [0, 0, 2, 3, 4, 8, 10, 12, 11, 9, 8, 9] }
    ]
  }
}

function trend() {
  return { dates: ['2026-10-07', '2026-10-08', '2026-10-09', '2026-10-10', '2026-10-11', '2026-10-12', '2026-10-13'], values: [320, 410, 368, 522, 448, 610, 326] }
}

function handleRequest(request) {
  const { path, method, params, data } = request
  const segments = path.split('/').filter(Boolean)
  const id = decodeURIComponent(segments[segments.length - 1] || '')

  if (path === '/login' && method === 'post') return { token: 'demo-admin-token', csrfToken: 'demo-csrf-token', username: data.username || 'admin' }
  if (path === '/logout') return { ok: true }
  if (path === '/system/status') return systemStatus()
  if (path === '/system/message-stats') return messageStats()
  if (path === '/system/update') return clone(demoState.update)
  if (path === '/system/update/status') return { status: 'idle', message: '演示环境不会执行升级' }
  if (path === '/system/update/upgrade') return { status: 'skipped', message: '演示环境不会执行升级' }
  if (path === '/statistics/overview') return { messages: { total: 4821, today: 326 }, plugins: { total: 3, enabled: 2 }, adapters: { total: 3, running: 2 }, storage: { database: 1024 * 1024 * 3.2, logs: 1024 * 512, plugins: 1024 * 1024 * 6.4 } }
  if (path === '/statistics/message-total-trend') return trend()
  if (path === '/statistics/plugin-trigger-trend') return trend()
  if (path === '/adapter-platforms') return clone(platformList)
  if (path === '/adapters' && method === 'get') return clone(demoState.adapters)
  if (segments[0] === 'adapters' && segments.length === 2) {
    const row = findByID(demoState.adapters, id)
    if (method === 'get') return clone(row || {})
    if (method === 'delete') { demoState.adapters = demoState.adapters.filter((item) => String(item.id) !== id); return { ok: true } }
    if (method === 'post') { if (row && data.action === 'pin') row.pinned = true; if (row && data.action === 'unpin') row.pinned = false; return clone(row || {}) }
  }
  if (path === '/adapters' && method === 'post') {
    const current = data.id ? findByID(demoState.adapters, data.id) : null
    if (current) Object.assign(current, data, { running: Boolean(data.enabled) })
    else demoState.adapters.push({ ...data, id: Math.max(...demoState.adapters.map((item) => item.id), 0) + 1, running: Boolean(data.enabled), pinned: false, config: data.config || {} })
    return clone(current || demoState.adapters[demoState.adapters.length - 1])
  }
  if (path === '/plugins' && method === 'get') return clone(demoState.plugins)
  if (path === '/plugins' && method === 'post') { const plugin = { id: `demo-plugin-${Date.now()}`, name: data.name || '新建演示插件', version: '1.0.0', runtime: data.runtime || 'nodejs', entry: data.entry || 'main.js', trigger: data.trigger || '^演示$', priority: 0, platforms: data.platforms || ['qq'], enabled: false, pinned: false, description: data.description || '演示插件' }; demoState.plugins.push(plugin); return clone(plugin) }
  if (segments[0] === 'plugins' && segments[1] === 'templates' && method === 'get') return [{ id: 'blank', name: '空白插件', runtime: 'nodejs' }, { id: 'account', name: '账号插件', runtime: 'nodejs' }]
  if (segments[0] === 'plugins' && segments[1] === 'files' && segments.length >= 3) {
    if (method === 'get' && params.has('path')) return { path: params.get('path'), editable: true, code: '// Demo plugin source\\nexport default async function main () {}' }
    if (method === 'get') return { entry: 'main.js', tree: [{ path: 'main.js', text: 'main.js', type: 'file' }, { path: 'README.md', text: 'README.md', type: 'file' }] }
    return { ok: true, message: '演示文件已更新' }
  }
  if (segments[0] === 'plugins' && segments[1] === 'config' && segments.length >= 3) {
    const plugin = findByID(demoState.plugins, segments[2]) || demoState.plugins[0]
    if (method === 'put') { Object.assign(plugin, { priority: data.priority, pinned: data.pinned, platforms: data.platforms, runtime_profile: data.runtime_profile }); return { ok: true } }
    return { ...clone(plugin), user_config_schema: [{ key: 'enabled', label: '启用演示配置', type: 'boolean', default: true }], user_config: { enabled: true }, allowed_adapter_ids: [], access_control: { mode: 'all' }, open_api: {}, script_env: {}, web_ui: {}, web_chat: {} }
  }
  if (segments[0] === 'plugins' && segments[1] === 'template-editor') {
    return { id, plugin_id: id, files: [{ path: 'main.js', language: 'javascript', code: '// Demo plugin source\\nexport default async function main () {}' }], template_source: 'generated' }
  }
  if (segments[0] === 'plugins' && segments[1] === 'recycle-bin') return method === 'get' ? { items: [{ name: 'weather-card-2026-10-13.zip', size: 98304, created_at: now() }] } : { ok: true }
  if (segments[0] === 'plugins' && segments.length === 2) {
    const plugin = findByID(demoState.plugins, id)
    if (method === 'post') { if (data.action === 'enable') plugin.enabled = true; if (data.action === 'disable') plugin.enabled = false; if (data.action === 'pin') plugin.pinned = true; if (data.action === 'unpin') plugin.pinned = false; return clone(plugin) }
    if (method === 'delete') { demoState.plugins = demoState.plugins.filter((item) => item.id !== id); return { ok: true } }
  }
  if (path === '/plugin-web/panels') return []
  if (path === '/script-envs' && method === 'get') return resultItems(demoState.envs, params)
  if (path === '/script-envs' && ['post', 'put'].includes(method)) { const item = data.id ? findByID(demoState.envs, data.id) : null; if (item) Object.assign(item, data); else demoState.envs.push({ ...data, id: Math.max(...demoState.envs.map((item) => item.id), 0) + 1 }); return clone(item || demoState.envs[demoState.envs.length - 1]) }
  if (segments[0] === 'script-envs' && segments.length === 2 && method === 'delete') { demoState.envs = demoState.envs.filter((item) => String(item.id) !== id); return { ok: true } }
  if (path === '/script-envs' && method === 'patch') return { ok: true }
  if (path === '/open-apis' && method === 'get') return clone(demoState.openApis)
  if (path === '/open-apis' && ['post', 'put'].includes(method)) { const item = data.id ? findByID(demoState.openApis, data.id) : null; if (item) Object.assign(item, data); else demoState.openApis.push({ ...data, id: data.path || `api-${Date.now()}`, call_stats: { total: 0, success: 0, rejected: 0, failed: 0 } }); return clone(item || demoState.openApis[demoState.openApis.length - 1]) }
  if (path === '/open-apis/settings') return { ip_whitelist: ['*'], trusted_proxies: [], call_log_retention_days: 30 }
  if (segments[0] === 'open-apis' && segments.length === 2 && method === 'get') return clone(findByID(demoState.openApis, id) || {})
  if (segments[0] === 'open-apis' && segments[2] === 'calls') return { items: [], total: 0, retention_days: 30, summary: { total: 0, success: 0, rejected: 0, failed: 0 } }
  if (segments[0] === 'open-apis' && segments[2] === 'code') return { code: '// Demo open API source\\nexport default async function handler () { return { ok: true } }' }
  if (path === '/runtime-profiles' && method === 'get') return clone(demoState.environments)
  if (path === '/runtime-profiles' && method === 'put') { demoState.environments = Array.isArray(data.profiles) ? data.profiles : []; return clone(demoState.environments) }
  if (path === '/runtime-profiles/status') return demoState.environments.map((item) => ({ profile_id: item.id, status: 'ready', version: item.version, message: '运行环境可用' }))
  if (path === '/runtime-profiles/download-settings') return { enabled: false, mirror_url: '' }
  if (path === '/runtime-profiles/download-candidates') return { candidates: [{ version: '22.14.0', label: 'Node.js 22.14.0', source: 'demo' }], warnings: [] }
  if (segments[0] === 'runtime-profiles' && segments[1] === 'init') return { id: `demo-job-${Date.now()}`, profile_id: data.profile_id || id, status: 'completed', result: { message: '演示环境初始化完成' } }
  if (path === '/runtime-profiles/test') return { ok: true, version_output: 'demo runtime ready' }
  if (path === '/users' && method === 'get') return resultItems(demoState.users, params)
  if (segments[0] === 'users' && segments.length === 2 && method === 'get') return { ...clone(findByID(demoState.users, id) || {}), accounts: clone(demoState.accounts.filter((item) => item.union_id === id)) }
  if (segments[0] === 'users' && segments[2] === 'point-transactions') return { items: [{ id: 1, delta: 100, balance: 860, description: '演示充值', created_at: now() }], total: 1 }
  if (segments[0] === 'users' && segments[2] === 'status') { const user = findByID(demoState.users, id); if (user) user.disabled = Boolean(data.disabled); return clone(user || {}) }
  if (segments[0] === 'users' && segments[2] === 'points') { const user = findByID(demoState.users, id); if (user) user.points += Number(data.delta) || 0; return clone(user || {}) }
  if (path === '/user-accounts') return resultItems(demoState.accounts, params)
  if (path === '/logs' && method === 'get') return { items: clone(demoState.logs), dates: [new Date().toISOString().slice(0, 10)], total: demoState.logs.length, page: 1, page_size: 50, date: new Date().toISOString().slice(0, 10), retention_days: demoState.logSettings.retention_days }
  if (path === '/logs/settings' && method === 'get') return clone(demoState.logSettings)
  if (path === '/logs/settings' && method === 'put') { Object.assign(demoState.logSettings, data); return clone(demoState.logSettings) }
  if (path === '/logs/cleanup') return { deleted: 0 }
  if (path === '/logs' && method === 'delete') { demoState.logs = []; return { ok: true } }
  if (path === '/backups' && method === 'get') return { items: clone(demoState.backups), settings: { enabled: true, cron: '0 3 * * *', retention: 7 } }
  if (path === '/backups/settings') return { enabled: true, cron: '0 3 * * *', retention: 7, ...data }
  if (path === '/backups' && method === 'post') return { name: `allbot-demo-${Date.now()}.zip`, message: '演示备份已创建' }
  if (segments[0] === 'backups' && method === 'delete') return { ok: true }
  if (path === '/images/settings' && method === 'get') return { storage_dir: '', max_size: 5 * 1024 * 1024, allowed_types: ['png', 'jpg', 'jpeg', 'gif', 'webp'] }
  if (path === '/images/settings' && method === 'put') return { ...data }
  if (path === '/images' && method === 'get') return { items: [], total: 0 }
  if (path === '/images' && method === 'post') return { id: `demo-image-${Date.now()}`, url: '' }
  if (segments[0] === 'images' && method === 'delete') return { ok: true }
  if (path === '/scheduled-tasks' && method === 'get') return clone(demoState.scheduledTasks)
  if (segments[0] === 'scheduled-tasks' && ['put', 'post'].includes(method)) { const item = data.id ? findByID(demoState.scheduledTasks, data.id) : null; if (item) Object.assign(item, data); else demoState.scheduledTasks.push({ ...data, id: Math.max(...demoState.scheduledTasks.map((row) => row.id), 0) + 1 }); return { ok: true } }
  if (segments[0] === 'scheduled-tasks' && method === 'delete') { demoState.scheduledTasks = demoState.scheduledTasks.filter((item) => String(item.id) !== id); return { ok: true } }
  if (path === '/script-tasks' && method === 'get') return { ...resultItems(demoState.scriptTasks, params), settings: { retention_days: 30, run_timeout_seconds: 3600, timeout_notify_admin_enabled: false } }
  if (path === '/script-tasks' && method === 'post') return { message: '脚本任务设置已保存', removed: 0 }
  if (segments[0] === 'script-tasks' && segments.length === 2 && method === 'get') return clone(findByID(demoState.scriptTasks, id) || {})
  if (segments[0] === 'script-tasks' && method === 'delete') return { message: '脚本任务已删除' }
  if (segments[0] === 'script-tasks' && method === 'put') return { message: '脚本任务已暂停' }
  if (path === '/plugin-tasks' && method === 'get') return { ...resultItems(demoState.pluginTasks, params), settings: { retention_days: 30, run_timeout_seconds: 3600, timeout_notify_admin_enabled: false } }
  if (path === '/plugin-tasks' && method === 'post') return { message: '插件任务设置已保存', removed: 0 }
  if (segments[0] === 'plugin-tasks' && segments.length === 2 && method === 'get') return clone(findByID(demoState.pluginTasks, id) || {})
  if (segments[0] === 'plugin-tasks' && method === 'delete') return { message: '插件任务已删除' }
  if (segments[0] === 'plugin-tasks' && method === 'put') return { message: '插件任务已停止' }
  if (path === '/replies/keywords' && method === 'get') return clone(demoState.replies)
  if (path === '/replies/keywords' && method === 'post') { const item = data.id ? findByID(demoState.replies, data.id) : null; if (item) Object.assign(item, data); else demoState.replies.push({ ...data, id: Math.max(...demoState.replies.map((row) => row.id), 0) + 1 }); return { ok: true } }
  if (segments[0] === 'replies' && method === 'delete') { demoState.replies = demoState.replies.filter((item) => String(item.id) !== id); return { ok: true } }
  if (path === '/settings' && method === 'get') return { auto_load_plugins: true, access_enabled: false, access_code: '', ...clone(demoState.settings) }
  if (path === '/settings' && method === 'put') { Object.assign(demoState.settings, data); return clone(demoState.settings) }
  if (path === '/payments/settings' && method === 'get') return clone(demoState.paymentSettings)
  if (path === '/payments/settings' && ['put', 'post'].includes(method)) { Object.assign(demoState.paymentSettings, data); return clone(demoState.paymentSettings) }
  if (path === '/payments/orders') return { items: [{ order_no: 'DEMO202610130001', provider: 'epay', status: 'paid', amount_cents: 990, points_amount: 99, created_at: now(), union_id: 'demo_user_001' }], total: 1 }
  if (segments[0] === 'payments' && segments[1] === 'orders' && segments.length >= 3) return { order: { order_no: id, status: 'paid', amount_cents: 990, points_amount: 99 }, events: [] }
  if (path === '/data/tables' && method === 'get') return clone(demoState.tables)
  if (segments[0] === 'data' && segments[1] === 'tables' && segments.length >= 4 && segments[3] === 'rows') return { columns: [{ name: 'union_id' }, { name: 'points' }, { name: 'disabled' }], rows: demoState.users.map((row, index) => ({ __rowid__: index + 1, ...row })), total: demoState.users.length }
  if (path === '/data/export') return JSON.stringify({ demo: true, tables: demoState.tables })
  if (path === '/dependencies') return { dependencies: [{ name: 'axios', version: '1.8.0', runtime: 'nodejs', profile_id: 'node-default' }, { name: 'requests', version: '2.32.3', runtime: 'python', profile_id: 'python-default' }] }
  if (path === '/sdk/files' && params.has('path')) return { path: params.get('path'), editable: true, code: '// Demo SDK source\\nexport default {}' }
  if (path === '/sdk/files') return { tree: [{ path: 'nodejs', text: 'Node.js', type: 'directory', children: [{ path: 'nodejs/allbot_direct.js', text: 'allbot_direct.js', type: 'file' }, { path: 'nodejs/account_ql_plugin.js', text: 'account_ql_plugin.js', type: 'file' }] }, { path: 'python', text: 'Python', type: 'directory', children: [{ path: 'python/allbot_direct.py', text: 'allbot_direct.py', type: 'file' }] }] }
  if (path === '/sdk/reference') return { nodejs: 'AllBot Node.js SDK reference', python: 'AllBot Python SDK reference' }
  return {}
}

export const demoRequest = async (config) => {
  await new Promise((resolve) => setTimeout(resolve, 70))
  const request = parseRequest(config)
  let data = handleRequest(request)
  if (config?.responseType === 'blob') data = new Blob([typeof data === 'string' ? data : JSON.stringify(data)], { type: 'application/json' })
  return { data: data instanceof Blob ? data : clone(data), status: 200, statusText: 'OK', headers: {}, config, request: null }
}

function webChatSession() {
  return { csrf_token: 'demo-web-chat-csrf', user: { username: 'demo', display_name: '演示用户', email: 'demo@example.com', union_id: 'demo_user_001' } }
}

export async function demoWebChatRequest(path, options = {}) {
  await new Promise((resolve) => setTimeout(resolve, 70))
  const cleanPath = String(path).split('?')[0]
  if (cleanPath === '/me') return webChatSession()
  if (cleanPath === '/plugins') return clone(webChatPlugins)
  if (cleanPath === '/platforms') return clone(demoState.adapters.filter((item) => item.running).map((item) => ({ id: item.id, platform: item.platform, display_name: item.display_name, remark: item.remark })))
  if (cleanPath === '/message-counts') return [{ plugin_id: '', count: webChatMessages.length }, ...webChatPlugins.map((item) => ({ plugin_id: item.id, count: 0 }))]
  if (cleanPath === '/messages' && String(options.method || 'GET').toUpperCase() === 'POST') {
    const payload = typeof options.body === 'string' ? JSON.parse(options.body || '{}') : {}
    const message = { message_id: Date.now(), direction: 'out', message_type: 'text', content: `演示回复：已收到“${payload.content || ''}”`, plugin_id: payload.plugin_id || '', created_at: now() }
    webChatMessages.push(message)
    return clone(message)
  }
  if (cleanPath === '/messages') return clone(webChatMessages)
  if (cleanPath === '/read-state') return { ok: true }
  if (cleanPath === '/logout') return { ok: true }
  if (cleanPath === '/email-code' || cleanPath === '/platform-code') return { ok: true, message: '演示环境验证码为 123456' }
  if (cleanPath === '/login' || cleanPath === '/email-login' || cleanPath === '/platform-login' || cleanPath === '/register') return webChatSession()
  if (cleanPath === '/bind-code') return { user: webChatSession().user }
  if (cleanPath === '/images') return { url: '' }
  return {}
}
