import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick } from 'vue'
import ConfigView from '../../views/ConfigView.vue'
import * as api from '@/api/rest'

vi.mock('@/api/rest')
vi.mock('naive-ui', async () => {
  const actual = await vi.importActual('naive-ui')
  return {
    ...actual,
    useMessage: () => ({
      success: vi.fn(),
      error: vi.fn(),
    }),
  }
})

describe('ConfigView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches settings on mount', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({
      config: { listen: ':8080' },
      defaults: { listen: ':8080' },
      overrides: {},
    })

    mount(ConfigView, {
      global: {
        stubs: ['n-card', 'n-spin', 'n-empty', 'n-tabs', 'n-tab-pane', 'n-form', 'n-form-item', 'n-input', 'n-input-group', 'n-select', 'n-button', 'n-space', 'n-text', 'n-alert', 'router-link'],
      },
    })

    await flushPromises()
    expect(api.getSettings).toHaveBeenCalled()
  })

  it('renders after settings load', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({
      config: { listen: ':8080' },
      defaults: { listen: ':8080' },
      overrides: {},
    })

    const wrapper = mount(ConfigView, {
      global: {
        stubs: ['n-card', 'n-spin', 'n-empty', 'n-tabs', 'n-tab-pane', 'n-form', 'n-form-item', 'n-input', 'n-input-group', 'n-select', 'n-button', 'n-space', 'n-text', 'n-alert', 'router-link'],
      },
    })

    await flushPromises()
    await nextTick()

    // After loading, settings should be available (no n-empty)
    expect(wrapper.findComponent({ name: 'n-empty' }).exists()).toBe(false)
  })
})
