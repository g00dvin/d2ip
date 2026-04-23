import axios from 'axios'

export const client = axios.create({
  baseURL: '/',
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
})

client.interceptors.response.use(
  (r) => r,
  (err) => {
    const message = err.response?.data?.error || err.message
    return Promise.reject(new Error(message))
  },
)
