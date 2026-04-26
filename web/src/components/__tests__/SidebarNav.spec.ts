import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SidebarNav from '../SidebarNav.vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ name: 'dashboard' }),
}))

describe('SidebarNav', () => {
  it('renders nav items', () => {
    const wrapper = mount(SidebarNav, { props: { open: true } })
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Pipeline')
    expect(wrapper.text()).toContain('Config')
  })

  it('shows open state', () => {
    const wrapper = mount(SidebarNav, { props: { open: true } })
    const aside = wrapper.find('aside')
    expect(aside.classes()).toContain('translate-x-0')
  })

  it('shows closed state on mobile', () => {
    const wrapper = mount(SidebarNav, { props: { open: false } })
    const aside = wrapper.find('aside')
    expect(aside.classes()).toContain('-translate-x-full')
  })

  it('emits close on backdrop click', () => {
    const wrapper = mount(SidebarNav, { props: { open: true } })
    wrapper.find('.fixed.inset-0').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
