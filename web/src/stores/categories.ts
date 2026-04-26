import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/rest'
import type { CategoryDomainsResponse } from '@/api/types'

export const useCategoriesStore = defineStore('categories', () => {
  const available = ref<string[]>([])
  const loading = ref(false)
  const error = ref<Error | null>(null)
  const browserOpen = ref(false)
  const browserData = ref<CategoryDomainsResponse | null>(null)

  async function fetchCategories() {
    loading.value = true
    try {
      const data = await api.getCategories()
      available.value = data.available
      error.value = null
    } catch (e) {
      error.value = e as Error
    } finally {
      loading.value = false
    }
  }

  async function browseCategory(code: string) {
    try {
      const data = await api.getCategoryDomains(code, { per_page: 500 })
      browserData.value = data
      browserOpen.value = true
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    }
  }

  function closeBrowser() {
    browserOpen.value = false
    browserData.value = null
  }

  return {
    available, loading, error, browserOpen, browserData,
    fetchCategories, browseCategory, closeBrowser,
  }
})
