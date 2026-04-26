import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePipelineStore } from './pipeline'
import * as api from '@/api/rest'

vi.mock('@/api/rest')

describe('pipeline store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('fetches status', async () => {
    vi.mocked(api.getPipelineStatus).mockResolvedValue({
      running: true,
      run_id: 1,
      report: { run_id: 1, domains: 10, resolved: 8, failed: 2, duration: 1e9, cache_hits: 0, ipv4_out: 5, ipv6_out: 3 },
    })
    const store = usePipelineStore()
    await store.fetchStatus()
    expect(store.status).toBeTruthy()
    expect(store.isRunning).toBe(true)
    expect(store.error).toBeNull()
  })

  it('handles status fetch error', async () => {
    vi.mocked(api.getPipelineStatus).mockRejectedValue(new Error('fail'))
    const store = usePipelineStore()
    await store.fetchStatus()
    expect(store.error).toBeInstanceOf(Error)
  })

  it('fetches history', async () => {
    vi.mocked(api.getPipelineHistory).mockResolvedValue({
      history: [{ run_id: 1, domains: 10, resolved: 8, failed: 2, duration: 1e9, cache_hits: 0, ipv4_out: 5, ipv6_out: 3 }],
    })
    const store = usePipelineStore()
    await store.fetchHistory()
    expect(store.history).toHaveLength(1)
  })

  it('handles history fetch error', async () => {
    vi.mocked(api.getPipelineHistory).mockRejectedValue(new Error('fail'))
    const store = usePipelineStore()
    await store.fetchHistory()
    expect(store.error).toBeInstanceOf(Error)
  })

  it('runs pipeline', async () => {
    vi.mocked(api.runPipeline).mockResolvedValue(undefined)
    const store = usePipelineStore()
    await store.runPipeline()
    expect(api.runPipeline).toHaveBeenCalledWith({})
    expect(store.loading).toBe(false)
  })

  it('runs pipeline with options', async () => {
    vi.mocked(api.runPipeline).mockResolvedValue(undefined)
    const store = usePipelineStore()
    await store.runPipeline({ forceResolve: true, dryRun: true, skipRouting: true })
    expect(api.runPipeline).toHaveBeenCalledWith({
      force_resolve: true,
      dry_run: true,
      skip_routing: true,
    })
  })

  it('handles run error', async () => {
    vi.mocked(api.runPipeline).mockRejectedValue(new Error('fail'))
    const store = usePipelineStore()
    await expect(store.runPipeline()).rejects.toThrow('fail')
    expect(store.loading).toBe(false)
    expect(store.error).toBeInstanceOf(Error)
  })

  it('cancels pipeline', async () => {
    vi.mocked(api.cancelPipeline).mockResolvedValue(undefined)
    const store = usePipelineStore()
    await store.cancelPipeline()
    expect(api.cancelPipeline).toHaveBeenCalled()
  })

  it('handles cancel error', async () => {
    vi.mocked(api.cancelPipeline).mockRejectedValue(new Error('fail'))
    const store = usePipelineStore()
    await expect(store.cancelPipeline()).rejects.toThrow('fail')
    expect(store.error).toBeInstanceOf(Error)
  })

  it('handles SSE start event', () => {
    const store = usePipelineStore()
    store.handleSSE('pipeline.start', { step: 'resolve', total: 100 })
    expect(store.liveProgress).toEqual({ step: 'resolve', total: 100 })
  })

  it('handles SSE progress event', () => {
    const store = usePipelineStore()
    store.handleSSE('pipeline.progress', { resolved: 50 })
    expect(store.liveProgress).toEqual({ resolved: 50 })
  })

  it('handles SSE complete event', async () => {
    vi.mocked(api.getPipelineStatus).mockResolvedValue({ running: false, run_id: 0, started: '', report: null })
    vi.mocked(api.getPipelineHistory).mockResolvedValue({ history: [] })
    const store = usePipelineStore()
    store.liveProgress = { resolved: 50 }
    store.handleSSE('pipeline.complete', {})
    expect(store.liveProgress).toBeNull()
  })

  it('handles SSE failed event', async () => {
    vi.mocked(api.getPipelineStatus).mockResolvedValue({ running: false, run_id: 0, started: '', report: null })
    vi.mocked(api.getPipelineHistory).mockResolvedValue({ history: [] })
    const store = usePipelineStore()
    store.liveProgress = { resolved: 50 }
    store.handleSSE('pipeline.failed', {})
    expect(store.liveProgress).toBeNull()
  })

  it('formats duration', () => {
    const store = usePipelineStore()
    expect(store.formatDuration(1_500_000_000)).toBe('1.5s')
    expect(store.formatDuration(2_000_000_000)).toBe('2.0s')
  })
})
