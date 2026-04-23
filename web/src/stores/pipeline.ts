import { ref, computed } from 'vue'
import api, { type PipelineStatus, type PipelineHistory } from '@/api'

export const status = ref<PipelineStatus | null>(null)
export const history = ref<PipelineHistory['history']>([])
export const loading = ref(false)

export const isRunning = computed(() => status.value?.running ?? false)

export async function fetchStatus() {
  try {
    const { data } = await api.get<PipelineStatus>('/pipeline/status')
    status.value = data
  } catch {
    // keep previous state on error
  }
}

export async function fetchHistory() {
  try {
    const { data } = await api.get<PipelineHistory>('/api/pipeline/history')
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
    await api.post('/pipeline/run', body)
    return true
  } catch (e) {
    throw e
  } finally {
    loading.value = false
  }
}

export async function cancelPipeline() {
  try {
    await api.post('/pipeline/cancel')
    return true
  } catch (e) {
    throw e
  }
}

export function formatDuration(ns: number): string {
  return (ns / 1_000_000_000).toFixed(1) + 's'
}

export function usePipelineStore() {
  return { status, history, loading, isRunning, fetchStatus, fetchHistory, runPipeline, cancelPipeline, formatDuration }
}