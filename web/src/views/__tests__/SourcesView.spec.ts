import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SourcesView from '../SourcesView.vue'

vi.mock('@/stores/sources', () => ({
  useSourcesStore: () => ({
    fetchSources: vi.fn(),
    sources: [],
    loading: false,
    addSource: vi.fn(),
    removeSource: vi.fn(),
    reloadSource: vi.fn(),
  }),
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn() }),
  NButton: { render: () => null },
  NUpload: { render: () => null },
}))

vi.mock('@/api/rest', () => ({
  uploadSourceFile: vi.fn(),
}))

describe('SourcesView', () => {
  it('renders sources page', () => {
    const wrapper = mount(SourcesView, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        },
      },
    })
    expect(wrapper.text()).toContain('Sources')
  })
})
