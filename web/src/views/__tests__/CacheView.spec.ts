import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CacheView from '../CacheView.vue'

vi.mock('@/stores/cache', () => ({
  useCacheStore: () => ({
    fetchStats: vi.fn(),
    stats: null,
    loading: false,
    vacuum: vi.fn(),
  }),
}))

vi.mock('@/composables/usePolling', () => ({
  usePolling: vi.fn(),
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({
    visible: { value: false },
    message: { value: '' },
    confirm: vi.fn().mockResolvedValue(true),
    onOk: vi.fn(),
    onCancel: vi.fn(),
  }),
}))

describe('CacheView', () => {
  it('renders cache page', () => {
    const wrapper = mount(CacheView, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        },
      },
    })
    expect(wrapper.text()).toContain('Cache Statistics')
  })
})
