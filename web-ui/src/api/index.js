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

// 获取插件列表
export const getPlugins = () => {
  return request({
    url: '/plugins',
    method: 'get'
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
