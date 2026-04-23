<script setup lang="ts">
import { onMounted } from 'vue'
import { usePolling } from '@/composables/usePolling'
import { useToast } from '@/stores/toast'
import { useRoutingStore } from '@/stores/routing'
const routing = useRoutingStore()
import StatusBadge from '@/components/StatusBadge.vue'
import { useConfirm } from '@/composables/useConfirm'

const toast = useToast()
const confirm = useConfirm()

const poll = usePolling(() => routing.fetchSnapshot(), 30_000)
onMounted(() => poll.start())

function fmtDate(d: string | null | undefined) {
  return d ? new Date(d).toLocaleString() : 'never'
}

async function handleDryRun() {
  try {
    await routing.dryRun([], [])
    toast.success('dry-run complete')
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function handleRollback() {
  if (!(await confirm.confirm('rollback routing changes?'))) return
  try {
    await routing.rollback()
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
      <template v-if="routing.snapshot">
        <div class="meta-text">backend: {{ routing.snapshot.backend || 'none' }}</div>
        <div class="meta-text">ipv4: {{ routing.snapshot.v4?.length ?? 0 }} prefixes</div>
        <div class="meta-text">ipv6: {{ routing.snapshot.v6?.length ?? 0 }} prefixes</div>
        <div class="meta-text">applied: {{ fmtDate(routing.snapshot.applied_at) }}</div>
      </template>
      <template v-else>
        <StatusBadge type="muted">routing disabled</StatusBadge>
      </template>
    </div>

    <div v-if="routing.snapshot && (routing.snapshot.backend === 'none' || !routing.snapshot.backend)" class="warning-banner mb-3">
      Routing is disabled. Enable it in Config → routing.enabled.
    </div>

    <div v-if="routing.dryRunResult" class="panel">
      <div class="panel-label">dry-run result</div>
      <template v-if="routing.dryRunResult.message">
        <div class="meta-text">{{ routing.dryRunResult.message }}</div>
      </template>
      <template v-else>
        <div class="meta-text">v4 add: {{ routing.dryRunResult.v4_plan?.add?.length ?? 0 }} | remove: {{ routing.dryRunResult.v4_plan?.remove?.length ?? 0 }}</div>
        <div class="meta-text">v6 add: {{ routing.dryRunResult.v6_plan?.add?.length ?? 0 }} | remove: {{ routing.dryRunResult.v6_plan?.remove?.length ?? 0 }}</div>
        <pre v-if="routing.dryRunResult.v4_diff" class="bg-surface-code border border-border p-3 text-xs overflow-x-auto mt-2">{{ routing.dryRunResult.v4_diff }}</pre>
        <pre v-if="routing.dryRunResult.v6_diff" class="bg-surface-code border border-border p-3 text-xs overflow-x-auto mt-2">{{ routing.dryRunResult.v6_diff }}</pre>
      </template>
    </div>

    <div class="panel">
      <div class="panel-label">actions</div>
      <div class="flex gap-2">
        <button class="btn btn-accent" @click="handleDryRun" :disabled="routing.loading">🔍 dry run</button>
        <button class="btn btn-danger" @click="handleRollback" :disabled="routing.loading">↩ rollback</button>
      </div>
    </div>
  </div>
</template>