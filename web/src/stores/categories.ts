import { ref } from 'vue'
import api, { type CategoriesResponse, type CategoryDomainsResponse, type CategoryInfo } from '@/api'

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
    const { data } = await api.get<CategoriesResponse>('/api/categories')
    configured.value = data.configured || []
    available.value = data.available || []
    allAvailable.value = data.available || []
  } finally {
    loading.value = false
  }
}

export async function addCategory(code: string) {
  await api.post('/api/categories', { code })
  await fetchCategories()
}

export async function removeCategory(code: string) {
  await api.delete(`/api/categories/${encodeURIComponent(code)}`)
  domainsCode.value = ''
  domains.value = []
  await fetchCategories()
}

export async function fetchDomains(code: string) {
  const { data } = await api.get<CategoryDomainsResponse>(
    `/api/categories/${encodeURIComponent(code)}/domains`,
    { params: { per_page: 500 } },
  )
  domains.value = data.domains || []
  domainsTotal.value = data.total
  domainsCode.value = code
}