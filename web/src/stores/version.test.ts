import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useVersionStore } from './version'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('version store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches version and build time', async () => {
    vi.mocked(api.getVersion).mockResolvedValue({ version: '1.2.3', build_time: '2024-06-01' })
    const store = useVersionStore()
    await store.fetchVersion()
    expect(store.version).toBe('1.2.3')
    expect(store.buildTime).toBe('2024-06-01')
  })

  it('silently ignores errors', async () => {
    vi.mocked(api.getVersion).mockRejectedValue(new Error('network'))
    const store = useVersionStore()
    await store.fetchVersion()
    expect(store.version).toBe('')
  })
})
