import request from '@/utils/request'

// 插件相关API
export const getPlugins = () => {
  return request.get('/plugins')
}

export const getPlugin = (id) => {
  return request.get(`/plugins/${id}`)
}

export const createPlugin = (data) => {
  return request.post('/plugins', data)
}

export const updatePlugin = (id, data) => {
  return request.put(`/plugins/${id}`, data)
}

export const deletePlugin = (id) => {
  return request.delete(`/plugins/${id}`)
}

export const uploadPlugin = (file) => {
  const formData = new FormData()
  formData.append('file', file)
  return request.post('/plugins/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

// 订单相关API
export const getOrders = () => {
  return request.get('/payment/orders')
}

export const getOrder = (orderNo) => {
  return request.get(`/payment/orders/${orderNo}`)
}

// 统计相关API
export const getStatistics = () => {
  return request.get('/statistics')
}

export const getRevenue = (params) => {
  return request.get('/statistics/revenue', { params })
}

export const getDownloads = (params) => {
  return request.get('/statistics/downloads', { params })
}
