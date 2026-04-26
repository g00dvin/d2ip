import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import App from './App.vue'

vi.mock('@/composables/useSSE', () => ({
  useSSE: vi.fn(),
}))

vi.mock('@/stores/pipeline', () => ({
  usePipelineStore: () => ({
    handleSSE: vi.fn(),
  }),
}))

vi.mock('@/stores/config', () => ({
  useConfigStore: () => ({
    handleSSE: vi.fn(),
  }),
}))

vi.mock('@/stores/routing', () => ({
  useRoutingStore: () => ({
    handleSSE: vi.fn(),
  }),
}))

describe('App', () => {
  it('renders router view', () => {
    const wrapper = mount(App, {
      global: {
        plugins: [createPinia()],
        stubs: {
          'router-view': { template: '<div class="router-view">RouterView</div>' },
          'n-message-provider': { template: '<div><slot /></div>' },
          AppLayout: { template: '<div class="app-layout"><slot /></div>' },
        },
      },
    })
    expect(wrapper.find('.router-view').exists()).toBe(true)
  })
})
