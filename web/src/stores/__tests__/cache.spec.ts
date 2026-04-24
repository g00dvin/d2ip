import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useCacheStore } from '../cache'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('cache store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches stats', async () => {
    const mockStats = {
      domains: 100,
      domains_valid: 80,
      domains_failed: 10,
      domains_nxdomain: 10,
      records_total: 200,
      records_v4: 150,
      records_v6: 50,
      records_valid: 200,
      records_failed: 0,
      records_nxdomain: 0,
      oldest_updated: 0,
      newest_updated: 0,
    }
    vi.mocked(api.getCacheStats).mockResolvedValue(mockStats)

    const store = useCacheStore()
    await store.fetchStats()

    expect(store.stats).toEqual(mockStats)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('handles fetch error', async () => {
    vi.mocked(api.getCacheStats).mockRejectedValue(new Error('network error'))

    const store = useCacheStore()
    await store.fetchStats()

    expect(store.stats).toBeNull()
    expect(store.error).toBeInstanceOf(Error)
  })

  it('vacuums and refreshes stats', async () => {
    const mockStats = {
      domains: 100,
      domains_valid: 80,
      domains_failed: 10,
      domains_nxdomain: 10,
      records_total: 200,
      records_v4: 150,
      records_v6: 50,
      records_valid: 200,
      records_failed: 0,
      records_nxdomain: 0,
      oldest_updated: 0,
      newest_updated: 0,
    }
    vi.mocked(api.vacuumCache).mockResolvedValue({ deleted: 5 })
    vi.mocked(api.getCacheStats).mockResolvedValue(mockStats)

    const store = useCacheStore()
    await store.vacuum()

    expect(api.vacuumCache).toHaveBeenCalled()
    expect(api.getCacheStats).toHaveBeenCalled()
    expect(store.stats).toEqual(mockStats)
  })
})
