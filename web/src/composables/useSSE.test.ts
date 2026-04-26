import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref, nextTick } from 'vue'
import { useSSE } from './useSSE'

let mockClose: ReturnType<typeof vi.fn>
let mockStatus: ReturnType<typeof ref>
let mockEvent: ReturnType<typeof ref>
let mockData: ReturnType<typeof ref>
let mockError: ReturnType<typeof ref>

vi.mock('@vueuse/core', () => ({
  useEventSource: vi.fn(() => ({
    get status() { return mockStatus },
    get close() { return mockClose },
    get event() { return mockEvent },
    get data() { return mockData },
    get error() { return mockError },
  })),
}))

describe('useSSE', () => {
  beforeEach(() => {
    mockClose = vi.fn()
    mockStatus = ref('CONNECTING')
    mockEvent = ref<string | null>(null)
    mockData = ref<string | null>(null)
    mockError = ref<Error | null>(null)
  })

  it('returns initial disconnected state', () => {
    const { connected, error } = useSSE({})
    expect(connected.value).toBe(false)
    expect(error.value).toBeNull()
  })

  it('reflects OPEN status', async () => {
    const { connected } = useSSE({})
    mockStatus.value = 'OPEN'
    await nextTick()
    expect(connected.value).toBe(true)
  })

  it('clears error on OPEN after error', async () => {
    const { error } = useSSE({})
    mockError.value = new Error('fail')
    await nextTick()
    expect(error.value).toBeInstanceOf(Error)
    mockStatus.value = 'OPEN'
    await nextTick()
    expect(error.value).toBeNull()
  })

  it('calls handler on event with JSON data', async () => {
    const handler = vi.fn()
    mockStatus.value = 'OPEN'
    useSSE({ 'test-event': handler })
    mockEvent.value = 'test-event'
    mockData.value = '{"key":"value"}'
    await nextTick()
    expect(handler).toHaveBeenCalledWith({ key: 'value' })
  })

  it('calls handler with raw string on JSON parse failure', async () => {
    const handler = vi.fn()
    mockStatus.value = 'OPEN'
    useSSE({ 'test-event': handler })
    mockEvent.value = 'test-event'
    mockData.value = 'plain text'
    await nextTick()
    expect(handler).toHaveBeenCalledWith('plain text')
  })

  it('ignores events without handler', async () => {
    const handler = vi.fn()
    mockStatus.value = 'OPEN'
    useSSE({ 'other-event': handler })
    mockEvent.value = 'unknown-event'
    mockData.value = '{}'
    await nextTick()
    expect(handler).not.toHaveBeenCalled()
  })
})
