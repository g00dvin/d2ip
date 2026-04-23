import { ref } from 'vue'
import { getRoutingSnapshot, dryRunRouting, rollbackRouting } from '@/api/rest'
import type { RoutingSnapshot, DryRunResult } from '@/api/types'

export const snapshot = ref<RoutingSnapshot | null>(null)
export const dryRunResult = ref<DryRunResult | null>(null)
export const loading = ref(false)

export async function fetchSnapshot() {
  try {
    snapshot.value = await getRoutingSnapshot()
  } catch {
    // keep previous state
  }
}

export async function dryRun() {
  loading.value = true
  try {
    dryRunResult.value = await dryRunRouting({
      ipv4_prefixes: [],
      ipv6_prefixes: [],
    })
  } catch (e) {
    throw e
  } finally {
    loading.value = false
  }
}

export async function rollback() {
  loading.value = true
  try {
    await rollbackRouting()
  } catch (e) {
    throw e
  } finally {
    loading.value = false
  }
  await fetchSnapshot()
}