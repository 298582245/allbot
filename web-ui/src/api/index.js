import request from '@/utils/request'

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

// 获取仪表盘消息统计
export const getMessageStats = (params = {}) => {
  return request({
    url: '/system/message-stats',
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

// 控制插件（启动/停止/重启）
export const controlPlugin = (pluginId, action) => {
  return request({
    url: `/plugins/${pluginId}`,
    method: 'post',
    data: { action }
  })
}

// 删除插件
export const deletePlugin = (pluginId) => {
  return request({
    url: `/plugins/${pluginId}`,
    method: 'delete'
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
export const getLogs = () => {
  return request({
    url: '/logs',
    method: 'get'
  })
}

// 清空日志
export const clearLogs = () => {
  return request({
    url: '/logs',
    method: 'delete'
  })
}
