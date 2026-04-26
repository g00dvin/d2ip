import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CategoryBrowser from '../CategoryBrowser.vue'

describe('CategoryBrowser', () => {
  it('does not render when data is null', () => {
    const wrapper = mount(CategoryBrowser, { props: { data: null } })
    expect(wrapper.find('n-drawer').exists()).toBe(false)
  })

  it('renders with data', () => {
    const data = { code: 'test:cat', domains: ['a.com', 'b.com'], total: 2 }
    const wrapper = mount(CategoryBrowser, {
      props: { data },
      global: {
        stubs: {
          'n-drawer-content': { template: '<div>{{ title }}<slot /></div>', props: ['title'] },
          'n-virtual-list': {
            template: '<div><div v-for="item in items" :key="item"><slot :item="item" /></div></div>',
            props: ['items'],
          },
        },
      },
    })
    expect(wrapper.text()).toContain('test:cat')
    expect(wrapper.text()).toContain('a.com')
    expect(wrapper.text()).toContain('b.com')
  })
})
