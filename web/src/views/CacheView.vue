<script setup lang="ts">
import { onMounted } from 'vue'
import { usePolling } from '@/composables/usePolling'
import { useToast } from '@/stores/toast'
import { stats, fetchStats, vacuum } from '@/stores/cache'
import StatusBadge from '@/components/StatusBadge.vue'
import { useConfirm } from '@/composables/useConfirm'

const toast = useToast()
const confirm = useConfirm()

const poll = usePolling(fetchStats, 30_000)
onMounted(() => poll.start())

function fmtDate(d: string | null | undefined) {
  return d ? new Date(d).toLocaleString() : '-'
}

async function handleVacuum() {
  if (!(await confirm.confirm('run sqlite vacuum?'))) return
  try {
    await vacuum()
    toast.success('vacuum complete')
  } catch (e) {
    toast.error((e as Error).message)
  }
}
</script>

<template>
  <div>
    <div class="panel">
      <div class="panel-label">cache statistics</div>
      <template v-if="stats">
        <div class="meta-text">domains: {{ stats.domains }}</div>
        <div class="meta-text">records: {{ stats.records_total }} (v4:{{ stats.records_v4 }} v6:{{ stats.records_v6 }})</div>
        <div class="meta-text">valid: {{ stats.records_valid }} | failed: {{ stats.records_failed }}</div>
        <div class="meta-text">oldest: {{ fmtDate(stats.oldest_updated) }}</div>
        <div class="meta-text">newest: {{ fmtDate(stats.newest_updated) }}</div>
      </template>
      <template v-else>
        <StatusBadge type="muted">loading...</StatusBadge>
      </template>
    </div>

    <div class="panel">
      <div class="panel-label">actions</div>
      <div class="flex gap-2">
        <button class="btn btn-warn" @click="handleVacuum">vacuum</button>
      </div>
    </div>
  </div>
</template>