import { ref } from 'vue'
import api, { type CacheStats } from '@/api'

export const stats = ref<CacheStats | null>(null)
export const loading = ref(false)

export async function fetchStats() {
  try {
    const { data } = await api.get<CacheStats>('/api/cache/stats')
    stats.value = data
  } catch {
    // keep previous state
  }
}

export async function vacuum() {
  loading.value = true
  try {
    await api.post('/api/cache/vacuum')
  } finally {
    loading.value = false
  }
  await fetchStats()
}