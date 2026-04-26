import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PipelineView from '../PipelineView.vue'

vi.mock('@/stores/pipeline', () => ({
  usePipelineStore: () => ({
    fetchStatus: vi.fn(),
    fetchHistory: vi.fn(),
    runPipeline: vi.fn(),
    cancelPipeline: vi.fn(),
    status: null,
    history: [],
    isRunning: false,
    loading: false,
    formatDuration: (ns: number) => (ns / 1e9).toFixed(1) + 's',
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

describe('PipelineView', () => {
  it('renders pipeline page', () => {
    const wrapper = mount(PipelineView, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-empty': { template: '<div>{{ description }}</div>', props: ['description'] },
        },
      },
    })
    expect(wrapper.text()).toContain('Pipeline Control')
    expect(wrapper.text()).toContain('Status')
  })
})
