import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AppLayout from '../AppLayout.vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ name: 'dashboard' }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    isDark: false,
    toggleDark: vi.fn(),
    openMobileDrawer: vi.fn(),
    closeMobileDrawer: vi.fn(),
    mobileDrawerOpen: false,
  }),
}))

vi.mock('@/stores/health', () => ({
  useHealthStore: () => ({
    status: 'healthy',
    fetchHealth: vi.fn(),
  }),
}))

vi.mock('@/stores/version', () => ({
  useVersionStore: () => ({
    version: '1.0.0',
    fetchVersion: vi.fn(),
  }),
}))

vi.mock('@/composables/usePolling', () => ({
  usePolling: vi.fn(),
}))

vi.mock('@vicons/ionicons5', () => ({
  AnalyticsOutline: { render: () => null },
  ListOutline: { render: () => null },
  OptionsOutline: { render: () => null },
  CopyOutline: { render: () => null },
  ServerOutline: { render: () => null },
  GlobeOutline: { render: () => null },
  NavigateOutline: { render: () => null },
  SunnyOutline: { render: () => null },
  MoonOutline: { render: () => null },
  MenuOutline: { render: () => null },
  ShieldOutline: { render: () => null },
}))

describe('AppLayout', () => {
  it('renders slot content', () => {
    const wrapper = mount(AppLayout, {
      slots: { default: '<div class="page">Content</div>' },
    })
    expect(wrapper.find('.page').exists()).toBe(true)
  })

  it('shows page title', () => {
    const wrapper = mount(AppLayout)
    expect(wrapper.text()).toContain('Dashboard')
  })

  it('shows version', () => {
    const wrapper = mount(AppLayout)
    expect(wrapper.text()).toContain('1.0.0')
  })

  it('shows health status', () => {
    const wrapper = mount(AppLayout)
    expect(wrapper.text()).toContain('healthy')
  })
})
