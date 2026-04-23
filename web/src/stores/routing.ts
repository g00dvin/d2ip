import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/rest'
import type { RoutingSnapshot, DryRunResult } from '@/api/types'

export const useRoutingStore = defineStore('routing', () => {
  const snapshot = ref<RoutingSnapshot | null>(null)
  const dryRunResult = ref<DryRunResult | null>(null)
  const loading = ref(false)
  const error = ref<Error | null>(null)

  async function fetchSnapshot() {
    try {
      const data = await api.getRoutingSnapshot()
      snapshot.value = data
      error.value = null
    } catch (e) {
      error.value = e as Error
    }
  }

  async function dryRun(ipv4: string[], ipv6: string[]) {
    loading.value = true
    try {
      const data = await api.dryRunRouting({ ipv4_prefixes: ipv4, ipv6_prefixes: ipv6 })
      dryRunResult.value = data
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    } finally {
      loading.value = false
    }
  }

  async function rollback() {
    try {
      await api.rollbackRouting()
      await fetchSnapshot()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    }
  }

  function handleSSE(_event: string, data: unknown) {
    snapshot.value = { ...snapshot.value, ...(data as Record<string, unknown>) } as RoutingSnapshot
  }

  return { snapshot, dryRunResult, loading, error, fetchSnapshot, dryRun, rollback, handleSSE }
})
