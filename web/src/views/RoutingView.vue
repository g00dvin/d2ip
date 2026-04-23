<script setup lang="ts">
import { onMounted } from 'vue'
import { usePolling } from '@/composables/usePolling'
import { useToast } from '@/stores/toast'
import { snapshot, dryRunResult, loading, fetchSnapshot, dryRun, rollback } from '@/stores/routing'
import StatusBadge from '@/components/StatusBadge.vue'
import { useConfirm } from '@/composables/useConfirm'

const toast = useToast()
const confirm = useConfirm()

const poll = usePolling(fetchSnapshot, 30_000)
onMounted(() => poll.start())

function fmtDate(d: string | null | undefined) {
  return d ? new Date(d).toLocaleString() : 'never'
}

async function handleDryRun() {
  try {
    await dryRun()
    toast.success('dry-run complete')
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function handleRollback() {
  if (!(await confirm.confirm('rollback routing changes?'))) return
  try {
    await rollback()
    toast.success('rollback complete')
  } catch (e) {
    toast.error((e as Error).message)
  }
}
</script>

<template>
  <div>
    <div class="panel">
      <div class="panel-label">routing state</div>
      <template v-if="snapshot">
        <div class="meta-text">backend: {{ snapshot.backend || 'none' }}</div>
        <div class="meta-text">ipv4: {{ snapshot.v4?.length ?? 0 }} prefixes</div>
        <div class="meta-text">ipv6: {{ snapshot.v6?.length ?? 0 }} prefixes</div>
        <div class="meta-text">applied: {{ fmtDate(snapshot.applied_at) }}</div>
      </template>
      <template v-else>
        <StatusBadge type="muted">routing disabled</StatusBadge>
      </template>
    </div>

    <div v-if="snapshot && (snapshot.backend === 'none' || !snapshot.backend)" class="warning-banner mb-3">
      Routing is disabled. Enable it in Config → routing.enabled.
    </div>

    <div v-if="dryRunResult" class="panel">
      <div class="panel-label">dry-run result</div>
      <template v-if="dryRunResult.message">
        <div class="meta-text">{{ dryRunResult.message }}</div>
      </template>
      <template v-else>
        <div class="meta-text">v4 add: {{ dryRunResult.v4_plan?.add?.length ?? 0 }} | remove: {{ dryRunResult.v4_plan?.remove?.length ?? 0 }}</div>
        <div class="meta-text">v6 add: {{ dryRunResult.v6_plan?.add?.length ?? 0 }} | remove: {{ dryRunResult.v6_plan?.remove?.length ?? 0 }}</div>
        <pre v-if="dryRunResult.v4_diff" class="bg-surface-code border border-border p-3 text-xs overflow-x-auto mt-2">{{ dryRunResult.v4_diff }}</pre>
        <pre v-if="dryRunResult.v6_diff" class="bg-surface-code border border-border p-3 text-xs overflow-x-auto mt-2">{{ dryRunResult.v6_diff }}</pre>
      </template>
    </div>

    <div class="panel">
      <div class="panel-label">actions</div>
      <div class="flex gap-2">
        <button class="btn btn-accent" @click="handleDryRun" :disabled="loading">🔍 dry run</button>
        <button class="btn btn-danger" @click="handleRollback" :disabled="loading">↩ rollback</button>
      </div>
    </div>
  </div>
</template>