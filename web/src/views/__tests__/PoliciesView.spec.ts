import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PoliciesView from '../PoliciesView.vue'

vi.mock('@/stores/policies', () => ({
  usePoliciesStore: () => ({
    fetchPolicies: vi.fn(),
    policies: [],
    loading: false,
    createPolicy: vi.fn(),
    updatePolicy: vi.fn(),
    deletePolicy: vi.fn(),
  }),
}))

vi.mock('@/stores/categories', () => ({
  useCategoriesStore: () => ({
    fetchCategories: vi.fn(),
    available: [],
    loading: false,
  }),
}))

vi.mock('@/composables/usePolling', () => ({
  usePolling: vi.fn(),
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn() }),
}))

describe('PoliciesView', () => {
  it('renders policies page', () => {
    const wrapper = mount(PoliciesView, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        },
      },
    })
    expect(wrapper.text()).toContain('Routing Policies')
  })
})
