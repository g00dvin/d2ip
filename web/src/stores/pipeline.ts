import { ref, computed } from 'vue'
import { getPipelineStatus, getPipelineHistory, runPipeline as apiRunPipeline, cancelPipeline as apiCancelPipeline } from '@/api/rest'
import type { PipelineStatus, PipelineHistory } from '@/api/types'

export const status = ref<PipelineStatus | null>(null)
export const history = ref<PipelineHistory['history']>([])
export const loading = ref(false)

export const isRunning = computed(() => status.value?.running ?? false)

export async function fetchStatus() {
  try {
    status.value = await getPipelineStatus()
  } catch {
    // keep previous state on error
  }
}

export async function fetchHistory() {
  try {
    const data = await getPipelineHistory()
    history.value = data.history || []
  } catch {
    // keep previous state
  }
}

export async function runPipeline(forceResolve = false) {
  loading.value = true
  try {
    const body: Record<string, unknown> = {}
    if (forceResolve) body.force_resolve = true
    await apiRunPipeline(body)
    return true
  } catch (e) {
    throw e
  } finally {
    loading.value = false
  }
}

export async function cancelPipeline() {
  try {
    await apiCancelPipeline()
    return true
  } catch (e) {
    throw e
  } finally {
    loading.value = false
  }
}

export function formatDuration(ns: number): string {
  return (ns / 1_000_000_000).toFixed(1) + 's'
}

export function usePipelineStore() {
  return { status, history, loading, isRunning, fetchStatus, fetchHistory, runPipeline, cancelPipeline, formatDuration }
}
