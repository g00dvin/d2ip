import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/rest'
import type { CacheStats } from '@/api/types'

export const useCacheStore = defineStore('cache', () => {
  const stats = ref<CacheStats | null>(null)
  const loading = ref(false)
  const error = ref<Error | null>(null)

  async function fetchStats() {
    loading.value = true
    try {
      const data = await api.getCacheStats()
      stats.value = data
      error.value = null
    } catch (e) {
      error.value = e as Error
    } finally {
      loading.value = false
    }
  }

  async function vacuum() {
    try {
      await api.vacuumCache()
      await fetchStats()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    }
  }

  return { stats, loading, error, fetchStats, vacuum }
})
