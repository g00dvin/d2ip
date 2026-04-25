import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { usePolling } from './usePolling'

describe('usePolling composable', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('calls fn immediately and then on interval', async () => {
    const fn = vi.fn().mockResolvedValue(undefined)
    const { start, stop } = usePolling(fn, 1000)

    start()
    expect(fn).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1000)
    expect(fn).toHaveBeenCalledTimes(2)

    await vi.advanceTimersByTimeAsync(1000)
    expect(fn).toHaveBeenCalledTimes(3)

    stop()
  })

  it('supports dynamic interval', async () => {
    const fn = vi.fn().mockResolvedValue(undefined)
    let interval = 500
    const { start, stop } = usePolling(fn, () => interval)

    start()
    expect(fn).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(500)
    expect(fn).toHaveBeenCalledTimes(2)

    interval = 1000
    stop()
    start()

    await vi.advanceTimersByTimeAsync(1000)
    expect(fn).toHaveBeenCalledTimes(4)

    stop()
  })

  it('tracks loading state', async () => {
    let resolveFn: (() => void) | null = null
    const fn = vi.fn().mockImplementation(() => new Promise<void>((resolve) => { resolveFn = resolve }))
    const { loading, start, stop } = usePolling(fn, 1000)

    start()
    expect(loading.value).toBe(true)

    resolveFn!()
    await flushPromises()
    expect(loading.value).toBe(false)

    stop()
  })

  it('tracks last error on failure', async () => {
    const fn = vi.fn().mockRejectedValue(new Error('Poll failed'))
    const { lastError, start, stop } = usePolling(fn, 1000)

    start()
    await vi.advanceTimersByTimeAsync(0)

    expect(lastError.value).toBeInstanceOf(Error)
    expect(lastError.value?.message).toBe('Poll failed')

    stop()
  })

  it('clears last error on success after failure', async () => {
    let shouldFail = true
    const fn = vi.fn().mockImplementation(() => {
      if (shouldFail) return Promise.reject(new Error('Fail'))
      return Promise.resolve()
    })
    const { lastError, start, stop } = usePolling(fn, 1000)

    start()
    await vi.advanceTimersByTimeAsync(0)
    expect(lastError.value).toBeInstanceOf(Error)

    shouldFail = false
    await vi.advanceTimersByTimeAsync(1000)
    expect(lastError.value).toBeNull()

    stop()
  })

  it('stops polling when stop is called', async () => {
    const fn = vi.fn().mockResolvedValue(undefined)
    const { start, stop } = usePolling(fn, 1000)

    start()
    expect(fn).toHaveBeenCalledTimes(1)

    stop()
    await vi.advanceTimersByTimeAsync(5000)
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('restarts polling when start is called after stop', async () => {
    const fn = vi.fn().mockResolvedValue(undefined)
    const { start, stop } = usePolling(fn, 1000)

    start()
    stop()
    start()

    await vi.advanceTimersByTimeAsync(1000)
    expect(fn).toHaveBeenCalledTimes(3) // initial + restart initial + one interval

    stop()
  })
})
