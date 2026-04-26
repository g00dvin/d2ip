import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ToastNotification from '../ToastNotification.vue'
import { useToast } from '@/stores/toast'

describe('ToastNotification', () => {
  beforeEach(() => {
    const { toasts } = useToast()
    toasts.value = []
  })

  it('renders success toasts', () => {
    const { show } = useToast()
    show('It worked', 'success')
    const wrapper = mount(ToastNotification)
    expect(wrapper.text()).toContain('It worked')
  })

  it('renders error toasts with error styling', () => {
    const { error } = useToast()
    error('It failed')
    const wrapper = mount(ToastNotification)
    expect(wrapper.text()).toContain('It failed')
    expect(wrapper.find('.border-err').exists()).toBe(true)
  })

  it('renders multiple toasts', () => {
    const { success, error } = useToast()
    success('First')
    error('Second')
    const wrapper = mount(ToastNotification)
    expect(wrapper.findAll('.fixed > div')).toHaveLength(2)
  })
})
