import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CategoriesView from '../CategoriesView.vue'

vi.mock('@/stores/categories', () => ({
  useCategoriesStore: () => ({
    fetchCategories: vi.fn(),
    configured: [],
    available: [],
    hasCategories: false,
    loading: false,
    browserData: null,
    browserOpen: false,
    addCategory: vi.fn(),
    removeCategory: vi.fn(),
    browseCategory: vi.fn(),
    closeBrowser: vi.fn(),
  }),
}))

vi.mock('naive-ui', () => ({
  NButton: { render: () => null },
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

describe('CategoriesView', () => {
  it('renders categories page', () => {
    const wrapper = mount(CategoriesView, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        },
      },
    })
    expect(wrapper.text()).toContain('Configured Categories')
    expect(wrapper.text()).toContain('Available Categories')
  })
})
