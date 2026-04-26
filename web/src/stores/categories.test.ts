import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useCategoriesStore } from './categories'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('Categories Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches categories and populates available state', async () => {
    const mockData = {
      configured: [{ code: 'geosite:ru', domain_count: 100 }],
      available: ['geosite:ru', 'geosite:google', 'ipverse:cn'],
    }
    vi.mocked(api.getCategories).mockResolvedValue(mockData)

    const store = useCategoriesStore()
    await store.fetchCategories()

    expect(store.available).toEqual(mockData.available)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('handles fetch error', async () => {
    vi.mocked(api.getCategories).mockRejectedValue(new Error('Network error'))

    const store = useCategoriesStore()
    await store.fetchCategories()

    expect(store.error).toBeInstanceOf(Error)
    expect(store.error?.message).toBe('Network error')
    expect(store.loading).toBe(false)
  })

  it('browses category and opens drawer', async () => {
    const mockDomains = {
      code: 'geosite:ru',
      domains: ['example.ru', 'test.ru'],
      page: 1,
      per_page: 100,
      total: 2,
      has_more: false,
    }
    vi.mocked(api.getCategoryDomains).mockResolvedValue(mockDomains)

    const store = useCategoriesStore()
    await store.browseCategory('geosite:ru')

    expect(api.getCategoryDomains).toHaveBeenCalledWith('geosite:ru', { per_page: 500 })
    expect(store.browserData).toEqual(mockDomains)
    expect(store.browserOpen).toBe(true)
  })

  it('closes browser drawer', () => {
    const store = useCategoriesStore()
    store.browserOpen = true
    store.browserData = { code: 'test', domains: [], page: 1, per_page: 10, total: 0, has_more: false }

    store.closeBrowser()

    expect(store.browserOpen).toBe(false)
    expect(store.browserData).toBeNull()
  })

  it('handles multi-source categories with prefixes correctly', async () => {
    const mockData = {
      configured: [],
      available: ['geosite:ru', 'geosite:google', 'ipverse:us', 'ipverse:de', 'mmdb:cn'],
    }
    vi.mocked(api.getCategories).mockResolvedValue(mockData)

    const store = useCategoriesStore()
    await store.fetchCategories()

    expect(store.available).toContain('ipverse:us')
    expect(store.available).toContain('mmdb:cn')
    expect(store.available).toHaveLength(5)
  })
})
