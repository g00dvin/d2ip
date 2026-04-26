import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import LiveProgress from '../LiveProgress.vue'

const mockPipelineStore = {
  isRunning: false,
  liveProgress: null as Record<string, unknown> | null,
}

vi.mock('@/stores/pipeline', () => ({
  usePipelineStore: () => mockPipelineStore,
}))

describe('LiveProgress', () => {
  beforeEach(() => {
    mockPipelineStore.isRunning = false
    mockPipelineStore.liveProgress = null
  })

  it('does not render when idle', () => {
    const wrapper = mount(LiveProgress)
    expect(wrapper.find('div').exists()).toBe(false)
  })

  it('renders progress when running', () => {
    mockPipelineStore.isRunning = true
    mockPipelineStore.liveProgress = { step: 'resolve', resolved: 50, failed: 2, total: 100 }
    const wrapper = mount(LiveProgress, {
      global: {
        stubs: {
          'n-card': { template: '<div><div v-if="title">{{ title }}</div><slot /></div>', props: ['title'] },
          'n-space': { template: '<div><slot /></div>' },
          'n-text': { template: '<span><slot /></span>' },
          'n-tag': { template: '<span><slot /></span>' },
          'n-progress': { template: '<div>{{ percentage }}%</div>', props: ['percentage'] },
          'n-statistic': { template: '<div>{{ label }}: {{ value }}</div>', props: ['label', 'value'] },
        },
      },
    })
    expect(wrapper.text()).toContain('resolve')
    expect(wrapper.text()).toContain('Resolved: 50')
    expect(wrapper.text()).toContain('Failed: 2')
    expect(wrapper.text()).toContain('Total: 100')
  })
})
