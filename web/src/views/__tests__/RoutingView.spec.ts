import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import RoutingView from '../RoutingView.vue'

vi.mock('@/stores/routing', () => ({
  useRoutingStore: () => ({
    fetchSnapshot: vi.fn(),
    snapshot: null,
    dryRunResult: null,
    loading: false,
    dryRun: vi.fn(),
    rollback: vi.fn(),
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

describe('RoutingView', () => {
  it('renders routing page', () => {
    const wrapper = mount(RoutingView, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        },
      },
    })
    expect(wrapper.text()).toContain('Routing State')
    expect(wrapper.text()).toContain('Dry Run')
  })
})
