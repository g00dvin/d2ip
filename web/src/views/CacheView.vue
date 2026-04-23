<script setup lang="ts">
import { onMounted } from 'vue'
import { usePolling } from '@/composables/usePolling'
import { useToast } from '@/stores/toast'
import { useCacheStore } from '@/stores/cache'
const cache = useCacheStore()
import StatusBadge from '@/components/StatusBadge.vue'
import { useConfirm } from '@/composables/useConfirm'

const toast = useToast()
const confirm = useConfirm()

const poll = usePolling(() => cache.fetchStats(), 30_000)
onMounted(() => poll.start())

function fmtDate(d: string | null | undefined) {
  return d ? new Date(d).toLocaleString() : '-'
}

async function handleVacuum() {
  if (!(await confirm.confirm('run sqlite vacuum?'))) return
  try {
    await cache.vacuum()
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
      <template v-if="cache.stats">
        <div class="meta-text">domains: {{ cache.stats.domains }}</div>
        <div class="meta-text">records: {{ cache.stats.records_total }} (v4:{{ cache.stats.records_v4 }} v6:{{ cache.stats.records_v6 }})</div>
        <div class="meta-text">valid: {{ cache.stats.records_valid }} | failed: {{ cache.stats.records_failed }}</div>
        <div class="meta-text">oldest: {{ fmtDate(cache.stats.oldest_updated) }}</div>
        <div class="meta-text">newest: {{ fmtDate(cache.stats.newest_updated) }}</div>
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