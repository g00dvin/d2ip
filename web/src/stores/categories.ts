import { ref } from 'vue'
import { getCategories, addCategory as apiAddCategory, removeCategory as apiRemoveCategory, getCategoryDomains } from '@/api/rest'
import type { CategoryInfo } from '@/api/types'

export const configured = ref<CategoryInfo[]>([])
export const available = ref<string[]>([])
export const allAvailable = ref<string[]>([])
export const domains = ref<string[]>([])
export const domainsTotal = ref(0)
export const domainsCode = ref('')
export const loading = ref(false)

export async function fetchCategories() {
  loading.value = true
  try {
    const data = await getCategories()
    configured.value = data.configured || []
    available.value = data.available || []
    allAvailable.value = data.available || []
  } finally {
    loading.value = false
  }
}

export async function addCategory(code: string) {
  await apiAddCategory(code)
  await fetchCategories()
}

export async function removeCategory(code: string) {
  await apiRemoveCategory(code)
  domainsCode.value = ''
  domains.value = []
  await fetchCategories()
}

export async function fetchDomains(code: string) {
  const data = await getCategoryDomains(code, { per_page: 500 })
  domains.value = data.domains || []
  domainsTotal.value = data.total
  domainsCode.value = code
}