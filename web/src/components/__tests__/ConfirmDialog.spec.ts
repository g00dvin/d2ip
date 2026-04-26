import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import ConfirmDialog from '../ConfirmDialog.vue'

const mockConfirmState = {
  visible: ref(false),
  message: ref(''),
  onOk: vi.fn(),
  onCancel: vi.fn(),
}

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => mockConfirmState,
}))

describe('ConfirmDialog', () => {
  beforeEach(() => {
    mockConfirmState.visible.value = false
    mockConfirmState.message.value = ''
    mockConfirmState.onOk.mockClear()
    mockConfirmState.onCancel.mockClear()
  })

  it('is hidden by default', () => {
    const wrapper = mount(ConfirmDialog)
    expect(wrapper.find('.fixed').exists()).toBe(false)
  })

  it('renders message when visible', () => {
    mockConfirmState.visible.value = true
    mockConfirmState.message.value = 'Are you sure?'
    const wrapper = mount(ConfirmDialog, {
      global: {
        stubs: {
          Teleport: { template: '<div><slot /></div>' },
        },
      },
    })
    expect(wrapper.text()).toContain('Are you sure?')
  })

  it('calls onOk when confirm clicked', () => {
    mockConfirmState.visible.value = true
    const wrapper = mount(ConfirmDialog, {
      global: {
        stubs: {
          Teleport: { template: '<div><slot /></div>' },
        },
      },
    })
    const buttons = wrapper.findAll('button')
    expect(buttons.length).toBeGreaterThan(0)
    buttons[0].trigger('click')
    expect(mockConfirmState.onOk).toHaveBeenCalled()
  })

  it('calls onCancel when cancel clicked', () => {
    mockConfirmState.visible.value = true
    const wrapper = mount(ConfirmDialog, {
      global: {
        stubs: {
          Teleport: { template: '<div><slot /></div>' },
        },
      },
    })
    const buttons = wrapper.findAll('button')
    expect(buttons.length).toBeGreaterThan(1)
    buttons[1].trigger('click')
    expect(mockConfirmState.onCancel).toHaveBeenCalled()
  })
})
