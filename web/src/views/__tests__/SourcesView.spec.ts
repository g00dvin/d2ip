import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import SourcesView from '../SourcesView.vue'

vi.mock('@/stores/sources', () => {
  const fetchSources = vi.fn()
  const addSource = vi.fn()
  const updateSource = vi.fn()
  const removeSource = vi.fn()
  const reloadSource = vi.fn()
  return {
    useSourcesStore: () => ({
      fetchSources,
      sources: [
        { id: 'test-source', provider: 'plaintext', prefix: 'test', enabled: true, categories: [] },
      ],
      loading: false,
      addSource,
      updateSource,
      removeSource,
      reloadSource,
    }),
  }
})

vi.mock('naive-ui', () => {
  const success = vi.fn()
  const error = vi.fn()
  return {
    useMessage: () => ({ success, error }),
    NButton: { render: () => null },
    NUpload: { render: () => null },
  }
})

vi.mock('@/api/rest', () => {
  const uploadSourceFile = vi.fn()
  const getSource = vi.fn()
  return {
    uploadSourceFile,
    getSource,
  }
})

function mountSourcesView() {
  return mount(SourcesView, {
    global: {
      stubs: {
        'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
        'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        'n-data-table': { template: '<div />', props: ['columns', 'data'] },
        'n-modal': {
          template: '<div v-if="show"><div class="modal-title">{{ title }}</div><slot /><slot name="footer" /></div>',
          props: ['show', 'title'],
        },
        'n-form': { template: '<div v-if="model"><slot /></div>', props: ['model'] },
        'n-form-item': { template: '<div><slot /></div>', props: ['label'] },
        'n-input': { template: '<input />', props: ['value', 'disabled', 'placeholder'] },
        'n-select': { template: '<select />', props: ['value', 'disabled', 'options'] },
        'n-button': { template: '<button><slot /></button>', props: ['type', 'size', 'onClick'] },
        'n-spin': { template: '<div />' },
        'n-upload': { template: '<div />' },
        'n-tag': { template: '<span><slot /></span>', props: ['type', 'size'] },
      },
    },
  })
}

describe('SourcesView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders sources page', () => {
    const wrapper = mountSourcesView()
    expect(wrapper.text()).toContain('Sources')
  })

  it('shows Edit Source title in edit mode', async () => {
    const wrapper = mountSourcesView()
    await wrapper.setData({
      showModal: true,
      modalMode: 'edit',
      editing: { id: 'test-source', provider: 'plaintext', prefix: 'test', enabled: true, config: {} },
    })
    expect(wrapper.find('.modal-title').text()).toBe('Edit Source')
  })

  it('shows Add Source title in add mode', async () => {
    const wrapper = mountSourcesView()
    await wrapper.setData({
      showModal: true,
      modalMode: 'add',
      editing: { id: '', provider: 'plaintext', prefix: '', enabled: true, config: { type: 'domains', file: '' } },
    })
    expect(wrapper.find('.modal-title').text()).toBe('Add Source')
  })

  it('sets up edit mode state correctly', async () => {
    const wrapper = mountSourcesView()
    await wrapper.setData({
      showModal: true,
      modalMode: 'edit',
      editing: { id: 'test-source', provider: 'plaintext', prefix: 'test', enabled: true, config: { type: 'domains', file: '/path/to/file.txt' } },
    })

    expect((wrapper.vm as any).modalMode).toBe('edit')
    expect((wrapper.vm as any).editing.id).toBe('test-source')
    expect((wrapper.vm as any).editing.config.file).toBe('/path/to/file.txt')
  })

  it('sets up add mode state correctly', async () => {
    const wrapper = mountSourcesView()
    await wrapper.setData({
      showModal: true,
      modalMode: 'add',
      editing: { id: 'new-source', provider: 'plaintext', prefix: 'new', enabled: true, config: { type: 'domains', file: '' } },
    })

    expect((wrapper.vm as any).modalMode).toBe('add')
    expect((wrapper.vm as any).editing.id).toBe('new-source')
  })

  it('resets modalMode to add on modal close', async () => {
    const wrapper = mountSourcesView()
    await wrapper.setData({
      showModal: true,
      modalMode: 'edit',
      editing: { id: 'test-source', provider: 'plaintext', prefix: 'test', enabled: true, config: {} },
    })
    expect((wrapper.vm as any).modalMode).toBe('edit')

    await wrapper.setData({ showModal: false })
    expect((wrapper.vm as any).modalMode).toBe('add')
  })
})
