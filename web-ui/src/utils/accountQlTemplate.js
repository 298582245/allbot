export const ACCOUNT_QL_TEMPLATES = ['nodejs_account_ql', 'python_account_ql']

export function isAccountQLTemplate(template) {
  return ACCOUNT_QL_TEMPLATES.includes(String(template || ''))
}

export function normalizeScriptRuntime(value, taskScript = '', fallback = 'nodejs') {
  const runtime = String(value || '').trim().toLowerCase()
  if (runtime === 'node' || runtime === 'js' || runtime === 'javascript') return 'nodejs'
  if (runtime === 'nodejs' || runtime === 'python') return runtime
  const extension = String(taskScript || '').trim().toLowerCase().split('?')[0]
  if (extension.endsWith('.py')) return 'python'
  if (extension.endsWith('.js') || extension.endsWith('.mjs') || extension.endsWith('.cjs')) return 'nodejs'
  return fallback === 'python' ? 'python' : 'nodejs'
}

export function defaultParseInputCode(runtime = 'nodejs') {
  if (runtime === 'python') {
    return `def parse_input(raw, ctx):
    value = str(raw or '').strip()
    if not value:
        raise RuntimeError('账号 CK 不能为空')
    return {
        "env_value": value,
        "unique_key": value,
        "display_name": value[:8],
    }`
  }
  return `function parseInput(raw, ctx) {
  const value = String(raw || '').trim();
  if (!value) throw new Error('账号 CK 不能为空');
  return {
    envValue: value,
    uniqueKey: value,
    displayName: value.slice(0, 8)
  };
}`
}

export function defaultQueryCode(runtime = 'nodejs') {
  if (runtime === 'python') {
    return `async def query(account, ctx, index):
    return {
        "状态": account.get("status") or "active",
    }`
  }
  return "async function query(account, ctx, index) {\n  return `${index + 1}. ${account.account_name}｜${account.status || 'active'}`;\n}"
}

export function defaultAfterRunCode(runtime = 'nodejs') {
  if (runtime === 'python') {
    return `async def after_run(ctx, accounts, result, helpers):
    if result.get("status") != "success":
        return
    # 示例：一键运行成功后给触发会话推送消息，可按业务条件改为 ctx.send_message 或 ctx.push
    await ctx.reply(f"一键运行完成，账号数：{len(accounts)}")`
  }
  return `async function afterRun(ctx, accounts, result, helpers) {
  if (result?.status !== 'success') return;
  // 示例：一键运行成功后给触发会话推送消息，可按业务条件改为 ctx.sendMessage 或 ctx.push
  await ctx.reply(\`一键运行完成，账号数：\${accounts.length}\`);
}`
}

export function defaultCheckCkCode(runtime = 'nodejs') {
  if (runtime === 'python') {
    return `async def check_ck(account, ctx):
    return {
        "valid": True,
        "reason": "",
    }`
  }
  return `async function checkCk(account, ctx) {
  return {
    valid: true,
    reason: ''
  };
}`
}

export function defaultRouteFunctionName(index = 0, runtime = 'nodejs') {
  return runtime === 'python' ? `custom_route_${index + 1}` : `customRoute${index + 1}`
}

export function defaultRouteCode(functionName = '', runtime = 'nodejs') {
  const name = functionName || defaultRouteFunctionName(0, runtime)
  if (runtime === 'python') {
    return `async def ${name}(ctx, helpers):
    accounts = await helpers.list_mine({"status": "active"})
    await ctx.reply(f"账号数：{len(accounts)}")`
  }
  return `async function ${name}(ctx, helpers) {
  const accounts = await helpers.listMine({ status: 'active' });
  return ctx.reply(\`账号数：\${accounts.length}\`);
}`
}

export function createEmptyAccountQLConfig(runtime = 'nodejs') {
  const normalizedRuntime = runtime === 'python' ? 'python' : 'nodejs'
  return {
    prefix: '',
    table_name: '',
    env_name: '',
    task_script: `scripts/task.${normalizedRuntime === 'python' ? 'py' : 'js'}`,
    script_runtime: normalizedRuntime,
    auth_price_per_month: 0,
    cron: '0 8 * * *',
    wait_scheduled: true,
    enable_after_run: false,
    after_run_code: defaultAfterRunCode(normalizedRuntime),
    enable_ck_check: true,
    ck_check_cron: '25 9 * * *',
    check_ck_code: defaultCheckCkCode(normalizedRuntime),
    enable_expire_check: false,
    expire_check_cron: '15 9 * * *',
    expire_notify_days: '7,3,1,0',
    expire_delete_after_days: -1,
    run_wait_timeout: 7200,
    parse_input_code: defaultParseInputCode(normalizedRuntime),
    query_code: defaultQueryCode(normalizedRuntime),
    routes: []
  }
}

export function normalizeAccountQLConfig(source = {}, runtime = 'nodejs') {
  const defaults = createEmptyAccountQLConfig(runtime)
  const value = source && typeof source === 'object' ? source : {}
  const normalizedRuntime = runtime === 'python' ? 'python' : 'nodejs'
  return {
    ...defaults,
    ...value,
    script_runtime: normalizeScriptRuntime(value.script_runtime, value.task_script, normalizedRuntime),
    auth_price_per_month: Math.max(0, Number(value.auth_price_per_month ?? defaults.auth_price_per_month)),
    run_wait_timeout: Math.max(1, Number(value.run_wait_timeout ?? defaults.run_wait_timeout)),
    wait_scheduled: value.wait_scheduled !== false,
    enable_after_run: Boolean(value.enable_after_run),
    enable_ck_check: value.enable_ck_check !== false,
    enable_expire_check: Boolean(value.enable_expire_check),
    expire_delete_after_days: Number(value.expire_delete_after_days ?? defaults.expire_delete_after_days),
    routes: Array.isArray(value.routes) ? value.routes.map((route, index) => ({
      id: route.id || `${Date.now()}_${index}`,
      command: String(route.command || ''),
      function_name: String(route.function_name || defaultRouteFunctionName(index, normalizedRuntime)),
      description: String(route.description || ''),
      code: String(route.code || '')
    })) : []
  }
}

export function accountQLCommands(accountQL = {}) {
  const commands = ['登录', '账号', '管理', '查询', '运行', '一键运行', '签到', '删除', '授权', '帮助']
  if (accountQL.enable_ck_check) commands.push('CK检测')
  if (accountQL.enable_expire_check) commands.push('过期检测')
  for (const route of accountQL.routes || []) {
    const command = String(route.command || '').trim()
    if (command) commands.push(command)
  }
  return commands
}

export function escapeRegExp(value) {
  return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function accountQLTriggerPreview(accountQL = {}) {
  const prefix = String(accountQL.prefix || '前缀').trim() || '前缀'
  return `^(${escapeRegExp(prefix)})(${accountQLCommands(accountQL).map(escapeRegExp).join('|')})$`
}

export const V2_SECTION_CATEGORY_LABELS = {
  parse: '输入解析',
  parser: '输入解析',
  parse_input: '输入解析',
  login: '登录',
  query: '查询',
  ck_check: 'CK 检测',
  ck: 'CK 检测',
  check_ck: 'CK 检测',
  auth: '授权',
  authorization: '授权',
  ql_registration: '青龙任务注册',
  registration: '青龙任务注册',
  ql_env: '青龙环境变量',
  env: '青龙环境变量',
  schedules: '定时任务',
  schedule: '定时任务',
  cron: '定时任务',
  routes: '自定义路由',
  route: '自定义路由',
  custom_route: '自定义路由',
  custom_routes: '自定义路由',
  after_run: '运行完成处理',
  afterrun: '运行完成处理',
  other: '其他'
}

export const V2_PLUGIN_EDITABLE_FIELDS = ['name', 'version', 'runtime_profile', 'priority', 'platforms', 'enabled', 'script_env']

export function isHybridTemplateSource(source = {}) {
  return Number(source?.version) === 2 && String(source?.mode || '').toLowerCase() === 'hybrid'
}

export function cloneTemplateValue(value) {
  if (value === undefined) return undefined
  return JSON.parse(JSON.stringify(value))
}

export function normalizeHybridTemplateSource(source = {}, pluginId = '') {
  const value = source && typeof source === 'object' ? cloneTemplateValue(source) : {}
  value.version = 2
  value.mode = 'hybrid'
  value.plugin = value.plugin && typeof value.plugin === 'object' ? value.plugin : {}
  value.plugin.id = String(pluginId || value.plugin.id || '')
  value.plugin.runtime = String(value.plugin.runtime || '').trim()
  value.plugin.template = String(value.plugin.template || value.template || '').trim()
  value.files = Array.isArray(value.files) ? value.files : []
  value.sections = Array.isArray(value.sections) ? value.sections.map(section => {
    if (!section || typeof section !== 'object' || Array.isArray(section)) return section
    return { ...section, content: String(section.content ?? '') }
  }) : []
  return value
}

export function hybridPluginEditableValue(source = {}, key) {
  if (!V2_PLUGIN_EDITABLE_FIELDS.includes(key)) return undefined
  const plugin = source?.plugin && typeof source.plugin === 'object' ? source.plugin : {}
  return cloneTemplateValue(plugin[key])
}

export function hybridSectionIsReadOnly(section = {}) {
  return String(section?.ownership || '').trim().toLowerCase() !== 'patchable'
}

export function hybridFileIsReadOnly(file = {}) {
  return String(file?.ownership || '').trim().toLowerCase() !== 'patchable'
}

export function hybridOwnershipLabel(ownership) {
  const value = String(ownership || '').trim().toLowerCase()
  if (value === 'referenced') return '引用（只读）'
  if (value === 'preserved') return '保留（只读）'
  if (value === 'generated') return '生成配置（只读）'
  if (value === 'patchable') return '可映射编辑'
  if (value === 'owned') return '模板管理'
  return value || '未标注'
}

export function hybridCategoryLabel(category) {
  const value = String(category || '').trim()
  return V2_SECTION_CATEGORY_LABELS[value.toLowerCase()] || value || '其他'
}

export function hybridSectionDiffs(initialSource = {}, currentSource = {}) {
  const initialSections = Array.isArray(initialSource.sections) ? initialSource.sections : []
  const currentSections = Array.isArray(currentSource.sections) ? currentSource.sections : []
  return initialSections.flatMap((initial, index) => {
    const current = currentSections[index]
    if (!current || String(initial?.content ?? '') === String(current?.content ?? '')) return []
    return [{
      id: String(initial?.id || index),
      label: current.label || initial?.label || initial?.id || `Section ${index + 1}`,
      category: current.category || initial?.category || 'other',
      path: current.path || initial?.path || ''
    }]
  })
}

export function hybridPluginDiffs(initialSource = {}, currentSource = {}) {
  const initialPlugin = initialSource?.plugin && typeof initialSource.plugin === 'object' ? initialSource.plugin : {}
  const currentPlugin = currentSource?.plugin && typeof currentSource.plugin === 'object' ? currentSource.plugin : {}
  return V2_PLUGIN_EDITABLE_FIELDS.filter(key => JSON.stringify(initialPlugin[key]) !== JSON.stringify(currentPlugin[key])).map(key => ({
    key,
    initial: cloneTemplateValue(initialPlugin[key]),
    current: cloneTemplateValue(currentPlugin[key])
  }))
}

export function hybridFileDiffs(initialSource = {}, currentSource = {}) {
  const initialFiles = Array.isArray(initialSource.files) ? initialSource.files : []
  const currentFiles = Array.isArray(currentSource.files) ? currentSource.files : []
  const length = Math.max(initialFiles.length, currentFiles.length)
  const files = []
  for (let index = 0; index < length; index += 1) {
    if (JSON.stringify(initialFiles[index]) === JSON.stringify(currentFiles[index])) continue
    const initial = initialFiles[index]
    const current = currentFiles[index]
    files.push({ path: String(current?.path || initial?.path || ''), initial, current })
  }
  return files
}

export function buildHybridTemplateSource(initialSource = {}, plugin = {}, sections = []) {
  const source = cloneTemplateValue(initialSource && typeof initialSource === 'object' ? initialSource : {}) || {}
  const initialPlugin = source.plugin && typeof source.plugin === 'object' && !Array.isArray(source.plugin) ? source.plugin : {}
  source.plugin = { ...initialPlugin }
  for (const key of V2_PLUGIN_EDITABLE_FIELDS) {
    if (Object.prototype.hasOwnProperty.call(plugin, key)) source.plugin[key] = cloneTemplateValue(plugin[key])
  }
  if (Array.isArray(source.sections)) {
    source.sections = source.sections.map((section, index) => {
      if (!section || typeof section !== 'object' || Array.isArray(section) || !Object.prototype.hasOwnProperty.call(section, 'content')) return section
      const edited = Array.isArray(sections) ? sections[index] : undefined
      if (!edited || typeof edited !== 'object' || Array.isArray(edited)) return section
      const content = String(edited.content ?? '')
      return content === String(section.content ?? '') ? section : { ...section, content }
    })
  }
  return source
}
