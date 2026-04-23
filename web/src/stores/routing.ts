import { ref } from 'vue'
import api, { type RoutingSnapshot, type DryRunResult } from '@/api'

export const snapshot = ref<RoutingSnapshot | null>(null)
export const dryRunResult = ref<DryRunResult | null>(null)
export const loading = ref(false)

export async function fetchSnapshot() {
  try {
    const { data } = await api.get<RoutingSnapshot>('/routing/snapshot')
    snapshot.value = data
  } catch {
    // keep previous state
  }
}

export async function dryRun() {
  loading.value = true
  try {
    const { data } = await api.post<DryRunResult>('/routing/dry-run', {
      ipv4_prefixes: [],
      ipv6_prefixes: [],
    })
    dryRunResult.value = data
  } catch (e) {
    throw e
  } finally {
    loading.value = false
  }
}

export async function rollback() {
  loading.value = true
  try {
    await api.post('/routing/rollback')
  } catch (e) {
    throw e
  } finally {
    loading.value = false
  }
  await fetchSnapshot()
}