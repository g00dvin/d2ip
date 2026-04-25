import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePoliciesStore } from '../policies'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('policies store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches policies', async () => {
    const mockPolicies = {
      policies: [
        { name: 'streaming', enabled: true, categories: ['netflix', 'youtube'], backend: 'iproute2' as const, table_id: 100, dry_run: false, export_format: 'plain' },
        { name: 'corporate', enabled: false, categories: ['microsoft'], backend: 'nftables' as const, nft_set_v4: 'corp_v4', dry_run: true, export_format: 'nft' },
      ],
    }
    vi.mocked(api.getPolicies).mockResolvedValue(mockPolicies)

    const store = usePoliciesStore()
    await store.fetchPolicies()

    expect(store.policies).toEqual(mockPolicies.policies)
    expect(store.loading).toBe(false)
  })

  it('handles fetch error', async () => {
    vi.mocked(api.getPolicies).mockRejectedValue(new Error('network error'))

    const store = usePoliciesStore()
    await expect(store.fetchPolicies()).rejects.toThrow('network error')
    expect(store.policies).toEqual([])
    expect(store.loading).toBe(false)
  })

  it('defaults to empty array when API returns null policies', async () => {
    vi.mocked(api.getPolicies).mockResolvedValue({ policies: null as any })

    const store = usePoliciesStore()
    await store.fetchPolicies()

    expect(store.policies).toEqual([])
    expect(store.loading).toBe(false)
  })

  it('creates policy and refreshes list', async () => {
    const newPolicy = { name: 'test', enabled: true, categories: [], backend: 'none' as const, dry_run: false, export_format: 'plain' }
    vi.mocked(api.createPolicy).mockResolvedValue(undefined)
    vi.mocked(api.getPolicies).mockResolvedValue({ policies: [newPolicy] })

    const store = usePoliciesStore()
    await store.createPolicy(newPolicy)

    expect(api.createPolicy).toHaveBeenCalledWith(newPolicy)
    expect(store.policies).toEqual([newPolicy])
  })
})
