import axios from 'axios';

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
});

// 应用管理 API
export const appsAPI = {
  // 获取应用列表
  getAll: (params) => api.get('/admin/apps', { params }),
  
  // 获取单个应用
  getById: (id) => api.get(`/admin/apps/${id}`),
  
  // 创建应用
  create: (data) => api.post('/admin/apps', data),
  
  // 更新应用
  update: (id, data) => api.put(`/admin/apps/${id}`, data),
  
  // 删除应用
  delete: (id) => api.delete(`/admin/apps/${id}`),
  
  // 调整应用顺序
  reorder: (orders) => api.put('/admin/apps/reorder', { orders }),
};

// 分类管理 API
export const categoriesAPI = {
  // 获取分类列表
  getAll: (params) => api.get('/admin/categories', { params }),
  
  // 获取单个分类
  getById: (id) => api.get(`/admin/categories/${id}`),
  
  // 创建分类
  create: (data) => api.post('/admin/categories', data),
  
  // 更新分类
  update: (id, data) => api.put(`/admin/categories/${id}`, data),
  
  // 删除分类
  delete: (id) => api.delete(`/admin/categories/${id}`),
  
  // 调整分类顺序
  reorder: (orders) => api.put('/admin/categories/reorder', { orders }),
};

// 客户端包管理 API
export const packageAPI = {
  // 获取包状态
  getStatus: () => api.get('/admin/package-status'),
  
  // 生成客户端包
  generate: () => api.post('/admin/generate-client-package'),
};

// 服务器版本 API
export const versionAPI = {
  // 获取服务器数据版本
  get: () => api.get('/version'),
};

export default api;


