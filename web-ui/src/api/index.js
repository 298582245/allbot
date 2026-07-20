import request from '@/utils/request'

const webChatRequest = async (path, options = {}) => {
  const response = await fetch(`/api/open/web-chat${path}`, {
    credentials: 'include',
    headers: options.body instanceof FormData ? options.headers : { 'Content-Type': 'application/json', ...(options.headers || {}) },
    ...options
  })
  const contentType = response.headers.get('Content-Type') || ''
  const data = contentType.includes('application/json') ? await response.json() : await response.text()
  const isEnvelope = data !== null && typeof data === 'object' && !Array.isArray(data) &&
    Object.prototype.hasOwnProperty.call(data, 'code') &&
    Object.prototype.hasOwnProperty.call(data, 'msg') &&
    Object.prototype.hasOwnProperty.call(data, 'data')
  if (!response.ok || data?.error) {
    throw new Error(data?.msg || data?.error || data?.message || '请求失败')
  }
  return isEnvelope ? data.data : data
}

export const sendWebChatEmailCode = (email, purpose = 'register') => webChatRequest('/email-code', { method: 'POST', body: JSON.stringify({ email, purpose }) })
export const registerWebChat = (data) => webChatRequest('/register', { method: 'POST', body: JSON.stringify(data) })
export const resetWebChatPassword = (data) => webChatRequest('/reset-password', { method: 'POST', body: JSON.stringify(data) })
export const loginWebChat = (data) => webChatRequest('/login', { method: 'POST', body: JSON.stringify(data) })
export const loginWebChatByEmailCode = (data) => webChatRequest('/email-login', { method: 'POST', body: JSON.stringify(data) })
export const getWebChatPlatforms = () => webChatRequest('/platforms')
export const sendWebChatPlatformCode = (data) => webChatRequest('/platform-code', { method: 'POST', body: JSON.stringify(data) })
export const loginWebChatByPlatformCode = (data) => webChatRequest('/platform-login', { method: 'POST', body: JSON.stringify(data) })
export const logoutWebChat = (csrfToken) => webChatRequest('/logout', { method: 'POST', headers: { 'X-AllBot-WebChat-CSRF': csrfToken || '' }, body: '{}' })
export const getWebChatMe = () => webChatRequest('/me')
export const bindWebChatCode = (code, csrfToken) => webChatRequest('/bind-code', { method: 'POST', headers: { 'X-AllBot-WebChat-CSRF': csrfToken }, body: JSON.stringify({ code }) })
export const getWebChatPlugins = () => webChatRequest('/plugins')
export const getWebChatMessageCounts = () => webChatRequest('/message-counts')
export const markWebChatRead = (data, csrfToken) => webChatRequest('/read-state', { method: 'POST', headers: { 'X-AllBot-WebChat-CSRF': csrfToken }, body: JSON.stringify(data || {}) })
export const getWebChatMessages = (params = {}) => {
  const search = new URLSearchParams(params).toString()
  return webChatRequest(`/messages${search ? `?${search}` : ''}`)
}
export const sendWebChatMessage = (data, csrfToken) => webChatRequest('/messages', { method: 'POST', headers: { 'X-AllBot-WebChat-CSRF': csrfToken }, body: JSON.stringify(data) })
export const uploadWebChatImage = (file, csrfToken) => {
  const formData = new FormData()
  formData.append('file', file)
  return webChatRequest('/images', { method: 'POST', headers: { 'X-AllBot-WebChat-CSRF': csrfToken }, body: formData })
}

// 登录
export const login = (data) => {
  return request({
    url: '/login',
    method: 'post',
    data
  })
}

// 获取系统状态
export const getSystemStatus = () => {
  return request({
    url: '/system/status',
    method: 'get'
  })
}

// 获取更新信息
export const getUpdateInfo = () => {
  return request({
    url: '/system/update',
    method: 'get'
  })
}

// 获取升级状态
export const getUpdateStatus = () => {
  return request({
    url: '/system/update/status',
    method: 'get',
    silent: true
  })
}

// 执行一键升级
export const startSystemUpgrade = () => {
  return request({
    url: '/system/update/upgrade',
    method: 'post',
    timeout: 15 * 60 * 1000
  })
}

// 获取仪表盘消息统计
export const getMessageStats = (params = {}) => {
  return request({
    url: '/system/message-stats',
    method: 'get',
    params
  })
}

// 获取数据统计概览
export const getStatisticsOverview = () => {
  return request({
    url: '/statistics/overview',
    method: 'get'
  })
}

// 获取消息总量趋势
export const getMessageTotalTrend = (params = {}) => {
  return request({
    url: '/statistics/message-total-trend',
    method: 'get',
    params
  })
}

// 获取插件触发趋势
export const getPluginTriggerTrend = (params = {}) => {
  return request({
    url: '/statistics/plugin-trigger-trend',
    method: 'get',
    params
  })
}

// 获取插件列表
export const getPlugins = () => {
  return request({
    url: '/plugins',
    method: 'get'
  })
}

// 获取插件 Web 面板列表
export const getPluginWebPanels = () => {
  return request({
    url: '/plugin-web/panels',
    method: 'get',
    silent: true
  })
}

// 获取插件创建模板
export const getPluginTemplates = () => {
  return request({
    url: '/plugins/templates',
    method: 'get',
    silent: true
  })
}

// 预览插件创建结果
export const previewCreatePlugin = (data) => {
  return request({
    url: '/plugins/preview',
    method: 'post',
    data,
    silent: true
  })
}

// 校验插件创建配置
export const validateCreatePlugin = (data) => {
  return request({
    url: '/plugins/validate',
    method: 'post',
    data
  })
}

// 创建插件
export const createPlugin = (data) => {
  return request({
    url: '/plugins',
    method: 'post',
    data
  })
}

// 获取插件模板编辑模型
export const getPluginTemplateEditor = (pluginId) => {
  return request({
    url: `/plugins/template-editor/${encodeURIComponent(String(pluginId))}`,
    method: 'get',
    silent: true
  })
}

// 保存插件模板编辑模型
export const updatePluginTemplateEditor = (pluginId, data) => {
  return request({
    url: `/plugins/template-editor/${encodeURIComponent(String(pluginId))}`,
    method: 'put',
    data,
    silent: true
  })
}

// 将旧账号模板插件转换为可分类编辑模型
export const convertPluginTemplateEditor = (pluginId, data = {}) => {
  return updatePluginTemplateEditor(pluginId, {
    ...data,
    convert_legacy: true,
    overwrite_generated_files: true
  })
}

// 控制插件（启动/停止/重启）
export const controlPlugin = (pluginId, action) => {
  return request({
    url: `/plugins/${pluginId}`,
    method: 'post',
    data: { action }
  })
}

// 设置插件置顶状态
export const setPluginPinned = (pluginId, pinned) => {
  return controlPlugin(pluginId, pinned ? 'pin' : 'unpin')
}

// 删除插件
export const deletePlugin = (pluginId) => {
  return request({
    url: `/plugins/${pluginId}`,
    method: 'delete'
  })
}

// 获取插件回收站
export const getPluginRecycleBin = () => {
  return request({
    url: '/plugins/recycle-bin',
    method: 'get'
  })
}

// 删除插件备份压缩包
export const deletePluginBackup = (name) => {
  return request({
    url: '/plugins/recycle-bin',
    method: 'delete',
    params: { name }
  })
}

// 获取脚本环境变量列表
export const getScriptEnvs = (params = {}) => {
  return request({
    url: '/script-envs',
    method: 'get',
    params
  })
}

// 创建脚本环境变量
export const createScriptEnv = (data) => {
  return request({
    url: '/script-envs',
    method: 'post',
    data
  })
}

// 更新脚本环境变量
export const updateScriptEnv = (id, data) => {
  return request({
    url: `/script-envs/${encodeURIComponent(String(id))}`,
    method: 'put',
    data
  })
}

// 删除脚本环境变量
export const deleteScriptEnv = (id) => {
  return request({
    url: `/script-envs/${encodeURIComponent(String(id))}`,
    method: 'delete'
  })
}

// 批量操作脚本环境变量
export const batchScriptEnvs = (action, ids) => {
  return request({
    url: '/script-envs',
    method: 'patch',
    data: { action, ids }
  })
}

// 导入脚本环境变量
export const importScriptEnvs = (data) => {
  return request({
    url: '/script-envs/import',
    method: 'post',
    data
  })
}

// 获取开放接口列表
export const getOpenApis = (params = {}) => {
  return request({
    url: '/open-apis',
    method: 'get',
    params
  })
}

// 获取开放接口详情
export const getOpenApi = (id) => {
  return request({
    url: `/open-apis/${encodeURIComponent(String(id))}`,
    method: 'get'
  })
}

// 创建开放接口
export const createOpenApi = (data) => {
  return request({
    url: '/open-apis',
    method: 'post',
    data
  })
}

// 更新开放接口
export const updateOpenApi = (id, data) => {
  return request({
    url: `/open-apis/${encodeURIComponent(String(id))}`,
    method: 'put',
    data
  })
}

// 删除开放接口
export const deleteOpenApi = (id) => {
  return request({
    url: `/open-apis/${encodeURIComponent(String(id))}`,
    method: 'delete'
  })
}

// 获取开放接口全局设置
export const getOpenApiSettings = () => {
  return request({
    url: '/open-apis/settings',
    method: 'get'
  })
}

// 保存开放接口全局设置
export const saveOpenApiSettings = (data) => {
  return request({
    url: '/open-apis/settings',
    method: 'put',
    data
  })
}

// 获取开放接口调用明细
export const getOpenApiCalls = (id, params = {}) => {
  return request({
    url: `/open-apis/${encodeURIComponent(String(id))}/calls`,
    method: 'get',
    params
  })
}

// 获取开放接口代码
export const getOpenApiCode = (id) => {
  return request({
    url: `/open-apis/${encodeURIComponent(String(id))}/code`,
    method: 'get'
  })
}

// 更新开放接口代码
export const updateOpenApiCode = (id, data) => {
  return request({
    url: `/open-apis/${encodeURIComponent(String(id))}/code`,
    method: 'put',
    data
  })
}

// 获取运行环境 Profile
export const getRuntimeProfiles = () => {
  return request({
    url: '/runtime-profiles',
    method: 'get'
  })
}

// 保存运行环境 Profile
export const saveRuntimeProfiles = (profiles) => {
  return request({
    url: '/runtime-profiles',
    method: 'put',
    data: { profiles }
  })
}

// 获取运行环境可下载候选版本
export const getRuntimeDownloadCandidates = (params = {}) => {
  return request({
    url: '/runtime-profiles/download-candidates',
    method: 'get',
    params,
    silent: true
  })
}

// 获取运行环境下载设置
export const getRuntimeDownloadSettings = () => {
  return request({
    url: '/runtime-profiles/download-settings',
    method: 'get'
  })
}

// 保存运行环境下载设置
export const saveRuntimeDownloadSettings = (data) => {
  return request({
    url: '/runtime-profiles/download-settings',
    method: 'put',
    data
  })
}

// 初始化运行环境 Profile
export const initRuntimeProfile = (data) => {
  return request({
    url: '/runtime-profiles/init',
    method: 'post',
    data,
    timeout: 15 * 60 * 1000
  })
}

// 获取运行环境 Profile 初始化任务
export const getRuntimeProfileInitJob = (jobId) => {
  return request({
    url: `/runtime-profiles/init/${encodeURIComponent(String(jobId))}`,
    method: 'get',
    silent: true
  })
}

// 获取运行环境 Profile 最新初始化任务
export const getLatestRuntimeProfileInitJob = (profileId) => {
  return request({
    url: '/runtime-profiles/init/latest',
    method: 'get',
    params: { profile_id: profileId },
    silent: true
  })
}

// 获取运行环境 Profile 状态
export const getRuntimeProfileStatus = () => {
  return request({
    url: '/runtime-profiles/status',
    method: 'get'
  })
}

// 测试运行环境 Profile
export const testRuntimeProfile = (profile) => {
  return request({
    url: '/runtime-profiles/test',
    method: 'post',
    data: profile
  })
}

// 获取用户列表
export const getUsers = (params = {}) => {
  return request({
    url: '/users',
    method: 'get',
    params
  })
}

// 获取用户详情
export const getUser = (unionId) => {
  return request({
    url: `/users/${encodeURIComponent(String(unionId))}`,
    method: 'get'
  })
}

// 更新用户整体状态
export const updateUserStatus = (unionId, data) => {
  return request({
    url: `/users/${encodeURIComponent(String(unionId))}/status`,
    method: 'patch',
    data
  })
}

// 调整用户积分
export const adjustUserPoints = (unionId, data) => {
  return request({
    url: `/users/${encodeURIComponent(String(unionId))}/points/adjust`,
    method: 'post',
    data
  })
}

// 获取用户积分流水
export const getUserPointTransactions = (unionId, params = {}) => {
  return request({
    url: `/users/${encodeURIComponent(String(unionId))}/point-transactions`,
    method: 'get',
    params
  })
}

// 获取平台账号列表
export const getUserAccounts = (params = {}) => {
  return request({
    url: '/user-accounts',
    method: 'get',
    params
  })
}

// 获取适配器平台列表
export const getAdapterPlatforms = () => {
  return request({
    url: '/adapter-platforms',
    method: 'get',
    silent: true
  })
}

// 获取适配器列表
export const getAdapters = () => {
  return request({
    url: '/adapters',
    method: 'get'
  })
}

// 创建/更新适配器
export const saveAdapter = (data) => {
  return request({
    url: '/adapters',
    method: 'post',
    data
  })
}

// 设置适配器置顶状态
export const setAdapterPinned = (adapterId, pinned) => {
  return request({
    url: `/adapters/${adapterId}`,
    method: 'post',
    data: { action: pinned ? 'pin' : 'unpin' }
  })
}

// 获取适配器详情
export const getAdapter = (platform) => {
  return request({
    url: `/adapters/${platform}`,
    method: 'get'
  })
}

// 删除适配器
export const deleteAdapter = (platform) => {
  return request({
    url: `/adapters/${platform}`,
    method: 'delete'
  })
}

// 获取日志
export const getLogs = (params = {}) => {
  return request({
    url: '/logs',
    method: 'get',
    params
  })
}

// 下载日志原文件或全部日志压缩包
export const downloadLogs = (params = {}) => {
  return request({
    url: '/logs/download',
    method: 'get',
    params,
    responseType: 'blob',
    transformResponse: [(data) => data],
    silent: true,
    timeout: 10 * 60 * 1000
  })
}

// 清空日志
export const clearLogs = (params = {}) => {
  return request({
    url: '/logs',
    method: 'delete',
    params
  })
}

// 获取日志设置
export const getLogSettings = () => {
  return request({
    url: '/logs/settings',
    method: 'get'
  })
}

// 保存日志设置
export const saveLogSettings = (data) => {
  return request({
    url: '/logs/settings',
    method: 'put',
    data
  })
}

// 立即清理日志
export const cleanupLogs = (data = {}) => {
  return request({
    url: '/logs/cleanup',
    method: 'post',
    data
  })
}

// 获取备份概览
export const getBackups = () => {
  return request({
    url: '/backups',
    method: 'get'
  })
}

// 保存备份配置
export const saveBackupSettings = (data) => {
  return request({
    url: '/backups/settings',
    method: 'put',
    data
  })
}

// 手动创建备份
export const createBackup = () => {
  return request({
    url: '/backups',
    method: 'post',
    timeout: 10 * 60 * 1000
  })
}

// 导入备份文件
export const importBackup = (formData) => {
  return request({
    url: '/backups/import',
    method: 'post',
    data: formData,
    timeout: 10 * 60 * 1000
  })
}

// 恢复备份文件
export const restoreBackup = (name, data) => {
  return request({
    url: `/backups/${encodeURIComponent(String(name))}/restore`,
    method: 'post',
    data,
    timeout: 15 * 60 * 1000
  })
}

// 删除备份文件
export const deleteBackup = (name) => {
  return request({
    url: `/backups/${encodeURIComponent(String(name))}/delete`,
    method: 'delete'
  })
}

// 获取图床配置
export const getImageSettings = () => {
  return request({
    url: '/images/settings',
    method: 'get'
  })
}

// 保存图床配置
export const saveImageSettings = (data) => {
  return request({
    url: '/images/settings',
    method: 'put',
    data
  })
}

// 获取图床图片列表
export const listImages = (params = {}) => {
  return request({
    url: '/images',
    method: 'get',
    params
  })
}

// 上传图床图片
export const uploadImage = (formData) => {
  return request({
    url: '/images',
    method: 'post',
    data: formData,
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 120000
  })
}

// 删除图床图片
export const deleteImage = (id) => {
  return request({
    url: `/images/${encodeURIComponent(String(id))}`,
    method: 'delete'
  })
}
