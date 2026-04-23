import { ref } from 'vue'
import { getHealth } from '@/api/rest'

export const healthStatus = ref<'healthy' | 'unhealthy' | 'checking'>('checking')

export async function fetchHealth() {
  try {
    const data = await getHealth()
    healthStatus.value = data.status === 'ok' ? 'healthy' : 'unhealthy'
  } catch {
    healthStatus.value = 'unhealthy'
  }
}