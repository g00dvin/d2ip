import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import StatusBadge from '../StatusBadge.vue'

describe('StatusBadge', () => {
  it.each([
    ['ok', 'text-ok'],
    ['warn', 'text-warn'],
    ['error', 'text-err'],
    ['muted', 'text-txt-muted'],
  ] as const)('applies correct class for type %s', (type, expectedClass) => {
    const wrapper = mount(StatusBadge, { props: { type } })
    expect(wrapper.find('span').classes()).toContain(expectedClass)
  })

  it('renders slot content', () => {
    const wrapper = mount(StatusBadge, {
      props: { type: 'ok' },
      slots: { default: 'Active' },
    })
    expect(wrapper.text()).toBe('Active')
  })
})
