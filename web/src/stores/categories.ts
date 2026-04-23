import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '@/api/rest'
import type { CategoryInfo, CategoryDomainsResponse } from '@/api/types'

export const useCategoriesStore = defineStore('categories', () => {
  const configured = ref<CategoryInfo[]>([])
  const available = ref<string[]>([])
  const loading = ref(false)
  const error = ref<Error | null>(null)
  const browserOpen = ref(false)
  const browserData = ref<CategoryDomainsResponse | null>(null)

  const hasCategories = computed(() => configured.value.length > 0)

  async function fetchCategories() {
    loading.value = true
    try {
      const data = await api.getCategories()
      configured.value = data.configured
      available.value = data.available
      error.value = null
    } catch (e) {
      error.value = e as Error
    } finally {
      loading.value = false
    }
  }

  async function addCategory(code: string) {
    try {
      await api.addCategory(code)
      await fetchCategories()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    }
  }

  async function removeCategory(code: string) {
    try {
      await api.removeCategory(code)
      browserOpen.value = false
      browserData.value = null
      await fetchCategories()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
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
    configured, available, loading, error, browserOpen, browserData,
    hasCategories,
    fetchCategories, addCategory, removeCategory, browseCategory, closeBrowser,
  }
})
