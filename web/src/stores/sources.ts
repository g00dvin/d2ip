import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/rest'
import type { SourceInfo, SourceConfig } from '@/api/types'

export const useSourcesStore = defineStore('sources', () => {
  const sources = ref<SourceInfo[]>([])
  const loading = ref(false)
  const error = ref<Error | null>(null)

  async function fetchSources() {
    loading.value = true
    try {
      const data = await api.getSources()
      sources.value = data.sources
      error.value = null
    } catch (e) {
      error.value = e as Error
    } finally {
      loading.value = false
    }
  }

  async function addSource(source: SourceConfig) {
    await api.createSource(source)
    await fetchSources()
  }

  async function updateSource(id: string, payload: SourceConfig) {
    await api.updateSource(id, payload)
    await fetchSources()
  }

  async function removeSource(id: string) {
    await api.deleteSource(id)
    sources.value = sources.value.filter(s => s.id !== id)
  }

  async function reloadSource(id: string) {
    await api.refreshSource(id)
    await fetchSources()
  }

  return { sources, loading, error, fetchSources, addSource, updateSource, removeSource, reloadSource }
})
