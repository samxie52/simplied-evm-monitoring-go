import axios from 'axios';

// API 基础配置
const API_BASE_URL = 'http://localhost:8080';

// 创建 axios 实例
const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    console.log('API Request:', config.method?.toUpperCase(), config.url);
    return config;
  },
  (error) => {
    console.error('Request Error:', error);
    return Promise.reject(error);
  }
);

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    console.log('API Response:', response.status, response.config.url);
    return response;
  },
  (error) => {
    console.error('Response Error:', error.response?.status, error.response?.data);
    return Promise.reject(error);
  }
);

// API 服务类
class ApiService {
  // 健康检查
  async getHealth() {
    const response = await api.get('/health');
    return response.data;
  }

  // 告警相关 API
  async getAlerts(params = {}) {
    const response = await api.get('/api/v1/alerts', { params });
    return response.data;
  }

  async getAlert(id) {
    const response = await api.get(`/api/v1/alerts/${id}`);
    return response.data;
  }

  async getAlertStats() {
    const response = await api.get('/api/v1/alerts/stats');
    return response.data;
  }

  // 监控相关 API
  async getSystemStatus() {
    const response = await api.get('/api/v1/monitoring/status');
    return response.data;
  }

  async getSystemMetrics() {
    const response = await api.get('/api/v1/monitoring/metrics');
    return response.data;
  }

  async getRules() {
    const response = await api.get('/api/v1/monitoring/rules');
    return response.data;
  }

  async createRule(rule) {
    const response = await api.post('/api/v1/monitoring/rules', rule);
    return response.data;
  }

  async updateRule(id, rule) {
    const response = await api.put(`/api/v1/monitoring/rules/${id}`, rule);
    return response.data;
  }

  async deleteRule(id) {
    const response = await api.delete(`/api/v1/monitoring/rules/${id}`);
    return response.data;
  }
}

// 导出单例实例
export default new ApiService();
