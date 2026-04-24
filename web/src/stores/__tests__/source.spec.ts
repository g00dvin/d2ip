import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSourceStore } from '../source'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('source store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches info', async () => {
    const mockInfo = {
      available: true,
      fetched_at: '2026-04-24T14:46:30Z',
      size: 123456,
      etag: '"abc123"',
      sha256: 'deadbeef',
      last_modified: '2026-04-24T14:46:30Z',
    }
    vi.mocked(api.getSourceInfo).mockResolvedValue(mockInfo)

    const store = useSourceStore()
    await store.fetchInfo()

    expect(store.info).toEqual(mockInfo)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('handles fetch error', async () => {
    vi.mocked(api.getSourceInfo).mockRejectedValue(new Error('network error'))

    const store = useSourceStore()
    await store.fetchInfo()

    expect(store.info).toBeNull()
    expect(store.error).toBeInstanceOf(Error)
  })

  it('fetches source and refreshes info', async () => {
    const mockInfo = {
      available: true,
      fetched_at: '2026-04-24T14:46:30Z',
      size: 123456,
    }
    vi.mocked(api.fetchSource).mockResolvedValue({ status: 'ok', fetched_at: '2026-04-24T14:46:30Z', size: 123456, sha256: 'deadbeef' })
    vi.mocked(api.getSourceInfo).mockResolvedValue(mockInfo)

    const store = useSourceStore()
    await store.fetchSource()

    expect(api.fetchSource).toHaveBeenCalled()
    expect(api.getSourceInfo).toHaveBeenCalled()
    expect(store.info).toEqual(mockInfo)
    expect(store.fetching).toBe(false)
  })

  it('handles fetch source error', async () => {
    vi.mocked(api.fetchSource).mockRejectedValue(new Error('download failed'))

    const store = useSourceStore()
    await expect(store.fetchSource()).rejects.toThrow('download failed')
    expect(store.fetching).toBe(false)
    expect(store.error).toBeInstanceOf(Error)
  })
})
