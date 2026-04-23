import { ref } from 'vue'
import api from '@/api'

export const healthStatus = ref<'healthy' | 'unhealthy' | 'checking'>('checking')

export async function fetchHealth() {
  try {
    const { data } = await api.get('/healthz')
    healthStatus.value = data.status === 'ok' ? 'healthy' : 'unhealthy'
  } catch {
    healthStatus.value = 'unhealthy'
  }
}