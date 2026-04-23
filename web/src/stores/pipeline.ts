import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '@/api/rest'
import type { PipelineStatus, PipelineHistory } from '@/api/types'

export const usePipelineStore = defineStore('pipeline', () => {
  const status = ref<PipelineStatus | null>(null)
  const history = ref<PipelineHistory['history']>([])
  const loading = ref(false)
  const error = ref<Error | null>(null)
  const liveProgress = ref<Record<string, unknown> | null>(null)

  const isRunning = computed(() => status.value?.running ?? false)

  async function fetchStatus() {
    try {
      const data = await api.getPipelineStatus()
      status.value = data
      error.value = null
    } catch (e) {
      error.value = e as Error
    }
  }

  async function fetchHistory() {
    try {
      const data = await api.getPipelineHistory()
      history.value = data.history ?? []
      error.value = null
    } catch (e) {
      error.value = e as Error
    }
  }

  async function runPipeline(opts?: { forceResolve?: boolean; dryRun?: boolean; skipRouting?: boolean }) {
    loading.value = true
    try {
      await api.runPipeline({
        force_resolve: opts?.forceResolve,
        dry_run: opts?.dryRun,
        skip_routing: opts?.skipRouting,
      })
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    } finally {
      loading.value = false
    }
  }

  async function cancelPipeline() {
    try {
      await api.cancelPipeline()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    }
  }

  function handleSSE(event: string, data: unknown) {
    switch (event) {
      case 'pipeline.start':
        liveProgress.value = { ...liveProgress.value, ...(data as Record<string, unknown>) }
        break
      case 'pipeline.progress':
        liveProgress.value = { ...liveProgress.value, ...(data as Record<string, unknown>) }
        break
      case 'pipeline.complete':
      case 'pipeline.failed':
      case 'pipeline.cancel':
        liveProgress.value = null
        fetchStatus()
        fetchHistory()
        break
    }
  }

  function formatDuration(ns: number): string {
    return (ns / 1_000_000_000).toFixed(1) + 's'
  }

  return {
    status, history, loading, error, liveProgress,
    isRunning,
    fetchStatus, fetchHistory, runPipeline, cancelPipeline,
    handleSSE, formatDuration,
  }
})
