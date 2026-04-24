import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useConfigStore } from '../config'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('config store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches settings', async () => {
    const mockSettings = {
      config: { listen: ':8080' },
      defaults: { listen: ':8080' },
      overrides: {},
    }
    vi.mocked(api.getSettings).mockResolvedValue(mockSettings)

    const store = useConfigStore()
    await store.fetchSettings()

    expect(store.settings).toEqual(mockSettings)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('handles fetch error', async () => {
    vi.mocked(api.getSettings).mockRejectedValue(new Error('network error'))

    const store = useConfigStore()
    await store.fetchSettings()

    expect(store.settings).toBeNull()
    expect(store.error).toBeInstanceOf(Error)
  })

  it('updates override', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({
      config: {}, defaults: {}, overrides: {},
    })
    vi.mocked(api.updateSettings).mockResolvedValue(undefined)

    const store = useConfigStore()
    await store.updateOverride('listen', ':9090')

    expect(api.updateSettings).toHaveBeenCalledWith({ listen: ':9090' })
    expect(api.getSettings).toHaveBeenCalled()
  })

  it('deletes override', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({
      config: {}, defaults: {}, overrides: {},
    })
    vi.mocked(api.deleteSetting).mockResolvedValue(undefined)

    const store = useConfigStore()
    await store.deleteOverride('listen')

    expect(api.deleteSetting).toHaveBeenCalledWith('listen')
    expect(api.getSettings).toHaveBeenCalled()
  })

  it('saves multiple settings', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({
      config: {}, defaults: {}, overrides: {},
    })
    vi.mocked(api.updateSettings).mockResolvedValue(undefined)

    const store = useConfigStore()
    await store.saveSettings({ listen: ':9090', 'logging.level': 'debug' })

    expect(api.updateSettings).toHaveBeenCalledWith({ listen: ':9090', 'logging.level': 'debug' })
  })
})
