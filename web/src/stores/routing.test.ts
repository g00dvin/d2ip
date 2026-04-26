import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useRoutingStore } from './routing'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('routing store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches snapshot', async () => {
    vi.mocked(api.getRoutingSnapshot).mockResolvedValue({
      backend: 'nftables',
      applied_at: '2024-01-01T00:00:00Z',
      v4: 10,
      v6: 5,
    })
    const store = useRoutingStore()
    await store.fetchSnapshot()
    expect(store.snapshot).toBeTruthy()
    expect(store.error).toBeNull()
  })

  it('handles snapshot fetch error', async () => {
    vi.mocked(api.getRoutingSnapshot).mockRejectedValue(new Error('fail'))
    const store = useRoutingStore()
    await store.fetchSnapshot()
    expect(store.error).toBeInstanceOf(Error)
  })

  it('runs dry run', async () => {
    vi.mocked(api.dryRunRouting).mockResolvedValue({
      v4_plan: { add: ['1.2.3.0/24'], remove: [] },
      v6_plan: { add: [], remove: [] },
      v4_diff: '+1.2.3.0/24',
      v6_diff: '',
    })
    const store = useRoutingStore()
    await store.dryRun(['1.2.3.0/24'], [])
    expect(store.dryRunResult).toBeTruthy()
    expect(store.loading).toBe(false)
  })

  it('handles dry run error', async () => {
    vi.mocked(api.dryRunRouting).mockRejectedValue(new Error('fail'))
    const store = useRoutingStore()
    await expect(store.dryRun([], [])).rejects.toThrow('fail')
    expect(store.loading).toBe(false)
    expect(store.error).toBeInstanceOf(Error)
  })

  it('rolls back and refreshes', async () => {
    vi.mocked(api.rollbackRouting).mockResolvedValue(undefined)
    vi.mocked(api.getRoutingSnapshot).mockResolvedValue({ backend: 'none', applied_at: '', v4: 0, v6: 0 })
    const store = useRoutingStore()
    await store.rollback()
    expect(api.rollbackRouting).toHaveBeenCalled()
    expect(api.getRoutingSnapshot).toHaveBeenCalled()
  })

  it('handles rollback error', async () => {
    vi.mocked(api.rollbackRouting).mockRejectedValue(new Error('fail'))
    const store = useRoutingStore()
    await expect(store.rollback()).rejects.toThrow('fail')
    expect(store.error).toBeInstanceOf(Error)
  })

  it('handles SSE event', () => {
    const store = useRoutingStore()
    store.snapshot = { backend: 'nftables', applied_at: '', v4: 0, v6: 0 }
    store.handleSSE('routing.update', { v4: 10 })
    expect(store.snapshot?.v4).toBe(10)
  })
})
