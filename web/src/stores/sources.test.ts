import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSourcesStore } from './sources'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('Sources Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches sources and populates state', async () => {
    const mockSources = [
      { id: 'v2fly-geosite', provider: 'v2flygeosite', prefix: 'geosite', enabled: true, categories: ['geosite:ru'] },
      { id: 'ipverse-blocks', provider: 'ipverse', prefix: 'ipverse', enabled: true, categories: ['ipverse:us'] },
    ]
    vi.mocked(api.getSources).mockResolvedValue({ sources: mockSources })

    const store = useSourcesStore()
    await store.fetchSources()

    expect(store.sources).toEqual(mockSources)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('handles fetch error', async () => {
    vi.mocked(api.getSources).mockRejectedValue(new Error('Connection refused'))

    const store = useSourcesStore()
    await store.fetchSources()

    expect(store.error).toBeInstanceOf(Error)
    expect(store.error?.message).toBe('Connection refused')
    expect(store.loading).toBe(false)
  })

  it('adds source and refreshes list', async () => {
    vi.mocked(api.createSource).mockResolvedValue(undefined)
    const newSources = [
      { id: 'mmdb-local', provider: 'mmdb', prefix: 'mmdb', enabled: true, categories: ['mmdb:de'] },
    ]
    vi.mocked(api.getSources).mockResolvedValue({ sources: newSources })

    const store = useSourcesStore()
    const sourceConfig = { id: 'mmdb-local', provider: 'mmdb', prefix: 'mmdb', enabled: true, config: { file: '/tmp/test.mmdb' } }
    await store.addSource(sourceConfig)

    expect(api.createSource).toHaveBeenCalledWith(sourceConfig)
    expect(store.sources).toEqual(newSources)
  })

  it('updates source and refreshes list', async () => {
    vi.mocked(api.updateSource).mockResolvedValue(undefined)
    const updatedSources = [
      { id: 'v2fly-geosite', provider: 'v2flygeosite', prefix: 'geosite', enabled: true, categories: ['geosite:ru', 'geosite:us'] },
    ]
    vi.mocked(api.getSources).mockResolvedValue({ sources: updatedSources })

    const store = useSourcesStore()
    const sourceConfig = { id: 'v2fly-geosite', provider: 'v2flygeosite', prefix: 'geosite', enabled: true, config: { url: 'https://example.com/dlc.dat' } }
    await store.updateSource('v2fly-geosite', sourceConfig)

    expect(api.updateSource).toHaveBeenCalledWith('v2fly-geosite', sourceConfig)
    expect(store.sources).toEqual(updatedSources)
  })

  it('removes source from local state', async () => {
    vi.mocked(api.deleteSource).mockResolvedValue(undefined)

    const store = useSourcesStore()
    store.sources = [
      { id: 'v2fly-geosite', provider: 'v2flygeosite', prefix: 'geosite', enabled: true, categories: [] },
      { id: 'ipverse-blocks', provider: 'ipverse', prefix: 'ipverse', enabled: true, categories: [] },
    ]

    await store.removeSource('ipverse-blocks')

    expect(api.deleteSource).toHaveBeenCalledWith('ipverse-blocks')
    expect(store.sources).toHaveLength(1)
    expect(store.sources[0].id).toBe('v2fly-geosite')
  })

  it('reloads source and refreshes list', async () => {
    vi.mocked(api.refreshSource).mockResolvedValue({ status: 'ok', info: { id: 'v2fly-geosite', provider: 'v2flygeosite', prefix: 'geosite', enabled: true, categories: [] } })
    const refreshedSources = [
      { id: 'v2fly-geosite', provider: 'v2flygeosite', prefix: 'geosite', enabled: true, categories: ['geosite:ru', 'geosite:us'] },
    ]
    vi.mocked(api.getSources).mockResolvedValue({ sources: refreshedSources })

    const store = useSourcesStore()
    await store.reloadSource('v2fly-geosite')

    expect(api.refreshSource).toHaveBeenCalledWith('v2fly-geosite')
    expect(store.sources).toEqual(refreshedSources)
  })
})
