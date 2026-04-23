import { ref } from 'vue'
import { getCacheStats, vacuumCache } from '@/api/rest'
import type { CacheStats } from '@/api/types'

export const stats = ref<CacheStats | null>(null)
export const loading = ref(false)

export async function fetchStats() {
  try {
    stats.value = await getCacheStats()
  } catch {
    // keep previous state
  }
}

export async function vacuum() {
  loading.value = true
  try {
    await vacuumCache()
  } finally {
    loading.value = false
  }
  await fetchStats()
}