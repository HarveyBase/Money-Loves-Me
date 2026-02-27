import axios from 'axios'

const api = axios.create({ baseURL: '/api/v1' })

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

export const authAPI = {
  login: (username: string, password: string) =>
    api.post('/auth/login', { username, password }),
}

export const marketAPI = {
  getKlines: (symbol: string) => api.get(`/market/klines/${symbol}`),
  getOrderBook: (symbol: string) => api.get(`/market/orderbook/${symbol}`),
}

export const orderAPI = {
  create: (data: Record<string, unknown>) => api.post('/orders', data),
  cancel: (id: string) => api.delete(`/orders/${id}`),
  list: (params?: Record<string, string>) => api.get('/orders', { params }),
  exportCSV: () => api.get('/orders/export', { responseType: 'blob' }),
}

export const accountAPI = {
  getBalances: () => api.get('/account/balances'),
  getPnL: () => api.get('/account/pnl'),
  getFees: () => api.get('/account/fees'),
}

export const strategyAPI = {
  start: () => api.post('/strategy/start'),
  stop: () => api.post('/strategy/stop'),
  status: () => api.get('/strategy/status'),
}

export const riskAPI = {
  getConfig: () => api.get('/risk/config'),
  updateConfig: (data: Record<string, unknown>) => api.put('/risk/config', data),
}

export const backtestAPI = {
  run: (data: Record<string, unknown>) => api.post('/backtest/run', data),
  results: () => api.get('/backtest/results'),
}

export const optimizerAPI = {
  history: () => api.get('/optimizer/history'),
}

export const notificationAPI = {
  list: () => api.get('/notifications'),
  markRead: (id: string) => api.put(`/notifications/${id}/read`),
  updateSettings: (data: Record<string, unknown>) =>
    api.put('/notifications/settings', data),
}

export const tradeAPI = {
  list: (params?: Record<string, string>) => api.get('/trades', { params }),
}

export default api
