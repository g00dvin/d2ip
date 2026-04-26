import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useHealthStore } from './health'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('health store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('sets healthy on ok status', async () => {
    vi.mocked(api.getHealth).mockResolvedValue({ status: 'ok' })
    const store = useHealthStore()
    await store.fetchHealth()
    expect(store.status).toBe('healthy')
  })

  it('sets unhealthy on error status', async () => {
    vi.mocked(api.getHealth).mockResolvedValue({ status: 'error' })
    const store = useHealthStore()
    await store.fetchHealth()
    expect(store.status).toBe('unhealthy')
  })

  it('sets unhealthy on fetch error', async () => {
    vi.mocked(api.getHealth).mockRejectedValue(new Error('network'))
    const store = useHealthStore()
    await store.fetchHealth()
    expect(store.status).toBe('unhealthy')
  })
})
