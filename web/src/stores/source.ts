import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/rest'
import type { SourceInfo } from '@/api/types'

export const useSourceStore = defineStore('source', () => {
  const info = ref<SourceInfo | null>(null)
  const loading = ref(false)
  const fetching = ref(false)
  const error = ref<Error | null>(null)

  async function fetchInfo() {
    loading.value = true
    try {
      const data = await api.getSourceInfo()
      info.value = data
      error.value = null
    } catch (e) {
      error.value = e as Error
    } finally {
      loading.value = false
    }
  }

  async function fetchSource() {
    fetching.value = true
    try {
      await api.fetchSource()
      await fetchInfo()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    } finally {
      fetching.value = false
    }
  }

  return { info, loading, fetching, error, fetchInfo, fetchSource }
})
