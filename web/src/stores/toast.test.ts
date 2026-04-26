import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useToast, toasts } from './toast'

describe('toast store', () => {
  beforeEach(() => {
    toasts.value = []
  })

  it('shows success toast', () => {
    const { success } = useToast()
    success('Saved')
    expect(toasts.value).toHaveLength(1)
    expect(toasts.value[0].message).toBe('Saved')
    expect(toasts.value[0].type).toBe('success')
  })

  it('shows error toast', () => {
    const { error } = useToast()
    error('Failed')
    expect(toasts.value).toHaveLength(1)
    expect(toasts.value[0].type).toBe('error')
  })

  it('auto-removes toast after 3 seconds', () => {
    vi.useFakeTimers()
    const { show } = useToast()
    show('Hello')
    expect(toasts.value).toHaveLength(1)
    vi.advanceTimersByTime(3000)
    expect(toasts.value).toHaveLength(0)
    vi.useRealTimers()
  })

  it('supports custom show with type', () => {
    const { show } = useToast()
    show('Warning', 'error')
    expect(toasts.value[0].type).toBe('error')
  })
})
