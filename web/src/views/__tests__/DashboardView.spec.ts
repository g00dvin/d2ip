import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DashboardView from '../DashboardView.vue'

vi.mock('@/stores/pipeline', () => ({
  usePipelineStore: () => ({
    status: null,
    history: [],
    loading: false,
    isRunning: false,
    liveProgress: null,
    fetchStatus: vi.fn(),
    runPipeline: vi.fn(),
    formatDuration: (ns: number) => (ns / 1e9).toFixed(1) + 's',
  }),
}))

vi.mock('@/stores/categories', () => ({
  useCategoriesStore: () => ({
    fetchCategories: vi.fn(),
    hasCategories: true,
    available: [],
    configured: [],
    loading: false,
  }),
}))

vi.mock('@/stores/routing', () => ({
  useRoutingStore: () => ({
    fetchSnapshot: vi.fn(),
    snapshot: null,
  }),
}))

vi.mock('@/stores/policies', () => ({
  usePoliciesStore: () => ({
    fetchPolicies: vi.fn(),
    policies: [],
    loading: false,
  }),
}))

vi.mock('@/composables/usePolling', () => ({
  usePolling: vi.fn(),
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn() }),
}))

vi.mock('@/api/rest', () => ({
  downloadExport: vi.fn(),
}))

describe('DashboardView', () => {
  it('renders dashboard sections', () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        },
      },
    })
    expect(wrapper.text()).toContain('Quick Actions')
    expect(wrapper.text()).toContain('Last Run')
    expect(wrapper.text()).toContain('Routing State')
  })
})
