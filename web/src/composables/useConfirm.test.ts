import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useConfirm } from './useConfirm'

describe('useConfirm composable', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('initializes with closed state', () => {
    const confirm = useConfirm()
    expect(confirm.visible.value).toBe(false)
    expect(confirm.message.value).toBe('')
  })

  it('opens dialog and returns promise on confirm', () => {
    const confirm = useConfirm()
    const promise = confirm.confirm('Are you sure?')

    expect(confirm.visible.value).toBe(true)
    expect(confirm.message.value).toBe('Are you sure?')

    confirm.onOk()
    expect(confirm.visible.value).toBe(false)
    return expect(promise).resolves.toBe(true)
  })

  it('resolves false on cancel', () => {
    const confirm = useConfirm()
    const promise = confirm.confirm('Delete this?')

    confirm.onCancel()
    expect(confirm.visible.value).toBe(false)
    return expect(promise).resolves.toBe(false)
  })

  it('handles multiple confirm calls sequentially', async () => {
    const confirm = useConfirm()

    const p1 = confirm.confirm('First?')
    confirm.onOk()
    expect(await p1).toBe(true)

    const p2 = confirm.confirm('Second?')
    confirm.onCancel()
    expect(await p2).toBe(false)
  })
})
