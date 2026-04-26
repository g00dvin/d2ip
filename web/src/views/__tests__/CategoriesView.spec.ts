import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CategoriesView from '../CategoriesView.vue'

vi.mock('@/stores/categories', () => ({
  useCategoriesStore: () => ({
    fetchCategories: vi.fn(),
    available: [],
    loading: false,
    browserData: null,
    browserOpen: false,
    browseCategory: vi.fn(),
    closeBrowser: vi.fn(),
  }),
}))

describe('CategoriesView', () => {
  it('renders browse categories page', () => {
    const wrapper = mount(CategoriesView, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        },
      },
    })
    expect(wrapper.text()).toContain('Browse Categories')
  })
})
